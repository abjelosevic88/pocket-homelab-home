package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-home/internal/pocket"
	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

func remarshal(in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

type recordInput struct {
	HomeID     *int64  `json:"home_id"`
	ThingID    *int64  `json:"thing_id"`
	PlantID    *int64  `json:"plant_id"`
	JobID      *int64  `json:"job_id"`
	ProID      *int64  `json:"pro_id"`
	Date       *string `json:"date"`
	Kind       *string `json:"kind"`
	Title      *string `json:"title"`
	Notes      *string `json:"notes"`
	Cost       *int64  `json:"cost"`
	Currency   *string `json:"currency"`
	ExternalID *string `json:"external_id"`

	Money      *bool `json:"money"` // book in Pocket Money (create: default the setting)
	AccountID  int64 `json:"account_id"`
	CategoryID int64 `json:"category_id"`
}

func (in recordInput) apply(r *store.Record, keys map[string]bool) string {
	if in.HomeID != nil {
		r.HomeID = *in.HomeID
	}
	if keys["thing_id"] {
		r.ThingID = in.ThingID
	}
	if keys["plant_id"] {
		r.PlantID = in.PlantID
	}
	if keys["job_id"] {
		r.JobID = in.JobID
	}
	if keys["pro_id"] {
		r.ProID = in.ProID
	}
	trimP(&r.Date, in.Date)
	trimP(&r.Kind, in.Kind)
	trimP(&r.Title, in.Title)
	trimP(&r.Notes, in.Notes)
	if in.Cost != nil {
		r.Cost = *in.Cost
	}
	if in.Currency != nil && cleanCurrency(*in.Currency) != "" {
		r.Currency = cleanCurrency(*in.Currency)
	}
	switch {
	case r.HomeID <= 0:
		return "choose the home"
	case r.Title == "" || len(r.Title) > 200:
		return "a log entry needs a title (up to 200 characters)"
	case r.Date == "" || !validDate(r.Date):
		return "the date is YYYY-MM-DD"
	case !oneOf(r.Kind, recordKinds):
		return "kind is maintenance, repair, improvement, inspection, purchase, plant or other"
	case r.Cost < 0 || r.Cost > 1e12:
		return "the cost looks wrong"
	}
	return ""
}

func recordPayload(r *store.Record) map[string]any {
	if r == nil {
		return map[string]any{}
	}
	return map[string]any{"record_id": r.ID, "date": r.Date, "kind": r.Kind, "title": r.Title, "thing": r.ThingName, "home": r.HomeName, "pro": r.ProName,
		"cost": r.Cost, "currency": r.Currency, "job_id": r.JobID, "plant_id": r.PlantID, "plant": r.PlantName}
}

func (s *Server) handleListRecords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := queryInt(r, "limit")
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	from, to := q.Get("from"), q.Get("to")
	if !validDate(from) || !validDate(to) {
		writeErr(w, http.StatusBadRequest, "dates are YYYY-MM-DD")
		return
	}
	recs, total, err := s.store.ListRecords(r.Context(), actorID(r.Context()), store.RecordFilter{HomeID: int64(queryInt(r, "home_id")), ThingID: int64(queryInt(r, "thing_id")),
		PlantID: int64(queryInt(r, "plant_id")), Scope: scopeParam(r), ProID: int64(queryInt(r, "pro_id")), Kind: q.Get("kind"), From: from, To: to, Q: strings.TrimSpace(q.Get("q")), Limit: limit, Offset: queryInt(r, "offset")})
	if err != nil {
		s.fail(w, err, "log")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": recs, "total": total, "limit": limit, "offset": queryInt(r, "offset")})
}

func (s *Server) handleGetRecord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	rec, err := s.store.GetRecord(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "log entry")
		return
	}
	atts, _ := s.store.ListAttachments(ctx, uid, "record", rec.ID)
	out := map[string]any{"record": rec, "attachments": s.attachmentViews(ctx, uid, atts), "money_url": ""}
	if rec.MoneyTxID != nil {
		out["money_url"] = s.siblingLink(ctx, uid, appMoney, "/transactions/"+itoa(*rec.MoneyTxID))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateRecord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in recordInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	rec := store.Record{Date: store.Today(), Kind: "maintenance", Source: firstNonEmpty(linkApp(ctx), "web")}
	if in.ExternalID != nil {
		rec.ExternalID = strings.TrimSpace(*in.ExternalID)
		if rec.ExternalID != "" {
			if old, err := s.store.RecordByExternalID(ctx, uid, rec.ExternalID); err == nil {
				writeJSON(w, http.StatusOK, old) // an identical retry
				return
			}
		}
	}
	if in.ThingID != nil && in.HomeID == nil {
		if t, err := s.store.GetThing(ctx, uid, *in.ThingID); err == nil {
			rec.HomeID = t.HomeID
		}
	}
	if in.PlantID != nil && in.HomeID == nil && rec.HomeID == 0 {
		if p, err := s.store.GetPlant(ctx, uid, *in.PlantID); err == nil {
			rec.HomeID = p.HomeID
		}
	}
	if in.JobID != nil && in.HomeID == nil && rec.HomeID == 0 {
		if j, err := s.store.GetJob(ctx, uid, *in.JobID); err == nil {
			rec.HomeID = j.HomeID
		}
	}
	if in.HomeID == nil && rec.HomeID == 0 {
		if hs, _ := s.store.ListHomes(ctx, uid); len(hs) == 1 {
			rec.HomeID = hs[0].ID
		}
	}
	if msg := in.apply(&rec, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if rec.Currency == "" {
		if h, err := s.store.GetHome(ctx, uid, rec.HomeID); err == nil {
			rec.Currency = h.Currency
		}
	}
	if err := s.store.CreateRecord(ctx, uid, &rec); err != nil {
		s.fail(w, err, "add to the log")
		return
	}
	nr, _ := s.store.GetRecord(ctx, uid, rec.ID)
	out := map[string]any{"record": nr}
	if nr.Cost > 0 && linkApp(ctx) == "" && s.wantsMoney(ctx, uid, in.Money) {
		out["money"] = s.bookRecord(ctx, uid, nr, in.AccountID, in.CategoryID)
		nr, _ = s.store.GetRecord(ctx, uid, rec.ID)
		out["record"] = nr
	}
	s.publish(ctx, uid, "home.record_added", "record:"+itoa(rec.ID), recordPayload(nr))
	writeJSON(w, http.StatusCreated, out)
}

// linkMayChange: a link token may only change what its own app logged.
func linkMayChange(ctx context.Context, source string) bool {
	app := linkApp(ctx)
	return app == "" || app == source
}

func (s *Server) handleUpdateRecord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	rec, err := s.store.GetRecord(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "log entry")
		return
	}
	if !linkMayChange(ctx, rec.Source) {
		writeErr(w, http.StatusForbidden, "this app may change only what it logged itself")
		return
	}
	var in recordInput
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := in.apply(rec, keys); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdateRecord(ctx, uid, rec); err != nil {
		s.fail(w, err, "save log entry")
		return
	}
	nr, _ := s.store.GetRecord(ctx, uid, rec.ID)
	out := map[string]any{"record": nr}
	switch {
	case nr.MoneyTxID != nil:
		out["money"] = s.syncRecordMoney(ctx, uid, nr)
	case in.Money != nil && *in.Money && nr.Cost > 0:
		out["money"] = s.bookRecord(ctx, uid, nr, in.AccountID, in.CategoryID)
	}
	nr, _ = s.store.GetRecord(ctx, uid, rec.ID)
	out["record"] = nr
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteRecord(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	rec, err := s.store.GetRecord(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "log entry")
		return
	}
	if !linkMayChange(ctx, rec.Source) {
		writeErr(w, http.StatusForbidden, "this app may change only what it logged itself")
		return
	}
	var moneyNote string
	if rec.MoneyTxID != nil {
		if res := s.unbookRecord(ctx, uid, rec); !res.OK {
			moneyNote = res.Error
		}
	}
	if err := s.store.DeleteRecord(ctx, uid, rec.ID); err != nil {
		s.fail(w, err, "delete log entry")
		return
	}
	if moneyNote != "" {
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "money_error": moneyNote})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRecordMoney books an entry's cost in Pocket Money now.
func (s *Server) handleRecordMoney(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	rec, err := s.store.GetRecord(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "log entry")
		return
	}
	var in struct {
		AccountID  int64 `json:"account_id"`
		CategoryID int64 `json:"category_id"`
	}
	_ = decode(r, &in)
	if rec.Cost <= 0 {
		writeErr(w, http.StatusBadRequest, "there is no cost to book")
		return
	}
	if rec.MoneyTxID != nil {
		writeJSON(w, http.StatusOK, s.syncRecordMoney(ctx, uid, rec))
		return
	}
	writeJSON(w, http.StatusOK, s.bookRecord(ctx, uid, rec, in.AccountID, in.CategoryID))
}

