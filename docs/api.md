# Pocket Booking API

Base: `<BASE_URL>/api/v1`. Auth: session cookie `pbkg_session`, a personal API token
(`Authorization: Bearer pbkg_…` or `X-API-Key`), or a Pocket Core app-link token
(`pcx_…`, audience `booking`). Errors are `{"error": "…"}`. Everything is per person:
another person's ids answer 404.

Times are RFC 3339 with an offset (`2026-11-18T10:00:00+01:00`). Money is in cents.

## Pocket Core app links

| Scope | Covers |
|---|---|
| `booking:read` | `GET /summary`, `/bookings`, `/bookings/{id}`, `/types`, `/types/{id}`, `/types/{id}/slots`, `/day/{date}`, `/search`, `/lookup`, `/invites` |
| `booking:write` | `POST /bookings` (book someone in), `POST /bookings/{id}/confirm`, `/cancel`, `/move`, `POST /invites` (a personal booking link), `POST /types` (a draft page), `PUT /types/{id}` (only pages that app made) |

`GET /auth/me` answers for any link. Everything else answers 403 to a link token.

## Objects

**Booking type** (a booking page):

```json
{"id": 3, "handle": "intro-call-k2f9", "title": "Intro call", "description": "…", "emoji": "📞",
 "status": "open", "durations": [30], "location_kind": "video", "location": "",
 "price": 0, "currency": "EUR", "confirm": "auto", "capacity": 1, "invite_only": false,
 "url": "https://booking.example/b/intro-call-k2f9",
 "link": "https://booking.example/types/3",
 "upcoming": 4, "bookings": 31}
```

`url` is the public page, `link` the owner's page.
`status`: `draft` (only the owner sees it) | `open` | `paused`.
`location_kind`: `video` (a Jitsi room made for each booking) | `link` (your own video link) | `phone` (you call them) | `phone_in` (they call you) | `place` (an address) | `ask` (they say where).

**Booking**:

```json
{"id": 41, "type_id": 3, "title": "Intro call", "emoji": "📞",
 "status": "confirmed",
 "start": "2026-11-18T10:00:00+01:00", "end": "2026-11-18T10:30:00+01:00", "minutes": 30,
 "date": "2026-11-18", "time": "10:00", "end_time": "10:30",
 "guest_tz": "Europe/Berlin",
 "name": "Ana Marić", "email": "ana@example.com", "phone": "+387 61 000 000",
 "answers": [{"id": "q1", "label": "What would you like to talk about?", "value": "A new website"}],
 "location_kind": "video", "location": "", "meeting_url": "https://meet.jit.si/pocket-…",
 "price": 5000, "currency": "EUR", "paid": false,
 "source": "page",
 "cancel_reason": "", "owner_note": "",
 "url": "https://booking.example/bookings/41",
 "created_at": "2026-11-12T08:31:00Z"}
```

`status`: `pending` (waits for you to confirm) | `confirmed` | `declined` | `cancelled` | `done` | `no_show`.
`date`/`time`/`end_time` are in the owner's time zone. `source`: `page` | `invite` | `owner` | the slug of the
app that booked through Core. `url` is the owner's page of the booking.

## Reading (booking:read)

- `GET /summary` → `{"upcoming": 6, "pending": 1, "today": 2, "this_week": 5, "types_open": 3,
  "next": <booking|null>, "today_list": [<booking>…], "pending_list": [<booking>…],
  "line": "2 bookings today, 1 waits for you to confirm", "url": "<BASE_URL>/"}`
- `GET /bookings?view=upcoming|pending|past|cancelled|all&from=YYYY-MM-DD&to=YYYY-MM-DD&q=&email=&type_id=&limit=&offset=`
  → `{"bookings": [<booking>…], "total": 12}`. `from`/`to` are days in the owner's zone (`to` included).
  `email` matches the guest's email exactly (any case).
