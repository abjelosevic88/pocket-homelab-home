package server

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// The kinds the API accepts; the UI names them.
var (
	homeKinds     = []string{"house", "apartment", "cottage", "other"}
	thingKinds    = []string{"heating", "cooling", "water", "electrical", "appliance", "kitchen", "structure", "outdoor", "safety", "other"}
	recordKinds   = []string{"maintenance", "repair", "improvement", "inspection", "purchase", "plant", "other"}
	meterKinds    = []string{"electricity", "water", "gas", "heating", "solar", "oil", "other"}
	attachKinds   = []string{"document", "url"}
	meterUnitsFor = map[string]string{"electricity": "kWh", "water": "m³", "gas": "m³", "heating": "kWh", "solar": "kWh", "oil": "L", "other": ""}
)

func oneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// cleanURL keeps http(s) links only ("" stays "").
func cleanURL(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", true
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	return v, true
}

func cleanCurrency(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if len(v) != 3 {
		return ""
	}
	for _, c := range v {
		if c < 'A' || c > 'Z' {
			return ""
		}
	}
	return v
}

// ---------- homes ----------

type homeInput struct {
	Name      *string  `json:"name"`
	Kind      *string  `json:"kind"`
	Address   *string  `json:"address"`
	YearBuilt *int     `json:"year_built"`
	AreaM2    *float64 `json:"area_m2"`
	Currency  *string  `json:"currency"`
	Notes     *string  `json:"notes"`
	MoneyTag  *string  `json:"money_tag"`
	Archived  *bool    `json:"archived"`
	Sort      *int     `json:"sort"`
}

func (in homeInput) apply(h *store.Home, keys map[string]bool) string {
	if in.Name != nil {
		h.Name = strings.TrimSpace(*in.Name)
	}
	if in.Kind != nil {
		h.Kind = *in.Kind
	}
	if in.Address != nil {
		h.Address = strings.TrimSpace(*in.Address)
	}
	if keys["year_built"] {
		h.YearBuilt = in.YearBuilt
	}
	if keys["area_m2"] {
		h.AreaM2 = in.AreaM2
	}
	if in.Currency != nil {
		h.Currency = cleanCurrency(*in.Currency)
	}
	if in.Notes != nil {
		h.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.MoneyTag != nil {
		h.MoneyTag = strings.TrimSpace(*in.MoneyTag)
	}
	if in.Archived != nil {
		h.Archived = *in.Archived
	}
	if in.Sort != nil {
		h.Sort = *in.Sort
	}
	switch {
	case h.Name == "" || len(h.Name) > 120:
		return "a home needs a name (up to 120 characters)"
	case !oneOf(h.Kind, homeKinds):
		return "kind is house, apartment, cottage or other"
	case h.Currency == "":
		return "currency is a three-letter code like EUR"
	case h.YearBuilt != nil && (*h.YearBuilt < 1000 || *h.YearBuilt > time.Now().Year()+5):
		return "the year it was built looks wrong"
	case h.AreaM2 != nil && (*h.AreaM2 <= 0 || *h.AreaM2 > 1e6):
		return "the area looks wrong"
	}
	return ""
}

func (s *Server) handleListHomes(w http.ResponseWriter, r *http.Request) {
	hs, err := s.store.ListHomes(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "homes")
		return
	}
	if hs == nil {
		hs = []store.Home{}
	}
	writeJSON(w, http.StatusOK, hs)
}

func (s *Server) handleGetHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	h, err := s.store.GetHome(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "home")
		return
	}
	if hs, err := s.store.ListHomes(ctx, uid); err == nil {
		for _, x := range hs {
			if x.ID == h.ID {
				h = &x
			}
		}
	}
	atts, _ := s.store.ListAttachments(ctx, uid, "home", h.ID)
	writeJSON(w, http.StatusOK, map[string]any{"home": h, "attachments": s.attachmentViews(ctx, uid, atts)})
}

func (s *Server) handleCreateHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in homeInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st, _ := s.store.GetSettings(ctx, uid)
	h := store.Home{Kind: "house", Currency: st.Currency}
	if msg := in.apply(&h, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.CreateHome(ctx, uid, &h); err != nil {
		s.fail(w, err, "create home")
		return
	}
	writeJSON(w, http.StatusCreated, h)
}

