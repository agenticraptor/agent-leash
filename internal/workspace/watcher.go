package workspace

import (
	"context"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// Watcher observes filesystem changes under a workspace root and feeds newly
// changed files into a Tracker, invoking onChange whenever a distinct file is
// counted for the first time. It watches directories recursively, adding new
// directories to the watch set as they appear.
type Watcher struct {
	tracker  *Tracker
	onChange func()
	fsw      *fsnotify.Watcher
}

// NewWatcher creates a Watcher for the tracker's root. onChange may be nil.
func NewWatcher(tracker *Tracker, onChange func()) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{tracker: tracker, onChange: onChange, fsw: fsw}, nil
}

// Start adds the initial watches and processes events until ctx is canceled,
// after which the underlying watcher is closed.
func (w *Watcher) Start(ctx context.Context) error {
	w.addRecursive(w.tracker.root, false) // baseline: watch, but do not count existing files
	go func() {
		<-ctx.Done()
		_ = w.fsw.Close()
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-w.fsw.Events:
			if !ok {
				return nil
			}
			w.handle(event)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return nil
			}
			// Watch errors are non-fatal; keep observing what we can.
		}
	}
}

func (w *Watcher) handle(event fsnotify.Event) {
	// A newly created directory must be watched so its children are seen. Any
	// files already inside it (created in the race between mkdir and the watch
	// registering) are counted now so fast "mkdir + write" bursts are not lost.
	if event.Op&fsnotify.Create != 0 {
		if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
			w.addRecursive(event.Name, true)
			return
		}
	}
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
		return
	}
	w.count(event.Name)
}

func (w *Watcher) count(path string) {
	if w.tracker.Observe(path) && w.onChange != nil {
		w.onChange()
	}
}

// addRecursive adds watches to dir and every non-ignored subdirectory. When
// observeExisting is true, files already present are counted as changes (used
// for directories that appear mid-session); during the initial baseline scan it
// is false so the project's existing files are not counted.
func (w *Watcher) addRecursive(dir string, observeExisting bool) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != w.tracker.root && ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			_ = w.fsw.Add(path)
			return nil
		}
		if observeExisting {
			w.count(path)
		}
		return nil
	})
}
