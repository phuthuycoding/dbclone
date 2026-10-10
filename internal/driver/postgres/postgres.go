// Package postgres is the PostgreSQL driver. A whole database is one
// `pg_dump -Fc | pg_restore -j` pipe: pg_restore parallelizes the restore and its
// TOC ordering keeps constraints, indexes and triggers behind the data they need —
// postgres has no non-superuser way to defer foreign keys during load, so the
// ordering must come from the tool. A selected subset (-only) runs a plain-format
// dump through an awk filter that drops the foreign keys pointing at unselected
// objects before psql sees them. All tools run inside the local postgres container
// towards any pair of endpoints.
package postgres

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/phuthuycoding/dbclone/internal/docker"
	"github.com/phuthuycoding/dbclone/internal/driver"
)

const (
	keyHost     = "postgres.host"
	keyPort     = "postgres.port"
	keyUser     = "postgres.user"
	keyPassword = "postgres.password"

	// objectNames is a tab-separated line per object: qualified name, relkind, size.
	objectNames = `SELECT n.nspname || '.' || c.relname || E'\t' || c.relkind::text || E'\t' || pg_total_relation_size(c.oid)
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r','p','v','m')
  AND n.nspname <> 'information_schema' AND n.nspname NOT LIKE 'pg\_%'
ORDER BY 1`
	listDBs = `SELECT datname FROM pg_database WHERE NOT datistemplate AND datname <> 'postgres' ORDER BY 1`
	// Extensions are the user's job on the target; listing them up front warns early.
	listExtensions = `SELECT extname FROM pg_extension WHERE extname <> 'plpgsql' ORDER BY 1`
	// $TABLES_IN holds a quoted IN-list of the selected schema.name pairs.
	ownedSequences = `SELECT DISTINCT pg_get_serial_sequence(n.nspname || '.' || c.relname, a.attname)
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
WHERE n.nspname || '.' || c.relname IN ($TABLES_IN)`
	// Trigger functions are not selectable objects; they are emitted ahead of the
	// post-data stream so user triggers on selected tables can be created.
	triggerFunctions = `SELECT pg_get_functiondef(p.oid) || ';'
FROM pg_proc p
JOIN pg_trigger t ON t.tgfoid = p.oid AND NOT t.tgisinternal
JOIN pg_class c ON c.oid = t.tgrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname || '.' || c.relname IN ($TABLES_IN)`
	countTriggerFunctions = `SELECT count(*)
FROM pg_trigger t
JOIN pg_class c ON c.oid = t.tgrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE NOT t.tgisinternal AND n.nspname || '.' || c.relname IN ($TABLES_IN)`
	// Foreign keys from a selected table to an unselected one cannot be created —
	// their statements are filtered out and reported as warnings.
	outboundFKs = `SELECT con.conname || E'\t' || n1.nspname || '.' || c1.relname || E'\t' || n2.nspname || '.' || c2.relname
FROM pg_constraint con
JOIN pg_class c1 ON c1.oid = con.conrelid JOIN pg_namespace n1 ON n1.oid = c1.relnamespace
JOIN pg_class c2 ON c2.oid = con.confrelid JOIN pg_namespace n2 ON n2.oid = c2.relnamespace
WHERE con.contype = 'f' AND n1.nspname || '.' || c1.relname IN ($TABLES_IN)`

	// awkFilter drops statements whose REFERENCES target is in $DROP_REFS. Records
	// are ';'-terminated statements — safe because the filter only ever sees the
	// post-data stream, which holds no COPY bodies.
	awkFilter = `
  if (match($0, /FOREIGN[[:space:]]+KEY/) && match($0, /REFERENCES[[:space:]]+[A-Za-z0-9_$.]+/)) {
    ref = substr($0, RSTART, RLENGTH)
    sub(/^REFERENCES[[:space:]]+/, "", ref)
    if (index(DROP_REFS, " " ref " ") > 0) next
  }
  print`

	psqlFlags = ` -X --no-password -v ON_ERROR_STOP=1 -d"$DB"`
)

var (
	systemDBs = []string{"postgres", "template0", "template1"}
	// Qualified schema.name pairs, lowercase only: anything needing quotes is
	// rejected rather than quoted, like mysql's safeName.
	safeName = regexp.MustCompile(`^[a-z0-9_$]+(\.[a-z0-9_$]+)*$`)
	copyLine = regexp.MustCompile(`COPY ([a-z0-9_$.]+)`)
	restData = regexp.MustCompile(`processing data for table "?([a-z0-9_]+)"?(?:\.|"|$)"?([a-z0-9_]+)"?`)
	// permanent matches tool errors no retry can fix: authentication, privileges,
	// a missing database or relation, and a pg_dump/server version mismatch.
	permanent = regexp.MustCompile(`password authentication failed|no pg_hba\.conf entry|permission denied|must be owner|does not exist|server version mismatch`)
)