// ---------- spending ----------

// handleSpending is what the log cost: per month for ?months (default 12),
// per kind, per thing and per year, in each currency.
func (s *Server) handleSpending(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	months := queryInt(r, "months")
	if months <= 0 || months > 120 {
		months = 12
	}
	home := int64(queryInt(r, "home_id"))
	today := store.Today()
	t, _ := store.ParseDate(today)
	from := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(months - 1), 0).Format("2006-01-02")
	byMonth, err := s.store.Spending(ctx, uid, home, "month", from, today)
	if err != nil {
		s.fail(w, err, "spending")
		return
	}
	byKind, _ := s.store.Spending(ctx, uid, home, "kind", from, today)
	byThing, _ := s.store.Spending(ctx, uid, home, "thing", from, today)
	byYear, _ := s.store.Spending(ctx, uid, home, "year", "0000-01-01", today)
	sort.Slice(byThing, func(i, j int) bool { return byThing[i].Total > byThing[j].Total })
	if len(byThing) > 10 {
		byThing = byThing[:10]
	}
	monthsList := []string{}
	for m := from[:7]; m <= today[:7]; {
		monthsList = append(monthsList, m)
		mt, _ := store.ParseDate(m + "-01")
		m = mt.AddDate(0, 1, 0).Format("2006-01")
	}
	writeJSON(w, http.StatusOK, map[string]any{"from": from, "to": today, "months": monthsList, "by_month": byMonth, "by_kind": byKind, "by_thing": byThing, "by_year": byYear})
}

