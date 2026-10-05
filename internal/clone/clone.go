// Package clone is the engine. It knows nothing engine-specific: it asks each driver for a
// Plan, then runs every stream of every database through one global, weighted pool.
//
// Scheduling:
//   - The pool (Options.Parallel slots) is shared by all databases, so a slot freed by a
//     small database is picked up immediately by a big one instead of sitting idle.
//   - A stream holds Weight slots — mongodump with 4 parallel collections costs 4.
//   - Streams are dispatched biggest first across all databases (longest-processing-time
//     first), which keeps the tail of the run short.
//   - Options.Workers caps the streams of one database, so one huge database cannot take
//     every connection to staging.
package clone

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/phuthuycoding/dbclone/internal/docker"
	"github.com/phuthuycoding/dbclone/internal/driver"
)

var validName = regexp.MustCompile(`^[A-Za-z0-9_$-]+$`)

type Job struct {
	Driver driver.Driver
	DB     string
	Only   []string // tables / collections to clone; empty = the whole database
}

func (j Job) String() string { return j.Driver.Name() + ":" + j.DB }

// Label is String plus the subset, for display.
func (j Job) Label() string {
	switch len(j.Only) {
	case 0:
		return j.String()
	case 1:
		return j.String() + "." + j.Only[0]
	}
	return fmt.Sprintf("%s (%d objects)", j.String(), len(j.Only))
}

// Reporter receives progress for one job. The UI implements it; the engine only calls it.
type Reporter interface {
	Start(estimate int64)
	Add(n int64)
	Stage(s string)
	Active(streams int)
	Done()
	Fail()
}

type Options struct {
	Parallel int  // global pool size: concurrent streams across all databases
	Workers  int  // max concurrent streams for one database
	Fresh    bool // drop each target database cloned whole before cloning it
	Source   driver.Endpoint
	Target   driver.Endpoint
	LogDir   string
}

type Result struct {
	Job      Job
	Err      error
	Log      string
	Warnings []string
}

type task struct {
	i    int
	job  Job
	rep  Reporter
	plan driver.Plan
	log  *logWriter
	path string

	ctx    context.Context // cancelled on the task's first failure, killing its streams
	cancel context.CancelCauseFunc
	first  sync.WaitGroup // phase-1 streams still queued or running

	mu     sync.Mutex
	err    error
	active int
}

type unit struct {
	t *task
	s driver.Stream
}

func Run(ctx context.Context, jobs []Job, reps []Reporter, opt Options) []Result {
	opt.Parallel = max(opt.Parallel, 1)
	opt.Workers = min(max(opt.Workers, 1), opt.Parallel)
	poolSize := int64(opt.Parallel)
	tasks := make([]*task, len(jobs))
	for i, j := range jobs {
		t := &task{i: i, job: j, rep: reps[i],
			path: filepath.Join(opt.LogDir, j.Driver.Name()+"-"+j.DB+".log")}
		t.ctx, t.cancel = context.WithCancelCause(ctx)
		tasks[i] = t
	}

	// Planning and Prepare are a few cheap queries per database; run them all at once so
	// the queue below is built from tasks that are ready to stream.
	var planning errgroup.Group
	planning.SetLimit(8)
	for _, t := range tasks {
		planning.Go(func() error {
			if err := t.prepare(opt); err != nil {
				t.fail(err)
			}
			return nil
		})
	}
	planning.Wait()

	// Phase 1 of every database goes through one queue, biggest stream first across all
	// databases (global LPT). A single dispatcher acquires slots in that order, so a big
	// stream is never overtaken by small ones that merely reached Acquire first.
	var queue []unit
	for _, t := range tasks {
		if t.failed() != nil || len(t.plan.Phases) == 0 {
			continue
		}
		for _, s := range t.plan.Phases[0] {
			queue = append(queue, unit{t, s})
			t.first.Add(1)
		}
	}
	slices.SortStableFunc(queue, func(a, b unit) int { return cmp.Compare(b.s.Size, a.s.Size) })

	pool := semaphore.NewWeighted(poolSize)
	results := make([]Result, len(jobs))
	var wg sync.WaitGroup
	for _, t := range tasks {
		wg.Go(func() {
			t.first.Wait()
			// Later phases (views, routines) are small and depend on phase 1 of their own
			// database only, so each task schedules them itself.
			if t.failed() == nil && len(t.plan.Phases) > 1 {
				for _, phase := range t.plan.Phases[1:] {
					if t.runPhase(pool, poolSize, phase); t.failed() != nil {
						break
					}
				}
			}
			t.finish()
			results[t.i] = Result{Job: t.job, Err: t.failed(), Log: t.path, Warnings: t.plan.Warnings}
		})
	}

	for _, u := range queue {
		w := min(max(u.s.Weight, 1), poolSize)
		if err := pool.Acquire(u.t.ctx, w); err != nil { // task failed or Ctrl-C
			u.t.first.Done()
			continue
		}
		go func() {
			defer u.t.first.Done()
			defer pool.Release(w)
			u.t.stream(u.s)
		}()
	}
	wg.Wait()
	return results
}

