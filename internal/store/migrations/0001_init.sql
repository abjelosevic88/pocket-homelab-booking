CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  role          TEXT NOT NULL DEFAULT 'member',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  last_used_at TEXT,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Box-wide settings (title, setup flag); everything else is personal.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  value   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, key)
);

CREATE TABLE cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL
);


-- ---------- Pocket Booking: everything below is per person ----------

-- Weekly hours people can book in, with days that differ (days off,
-- longer days). hours: {"1": [["09:00","17:00"]], …} by weekday, 0 = Sunday.
CREATE TABLE schedules (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       TEXT NOT NULL DEFAULT '',
  tz         TEXT NOT NULL DEFAULT '',
  hours      TEXT NOT NULL DEFAULT '{}',
  is_default INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX schedules_user ON schedules(user_id, name);

-- A day that is not like its weekday: hours [] = closed that day.
CREATE TABLE overrides (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  schedule_id INTEGER NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
  day         TEXT NOT NULL,
  hours       TEXT NOT NULL DEFAULT '[]',
  note        TEXT NOT NULL DEFAULT '',
  UNIQUE (schedule_id, day)
);
CREATE INDEX overrides_user ON overrides(user_id, day);

-- A booking page: what people book (an intro call, a viewing, a haircut),
-- how long, where, when (a schedule) and what happens after. handle is
-- box-wide unique: people open /b/<handle>.
CREATE TABLE types (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  handle        TEXT NOT NULL UNIQUE,
  title         TEXT NOT NULL DEFAULT '',
  description   TEXT NOT NULL DEFAULT '',
  emoji         TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'draft',          -- draft | open | paused
  durations     TEXT NOT NULL DEFAULT '[30]',           -- minutes; more than one = the guest picks
  location_kind TEXT NOT NULL DEFAULT 'video',          -- video | link | phone | phone_in | place | ask
  location      TEXT NOT NULL DEFAULT '',               -- the link, number or address
  place_id      TEXT NOT NULL DEFAULT '',               -- a Pocket Places place it is at
  schedule_id   INTEGER REFERENCES schedules(id) ON DELETE SET NULL,
  notice_min    INTEGER NOT NULL DEFAULT 240,           -- not sooner than this many minutes from now
  days_ahead    INTEGER NOT NULL DEFAULT 60,            -- not further than this many days
  buffer_before INTEGER NOT NULL DEFAULT 0,
  buffer_after  INTEGER NOT NULL DEFAULT 0,
  step_min      INTEGER NOT NULL DEFAULT 0,             -- times start every … minutes (0 = the length)
  max_per_day   INTEGER NOT NULL DEFAULT 0,             -- 0 = no limit
  capacity      INTEGER NOT NULL DEFAULT 1,             -- people per time (a class, a tour)
  confirm       TEXT NOT NULL DEFAULT 'auto',           -- auto | manual
  ask_phone     TEXT NOT NULL DEFAULT 'no',             -- no | optional | required
  questions     TEXT NOT NULL DEFAULT '[]',             -- [{id,type,label,help,required,options}]
  price         INTEGER NOT NULL DEFAULT 0,             -- cents
  currency      TEXT NOT NULL DEFAULT 'EUR',
  pay_note      TEXT NOT NULL DEFAULT '',               -- how to pay (shown on the page and in the email)
  reminder_h    INTEGER NOT NULL DEFAULT 24,            -- email the guest this many hours before (0 = no)
  language      TEXT NOT NULL DEFAULT 'en',
  accent        TEXT NOT NULL DEFAULT '',
  invite_only   INTEGER NOT NULL DEFAULT 0,             -- only with a personal link
  after         TEXT NOT NULL DEFAULT '{}',             -- made by itself: {"event","contact","deal","task","note","invoice"}
  thanks        TEXT NOT NULL DEFAULT '',
  source        TEXT NOT NULL DEFAULT '',               -- the app that made it through a Core link
  position      INTEGER NOT NULL DEFAULT 0,
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX types_user ON types(user_id, status, position);

-- A personal link: books once, with the name and email filled in.
CREATE TABLE invites (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type_id    INTEGER NOT NULL REFERENCES types(id) ON DELETE CASCADE,
  token      TEXT NOT NULL UNIQUE,
  name       TEXT NOT NULL DEFAULT '',
  email      TEXT NOT NULL DEFAULT '',
  note       TEXT NOT NULL DEFAULT '',
  expires_at TEXT NOT NULL DEFAULT '',
  booking_id INTEGER,
  source     TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX invites_user ON invites(user_id, created_at);

-- One booking. user_id is the page owner's; start_at/end_at are UTC.
CREATE TABLE bookings (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type_id       INTEGER REFERENCES types(id) ON DELETE SET NULL,
  title         TEXT NOT NULL DEFAULT '',
  emoji         TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'confirmed',      -- pending | confirmed | declined | cancelled | done | no_show
  start_at      TEXT NOT NULL,
  end_at        TEXT NOT NULL,
  minutes       INTEGER NOT NULL DEFAULT 30,
  guest_tz      TEXT NOT NULL DEFAULT '',
  name          TEXT NOT NULL DEFAULT '',
  email         TEXT NOT NULL DEFAULT '',
  phone         TEXT NOT NULL DEFAULT '',
  answers       TEXT NOT NULL DEFAULT '[]',             -- [{id,label,value}]
  location_kind TEXT NOT NULL DEFAULT '',
  location      TEXT NOT NULL DEFAULT '',
  meeting_url   TEXT NOT NULL DEFAULT '',
  price         INTEGER NOT NULL DEFAULT 0,
  currency      TEXT NOT NULL DEFAULT '',
  paid          INTEGER NOT NULL DEFAULT 0,
  paid_at       TEXT NOT NULL DEFAULT '',
  token         TEXT NOT NULL UNIQUE,                   -- the guest's own page /m/<token>
  source        TEXT NOT NULL DEFAULT 'page',           -- page | invite | owner | import | <app slug>
  invite_id     INTEGER,
  cancel_reason TEXT NOT NULL DEFAULT '',
  cancelled_by  TEXT NOT NULL DEFAULT '',               -- guest | owner
  moved_from    TEXT NOT NULL DEFAULT '',               -- the start before the last move
  owner_note    TEXT NOT NULL DEFAULT '',
  language      TEXT NOT NULL DEFAULT 'en',
  reminded      INTEGER NOT NULL DEFAULT 0,
  ip_hash       TEXT NOT NULL DEFAULT '',
  client_key    TEXT NOT NULL DEFAULT '',               -- the same booking is never made twice (imports, apps)
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX bookings_user ON bookings(user_id, start_at);
CREATE INDEX bookings_type ON bookings(type_id, start_at);
CREATE INDEX bookings_email ON bookings(user_id, email);
CREATE INDEX bookings_created ON bookings(user_id, created_at);
CREATE INDEX bookings_ip ON bookings(ip_hash, created_at);
CREATE UNIQUE INDEX bookings_client ON bookings(user_id, client_key) WHERE client_key <> '';

-- What was made from a booking in the other Pocket apps (the calendar
-- event, a contact, a lead…), by hand or by itself, with a link — or why not.
CREATE TABLE made (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  booking_id  INTEGER NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
  target      TEXT NOT NULL,                            -- event | contact | deal | task | note | invoice | payment | time | email
  app         TEXT NOT NULL DEFAULT '',
  ref         TEXT NOT NULL DEFAULT '',
  title       TEXT NOT NULL DEFAULT '',
  url         TEXT NOT NULL DEFAULT '',
  error       TEXT NOT NULL DEFAULT '',
  auto        INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX made_user ON made(user_id, booking_id);

-- How many people opened a booking page, per day (no cookies, no addresses kept).
CREATE TABLE views (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type_id INTEGER NOT NULL REFERENCES types(id) ON DELETE CASCADE,
  day     TEXT NOT NULL,
  count   INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (type_id, day)
);
CREATE INDEX views_user ON views(user_id, day);
