package queue

import "github.com/sudok8s/hookr/internal/model"

// Queue is a simple in-memory event queue backed by a buffered Go channel.
// Later you can swap this for Redis or Postgres LISTEN/NOTIFY without
// changing the dispatcher at all — just fulfil the same interface.
type Queue struct {
	ch chan model.Event
}

// New creates a Queue with the given buffer size.
// 1000 is a safe starting point; tune once you add Prometheus queue-depth metrics.
func New(bufferSize int) *Queue {
	return &Queue{ch: make(chan model.Event, bufferSize)}
}

// Push adds an event to the queue. Blocks if the buffer is full.
// TODO: replace with a non-blocking select + error when you add backpressure.
func (q *Queue) Push(e model.Event) {
	q.ch <- e
}

// Pop returns the next event. Blocks until one is available.
// Dispatcher workers call this in a loop.
func (q *Queue) Pop() model.Event {
	return <-q.ch
}

// Len returns an approximate count of items waiting.
// This becomes your "queue_depth" Prometheus gauge.
func (q *Queue) Len() int {
	return len(q.ch)
}
