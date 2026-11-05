# API

JSON over HTTP at `/api/v1`. Authenticate with the session cookie, a personal token (`Authorization: Bearer phm_…`, Settings → API & shortcuts), a Pocket Core device or user token, or a Pocket Core app-link token. Everything is per person: another person's ids answer 404. Money is in **minor units** (cents) of the home's currency; dates are `YYYY-MM-DD`. PUT bodies are partial: only the keys you send change.

## Overview

| Route | What |
|---|---|
| `GET /overview?home_id` | Counts (overdue, due, soon — plant care included), what needs doing, the next two months (plant care only when rarer than every three months), recent log (without plant care), warranties ending, meters, `plants {count, needs_care, water_due}`, spending this year vs last year to date |
| `GET /due?days=30&home_id&scope` | `{overdue, due, soon, upcoming, counts}` — what the Pocket AI briefing reads. Plant care is in it (`plant_id`, `plant_name`, `care` set); `scope=house` or `scope=plants` for one side only |
| `GET /day/{date}` | One day: log entries (plant care too, e.g. "Watered Monstera"), meter readings, upkeep due that day, and `plant_care [{care, label, plants, plant_ids}]` ("Watered: Monstera, Pothos") — Pocket Day |
| `GET /search?q&limit` | Homes, things, plants, upkeep, log entries, pros and meters matching `q` |
| `GET /spending?months=12&home_id` | Spent per month, per kind, per thing (top 10) and per year |

## Homes, things, pros

| Route | What |
|---|---|
| `GET/POST /homes`, `GET/PUT/DELETE /homes/{id}` | `name`, `kind` (house, apartment, cottage, other), `address`, `year_built`, `area_m2`, `currency`, `notes`, `money_tag`, `archived`. DELETE removes everything in it |
| `GET /things?home_id&category&room&q&archived=1`, `POST /things` | Appliances and parts: `name`, `category` (heating, cooling, water, electrical, appliance, kitchen, structure, outdoor, safety, other), `room`, `brand`, `model`, `serial`, `installed_on`, `purchased_on`, `price`, `vendor`, `warranty_until`, `manual_url`, `inventory_id`, `notes`, `archived` |
| `GET/PUT/DELETE /things/{id}` | GET adds its upkeep, log, attachments, warranty state and suggestions |
| `GET /templates?thing_id` or `?category&name` | Built-in upkeep suggestions |
| `POST /things/{id}/suggest` | Upkeep suggestions from the AI for this model (nothing saved) |
| `GET /rooms` | Rooms in use |
| `GET/POST /pros`, `GET/PUT/DELETE /pros/{id}`, `POST /pros/from-contact {contact_id}` | `name`, `trade`, `company`, `phone`, `email`, `website`, `rating` 0–5, `notes` |
| `POST /attachments`, `DELETE /attachments/{id}` | `{owner_kind: home|thing|plant|record, owner_id, kind: document|url, ref (Pocket Documents id) or url, title}` |

## Plants

House plants. A plant's care is upkeep — a job with `plant_id` and `care` set — so it shares the due list, the daily message, the calendar and `done`; what was done is a log entry with `plant_id` and `kind: plant`. Care kinds: `water`, `fertilise`, `mist`, `rotate`, `prune`, `pests` (check for pests), `repot`.

| Route | What |
|---|---|
| `GET /plants?home_id&room&q&archived=1` | Each plant with `care` (its open care jobs, most urgent first), `status` (of the most urgent), `next_care`, `last_watered`. Archived (gone) plants only with `archived=1` |
| `POST /plants` | `name` (required), `home_id` (needed when there is more than one home), `species`, `kind` (foliage, flowering, succulent, cactus, herb, fern, orchid, palm, other; guessed from the name when left out), `room` (room or spot), `light` (low, medium, bright, direct; default medium), `acquired_on`, `pot`, `notes`, `last_watered` (YYYY-MM-DD, watering is counted from it), `care` (the schedule, see below; left out = the suggested one, `[]` = none) → the plant, 201 |
| `GET /plants/{id}` | `{plant, records, records_total, attachments, suggestions}` — the plant with all its care (paused too), its last 100 log entries and the suggested schedule for its kind and light |
| `PUT /plants/{id}` | Any plant field, plus `archived` with `gone_reason` (died, given, other) and `gone_on` (default today); archiving pauses its care, un-archiving starts it again. `care` here changes the schedule line by line: `[{care, every_n, every_unit, soon_days, due_on, last_done, notes, paused}]` adds or changes that kind of care (fields left out keep their value, or come from the suggestion), `every_n: 0` removes it. A new rhythm counts from `last_done` |
| `DELETE /plants/{id}` | The plant, its care and its care log go (entries booked in Pocket Money stay) |
| `GET /plants/templates?kind&light&name&species` | `{kind, guessed_kind, light, care: [{care, title, every_n, every_unit, soon_days, notes}]}` — the suggested schedule; watering is stretched for low light and shortened for direct sun |
| `POST /plants/{id}/done` | `{care: "water" (default), date, notes, cost, title, money}` → `{plant, job, record}`. Care on the schedule moves to its next date ("Watered"); care that is not on it is only logged (`job: null`) |
| `POST /plants/done` | `{care: "water" (default), plant_ids: [], date, home_id}` — several at once: the plants in `plant_ids` (any state), or with none given every plant whose care is due today or overdue ("Water all due") → `{care, count, done: [{plant_id, plant, job_id, next_due}]}` |

