package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sudok8s/hookr/internal/model"
	"github.com/sudok8s/hookr/internal/queue"
	"github.com/sudok8s/hookr/internal/store"

	"github.com/google/uuid"
)

// Handler holds the dependencies our HTTP handlers need.
type Handler struct {
	store *store.Store
	queue *queue.Queue
}

func NewHandler(s *store.Store, q *queue.Queue) *Handler {
	return &Handler{store: s, queue: q}
}

// --- /subscribers ---

func (h *Handler) CreateSubscriber(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		http.Error(w, `{"error":"url is required"}`, http.StatusBadRequest)
		return
	}

	sub := model.Subscriber{
		ID:        uuid.NewString(),
		URL:       body.URL,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.store.CreateSubscriber(r.Context(), sub); err != nil {
		http.Error(w, `{"error":"could not create subscriber"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, sub)
}

func (h *Handler) ListSubscribers(w http.ResponseWriter, r *http.Request) {
	subs, err := h.store.ListSubscribers(r.Context())
	if err != nil {
		http.Error(w, `{"error":"could not list subscribers"}`, http.StatusInternalServerError)
		return
	}
	if subs == nil {
		subs = []model.Subscriber{} // always return an array, never null
	}
	writeJSON(w, http.StatusOK, subs)
}

// --- /events ---

func (h *Handler) PublishEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Topic   string         `json:"topic"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Topic == "" {
		http.Error(w, `{"error":"topic is required"}`, http.StatusBadRequest)
		return
	}

	event := model.Event{
		ID:        uuid.NewString(),
		Topic:     body.Topic,
		Payload:   body.Payload,
		CreatedAt: time.Now().UTC(),
	}

	if err := h.store.CreateEvent(r.Context(), event); err != nil {
		http.Error(w, `{"error":"could not store event"}`, http.StatusInternalServerError)
		return
	}

	// Push to queue — dispatcher workers will pick this up asynchronously.
	h.queue.Push(event)

	writeJSON(w, http.StatusAccepted, event)
}

// --- /events/{id}/deliveries ---

func (h *Handler) GetDeliveries(w http.ResponseWriter, r *http.Request) {
	// PathValue requires Go 1.22+
	eventID := r.PathValue("id")
	if eventID == "" {
		http.Error(w, `{"error":"event id required"}`, http.StatusBadRequest)
		return
	}

	attempts, err := h.store.ListDeliveriesForEvent(r.Context(), eventID)
	if err != nil {
		http.Error(w, `{"error":"could not fetch deliveries"}`, http.StatusInternalServerError)
		return
	}
	if attempts == nil {
		attempts = []model.DeliveryAttempt{}
	}
	writeJSON(w, http.StatusOK, attempts)
}

// --- /health ---

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	// TODO Phase 2: query store liveness, queue depth, return 503 if unhealthy
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeJSON is a tiny helper so handlers don't repeat boilerplate.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
