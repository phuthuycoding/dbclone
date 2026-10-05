// dbclone copies selected databases from staging into the local docker containers.
//
// Layers:
//
//	main            flags + wiring
//	internal/ui     terminal prompts (huh) and live progress board (mpb)
//	internal/clone  engine: worker pool, dump → restore streaming, progress events
//	internal/driver adapter interface; one package per engine (mongo, mysql, ...)
//	internal/docker runs the engine's own tools inside the local containers
//	internal/config .env.staging load/save
//
// Adding an engine = a new package implementing driver.Driver + one line in drivers below.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/phuthuycoding/dbclone/internal/clone"
	"github.com/phuthuycoding/dbclone/internal/config"
	"github.com/phuthuycoding/dbclone/internal/driver"
	"github.com/phuthuycoding/dbclone/internal/driver/mongo"
	"github.com/phuthuycoding/dbclone/internal/driver/mysql"
	"github.com/phuthuycoding/dbclone/internal/preflight"
	"github.com/phuthuycoding/dbclone/internal/ui"
)

// version is set by the release build (-ldflags "-X main.version=..."); `go install`
// builds fall back to the module version recorded in the binary.
var version = "dev"

var drivers = []driver.Driver{
	mongo.New(),
	mysql.New(),
}

var (
	envFile  = flag.String("env", ".env.staging", "file holding the staging connection settings")
	parallel = flag.Int("j", 8, "global pool: concurrent dump→restore streams across all databases")
	workers  = flag.Int("w", 4, "max concurrent streams for one database (mongo collections / mysql table groups)")
	only     = flag.String("only", "", "skip the picker: comma list of engine:db or engine:db.table, e.g. mongo:shop.orders,mysql:app")
	yes      = flag.Bool("yes", false, "do not ask before overwriting a database that already exists locally")
	all      = flag.Bool("all", false, "skip the picker and clone every database on staging")
	fresh    = flag.Bool("fresh", false, "drop each local database first, so local ends up identical to staging")
	logDir   = flag.String("logs", "logs", "directory for per-database logs")
	check    = flag.Bool("check", false, "check the local setup (Docker, containers) and exit")
	showVer  = flag.Bool("version", false, "print the version and exit")
	setup    = flag.Bool("setup", false, "re-enter the staging connections even if the env file exists")
)

func main() {
	flag.Parse()
	if *showVer {
		if info, ok := debug.ReadBuildInfo(); ok && version == "dev" && info.Main.Version != "" {
			version = info.Main.Version
		}
		fmt.Println("dbclone", version)
		return
	}
	if err := run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Println("aborted")
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	report, dockerErr := preflight.Run(ctx, drivers)
	found, err := config.Load(*envFile)
	if err != nil {
		return err
	}
	needSetup := !found || *setup
	if *check || needSetup || report.Failed() {
		ui.PrintReport(report)
	}
	switch {
	case dockerErr != nil:
		return errors.New("fix Docker first (see above), then run dbclone again")
	case len(report.Ready) == 0:
		return errors.New("no usable local database container; see the fixes above")
	case *check && report.Failed():
		return errors.New("some engines cannot be used; see the fixes above")
	case *check:
		return nil
	}
	ready := report.Ready
	scripted := *only != "" || *all
	if needSetup && scripted {
		return fmt.Errorf("no connection file %s; run dbclone without -only/-all once to set it up", *envFile)
	}

	// Ask for the source connections when needed, test them by listing databases, and on
	// failure offer to re-enter them instead of exiting.
	var available []clone.Job
	var local map[string]bool
	for {
		if needSetup {
			if err := ui.Onboard(ready); err != nil {
				return err
			}
		}
		available, local, err = discover(ctx, ready)
		if err == nil {
			break
		}
		if scripted {
			return err
		}
		ui.PrintError(err)
		again, cerr := ui.Confirm("Could not use the source connections. Enter them again?", "", "Re-enter", "Quit")
		if cerr != nil {
			return cerr
		}
		if !again {
			return err
		}
		needSetup = true
	}
	if needSetup {
		save, err := ui.Confirm("Source connections work. Save them to "+*envFile+" for next time?",
			"The file holds passwords and is created with mode 0600", "Save", "Skip")
		if err != nil {
			return err
		}
		if save {
			if err := config.Save(*envFile, drivers); err != nil {
				return err
			}
			fmt.Println("saved " + *envFile)
		}
	}

	jobs, err := choose(ctx, available, local)
	if err != nil || len(jobs) == 0 {
		if err == nil {
			fmt.Println("no database selected")
		}
		return err
	}

	runDir := filepath.Join(*logDir, time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return err
	}
	fmt.Printf("Cloning %d databases · pool %d streams · max %d streams/db · logs: %s\n\n",
		len(jobs), *parallel, min(*workers, *parallel), runDir)

	board := ui.NewBoard()
	reps := make([]clone.Reporter, len(jobs))
	for i, j := range jobs {
		reps[i] = board.Track(j.Label())
	}
	started := time.Now()
	results := clone.Run(ctx, jobs, reps, clone.Options{Parallel: *parallel, Workers: *workers, Fresh: *fresh, LogDir: runDir})
	board.Wait()

	var failed []string
	for _, r := range results {
		for _, w := range r.Warnings {
			fmt.Printf("\n! %s: %s\n", r.Job.Label(), w)
		}
		if r.Err != nil {
			failed = append(failed, r.Job.String())
			fmt.Printf("\n✗ %s: %v\n  log: %s\n%s", r.Job.Label(), r.Err, r.Log, tail(r.Log, 5))
		}
	}
	fmt.Printf("\nDone %d/%d databases in %s\n", len(jobs)-len(failed), len(jobs),
		time.Since(started).Round(time.Second))
	if len(failed) > 0 {
		return fmt.Errorf("failed: %s", strings.Join(failed, ", "))
	}
	return nil
}

