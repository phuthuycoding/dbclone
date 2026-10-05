// Package mysql is the MySQL driver. mysqldump is single-threaded, so the driver gets its
// parallelism by splitting the tables of a database into groups and running one
// `mysqldump <tables> | mysql` pipe per group, balanced by table size.
package mysql

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/phuthuycoding/dbclone/internal/docker"
	"github.com/phuthuycoding/dbclone/internal/driver"
)

const (
	keyHost     = "STG_MYSQL_HOST"
	keyPort     = "STG_MYSQL_PORT"
	keyUser     = "STG_MYSQL_USER"
	keyPassword = "STG_MYSQL_PASSWORD"

	// MYSQL_PWD keeps the password off argv inside the container too.
	source = `MYSQL_PWD="$STG_MYSQL_PASSWORD" `
	conn   = `-h"$STG_MYSQL_HOST" -P"$STG_MYSQL_PORT" -u"$STG_MYSQL_USER"`
	// The local container already carries MYSQL_ROOT_PASSWORD from docker-compose.
	local = `MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot`

	scriptQuerySource = source + `mysql ` + conn + ` -N -B -e "$SQL"`
	scriptQueryLocal  = local + ` -N -B -e "$SQL"`

	dumpBase = source + `mysqldump ` + conn +
		` --single-transaction --quick --set-gtid-purged=OFF --no-tablespaces --column-statistics=0`
	// $TABLES is a space-separated list of names validated against safeName, so the
	// unquoted expansion splits exactly on table boundaries.
	scriptDumpTables = dumpBase + ` --skip-routines --skip-events --triggers "$DB" $TABLES`
	// Views reference tables, so they are dumped after every table group has landed.
	scriptDumpViews = dumpBase + ` --no-data --skip-triggers --skip-routines --skip-events "$DB" $TABLES`
	// $EVENTS is --events or --skip-events, depending on the staging user's EVENT privilege.
	scriptDumpCode = dumpBase + ` --no-data --no-create-info --skip-triggers --routines $EVENTS "$DB"`
	// Binary logging off for the import session: the local server does not replicate and
	// writing every row twice roughly halves import speed.
	scriptRestore = local + ` --init-command="SET SESSION sql_log_bin=0" "$DB"`
)

var (
	systemDBs   = []string{"information_schema", "mysql", "performance_schema", "sys", "mysql_innodb_cluster_metadata"}
	safeName    = regexp.MustCompile(`^[A-Za-z0-9_$-]+$`)
	tableMarker = regexp.MustCompile("-- Dumping data for table `([^`]+)`")
)

type Driver struct{}

func New() *Driver { return &Driver{} }

func (*Driver) Name() string { return "mysql" }

func (*Driver) Fields() []driver.Field {
	return []driver.Field{
		{Key: keyHost, Title: "Host", Description: "Leave empty to skip MySQL"},
		{Key: keyPort, Title: "Port", Default: "3306"},
		{Key: keyUser, Title: "User"},
		{Key: keyPassword, Title: "Password", Secret: true},
	}
}

func (*Driver) Configured() bool {
	return os.Getenv(keyHost) != "" && os.Getenv(keyUser) != ""
}

func container() string { return driver.Env("DBCLONE_MYSQL_CONTAINER", "mysql") }

func (*Driver) Local() driver.Local {
	return driver.Local{
		Container:    container(),
		ContainerEnv: "DBCLONE_MYSQL_CONTAINER",
		Tools:        []string{"mysql", "mysqldump"},
		Credentials:  []string{"MYSQL_ROOT_PASSWORD"},
		RunExample: "docker run -d --name " + container() + " -p 3306:3306 " +
			"-e MYSQL_ROOT_PASSWORD=<password> mysql:8.4",
	}
}

func sh(ctx context.Context, script string, stdin bool, vars map[string]string) *exec.Cmd {
	env := map[string]string{
		keyHost:     os.Getenv(keyHost),
		keyPort:     driver.Env(keyPort, "3306"),
		keyUser:     os.Getenv(keyUser),
		keyPassword: os.Getenv(keyPassword),
		"DB":        "",
		"SQL":       "",
		"TABLES":    "",
		"EVENTS":    "",
	}
	for k, v := range vars {
		env[k] = v
	}
	return docker.Sh(ctx, container(), script, stdin, env)
}

func (*Driver) ListSource(ctx context.Context) ([]string, error) {
	return docker.Lines(sh(ctx, scriptQuerySource, false, map[string]string{"SQL": "SHOW DATABASES"}), systemDBs...)
}

func (*Driver) ListLocal(ctx context.Context) ([]string, error) {
	return docker.Lines(sh(ctx, scriptQueryLocal, false, map[string]string{"SQL": "SHOW DATABASES"}), systemDBs...)
}

func (*Driver) Prepare(ctx context.Context, db string, fresh bool) error {
	sql := "CREATE DATABASE IF NOT EXISTS `" + db + "`"
	if fresh {
		sql = "DROP DATABASE IF EXISTS `" + db + "`; " + sql
	}
	_, err := docker.Output(sh(ctx, scriptQueryLocal, false, map[string]string{"SQL": sql}))
	return err
}

type table struct {
	name string
	size int64
}

