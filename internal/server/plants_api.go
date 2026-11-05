package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// ---------- plants ----------
//
// A plant's care is upkeep (jobs with plant_id and care), so it shares the
// due list, the reminders, the calendar and "done"; what was done is in the
// log. These handlers add the plant itself, its schedule in one go, and
// "Watered" for one plant or many.

// careInput is one line of a plant's schedule. On update it changes only the
// fields sent; every_n 0 removes that care.
type careInput struct {
	Care      string  `json:"care"`
	EveryN    *int    `json:"every_n"`
	EveryUnit *string `json:"every_unit"`
	SoonDays  *int    `json:"soon_days"`
	DueOn     *string `json:"due_on"`
	LastDone  *string `json:"last_done"`
	Notes     *string `json:"notes"`
	Paused    *bool   `json:"paused"`
}

type plantInput struct {
	HomeID      *int64       `json:"home_id"`
	Name        *string      `json:"name"`
	Species     *string      `json:"species"`
	Kind        *string      `json:"kind"`
	Room        *string      `json:"room"`
	Light       *string      `json:"light"`
	AcquiredOn  *string      `json:"acquired_on"`
	Pot         *string      `json:"pot"`
	Notes       *string      `json:"notes"`
	Archived    *bool        `json:"archived"`
	GoneReason  *string      `json:"gone_reason"`
	GoneOn      *string      `json:"gone_on"`
	LastWatered *string      `json:"last_watered"` // create: when it was last watered (default today)
	Care        *[]careInput `json:"care"`         // create: the schedule (default: suggested); update: changes to it
}

func (in plantInput) apply(p *store.Plant) string {
	if in.HomeID != nil {
		p.HomeID = *in.HomeID
	}
	trimP(&p.Name, in.Name)
	trimP(&p.Species, in.Species)
	if in.Kind != nil && strings.TrimSpace(*in.Kind) != "" {
		p.Kind = strings.TrimSpace(*in.Kind)
	}
	trimP(&p.Room, in.Room)
	if in.Light != nil && strings.TrimSpace(*in.Light) != "" {
		p.Light = strings.TrimSpace(*in.Light)
	}
	trimP(&p.AcquiredOn, in.AcquiredOn)
	trimP(&p.Pot, in.Pot)
	trimP(&p.Notes, in.Notes)
	trimP(&p.GoneReason, in.GoneReason)
	trimP(&p.GoneOn, in.GoneOn)
	if in.Archived != nil {
		p.Archived = *in.Archived
	}
	if p.Archived && p.GoneOn == "" {
		p.GoneOn = store.Today()
	}
	if !p.Archived {
		p.GoneReason, p.GoneOn = "", ""
	}
	switch {
	case p.HomeID <= 0:
		return "choose the home it is in"
	case p.Name == "" || len(p.Name) > 120:
		return "a plant needs a name (up to 120 characters)"
	case len(p.Species) > 160 || len(p.Room) > 60 || len(p.Pot) > 200:
		return "the species, place or pot is too long"
	case !oneOf(p.Kind, plantKinds):
		return "the kind is foliage, flowering, succulent, cactus, herb, fern, orchid, palm or other"
	case !oneOf(p.Light, plantLights):
		return "the light is low, medium, bright or direct"
	case p.GoneReason != "" && !oneOf(p.GoneReason, goneReasons):
		return "it died, was given away, or other"
	case !validDate(p.AcquiredOn) || !validDate(p.GoneOn):
		return "dates are YYYY-MM-DD"
	}
	return ""
}