func (t *task) prepare(opt Options) error {
	d, db := t.job.Driver, t.job.DB
	if !validName.MatchString(db) {
		return fmt.Errorf("database name %q has unsupported characters", db)
	}
	f, err := os.Create(t.path)
	if err != nil {
		return err
	}
	t.log = &logWriter{f: f, d: d, rep: t.rep}
	if t.plan, err = d.Plan(t.ctx, opt.Source, opt.Target, db, opt.Workers, t.job.Only); err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	for _, w := range t.plan.Warnings {
		fmt.Fprintf(t.log, "dbclone: warning: %s\n", w)
	}
	// A subset only replaces its own objects (each restore drops what it brings), so the
	// database itself is dropped only when it is cloned whole.
	if err := d.Prepare(t.ctx, opt.Target, db, opt.Fresh && len(t.job.Only) == 0); err != nil {
		return fmt.Errorf("prepare target database: %w", err)
	}
	t.rep.Start(t.plan.Estimate)
	return nil
}

// runPhase runs every stream of one phase in parallel, each in its own pool slot.
func (t *task) runPhase(pool *semaphore.Weighted, poolSize int64, phase []driver.Stream) {
	slices.SortStableFunc(phase, func(a, b driver.Stream) int { return cmp.Compare(b.Size, a.Size) })
	var wg sync.WaitGroup
	for _, s := range phase {
		wg.Go(func() {
			w := min(max(s.Weight, 1), poolSize)
			if err := pool.Acquire(t.ctx, w); err != nil {
				t.fail(err)
				return
			}
			defer pool.Release(w)
			t.stream(s)
		})
	}
	wg.Wait()
}

// attempts per stream: staging is reached over the internet and a long dump can lose its
// connection midway. Streams restore by drop-and-recreate, so a retry starts clean.
const attempts = 3

func (t *task) stream(s driver.Stream) {
	t.setActive(+1)
	defer t.setActive(-1)
	name := cmp.Or(s.Label, "stream")
	for attempt := 1; ; attempt++ {
		if s.Label != "" {
			t.rep.Stage(s.Label)
		}
		m := &meter{d: t.job.Driver, rep: t.rep}
		err := pipe(t.ctx, s, t.log, m)
		if err == nil {
			return
		}
		if t.ctx.Err() != nil || attempt == attempts || t.log.permanent.Swap(false) {
			t.fail(fmt.Errorf("%s: %w", name, err))
			return
		}
		t.rep.Add(-m.n) // the retry streams those bytes again
		wait := time.Duration(attempt*attempt) * 5 * time.Second
		fmt.Fprintf(t.log, "dbclone: %s: attempt %d/%d failed: %v — retrying in %s\n", name, attempt, attempts, err, wait)
		t.rep.Stage(fmt.Sprintf("retry %d/%d %s", attempt+1, attempts, s.Label))
		select {
		case <-time.After(wait):
		case <-t.ctx.Done():
			t.fail(fmt.Errorf("%s: %w", name, err))
			return
		}
	}
}

