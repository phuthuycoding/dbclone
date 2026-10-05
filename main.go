// dbclone clones databases between any two connections — a remote profile (staging,
// prod, …) or the local Docker containers — in parallel, with retries and live progress.
//
// Layers:
//
//	main               flags + wiring + safety checks
//	internal/ui        terminal prompts (huh) and live progress board (mpb)
//	internal/clone     engine: worker pool, dump → restore streaming, progress events
//	internal/driver    adapter interface; one package per engine (mongo, mysql, ...)
//	internal/preflight checks Docker and the local containers before anything runs
//	internal/docker    runs the engine's own tools inside the local containers
//	internal/config    connection profiles
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
	configPath = flag.String("config", config.DefaultPath(), "connection profiles file")
	from       = flag.String("from", "", "source profile (asked when empty; \"local\" = your Docker containers)")
	to         = flag.String("to", "", "target profile (default local; asked when empty and run interactively)")
	confirm    = flag.String("confirm", "", "the target profile name, required to write to a non-local target without the prompt")
	parallel   = flag.Int("j", 8, "global pool: concurrent dump→restore streams across all databases")
	workers    = flag.Int("w", 4, "max concurrent streams for one database (mongo collections / mysql table groups)")
	only       = flag.String("only", "", "skip the picker: comma list of engine:db or engine:db.table, e.g. mongo:shop.orders,mysql:app")
	yes        = flag.Bool("yes", false, "do not ask before overwriting databases on the local target")
	all        = flag.Bool("all", false, "skip the picker and clone every database of the source")
	fresh      = flag.Bool("fresh", false, "drop each database on the local target first, so it ends up identical to the source")
	logDir     = flag.String("logs", "logs", "directory for per-database logs")
	check      = flag.Bool("check", false, "check the local setup (Docker, containers) and exit")
	setup      = flag.Bool("setup", false, "add, edit or delete connection profiles before cloning")
	showVer    = flag.Bool("version", false, "print the version and exit")
)

