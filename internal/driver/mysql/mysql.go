// Package mysql is the MySQL driver. mysqldump is single-threaded, so the driver gets its
// parallelism by splitting the tables of a database into groups and running one
// `mysqldump <tables> | mysql` pipe per group, balanced by table size. Both tools run
// inside the local mysql container towards any pair of endpoints.
package mysql

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/phuthuycoding/dbclone/internal/docker"
	"github.com/phuthuycoding/dbclone/internal/driver"
)

const (
	keyHost     = "mysql.host"
	keyPort     = "mysql.port"
	keyUser     = "mysql.user"
	keyPassword = "mysql.password"

	dumpFlags = ` --single-transaction --quick --set-gtid-purged=OFF --no-tablespaces --column-statistics=0`
	// $TABLES is a space-separated list of names validated against safeName, so the
	// unquoted expansion splits exactly on table boundaries.
	dumpTables = dumpFlags + ` --skip-routines --skip-events --skip-triggers "$DB" $TABLES`
	// Triggers come after the data: creating one can be refused on a target with binary
	// logging (needs SUPER or log_bin_trust_function_creators), and that must not cost the data.
	dumpTriggers = dumpFlags + ` --no-data --no-create-info --triggers --skip-routines --skip-events "$DB" $TABLES`
	// Views reference tables, so they are dumped after every table group has landed.
	dumpViews = dumpFlags + ` --no-data --skip-triggers --skip-routines --skip-events "$DB" $TABLES`
	// $EVENTS is --events or --skip-events, depending on the source user's EVENT privilege.
	dumpCode = dumpFlags + ` --no-data --no-create-info --skip-triggers --routines $EVENTS "$DB"`

	// A remote target user is usually not allowed to create objects owned by someone
	// else, so DEFINER clauses are stripped from view, routine and trigger definitions
	// (lines starting with /*! or CREATE; data lines start with INSERT).
	stripDefiner = "sed -E '/^(\\/\\*!|CREATE)/ s/ ?DEFINER=`[^`]*`@`[^`]*`//g' | "
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
		{Key: keyHost, Title: "MySQL host", Description: "Leave empty if this profile has no MySQL"},
		{Key: keyPort, Title: "MySQL port", Default: "3306"},
		{Key: keyUser, Title: "MySQL user"},
		{Key: keyPassword, Title: "MySQL password", Secret: true},
	}
}

func (*Driver) Configured(ep driver.Endpoint) bool {
	return ep.Local || (ep.Get(keyHost) != "" && ep.Get(keyUser) != "")
}

func (*Driver) Address(ep driver.Endpoint) string {
	if ep.Local {
		return driver.LocalName
	}
	return strings.ToLower(ep.Get(keyHost)) + ":" + port(ep)
}

func port(ep driver.Endpoint) string { return cmp.Or(ep.Get(keyPort), "3306") }

func container() string { return driver.Env("DBCLONE_MYSQL_CONTAINER", "mysql") }

func (*Driver) Local() driver.Local {
	return driver.Local{
		Container:    container(),
		ContainerEnv: "DBCLONE_MYSQL_CONTAINER",
		Tools:        []string{"mysql", "mysqldump", "sed"},
		Credentials:  []string{"MYSQL_ROOT_PASSWORD"},
		RunExample: "docker run -d --name " + container() + " -p 3306:3306 " +
			"-e MYSQL_ROOT_PASSWORD=<password> mysql:8.4",
	}
}

// conn returns the prefix that makes a client reach ep — password via MYSQL_PWD so it
// never sits on argv — and the env vars it reads, named after prefix (e.g. SRC_HOST).
// The local endpoint logs in as root with the password the container was created with.
func conn(ep driver.Endpoint, prefix string) (pwd, args string, env map[string]string) {
	if ep.Local {
		return `MYSQL_PWD="$MYSQL_ROOT_PASSWORD" `, ` -uroot`, nil
	}
	return `MYSQL_PWD="$` + prefix + `_PASSWORD" `,
		` -h"$` + prefix + `_HOST" -P"$` + prefix + `_PORT" -u"$` + prefix + `_USER"`,
		map[string]string{
			prefix + "_HOST":     ep.Get(keyHost),
			prefix + "_PORT":     port(ep),
			prefix + "_USER":     ep.Get(keyUser),
			prefix + "_PASSWORD": ep.Get(keyPassword),
		}
}

func sh(ctx context.Context, script string, stdin bool, env ...map[string]string) *exec.Cmd {
	all := map[string]string{}
	for _, m := range env {
		for k, v := range m {
			all[k] = v
		}
	}
	return docker.Sh(ctx, container(), script, stdin, all)
}

// query runs one SQL statement on ep and returns its tab-separated rows.
func query(ctx context.Context, ep driver.Endpoint, sql string) (string, error) {
	pwd, args, env := conn(ep, "EP")
	return docker.Output(sh(ctx, pwd+`mysql`+args+` -N -B -e "$SQL"`, false, env, map[string]string{"SQL": sql}))
}

