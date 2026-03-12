package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/sudok8s/hookr/internal/model"
	"github.com/sudok8s/hookr/internal/queue"
	"github.com/sudok8s/hookr/internal/store"

	"github.com/google/uuid"
)

const (
	maxAttempts    = 5
	initialBackoff = 2 * time.Second
)

// Dispatcher pulls events from the queue and fans them out to all subscribers.
type Dispatcher struct {
	queue      *queue.Queue
	store      *store.Store
	httpClient *http.Client
	logger     *slog.Logger
	workers    int
}

// New creates a Dispatcher. workers controls how many goroutines run in parallel.
func New(q *queue.Queue, s *store.Store, logger *slog.Logger, workers int) *Dispatcher {
	return &Dispatcher{
		queue:   q,
		store:   s,
		logger:  logger,
		workers: workers,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start launches the worker goroutines. Call it in a separate goroutine from main.
// It runs until ctx is cancelled.
func (d *Dispatcher) Start(ctx context.Context) {
	d.logger.Info("dispatcher starting", "workers", d.workers)
	for i := 0; i < d.workers; i++ {
		go d.runWorker(ctx, i)
	}
	<-ctx.Done()
	d.logger.Info("dispatcher stopped")
}

func (d *Dispatcher) runWorker(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Pop blocks until an event arrives.
			// TODO: replace with a context-aware select once you need graceful drain.
			event := d.queue.Pop()
			d.dispatchEvent(ctx, event)
		}
	}
}

func (d *Dispatcher) dispatchEvent(ctx context.Context, event model.Event) {
	subscribers, err := d.store.ListSubscribers(ctx)
	if err != nil {
		d.logger.Error("failed to list subscribers", "event_id", event.ID, "err", err)
		return
	}

	for _, sub := range subscribers {
		go d.deliverWithRetry(ctx, event, sub)
	}
}

func (d *Dispatcher) deliverWithRetry(ctx context.Context, event model.Event, sub model.Subscriber) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		statusCode, err := d.deliver(ctx, event, sub)

		da := model.DeliveryAttempt{
			ID:           uuid.NewString(),
			EventID:      event.ID,
			SubscriberID: sub.ID,
			AttemptNum:   attempt,
			StatusCode:   statusCode,
			AttemptedAt:  time.Now().UTC(),
		}

		if err == nil && statusCode >= 200 && statusCode < 300 {
			da.Status = "success"
			_ = d.store.CreateDeliveryAttempt(ctx, da)
			d.logger.Info("delivery success",
				"event_id", event.ID,
				"subscriber_id", sub.ID,
				"attempt", attempt,
				"status_code", statusCode,
			)
			return
		}

		// Record the failure
		if err != nil {
			da.Error = err.Error()
		}

		if attempt < maxAttempts {
			da.Status = "retrying"
			_ = d.store.CreateDeliveryAttempt(ctx, da)
			backoff := backoffDuration(attempt)
			d.logger.Warn("delivery failed, retrying",
				"event_id", event.ID,
				"subscriber_id", sub.ID,
				"attempt", attempt,
				"backoff_seconds", backoff.Seconds(),
				"err", fmt.Sprintf("%v", err),
			)
			time.Sleep(backoff)
		} else {
			da.Status = "dead"
			_ = d.store.CreateDeliveryAttempt(ctx, da)
			d.logger.Error("delivery dead-lettered",
				"event_id", event.ID,
				"subscriber_id", sub.ID,
				"attempts", maxAttempts,
			)
		}
	}
}

func (d *Dispatcher) deliver(ctx context.Context, event model.Event, sub model.Subscriber) (int, error) {
	body, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hookr-Event-ID", event.ID)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// backoffDuration returns exponential backoff: 2s, 4s, 8s, 16s …
// TODO: add jitter here to avoid thundering herd when you have many subscribers.
func backoffDuration(attempt int) time.Duration {
	secs := math.Pow(2, float64(attempt))
	return time.Duration(secs) * time.Second
}
