package events

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recordingWriter struct {
	mu     sync.Mutex
	events []Event
}

func (w *recordingWriter) Write(_ context.Context, e []Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, e...)
	return nil
}
func TestQueueDropsWithoutBlocking(t *testing.T) {
	w := &recordingWriter{}
	q := NewQueue(1, 1, time.Hour, w)
	if !q.Enqueue(Event{}) || q.Enqueue(Event{}) || q.Dropped() != 1 {
		t.Fatal("fila deveria descartar quando cheia")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go q.Run(ctx)
	time.Sleep(10 * time.Millisecond)
	cancel()
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.events) != 1 {
		t.Fatalf("eventos gravados: %d", len(w.events))
	}
}
