package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

type jobInput struct {
	HomeID    *int64  `json:"home_id"`
	ThingID   *int64  `json:"thing_id"`
	PlantID   *int64  `json:"plant_id"`
	Care      *string `json:"care"`
	ProID     *int64  `json:"pro_id"`
	Title     *string `json:"title"`
	Notes     *string `json:"notes"`
	EveryN    *int    `json:"every_n"`
	EveryUnit *string `json:"every_unit"`
	DueOn     *string `json:"due_on"`
	SoonDays  *int    `json:"soon_days"`
	EstCost   *int64  `json:"est_cost"`
	Paused    *bool   `json:"paused"`
	LastDone  *string `json:"last_done"`
}

func (in jobInput) apply(j *store.Job, keys map[string]bool) string {
	if in.HomeID != nil {
		j.HomeID = *in.HomeID
	}
	if keys["thing_id"] {
		j.ThingID = in.ThingID
	}
	if keys["plant_id"] {
		j.PlantID = in.PlantID
		if j.PlantID == nil {
			j.Care = ""
		}
	}
	trimP(&j.Care, in.Care)
	if keys["pro_id"] {
		j.ProID = in.ProID
	}
	trimP(&j.Title, in.Title)
	if j.Title == "" && j.Care != "" {
		j.Title = careTitle(j.Care)
	}
	trimP(&j.Notes, in.Notes)
	if in.EveryN != nil {
		j.EveryN = *in.EveryN
	}
	if in.EveryUnit != nil {
		j.EveryUnit = *in.EveryUnit
	}
	trimP(&j.DueOn, in.DueOn)
	trimP(&j.LastDone, in.LastDone)
	if in.SoonDays != nil {
		j.SoonDays = *in.SoonDays
	}
	if keys["est_cost"] {
		j.EstCost = in.EstCost
	}
	if in.Paused != nil {
		j.Paused = *in.Paused
	}
	switch {
	case j.HomeID <= 0:
		return "choose the home"
	case j.Care != "" && (!validCare(j.Care) || j.PlantID == nil):
		return "care is water, fertilise, mist, rotate, prune, pests or repot, for a plant"
	case j.Title == "" || len(j.Title) > 160:
		return "upkeep needs a title (up to 160 characters)"
	case j.EveryN < 0 || j.EveryN > 1000:
		return "repeat every 1 to 1000 units, or 0 for once"
	case !store.ValidUnit(j.EveryUnit):
		return "the repeat unit is day, week, month or year"
	case j.DueOn == "" || !validDate(j.DueOn):
		return "the due date is YYYY-MM-DD"
	case !validDate(j.LastDone):
		return "the last done date is YYYY-MM-DD"
	case j.SoonDays < 0 || j.SoonDays > 365:
		return "remind 0 to 365 days before"
	case j.EstCost != nil && *j.EstCost < 0:
		return "the cost can't be negative"
	}
	return ""
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	js, err := s.store.ListJobs(r.Context(), actorID(r.Context()), store.JobFilter{HomeID: int64(queryInt(r, "home_id")), ThingID: int64(queryInt(r, "thing_id")),
		PlantID: int64(queryInt(r, "plant_id")), Scope: scopeParam(r), Q: strings.TrimSpace(r.URL.Query().Get("q")), All: queryBool(r, "all")})
	if err != nil {
		s.fail(w, err, "upkeep")
		return
	}
	writeJSON(w, http.StatusOK, js)
}