func (*Driver) Databases(ctx context.Context, ep driver.Endpoint) ([]string, error) {
	out, err := query(ctx, ep, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	return docker.SplitLines(out, systemDBs...), nil
}

// Prepare creates the database only when it is missing — a remote target user may lack the
// CREATE privilege on a database that already exists. db was validated by the engine.
func (*Driver) Prepare(ctx context.Context, dst driver.Endpoint, db string, fresh bool) error {
	if fresh {
		if _, err := query(ctx, dst, "DROP DATABASE IF EXISTS `"+db+"`"); err != nil {
			return err
		}
	}
	n, err := query(ctx, dst, fmt.Sprintf("SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='%s'", db))
	if err != nil || n != "0" {
		return err
	}
	_, err = query(ctx, dst, "CREATE DATABASE `"+db+"`")
	return err
}

func (*Driver) Objects(ctx context.Context, ep driver.Endpoint, db string) ([]driver.Object, error) {
	// data_length is InnoDB pages, not SQL text, so the sizes are only a scheduling and
	// progress guide. db was validated by the engine before reaching here.
	out, err := query(ctx, ep, fmt.Sprintf(
		"SELECT TABLE_NAME, TABLE_TYPE, COALESCE(DATA_LENGTH,0) FROM information_schema.TABLES WHERE TABLE_SCHEMA='%s'", db))
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

type table struct {
	name string
	size int64
}

// Plan: phase 1 = the selected base tables in up to `workers` size-balanced groups,
// phase 2 = the selected views, plus routines/events when the whole database is cloned.
// Each group is its own --single-transaction, so groups are not one snapshot of each
// other — fine for a dev copy, not for a backup.
func (d *Driver) Plan(ctx context.Context, src, dst driver.Endpoint, db string, workers int, only []string) (driver.Plan, error) {
	objs, err := d.Objects(ctx, src, db)
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

	p := pipe{src: src, dst: dst, db: db}
	var phase1 []driver.Stream
	for _, g := range balance(tables, workers) {
		phase1 = append(phase1, p.stream(dumpTables, map[string]string{"TABLES": strings.Join(g.names, " ")}, g.size, label(g.names)))
	}
	var phase2 []driver.Stream
	var warnings []string
	if len(only) == 0 {
		// SHOW EVENTS needs the EVENT privilege, which read-only users often lack; without
		// it mysqldump --events aborts the whole dump, so skip events and say so.
		events, codeLabel := "--events", "routines/events"
		if _, err := query(ctx, src, "SHOW EVENTS FROM `"+db+"`"); err != nil {
			events, codeLabel = "--skip-events", "routines"
			warnings = append(warnings, "events skipped: source user cannot SHOW EVENTS ("+firstLine(err.Error())+")")
		}
		phase2 = append(phase2, p.stream(dumpCode, map[string]string{"EVENTS": events}, 0, codeLabel))
	}
	if len(tables) > 0 {
		names := make([]string, len(tables))
		for i, t := range tables {
			names[i] = t.name
		}
		phase2 = append(phase2, p.stream(dumpTriggers, map[string]string{"TABLES": strings.Join(names, " ")}, 0, "triggers"))
	}
	if len(views) > 0 {
		phase2 = append(phase2, p.stream(dumpViews, map[string]string{"TABLES": strings.Join(views, " ")}, 0, "views"))
	}
	return driver.Plan{Estimate: total, Phases: [][]driver.Stream{phase1, phase2}, Warnings: warnings}, nil
}

type pipe struct {
	src, dst driver.Endpoint
	db       string
}

// stream pipes `mysqldump <dumpArgs>` from src into `mysql` on dst.
func (p pipe) stream(dumpArgs string, vars map[string]string, size int64, lbl string) driver.Stream {
	srcPwd, srcArgs, srcEnv := conn(p.src, "SRC")
	dstPwd, dstArgs, dstEnv := conn(p.dst, "DST")
	vars["DB"] = p.db
	var restore string
	if p.dst.Local {
		// Binary logging off for the import session: the local server does not replicate
		// and writing every row twice roughly halves import speed. Needs root, so local only.
		restore = dstPwd + `mysql` + dstArgs + ` --init-command="SET SESSION sql_log_bin=0" "$DB"`
	} else {
		restore = stripDefiner + dstPwd + `mysql` + dstArgs + ` "$DB"`
	}
	return driver.Stream{
		Label:  lbl,
		Size:   size,
		Weight: 1,
		Dump: func(ctx context.Context) *exec.Cmd {
			return sh(ctx, srcPwd+`mysqldump`+srcArgs+dumpArgs, false, srcEnv, vars)
		},
		Restore: func(ctx context.Context) *exec.Cmd { return sh(ctx, restore, true, dstEnv, vars) },
	}
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

// permanent matches client errors no retry can fix: access denied (1044, 1045, 1142, 1227),
// unknown database (1049) and routine/trigger creation refused under binary logging (1419).
var permanent = regexp.MustCompile(`^ERROR (1044|1045|1049|1142|1227|1419) `)

func (*Driver) Permanent(line string) bool { return permanent.MatchString(line) }

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if i := strings.Index(s, "ERROR "); i >= 0 {
		s = s[i:]
	}
	return s
}