func (s *Server) handleUpdateHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	h, err := s.store.GetHome(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "home")
		return
	}
	var in homeInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := in.apply(h, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdateHome(ctx, uid, h); err != nil {
		s.fail(w, err, "save home")
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) handleDeleteHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	if err := s.store.OwnsHome(ctx, uid, id); err != nil {
		s.fail(w, err, "home")
		return
	}
	jobs, _ := s.store.ListJobs(ctx, uid, store.JobFilter{HomeID: id, All: true})
	if err := s.store.DeleteHome(ctx, uid, id); err != nil {
		s.fail(w, err, "delete home")
		return
	}
	s.dropCalendarEvents(uid, jobs)
	w.WriteHeader(http.StatusNoContent)
}

// ---------- things ----------

type thingInput struct {
	HomeID        *int64  `json:"home_id"`
	Name          *string `json:"name"`
	Category      *string `json:"category"`
	Room          *string `json:"room"`
	Brand         *string `json:"brand"`
	Model         *string `json:"model"`
	Serial        *string `json:"serial"`
	InstalledOn   *string `json:"installed_on"`
	PurchasedOn   *string `json:"purchased_on"`
	Price         *int64  `json:"price"`
	Vendor        *string `json:"vendor"`
	WarrantyUntil *string `json:"warranty_until"`
	ManualURL     *string `json:"manual_url"`
	InventoryID   *int64  `json:"inventory_id"`
	InventoryName *string `json:"inventory_name"`
	Notes         *string `json:"notes"`
	Archived      *bool   `json:"archived"`
}

func trimP(dst *string, v *string) {
	if v != nil {
		*dst = strings.TrimSpace(*v)
	}
}

func (in thingInput) apply(t *store.Thing, keys map[string]bool) string {
	if in.HomeID != nil {
		t.HomeID = *in.HomeID
	}
	trimP(&t.Name, in.Name)
	trimP(&t.Category, in.Category)
	trimP(&t.Room, in.Room)
	trimP(&t.Brand, in.Brand)
	trimP(&t.Model, in.Model)
	trimP(&t.Serial, in.Serial)
	trimP(&t.InstalledOn, in.InstalledOn)
	trimP(&t.PurchasedOn, in.PurchasedOn)
	trimP(&t.Vendor, in.Vendor)
	trimP(&t.WarrantyUntil, in.WarrantyUntil)
	trimP(&t.Notes, in.Notes)
	trimP(&t.InventoryName, in.InventoryName)
	if keys["price"] {
		t.Price = in.Price
	}
	if keys["inventory_id"] {
		t.InventoryID = in.InventoryID
		if in.InventoryID == nil {
			t.InventoryName = ""
		}
	}
	if in.Archived != nil {
		t.Archived = *in.Archived
	}
	if in.ManualURL != nil {
		u, ok := cleanURL(*in.ManualURL)
		if !ok {
			return "the manual link must start with http:// or https://"
		}
		t.ManualURL = u
	}
	switch {
	case t.HomeID <= 0:
		return "choose the home it is in"
	case t.Name == "" || len(t.Name) > 120:
		return "a thing needs a name (up to 120 characters)"
	case !oneOf(t.Category, thingKinds):
		return "unknown category"
	case !validDate(t.InstalledOn) || !validDate(t.PurchasedOn) || !validDate(t.WarrantyUntil):
		return "dates are YYYY-MM-DD"
	case t.Price != nil && *t.Price < 0:
		return "the price can't be negative"
	}
	return ""
}

func (s *Server) handleListThings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ts, err := s.store.ListThings(r.Context(), actorID(r.Context()), store.ThingFilter{HomeID: int64(queryInt(r, "home_id")), Category: q.Get("category"),
		Room: q.Get("room"), Q: strings.TrimSpace(q.Get("q")), Archived: queryBool(r, "archived")})
	if err != nil {
		s.fail(w, err, "things")
		return
	}
	if ts == nil {
		ts = []store.Thing{}
	}
	writeJSON(w, http.StatusOK, ts)
}

