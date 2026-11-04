package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-home/internal/pocket"
	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// ---------- Pocket Calendar: upkeep due dates as all-day events ----------
//
// With "Show upkeep on my calendar" on, every open job has one all-day event
// on its due date in the person's Pocket Calendar. A job remembers what its
// event shows (cal_synced); any change clears that and the next sync (right
// after the change, and every ten minutes) moves the event. Paused, finished
// and deleted jobs lose theirs. Calendar lets an app change only the events
// it created, so the person's own events are never touched.

func calendarText(j store.Job) (title, desc string) {
	title = j.Title
	if j.ThingName != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(j.ThingName)) {
		title = j.ThingName + ": " + title
	}
	desc = "Upkeep in " + j.HomeName + " (Pocket Home)."
	if j.Recurring() {
		desc += fmt.Sprintf(" Every %d %s.", j.EveryN, j.EveryUnit+plural(j.EveryN))
	}
	if j.Notes != "" {
		desc += "\n\n" + j.Notes
	}
	return title, desc
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func syncedKey(j store.Job, calID int64) string {
	t, d := calendarText(j)
	return fmt.Sprintf("%d|%s|%s|%x", calID, j.DueOn, t, len(d))
}

// syncCalendarSoon syncs one person's calendar in the background.
func (s *Server) syncCalendarSoon(uid int64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, _ = s.syncCalendar(ctx, uid)
	}()
}

func (s *Server) syncAllCalendars(ctx context.Context) {
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	for _, uid := range ids {
		if _, err := s.syncCalendar(ctx, uid); err != nil && !isNotConnected(err) {
			s.log.Debug("calendar sync", "user", uid, "err", err)
		}
	}
}

// syncCalendar brings one person's events in line with their upkeep and
// says how many it created, moved and removed.
func (s *Server) syncCalendar(ctx context.Context, uid int64) (map[string]int, error) {
	s.calMu.Lock()
	defer s.calMu.Unlock()
	st, err := s.store.GetSettings(ctx, uid)
	if err != nil {
		return nil, err
	}
	jobs, err := s.store.ListJobs(ctx, uid, store.JobFilter{All: true})
	if err != nil {
		return nil, err
	}
	n := map[string]int{"created": 0, "moved": 0, "removed": 0, "failed": 0}
	var work []store.Job
	for _, j := range jobs {
		live := st.CalendarSync && j.FinishedAt == "" && !j.Paused
		if (live && j.CalSynced != syncedKey(j, st.CalendarID)) || (!live && j.CalEventID != nil) {
			work = append(work, j)
		}
	}
	if len(work) == 0 {
		return n, nil
	}
	conn, err := s.clients(ctx, uid).conn(ctx, appCalendar, scopeCalendarWrite)
	if err != nil {
		return n, err
	}
	for _, j := range work {
		live := st.CalendarSync && j.FinishedAt == "" && !j.Paused
		if !live {
			if err := conn.client.DeleteEvent(ctx, *j.CalEventID); err != nil {
				n["failed"]++
				continue
			}
			_ = s.store.SetJobCalendar(ctx, uid, j.ID, nil, "")
			n["removed"]++
			continue
		}
		title, desc := calendarText(j)
		ev := pocket.Event{CalendarID: st.CalendarID, Title: title, Description: desc, URL: s.cfg.BaseURL + "/upkeep/" + itoa(j.ID), Date: j.DueOn}
		if j.CalEventID != nil {
			err := conn.client.UpdateEvent(ctx, *j.CalEventID, ev)
			if err == nil {
				_ = s.store.SetJobCalendar(ctx, uid, j.ID, j.CalEventID, syncedKey(j, st.CalendarID))
				n["moved"]++
				continue
			}
			if !errors.Is(err, pocket.ErrNotFound) {
				n["failed"]++
				continue
			}
			// deleted in Calendar: make a new one
		}
		id, err := conn.client.CreateEvent(ctx, ev)
		if err != nil || id == 0 {
			n["failed"]++
			continue
		}
		_ = s.store.SetJobCalendar(ctx, uid, j.ID, &id, syncedKey(j, st.CalendarID))
		n["created"]++
	}
	if n["failed"] > 0 {
		return n, fmt.Errorf("%d calendar changes failed", n["failed"])
	}
	return n, nil
}