// handleDue is what needs doing: overdue, due today, due soon, and the next
// ?days (default 30) — the briefing's maintenance section.
func (s *Server) handleDue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	days := queryInt(r, "days")
	if days <= 0 || days > 366 {
		days = 30
	}
	today := store.Today()
	js, err := s.store.ListJobs(ctx, actorID(ctx), store.JobFilter{HomeID: int64(queryInt(r, "home_id")), Scope: scopeParam(r), Until: store.AddDays(today, days)})
	if err != nil {
		s.fail(w, err, "upkeep")
		return
	}
	out := map[string][]store.Job{"overdue": {}, "due": {}, "soon": {}, "upcoming": {}}
	for _, j := range js {
		switch j.Status {
		case store.StatusOverdue, store.StatusDue, store.StatusSoon:
			out[j.Status] = append(out[j.Status], j)
		default:
			out["upcoming"] = append(out["upcoming"], j)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"today": today, "days": days, "overdue": out["overdue"], "due": out["due"], "soon": out["soon"], "upcoming": out["upcoming"],
		"counts": map[string]int{"overdue": len(out["overdue"]), "due": len(out["due"]), "soon": len(out["soon"]), "upcoming": len(out["upcoming"])}})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	j, err := s.store.GetJob(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "upkeep")
		return
	}
	recs, _, _ := s.store.ListRecords(ctx, uid, store.RecordFilter{JobID: j.ID, Limit: 50})
	out := map[string]any{"job": j, "records": recs, "task_url": "", "calendar_url": ""}
	if j.TaskID != nil {
		out["task_url"] = s.siblingLink(ctx, uid, appTasks, "/tasks/"+itoa(*j.TaskID))
	}
	if j.CalEventID != nil {
		out["calendar_url"] = s.siblingLink(ctx, uid, appCalendar, "/?date="+j.DueOn)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) newJob(ctx context.Context, uid int64, in jobInput, keys map[string]bool) (*store.Job, string, error) {
	j := store.Job{EveryUnit: "month", SoonDays: 14, Source: firstNonEmpty(linkApp(ctx), "web")}
	if in.ThingID != nil && in.HomeID == nil {
		if t, err := s.store.GetThing(ctx, uid, *in.ThingID); err == nil {
			j.HomeID = t.HomeID
		}
	}
	if in.PlantID != nil && in.HomeID == nil {
		if p, err := s.store.GetPlant(ctx, uid, *in.PlantID); err == nil {
			j.HomeID = p.HomeID
		}
	}
	if in.HomeID == nil && j.HomeID == 0 {
		if hs, _ := s.store.ListHomes(ctx, uid); len(hs) == 1 {
			j.HomeID = hs[0].ID
		}
	}
	if in.DueOn == nil || *in.DueOn == "" {
		j.DueOn = store.Today()
		if in.LastDone != nil && validDate(*in.LastDone) && *in.LastDone != "" && in.EveryN != nil && *in.EveryN > 0 {
			j.DueOn = store.AddInterval(*in.LastDone, *in.EveryN, firstNonEmpty(strPtr(in.EveryUnit), "month"))
		}
	}
	if msg := in.apply(&j, keys); msg != "" {
		return nil, msg, nil
	}
	if err := s.store.CreateJob(ctx, uid, &j); err != nil {
		return nil, "", err
	}
	nj, err := s.store.GetJob(ctx, uid, j.ID)
	return nj, "", err
}

func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in jobInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	j, msg, err := s.newJob(ctx, uid, in, keys)
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err != nil {
		s.fail(w, err, "create upkeep")
		return
	}
	s.syncCalendarSoon(uid)
	writeJSON(w, http.StatusCreated, j)
}

// handleCreateJobs adds several jobs at once (suggestions picked for a thing).
func (s *Server) handleCreateJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Jobs []map[string]any `json:"jobs"`
	}
	if err := decode(r, &in); err != nil || len(in.Jobs) == 0 || len(in.Jobs) > 50 {
		writeErr(w, http.StatusBadRequest, "send 1 to 50 jobs")
		return
	}
	out := []store.Job{}
	for _, raw := range in.Jobs {
		var ji jobInput
		keys := map[string]bool{}
		for k := range raw {
			keys[k] = true
		}
		if err := remarshal(raw, &ji); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid job")
			return
		}
		j, msg, err := s.newJob(ctx, uid, ji, keys)
		if msg != "" {
			writeErr(w, http.StatusBadRequest, msg)
			return
		}
		if err != nil {
			s.fail(w, err, "create upkeep")
			return
		}
		out = append(out, *j)
	}
	s.syncCalendarSoon(uid)
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	j, err := s.store.GetJob(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "upkeep")
		return
	}
	var in jobInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.ThingID != nil && in.HomeID == nil {
		if t, err := s.store.GetThing(ctx, uid, *in.ThingID); err == nil {
			j.HomeID = t.HomeID
		}
	}
	if in.PlantID != nil && in.HomeID == nil {
		if p, err := s.store.GetPlant(ctx, uid, *in.PlantID); err == nil {
			j.HomeID = p.HomeID
		}
	}
	if msg := in.apply(j, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdateJob(ctx, uid, j); err != nil {
		s.fail(w, err, "save upkeep")
		return
	}
	_ = s.store.SetJobCalendar(ctx, uid, j.ID, j.CalEventID, "")
	s.syncCalendarSoon(uid)
	nj, _ := s.store.GetJob(ctx, uid, j.ID)
	writeJSON(w, http.StatusOK, nj)
}

