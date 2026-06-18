package budget

import (
	"testing"
	"time"

	"github.com/agenticraptor/agent-leash/internal/policy"
)

func fixedClock(t *time.Time) func() time.Time {
	return func() time.Time { return *t }
}

func TestWithinBudget(t *testing.T) {
	now := time.Now()
	m := New(policy.Limits{MaxFilesChanged: 10, MaxNewDeps: 2}, fixedClock(&now))
	m.SetFilesChanged(5)
	m.SetNewDeps(1)
	if v := m.Check(); v != nil {
		t.Fatalf("expected within budget, got %+v", v)
	}
}

func TestFilesViolation(t *testing.T) {
	now := time.Now()
	m := New(policy.Limits{MaxFilesChanged: 3}, fixedClock(&now))
	m.SetFilesChanged(4)
	v := m.Check()
	if v == nil || v.Kind != KindFiles {
		t.Fatalf("expected files violation, got %+v", v)
	}
}

func TestDepsViolation(t *testing.T) {
	now := time.Now()
	m := New(policy.Limits{MaxNewDeps: 1}, fixedClock(&now))
	m.SetNewDeps(2)
	if v := m.Check(); v == nil || v.Kind != KindDeps {
		t.Fatalf("expected deps violation, got %+v", v)
	}
}

func TestCommandsViolation(t *testing.T) {
	now := time.Now()
	m := New(policy.Limits{MaxCommands: 2}, fixedClock(&now))
	m.AddCommand()
	m.AddCommand()
	if v := m.Check(); v != nil {
		t.Fatalf("two commands should be within limit 2, got %+v", v)
	}
	m.AddCommand()
	if v := m.Check(); v == nil || v.Kind != KindCommands {
		t.Fatalf("expected commands violation, got %+v", v)
	}
}

func TestDurationViolation(t *testing.T) {
	now := time.Now()
	clock := now
	m := New(policy.Limits{MaxDuration: policy.Duration{Duration: time.Minute}}, fixedClock(&clock))
	clock = now.Add(30 * time.Second)
	if v := m.Check(); v != nil {
		t.Fatalf("30s should be within 1m, got %+v", v)
	}
	clock = now.Add(2 * time.Minute)
	if v := m.Check(); v == nil || v.Kind != KindDuration {
		t.Fatalf("expected duration violation, got %+v", v)
	}
}

func TestCostViolation(t *testing.T) {
	now := time.Now()
	m := New(policy.Limits{MaxCostUSD: 1.0}, fixedClock(&now))
	m.AddCost(0.50)
	m.AddCost(0.40)
	if v := m.Check(); v != nil {
		t.Fatalf("$0.90 should be within $1.00, got %+v", v)
	}
	m.AddCost(0.20)
	if v := m.Check(); v == nil || v.Kind != KindCost {
		t.Fatalf("expected cost violation, got %+v", v)
	}
}

func TestZeroLimitsAreUnlimited(t *testing.T) {
	now := time.Now()
	m := New(policy.Limits{}, fixedClock(&now))
	m.SetFilesChanged(10_000)
	m.SetNewDeps(999)
	m.AddCommand()
	m.AddCost(1_000_000)
	if v := m.Check(); v != nil {
		t.Fatalf("zero limits should never trip, got %+v", v)
	}
}

func TestDurationTakesPrecedence(t *testing.T) {
	now := time.Now()
	clock := now
	m := New(policy.Limits{
		MaxDuration:     policy.Duration{Duration: time.Second},
		MaxFilesChanged: 1,
	}, fixedClock(&clock))
	m.SetFilesChanged(100)
	clock = now.Add(time.Hour)
	if v := m.Check(); v == nil || v.Kind != KindDuration {
		t.Fatalf("duration should be reported first, got %+v", v)
	}
}

func TestSnapshot(t *testing.T) {
	now := time.Now()
	clock := now
	m := New(policy.Limits{}, fixedClock(&clock))
	m.SetFilesChanged(3)
	m.SetNewDeps(2)
	m.AddCommand()
	m.AddCost(1.25)
	clock = now.Add(10 * time.Second)
	s := m.Snapshot()
	if s.FilesChanged != 3 || s.NewDeps != 2 || s.Commands != 1 || s.CostUSD != 1.25 {
		t.Errorf("unexpected snapshot: %+v", s)
	}
	if s.Elapsed != 10*time.Second {
		t.Errorf("elapsed = %v, want 10s", s.Elapsed)
	}
}
