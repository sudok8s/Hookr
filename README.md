# Hookr — webhook delivery service

A Go REST API that accepts events and fans them out to registered subscriber URLs, with retries, exponential backoff, and a dead-letter queue.

Built as a DevOps portfolio project with observability hooks prepared for Phase 2 (Prometheus, OpenTelemetry, ELK).

## Requirements

- Go 1.22+

## Run

```bash
go mod tidy
go run ./cmd/server
```

The server starts on `:8080` by default.

## Environment variables

| Variable     | Default    | Description                  |
|--------------|------------|------------------------------|
| `HOOKR_ADDR` | `:8080`    | Listen address               |
| `HOOKR_DB`   | `hookr.db` | SQLite file path             |

## API

### Register a subscriber
```bash
curl -X POST http://localhost:8080/subscribers \
  -H "Content-Type: application/json" \
  -d '{"url": "https://webhook.site/your-id"}'
```

### List subscribers
```bash
curl http://localhost:8080/subscribers
```

### Publish an event
```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"topic": "user.signup", "payload": {"user_id": "123", "email": "hi@example.com"}}'
```

### Check delivery status
```bash
curl http://localhost:8080/events/{event-id}/deliveries
```

### Health check
```bash
curl http://localhost:8080/health
```

## Architecture

```
Caller → HTTP API → Event queue (Go channel)
                          ↓
                   Dispatcher workers (goroutine pool)
                          ↓
                   Subscriber endpoints  ← retry with exp. backoff
                          ↓ (on failure)
                   Dead-letter queue (status = "dead" in DB)
```

## Phase 2 — Observability (TODO)

- [ ] Prometheus metrics: delivery success/fail counters, latency histograms, queue depth gauge
- [ ] OpenTelemetry traces: one span per event publish, child spans per delivery attempt
- [ ] Structured logging already in place — ship logs to ELK with Filebeat
- [ ] Grafana dashboard: delivery rate, retry rate, DLQ size alert