func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	j, err := s.store.GetJob(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "upkeep")
		return
	}
	if err := s.store.DeleteJob(ctx, uid, j.ID); err != nil {
		s.fail(w, err, "delete upkeep")
		return
	}
	s.dropCalendarEvents(uid, []store.Job{*j})
	w.WriteHeader(http.StatusNoContent)
}

// doneInput is "I did it": when, and what to write in the log.
type doneInput struct {
	Date       string  `json:"date"`
	Log        *bool   `json:"log"` // default true
	Title      string  `json:"title"`
	Kind       string  `json:"kind"`
	Notes      string  `json:"notes"`
	Cost       int64   `json:"cost"`
	ProID      *int64  `json:"pro_id"`
	Money      *bool   `json:"money"` // book the cost in Pocket Money (default: the setting)
	AccountID  int64   `json:"account_id"`
	CategoryID int64   `json:"category_id"`
	ExternalID string  `json:"external_id"`
	Reading    *string `json:"-"`
}

// handleJobDone marks a job done: a log entry (with its cost booked in
// Pocket Money when asked) and, for upkeep that comes back, the next date.
func (s *Server) handleJobDone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	j, err := s.store.GetJob(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "upkeep")
		return
	}
	var in doneInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	out, code, msg := s.markDone(ctx, uid, j, in)
	if msg != "" {
		writeErr(w, code, msg)
		return
	}
	s.syncCalendarSoon(uid)
	writeJSON(w, http.StatusOK, out)
}

// markDone does one job: the log entry (plant care is logged as "Watered
// Monstera", kind plant), the next due date and the events. A message comes
// back with the HTTP status when it can't be done.
func (s *Server) markDone(ctx context.Context, uid int64, j *store.Job, in doneInput) (map[string]any, int, string) {
	if in.Date == "" {
		in.Date = store.Today()
	}
	if !validDate(in.Date) || in.Date > store.AddDays(store.Today(), 1) {
		return nil, http.StatusBadRequest, "the date is YYYY-MM-DD, today or earlier"
	}
	if j.FinishedAt != "" {
		return nil, http.StatusConflict, "this was done already"
	}
	var rec *store.Record
	if in.Log == nil || *in.Log {
		kind, title := firstNonEmpty(in.Kind, "maintenance"), firstNonEmpty(strings.TrimSpace(in.Title), j.Title)
		if j.PlantID != nil {
			kind, title = firstNonEmpty(in.Kind, "plant"), firstNonEmpty(strings.TrimSpace(in.Title), careDoneTitle(j.Care, j.PlantName))
		}
		if !oneOf(kind, recordKinds) || in.Cost < 0 {
			return nil, http.StatusBadRequest, "unknown kind, or a negative cost"
		}
		pro := in.ProID
		if pro == nil {
			pro = j.ProID
		}
		rec = &store.Record{HomeID: j.HomeID, ThingID: j.ThingID, PlantID: j.PlantID, JobID: &j.ID, ProID: pro, Date: in.Date, Kind: kind,
			Title: title, Notes: strings.TrimSpace(in.Notes), Cost: in.Cost, Currency: j.Currency,
			Source: firstNonEmpty(linkApp(ctx), "web"), ExternalID: strings.TrimSpace(in.ExternalID)}
	}
	if err := s.store.CompleteJob(ctx, uid, j, in.Date, rec); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, http.StatusNotFound, "not found"
		}
		if errors.Is(err, store.ErrConflict) {
			return nil, http.StatusConflict, "already exists"
		}
		return nil, http.StatusInternalServerError, "mark done: " + err.Error()
	}
	out := map[string]any{"job": j}
	if rec != nil {
		nr, _ := s.store.GetRecord(ctx, uid, rec.ID)
		out["record"] = nr
		if rec.Cost > 0 && s.wantsMoney(ctx, uid, in.Money) {
			out["money"] = s.bookRecord(ctx, uid, nr, in.AccountID, in.CategoryID)
		}
		s.publish(ctx, uid, "home.record_added", "record:"+itoa(rec.ID), recordPayload(nr))
	}
	nj, _ := s.store.GetJob(ctx, uid, j.ID)
	out["job"] = nj
	if j.PlantID != nil {
		ev := map[string]any{"plant_id": *j.PlantID, "plant": j.PlantName, "care": j.Care, "job_id": j.ID, "home": j.HomeName, "date": in.Date, "next_due": nj.DueOn}
		if rec != nil {
			ev["record_id"] = rec.ID
		}
		s.publish(ctx, uid, "home.plant_cared", "plant:"+itoa(*j.PlantID), ev)
	} else {
		s.publish(ctx, uid, "home.job_done", "job:"+itoa(j.ID), map[string]any{"job_id": j.ID, "title": j.Title, "thing": j.ThingName, "home": j.HomeName,
			"date": in.Date, "next_due": nj.DueOn, "recurring": j.Recurring(), "cost": in.Cost, "currency": j.Currency})
	}
	return out, http.StatusOK, ""
}

