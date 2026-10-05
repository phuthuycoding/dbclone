// Package mongo is the MongoDB driver: mongodump --archive | mongorestore --archive, both
// run inside the local mongo container towards any pair of endpoints.
package mongo

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/phuthuycoding/dbclone/internal/docker"
	"github.com/phuthuycoding/dbclone/internal/driver"
)

const (
	keyURI = "mongo.uri"

	// The local endpoint logs in with the root credentials the container was created with.
	localAuth = `-u "$MONGO_INITDB_ROOT_USERNAME" -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin`

	listDBs = `db.adminCommand({listDatabases:1,nameOnly:true}).databases.map(d=>d.name).join("\n")`
	// One line per collection: name, type, size in bytes (0 when collStats is not allowed).
	listCollections = `
const d = db.getSiblingDB(process.env.DB);
d.getCollectionInfos().forEach(c => {
  let size = 0;
  if (c.type === "collection") { try { size = d.getCollection(c.name).stats().size } catch (e) {} }
  print(c.name + "\t" + c.type + "\t" + size);
})`
	dropDB = `db.getSiblingDB(process.env.DB).dropDatabase()`

	// Collections at least this big are cloned as their own stream.
	splitSize = 64 << 20
)

var (
	systemDBs = []string{"admin", "config", "local"}
	safeName  = regexp.MustCompile(`^[A-Za-z0-9_.$-]+$`)
	restoring = regexp.MustCompile("restoring `[^.`]+\\.([^`]+)` from")
)

type Driver struct{}

func New() *Driver { return &Driver{} }

func (*Driver) Name() string { return "mongo" }

func (*Driver) Fields() []driver.Field {
	return []driver.Field{{
		Key:         keyURI,
		Title:       "MongoDB URI",
		Description: "Leave empty if this profile has no MongoDB",
		Placeholder: "mongodb://user:pass@host:27017/?authSource=admin",
		Secret:      true,
	}}
}

func (*Driver) Configured(ep driver.Endpoint) bool { return ep.Local || ep.Get(keyURI) != "" }

// Address is the host list of the URI, credentials and options left out.
func (*Driver) Address(ep driver.Endpoint) string {
	if ep.Local {
		return driver.LocalName
	}
	_, rest, _ := strings.Cut(serverURI(ep.Get(keyURI)), "://")
	rest = rest[strings.LastIndex(rest, "@")+1:]
	host, _, _ := strings.Cut(rest, "/")
	host, _, _ = strings.Cut(host, "?")
	return strings.ToLower(host)
}

func container() string { return driver.Env("DBCLONE_MONGO_CONTAINER", "mongodb") }

func (*Driver) Local() driver.Local {
	return driver.Local{
		Container:    container(),
		ContainerEnv: "DBCLONE_MONGO_CONTAINER",
		Tools:        []string{"mongosh", "mongodump", "mongorestore"},
		Credentials:  []string{"MONGO_INITDB_ROOT_USERNAME", "MONGO_INITDB_ROOT_PASSWORD"},
		RunExample: "docker run -d --name " + container() + " -p 27017:27017 " +
			"-e MONGO_INITDB_ROOT_USERNAME=root -e MONGO_INITDB_ROOT_PASSWORD=<password> mongo:7.0",
	}
}