// Plan: phase 1 = base tables in up to `workers` size-balanced groups, phase 2 = views
// plus routines/events. Each group is its own --single-transaction, so groups are not one
// snapshot of each other — fine for a dev copy, not for a backup.
func (*Driver) Objects(ctx context.Context, db string) ([]driver.Object, error) {
	// data_length is InnoDB pages, not SQL text, so the sizes are only a scheduling and
	// progress guide. db was validated by the engine before reaching here.
	out, err := docker.Output(sh(ctx, scriptQuerySource, false, map[string]string{"SQL": fmt.Sprintf(
		"SELECT TABLE_NAME, TABLE_TYPE, COALESCE(DATA_LENGTH,0) FROM information_schema.TABLES WHERE TABLE_SCHEMA='%s'", db)}))
	if err != nil {
		return nil, err
	}
	var objs []driver.Object
	for _, line := range strings.Split(out, "\n") {
		cols := strings.Split(line, "\t")
		if len(cols) != 3 {
			continue
		}
		kind := "table"
		if cols[1] == "VIEW" {
			kind = "view"
		}
		size, _ := strconv.ParseInt(cols[2], 10, 64)
		objs = append(objs, driver.Object{Name: cols[0], Kind: kind, Size: size})
	}
	return objs, nil
}

// Plan: phase 1 = the selected base tables in up to `workers` size-balanced groups,
// phase 2 = the selected views, plus routines/events when the whole database is cloned.
// Each group is its own --single-transaction, so groups are not one snapshot of each
// other — fine for a dev copy, not for a backup.
func (d *Driver) Plan(ctx context.Context, db string, workers int, only []string) (driver.Plan, error) {
	objs, err := d.Objects(ctx, db)
	if err != nil {
		return driver.Plan{}, err
	}
	selected, err := driver.Select(objs, only)
	if err != nil {
		return driver.Plan{}, err
	}
	var tables []table
	var views []string
	var total int64
	for _, o := range objs {
		if !selected[o.Name] {
			continue
		}
		if !safeName.MatchString(o.Name) {
			return driver.Plan{}, fmt.Errorf("table %q has unsupported characters", o.Name)
		}
		if o.Kind == "view" {
			views = append(views, o.Name)
			continue
		}
		tables = append(tables, table{o.Name, o.Size})
		total += o.Size
	}

	var phase1 []driver.Stream
	for _, g := range balance(tables, workers) {
		phase1 = append(phase1, stream(scriptDumpTables, map[string]string{"DB": db, "TABLES": strings.Join(g.names, " ")}, g.size, label(g.names)))
	}
	var phase2 []driver.Stream
	var warnings []string
	if len(only) == 0 {
		// SHOW EVENTS needs the EVENT privilege, which read-only staging users often lack;
		// without it mysqldump --events aborts the whole dump, so skip events and say so.
		events, codeLabel := "--events", "routines/events"
		if _, err := docker.Output(sh(ctx, scriptQuerySource, false, map[string]string{"SQL": "SHOW EVENTS FROM `" + db + "`"})); err != nil {
			events, codeLabel = "--skip-events", "routines"
			warnings = append(warnings, "events skipped: staging user cannot SHOW EVENTS ("+firstLine(err.Error())+")")
		}
		phase2 = append(phase2, stream(scriptDumpCode, map[string]string{"DB": db, "EVENTS": events}, 0, codeLabel))
	}
	if len(views) > 0 {
		phase2 = append(phase2, stream(scriptDumpViews, map[string]string{"DB": db, "TABLES": strings.Join(views, " ")}, 0, "views"))
	}
	return driver.Plan{Estimate: total, Phases: [][]driver.Stream{phase1, phase2}, Warnings: warnings}, nil
}

type group struct {
	names []string
	size  int64
}

// balance spreads tables over at most n groups with the longest-processing-time rule:
// biggest table first, always into the currently lightest group. It keeps the slowest
// group — the one the whole database waits for — as short as a greedy split can.
func balance(tables []table, n int) []group {
	n = min(max(n, 1), len(tables))
	slices.SortFunc(tables, func(a, b table) int { return cmp.Compare(b.size, a.size) })
	groups := make([]group, n)
	for _, t := range tables {
		i := 0
		for j := range groups {
			if groups[j].size < groups[i].size {
				i = j
			}
		}
		groups[i].names = append(groups[i].names, t.name)
		groups[i].size += t.size
	}
	return groups
}

func stream(dump string, vars map[string]string, size int64, lbl string) driver.Stream {
	return driver.Stream{
		Label:   lbl,
		Size:    size,
		Weight:  1,
		Dump:    func(ctx context.Context) *exec.Cmd { return sh(ctx, dump, false, vars) },
		Restore: func(ctx context.Context) *exec.Cmd { return sh(ctx, scriptRestore, true, vars) },
	}
}

func label(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%s +%d", names[0], len(names)-1)
}

func (*Driver) StageFromStream(chunk []byte) string {
	if !bytes.Contains(chunk, []byte("-- Dumping data for table")) {
		return ""
	}
	all := tableMarker.FindAllSubmatch(chunk, -1)
	if len(all) == 0 {
		return ""
	}
	return string(all[len(all)-1][1])
}

func (*Driver) StageFromLog(string) string { return "" }

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if i := strings.Index(s, "ERROR "); i >= 0 {
		s = s[i:]
	}
	return s
}
