package server

import (
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

type meterInput struct {
	HomeID       *int64   `json:"home_id"`
	Name         *string  `json:"name"`
	Kind         *string  `json:"kind"`
	Unit         *string  `json:"unit"`
	PricePerUnit *float64 `json:"price_per_unit"`
	Number       *string  `json:"number"`
	Notes        *string  `json:"notes"`
	RemindDay    *int     `json:"remind_day"`
	Archived     *bool    `json:"archived"`
}

func (in meterInput) apply(m *store.Meter, keys map[string]bool) string {
	if in.HomeID != nil {
		m.HomeID = *in.HomeID
	}
	trimP(&m.Name, in.Name)
	trimP(&m.Kind, in.Kind)
	trimP(&m.Unit, in.Unit)
	trimP(&m.Number, in.Number)
	trimP(&m.Notes, in.Notes)
	if keys["price_per_unit"] {
		m.PricePerUnit = in.PricePerUnit
	}
	if in.RemindDay != nil {
		m.RemindDay = *in.RemindDay
	}
	if in.Archived != nil {
		m.Archived = *in.Archived
	}
	if m.Unit == "" {
		m.Unit = meterUnitsFor[m.Kind]
	}
	switch {
	case m.HomeID <= 0:
		return "choose the home"
	case !oneOf(m.Kind, meterKinds):
		return "kind is electricity, water, gas, heating, solar, oil or other"
	case m.Name == "" || len(m.Name) > 80:
		return "a meter needs a name (up to 80 characters)"
	case len(m.Unit) > 16:
		return "the unit is up to 16 characters"
	case m.PricePerUnit != nil && (*m.PricePerUnit < 0 || *m.PricePerUnit > 1e6):
		return "the price per unit looks wrong"
	case m.RemindDay < 0 || m.RemindDay > 31:
		return "the reading day is 1 to 31, or 0 for none"
	}
	return ""
}

func (s *Server) handleListMeters(w http.ResponseWriter, r *http.Request) {
	ms, err := s.store.ListMeters(r.Context(), actorID(r.Context()), int64(queryInt(r, "home_id")))
	if err != nil {
		s.fail(w, err, "meters")
		return
	}
	writeJSON(w, http.StatusOK, ms)
}

func (s *Server) handleGetMeter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	m, err := s.store.GetMeter(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "meter")
		return
	}
	rs, _ := s.store.ListReadings(ctx, uid, m.ID, 0)
	writeJSON(w, http.StatusOK, map[string]any{"meter": m, "readings": reverse(rs), "usage": store.MonthlyUsage(rs, m.PricePerUnit, 13, store.Today()), "summary": usageSummary(rs, m)})
}

func reverse[T any](in []T) []T {
	out := make([]T, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}

// usageSummary: average per day over the last year and the year before.
func usageSummary(rs []store.Reading, m *store.Meter) map[string]any {
	today := store.Today()
	yearAgo, twoAgo := store.AddDays(today, -365), store.AddDays(today, -730)
	var u1, u2 float64
	var d1, d2 int
	for _, r := range rs {
		if r.Usage == nil || r.Days <= 0 {
			continue
		}
		switch {
		case r.ReadOn > yearAgo:
			u1 += *r.Usage
			d1 += r.Days
		case r.ReadOn > twoAgo:
			u2 += *r.Usage
			d2 += r.Days
		}
	}
	out := map[string]any{"unit": m.Unit}
	if d1 > 0 {
		out["per_day"] = u1 / float64(d1)
		out["per_year"] = u1 / float64(d1) * 365
		if m.PricePerUnit != nil {
			out["cost_per_year"] = u1 / float64(d1) * 365 * *m.PricePerUnit
		}
	}
	if d2 > 0 {
		out["per_day_before"] = u2 / float64(d2)
	}
	return out
}

func (s *Server) handleCreateMeter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in meterInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	m := store.Meter{Kind: "electricity"}
	if in.HomeID == nil {
		if hs, _ := s.store.ListHomes(ctx, uid); len(hs) == 1 {
			m.HomeID = hs[0].ID
		}
	}
	if msg := in.apply(&m, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.CreateMeter(ctx, uid, &m); err != nil {
		s.fail(w, err, "create meter")
		return
	}
	nm, _ := s.store.GetMeter(ctx, uid, m.ID)
	writeJSON(w, http.StatusCreated, nm)
}

func (s *Server) handleUpdateMeter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	m, err := s.store.GetMeter(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "meter")
		return
	}
	var in meterInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := in.apply(m, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdateMeter(ctx, uid, m); err != nil {
		s.fail(w, err, "save meter")
		return
	}
	nm, _ := s.store.GetMeter(ctx, uid, m.ID)
	writeJSON(w, http.StatusOK, nm)
}

