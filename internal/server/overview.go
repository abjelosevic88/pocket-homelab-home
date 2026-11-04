package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// handleOverview is the first page: what needs doing, what is coming,
// what was done lately, what it cost, warranties and meters.
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	home := int64(queryInt(r, "home_id"))
	today := store.Today()
	homes, err := s.store.ListHomes(ctx, uid)
	if err != nil {
		s.fail(w, err, "overview")
		return
	}
	if homes == nil {
		homes = []store.Home{}
	}
	jobs, _ := s.store.ListJobs(ctx, uid, store.JobFilter{HomeID: home, Until: store.AddDays(today, 60)})
	attention, upcoming := []store.Job{}, []store.Job{}
	counts := map[string]int{"overdue": 0, "due": 0, "soon": 0}
	for _, j := range jobs {
		switch j.Status {
		case store.StatusOverdue, store.StatusDue, store.StatusSoon:
			attention = append(attention, j)
			counts[j.Status]++
		default:
			upcoming = append(upcoming, j)
		}
	}
	if len(upcoming) > 8 {
		upcoming = upcoming[:8]
	}
	recent, _, _ := s.store.ListRecords(ctx, uid, store.RecordFilter{HomeID: home, Limit: 6})
	things, _ := s.store.ListThings(ctx, uid, store.ThingFilter{HomeID: home})
	warranties := []map[string]any{}
	for _, t := range things {
		if st := warrantyState(t.WarrantyUntil, today); st == "ending" || (st == "ended" && store.DaysBetween(t.WarrantyUntil, today) <= 30) {
			warranties = append(warranties, map[string]any{"thing_id": t.ID, "name": t.Name, "until": t.WarrantyUntil, "state": st, "days": store.DaysBetween(today, t.WarrantyUntil)})
		}
	}
	sort.Slice(warranties, func(i, j int) bool { return warranties[i]["until"].(string) < warranties[j]["until"].(string) })
	meters, _ := s.store.ListMeters(ctx, uid, home)
	meterViews := []map[string]any{}
	for _, m := range meters {
		if m.Archived {
			continue
		}
		v := map[string]any{"id": m.ID, "name": m.Name, "kind": m.Kind, "unit": m.Unit, "home_name": m.HomeName, "last": m.Last, "readings": m.Readings}
		v["needs_reading"] = m.Last == nil || store.DaysBetween(m.Last.ReadOn, today) > 35
		meterViews = append(meterViews, v)
	}
	yearFrom := today[:4] + "-01-01"
	spentYear, _ := s.store.Spending(ctx, uid, home, "year", yearFrom, today)
	lastYear, _ := s.store.Spending(ctx, uid, home, "year", store.AddDays(yearFrom, -365)[:4]+"-01-01", store.AddDays(today, -365))
	writeJSON(w, http.StatusOK, map[string]any{"today": today, "homes": homes, "counts": counts, "attention": attention, "upcoming": upcoming, "recent": recent,
		"warranties": warranties, "meters": meterViews, "things": len(things), "spent_year": spentYear, "spent_last_year_to_date": lastYear})
}

// handleDay is one day in the home: what was done and read — what Pocket
// Day writes into the journal.
func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	date := chi.URLParam(r, "date")
	if !validDate(date) || date == "" {
		writeErr(w, http.StatusBadRequest, "the date is YYYY-MM-DD")
		return
	}
	recs, _, err := s.store.ListRecords(ctx, uid, store.RecordFilter{From: date, To: date})
	if err != nil {
		s.fail(w, err, "day")
		return
	}
	readings := []map[string]any{}
	meters, _ := s.store.ListMeters(ctx, uid, 0)
	for _, m := range meters {
		rs, _ := s.store.ListReadings(ctx, uid, m.ID, 0)
		for _, rd := range rs {
			if rd.ReadOn == date {
				readings = append(readings, map[string]any{"meter_id": m.ID, "meter": m.Name, "kind": m.Kind, "unit": m.Unit, "value": rd.Value, "usage": rd.Usage, "days": rd.Days})
			}
		}
	}
	due, _ := s.store.ListJobs(ctx, uid, store.JobFilter{Until: date})
	dueThat := []store.Job{}
	for _, j := range due {
		if j.DueOn == date {
			dueThat = append(dueThat, j)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "records": recs, "readings": readings, "due": dueThat})
}

// handleSearch looks through homes, things, upkeep, the log, pros and meters.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) > 200 {
		writeErr(w, http.StatusBadRequest, "search for 1 to 200 characters")
		return
	}
	limit := queryInt(r, "limit")
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	lq := strings.ToLower(q)
	homes := []store.Home{}
	hs, _ := s.store.ListHomes(ctx, uid)
	for _, h := range hs {
		if strings.Contains(strings.ToLower(h.Name+" "+h.Address+" "+h.Notes), lq) {
			homes = append(homes, h)
		}
	}
	things, _ := s.store.ListThings(ctx, uid, store.ThingFilter{Q: q, Archived: true})
	jobs, _ := s.store.ListJobs(ctx, uid, store.JobFilter{Q: q})
	recs, _, _ := s.store.ListRecords(ctx, uid, store.RecordFilter{Q: q, Limit: limit})
	pros, _ := s.store.ListPros(ctx, uid, q)
	meters := []store.Meter{}
	ms, _ := s.store.ListMeters(ctx, uid, 0)
	for _, m := range ms {
		if strings.Contains(strings.ToLower(m.Name+" "+m.Number+" "+m.Notes+" "+m.Kind), lq) {
			meters = append(meters, m)
		}
	}
	cut := func(n int) int {
		if n > limit {
			return limit
		}
		return n
	}
	if things == nil {
		things = []store.Thing{}
	}
	if pros == nil {
		pros = []store.Pro{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": q, "homes": homes[:cut(len(homes))], "things": things[:cut(len(things))], "jobs": jobs[:cut(len(jobs))],
		"records": recs, "pros": pros[:cut(len(pros))], "meters": meters[:cut(len(meters))]})
}
