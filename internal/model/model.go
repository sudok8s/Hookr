package model

import "time"

// Subscriber is an external URL that wants to receive events.
type Subscriber struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

// Event is a payload published by a caller.
type Event struct {
	ID        string         `json:"id"`
	Topic     string         `json:"topic"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

// DeliveryAttempt records one attempt to deliver an event to a subscriber.
type DeliveryAttempt struct {
	ID           string    `json:"id"`
	EventID      string    `json:"event_id"`
	SubscriberID string    `json:"subscriber_id"`
	AttemptNum   int       `json:"attempt_num"`
	StatusCode   int       `json:"status_code,omitempty"`
	Status       string    `json:"status"` // "success" | "failed" | "retrying" | "dead"
	Error        string    `json:"error,omitempty"`
	AttemptedAt  time.Time `json:"attempted_at"`
}
