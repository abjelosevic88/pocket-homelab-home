# API

JSON over HTTP at `/api/v1`. Authenticate with the session cookie, a personal token (`Authorization: Bearer phm_…`, Settings → API & shortcuts), a Pocket Core device or user token, or a Pocket Core app-link token. Everything is per person: another person's ids answer 404. Money is in **minor units** (cents) of the home's currency; dates are `YYYY-MM-DD`. PUT bodies are partial: only the keys you send change.

## Overview

| Route | What |
|---|---|
| `GET /overview?home_id` | Counts (overdue, due, soon), what needs doing, the next two months, recent log, warranties ending, meters, spending this year vs last year to date |
| `GET /due?days=30&home_id` | `{overdue, due, soon, upcoming, counts}` — what the Pocket AI briefing reads |
| `GET /day/{date}` | One day: log entries, meter readings and upkeep due that day (Pocket Day) |
| `GET /search?q&limit` | Homes, things, upkeep, log entries, pros and meters matching `q` |
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
| `POST /attachments`, `DELETE /attachments/{id}` | `{owner_kind: home|thing|record, owner_id, kind: document|url, ref (Pocket Documents id) or url, title}` |

## Upkeep

| Route | What |
|---|---|
| `GET /jobs?home_id&thing_id&q&all=1` | Soonest first, each with `status` (overdue, due, soon, ok, paused, done) and `days_left` |
| `POST /jobs`, `POST /jobs/bulk {jobs: […]}` | `title`, `home_id` or `thing_id`, `every_n` (0 = once), `every_unit` (day, week, month, year), `due_on` (or `last_done` to count from), `soon_days`, `pro_id`, `est_cost`, `notes`, `paused` |
| `GET/PUT/DELETE /jobs/{id}` | |
| `POST /jobs/{id}/done` | `{date, cost, pro_id, notes, log: true, money: true, account_id, category_id}` → a log entry (booked in Pocket Money when asked) and the next due date, counted from `date` |
| `POST /jobs/{id}/skip`, `POST /jobs/{id}/snooze {days}` | Next interval without a log entry; move the date |
| `POST /jobs/{id}/task` | Add it to Pocket Tasks |

## The log

| Route | What |
|---|---|
| `GET /records?home_id&thing_id&pro_id&kind&from&to&q&limit&offset` | Newest first, `{records, total}` |
| `POST /records` | `title`, `date`, `kind` (maintenance, repair, improvement, inspection, purchase, other), `cost`, `home_id`/`thing_id`, `job_id`, `pro_id`, `notes`, `external_id` (a repeat returns the first), `money` (book in Pocket Money; default: the setting) |
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

Home offers `home:read` (overview, due, day, search, spending, homes, things, upkeep, log, meters and readings, pros) and `home:write` (`POST /records`, `PUT/DELETE /records/{id}` for entries that app made, `POST /meters/{id}/readings`, `POST /jobs/{id}/done`). Everything else answers 403 to a link token.

It uses Pocket Money (`accounts:read`, `transactions:write`), Pocket Calendar (`calendar:read`, `calendar:write`), Pocket Tasks (`tasks:write`), Pocket Contacts (`contacts:read`), Pocket Documents (`documents:read`), Pocket Inventory (`items:read`) and Pocket AI inference. Events: `home.job_done`, `home.job_due`, `home.record_added`, `home.reading_added`, `home.warranty_ending`, each with `user_email`.

## Settings, data

`GET/PUT /settings`, `GET /settings/system`, `POST /integrations/test {which}`, `POST /notify/test`, `POST /notify/check` (run the daily check now), `GET/POST/DELETE /tokens`, `GET /backup` (admin), `GET /export.json`, `GET /export.csv`, `POST /import`. Health: `GET /healthz`, `/readyz`, `/metrics`.
