package events

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/hublinks/hublinks/internal/metrics"
)

type Event struct {
	OccurredAt                 time.Time
	OrgID, TargetID, ChannelID string
	Code                       string
	VisitorID                  []byte
	Referer, UserAgent         string
	IsBot                      bool
}
type Writer interface {
	Write(context.Context, []Event) error
}

const (
	writeAttempts = 3
	drainTimeout  = 5 * time.Second
)

// Queue é a fila em memória: Enqueue nunca bloqueia e Run grava em lotes.
type Queue struct {
	ch       chan Event
	dropped  atomic.Uint64
	writer   Writer
	batch    int
	interval time.Duration
	metrics  *metrics.Metrics
	backoff  time.Duration
}

// NewQueue cria a fila. m pode ser nil (sem métricas).
func NewQueue(size, batch int, interval time.Duration, w Writer, m *metrics.Metrics) *Queue {
	if batch < 1 {
		batch = 1
	}
	return &Queue{ch: make(chan Event, size), writer: w, batch: batch, interval: interval, metrics: m, backoff: 200 * time.Millisecond}
}

// Enqueue tenta enfileirar sem bloquear; com a fila cheia descarta e contabiliza.
func (q *Queue) Enqueue(e Event) bool {
	select {
	case q.ch <- e:
		if q.metrics != nil {
			q.metrics.Enqueued.Inc()
			q.metrics.QueueLength.Set(float64(len(q.ch)))
		}
		return true
	default:
		q.dropped.Add(1)
		if q.metrics != nil {
			q.metrics.Dropped.Inc()
		}
		return false
	}
}
func (q *Queue) Len() int        { return len(q.ch) }
func (q *Queue) Dropped() uint64 { return q.dropped.Load() }

// Run processa a fila até ctx ser cancelado e então drena o que restar, com
// prazo próprio (ctx já está cancelado). Um pânico do worker o reinicia.
func (q *Queue) Run(ctx context.Context) {
	for !q.runOnce(ctx) {
		select {
		case <-ctx.Done():
		case <-time.After(q.backoff):
		}
	}
}

// runOnce devolve true quando terminou normalmente (ctx cancelado e fila drenada).
func (q *Queue) runOnce(ctx context.Context) (done bool) {
	batch := make([]Event, 0, q.batch)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("worker de eventos falhou; reiniciando", "panic", r)
			done = false
		}
	}()
	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()
	flush := func() {
		if len(batch) == 0 {
			return
		}
		q.write(ctx, batch)
		batch = batch[:0]
	}
	for {
		select {
		case e := <-q.ch:
			batch = append(batch, e)
			if len(batch) >= q.batch {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			q.drain(&batch)
			return true
		}
	}
}

func (q *Queue) drain(batch *[]Event) {
	dctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()
	for {
		select {
		case e := <-q.ch:
			*batch = append(*batch, e)
			if len(*batch) >= q.batch {
				q.write(dctx, *batch)
				*batch = (*batch)[:0]
			}
		default:
			if len(*batch) > 0 {
				q.write(dctx, *batch)
				*batch = (*batch)[:0]
			}
			return
		}
	}
}

// write tenta gravar o lote até writeAttempts vezes; depois contabiliza o erro e
// descarta o lote para não travar a fila.
func (q *Queue) write(ctx context.Context, batch []Event) {
	var err error
	for attempt := 1; attempt <= writeAttempts; attempt++ {
		if err = q.writer.Write(ctx, batch); err == nil {
			if q.metrics != nil {
				q.metrics.Written.Add(float64(len(batch)))
				q.metrics.QueueLength.Set(float64(len(q.ch)))
			}
			return
		}
		if attempt < writeAttempts {
			select {
			case <-ctx.Done():
			case <-time.After(q.backoff):
			}
		}
	}
	slog.Error("falha ao gravar lote de eventos", "events", len(batch), "error", err)
	if q.metrics != nil {
		q.metrics.WriteErrors.Inc()
	}
}