// careLine checks one line of a schedule and fills it in: from the job it
// changes, else from the suggestion for this plant, else a month.
func careLine(p *store.Plant, c careInput, j *store.Job) (*store.Job, string) {
	if !validCare(c.Care) {
		return nil, "care is water, fertilise, mist, rotate, prune, pests or repot"
	}
	if j == nil {
		base := cs(c.Care, 1, "month", 2, "")
		for _, sg := range careFor(p.Kind, p.Light) {
			if sg.Care == c.Care {
				base = sg
			}
		}
		from := store.Today()
		if c.LastDone != nil && *c.LastDone != "" {
			from = *c.LastDone
		}
		if c.EveryN != nil {
			base.EveryN = *c.EveryN
		}
		if c.EveryUnit != nil {
			base.EveryUnit = *c.EveryUnit
		}
		nj := careJob(p, base, from)
		if c.LastDone != nil {
			nj.LastDone = *c.LastDone
		}
		j = &nj
	} else {
		moved := false
		if c.EveryN != nil && *c.EveryN != j.EveryN {
			j.EveryN, moved = *c.EveryN, true
		}
		if c.EveryUnit != nil && *c.EveryUnit != j.EveryUnit {
			j.EveryUnit, moved = *c.EveryUnit, true
		}
		if c.LastDone != nil {
			j.LastDone, moved = *c.LastDone, true
		}
		if moved && j.LastDone != "" && j.EveryN > 0 {
			// a new rhythm counts from the day it was last done
			j.DueOn = store.AddInterval(j.LastDone, j.EveryN, j.EveryUnit)
		}
	}
	if c.SoonDays != nil {
		j.SoonDays = *c.SoonDays
	}
	if c.DueOn != nil && *c.DueOn != "" {
		j.DueOn = *c.DueOn
	}
	if c.Notes != nil {
		j.Notes = strings.TrimSpace(*c.Notes)
	}
	if c.Paused != nil {
		j.Paused = *c.Paused
	}
	if p.Archived {
		j.Paused = true
	}
	j.HomeID, j.PlantID, j.Care, j.Title = p.HomeID, &p.ID, c.Care, careTitle(c.Care)
	switch {
	case j.EveryN < 1 || j.EveryN > 1000:
		return nil, "repeat every 1 to 1000 days, weeks, months or years"
	case !store.ValidUnit(j.EveryUnit):
		return nil, "the repeat unit is day, week, month or year"
	case !validDate(j.DueOn) || j.DueOn == "" || !validDate(j.LastDone):
		return nil, "dates are YYYY-MM-DD"
	case j.SoonDays < 0 || j.SoonDays > 365:
		return nil, "remind 0 to 365 days before"
	}
	return j, ""
}

// saveCare applies schedule changes to a plant: a line per care kind is
// added or changed, every_n 0 removes it. It returns removed jobs.
func (s *Server) saveCare(ctx context.Context, uid int64, p *store.Plant, lines []careInput) ([]store.Job, string, error) {
	have := map[string]*store.Job{}
	cur, err := s.store.ListJobs(ctx, uid, store.JobFilter{PlantID: p.ID})
	if err != nil {
		return nil, "", err
	}
	for i := range cur {
		have[cur[i].Care] = &cur[i]
	}
	// check everything before saving anything
	type step struct {
		j      *store.Job
		create bool
		remove bool
	}
	var steps []step
	seen := map[string]bool{}
	for _, c := range lines {
		c.Care = strings.TrimSpace(c.Care)
		if seen[c.Care] {
			return nil, "each kind of care once", nil
		}
		seen[c.Care] = true
		old := have[c.Care]
		if c.EveryN != nil && *c.EveryN == 0 {
			if !validCare(c.Care) {
				return nil, "care is water, fertilise, mist, rotate, prune, pests or repot", nil
			}
			if old != nil {
				steps = append(steps, step{j: old, remove: true})
			}
			continue
		}
		j, msg := careLine(p, c, old)
		if msg != "" {
			return nil, msg, nil
		}
		steps = append(steps, step{j: j, create: old == nil})
	}
	var removed []store.Job
	for _, st := range steps {
		switch {
		case st.remove:
			if err := s.store.DeleteJob(ctx, uid, st.j.ID); err != nil {
				return nil, "", err
			}
			removed = append(removed, *st.j)
		case st.create:
			st.j.Source = firstNonEmpty(linkApp(ctx), "web")
			if err := s.store.CreateJob(ctx, uid, st.j); err != nil {
				return nil, "", err
			}
		default:
			if err := s.store.UpdateJob(ctx, uid, st.j); err != nil {
				return nil, "", err
			}
			_ = s.store.SetJobCalendar(ctx, uid, st.j.ID, st.j.CalEventID, "")
		}
	}
	return removed, "", nil
}

