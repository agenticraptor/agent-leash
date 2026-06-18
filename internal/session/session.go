// Package session is the small on-disk channel between the PATH shims that
// intercept an agent's commands and the supervisor that watches the session.
// Each guarded command appends one JSON line to an events file; the supervisor
// tails that file to count commands and to react to denials in real time.
package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// EventsFile is the append-only log inside a session directory.
const EventsFile = "events.jsonl"

// Event is a single record appended to the session log — a guarded command
// decision, or a reported cost.
type Event struct {
	Time     time.Time `json:"time"`
	Type     string    `json:"type"` // "command" or "cost"
	Command  string    `json:"command,omitempty"`
	Allow    bool      `json:"allow"`
	Category string    `json:"category,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	Rule     string    `json:"rule,omitempty"`
	Action   string    `json:"action,omitempty"` // policy action in effect when decided
	Cost     float64   `json:"cost,omitempty"`   // reported spend in USD (for type "cost")
}

// Event types.
const (
	TypeCommand = "command"
	TypeCost    = "cost"
)

// Path returns the events file path within a session directory.
func Path(dir string) string { return filepath.Join(dir, EventsFile) }

// Append writes one event as a JSON line. Writes use O_APPEND so concurrent
// shims do not interleave (each line is small and written atomically).
func Append(dir string, e Event) error {
	if dir == "" {
		return nil // no session active; nothing to record
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(Path(dir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // session dir is created 0700 by the supervisor
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // append-only log; close error is non-fatal
	_, err = f.Write(data)
	return err
}

// ReadFrom reads events from the file starting at byte offset off, returning the
// decoded events and the new offset. Malformed trailing lines (a shim mid-write)
// are left for the next read by stopping at the last complete newline.
func ReadFrom(dir string, off int64) ([]Event, int64, error) {
	f, err := os.Open(Path(dir)) //nolint:gosec // path derived from the session dir
	if err != nil {
		if os.IsNotExist(err) {
			return nil, off, nil
		}
		return nil, off, err
	}
	defer f.Close() //nolint:errcheck // read-only

	if _, err := f.Seek(off, 0); err != nil {
		return nil, off, err
	}
	var (
		events []Event
		read   int64
	)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			break // partial or corrupt line; resume here next time
		}
		events = append(events, e)
		read += int64(len(line)) + 1 // include the newline
	}
	return events, off + read, sc.Err()
}

// ReadAll returns every event currently in the file.
func ReadAll(dir string) ([]Event, error) {
	events, _, err := ReadFrom(dir, 0)
	return events, err
}