// parseOnly turns "engine:db" and "engine:db.table" entries into jobs. Entries for the
// same database merge; a bare "engine:db" anywhere means the whole database.
func parseOnly(spec string, available []clone.Job) ([]clone.Job, error) {
	var jobs []clone.Job
	whole := map[string]bool{}
	for _, s := range strings.Split(spec, ",") {
		s = strings.TrimSpace(s)
		engine, rest, _ := strings.Cut(s, ":")
		db, obj, _ := strings.Cut(rest, ".") // database names have no dot; object names may
		key := engine + ":" + db
		i := slices.IndexFunc(available, func(j clone.Job) bool { return j.String() == key })
		if i < 0 {
			return nil, fmt.Errorf("%q not found on staging", key)
		}
		k := slices.IndexFunc(jobs, func(j clone.Job) bool { return j.String() == key })
		if k < 0 {
			jobs = append(jobs, available[i])
			k = len(jobs) - 1
		}
		if obj == "" {
			whole[key] = true
		} else if !slices.Contains(jobs[k].Only, obj) {
			jobs[k].Only = append(jobs[k].Only, obj)
		}
	}
	for k := range jobs {
		if whole[jobs[k].String()] {
			jobs[k].Only = nil
		}
	}
	return jobs, nil
}

// discover lists source databases per configured driver, and which already exist locally.
func discover(ctx context.Context, ds []driver.Driver) ([]clone.Job, map[string]bool, error) {
	var available []clone.Job
	local := map[string]bool{}
	used := 0
	for _, d := range ds {
		if !d.Configured() {
			continue
		}
		used++
		names, err := d.ListSource(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot list %s databases on the source: %w", d.Name(), err)
		}
		for _, n := range names {
			available = append(available, clone.Job{Driver: d, DB: n})
		}
		existing, err := d.ListLocal(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot list local %s databases: %w", d.Name(), err)
		}
		for _, n := range existing {
			local[d.Name()+":"+n] = true
		}
	}
	if used == 0 {
		return nil, nil, errors.New("no source connection entered for any engine")
	}
	return available, local, nil
}

// choose resolves -only or shows the picker, then confirms overwriting local databases.
func choose(ctx context.Context, available []clone.Job, local map[string]bool) ([]clone.Job, error) {
	var jobs []clone.Job
	var err error
	switch {
	case *all:
		jobs = available
	case *only != "":
		if jobs, err = parseOnly(*only, available); err != nil {
			return nil, err
		}
	default:
		if len(available) == 0 {
			return nil, errors.New("staging has no database to clone")
		}
		if jobs, err = ui.Pick(available, local); err != nil {
			return nil, err
		}
		if len(jobs) > 0 {
			if jobs, err = ui.Narrow(ctx, jobs); err != nil {
				return nil, err
			}
		}
	}

	var overwrite []string
	for _, j := range jobs {
		switch {
		case !local[j.String()]:
		case len(j.Only) > 0:
			overwrite = append(overwrite, j.Label()+" — the selected objects are replaced, the rest is kept")
		case *fresh:
			overwrite = append(overwrite, j.String()+" — DROPPED and recreated (-fresh)")
		default:
			overwrite = append(overwrite, j.String()+" — objects with the same name are replaced")
		}
	}
	if len(overwrite) > 0 && !*yes {
		ok, err := ui.Confirm("These already exist locally:", strings.Join(overwrite, "\n"), "Overwrite", "Cancel")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, huh.ErrUserAborted
		}
	}
	return jobs, nil
}

func tail(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return "    " + strings.Join(lines, "\n    ") + "\n"
}