func (s *Server) handleListPlants(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ps, err := s.store.ListPlants(r.Context(), actorID(r.Context()), store.PlantFilter{HomeID: int64(queryInt(r, "home_id")), Room: q.Get("room"),
		Q: strings.TrimSpace(q.Get("q")), Archived: queryBool(r, "archived")})
	if err != nil {
		s.fail(w, err, "plants")
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *Server) handleGetPlant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlant(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plant")
		return
	}
	recs, total, _ := s.store.ListRecords(ctx, uid, store.RecordFilter{PlantID: p.ID, Limit: 100})
	atts, _ := s.store.ListAttachments(ctx, uid, "plant", p.ID)
	writeJSON(w, http.StatusOK, map[string]any{"plant": p, "records": recs, "records_total": total, "attachments": s.attachmentViews(ctx, uid, atts),
		"suggestions": careFor(p.Kind, p.Light)})
}

// handlePlantTemplates is the suggested care for a kind of plant in its
// light (?kind&light), the kind guessed from ?name and ?species when not given.
func (s *Server) handlePlantTemplates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind, guessed := q.Get("kind"), ""
	if g := guessPlantKind(q.Get("name"), q.Get("species")); g != "" {
		guessed = g
	}
	if !oneOf(kind, plantKinds) {
		kind = firstNonEmpty(guessed, "foliage")
	}
	light := q.Get("light")
	if !oneOf(light, plantLights) {
		light = "medium"
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": kind, "guessed_kind": guessed, "light": light, "care": careFor(kind, light)})
}

func (s *Server) handleCreatePlant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in plantInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p := store.Plant{Light: "medium"}
	if in.Kind == nil || strings.TrimSpace(*in.Kind) == "" {
		p.Kind = firstNonEmpty(guessPlantKind(strPtr(in.Name), strPtr(in.Species)), "foliage")
		in.Kind = nil
	}
	if in.HomeID == nil {
		if hs, _ := s.store.ListHomes(ctx, uid); len(hs) == 1 {
			p.HomeID = hs[0].ID
		}
	}
	if msg := in.apply(&p); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	watered := strPtr(in.LastWatered)
	if !validDate(watered) || watered > store.Today() {
		writeErr(w, http.StatusBadRequest, "the last watering is YYYY-MM-DD, today or earlier")
		return
	}
	var lines []careInput
	if in.Care != nil {
		lines = *in.Care
	} else {
		for _, c := range careFor(p.Kind, p.Light) {
			n, u, soon := c.EveryN, c.EveryUnit, c.SoonDays
			lines = append(lines, careInput{Care: c.Care, EveryN: &n, EveryUnit: &u, SoonDays: &soon, Notes: &c.Notes})
		}
	}
	for i := range lines {
		if lines[i].Care == "water" && lines[i].LastDone == nil && watered != "" {
			w := watered
			lines[i].LastDone = &w
		}
	}
	// check the schedule before the plant exists
	for _, c := range lines {
		if c.EveryN != nil && *c.EveryN == 0 {
			continue
		}
		if _, msg := careLine(&p, c, nil); msg != "" {
			writeErr(w, http.StatusBadRequest, msg)
			return
		}
	}
	if err := s.store.CreatePlant(ctx, uid, &p); err != nil {
		s.fail(w, err, "add plant")
		return
	}
	if _, msg, err := s.saveCare(ctx, uid, &p, lines); msg != "" || err != nil {
		_, _ = s.store.DeletePlant(ctx, uid, p.ID)
		if msg != "" {
			writeErr(w, http.StatusBadRequest, msg)
		} else {
			s.fail(w, err, "add plant")
		}
		return
	}
	np, _ := s.store.GetPlant(ctx, uid, p.ID)
	s.syncCalendarSoon(uid)
	s.plantSaved(ctx, uid, np)
	writeJSON(w, http.StatusCreated, np)
}