func (s *Server) handleGetThing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	t, err := s.store.GetThing(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "thing")
		return
	}
	jobs, _ := s.store.ListJobs(ctx, uid, store.JobFilter{ThingID: t.ID, All: true})
	recs, _, _ := s.store.ListRecords(ctx, uid, store.RecordFilter{ThingID: t.ID, Limit: 100})
	atts, _ := s.store.ListAttachments(ctx, uid, "thing", t.ID)
	writeJSON(w, http.StatusOK, map[string]any{"thing": t, "jobs": jobs, "records": recs, "attachments": s.attachmentViews(ctx, uid, atts),
		"warranty": warrantyState(t.WarrantyUntil, store.Today()), "suggestions": templatesFor(t)})
}

// warrantyState is "" (none), "active", "ending" (within 90 days) or "ended".
func warrantyState(until, today string) string {
	switch {
	case until == "":
		return ""
	case until < today:
		return "ended"
	case store.DaysBetween(today, until) <= 90:
		return "ending"
	}
	return "active"
}

func (s *Server) handleCreateThing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in thingInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	t := store.Thing{Category: "appliance"}
	if t.HomeID == 0 && in.HomeID == nil {
		if hs, _ := s.store.ListHomes(ctx, uid); len(hs) == 1 {
			t.HomeID = hs[0].ID
		}
	}
	if msg := in.apply(&t, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.CreateThing(ctx, uid, &t); err != nil {
		s.fail(w, err, "create thing")
		return
	}
	nt, _ := s.store.GetThing(ctx, uid, t.ID)
	s.thingSaved(ctx, uid, nt)
	writeJSON(w, http.StatusCreated, nt)
}

func (s *Server) handleUpdateThing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	t, err := s.store.GetThing(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "thing")
		return
	}
	var in thingInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := in.apply(t, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdateThing(ctx, uid, t); err != nil {
		s.fail(w, err, "save thing")
		return
	}
	nt, _ := s.store.GetThing(ctx, uid, t.ID)
	s.syncCalendarSoon(uid)
	s.thingSaved(ctx, uid, nt)
	writeJSON(w, http.StatusOK, nt)
}

// thingSaved tells the Core bus a thing was created or changed, so apps such
// as Pocket Renewals can pick up its warranty right away.
func (s *Server) thingSaved(ctx context.Context, uid int64, t *store.Thing) {
	if t == nil {
		return
	}
	s.publish(ctx, uid, "home.thing_saved", "thing:"+itoa(t.ID), map[string]any{"thing_id": t.ID, "name": t.Name, "home": t.HomeName, "warranty_until": t.WarrantyUntil})
}

func (s *Server) handleDeleteThing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	jobs, err := s.store.DeleteThing(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "delete thing")
		return
	}
	s.dropCalendarEvents(uid, jobs)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := s.store.Rooms(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "rooms")
		return
	}
	writeJSON(w, http.StatusOK, rooms)
}

// handleTemplates lists the built-in upkeep for a thing (?thing_id) or a
// category and name (?category&name).
func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t := &store.Thing{Category: r.URL.Query().Get("category"), Name: r.URL.Query().Get("name")}
	if id := int64(queryInt(r, "thing_id")); id > 0 {
		var err error
		if t, err = s.store.GetThing(ctx, actorID(ctx), id); err != nil {
			s.fail(w, err, "thing")
			return
		}
	}
	writeJSON(w, http.StatusOK, templatesFor(t))
}

// ---------- pros ----------

type proInput struct {
	Name      *string `json:"name"`
	Trade     *string `json:"trade"`
	Company   *string `json:"company"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"`
	Website   *string `json:"website"`
	Notes     *string `json:"notes"`
	Rating    *int    `json:"rating"`
	ContactID *int64  `json:"contact_id"`
}