func (s *Server) handleDeleteMeter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteMeter(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete meter")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListReadings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	if _, err := s.store.GetMeter(ctx, uid, id); err != nil {
		s.fail(w, err, "meter")
		return
	}
	rs, err := s.store.ListReadings(ctx, uid, id, queryInt(r, "limit"))
	if err != nil {
		s.fail(w, err, "readings")
		return
	}
	writeJSON(w, http.StatusOK, reverse(rs))
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	m, err := s.store.GetMeter(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "meter")
		return
	}
	months := queryInt(r, "months")
	if months <= 0 || months > 60 {
		months = 12
	}
	rs, _ := s.store.ListReadings(ctx, uid, m.ID, 0)
	writeJSON(w, http.StatusOK, map[string]any{"meter": m, "months": store.MonthlyUsage(rs, m.PricePerUnit, months, store.Today()), "summary": usageSummary(rs, m)})
}

type readingInput struct {
	ReadOn *string  `json:"read_on"`
	Value  *float64 `json:"value"`
	Reset  *bool    `json:"reset"`
	Note   *string  `json:"note"`
}

func (s *Server) handleCreateReading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	m, err := s.store.GetMeter(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "meter")
		return
	}
	var in readingInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	rd := store.Reading{MeterID: m.ID, ReadOn: store.Today(), Source: firstNonEmpty(linkApp(ctx), "web")}
	if in.ReadOn != nil && *in.ReadOn != "" {
		rd.ReadOn = *in.ReadOn
	}
	if in.Value == nil || !validDate(rd.ReadOn) || rd.ReadOn > store.AddDays(store.Today(), 1) {
		writeErr(w, http.StatusBadRequest, "a reading needs a value and a date (YYYY-MM-DD, not in the future)")
		return
	}
	rd.Value = *in.Value
	if in.Reset != nil {
		rd.Reset = *in.Reset
	}
	if in.Note != nil {
		rd.Note = strings.TrimSpace(*in.Note)
	}
	if !rd.Reset && m.Last != nil && m.Last.ReadOn <= rd.ReadOn && rd.Value < m.Last.Value {
		writeErr(w, http.StatusBadRequest, "this is lower than the last reading; tick “New meter” if the meter was replaced")
		return
	}
	if err := s.store.CreateReading(ctx, uid, &rd); err != nil {
		s.fail(w, err, "add reading")
		return
	}
	s.publish(ctx, uid, "home.reading_added", "meter:"+itoa(m.ID), map[string]any{"meter_id": m.ID, "meter": m.Name, "kind": m.Kind, "home": m.HomeName,
		"date": rd.ReadOn, "value": rd.Value, "unit": m.Unit})
	nm, _ := s.store.GetMeter(ctx, uid, m.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"reading": rd, "meter": nm})
}

func (s *Server) handleUpdateReading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	rd, err := s.store.GetReading(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "reading")
		return
	}
	var in readingInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.ReadOn != nil {
		rd.ReadOn = *in.ReadOn
	}
	if in.Value != nil {
		rd.Value = *in.Value
	}
	if in.Reset != nil {
		rd.Reset = *in.Reset
	}
	if in.Note != nil {
		rd.Note = strings.TrimSpace(*in.Note)
	}
	if !validDate(rd.ReadOn) || rd.ReadOn == "" {
		writeErr(w, http.StatusBadRequest, "the date is YYYY-MM-DD")
		return
	}
	if err := s.store.UpdateReading(ctx, uid, rd); err != nil {
		s.fail(w, err, "save reading")
		return
	}
	writeJSON(w, http.StatusOK, rd)
}

func (s *Server) handleDeleteReading(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteReading(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete reading")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