func (s *Server) handleUpdatePlant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlant(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plant")
		return
	}
	var in plantInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := in.apply(p); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdatePlant(ctx, uid, p); err != nil {
		s.fail(w, err, "save plant")
		return
	}
	if in.Care != nil {
		removed, msg, err := s.saveCare(ctx, uid, p, *in.Care)
		if msg != "" {
			writeErr(w, http.StatusBadRequest, msg)
			return
		}
		if err != nil {
			s.fail(w, err, "save care")
			return
		}
		s.dropCalendarEvents(uid, removed)
	}
	np, _ := s.store.GetPlant(ctx, uid, p.ID)
	s.syncCalendarSoon(uid)
	s.plantSaved(ctx, uid, np)
	writeJSON(w, http.StatusOK, np)
}

// plantSaved tells the Core bus a plant was added or changed (or archived).
func (s *Server) plantSaved(ctx context.Context, uid int64, p *store.Plant) {
	if p == nil {
		return
	}
	s.publish(ctx, uid, "home.plant_saved", "plant:"+itoa(p.ID), map[string]any{"plant_id": p.ID, "name": p.Name, "species": p.Species, "kind": p.Kind,
		"home": p.HomeName, "room": p.Room, "light": p.Light, "archived": p.Archived, "gone_reason": p.GoneReason})
}

func (s *Server) handleDeletePlant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	jobs, err := s.store.DeletePlant(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "delete plant")
		return
	}
	s.dropCalendarEvents(uid, jobs)
	w.WriteHeader(http.StatusNoContent)
}

// plantDoneInput is "I watered it" (or another care) for one plant.
type plantDoneInput struct {
	doneInput
	Care string `json:"care"` // default water
}

// handlePlantDone marks one kind of care done for a plant ("Watered"). Care
// that is not on its schedule is just written in the log.
func (s *Server) handlePlantDone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlant(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plant")
		return
	}
	var in plantDoneInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	in.Care = firstNonEmpty(strings.TrimSpace(in.Care), "water")
	if !validCare(in.Care) {
		writeErr(w, http.StatusBadRequest, "care is water, fertilise, mist, rotate, prune, pests or repot")
		return
	}
	var job *store.Job
	for i := range p.Care {
		if p.Care[i].Care == in.Care && p.Care[i].FinishedAt == "" {
			job = &p.Care[i]
		}
	}
	if job != nil {
		out, code, msg := s.markDone(ctx, uid, job, in.doneInput)
		if msg != "" {
			writeErr(w, code, msg)
			return
		}
		out["plant"], _ = s.store.GetPlant(ctx, uid, p.ID)
		s.syncCalendarSoon(uid)
		writeJSON(w, http.StatusOK, out)
		return
	}
	// not on the schedule: only the log
	date := firstNonEmpty(in.Date, store.Today())
	if !validDate(date) || date > store.AddDays(store.Today(), 1) {
		writeErr(w, http.StatusBadRequest, "the date is YYYY-MM-DD, today or earlier")
		return
	}
	if in.Cost < 0 {
		writeErr(w, http.StatusBadRequest, "the cost can't be negative")
		return
	}
	cur := "EUR"
	if h, err := s.store.GetHome(ctx, uid, p.HomeID); err == nil {
		cur = h.Currency
	}
	rec := store.Record{HomeID: p.HomeID, PlantID: &p.ID, Date: date, Kind: "plant", Title: firstNonEmpty(strings.TrimSpace(in.Title), careDoneTitle(in.Care, p.Name)),
		Notes: strings.TrimSpace(in.Notes), Cost: in.Cost, Currency: cur, Source: firstNonEmpty(linkApp(ctx), "web"), ExternalID: strings.TrimSpace(in.ExternalID)}
	if err := s.store.CreateRecord(ctx, uid, &rec); err != nil {
		s.fail(w, err, "log care")
		return
	}
	nr, _ := s.store.GetRecord(ctx, uid, rec.ID)
	out := map[string]any{"job": nil, "record": nr}
	if nr.Cost > 0 && s.wantsMoney(ctx, uid, in.Money) {
		out["money"] = s.bookRecord(ctx, uid, nr, in.AccountID, in.CategoryID)
	}
	s.publish(ctx, uid, "home.record_added", "record:"+itoa(rec.ID), recordPayload(nr))
	s.publish(ctx, uid, "home.plant_cared", "plant:"+itoa(p.ID), map[string]any{"plant_id": p.ID, "plant": p.Name, "care": in.Care, "home": p.HomeName,
		"date": date, "record_id": rec.ID})
	out["plant"], _ = s.store.GetPlant(ctx, uid, p.ID)
	writeJSON(w, http.StatusOK, out)
}

