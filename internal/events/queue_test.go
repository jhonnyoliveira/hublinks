package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hublinks/hublinks/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type recordingWriter struct {
	mu      sync.Mutex
	events  []Event
	calls   int
	failFor int   // primeiras chamadas que falham
	panics  int32 // primeiras chamadas que entram em pânico
}

func (w *recordingWriter) Write(_ context.Context, e []Event) error {
	if atomic.AddInt32(&w.panics, -1) >= 0 {
		panic("boom")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls++
	if w.calls <= w.failFor {
		return errors.New("falha simulada")
	}
	w.events = append(w.events, e...)
	return nil
}
func (w *recordingWriter) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.events)
}

func newTestQueue(size, batch int, interval time.Duration, w Writer) (*Queue, *metrics.Metrics) {
	m := metrics.New()
	q := NewQueue(size, batch, interval, w, m)
	q.backoff = time.Millisecond
	return q, m
}

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condição não atingida a tempo")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestEnqueueNeverBlocksAndCountsDrops(t *testing.T) {
	q, m := newTestQueue(2, 1, time.Hour, &recordingWriter{}) // sem worker
	done := make(chan struct{})
	go func() {
		for range 100 {
			q.Enqueue(Event{})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Enqueue bloqueou")
	}
	if got := testutil.ToFloat64(m.Dropped); got != 98 || q.Dropped() != 98 {
		t.Fatalf("descartados: métrica=%v contador=%d", got, q.Dropped())
	}
	if got := testutil.ToFloat64(m.Enqueued); got != 2 {
		t.Fatalf("enfileirados: %v", got)
	}
}

func TestFlushesByBatchSize(t *testing.T) {
	w := &recordingWriter{}
	q, m := newTestQueue(10, 3, time.Hour, w)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)
	for range 3 {
		q.Enqueue(Event{})
	}
	eventually(t, func() bool { return w.count() == 3 })
	if got := testutil.ToFloat64(m.Written); got != 3 {
		t.Fatalf("gravados: %v", got)
	}
}

func TestFlushesByInterval(t *testing.T) {
	w := &recordingWriter{}
	q, _ := newTestQueue(10, 100, 10*time.Millisecond, w)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)
	q.Enqueue(Event{})
	eventually(t, func() bool { return w.count() == 1 })
}

func TestShutdownDrainsQueue(t *testing.T) {
	w := &recordingWriter{}
	q, _ := newTestQueue(10, 100, time.Hour, w)
	for range 5 {
		q.Enqueue(Event{})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // o contexto já nasce cancelado: a drenagem não pode depender dele
	q.Run(ctx)
	if w.count() != 5 {
		t.Fatalf("drenados: %d", w.count())
	}
}

func TestWorkerPanicDoesNotAffectEnqueueAndRestarts(t *testing.T) {
	w := &recordingWriter{panics: 1}
	q, _ := newTestQueue(10, 1, time.Hour, w)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)
	if !q.Enqueue(Event{}) { // este lote derruba o worker
		t.Fatal("Enqueue falhou")
	}
	time.Sleep(20 * time.Millisecond)
	if !q.Enqueue(Event{}) {
		t.Fatal("Enqueue deveria continuar funcionando")
	}
	eventually(t, func() bool { return w.count() == 1 }) // worker reiniciou
}

func TestStoppedWorkerDoesNotBlockEnqueue(t *testing.T) {
	q, _ := newTestQueue(1, 1, time.Hour, &recordingWriter{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	q.Run(ctx)
	done := make(chan struct{})
	go func() { q.Enqueue(Event{}); q.Enqueue(Event{}); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Enqueue bloqueou com o worker parado")
	}
}

func TestRetriesBatchThenCountsError(t *testing.T) {
	w := &recordingWriter{failFor: 2}
	q, m := newTestQueue(10, 1, time.Hour, w)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)
	q.Enqueue(Event{})
	eventually(t, func() bool { return w.count() == 1 })
	if got := testutil.ToFloat64(m.WriteErrors); got != 0 {
		t.Fatalf("erros após sucesso na 3ª tentativa: %v", got)
	}

	w2 := &recordingWriter{failFor: 100}
	q2, m2 := newTestQueue(10, 1, time.Hour, w2)
	go q2.Run(ctx)
	q2.Enqueue(Event{})
	eventually(t, func() bool { return testutil.ToFloat64(m2.WriteErrors) == 1 })
	w2.mu.Lock()
	defer w2.mu.Unlock()
	if w2.calls != 3 {
		t.Fatalf("tentativas: %d", w2.calls)
	}
}
