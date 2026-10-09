// Package buildlog stores build stdout in an append-only file and fans it out
// to HTTP subscribers. Readers address the stream by byte offset, so a browser
// that reconnects resumes exactly where it stopped.
package buildlog

import (
	"os"
	"path/filepath"
	"sync"
)

// Line is one log line tagged with its byte offset in the log file.
type Line struct {
	Offset int64  `json:"offset"`
	Text   string `json:"line"`
}

// File is one build's log: an append-only file plus a subscription hub.
type File struct {
	mu     sync.Mutex
	path   string
	f      *os.File
	size   int64
	subs   map[int]chan Line
	nextID int
	closed bool
}

// Create truncates (or creates) the log file for a build.
func Create(dir, buildID string) (*File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, buildID+".log")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	return &File{path: p, f: f, subs: map[int]chan Line{}}, nil
}

// Path returns the log file path.
func (l *File) Path() string { return l.path }

// Size returns the number of bytes written so far.
func (l *File) Size() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.size
}

// Write appends lines, publishing each to subscribers.
func (l *File) Write(lines []Line) error {
	if len(lines) == 0 {
		return nil
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	for i := range lines {
		lines[i].Offset = l.size
		n, err := l.f.WriteString(lines[i].Text)
		if err != nil {
			l.mu.Unlock()
			return err
		}
		l.size += int64(n)
	}
	subs := make([]chan Line, 0, len(l.subs))
	for _, ch := range l.subs {
		subs = append(subs, ch)
	}
	l.mu.Unlock()

	for _, ch := range subs {
		for _, ln := range lines {
			select {
			case ch <- ln:
			default:
				// A slow subscriber drops lines; it reconnects by offset and
				// re-reads the file, so nothing is lost permanently.
			}
		}
	}
	return nil
}

// Subscribe returns a channel of new lines and an unsubscribe function.
func (l *File) Subscribe() (<-chan Line, func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := l.nextID
	l.nextID++
	ch := make(chan Line, 256)
	l.subs[id] = ch
	return ch, func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if c, ok := l.subs[id]; ok {
			delete(l.subs, id)
			close(c)
		}
	}
}

// Close flushes and closes the log file, and releases subscribers.
func (l *File) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	for id, ch := range l.subs {
		delete(l.subs, id)
		close(ch)
	}
	return l.f.Close()
}
