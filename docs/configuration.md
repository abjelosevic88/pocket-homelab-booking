# Configuration

Environment variables; every one has a default. Personal settings (time zone, your page, notifications, connections, AI) live in the UI per person — the `OPENAI_*`/`NOTIFY_*` values below are copied once into the first admin's settings. Everything about a booking page (length, place, hours, questions, limits, what happens by itself) is set in the app.

| Variable | Default | What it does |
|---|---|---|
| `BASE_URL` | – | Where the app is (links in pushes and events, the OIDC callback); booking pages are at `BASE_URL/b/<address>`, a person's page at `/p/<name>`, a guest's own booking at `/m/<token>` |
| `PUBLIC_URL` | – | A second hostname that serves **only** the public pages (`/b/…`, `/p/…`, `/m/…`, `/i/…` and their stylesheet) — no sign-in page, no API. Use it to put the booking pages on the internet while the app stays private; links in emails then use it |
| `HOST` / `PORT` | `0.0.0.0` / `8080` | Where it listens |
| `DATA_DIR` | `./data` | The database and the Pocket Core token |
| `DB_PATH` | `$DATA_DIR/pocket-booking.db` | The SQLite file |
| `AUTH_MODE` | `local` | `local` (passwords), `proxy` (a header from your reverse proxy), `none` (one user, for trying it out) |
| `SECURE_COOKIES` | `false` | Set `true` behind HTTPS |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` / `ADMIN_NAME` | – | The first account, made on an empty database |
| `OIDC_ISSUER` / `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | – | Sign in with an OpenID Connect provider (not needed with Pocket Core) |
| `NOTIFY_URL` / `NOTIFY_FORMAT` | – / `apprise` | Pushes: an Apprise URL, an ntfy topic or a JSON webhook |
| `NOTIFY_HOUR` | `8` | Default hour of the morning note |
| `OPENAI_API_KEY` / `OPENAI_MODEL` / `OPENAI_BASE_URL` | – | AI without Pocket Core |
| `CORE_URL` / `CORE_REGISTRATION_KEY` / `CORE_APP_URL` | – | Pocket Core |
| `TRUST_PROXY` | `false` | Take the client address from `X-Forwarded-For` (rate limits per address) |
| `METRICS_ENABLED` | `false` | Prometheus `/metrics` |
| `MAX_BODY_BYTES` | `67108864` | Largest import |
| `DEMO_DATA` | `false` | Seed months of booking pages and bookings into the owner account |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `text` | |

Connections to Pocket Calendar, Mail, Contacts, CRM, Invoicing, Money, Tasks, Notes, Places and Time are made per person under Settings → Connected apps: through Pocket Core (nothing to paste) or directly with the app's URL and an API token.

## Putting the booking pages on the internet

The app and the booking pages can live on different hostnames. A reverse proxy sends a public hostname (say `book.example.com`) to the same container and you set `PUBLIC_URL=https://book.example.com`: on that hostname only `/b/…`, `/p/…`, `/m/…`, `/i/…`, the page stylesheet and the icon answer; everything else is a 404. Set `TRUST_PROXY=true` so the per-address limit sees the real visitor.
