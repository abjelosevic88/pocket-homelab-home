package server

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-home/internal/ai"
	"github.com/abjelosevic88/pocket-homelab-home/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-home/internal/pocket"
	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// ---- settings ----

func mask(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••"
	}
	return s[:4] + "••••" + s[len(s)-4:]
}

// publicSettings is what one person sees: their settings with the secrets
// masked and, per Pocket app, the mode in effect (<app>_mode), whether it
// works and how (<app>_via: core | direct | "") and why not (<app>_detail).
func (s *Server) publicSettings(ctx context.Context, u *store.User, st store.Settings) map[string]any {
	out := map[string]any{
		"core_configured": s.core.Configured(), "core_registered": s.coreStatus()["registered"],
		"title": st.Title, "date_format": st.DateFormat, "time_format": st.TimeFormat, "week_start": st.WeekStart,
		"currency": st.Currency, "area_unit": st.AreaUnit,
		"notify_upkeep": st.NotifyUpkeep, "notify_warranty": st.NotifyWarranty, "notify_meters": st.NotifyMeters, "notify_hour": st.NotifyHour,
		"notify_url": st.NotifyURL, "notify_format": st.NotifyFormat, "notify_configured": st.NotifyURL != "",
		"money_book": st.MoneyBook, "money_account_id": st.MoneyAccountID, "money_category_id": st.MoneyCategoryID,
		"calendar_sync": st.CalendarSync, "calendar_id": st.CalendarID,
		"ai_provider": st.AIProvider, "ai_pocket_model": st.AIPocketModel, "openai_api_key": mask(st.OpenAIKey), "openai_model": st.OpenAIModel, "openai_base_url": st.OpenAIBaseURL,
		"setup_complete": st.SetupComplete, "base_url": s.cfg.BaseURL,
	}
	for _, a := range pocketApps {
		mode, via, detail := s.connStatus(ctx, u, st, a)
		out[a.slug+"_mode"], out[a.slug+"_via"], out[a.slug+"_detail"] = mode, via, detail
		out[a.slug+"_configured"] = via != ""
		_, url, token, public := connSettings(st, a)
		out[a.slug+"_url"], out[a.slug+"_token"], out[a.slug+"_public_url"] = url, mask(token), public
		if public == "" && via != "" {
			public = s.resolvePublic(ctx, a.slug)
		}
		out[a.slug+"_open_url"] = public
	}
	return out
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetSettings(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "settings")
		return
	}
	writeJSON(w, http.StatusOK, s.publicSettings(r.Context(), userFrom(r.Context()), st))
}