// dropCalendarEvents removes the events of jobs that are gone (best effort).
func (s *Server) dropCalendarEvents(uid int64, jobs []store.Job) {
	var ids []int64
	for _, j := range jobs {
		if j.CalEventID != nil {
			ids = append(ids, *j.CalEventID)
		}
	}
	if len(ids) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		conn, err := s.clients(ctx, uid).conn(ctx, appCalendar, scopeCalendarWrite)
		if err != nil {
			return
		}
		for _, id := range ids {
			if err := conn.client.DeleteEvent(ctx, id); err != nil {
				s.log.Debug("calendar: removing an event failed", "event", id, "err", err)
			}
		}
	}()
}

// handleCalendarSync runs the sync now (Settings → Connected apps).
func (s *Server) handleCalendarSync(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	n, err := s.syncCalendar(ctx, actorID(ctx))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "counts": n})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "counts": n})
}

// ---------- pickers: what the other apps have ----------

// pick answers a picker: the items, or why the app is not reachable.
func pick(w http.ResponseWriter, items any, err error, app pocketApp) {
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"connected": !isNotConnected(err), "detail": app.name + ": " + strings.TrimPrefix(err.Error(), app.name+" "), "items": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connected": true, "items": items})
}

func (s *Server) handlePickMoneyAccounts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	conn, err := s.clients(ctx, actorID(ctx)).conn(ctx, appMoney, scopeAccountsRead)
	if err != nil {
		pick(w, nil, err, appMoney)
		return
	}
	accts, err := conn.client.Accounts(ctx)
	pick(w, accts, err, appMoney)
}

func (s *Server) handlePickMoneyCategories(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	conn, err := s.clients(ctx, actorID(ctx)).conn(ctx, appMoney, scopeAccountsRead)
	if err != nil {
		pick(w, nil, err, appMoney)
		return
	}
	cats, err := conn.client.Categories(ctx, int64(queryInt(r, "account_id")))
	pick(w, cats, err, appMoney)
}

func (s *Server) handlePickCalendars(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	conn, err := s.clients(ctx, actorID(ctx)).conn(ctx, appCalendar, scopeCalendarRead)
	if err != nil {
		pick(w, nil, err, appCalendar)
		return
	}
	cals, err := conn.client.Calendars(ctx)
	pick(w, cals, err, appCalendar)
}

func (s *Server) handlePickContacts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"connected": true, "items": []any{}})
		return
	}
	conn, err := s.clients(ctx, actorID(ctx)).conn(ctx, appContacts, scopeContactsRead)
	if err != nil {
		pick(w, nil, err, appContacts)
		return
	}
	cs, err := conn.client.SearchContacts(ctx, q, 12)
	pick(w, cs, err, appContacts)
}

func (s *Server) handlePickDocuments(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	conn, err := s.clients(ctx, actorID(ctx)).conn(ctx, appDocuments, scopeDocumentsRead)
	if err != nil {
		pick(w, nil, err, appDocuments)
		return
	}
	docs, err := conn.client.SearchDocuments(ctx, strings.TrimSpace(r.URL.Query().Get("q")), 15)
	pick(w, docs, err, appDocuments)
}

func (s *Server) handlePickInventory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	conn, err := s.clients(ctx, actorID(ctx)).conn(ctx, appInventory, scopeItemsRead)
	if err != nil {
		pick(w, nil, err, appInventory)
		return
	}
	items, err := conn.client.SearchItems(ctx, strings.TrimSpace(r.URL.Query().Get("q")), 15)
	pick(w, items, err, appInventory)
}

// handleProFromContact makes a pro from a Pocket Contacts card.
func (s *Server) handleProFromContact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		ContactID int64  `json:"contact_id"`
		Trade     string `json:"trade"`
	}
	if err := decode(r, &in); err != nil || in.ContactID <= 0 {
		writeErr(w, http.StatusBadRequest, "choose a contact")
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := s.clients(cctx, uid).conn(cctx, appContacts, scopeContactsRead)
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	c, err := conn.client.GetContact(cctx, in.ContactID)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "Pocket Contacts: "+err.Error())
		return
	}
	p := store.Pro{Name: firstNonEmpty(strings.TrimSpace(c.FN), strings.TrimSpace(c.Org), "Contact"), Trade: strings.TrimSpace(firstNonEmpty(in.Trade, c.Title)), ContactID: &c.ID}
	if strings.TrimSpace(c.Org) != p.Name {
		p.Company = strings.TrimSpace(c.Org)
	}
	if len(c.Phones) > 0 {
		p.Phone = c.Phones[0]
	}
	if len(c.Emails) > 0 {
		p.Email = c.Emails[0]
	}
	if len(c.URLs) > 0 {
		if u, ok := cleanURL(c.URLs[0]); ok {
			p.Website = u
		}
	}
	if err := s.store.CreatePro(ctx, uid, &p); err != nil {
		s.fail(w, err, "create pro")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}
