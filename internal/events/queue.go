package events

import (
	"context"
	"sync/atomic"
	"time"
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
type Queue struct {
	ch       chan Event
	dropped  atomic.Uint64
	writer   Writer
	batch    int
	interval time.Duration
}

func NewQueue(size, batch int, interval time.Duration, w Writer) *Queue {
	return &Queue{ch: make(chan Event, size), writer: w, batch: batch, interval: interval}
}
func (q *Queue) Enqueue(e Event) bool {
	select {
	case q.ch <- e:
		return true
	default:
		q.dropped.Add(1)
		return false
	}
}
func (q *Queue) Dropped() uint64 { return q.dropped.Load() }
func (q *Queue) Run(ctx context.Context) {
	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()
	batch := make([]Event, 0, q.batch)
	flush := func() {
		if len(batch) > 0 {
			_ = q.writer.Write(ctx, batch)
			batch = batch[:0]
		}
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
			flush()
			return
		}
	}
}