// handlePutSettings saves the caller's personal settings. The box-wide title
// is only taken from an admin.
func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	isAdmin := me.Role == "admin"
	cur, err := s.store.GetSettings(r.Context(), me.ID)
	if err != nil {
		s.fail(w, err, "settings")
		return
	}
	raw := map[string]any{}
	var in store.Settings
	keys, err := decodeKeys(r, &raw)
	if err != nil || remarshal(raw, &in) != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	bad := func(msg string) { writeErr(w, http.StatusBadRequest, msg) }
	if keys["title"] && isAdmin {
		cur.Title = strings.TrimSpace(in.Title)
		if cur.Title == "" {
			cur.Title = store.DefaultTitle
		}
	}
	if keys["date_format"] {
		cur.DateFormat = in.DateFormat
	}
	if keys["time_format"] {
		cur.TimeFormat = in.TimeFormat
	}
	if keys["week_start"] && (in.WeekStart == 1 || in.WeekStart == 7 || in.WeekStart == 6) {
		cur.WeekStart = in.WeekStart
	}
	if keys["currency"] {
		if c := cleanCurrency(in.Currency); c != "" {
			cur.Currency = c
		} else {
			bad("currency is a three-letter code like EUR")
			return
		}
	}
	if keys["area_unit"] && (in.AreaUnit == "m2" || in.AreaUnit == "ft2") {
		cur.AreaUnit = in.AreaUnit
	}
	if keys["notify_hour"] {
		if in.NotifyHour < 0 || in.NotifyHour > 23 {
			bad("the hour is 0 to 23")
			return
		}
		cur.NotifyHour = in.NotifyHour
	}
	secret := func(k string, dst *string, v string) {
		if keys[k] && !strings.Contains(v, "••") {
			*dst = strings.TrimSpace(v)
		}
	}
	url := func(k string, dst *string, v string) {
		if keys[k] {
			*dst = strings.TrimRight(strings.TrimSpace(v), "/")
		}
	}
	text := func(k string, dst *string, v string) {
		if keys[k] {
			*dst = strings.TrimSpace(v)
		}
	}
	boolIn := func(k string, dst *bool, v bool) {
		if keys[k] {
			*dst = v
		}
	}
	idIn := func(k string, dst *int64, v int64) {
		if keys[k] && v >= 0 {
			*dst = v
		}
	}
	boolIn("notify_upkeep", &cur.NotifyUpkeep, in.NotifyUpkeep)
	boolIn("notify_warranty", &cur.NotifyWarranty, in.NotifyWarranty)
	boolIn("notify_meters", &cur.NotifyMeters, in.NotifyMeters)
	text("notify_url", &cur.NotifyURL, in.NotifyURL)
	if keys["notify_format"] && in.NotifyFormat != "" {
		cur.NotifyFormat = in.NotifyFormat
	}
	boolIn("money_book", &cur.MoneyBook, in.MoneyBook)
	idIn("money_account_id", &cur.MoneyAccountID, in.MoneyAccountID)
	idIn("money_category_id", &cur.MoneyCategoryID, in.MoneyCategoryID)
	calWas := cur.CalendarSync
	boolIn("calendar_sync", &cur.CalendarSync, in.CalendarSync)
	idIn("calendar_id", &cur.CalendarID, in.CalendarID)
	// each Pocket app: mode, URL, token, public URL
	inMap := map[string]string{}
	for k, v := range raw {
		if sv, ok := v.(string); ok {
			inMap[k] = sv
		}
	}
	for _, a := range pocketApps {
		mode, u, tok, pub := connFields(&cur, a.slug)
		if v := inMap[a.slug+"_mode"]; keys[a.slug+"_mode"] && (v == modeCore || v == modeDirect || v == modeOff) {
			*mode = v
		}
		url(a.slug+"_url", u, inMap[a.slug+"_url"])
		secret(a.slug+"_token", tok, inMap[a.slug+"_token"])
		url(a.slug+"_public_url", pub, inMap[a.slug+"_public_url"])
	}
	if keys["ai_provider"] {
		if !validAIProvider(in.AIProvider) {
			bad("AI is automatic, Pocket AI or OpenAI")
			return
		}
		cur.AIProvider = in.AIProvider
	}
	if keys["ai_pocket_model"] {
		if !validPocketModel(in.AIPocketModel) {
			bad("unknown model")
			return
		}
		cur.AIPocketModel = in.AIPocketModel
	}
	secret("openai_api_key", &cur.OpenAIKey, in.OpenAIKey)
	text("openai_model", &cur.OpenAIModel, in.OpenAIModel)
	url("openai_base_url", &cur.OpenAIBaseURL, in.OpenAIBaseURL)
	if err := s.store.SaveUserSettings(r.Context(), me.ID, cur); err != nil {
		s.fail(w, err, "save settings")
		return
	}
	if isAdmin && keys["title"] {
		if err := s.store.SaveBoxSettings(r.Context(), cur); err != nil {
			s.fail(w, err, "save settings")
			return
		}
	}
	for _, k := range modeKeys {
		if keys[k] {
			s.reportModesSoon()
			break
		}
	}
	if cur.CalendarSync != calWas || keys["calendar_id"] || keys["calendar_mode"] {
		s.syncCalendarSoon(me.ID)
	}
	writeJSON(w, http.StatusOK, s.publicSettings(r.Context(), me, cur))
}

