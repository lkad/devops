# P2 — Operations: on-call rotation + runbook

**Status:** Draft
**Last updated:** 2026-06-10
**Builds on:** [service-layer.md](./service-layer.md) (P0), [observability-p1.md](./observability-p1.md) (P1)

---

## 1. Why this exists

P0 + P1 made the system fast to triage — open `/services`,
see who's unhealthy, find the trace, page the right
person. But "page the right person" is a hand-wave
unless the right person is on the page, with the
right runbook, right now. P2 closes the two remaining
operational gaps:

1. **On-call rotation** — who is on call for this
   service right now? A small DB-backed rotation
   table; the service detail page embeds the current
   shift so the on-call is the first thing the page
   shows.
2. **Runbook** — what does the on-call do when this
   service is on fire? A small per-service notebook
   with title + body, rendered in the service detail
   page.

Both are deliberately small P2 commits. Larger
features (multi-week rotation, on-call handoff notes,
markdown attachments, runbook version history) are
P3.

## 2. On-call rotation

### 2.1 Data model

`on_calls` table:

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | text | PK | uuid |
| `service_id` | text | nullable, indexed | empty = global rotation |
| `user` | text | not null | email or team name |
| `shift_start` | timestamptz | not null, indexed | inclusive |
| `shift_end` | timestamptz | not null, indexed | exclusive |
| `created_at` | timestamptz | not null | GORM auto |
| `updated_at` | timestamptz | not null | GORM auto |

### 2.2 Lookup rule

`CurrentOnCall(serviceID, at)` does a two-step
query:

1. Per-service shifts covering `at` — newest first.
2. If none, global shifts (`service_id = ''`) covering
   `at` — newest first.

This makes the per-service rotation a hard override
on the global one, with a sensible default for
services that do not have their own rotation yet.

### 2.3 Wire shape

`GET /api/v1/services/:id` embeds:

```json
{
  "id": "svc-abc",
  "name": "payments-api",
  ...,
  "oncall": {
    "id": "shift-1",
    "user": "alice@example.com",
    "shift_start": "2026-06-10T08:00:00Z",
    "shift_end":   "2026-06-10T20:00:00Z"
  },
  "oncall_absent": false
}
```

`oncall` is `null` (not omitted) when no shift is
active — the frontend can render "no one on call"
without a missing-key check.

## 3. Runbook

### 3.1 Data model

`runbook_entries` table:

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | text | PK | uuid |
| `service_id` | text | not null, indexed | FK to services |
| `title` | text | not null, ≤ 256 chars | |
| `body` | text | nullable | plain text; markdown rendering is a frontend concern |
| `created_at` | timestamptz | not null | GORM auto |
| `updated_at` | timestamptz | not null | GORM auto |

### 3.2 Lookup rule

`ListRunbook(serviceID)` returns every entry for the
service, **newest first**. The detail page shows
them in that order; the most recent runbook update
sits at the top.

### 3.3 Wire shape

`GET /api/v1/services/:id` embeds:

```json
{
  "...": "...",
  "runbook": [
    {
      "id": "rb-1",
      "title": "Restart procedure",
      "body": "1. ssh in\n2. systemctl restart payments",
      "created_at": "2026-06-10T08:00:00Z",
      "updated_at": "2026-06-10T08:00:00Z"
    }
  ]
}
```

`runbook` is always an array (empty when no
entries). Plain-text body in P2; markdown
rendering on the frontend is a one-line change in
P3.

## 4. Test plan (TDD)

| Layer | Test target | Initial red signal |
|-------|-------------|-------------------|
| `internal/servicecatalog/oncall_test.go` | per-service shift lookup; global fallback; absent-shift returns nil | `undefined: OnCall` |
| `internal/servicecatalog/runbook_test.go` | ordered by created_at DESC; scoped to service_id | `undefined: ListRunbook` |
| `internal/servicecatalog/oncall_runbook_test.go` | GET /services/:id embeds `oncall` (object or null) and `runbook` (array) | `oncall` field absent |
| `frontend/src/pages/Services.tsx` | detail page renders oncall + runbook; null and empty cases | (TS + manual) |
| Full repo | `go test ./...` and `vitest run` | (eventual green) |

The first failing test for each layer is the entry
point — once it exists and the failure is confirmed,
the production change is written to satisfy *that*
test, not the whole layer in one go.
