// Package driver is the adapter layer: one implementation per database engine. The clone
// engine and the UI only ever talk to this interface, so adding an engine (PostgreSQL, ...)
// means writing one package under driver/ and registering it in main.go.
package driver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Field is one connection setting of a profile, e.g. "mysql.host".
type Field struct {
	Key         string // key in Endpoint.Values, namespaced by engine
	Title       string
	Description string
	Placeholder string
	Default     string
	Secret      bool
}

// LocalName is the built-in endpoint for the local container.
const LocalName = "local"

// Endpoint is one side of a clone: the local container, or a remote connection made of a
// profile's settings.
type Endpoint struct {
	Name   string
	Local  bool              // the local container; credentials come from its environment
	Values map[string]string // remote settings, keyed by Field.Key
}

// Get returns a setting of a remote endpoint.
func (e Endpoint) Get(key string) string { return e.Values[key] }

// Local is what a driver needs on this machine: a running container of the engine's
// official image. Its tools run every dump and restore — towards remote endpoints too —
// and its root credentials are used for the local endpoint.
type Local struct {
	Container    string   // container name in use
	ContainerEnv string   // env var that overrides the container name
	Tools        []string // binaries that must exist inside the container
	Credentials  []string // env vars the container must carry (root credentials)
	RunExample   string   // a docker run command that creates a suitable container
}

// Object is one table, view or collection inside a database.
type Object struct {
	Name string
	Kind string // "table", "view", "collection"
	Size int64  // bytes on the source, 0 when unknown
}

// Stream is one dump → restore pipe. The engine runs it in a slot of the global pool and
// retries it on failure, so Dump/Restore build fresh commands per attempt and restoring
// the same stream twice must be safe (drop-and-recreate).
type Stream struct {
	Label   string // shown while the stream runs, e.g. a table group
	Size    int64  // estimated bytes; bigger streams are scheduled first
	Weight  int64  // pool slots held while running (a tool's own -j counts here)
	Dump    func(context.Context) *exec.Cmd
	Restore func(context.Context) *exec.Cmd
}

// Plan is how one database is cloned. Phases run in order; the streams of a phase run in
// parallel, e.g. MySQL table groups first, then views and routines that depend on them.
type Plan struct {
	Estimate int64
	Phases   [][]Stream
	// Warnings are things the clone will skip but that do not fail it, e.g. objects the
	// source user has no privilege to read.
	Warnings []string
}

type Driver interface {
	// Name is the short engine id shown to the user and used in `engine:db`.
	Name() string
	// Fields lists the connection settings a remote profile holds for this engine.
	Fields() []Field
	// Configured reports whether ep can be used with this engine. The local endpoint
	// always can.
	Configured(ep Endpoint) bool
	// Address identifies the server ep points at, to refuse cloning a server onto itself.
	Address(ep Endpoint) string
	// Local describes the local container whose tools run every dump and restore.
	Local() Local

	// Databases lists the user databases on ep (system ones excluded).
	Databases(ctx context.Context, ep Endpoint) ([]string, error)
	// Objects lists the tables / collections of db on ep, for picking a subset.
	Objects(ctx context.Context, ep Endpoint, db string) ([]Object, error)
	// Plan splits db into streams from src to dst using at most workers parallel streams
	// per phase. only restricts the clone to those objects; empty means the whole
	// database, including database-level objects such as routines and events.
	Plan(ctx context.Context, src, dst Endpoint, db string, workers int, only []string) (Plan, error)
	// Prepare runs on dst before the first phase, e.g. creating the database when it is
	// missing. With fresh it first drops the database, so dst ends up identical to src.
	Prepare(ctx context.Context, dst Endpoint, db string, fresh bool) error

	// StageFromStream / StageFromLog extract the collection or table currently in flight
	// from a chunk of the dump stream or a line of tool output; "" when there is none.
	StageFromStream(chunk []byte) string
	StageFromLog(line string) string
	// Permanent reports whether a line of tool output is an error that retrying cannot
	// fix (e.g. access denied), so the engine fails the stream at once.
	Permanent(line string) bool
}

// Select returns the set of object names to clone: every object when only is empty,
// otherwise exactly the names in only, which must all exist.
func Select(objs []Object, only []string) (map[string]bool, error) {
	set := make(map[string]bool, len(objs))
	if len(only) == 0 {
		for _, o := range objs {
			set[o.Name] = true
		}
		return set, nil
	}
	exists := make(map[string]bool, len(objs))
	for _, o := range objs {
		exists[o.Name] = true
	}
	for _, n := range only {
		if !exists[n] {
			return nil, fmt.Errorf("%q not found on the source", n)
		}
		set[n] = true
	}
	return set, nil
}

// Env returns the value of key, or def when it is unset or empty.
func Env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