func (t *task) setActive(delta int) {
	t.mu.Lock()
	t.active += delta
	t.rep.Active(t.active)
	t.mu.Unlock()
}

// fail records the first error and cancels the task so its sibling streams stop.
func (t *task) fail(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err == nil {
		t.err = err
		t.cancel(err)
	}
}

func (t *task) failed() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

func (t *task) finish() {
	err := t.failed()
	if t.log != nil {
		if err != nil {
			fmt.Fprintf(t.log, "dbclone: %v\n", err)
		}
		t.log.Close()
	}
	if err != nil {
		t.rep.Fail()
	} else {
		t.rep.Done()
	}
	t.cancel(nil)
}

// pipe streams Dump's stdout into Restore's stdin through the meter.
// Both commands are bound to ctx, so a failed sibling stream or Ctrl-C kills them.
func pipe(ctx context.Context, s driver.Stream, log io.Writer, m *meter) error {
	dump, restore := s.Dump(ctx), s.Restore(ctx)
	dump.Stderr = log
	restore.Stdout, restore.Stderr = log, log
	out, err := dump.StdoutPipe()
	if err != nil {
		return err
	}
	in, err := restore.StdinPipe()
	if err != nil {
		return err
	}
	if err := restore.Start(); err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	if err := dump.Start(); err != nil {
		in.Close()
		restore.Wait()
		return fmt.Errorf("dump: %w", err)
	}
	_, copyErr := io.Copy(in, io.TeeReader(out, m))
	in.Close()
	if copyErr != nil {
		out.Close() // restore went away: unblock dump so Wait returns
	}
	dumpErr, restoreErr := dump.Wait(), restore.Wait()
	switch {
	case ctx.Err() != nil:
		return context.Cause(ctx)
	case restoreErr != nil:
		return fmt.Errorf("restore: %w", restoreErr)
	case dumpErr != nil:
		return fmt.Errorf("dump: %w", dumpErr)
	case copyErr != nil:
		return fmt.Errorf("stream: %w", copyErr)
	}
	return nil
}

// meter sees every byte of one dump stream on its way to restore.
type meter struct {
	d   driver.Driver
	rep Reporter
	n   int64
}

func (m *meter) Write(p []byte) (int, error) {
	m.n += int64(len(p))
	m.rep.Add(int64(len(p)))
	if s := m.d.StageFromStream(p); s != "" {
		m.rep.Stage(s)
	}
	return len(p), nil
}

// logWriter copies tool output to the job log and lets the driver read stages from it.
// All streams of a job write to it concurrently, hence the mutex.
type logWriter struct {
	permanent atomic.Bool // a tool reported an error no retry can fix
	mu        sync.Mutex
	f         *os.File
	d         driver.Driver
	rep       Reporter
	part      []byte
}

// Output is written a whole line at a time so every line can be redacted before it
// reaches the file.
func (l *logWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.part = append(l.part, p...)
	for {
		i := bytes.IndexByte(l.part, '\n')
		if i < 0 {
			break
		}
		line := docker.Redact(string(l.part[:i]))
		if _, err := l.f.WriteString(line + "\n"); err != nil {
			return 0, err
		}
		if s := l.d.StageFromLog(line); s != "" {
			l.rep.Stage(s)
		}
		if l.d.Permanent(line) {
			l.permanent.Store(true)
		}
		l.part = l.part[i+1:]
	}
	return len(p), nil
}

func (l *logWriter) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.part) > 0 {
		l.f.WriteString(docker.Redact(string(l.part)) + "\n")
	}
	return l.f.Close()
}