// conn returns how mongosh (shell) and the dump/restore tools (tool) reach ep, plus the
// env var carrying its URI. Values only travel as env vars; scripts reference "$NAME".
func conn(ep driver.Endpoint, name string) (shell, tool string, env map[string]string) {
	if ep.Local {
		return localAuth, localAuth, nil
	}
	return `"$` + name + `"`, `--uri="$` + name + `"`, map[string]string{name: serverURI(ep.Get(keyURI))}
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

// eval runs a mongosh script against ep.
func eval(ctx context.Context, ep driver.Endpoint, js string, vars map[string]string) (string, error) {
	shell, _, env := conn(ep, "URI")
	return docker.Output(sh(ctx, `mongosh `+shell+` --quiet --eval '`+js+`'`, false, env, vars))
}

// serverURI drops the database path from a connection string, because mongodump refuses
// a --db that differs from the one in the URI. The path also names the auth database when
// authSource is absent, so it is kept as authSource:
//
//	mongodb://u:p@h1,h2/shop?replicaSet=rs  →  mongodb://u:p@h1,h2/?replicaSet=rs&authSource=shop
func serverURI(uri string) string {
	scheme, rest, ok := strings.Cut(uri, "://")
	if !ok {
		return uri
	}
	// Userinfo is percent-encoded, so the last '@' ends it; the host list ends at '/' or '?'.
	hostStart := strings.LastIndex(rest, "@") + 1
	end := strings.IndexAny(rest[hostStart:], "/?")
	if end < 0 {
		return uri
	}
	authority, tail := rest[:hostStart+end], rest[hostStart+end:]
	path, query, _ := strings.Cut(tail, "?")
	db := strings.TrimPrefix(path, "/")
	if db == "" {
		return uri
	}
	if !strings.Contains("&"+query, "&authSource=") {
		if query != "" {
			query += "&"
		}
		query += "authSource=" + db
	}
	return scheme + "://" + authority + "/?" + query
}

func (*Driver) Databases(ctx context.Context, ep driver.Endpoint) ([]string, error) {
	out, err := eval(ctx, ep, listDBs, nil)
	if err != nil {
		return nil, err
	}
	return docker.SplitLines(out, systemDBs...), nil
}

func (*Driver) Objects(ctx context.Context, ep driver.Endpoint, db string) ([]driver.Object, error) {
	out, err := eval(ctx, ep, listCollections, map[string]string{"DB": db})
	if err != nil {
		return nil, err
	}
	var objs []driver.Object
	for _, line := range strings.Split(out, "\n") {
		cols := strings.Split(line, "\t")
		// system.* (e.g. system.views) is metadata mongodump handles on its own.
		if len(cols) != 3 || strings.HasPrefix(cols[0], "system.") {
			continue
		}
		size, _ := strconv.ParseInt(cols[2], 10, 64)
		objs = append(objs, driver.Object{Name: cols[0], Kind: cols[1], Size: size})
	}
	return objs, nil
}

// Plan gives every collection of at least splitSize its own stream and bundles the rest
// (small collections and views) into one more, which excludes everything that is either
// streamed on its own or not selected. If the collections cannot be listed, the whole
// database goes as a single stream.
func (d *Driver) Plan(ctx context.Context, src, dst driver.Endpoint, db string, workers int, only []string) (driver.Plan, error) {
	p := pipe{src: src, dst: dst, db: db}
	objs, err := d.Objects(ctx, src, db)
	if err != nil {
		if len(only) > 0 {
			return driver.Plan{}, fmt.Errorf("list collections: %w", err)
		}
		return driver.Plan{
			Phases:   [][]driver.Stream{{p.rest(nil, 0, workers)}},
			Warnings: []string{"cannot list collections, cloning as one stream: " + err.Error()},
		}, nil
	}
	selected, err := driver.Select(objs, only)
	if err != nil {
		return driver.Plan{}, err
	}
	var streams []driver.Stream
	var exclude []string
	var total, restSize int64
	restCount := 0
	for _, o := range objs {
		if !selected[o.Name] {
			if !safeName.MatchString(o.Name) {
				return driver.Plan{}, fmt.Errorf("cannot leave out collection %q: unsupported characters", o.Name)
			}
			exclude = append(exclude, o.Name)
			continue
		}
		total += o.Size
		if o.Kind == "collection" && o.Size >= splitSize && safeName.MatchString(o.Name) {
			exclude = append(exclude, o.Name)
			streams = append(streams, p.one(o.Name, o.Size))
			continue
		}
		restSize += o.Size
		restCount++
	}
	if restCount > 0 {
		streams = append(streams, p.rest(exclude, restSize, min(workers, restCount)))
	}
	return driver.Plan{Estimate: total, Phases: [][]driver.Stream{streams}}, nil
}

// pipe builds the dump → restore commands of one database between two endpoints.
// No --gzip: dump and restore run side by side in the local container, so compressing the
// pipe only burns CPU.
type pipe struct {
	src, dst driver.Endpoint
	db       string
}

func (p pipe) stream(label string, size, weight int64, dumpArgs, restoreArgs string, vars map[string]string) driver.Stream {
	_, srcTool, srcEnv := conn(p.src, "SRC_URI")
	_, dstTool, dstEnv := conn(p.dst, "DST_URI")
	vars["DB"] = p.db
	return driver.Stream{
		Label:  label,
		Size:   size,
		Weight: weight,
		Dump: func(ctx context.Context) *exec.Cmd {
			return sh(ctx, `mongodump `+srcTool+` --db="$DB" --archive `+dumpArgs, false, srcEnv, vars)
		},
		Restore: func(ctx context.Context) *exec.Cmd {
			return sh(ctx, `mongorestore `+dstTool+` --archive --drop `+restoreArgs, true, dstEnv, vars)
		},
	}
}

// one streams a single big collection; the restore side, the slower half of the pipe,
// gets several insertion workers.
func (p pipe) one(coll string, size int64) driver.Stream {
	return p.stream(coll, size, 1,
		`--collection="$COLL"`,
		`--nsInclude="$DB.$COLL" --numInsertionWorkersPerCollection=4`,
		map[string]string{"COLL": coll})
}

// rest dumps the database minus the collections that have their own stream or were not
// selected. It moves `workers` collections at once, so it holds that many pool slots.
// $EXCLUDES holds names validated against safeName, so its unquoted expansion splits cleanly.
func (p pipe) rest(exclude []string, size int64, workers int) driver.Stream {
	flags := make([]string, len(exclude))
	for i, c := range exclude {
		flags[i] = "--excludeCollection=" + c
	}
	workers = max(workers, 1)
	return p.stream("", size, int64(workers),
		`--numParallelCollections="$WORKERS" $EXCLUDES`,
		`--nsInclude="$DB.*" --numParallelCollections="$WORKERS" --numInsertionWorkersPerCollection=2`,
		map[string]string{"EXCLUDES": strings.Join(flags, " "), "WORKERS": strconv.Itoa(workers)})
}

// Prepare only acts with fresh: mongorestore creates the database on first insert.
func (*Driver) Prepare(ctx context.Context, dst driver.Endpoint, db string, fresh bool) error {
	if !fresh {
		return nil
	}
	_, err := eval(ctx, dst, dropDB, map[string]string{"DB": db})
	return err
}

func (*Driver) StageFromStream([]byte) string { return "" }

// Permanent matches errors no retry can fix: authentication and authorization failures.
func (*Driver) Permanent(line string) bool {
	return strings.Contains(line, "Authentication failed") || strings.Contains(line, "not authorized on")
}

func (*Driver) StageFromLog(line string) string {
	if m := restoring.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}