- `GET /bookings/{id}` → `<booking>` plus `"made": [{"target","app","ref","title","url","error"}]`.
- `GET /types` → `{"types": [<type>…]}`; `GET /types/{id}` → `<type>` with its questions and limits.
- `GET /types/{id}/slots?from=YYYY-MM-DD&days=7&minutes=30&tz=Europe/Sarajevo`
  → `{"tz": "Europe/Sarajevo", "minutes": 30, "days": [{"date": "2026-11-18", "slots": [{"start": "…", "end": "…", "seats": 1}]}]}`
  (at most 62 days at a time).
- `GET /day/{YYYY-MM-DD}` → `{"date": "2026-11-18", "count": 2, "summary": "2 bookings",
  "items": [{"kind": "booking", "id": 41, "time": "10:00", "end": "10:30", "start": "…", "title": "Intro call — Ana Marić",
  "type": "Intro call", "emoji": "📞", "who": "Ana Marić", "email": "ana@example.com", "status": "confirmed",
  "location": "…", "meeting_url": "…", "url": "<owner page>"}], "made": [<same shape>]}`.
  `items` = bookings that take place that day (not cancelled or declined); `made` = bookings people made that day.
- `GET /search?q=&limit=20` → `{"items": [{"kind": "booking"|"type", "id", "title", "subtitle", "date", "url"}],
  "bookings": [<booking>…], "types": [<type>…]}`
- `GET /lookup?email=ana@example.com` → `{"email": "…", "bookings": [<booking>…] (newest first, at most 20),
  "upcoming": [<booking>…], "count": 7, "last": "2026-10-02", "next": "2026-11-18"}`
- `GET /invites` → `{"invites": [{"id","type_id","type","name","email","url","used","booking_id","expires_at","created_at"}]}`

## Writing (booking:write)

- `POST /bookings` `{"type_id": 3, "start": "<RFC 3339>", "minutes": 30, "name": "…", "email": "…", "phone": "…",
  "note": "…", "tell_guest": true, "force": false, "client_key": "crm:deal:12"}` → 201 `<booking>`.
  The slot must be free (409 `{"error": "that time is taken"}`) unless `force`. `client_key` makes it idempotent
  (the same key answers the booking made the first time).
- `POST /bookings/{id}/confirm` (pending → confirmed), `POST /bookings/{id}/cancel` `{"message": "…", "tell_guest": true}`,
  `POST /bookings/{id}/move` `{"start": "…", "minutes": 30, "tell_guest": true}` → `<booking>`.
- `POST /invites` `{"type_id": 3, "name": "Ana", "email": "ana@example.com", "note": "…", "days": 14}` (`type_id` 0 = the
  person's first open page) → 201 `{"id", "url", "type", "type_id", "expires_at"}`: a personal link that books once,
  with the name and email filled in.
- `POST /types` `{"title": "…", "minutes": 30, "description": "…", "location_kind": "video"}` → 201 `<type>` (a draft).

## Events (Pocket Core bus)

Every payload has `user_email`, `booking_id`, `type_id`, `type`, `title`, `start`, `end`, `minutes`, `status`,
`name`, `email`, `phone`, `location`, `meeting_url`, `url`, `source`.

| Topic | When |
|---|---|
| `booking.booked` | someone booked (a page, a personal link, the owner, another app); `status` is `pending` when it waits for you |
| `booking.confirmed` | you confirmed a booking that waited |
| `booking.declined` | you said no to a booking that waited |
| `booking.cancelled` | cancelled (`by`: `guest` or `owner`, `reason`) |
| `booking.moved` | moved to another time (`from` = the old start) |
| `booking.done` / `booking.no_show` | after the time: it happened / they did not come |

## The rest

Owner-only (not open to links): booking pages (`/types…` beyond the above), `/schedules…` (weekly hours + days off),
`PUT`/`DELETE /bookings/{id}`, `/bookings/{id}/(decline|done|no-show|paid|make)`, `/bookings/{id}/ai/prep`,
`/insights`, `/apps`, settings, users, tokens, backup, `export.json`, `import`.

Public pages (no account): `/p/<page handle>` (a person's pages), `/b/<handle>` (pick a time), `/b/<handle>/book`
(details), `/m/<token>` (the guest's booking: add to calendar `.ics`, move, cancel), `/i/<token>` (a personal link).
