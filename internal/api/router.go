package api

import "net/http"

// NewRouter wires all routes to their handlers and returns a ready ServeMux.
// Using the standard library ServeMux (Go 1.22+) which supports path parameters.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /subscribers", h.CreateSubscriber)
	mux.HandleFunc("GET /subscribers", h.ListSubscribers)

	mux.HandleFunc("POST /events", h.PublishEvent)
	mux.HandleFunc("GET /events/{id}/deliveries", h.GetDeliveries)

	mux.HandleFunc("GET /health", h.Health)

	// TODO Phase 2: add a middleware chain here for:
	//   - request logging (structured, with latency)
	//   - Prometheus request duration histogram
	//   - OpenTelemetry trace injection

	return mux
}
