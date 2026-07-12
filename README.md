# cron

A Redis-backed HTTP job runner in Go: submit an outbound API call over REST, and have it executed asynchronously — immediately or at a scheduled future time — with automatic retries.

## Overview

Applications frequently need to call a third-party API *later* (send a reminder at 09:00, hit a billing webhook at the end of a trial) or *reliably* (retry until the endpoint responds). Doing this inline couples the caller's request latency and success to a remote system it does not control.

This service takes that work off the request path. A client `POST`s a task describing an HTTP request — URL, method, headers, body, execution time, retry budget — and the service enqueues it onto Redis via [Asynq](https://github.com/hibiken/asynq). A pool of background workers dequeues, performs the call, retries on failure, and retains the response body for later inspection.

## Features

- **HTTP task submission** — enqueue an arbitrary outbound API call through a single REST endpoint.
- **Deferred execution** — supply an RFC 3339 `exe_at` timestamp and the task is enqueued with `asynq.ProcessAt` to fire at that moment. Omit it and the task runs immediately. Timestamps in the past are rejected with `400`.
- **Configurable retries** — per-task `retry` count (default `5`) with a fixed 5-second backoff between attempts.
- **Response-code assertion** — set `response_code` and a task is only considered successful if the remote endpoint returns exactly that status; anything else fails the task and triggers a retry.
- **Result retention** — the remote response body is written to the task's `ResultWriter` and retained for 24 hours, so a completed task's output can be read back.
- **Prioritised queues** — workers run `critical` / `default` / `low` queues at a 3:2:1 weighting with a concurrency of 10.
- **Context-aware cancellation** — the HTTP call runs in a goroutine raced against the task context, so worker shutdown or task timeout cancels cleanly. Outbound calls carry a 10-second client timeout.
- **Queue observability** — `docker-compose` ships [Asynqmon](https://github.com/hibiken/asynqmon), a web UI for inspecting queues, retries, and dead tasks.
- **Live reload** — [Air](https://github.com/cosmtrek/air) is configured for hot-rebuild during development.

## Tech Stack

| Layer | Technology |
| --- | --- |
| Language | Go 1.20 |
| HTTP framework | [Gin](https://github.com/gin-gonic/gin) v1.9.1 |
| Task queue | [Asynq](https://github.com/hibiken/asynq) v0.24.1 |
| Broker / store | Redis 7 |
| Scheduling | `robfig/cron` v3 (via Asynq's scheduler) |
| Config | [godotenv](https://github.com/joho/godotenv) |
| Monitoring | Asynqmon |
| Dev tooling | Air (live reload) |
| Packaging | Docker (multi-stage), Docker Compose |

## Architecture

The service runs the API and the worker pool in a single process. The API layer is a *producer* that writes tasks to Redis; the worker layer is a *consumer* that reads them back and executes them. Redis is the durable boundary between the two, so a task survives even if execution is deferred by hours.

```
POST /api-caller
      │
      ▼
┌─────────────┐   enqueue    ┌─────────┐   dequeue   ┌─────────────────┐
│   client    │─────────────▶│  Redis  │────────────▶│     worker      │
│ (Gin route) │  ProcessAt   │ (asynq) │  by queue   │  (asynq server) │
└─────────────┘              └─────────┘             └────────┬────────┘
                                                              │ handles "instant"
                                                              ▼
                                                     ┌─────────────────┐
                                                     │ task.ApiRequest │
                                                     │ → outbound HTTP │
                                                     └─────────────────┘
```

```
.
├── main.go              # entrypoint: loads env, wires Gin routes, starts the worker
├── client/              # producer — Gin handler that validates payloads and enqueues tasks
│   └── client.go
├── worker/              # consumer — asynq server, queue priorities, retry policy, mux
│   └── worker.go
├── task/                # task payload type + the ApiRequest handler that performs the call
│   └── task.go
├── schedular/           # asynq Scheduler for recurring (cron-spec) tasks — see note below
│   └── maing.go
├── Dockerfile           # multi-stage build → alpine runtime image
├── docker-compose.yaml  # Redis + Asynqmon
└── .air.toml            # live-reload config
```

> **Note on `schedular/`:** this package wraps `asynq.NewScheduler` (pinned to the `Asia/Dhaka` timezone) to support *recurring* cron-spec tasks such as `@every 24h`. It is currently scaffolding — it is not yet instantiated from `main.go` and has no registered entries. All scheduling in the live code path is one-shot, via the `exe_at` field.

## Getting Started

### Prerequisites

- Go 1.20+
- Docker & Docker Compose (for Redis and Asynqmon)

### Install

```bash
git clone https://github.com/shariarfaisal/cron.git
cd cron
go mod download
```

### Configure

The service reads a `.env` file from the working directory at startup and **exits if it is missing**.

| Variable | Description | Example |
| --- | --- | --- |
| `PORT` | Address Gin binds to. Passed straight to `r.Run()`, so it accepts a full `host:port`. | `localhost:8000` |
| `REDIS_URI` | Redis address used by both the enqueuing client and the worker. | `localhost:6379` |

```bash
cat > .env <<'EOF'
PORT=localhost:8000
REDIS_URI=localhost:6379
EOF
```

### Run

Start the backing services (Redis on `:6379`, Asynqmon on `:8080`):

```bash
docker compose up -d
```

Then run the app:

```bash
go run main.go
```

Or with live reload:

```bash
air
```

The API is now on `http://localhost:8000` and the queue dashboard on `http://localhost:8080`.

### Docker

A multi-stage `Dockerfile` builds the binary onto `alpine:3.14` and exposes port `8000`:

```bash
docker build -t cron .
docker run --env-file .env -p 8000:8000 cron
```

## Usage

### Fire a request immediately

Omit `exe_at` and the task is enqueued for immediate processing.

```bash
curl -X POST http://localhost:8000/api-caller \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "notify-slack",
    "url": "https://hooks.slack.com/services/XXX/YYY/ZZZ",
    "method": "POST",
    "headers": { "Content-Type": "application/json" },
    "body": "{\"text\":\"deploy finished\"}"
  }'
```

### Schedule a request for later

```bash
curl -X POST http://localhost:8000/api-caller \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "trial-expiry-webhook",
    "url": "https://api.example.com/v1/billing/expire",
    "method": "POST",
    "headers": { "Authorization": "Bearer sk_live_..." },
    "body": "{\"user_id\":42}",
    "exe_at": "2026-08-01T09:00:00Z",
    "retry": 10,
    "response_code": 200
  }'
```

Response:

```json
{
  "message": "success",
  "status": 200,
  "info": { "id": "...", "queue": "default", "state": "scheduled" }
}
```

Because `response_code` is `200`, the task fails and is retried (up to 10 times, 5s apart) unless the endpoint returns exactly `200`.

### Task payload

Defined in `task.ApiRequestPayload`:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | `string` | yes | Human-readable label for the task. |
| `url` | `string` | yes | Target endpoint of the outbound request. |
| `method` | `string` | yes | HTTP verb; upper-cased before dispatch. |
| `body` | `string` | no | Raw request body sent to the target. |
| `headers` | `map[string]string` | no | Headers applied to the outbound request. |
| `exe_at` | `string` | no | RFC 3339 execution time. Must be in the future. Defaults to now. |
| `retry` | `int` | no | Max retry attempts. Defaults to `5`. |
| `response_code` | `int` | no | If set, the task only succeeds when the response status matches exactly. |

## API Reference

| Method | Endpoint | Description | Success | Errors |
| --- | --- | --- | --- | --- |
| `GET` | `/ping` | Health check. Returns `{"message":"pong"}`. | `200` | — |
| `POST` | `/add` | Sample endpoint; sums `A` and `B` from the JSON body. | `200` | `400` invalid body |
| `POST` | `/api-caller` | Enqueues an outbound API call. Body = task payload above. | `200` with `info` | `400` invalid body, `exe_at` in the past, or enqueue failure |

## Testing

There is currently no automated test suite in this repository. The service is verified manually against the running stack:

```bash
# 1. health check
curl http://localhost:8000/ping

# 2. enqueue a task against a request bin
curl -X POST http://localhost:8000/api-caller \
  -H 'Content-Type: application/json' \
  -d '{"name":"smoke","url":"https://httpbin.org/post","method":"POST","response_code":200}'

# 3. confirm processing, retries, and results in the queue dashboard
open http://localhost:8080
```

Asynqmon at `http://localhost:8080` is the primary verification surface: it shows tasks moving through *scheduled → pending → active → completed*, along with retry counts, failure reasons, and retained results.