type Driver struct{}

func New() *Driver { return &Driver{} }

func (*Driver) Name() string { return "postgres" }

func (*Driver) Fields() []driver.Field {
	return []driver.Field{
		{Key: keyHost, Title: "Postgres host", Description: "Leave empty if this profile has no Postgres"},
		{Key: keyPort, Title: "Postgres port", Default: "5432"},
		{Key: keyUser, Title: "Postgres user"},
		{Key: keyPassword, Title: "Postgres password", Secret: true},
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

func port(ep driver.Endpoint) string { return cmp.Or(ep.Get(keyPort), "5432") }

func container() string { return driver.Env("DBCLONE_POSTGRES_CONTAINER", "postgres") }

func (*Driver) Local() driver.Local {
	return driver.Local{
		Container:    container(),
		ContainerEnv: "DBCLONE_POSTGRES_CONTAINER",
		Tools:        []string{"pg_dump", "pg_restore", "psql", "awk"},
		// POSTGRES_USER is optional in the image (defaults to postgres), so only the
		// password is a hard requirement.
		Credentials: []string{"POSTGRES_PASSWORD"},
		RunExample: "docker run -d --name " + container() + " -p 5432:5432 " +
			"-e POSTGRES_PASSWORD=<password> postgres:17",
	}
}

// conn returns the prefix that makes a client reach ep — password via PGPASSWORD so
// it never sits on argv — and the env vars it reads, named after prefix. The local
// endpoint logs in as the container's superuser.
func conn(ep driver.Endpoint, prefix string) (pwd, args string, env map[string]string) {
	if ep.Local {
		return `PGPASSWORD="$POSTGRES_PASSWORD" `, ` -U"${POSTGRES_USER:-postgres}"`, nil
	}
	return `PGPASSWORD="$` + prefix + `_PASSWORD" `,
		` -h"$` + prefix + `_HOST" -p"$` + prefix + `_PORT" -U"$` + prefix + `_USER"`,
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

// query runs one SQL statement against the QDB database on ep and returns its rows.
func query(ctx context.Context, ep driver.Endpoint, db, sql string) (string, error) {
	pwd, args, env := conn(ep, "EP")
	return docker.Output(sh(ctx, pwd+`psql`+args+` -X -A -t --no-password -d"$QDB" -c "$SQL"`, false, env, map[string]string{"QDB": db, "SQL": sql}))
}

// queryMaint runs against the maintenance database: the image's initial database
// locally, the conventional "postgres" on remote endpoints.
func queryMaint(ctx context.Context, ep driver.Endpoint, sql string) (string, error) {
	pwd, args, env := conn(ep, "EP")
	db := `"postgres"`
	if ep.Local {
		db = `"${POSTGRES_DB:-${POSTGRES_USER:-postgres}}"`
	}
	return docker.Output(sh(ctx, pwd+`psql`+args+` -X -A -t --no-password -d`+db+` -c "$SQL"`, false, env, map[string]string{"SQL": sql}))
}

func (*Driver) Databases(ctx context.Context, ep driver.Endpoint) ([]string, error) {
	out, err := queryMaint(ctx, ep, listDBs)
	if err != nil {
		return nil, err
	}
	return docker.SplitLines(out, systemDBs...), nil
}

func (*Driver) Objects(ctx context.Context, ep driver.Endpoint, db string) ([]driver.Object, error) {
	out, err := query(ctx, ep, db, objectNames)
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
		switch cols[1] {
		case "v":
			kind = "view"
		case "m":
			kind = "matview"
		}
		size, _ := strconv.ParseInt(cols[2], 10, 64)
		objs = append(objs, driver.Object{Name: cols[0], Kind: kind, Size: size})
	}
	return objs, nil
}

// Prepare creates the database only when it is missing — a remote target user may
// lack the CREATE privilege on a database that already exists.
func (*Driver) Prepare(ctx context.Context, dst driver.Endpoint, db string, fresh bool) error {
	if fresh {
		if _, err := queryMaint(ctx, dst, `DROP DATABASE IF EXISTS `+quoteIdent(db)+` WITH (FORCE)`); err != nil {
			// WITH (FORCE) needs PG13; on older servers drop the connections by hand.
			if _, err := queryMaint(ctx, dst, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = `+quoteLit(db)); err != nil {
				return err
			}
			if _, err := queryMaint(ctx, dst, `DROP DATABASE IF EXISTS `+quoteIdent(db)); err != nil {
				return err
			}
		}
	}
	n, err := queryMaint(ctx, dst, `SELECT count(*) FROM pg_database WHERE datname = `+quoteLit(db))
	if err != nil || n != "0" {
		return err
	}
	_, err = queryMaint(ctx, dst, `CREATE DATABASE `+quoteIdent(db))
	return err
}

func (d *Driver) Plan(ctx context.Context, src, dst driver.Endpoint, db string, workers int, only []string) (driver.Plan, error) {
	p := pipe{src: src, dst: dst, db: db}
	objs, err := d.Objects(ctx, src, db)
	if len(only) == 0 {
		var total int64
		var warnings []string
		if err != nil {
			warnings = append(warnings, "cannot list objects, size estimate unknown: "+err.Error())
		} else {
			for _, o := range objs {
				total += o.Size
			}
		}
		if exts, e := query(ctx, src, db, listExtensions); e != nil {
			warnings = append(warnings, "cannot list extensions: "+e.Error())
		} else if names := docker.SplitLines(exts); len(names) > 0 {
			warnings = append(warnings, "extensions must exist on the target: "+strings.Join(names, ", "))
		}
		return driver.Plan{Estimate: total, Phases: [][]driver.Stream{{p.whole(workers, total)}}, Warnings: warnings}, nil
	}
	if err != nil {
		return driver.Plan{}, fmt.Errorf("list objects: %w", err)
	}
	return d.subset(ctx, p, src, db, workers, only, objs)
}

// subset plans a -only clone in three phases: pre-data + data, then the trigger
// functions the selected tables need, then post-data (indexes, constraints,
// triggers) with foreign keys to unselected objects filtered out.
func (d *Driver) subset(ctx context.Context, p pipe, src driver.Endpoint, db string, _ int, only []string, objs []driver.Object) (driver.Plan, error) {
	selected, err := driver.Select(objs, only)
	if err != nil {
		return driver.Plan{}, err
	}
	var sel, unsel []string
	var total int64
	for _, o := range objs {
		if !selected[o.Name] {
			unsel = append(unsel, o.Name)
			continue
		}
		if !safeName.MatchString(o.Name) {
			return driver.Plan{}, fmt.Errorf("object %q has unsupported characters", o.Name)
		}
		sel = append(sel, o.Name)
		total += o.Size
	}
	in := inList(sel)
	seqs, err := query(ctx, src, db, strings.Replace(ownedSequences, "$TABLES_IN", in, 1))
	if err != nil {
		return driver.Plan{}, fmt.Errorf("list sequences: %w", err)
	}
	tables := tableArgs(sel, docker.SplitLines(seqs))
	vars := map[string]string{
		"TABLES":    tables,
		"FUNCQ":     strings.Replace(triggerFunctions, "$TABLES_IN", in, 1),
		"DROP_REFS": " " + strings.Join(unsel, " ") + " ",
	}
	var warnings []string
	fks, err := query(ctx, src, db, strings.Replace(outboundFKs, "$TABLES_IN", in, 1))
	if err != nil {
		return driver.Plan{}, fmt.Errorf("list foreign keys: %w", err)
	}
	for _, line := range strings.Split(fks, "\n") {
		cols := strings.Split(line, "\t")
		if len(cols) == 3 && !selected[cols[2]] {
			warnings = append(warnings, fmt.Sprintf("FK %s on %s -> %s skipped: target object not selected", cols[0], cols[1], cols[2]))
		}
	}
	s1 := p.stream(label(sel), total, 1,
		` -Fp --section=pre-data --section=data --clean --if-exists$TABLES --dbname="$DB"`,
		"psql", vars)
	s2 := p.stream("indexes/constraints", 0, 1,
		` -Fp --section=post-data --clean --if-exists$TABLES --dbname="$DB"`,
		"psql-filtered", vars)
	phases := [][]driver.Stream{{s1}}
	n, err := query(ctx, src, db, strings.Replace(countTriggerFunctions, "$TABLES_IN", in, 1))
	if err != nil {
		return driver.Plan{}, fmt.Errorf("count trigger functions: %w", err)
	}
	if n != "0" {
		phases = append(phases, []driver.Stream{p.funcs(vars)})
	}
	return driver.Plan{Estimate: total, Phases: append(phases, []driver.Stream{s2}), Warnings: warnings}, nil
}

type pipe struct {
	src, dst driver.Endpoint
	db       string
}

// stream builds one `pg_dump <dumpArgs>` from src into the restoreScript on dst,
// both running inside the local container.
func (p pipe) stream(lbl string, size, weight int64, dumpArgs, restoreTool string, vars map[string]string) driver.Stream {
	srcPwd, srcArgs, srcEnv := conn(p.src, "SRC")
	dstPwd, dstArgs, dstEnv := conn(p.dst, "DST")
	vars["DB"] = p.db
	var restore string
	if restoreTool == "psql-filtered" {
		restore = `awk -v RS=';' -v ORS=';' -v DROP_REFS="$DROP_REFS" '{` + awkFilter + `}' | ` +
			dstPwd + `psql` + dstArgs + psqlFlags
	} else {
		restore = dstPwd + `psql` + dstArgs + psqlFlags
	}
	return driver.Stream{
		Label:  lbl,
		Size:   size,
		Weight: weight,
		Dump: func(ctx context.Context) *exec.Cmd {
			return sh(ctx, srcPwd+`pg_dump`+srcArgs+` --no-password`+dumpArgs, false, srcEnv, vars)
		},
		Restore: func(ctx context.Context) *exec.Cmd { return sh(ctx, restore, true, dstEnv, vars) },
	}
}

// funcs emits the CREATE OR REPLACE FUNCTION statements of the selected tables'
// trigger functions — the only function dependency a post-data stream can hit.
func (p pipe) funcs(vars map[string]string) driver.Stream {
	srcPwd, srcArgs, srcEnv := conn(p.src, "SRC")
	dstPwd, dstArgs, dstEnv := conn(p.dst, "DST")
	vars["DB"] = p.db
	return driver.Stream{
		Label:  "trigger functions",
		Weight: 1,
		Dump: func(ctx context.Context) *exec.Cmd {
			return sh(ctx, srcPwd+`psql`+srcArgs+` -X -A -t --no-password -d"$DB" -c "$FUNCQ"`, false, srcEnv, vars)
		},
		Restore: func(ctx context.Context) *exec.Cmd {
			return sh(ctx, dstPwd+`psql`+dstArgs+psqlFlags, true, dstEnv, vars)
		},
	}
}

// whole is the single stream of a full-database clone. pg_restore -j cannot read
// an archive from stdin, so the dump is a serial -Fd directory written to a
// unique path inside the container; the restore first drains stdin (waits for the
// dump to finish), then restores it with -j and removes it. Dump-side -j would
// want snapshot privileges managed hosts often withhold; restore-side -j needs
// none, and it is the slower half anyway. The stream holds `workers` pool slots.
func (p pipe) whole(workers int, size int64) driver.Stream {
	workers = max(workers, 1)
	srcPwd, srcArgs, srcEnv := conn(p.src, "SRC")
	dstPwd, dstArgs, dstEnv := conn(p.dst, "DST")
	vars := map[string]string{
		"DB":      p.db,
		"WORKERS": strconv.Itoa(workers),
		"STAGE":   "/tmp/dbclone-" + strconv.FormatInt(time.Now().UnixNano(), 36),
	}
	dump := `rm -rf "$STAGE"; ` + srcPwd + `pg_dump` + srcArgs +
		` --no-password -Fd -f "$STAGE" --dbname="$DB"`
	restore := `cat >/dev/null; ` + dstPwd + `pg_restore` + dstArgs +
		` --no-password -d"$DB" -j "$WORKERS" --clean --if-exists --no-owner --no-privileges --verbose "$STAGE"; ` +
		`rc=$?; rm -rf "$STAGE"; exit $rc`
	return driver.Stream{
		Size:    size,
		Weight:  int64(workers),
		Dump:    func(ctx context.Context) *exec.Cmd { return sh(ctx, dump, false, srcEnv, vars) },
		Restore: func(ctx context.Context) *exec.Cmd { return sh(ctx, restore, true, dstEnv, vars) },
	}
}

// tableArgs renders the -t list for the selected objects plus their owned
// sequences. The args travel as one env var expanded unquoted, so names carry no
// quotes — safeName already guarantees they are single words.
func tableArgs(sel, seqs []string) string {
	var b strings.Builder
	for _, n := range sel {
		b.WriteString(" -t " + n)
	}
	for _, n := range seqs {
		if safeName.MatchString(n) {
			b.WriteString(" -t " + n)
		}
	}
	return b.String()
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func quoteLit(s string) string   { return `'` + strings.ReplaceAll(s, `'`, `''`) + `'` }

// inList renders qualified names as a SQL IN list; callers validate the names
// against safeName first.
func inList(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = quoteLit(n)
	}
	return strings.Join(q, ",")
}

func label(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%s +%d", names[0], len(names)-1)
}

// StageFromStream reads plain-format chunks (-only path): the last COPY statement
// names the table being loaded.
func (*Driver) StageFromStream(chunk []byte) string {
	if !bytes.Contains(chunk, []byte("COPY ")) {
		return ""
	}
	all := copyLine.FindAllSubmatch(chunk, -1)
	if len(all) == 0 {
		return ""
	}
	return string(all[len(all)-1][1])
}

// StageFromLog reads pg_restore --verbose output ("processing data for table").
func (*Driver) StageFromLog(line string) string {
	if m := restData.FindStringSubmatch(line); m != nil {
		return m[1] + "." + m[2]
	}
	return ""
}

func (*Driver) Permanent(line string) bool { return permanent.MatchString(line) }
