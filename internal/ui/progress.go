package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// Board renders one live line per job:
//
//	⠹ mongo:shop   orders ⇉4   [=====>------]  42%  1.2 GiB / ~2.9 GiB  18.4 MiB/s  01:07
type Board struct{ p *mpb.Progress }

func NewBoard() *Board {
	return &Board{p: mpb.New(mpb.WithWidth(32), mpb.WithRefreshRate(150*time.Millisecond))}
}

// Wait blocks until every bar has finished rendering.
func (b *Board) Wait() { b.p.Wait() }

type state int

const (
	waiting state = iota
	running
	done
	failed
)

var spinner = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// Bar implements clone.Reporter.
type Bar struct {
	bar *mpb.Bar
	mu  sync.Mutex
	s   stats
}

type stats struct {
	state    state
	stage    string
	streams  int
	estimate int64
	total    int64
	current  int64
	start    time.Time
	took     time.Duration
}

func (b *Board) Track(name string) *Bar {
	r := &Bar{}
	r.bar = b.p.AddBar(0,
		mpb.PrependDecorators(
			decor.Any(r.icon, decor.WC{W: 2, C: decor.DindentRight}),
			decor.Name(name, decor.WCSyncSpaceR),
			decor.Any(r.stageText, decor.WCSyncSpaceR),
		),
		mpb.AppendDecorators(
			decor.Any(r.percent, decor.WC{W: 5}),
			decor.Any(r.sizes, decor.WCSyncSpace),
			decor.Any(r.speed, decor.WCSyncSpace),
			decor.Any(r.elapsed, decor.WCSyncSpace),
		),
	)
	return r
}

func (r *Bar) Start(estimate int64) {
	r.mu.Lock()
	r.s.state, r.s.estimate, r.s.total = running, estimate, estimate
	r.mu.Unlock()
	if estimate > 0 {
		r.bar.SetTotal(estimate, false)
	}
}

// Add advances the bar. The estimate is rough, so when the stream outgrows it the total is
// pushed ahead instead of letting the bar sit at 100% while data is still flowing.
func (r *Bar) Add(n int64) {
	r.mu.Lock()
	r.s.current += n
	bump := r.s.total > 0 && r.s.current >= r.s.total
	if bump {
		r.s.total = r.s.current + r.s.current/10
	}
	total, current := r.s.total, r.s.current
	r.mu.Unlock()
	if bump {
		r.bar.SetTotal(total, false)
	}
	r.bar.SetCurrent(current) // n is negative when a failed attempt is rolled back
}

func (r *Bar) Stage(s string) {
	r.mu.Lock()
	r.s.stage = s
	r.mu.Unlock()
}

func (r *Bar) Active(streams int) {
	r.mu.Lock()
	r.s.streams = streams
	if streams > 0 && r.s.start.IsZero() {
		r.s.start = time.Now() // the clock starts with the first stream, not while queued
	}
	r.mu.Unlock()
}

func (r *Bar) Done() {
	r.finish(done)
	r.bar.SetTotal(-1, true)
}

func (r *Bar) Fail() {
	r.finish(failed)
	r.bar.Abort(false)
}

func (r *Bar) finish(s state) {
	r.mu.Lock()
	r.s.state = s
	if !r.s.start.IsZero() {
		r.s.took = time.Since(r.s.start)
	}
	r.mu.Unlock()
}

func (r *Bar) snapshot() stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.s
}

func (r *Bar) icon(decor.Statistics) string {
	switch r.snapshot().state {
	case done:
		return "✓"
	case failed:
		return "✗"
	case running:
		return string(spinner[time.Now().UnixMilli()/100%int64(len(spinner))])
	}
	return "·"
}

func (r *Bar) stageText(decor.Statistics) string {
	s := r.snapshot()
	switch s.state {
	case waiting:
		return "planning…"
	case done:
		return "done"
	case failed:
		return "failed"
	}
	if s.streams == 0 {
		return "waiting for slot"
	}
	stage := s.stage
	if stage == "" {
		stage = "dumping…"
	}
	if len([]rune(stage)) > 22 {
		stage = string([]rune(stage)[:21]) + "…"
	}
	return fmt.Sprintf("%s ⇉%d", stage, s.streams)
}

func (r *Bar) percent(decor.Statistics) string {
	s := r.snapshot()
	switch {
	case s.state == done:
		return "100%"
	case s.total <= 0:
		return "--"
	}
	return fmt.Sprintf("%d%%", s.current*100/s.total)
}

func (r *Bar) sizes(decor.Statistics) string {
	s := r.snapshot()
	if s.estimate <= 0 || s.state == done {
		return fmt.Sprintf("% .1f", decor.SizeB1024(s.current))
	}
	return fmt.Sprintf("% .1f / ~% .1f", decor.SizeB1024(s.current), decor.SizeB1024(s.estimate))
}

func (r *Bar) speed(decor.Statistics) string {
	s := r.snapshot()
	d := s.took
	if s.state == running {
		d = time.Since(s.start)
	}
	if s.start.IsZero() || d < time.Second {
		return ""
	}
	return fmt.Sprintf("% .1f/s", decor.SizeB1024(int64(float64(s.current)/d.Seconds())))
}

func (r *Bar) elapsed(decor.Statistics) string {
	s := r.snapshot()
	d := s.took
	if s.start.IsZero() {
		return ""
	}
	if s.state == running {
		d = time.Since(s.start)
	}
	d = d.Round(time.Second)
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}
