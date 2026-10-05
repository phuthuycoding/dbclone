// Package mongo is the MongoDB driver: mongodump --archive | mongorestore --archive.
package mongo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/phuthuycoding/dbclone/internal/docker"
	"github.com/phuthuycoding/dbclone/internal/driver"
)

const (
	keyURI = "STG_MONGO_URI"

	listDBs = `db.adminCommand({listDatabases:1,nameOnly:true}).databases.map(d=>d.name).join("\n")`
	// The local container already carries its root credentials from docker-compose.
	localAuth = `-u "$MONGO_INITDB_ROOT_USERNAME" -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin`

	scriptListSource = `mongosh "$STG_MONGO_URI" --quiet --eval '` + listDBs + `'`
	scriptListLocal  = `mongosh ` + localAuth + ` --quiet --eval '` + listDBs + `'`
	// One line per collection: name, type, size in bytes (0 when collStats is not allowed).
	scriptCollections = `mongosh "$STG_MONGO_URI" --quiet --eval '
const d = db.getSiblingDB(process.env.DB);
d.getCollectionInfos().forEach(c => {
  let size = 0;
  if (c.type === "collection") { try { size = d.getCollection(c.name).stats().size } catch (e) {} }
  print(c.name + "\t" + c.type + "\t" + size);
})'`
	// No --gzip: dump and restore run side by side in the local container, so compressing
	// the pipe only burns CPU. The network leg is the mongo wire protocol either way.
	//
	// A big collection gets its own stream: it is scheduled and retried on its own, and the
	// local insert side — the slower half of the pipe — gets several insertion workers.
	scriptDumpOne    = `mongodump --uri="$STG_MONGO_URI" --db="$DB" --collection="$COLL" --archive`
	scriptRestoreOne = `mongorestore ` + localAuth + ` --archive --nsInclude="$DB.$COLL" --drop --numInsertionWorkersPerCollection=4`
	// Everything else (small collections, views) travels in one stream. $EXCLUDES is a list of
	// --excludeCollection flags for names validated against safeName, so it splits cleanly.
	scriptDumpRest    = `mongodump --uri="$STG_MONGO_URI" --db="$DB" --archive --numParallelCollections="$WORKERS" $EXCLUDES`
	scriptRestoreRest = `mongorestore ` + localAuth + ` --archive --nsInclude="$DB.*" --drop` +
		` --numParallelCollections="$WORKERS" --numInsertionWorkersPerCollection=2`

	scriptDropLocal = `mongosh ` + localAuth + ` --quiet --eval 'db.getSiblingDB(process.env.DB).dropDatabase()'`

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
		Description: "Leave empty to skip MongoDB",
		Placeholder: "mongodb://user:pass@host:27017/?authSource=admin",
		Secret:      true,
	}}
}

func (*Driver) Configured() bool { return os.Getenv(keyURI) != "" }

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

func sh(ctx context.Context, script string, stdin bool, vars map[string]string) *exec.Cmd {
	env := map[string]string{keyURI: serverURI(os.Getenv(keyURI)), "DB": "", "COLL": "", "EXCLUDES": "", "WORKERS": "1"}
	for k, v := range vars {
		env[k] = v
	}
	return docker.Sh(ctx, container(), script, stdin, env)
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

func (*Driver) ListSource(ctx context.Context) ([]string, error) {
	return docker.Lines(sh(ctx, scriptListSource, false, nil), systemDBs...)
}

func (*Driver) ListLocal(ctx context.Context) ([]string, error) {
	return docker.Lines(sh(ctx, scriptListLocal, false, nil), systemDBs...)
}

func (*Driver) Objects(ctx context.Context, db string) ([]driver.Object, error) {
	out, err := docker.Output(sh(ctx, scriptCollections, false, map[string]string{"DB": db}))
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
func (d *Driver) Plan(ctx context.Context, db string, workers int, only []string) (driver.Plan, error) {
	objs, err := d.Objects(ctx, db)
	if err != nil {
		if len(only) > 0 {
			return driver.Plan{}, fmt.Errorf("list collections: %w", err)
		}
		return driver.Plan{
			Phases:   [][]driver.Stream{{rest(db, nil, 0, workers)}},
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
			streams = append(streams, one(db, o.Name, o.Size))
			continue
		}
		restSize += o.Size
		restCount++
	}
	if restCount > 0 {
		streams = append(streams, rest(db, exclude, restSize, min(workers, restCount)))
	}
	return driver.Plan{Estimate: total, Phases: [][]driver.Stream{streams}}, nil
}

func one(db, coll string, size int64) driver.Stream {
	vars := map[string]string{"DB": db, "COLL": coll}
	return driver.Stream{
		Label:   coll,
		Size:    size,
		Weight:  1,
		Dump:    func(ctx context.Context) *exec.Cmd { return sh(ctx, scriptDumpOne, false, vars) },
		Restore: func(ctx context.Context) *exec.Cmd { return sh(ctx, scriptRestoreOne, true, vars) },
	}
}

// rest dumps the database minus the collections that have their own stream. It moves
// `workers` collections at once, so it holds that many pool slots.
func rest(db string, exclude []string, size int64, workers int) driver.Stream {
	flags := make([]string, len(exclude))
	for i, c := range exclude {
		flags[i] = "--excludeCollection=" + c
	}
	workers = max(workers, 1)
	vars := map[string]string{"DB": db, "EXCLUDES": strings.Join(flags, " "), "WORKERS": strconv.Itoa(workers)}
	return driver.Stream{
		Size:    size,
		Weight:  int64(workers),
		Dump:    func(ctx context.Context) *exec.Cmd { return sh(ctx, scriptDumpRest, false, vars) },
		Restore: func(ctx context.Context) *exec.Cmd { return sh(ctx, scriptRestoreRest, true, vars) },
	}
}

// Prepare only acts with fresh: mongorestore creates the database on first insert.
func (*Driver) Prepare(ctx context.Context, db string, fresh bool) error {
	if !fresh {
		return nil
	}
	_, err := docker.Output(sh(ctx, scriptDropLocal, false, map[string]string{"DB": db}))
	return err
}

func (*Driver) StageFromStream([]byte) string { return "" }

func (*Driver) StageFromLog(line string) string {
	if m := restoring.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}