// handlePlantsDone marks one kind of care (default water) done for several
// plants at once: the ones in plant_ids, or every plant whose care is due
// today or overdue ("Water all due").
func (s *Server) handlePlantsDone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Care     string  `json:"care"`
		PlantIDs []int64 `json:"plant_ids"`
		Date     string  `json:"date"`
		HomeID   int64   `json:"home_id"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	in.Care = firstNonEmpty(strings.TrimSpace(in.Care), "water")
	if !validCare(in.Care) {
		writeErr(w, http.StatusBadRequest, "care is water, fertilise, mist, rotate, prune, pests or repot")
		return
	}
	if len(in.PlantIDs) > 200 {
		writeErr(w, http.StatusBadRequest, "up to 200 plants at once")
		return
	}
	jobs, err := s.store.ListJobs(ctx, uid, store.JobFilter{HomeID: in.HomeID, Scope: "plants"})
	if err != nil {
		s.fail(w, err, "plants")
		return
	}
	want := map[int64]bool{}
	for _, id := range in.PlantIDs {
		want[id] = true
	}
	var todo []store.Job
	for _, j := range jobs {
		if j.Care != in.Care || j.Paused {
			continue
		}
		if len(want) > 0 {
			if want[*j.PlantID] {
				todo = append(todo, j)
				delete(want, *j.PlantID)
			}
		} else if j.Status == store.StatusOverdue || j.Status == store.StatusDue {
			todo = append(todo, j)
		}
	}
	if len(want) > 0 {
		// an id that is not the caller's plant (or has no such care)
		for id := range want {
			if _, err := s.store.GetPlant(ctx, uid, id); err != nil {
				s.fail(w, err, "plant")
				return
			}
		}
		writeErr(w, http.StatusBadRequest, "some of these plants have no "+careTitle(in.Care)+" on their schedule")
		return
	}
	done := []map[string]any{}
	for i := range todo {
		out, code, msg := s.markDone(ctx, uid, &todo[i], doneInput{Date: in.Date})
		if msg != "" {
			if len(done) == 0 {
				writeErr(w, code, msg)
				return
			}
			continue
		}
		next := todo[i].DueOn
		if nj, ok := out["job"].(*store.Job); ok && nj != nil {
			next = nj.DueOn
		}
		done = append(done, map[string]any{"plant_id": *todo[i].PlantID, "plant": todo[i].PlantName, "job_id": todo[i].ID, "next_due": next})
	}
	if len(done) > 0 {
		s.syncCalendarSoon(uid)
	}
	writeJSON(w, http.StatusOK, map[string]any{"care": in.Care, "count": len(done), "done": done})
}
