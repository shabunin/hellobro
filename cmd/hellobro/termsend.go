package main

// What a terminal sends its remote (the keys, the mouse, the replies to
// a program's queries) goes by way of a queue. The emulator writes it
// into a pipe, under the session's lock; a program that never reads its
// input, or a remote that stalls, would hold the pipe, so the lock, so
// the screen. Past termSendMax waiting, what's sent is let go: whatever
// isn't reading wouldn't have read it.

import (
	"io"
	"log"
	"sync"
)

const termSendMax = 1 << 20

type sendQueue struct {
	mu      sync.Mutex
	more    sync.Cond
	buf     []byte
	done    bool
	dropped int
}

// sendTo is a queue to w, written by a goroutine of its own.
func sendTo(w io.Writer) *sendQueue {
	q := &sendQueue{}
	q.more.L = &q.mu
	go func() {
		for {
			q.mu.Lock()
			for len(q.buf) == 0 && !q.done {
				q.more.Wait()
			}
			b := q.buf
			q.buf = nil
			q.mu.Unlock()
			if len(b) == 0 { // closed, all of it sent
				return
			}
			if _, err := w.Write(b); err != nil {
				q.mu.Lock()
				q.done, q.buf = true, nil
				q.mu.Unlock()
				return
			}
		}
	}()
	return q
}

// put queues p; it never waits.
func (q *sendQueue) put(p []byte) {
	q.mu.Lock()
	defer q.mu.Unlock()
	switch {
	case q.done:
	case len(q.buf)+len(p) > termSendMax:
		if q.dropped == 0 {
			log.Printf("term: the remote isn't reading: what's sent is let go")
		}
		q.dropped += len(p)
	default:
		q.buf = append(q.buf, p...)
		q.more.Signal()
	}
}

// close sends what's queued, then stops.
func (q *sendQueue) close() {
	q.mu.Lock()
	q.done = true
	q.more.Signal()
	q.mu.Unlock()
}