Plant care can also be read and changed through `/jobs` (`?plant_id`, `?scope=plants`; `POST /jobs` with `plant_id` and `care`) and done with `POST /jobs/{id}/done`, which logs it as `kind: plant`, "Watered Monstera".

## Upkeep

| Route | What |
|---|---|
| `GET /jobs?home_id&thing_id&plant_id&scope&q&all=1` | Soonest first, each with `status` (overdue, due, soon, ok, paused, done) and `days_left`; `scope=house` or `scope=plants` |
| `POST /jobs`, `POST /jobs/bulk {jobs: […]}` | `title`, `home_id` or `thing_id`, `every_n` (0 = once), `every_unit` (day, week, month, year), `due_on` (or `last_done` to count from), `soon_days`, `pro_id`, `est_cost`, `notes`, `paused` |
| `GET/PUT/DELETE /jobs/{id}` | |
| `POST /jobs/{id}/done` | `{date, cost, pro_id, notes, log: true, money: true, account_id, category_id}` → a log entry (booked in Pocket Money when asked) and the next due date, counted from `date` |
| `POST /jobs/{id}/skip`, `POST /jobs/{id}/snooze {days}` | Next interval without a log entry; move the date |
| `POST /jobs/{id}/task` | Add it to Pocket Tasks |

## The log

| Route | What |
|---|---|
| `GET /records?home_id&thing_id&plant_id&pro_id&kind&scope&from&to&q&limit&offset` | Newest first, `{records, total}`; `scope=house` leaves plant care out, `scope=plants` is plant care only |
| `POST /records` | `title`, `date`, `kind` (maintenance, repair, improvement, inspection, purchase, plant, other), `cost`, `home_id`/`thing_id`/`plant_id`, `job_id`, `pro_id`, `notes`, `external_id` (a repeat returns the first), `money` (book in Pocket Money; default: the setting) |
| `GET/PUT/DELETE /records/{id}` | Changes to the date, cost, title and pro follow to the Money transaction; a cost of 0 or DELETE removes it |
| `POST /records/{id}/money` | Book it in Pocket Money now |

## Meters

| Route | What |
|---|---|
| `GET/POST /meters`, `GET/PUT/DELETE /meters/{id}` | `name`, `kind` (electricity, water, gas, heating, solar, oil, other), `unit`, `price_per_unit` (major units), `number`, `remind_day` (1–31, 0 = none). GET adds readings, use per month and a summary |
| `GET /meters/{id}/readings`, `POST /meters/{id}/readings` | `{value, read_on, reset, note}`; a value below the last one needs `reset` (a new meter) |
| `PUT/DELETE /readings/{id}` | |
| `GET /meters/{id}/usage?months=12` | Use (and cost) per calendar month, spread evenly between readings |

## The other Pocket apps

`GET /links/money/accounts`, `/links/money/categories?account_id`, `/links/calendar/calendars`, `/links/contacts?q`, `/links/documents?q`, `/links/inventory?q` answer `{connected, detail, items}` from the person's own connection. `POST /links/calendar/sync` brings the calendar up to date now.

## Pocket Core app links

Home offers `home:read` (overview, due, day, search, spending, homes, things, plants and plant templates, upkeep, log, meters and readings, pros) and `home:write` (`POST /records`, `PUT/DELETE /records/{id}` for entries that app made, `POST /meters/{id}/readings`, `POST /jobs/{id}/done`, `POST /plants/{id}/done`, `POST /plants/done`). Everything else answers 403 to a link token.

It uses Pocket Money (`accounts:read`, `transactions:write`), Pocket Calendar (`calendar:read`, `calendar:write`), Pocket Tasks (`tasks:write`), Pocket Contacts (`contacts:read`), Pocket Documents (`documents:read`), Pocket Inventory (`items:read`) and Pocket AI inference. Events: `home.job_done`, `home.job_due`, `home.record_added`, `home.reading_added`, `home.thing_saved`, `home.warranty_ending`, `home.plant_saved`, `home.plant_cared`, `home.plant_care_due`, each with `user_email`. Plant care sends `home.plant_cared` (`plant_id`, `plant`, `care`, `home`, `date`, `next_due`, `job_id`, `record_id`) instead of `home.job_done`, and `home.plant_care_due` (`plant_id`, `plant`, `care`, `job_id`, `home`, `due_on`, `status`, `days_left`) instead of `home.job_due`; `home.plant_saved` (`plant_id`, `name`, `species`, `kind`, `home`, `room`, `light`, `archived`, `gone_reason`) after a plant is added or changed. Subjects are `plant:<id>`. `home.thing_saved` fires after a thing is created or updated (subject `thing:<id>`, payload `thing_id`, `name`, `home`, `warranty_until` as YYYY-MM-DD or ""), so Pocket Renewals can pick up a warranty as soon as it is saved.

## Settings, data

`GET/PUT /settings`, `GET /settings/system`, `POST /integrations/test {which}`, `POST /notify/test`, `POST /notify/check` (run the daily check now), `GET/POST/DELETE /tokens`, `GET /backup` (admin), `GET /export.json`, `GET /export.csv`, `POST /import`. Health: `GET /healthz`, `/readyz`, `/metrics`.
