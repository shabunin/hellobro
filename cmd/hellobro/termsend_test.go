package main

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

// stuckWriter is a remote that doesn't read till it's let.
type stuckWriter struct {
	mu   sync.Mutex
	go_  chan struct{}
	got  bytes.Buffer
	done chan struct{}
}

func (w *stuckWriter) Write(p []byte) (int, error) {
	<-w.go_
	w.mu.Lock()
	defer w.mu.Unlock()
	w.got.Write(p)
	return len(p), nil
}

func TestSendQueue(t *testing.T) {
	w := &stuckWriter{go_: make(chan struct{})}
	q := sendTo(w)
	put := make(chan struct{})
	go func() {
		for range 3 * termSendMax / 1024 { // more than it keeps
			q.put(make([]byte, 1024))
		}
		q.put([]byte("end"))
		close(put)
	}()
	select {
	case <-put:
	case <-time.After(5 * time.Second):
		t.Fatal("put waited on a remote that isn't reading")
	}
	close(w.go_)
	q.close()
	deadline := time.Now().Add(5 * time.Second)
	n := 0
	for n < termSendMax && time.Now().Before(deadline) { // what it kept, at least
		time.Sleep(10 * time.Millisecond)
		w.mu.Lock()
		n = w.got.Len()
		w.mu.Unlock()
	}
	if n < termSendMax || n > 2*termSendMax+1024 { // what the writer took before it stuck, and what it kept
		t.Fatalf("sent %d bytes", n)
	}
}