// legacyEnv is the connection file of versions before profiles; it is imported once.
const legacyEnv = ".env.staging"

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
	store, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if added, err := store.ImportLegacy(legacyEnv); err != nil {
		return err
	} else if added {
		fmt.Printf("imported %s as profile \"staging\" into %s\n\n", legacyEnv, store.Path)
	}
	scripted := *only != "" || *all
	firstRun := len(store.Profiles) == 0
	if *check || *setup || firstRun || report.Failed() {
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

	if *setup || (firstRun && !scripted && *from != driver.LocalName) {
		if firstRun {
			fmt.Println("No connection profiles yet — add the servers you clone from or to.")
		}
		if err := ui.ManageProfiles(store, ready); err != nil {
			return err
		}
	}

	src, dst, err := pickEndpoints(store, scripted)
	if err != nil {
		return err
	}
	if err := guard(src, dst, ready); err != nil {
		return err
	}

	// List the databases on both sides; when a profile does not work, offer to fix it.
	var available []clone.Job
	var existing map[string]bool
	for {
		available, existing, err = discover(ctx, ready, src, dst)
		if err == nil {
			break
		}
		var epErr *endpointError
		if scripted || !errors.As(err, &epErr) || epErr.name == driver.LocalName {
			return err
		}
		ui.PrintError(err)
		fix, cerr := ui.Confirm("Edit profile "+epErr.name+" and try again?", "", "Edit", "Quit")
		if cerr != nil || !fix {
			if cerr != nil {
				return cerr
			}
			return err
		}
		p, _ := store.Get(epErr.name)
		if err := ui.EditProfile(ready, &p, store.Names()); err != nil {
			return err
		}
		store.Put(epErr.name, p)
		if err := store.Save(); err != nil {
			return err
		}
		if src, err = store.Endpoint(endpointName(src, epErr.name, p.Name)); err != nil {
			return err
		}
		if dst, err = store.Endpoint(endpointName(dst, epErr.name, p.Name)); err != nil {
			return err
		}
	}

	jobs, err := choose(ctx, src, dst, available, existing)
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
	fmt.Printf("Cloning %d databases %s → %s · pool %d streams · max %d streams/db · logs: %s\n\n",
		len(jobs), src.Name, dst.Name, *parallel, min(*workers, *parallel), runDir)

	board := ui.NewBoard()
	reps := make([]clone.Reporter, len(jobs))
	for i, j := range jobs {
		reps[i] = board.Track(j.Label())
	}
	started := time.Now()
	results := clone.Run(ctx, jobs, reps, clone.Options{
		Parallel: *parallel, Workers: *workers, Fresh: *fresh, LogDir: runDir, Source: src, Target: dst,
	})
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

func endpointName(ep driver.Endpoint, old, renamed string) string {
	if ep.Name == old {
		return renamed
	}
	return ep.Name
}

// pickEndpoints resolves -from / -to, asking for whatever is missing when interactive.
func pickEndpoints(store *config.Store, scripted bool) (src, dst driver.Endpoint, err error) {
	names := append(store.Names(), driver.LocalName)
	srcName, dstName := *from, *to
	if srcName == "" {
		switch {
		case scripted && len(store.Profiles) == 1:
			srcName = store.Profiles[0].Name
		case scripted:
			return src, dst, errors.New("-from is required with -only/-all when there is not exactly one profile")
		default:
			if srcName, err = ui.SelectProfile("Clone FROM", names, names[0]); err != nil {
				return src, dst, err
			}
		}
	}
	if dstName == "" {
		dstName = driver.LocalName
		if !scripted {
			targets := slices.DeleteFunc(slices.Clone(names), func(n string) bool { return n == srcName })
			if len(targets) == 0 {
				return src, dst, errors.New("nothing to clone to yet; add a connection profile with -setup")
			}
			def := driver.LocalName
			if srcName == driver.LocalName && len(targets) > 0 {
				def = targets[0]
			}
			if dstName, err = ui.SelectProfile("Clone "+srcName+" TO", targets, def); err != nil {
				return src, dst, err
			}
		}
	}
	if src, err = store.Endpoint(srcName); err != nil {
		return src, dst, err
	}
	dst, err = store.Endpoint(dstName)
	return src, dst, err
}

// guard refuses combinations that can destroy data by accident.
func guard(src, dst driver.Endpoint, ds []driver.Driver) error {
	if src.Name == dst.Name {
		return fmt.Errorf("source and target are the same profile %q", src.Name)
	}
	for _, d := range ds {
		if d.Configured(src) && d.Configured(dst) && d.Address(src) == d.Address(dst) {
			return fmt.Errorf("%s: %q and %q point at the same server (%s); cloning it onto itself would overwrite the source",
				d.Name(), src.Name, dst.Name, d.Address(src))
		}
	}
	if !dst.Local && *fresh {
		return fmt.Errorf("-fresh drops whole databases and is only allowed when the target is %q", driver.LocalName)
	}
	return nil
}

// endpointError names the profile whose connection failed, so it can be edited.
type endpointError struct {
	name string
	err  error
}

func (e *endpointError) Error() string { return e.err.Error() }
func (e *endpointError) Unwrap() error { return e.err }

// discover lists the source databases of every engine both endpoints have, and which of
// them already exist on the target.
func discover(ctx context.Context, ds []driver.Driver, src, dst driver.Endpoint) ([]clone.Job, map[string]bool, error) {
	var available []clone.Job
	existing := map[string]bool{}
	used := 0
	for _, d := range ds {
		if !d.Configured(src) || !d.Configured(dst) {
			continue
		}
		used++
		names, err := d.Databases(ctx, src)
		if err != nil {
			return nil, nil, &endpointError{src.Name, fmt.Errorf("cannot list %s databases on %s: %w", d.Name(), src.Name, err)}
		}
		for _, n := range names {
			available = append(available, clone.Job{Driver: d, DB: n})
		}
		have, err := d.Databases(ctx, dst)
		if err != nil {
			return nil, nil, &endpointError{dst.Name, fmt.Errorf("cannot list %s databases on %s: %w", d.Name(), dst.Name, err)}
		}
		for _, n := range have {
			existing[d.Name()+":"+n] = true
		}
	}
	if used == 0 {
		return nil, nil, fmt.Errorf("%s and %s have no engine in common", src.Name, dst.Name)
	}
	return available, existing, nil
}

// choose resolves -only / -all or shows the pickers, then confirms what will be written.
// A non-local target always needs its name typed (or passed with -confirm); -yes does not
// skip that.
func choose(ctx context.Context, src, dst driver.Endpoint, available []clone.Job, existing map[string]bool) ([]clone.Job, error) {
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
			return nil, fmt.Errorf("%s has no database to clone", src.Name)
		}
		if jobs, err = ui.Pick(available, existing, dst.Name); err != nil {
			return nil, err
		}
		if len(jobs) > 0 {
			if jobs, err = ui.Narrow(ctx, src, jobs); err != nil {
				return nil, err
			}
		}
	}
	if len(jobs) == 0 {
		return nil, nil
	}

	var lines []string
	for _, j := range jobs {
		switch {
		case !existing[j.String()]:
			lines = append(lines, j.Label()+" — new on "+dst.Name)
		case len(j.Only) > 0:
			lines = append(lines, j.Label()+" — the selected objects are replaced, the rest is kept")
		case *fresh:
			lines = append(lines, j.String()+" — DROPPED and recreated (-fresh)")
		default:
			lines = append(lines, j.String()+" — objects with the same name are replaced")
		}
	}
	summary := strings.Join(lines, "\n")

	if !dst.Local {
		switch {
		case *confirm == dst.Name:
			return jobs, nil
		case *confirm != "":
			return nil, fmt.Errorf("-confirm %q does not match the target %q", *confirm, dst.Name)
		case scriptedRun():
			return nil, fmt.Errorf("writing to %q needs -confirm %s", dst.Name, dst.Name)
		}
		return jobs, ui.ConfirmTarget(dst.Name, summary)
	}
	if slices.ContainsFunc(jobs, func(j clone.Job) bool { return existing[j.String()] }) && !*yes {
		ok, err := ui.Confirm("Clone into "+dst.Name+":", summary, "Overwrite", "Cancel")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, huh.ErrUserAborted
		}
	}
	return jobs, nil
}

func scriptedRun() bool { return *only != "" || *all }

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