// handleIntegrationTest checks a connection with the caller's own saved
// settings (or the URL/key typed into the form).
func (s *Server) handleIntegrationTest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Which  string `json:"which"`
		URL    string `json:"url"`
		Key    string `json:"key"`
		Format string `json:"format"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	uid := actorID(r.Context())
	st, _ := s.store.GetSettings(r.Context(), uid)
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	keyOr := func(saved string) string {
		if strings.Contains(in.Key, "••") || in.Key == "" {
			return saved
		}
		return in.Key
	}
	urlOr := func(saved string) string {
		if strings.TrimSpace(in.URL) == "" {
			return saved
		}
		return strings.TrimRight(strings.TrimSpace(in.URL), "/")
	}
	var msg string
	var err error
	if a, ok := appBySlug(in.Which); ok {
		mode, savedURL, savedTok, _ := connSettings(st, a)
		typed := in.Key != "" && !strings.Contains(in.Key, "••")
		var conn *siblingConn
		if tok := keyOr(savedTok); tok != "" && (typed || s.connMode(mode, savedTok) == modeDirect) {
			base := urlOr(savedURL)
			if base == "" {
				base = s.resolveSibling(ctx, a.slug)
			}
			conn = &siblingConn{client: pocket.New(base, tok, 15*time.Second), via: modeDirect}
		} else {
			conn, err = s.clients(ctx, uid).conn(ctx, a, a.scopes...)
		}
		if err == nil {
			how := "direct connection"
			if conn.via == modeCore {
				how = "through Pocket Core"
			}
			var v string
			if v, err = conn.client.Health(ctx); err == nil {
				msg = fmt.Sprintf("%s %s, %s", a.name, v, how)
				switch a.slug {
				case "money":
					var accts []pocket.Account
					if accts, err = conn.client.Accounts(ctx); err == nil {
						msg += fmt.Sprintf(", %d accounts you can book on", len(accts))
					}
				case "calendar":
					var cals []pocket.Calendar
					if cals, err = conn.client.Calendars(ctx); err == nil {
						msg += fmt.Sprintf(", %d calendars you can write to", len(cals))
					}
				case "documents":
					_, err = conn.client.SearchDocuments(ctx, "", 1)
				case "inventory":
					_, err = conn.client.SearchItems(ctx, "", 1)
				case "contacts":
					_, err = conn.client.SearchContacts(ctx, "a", 1)
				}
			}
		}
	} else if in.Which == "openai" {
		c := ai.New(urlOr(st.OpenAIBaseURL), keyOr(st.OpenAIKey), st.OpenAIModel, 20*time.Second)
		var ids []string
		if ids, err = c.Models(ctx); err == nil {
			msg = fmt.Sprintf("The key works (%d models)", len(ids))
		}
	} else if in.Which == "notify" {
		f := in.Format
		if f == "" {
			f = st.NotifyFormat
		}
		n := notify.New(urlOr(st.NotifyURL), f)
		err = n.Send(ctx, notify.Message{Title: "Pocket Home", Body: "Notifications work. Upkeep that falls due, warranties that end and meter readings will arrive here.", Priority: "info", Tags: []string{"house"}})
		msg = "Test message sent via " + n.Format
	} else {
		writeErr(w, http.StatusBadRequest, "unknown integration")
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

// handleCheckNow runs the daily check for the caller now and says what it sent.
func (s *Server) handleCheckNow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, _ := s.store.GetSettings(ctx, uid)
	body, sent := s.dailyCheck(ctx, uid, st, time.Now())
	writeJSON(w, http.StatusOK, map[string]any{"sent": sent, "body": body})
}

// ---- system / backup / export ----

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	var size int64
	if fi, err := os.Stat(s.cfg.DBPath); err == nil {
		size = fi.Size()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version": Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
		"core":      s.coreStatus(),
		"auth_mode": s.cfg.AuthMode, "data_dir": s.cfg.DataDir, "db_path": s.cfg.DBPath, "db_bytes": size,
		"uptime": time.Since(s.started).Round(time.Second).String(), "timezone": time.Now().Location().String(),
		"counts": s.store.CountsFor(r.Context(), actorID(r.Context())),
	})
}

// handleBackup downloads the whole database, an admin-only box operation.
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	dir := filepath.Join(s.cfg.DataDir, "backups")
	name := fmt.Sprintf("pocket-home-%s.db", time.Now().Format("20060102-150405"))
	dst := filepath.Join(dir, name)
	if err := s.store.Backup(r.Context(), dst); err != nil {
		s.fail(w, err, "backup")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFile(w, r, dst)
}

// exportDoc is the JSON export of one person's data.
type exportDoc struct {
	App         string             `json:"app"`
	Version     string             `json:"version"`
	Exported    string             `json:"exported_at"`
	Homes       []store.Home       `json:"homes"`
	Things      []store.Thing      `json:"things"`
	Pros        []store.Pro        `json:"pros"`
	Jobs        []store.Job        `json:"jobs"`
	Records     []store.Record     `json:"records"`
	Meters      []store.Meter      `json:"meters"`
	Readings    []store.Reading    `json:"readings"`
	Attachments []store.Attachment `json:"attachments"`
}

func (s *Server) export(ctx context.Context, uid int64) exportDoc {
	doc := exportDoc{App: "pocket-home", Version: Version, Exported: store.Now(), Readings: []store.Reading{}}
	doc.Homes, _ = s.store.ListHomes(ctx, uid)
	doc.Things, _ = s.store.ListThings(ctx, uid, store.ThingFilter{Archived: true})
	doc.Pros, _ = s.store.ListPros(ctx, uid, "")
	doc.Jobs, _ = s.store.ListJobs(ctx, uid, store.JobFilter{All: true})
	doc.Records, _, _ = s.store.ListRecords(ctx, uid, store.RecordFilter{})
	doc.Meters, _ = s.store.ListMeters(ctx, uid, 0)
	for _, m := range doc.Meters {
		rs, _ := s.store.ListReadings(ctx, uid, m.ID, 0)
		doc.Readings = append(doc.Readings, rs...)
	}
	doc.Attachments, _ = s.store.AllAttachments(ctx, uid)
	return doc
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", `attachment; filename="pocket-home-export.json"`)
	writeJSON(w, http.StatusOK, s.export(r.Context(), actorID(r.Context())))
}

// handleExportCSV writes the log as CSV rows.
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	recs, _, err := s.store.ListRecords(ctx, actorID(ctx), store.RecordFilter{})
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="pocket-home-log.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"date", "home", "thing", "kind", "title", "cost", "currency", "pro", "upkeep", "notes", "money_tx_id"})
	for i := len(recs) - 1; i >= 0; i-- {
		rc := recs[i]
		tx := ""
		if rc.MoneyTxID != nil {
			tx = strconv.FormatInt(*rc.MoneyTxID, 10)
		}
		_ = cw.Write([]string{rc.Date, rc.HomeName, rc.ThingName, rc.Kind, rc.Title, strconv.FormatFloat(float64(rc.Cost)/100, 'f', 2, 64), rc.Currency, rc.ProName, rc.JobTitle, rc.Notes, tx})
	}
	cw.Flush()
}

// handleImportJSON merges an export into the caller's own data. Rows keep
// their identity (the export's ids, remembered per person), so importing
// the same file twice adds nothing. Money and calendar links are not
// carried over: they belong to the account the file came from.
func (s *Server) handleImportJSON(w http.ResponseWriter, r *http.Request) {
	var doc exportDoc
	if err := decode(r, &doc); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if doc.App != "" && doc.App != "pocket-home" {
		writeErr(w, http.StatusBadRequest, "this is not a Pocket Home export")
		return
	}
	ctx := r.Context()
	uid := actorID(ctx)
	counts := map[string]int{}
	// seen maps "<kind>:<old id>" to the new id; it survives in the cache so
	// a second import of the same file finds everything already there.
	idOf := func(kind string, old int64) (int64, bool) {
		v, _, ok := s.store.CacheGet(ctx, fmt.Sprintf("import:%d:%s:%d", uid, kind, old))
		if !ok {
			return 0, false
		}
		n, _ := strconv.ParseInt(v, 10, 64)
		return n, n > 0
	}
	remember := func(kind string, old, id int64) {
		_ = s.store.CachePut(ctx, fmt.Sprintf("import:%d:%s:%d", uid, kind, old), strconv.FormatInt(id, 10))
	}
	exists := func(table string, id int64) bool {
		var n int
		_ = s.store.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM `+table+` WHERE id = ? AND user_id = ?`, id, uid).Scan(&n)
		return n > 0
	}
	mapID := func(kind, table string, p *int64) *int64 {
		if p == nil {
			return nil
		}
		if id, ok := idOf(kind, *p); ok && exists(table, id) {
			return &id
		}
		return nil
	}
	for _, h := range doc.Homes {
		if id, ok := idOf("home", h.ID); ok && exists("homes", id) {
			continue
		}
		old := h.ID
		if err := s.store.CreateHome(ctx, uid, &h); err == nil {
			remember("home", old, h.ID)
			counts["homes"]++
		}
	}
	for _, p := range doc.Pros {
		if id, ok := idOf("pro", p.ID); ok && exists("pros", id) {
			continue
		}
		old := p.ID
		if err := s.store.CreatePro(ctx, uid, &p); err == nil {
			remember("pro", old, p.ID)
			counts["pros"]++
		}
	}
	for _, t := range doc.Things {
		if id, ok := idOf("thing", t.ID); ok && exists("things", id) {
			continue
		}
		hid := mapID("home", "homes", &t.HomeID)
		if hid == nil {
			counts["skipped"]++
			continue
		}
		old := t.ID
		t.HomeID = *hid
		if err := s.store.CreateThing(ctx, uid, &t); err == nil {
			remember("thing", old, t.ID)
			counts["things"]++
		}
	}
	for _, j := range doc.Jobs {
		if id, ok := idOf("job", j.ID); ok && exists("jobs", id) {
			continue
		}
		hid := mapID("home", "homes", &j.HomeID)
		if hid == nil {
			counts["skipped"]++
			continue
		}
		old := j.ID
		j.HomeID, j.ThingID, j.ProID, j.Source = *hid, mapID("thing", "things", j.ThingID), mapID("pro", "pros", j.ProID), "import"
		if err := s.store.CreateJob(ctx, uid, &j); err == nil {
			if j.FinishedAt != "" {
				_ = s.store.UpdateJob(ctx, uid, &j)
			}
			remember("job", old, j.ID)
			counts["jobs"]++
		}
	}
	for _, rc := range doc.Records {
		if id, ok := idOf("record", rc.ID); ok && exists("records", id) {
			continue
		}
		hid := mapID("home", "homes", &rc.HomeID)
		if hid == nil {
			counts["skipped"]++
			continue
		}
		old := rc.ID
		rc.HomeID, rc.ThingID, rc.ProID, rc.JobID, rc.Source = *hid, mapID("thing", "things", rc.ThingID), mapID("pro", "pros", rc.ProID), mapID("job", "jobs", rc.JobID), "import"
		if rc.ExternalID == "" {
			rc.ExternalID = fmt.Sprintf("import:%d", old)
		}
		if err := s.store.CreateRecord(ctx, uid, &rc); err == nil {
			remember("record", old, rc.ID)
			counts["records"]++
		}
	}
	for _, m := range doc.Meters {
		if id, ok := idOf("meter", m.ID); ok && exists("meters", id) {
			continue
		}
		hid := mapID("home", "homes", &m.HomeID)
		if hid == nil {
			counts["skipped"]++
			continue
		}
		old := m.ID
		m.HomeID = *hid
		if err := s.store.CreateMeter(ctx, uid, &m); err == nil {
			remember("meter", old, m.ID)
			counts["meters"]++
		}
	}
	for _, rd := range doc.Readings {
		if id, ok := idOf("reading", rd.ID); ok && exists("readings", id) {
			continue
		}
		mid := mapID("meter", "meters", &rd.MeterID)
		if mid == nil {
			continue
		}
		old := rd.ID
		rd.MeterID, rd.Source = *mid, "import"
		if err := s.store.CreateReading(ctx, uid, &rd); err == nil {
			remember("reading", old, rd.ID)
			counts["readings"]++
		}
	}
	for _, a := range doc.Attachments {
		if id, ok := idOf("attachment", a.ID); ok && exists("attachments", id) {
			continue
		}
		tbl := map[string]string{"home": "homes", "thing": "things", "record": "records"}[a.OwnerKind]
		if tbl == "" {
			continue
		}
		oid := mapID(a.OwnerKind, tbl, &a.OwnerID)
		if oid == nil {
			continue
		}
		old := a.ID
		a.OwnerID = *oid
		if err := s.store.CreateAttachment(ctx, uid, &a); err == nil {
			remember("attachment", old, a.ID)
			counts["attachments"]++
		}
	}
	writeJSON(w, http.StatusOK, counts)
}
