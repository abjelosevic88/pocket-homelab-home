-- ---------- Plants: house plants and their care ----------
--
-- A plant's care (water, fertilise, repot…) is upkeep: a row in jobs with
-- plant_id and care set, so it has the same due dates, reminders, calendar
-- events and "done" as the rest. What was done is a log entry (records) with
-- plant_id set and kind 'plant'. One due list, one log.

CREATE TABLE plants (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  home_id      INTEGER NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  species      TEXT NOT NULL DEFAULT '',            -- species or common name
  kind         TEXT NOT NULL DEFAULT 'foliage',     -- foliage | flowering | succulent | cactus | herb | fern | orchid | palm | other
  room         TEXT NOT NULL DEFAULT '',            -- the room or the spot
  light        TEXT NOT NULL DEFAULT 'medium',      -- low | medium | bright (indirect) | direct
  acquired_on  TEXT NOT NULL DEFAULT '',
  pot          TEXT NOT NULL DEFAULT '',            -- pot size, soil, notes on the pot
  notes        TEXT NOT NULL DEFAULT '',
  archived     INTEGER NOT NULL DEFAULT 0,
  gone_reason  TEXT NOT NULL DEFAULT '',            -- died | given | other, when archived
  gone_on      TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX plants_user ON plants(user_id, home_id, archived);

ALTER TABLE jobs ADD COLUMN plant_id INTEGER REFERENCES plants(id) ON DELETE CASCADE;
ALTER TABLE jobs ADD COLUMN care TEXT NOT NULL DEFAULT '';   -- water | fertilise | mist | repot | prune | rotate | pests, for a plant
CREATE INDEX jobs_plant ON jobs(user_id, plant_id);

ALTER TABLE records ADD COLUMN plant_id INTEGER REFERENCES plants(id) ON DELETE SET NULL;
CREATE INDEX records_plant ON records(user_id, plant_id, date);
