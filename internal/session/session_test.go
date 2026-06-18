package session

import (
	"testing"
	"time"
)

func TestAppendAndReadAll(t *testing.T) {
	dir := t.TempDir()
	events := []Event{
		{Type: TypeCommand, Command: "ls -la", Allow: true},
		{Type: TypeCommand, Command: "rm -rf /", Allow: false, Category: "denied-command", Reason: "matched", Action: "stop"},
	}
	for _, e := range events {
		if err := Append(dir, e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	got, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d events, want 2", len(got))
	}
	if got[1].Command != "rm -rf /" || got[1].Allow || got[1].Action != "stop" {
		t.Errorf("unexpected second event: %+v", got[1])
	}
	if got[0].Time.IsZero() {
		t.Error("Append should stamp a time")
	}
}

func TestReadFromIncremental(t *testing.T) {
	dir := t.TempDir()
	if err := Append(dir, Event{Type: TypeCommand, Command: "a", Allow: true}); err != nil {
		t.Fatal(err)
	}
	first, off, err := ReadFrom(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 {
		t.Fatalf("first read = %d, want 1", len(first))
	}
	// Nothing new yet.
	none, off2, err := ReadFrom(dir, off)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no new events, got %d", len(none))
	}
	// Append and read only the delta.
	if err := Append(dir, Event{Type: TypeCommand, Command: "b", Allow: true}); err != nil {
		t.Fatal(err)
	}
	delta, _, err := ReadFrom(dir, off2)
	if err != nil {
		t.Fatal(err)
	}
	if len(delta) != 1 || delta[0].Command != "b" {
		t.Fatalf("delta = %+v, want one event 'b'", delta)
	}
}

func TestReadMissingFile(t *testing.T) {
	dir := t.TempDir()
	got, off, err := ReadFrom(dir, 0)
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(got) != 0 || off != 0 {
		t.Errorf("expected empty read for missing file")
	}
}

func TestAppendNoDirIsNoop(t *testing.T) {
	if err := Append("", Event{Command: "x", Time: time.Now()}); err != nil {
		t.Errorf("append with empty dir should be a no-op, got %v", err)
	}
}
