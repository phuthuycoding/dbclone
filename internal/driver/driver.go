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

// Field is one staging connection setting the user is asked for during onboarding.
type Field struct {
	Key         string // env var name, e.g. STG_MYSQL_HOST
	Title       string
	Description string
	Placeholder string
	Default     string
	Secret      bool
}

// Object is one table, view or collection inside a database.
type Object struct {
	Name string
	Kind string // "table", "view", "collection"
	Size int64  // bytes on staging, 0 when unknown
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
	// staging user has no privilege to read.
	Warnings []string
}

type Driver interface {
	// Name is the short engine id shown to the user and used in `engine:db`.
	Name() string
	// Fields lists the staging settings this driver reads from the environment.
	Fields() []Field
	// Configured reports whether enough settings are present to use the driver.
	Configured() bool

	// ListSource / ListLocal return user databases (system ones excluded).
	ListSource(ctx context.Context) ([]string, error)
	ListLocal(ctx context.Context) ([]string, error)

	// Objects lists the tables / collections of db on staging, for picking a subset.
	Objects(ctx context.Context, db string) ([]Object, error)
	// Plan splits db into streams using at most workers parallel streams per phase. only
	// restricts the clone to those objects; empty means the whole database, including
	// database-level objects such as routines and events.
	Plan(ctx context.Context, db string, workers int, only []string) (Plan, error)
	// Prepare runs before the first phase on the local side, e.g. CREATE DATABASE IF NOT
	// EXISTS. With fresh it first drops the local database, so nothing that exists only
	// locally survives and local ends up identical to staging.
	Prepare(ctx context.Context, db string, fresh bool) error

	// StageFromStream / StageFromLog extract the collection or table currently in flight
	// from a chunk of the dump stream or a line of tool output; "" when there is none.
	StageFromStream(chunk []byte) string
	StageFromLog(line string) string
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
			return nil, fmt.Errorf("%q not found on staging", n)
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