// handleJobSkip moves recurring upkeep on by one interval without a log entry.
func (s *Server) handleJobSkip(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	j, err := s.store.GetJob(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "upkeep")
		return
	}
	if !j.Recurring() {
		writeErr(w, http.StatusBadRequest, "only upkeep that comes back can be skipped")
		return
	}
	j.DueOn = store.AddInterval(j.DueOn, j.EveryN, j.EveryUnit)
	for j.DueOn < store.Today() {
		j.DueOn = store.AddInterval(j.DueOn, j.EveryN, j.EveryUnit)
	}
	if err := s.store.UpdateJob(ctx, uid, j); err != nil {
		s.fail(w, err, "skip")
		return
	}
	_ = s.store.SetJobCalendar(ctx, uid, j.ID, j.CalEventID, "")
	s.syncCalendarSoon(uid)
	nj, _ := s.store.GetJob(ctx, uid, j.ID)
	writeJSON(w, http.StatusOK, nj)
}

// handleJobSnooze moves the due date by ?days (from today when overdue).
func (s *Server) handleJobSnooze(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	j, err := s.store.GetJob(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "upkeep")
		return
	}
	var in struct {
		Days int `json:"days"`
	}
	if err := decode(r, &in); err != nil || in.Days < 1 || in.Days > 365 {
		writeErr(w, http.StatusBadRequest, "snooze 1 to 365 days")
		return
	}
	from := j.DueOn
	if from < store.Today() {
		from = store.Today()
	}
	j.DueOn = store.AddDays(from, in.Days)
	if err := s.store.UpdateJob(ctx, uid, j); err != nil {
		s.fail(w, err, "snooze")
		return
	}
	_ = s.store.SetJobCalendar(ctx, uid, j.ID, j.CalEventID, "")
	s.syncCalendarSoon(uid)
	nj, _ := s.store.GetJob(ctx, uid, j.ID)
	writeJSON(w, http.StatusOK, nj)
}

// handleJobTask adds the job to the person's Pocket Tasks.
func (s *Server) handleJobTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	j, err := s.store.GetJob(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "upkeep")
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := s.clients(cctx, uid).conn(cctx, appTasks, scopeTasksWrite)
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	notes := strings.TrimSpace(j.Notes + "\n\n" + fmt.Sprintf("From Pocket Home (%s).", j.HomeName))
	t, err := conn.client.CreateTask(cctx, jobName(*j), notes, j.DueOn, s.jobURL(*j))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "Pocket Tasks: "+err.Error())
		return
	}
	_ = s.store.SetJobTask(ctx, uid, j.ID, &t.ID)
	writeJSON(w, http.StatusOK, map[string]any{"task_id": t.ID, "task_url": strings.TrimRight(conn.public, "/") + "/tasks/" + itoa(t.ID), "via": conn.via})
}

// scopeParam is ?scope=house|plants ("" for everything).
func scopeParam(r *http.Request) string {
	if v := r.URL.Query().Get("scope"); v == "house" || v == "plants" {
		return v
	}
	return ""
}

// jobName is how a job reads outside its page: "Gas boiler: Annual
// service", "Monstera: Water".
func jobName(j store.Job) string {
	of := j.ThingName
	if j.PlantName != "" {
		of = j.PlantName
	}
	if of != "" && !strings.Contains(strings.ToLower(j.Title), strings.ToLower(of)) {
		return of + ": " + j.Title
	}
	return j.Title
}

// jobURL is the job's page: the plant's for plant care.
func (s *Server) jobURL(j store.Job) string {
	if j.PlantID != nil {
		return s.cfg.BaseURL + "/plants/" + itoa(*j.PlantID)
	}
	return s.cfg.BaseURL + "/upkeep/" + itoa(j.ID)
}