// ---------- Pocket Money ----------

// moneyResult says what happened to a booking.
type moneyResult struct {
	OK      bool   `json:"ok"`
	TxID    int64  `json:"tx_id,omitempty"`
	Account string `json:"account,omitempty"`
	URL     string `json:"url,omitempty"`
	Via     string `json:"via,omitempty"`
	Error   string `json:"error,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

// wantsMoney is the person's choice for this cost: the form's tick, else
// the setting — and only when Money is connected at all.
func (s *Server) wantsMoney(ctx context.Context, uid int64, asked *bool) bool {
	st, _ := s.store.GetSettings(ctx, uid)
	mode, _, token, _ := connSettings(st, appMoney)
	if s.connMode(mode, token) == modeOff {
		return false
	}
	if asked != nil {
		return *asked
	}
	return st.MoneyBook
}

// homeCategoryWords match a Money category for house costs by name.
var homeCategoryWords = []string{"home", "house", "housing", "household", "maintenance", "repairs", "dom", "kuća", "kuca", "stan", "stanovanje", "održavanje", "odrzavanje", "utilities", "režije"}

func pickAccount(accts []pocket.Account, want, setting int64) *pocket.Account {
	for _, id := range []int64{want, setting} {
		for i := range accts {
			if id > 0 && accts[i].ID == id {
				return &accts[i]
			}
		}
	}
	for i := range accts {
		if accts[i].Everyday {
			return &accts[i]
		}
	}
	if len(accts) > 0 {
		return &accts[0]
	}
	return nil
}

func pickCategory(cats []pocket.Category, want, setting int64) int64 {
	for _, id := range []int64{want, setting} {
		for _, c := range cats {
			if id > 0 && c.ID == id {
				return c.ID
			}
		}
	}
	for _, w := range homeCategoryWords {
		for _, c := range cats {
			if strings.EqualFold(c.Name, w) {
				return c.ID
			}
		}
	}
	for _, w := range homeCategoryWords {
		for _, c := range cats {
			if strings.Contains(strings.ToLower(c.FullName+" "+c.Name), w) {
				return c.ID
			}
		}
	}
	return 0
}

func (s *Server) moneyTx(ctx context.Context, uid int64, rec *store.Record) pocket.Tx {
	desc := rec.Title
	if rec.ThingName != "" && !strings.Contains(strings.ToLower(desc), strings.ToLower(rec.ThingName)) {
		desc = rec.ThingName + ": " + desc
	}
	tags := []string{"home"}
	if h, err := s.store.GetHome(ctx, uid, rec.HomeID); err == nil && h.MoneyTag != "" {
		tags = append(tags, h.MoneyTag)
	}
	return pocket.Tx{Date: rec.Date, Amount: rec.Cost, Currency: rec.Currency, Description: desc, Payee: rec.ProName, Tags: tags,
		Notes: fmt.Sprintf("Pocket Home — %s (%s). %s", rec.HomeName, rec.Kind, rec.Notes), ExternalID: fmt.Sprintf("home:record:%d", rec.ID)}
}

// bookRecord creates the transaction for an entry's cost, as the person,
// and remembers it. Errors come back as text; the entry itself is kept.
func (s *Server) bookRecord(ctx context.Context, uid int64, rec *store.Record, accountID, categoryID int64) moneyResult {
	s.moneyMu.Lock()
	defer s.moneyMu.Unlock()
	if rec.MoneyTxID != nil {
		return moneyResult{OK: true, TxID: *rec.MoneyTxID, Skipped: true}
	}
	cctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	c := s.clients(cctx, uid)
	conn, err := c.conn(cctx, appMoney, scopeAccountsRead, scopeTxWrite)
	if err != nil {
		return moneyResult{Error: err.Error()}
	}
	accts, err := conn.client.Accounts(cctx)
	if err != nil {
		return moneyResult{Error: "Pocket Money: " + err.Error(), Via: conn.via}
	}
	acct := pickAccount(accts, accountID, c.st.MoneyAccountID)
	if acct == nil {
		return moneyResult{Error: "Pocket Money has no account you can book on", Via: conn.via}
	}
	cats, _ := conn.client.Categories(cctx, acct.ID)
	tx := s.moneyTx(cctx, uid, rec)
	tx.AccountID, tx.CategoryID = acct.ID, pickCategory(cats, categoryID, c.st.MoneyCategoryID)
	id, err := conn.client.CreateTx(cctx, tx)
	if err != nil {
		return moneyResult{Error: "Pocket Money: " + err.Error(), Via: conn.via}
	}
	_ = s.store.SetRecordMoney(ctx, uid, rec.ID, &id)
	return moneyResult{OK: true, TxID: id, Account: acct.Name, Via: conn.via, URL: strings.TrimRight(conn.public, "/") + "/transactions/" + itoa(id)}
}

// syncRecordMoney carries an edited entry over to its transaction; a cost
// of 0 removes the transaction.
func (s *Server) syncRecordMoney(ctx context.Context, uid int64, rec *store.Record) moneyResult {
	if rec.MoneyTxID == nil {
		return moneyResult{Skipped: true}
	}
	if rec.Cost == 0 {
		return s.unbookRecord(ctx, uid, rec)
	}
	s.moneyMu.Lock()
	defer s.moneyMu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	conn, err := s.clients(cctx, uid).conn(cctx, appMoney, scopeTxWrite)
	if err != nil {
		return moneyResult{Error: err.Error()}
	}
	err = conn.client.UpdateTx(cctx, *rec.MoneyTxID, s.moneyTx(cctx, uid, rec))
	if errors.Is(err, pocket.ErrNotFound) {
		// deleted in Money meanwhile: forget it
		_ = s.store.SetRecordMoney(ctx, uid, rec.ID, nil)
		return moneyResult{OK: true, Skipped: true}
	}
	if err != nil {
		return moneyResult{Error: "Pocket Money: " + err.Error(), Via: conn.via}
	}
	return moneyResult{OK: true, TxID: *rec.MoneyTxID, Via: conn.via}
}

// unbookRecord removes an entry's transaction.
func (s *Server) unbookRecord(ctx context.Context, uid int64, rec *store.Record) moneyResult {
	if rec.MoneyTxID == nil {
		return moneyResult{OK: true, Skipped: true}
	}
	s.moneyMu.Lock()
	defer s.moneyMu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	conn, err := s.clients(cctx, uid).conn(cctx, appMoney, scopeTxWrite)
	if err != nil {
		return moneyResult{Error: err.Error()}
	}
	if err := conn.client.DeleteTx(cctx, *rec.MoneyTxID); err != nil {
		return moneyResult{Error: "Pocket Money: " + err.Error(), Via: conn.via}
	}
	_ = s.store.SetRecordMoney(ctx, uid, rec.ID, nil)
	return moneyResult{OK: true, Via: conn.via}
}
