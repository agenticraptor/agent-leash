// Package budget tracks the countable resources an agent session consumes —
// files changed, dependencies added, commands run, wall-clock time, and spend —
// and reports the first limit that is crossed. It is safe for concurrent use:
// the supervisor's watcher, ticker, and command hooks all update one Meter.
package budget

import (
	"fmt"
	"sync"
	"time"

	"github.com/agenticraptor/agent-leash/internal/policy"
)

// Kind identifies which limit a violation belongs to.
type Kind string

// Violation kinds.
const (
	KindFiles    Kind = "files"
	KindDeps     Kind = "deps"
	KindCommands Kind = "commands"
	KindDuration Kind = "duration"
	KindCost     Kind = "cost"
)

// Violation describes a crossed limit in human-readable terms.
type Violation struct {
	Kind    Kind
	Reason  string // e.g. "changed 51 files (limit 50)"
	Limit   string // the configured limit, formatted
	Current string // the observed value, formatted
}

// Snapshot is an immutable view of the meter's current counts.
type Snapshot struct {
	FilesChanged int
	NewDeps      int
	Commands     int
	CostUSD      float64
	Elapsed      time.Duration
}

// Meter accumulates session usage and checks it against limits.
type Meter struct {
	limits policy.Limits
	now    func() time.Time

	mu       sync.Mutex
	start    time.Time
	files    int
	newDeps  int
	commands int
	cost     float64
}

// New returns a Meter for the given limits. clock may be nil, in which case
// time.Now is used; tests inject a deterministic clock.
func New(limits policy.Limits, clock func() time.Time) *Meter {
	if clock == nil {
		clock = time.Now
	}
	return &Meter{limits: limits, now: clock, start: clock()}
}

// SetFilesChanged records the current distinct count of files changed.
func (m *Meter) SetFilesChanged(n int) {
	m.mu.Lock()
	m.files = n
	m.mu.Unlock()
}

// SetNewDeps records the current count of dependencies added this session.
func (m *Meter) SetNewDeps(n int) {
	m.mu.Lock()
	m.newDeps = n
	m.mu.Unlock()
}

// AddCommand increments the count of guarded commands run.
func (m *Meter) AddCommand() {
	m.mu.Lock()
	m.commands++
	m.mu.Unlock()
}

// AddCost adds reported spend in US dollars.
func (m *Meter) AddCost(usd float64) {
	m.mu.Lock()
	m.cost += usd
	m.mu.Unlock()
}

// Snapshot returns the current counts.
func (m *Meter) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{
		FilesChanged: m.files,
		NewDeps:      m.newDeps,
		Commands:     m.commands,
		CostUSD:      m.cost,
		Elapsed:      m.now().Sub(m.start),
	}
}

// Check returns the first limit that has been crossed, or nil if the session is
// still within budget. Limits set to zero (or an empty duration) are unlimited.
func (m *Meter) Check() *Violation {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.limits

	if d := l.MaxDuration.Duration; d > 0 {
		if elapsed := m.now().Sub(m.start); elapsed > d {
			return &Violation{
				Kind:    KindDuration,
				Reason:  fmt.Sprintf("ran for %s (limit %s)", round(elapsed), d),
				Limit:   d.String(),
				Current: round(elapsed).String(),
			}
		}
	}
	if l.MaxFilesChanged > 0 && m.files > l.MaxFilesChanged {
		return &Violation{
			Kind:    KindFiles,
			Reason:  fmt.Sprintf("changed %d files (limit %d)", m.files, l.MaxFilesChanged),
			Limit:   fmt.Sprintf("%d", l.MaxFilesChanged),
			Current: fmt.Sprintf("%d", m.files),
		}
	}
	if l.MaxNewDeps > 0 && m.newDeps > l.MaxNewDeps {
		return &Violation{
			Kind:    KindDeps,
			Reason:  fmt.Sprintf("added %d dependencies (limit %d)", m.newDeps, l.MaxNewDeps),
			Limit:   fmt.Sprintf("%d", l.MaxNewDeps),
			Current: fmt.Sprintf("%d", m.newDeps),
		}
	}
	if l.MaxCommands > 0 && m.commands > l.MaxCommands {
		return &Violation{
			Kind:    KindCommands,
			Reason:  fmt.Sprintf("ran %d guarded commands (limit %d)", m.commands, l.MaxCommands),
			Limit:   fmt.Sprintf("%d", l.MaxCommands),
			Current: fmt.Sprintf("%d", m.commands),
		}
	}
	if l.MaxCostUSD > 0 && m.cost > l.MaxCostUSD {
		return &Violation{
			Kind:    KindCost,
			Reason:  fmt.Sprintf("spent $%.2f (limit $%.2f)", m.cost, l.MaxCostUSD),
			Limit:   fmt.Sprintf("$%.2f", l.MaxCostUSD),
			Current: fmt.Sprintf("$%.2f", m.cost),
		}
	}
	return nil
}

func round(d time.Duration) time.Duration {
	if d < time.Minute {
		return d.Round(time.Second)
	}
	return d.Round(time.Second)
}
