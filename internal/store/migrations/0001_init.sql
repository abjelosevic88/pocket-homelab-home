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

-- ---------- Pocket Home: everything below is per person ----------

-- A place you look after: a house, a flat, a weekend cottage.
CREATE TABLE homes (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL DEFAULT 'house',       -- house | apartment | cottage | other
  address     TEXT NOT NULL DEFAULT '',
  year_built  INTEGER,
  area_m2     REAL,
  currency    TEXT NOT NULL DEFAULT 'EUR',
  notes       TEXT NOT NULL DEFAULT '',
  money_tag   TEXT NOT NULL DEFAULT '',            -- tag put on Pocket Money bookings
  archived    INTEGER NOT NULL DEFAULT 0,
  sort        INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX homes_user ON homes(user_id, archived, sort);

-- An appliance or a part of the house: the boiler, the roof, the fridge.
CREATE TABLE things (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  home_id        INTEGER NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
  name           TEXT NOT NULL,
  category       TEXT NOT NULL DEFAULT 'appliance', -- heating | cooling | water | electrical | appliance | kitchen | structure | outdoor | safety | other
  room           TEXT NOT NULL DEFAULT '',
  brand          TEXT NOT NULL DEFAULT '',
  model          TEXT NOT NULL DEFAULT '',
  serial         TEXT NOT NULL DEFAULT '',
  installed_on   TEXT NOT NULL DEFAULT '',
  purchased_on   TEXT NOT NULL DEFAULT '',
  price          INTEGER,                            -- minor units of the home's currency
  vendor         TEXT NOT NULL DEFAULT '',
  warranty_until TEXT NOT NULL DEFAULT '',
  manual_url     TEXT NOT NULL DEFAULT '',
  inventory_id   INTEGER,                            -- the item in Pocket Inventory, if linked
  inventory_name TEXT NOT NULL DEFAULT '',
  notes          TEXT NOT NULL DEFAULT '',
  archived       INTEGER NOT NULL DEFAULT 0,
  created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX things_user ON things(user_id, home_id, archived);

-- Someone who works on the house: a plumber, the chimney sweep.
CREATE TABLE pros (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  trade       TEXT NOT NULL DEFAULT '',
  company     TEXT NOT NULL DEFAULT '',
  phone       TEXT NOT NULL DEFAULT '',
  email       TEXT NOT NULL DEFAULT '',
  website     TEXT NOT NULL DEFAULT '',
  notes       TEXT NOT NULL DEFAULT '',
  rating      INTEGER NOT NULL DEFAULT 0,             -- 0 none, 1–5
  contact_id  INTEGER,                                -- the card in Pocket Contacts, if linked
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX pros_user ON pros(user_id, name);

-- Upkeep: something to do, every so often or once. Done -> a log entry and
-- the next due date.
CREATE TABLE jobs (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  home_id         INTEGER NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
  thing_id        INTEGER REFERENCES things(id) ON DELETE SET NULL,
  pro_id          INTEGER REFERENCES pros(id) ON DELETE SET NULL,
  title           TEXT NOT NULL,
  notes           TEXT NOT NULL DEFAULT '',
  every_n         INTEGER NOT NULL DEFAULT 0,          -- 0 = once
  every_unit      TEXT NOT NULL DEFAULT 'month',       -- day | week | month | year
  due_on          TEXT NOT NULL,                       -- YYYY-MM-DD
  soon_days       INTEGER NOT NULL DEFAULT 14,         -- "due soon" this many days before
  est_cost        INTEGER,                             -- minor units, a guess
  last_done       TEXT NOT NULL DEFAULT '',
  done_count      INTEGER NOT NULL DEFAULT 0,
  paused          INTEGER NOT NULL DEFAULT 0,
  finished_at     TEXT NOT NULL DEFAULT '',            -- a one-off that is done
  notified_level  TEXT NOT NULL DEFAULT '',            -- soon | due | overdue, the last push
  notified_on     TEXT NOT NULL DEFAULT '',
  cal_event_id    INTEGER,                             -- the event in Pocket Calendar
  cal_synced      TEXT NOT NULL DEFAULT '',            -- what the event shows (title|due), "" = needs a push
  task_id         INTEGER,                             -- a task in Pocket Tasks
  source          TEXT NOT NULL DEFAULT 'web',
  created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX jobs_user ON jobs(user_id, home_id, due_on);
CREATE INDEX jobs_thing ON jobs(user_id, thing_id);

-- The log: what was done on the house, when, by whom and what it cost.
CREATE TABLE records (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  home_id       INTEGER NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
  thing_id      INTEGER REFERENCES things(id) ON DELETE SET NULL,
  job_id        INTEGER REFERENCES jobs(id) ON DELETE SET NULL,
  pro_id        INTEGER REFERENCES pros(id) ON DELETE SET NULL,
  date          TEXT NOT NULL,
  kind          TEXT NOT NULL DEFAULT 'maintenance',   -- maintenance | repair | improvement | inspection | purchase | other
  title         TEXT NOT NULL,
  notes         TEXT NOT NULL DEFAULT '',
  cost          INTEGER NOT NULL DEFAULT 0,            -- minor units
  currency      TEXT NOT NULL DEFAULT 'EUR',
  money_tx_id   INTEGER,                               -- the booking in Pocket Money
  money_user_id INTEGER,                               -- whose Money it is in (always the owner here)
  source        TEXT NOT NULL DEFAULT 'web',           -- web | api | import | demo | <app slug>
  external_id   TEXT NOT NULL DEFAULT '',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX records_user ON records(user_id, home_id, date);
CREATE INDEX records_thing ON records(user_id, thing_id, date);
CREATE UNIQUE INDEX records_external ON records(user_id, external_id) WHERE external_id <> '';

-- A meter: electricity, water, gas. Readings make the usage.
CREATE TABLE meters (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  home_id        INTEGER NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
  name           TEXT NOT NULL,
  kind           TEXT NOT NULL DEFAULT 'electricity',  -- electricity | water | gas | heating | solar | oil | other
  unit           TEXT NOT NULL DEFAULT 'kWh',
  price_per_unit REAL,                                 -- major units of the home's currency per unit
  number         TEXT NOT NULL DEFAULT '',             -- the meter's own number
  notes          TEXT NOT NULL DEFAULT '',
  remind_day     INTEGER NOT NULL DEFAULT 0,           -- day of the month to read it (0 = no reminder)
  reminded_on    TEXT NOT NULL DEFAULT '',
  archived       INTEGER NOT NULL DEFAULT 0,
  created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX meters_user ON meters(user_id, home_id);

CREATE TABLE readings (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  meter_id    INTEGER NOT NULL REFERENCES meters(id) ON DELETE CASCADE,
  read_on     TEXT NOT NULL,                           -- YYYY-MM-DD
  value       REAL NOT NULL,
  reset       INTEGER NOT NULL DEFAULT 0,              -- a new meter: no usage from the reading before
  note        TEXT NOT NULL DEFAULT '',
  source      TEXT NOT NULL DEFAULT 'web',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX readings_meter ON readings(user_id, meter_id, read_on);

-- A paper or a link kept with a home, a thing or a log entry: a manual, a
-- warranty card, an invoice in Pocket Documents.
CREATE TABLE attachments (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  owner_kind  TEXT NOT NULL,                           -- home | thing | record
  owner_id    INTEGER NOT NULL,
  kind        TEXT NOT NULL DEFAULT 'url',             -- document | url
  ref         TEXT NOT NULL DEFAULT '',                -- the Pocket Documents id
  title       TEXT NOT NULL DEFAULT '',
  url         TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX attachments_owner ON attachments(user_id, owner_kind, owner_id);