func (in proInput) apply(p *store.Pro, keys map[string]bool) string {
	trimP(&p.Name, in.Name)
	trimP(&p.Trade, in.Trade)
	trimP(&p.Company, in.Company)
	trimP(&p.Phone, in.Phone)
	trimP(&p.Email, in.Email)
	trimP(&p.Notes, in.Notes)
	if in.Rating != nil {
		p.Rating = *in.Rating
	}
	if keys["contact_id"] {
		p.ContactID = in.ContactID
	}
	if in.Website != nil {
		u, ok := cleanURL(*in.Website)
		if !ok && strings.TrimSpace(*in.Website) != "" {
			u, ok = cleanURL("https://" + strings.TrimSpace(*in.Website))
		}
		if !ok {
			return "the website must be a web address"
		}
		p.Website = u
	}
	switch {
	case p.Name == "" || len(p.Name) > 120:
		return "a pro needs a name (up to 120 characters)"
	case p.Rating < 0 || p.Rating > 5:
		return "the rating is 0 to 5"
	}
	return ""
}

func (s *Server) handleListPros(w http.ResponseWriter, r *http.Request) {
	ps, err := s.store.ListPros(r.Context(), actorID(r.Context()), strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		s.fail(w, err, "pros")
		return
	}
	if ps == nil {
		ps = []store.Pro{}
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *Server) handleGetPro(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPro(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "pro")
		return
	}
	recs, _, _ := s.store.ListRecords(ctx, uid, store.RecordFilter{ProID: p.ID, Limit: 100})
	out := map[string]any{"pro": p, "records": recs, "contact_url": ""}
	if p.ContactID != nil {
		out["contact_url"] = s.siblingLink(ctx, uid, appContacts, "/contacts/"+itoa(*p.ContactID))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreatePro(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in proInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	var p store.Pro
	if msg := in.apply(&p, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.CreatePro(ctx, actorID(ctx), &p); err != nil {
		s.fail(w, err, "create pro")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdatePro(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPro(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "pro")
		return
	}
	var in proInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := in.apply(p, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdatePro(ctx, uid, p); err != nil {
		s.fail(w, err, "save pro")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeletePro(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeletePro(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete pro")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- attachments ----------

// attachmentView is an attachment with the link to open it.
type attachmentView struct {
	store.Attachment
	Open string `json:"open_url"`
}

func (s *Server) attachmentViews(ctx context.Context, uid int64, atts []store.Attachment) []attachmentView {
	out := make([]attachmentView, 0, len(atts))
	var docs string
	for _, a := range atts {
		v := attachmentView{Attachment: a, Open: a.URL}
		if a.Kind == "document" && a.Ref != "" {
			if docs == "" {
				docs = s.siblingLink(ctx, uid, appDocuments, "")
			}
			if docs != "" {
				v.Open = docs + "/documents/" + url.PathEscape(a.Ref)
			}
		}
		out = append(out, v)
	}
	return out
}

func (s *Server) handleCreateAttachment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var a store.Attachment
	if err := decode(r, &a); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	a.Title = strings.TrimSpace(a.Title)
	if !store.ValidOwnerKind(a.OwnerKind) || !oneOf(a.Kind, attachKinds) {
		writeErr(w, http.StatusBadRequest, "attach to a home, a thing, a plant or a log entry; a document or a link")
		return
	}
	if a.Kind == "url" {
		u, ok := cleanURL(a.URL)
		if !ok || u == "" {
			writeErr(w, http.StatusBadRequest, "the link must start with http:// or https://")
			return
		}
		a.URL, a.Ref = u, ""
		if a.Title == "" {
			if pu, err := url.Parse(u); err == nil {
				a.Title = pu.Host
			}
		}
	} else {
		a.Ref = strings.TrimSpace(a.Ref)
		if a.Ref == "" {
			writeErr(w, http.StatusBadRequest, "choose a document")
			return
		}
		a.URL = ""
	}
	if a.Title == "" || len(a.Title) > 200 {
		writeErr(w, http.StatusBadRequest, "give it a title (up to 200 characters)")
		return
	}
	if err := s.store.CreateAttachment(ctx, uid, &a); err != nil {
		s.fail(w, err, "attach")
		return
	}
	writeJSON(w, http.StatusCreated, s.attachmentViews(ctx, uid, []store.Attachment{a})[0])
}

func (s *Server) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteAttachment(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "remove")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
