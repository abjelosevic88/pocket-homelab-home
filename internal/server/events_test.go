package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// Saving a thing queues home.thing_saved on the Core outbox, with the owner's
// email and the warranty date.
func TestThingSavedEvent(t *testing.T) {
	cfg := testConfig(t.TempDir())
	cfg.CoreURL = "http://core.invalid"
	st := openTestStore(t, cfg)
	s := New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	ctx := context.Background()
	// Init creates the outbox; no Start, so nothing drains it.
	if err := s.core.Init(ctx); err != nil {
		t.Fatal(err)
	}
	u := &store.User{Email: "ana@example.com", Name: "Ana", Role: "admin"}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	tok, _, _ := st.CreateAPIToken(ctx, u.ID, "t")
	ts := httptest.NewServer(s.Router())
	t.Cleanup(ts.Close)
	a := &app{t: t, url: ts.URL, s: s, st: st}

	h := a.ok(tok, "POST", "/api/v1/homes", map[string]any{"name": "Flat"})
	th := a.ok(tok, "POST", "/api/v1/things", map[string]any{"home_id": num(h, "id"), "name": "Dishwasher", "warranty_until": "2028-03-01"})
	id := int64(num(th, "id"))
	a.ok(tok, "PUT", "/api/v1/things/"+itoa(id), map[string]any{"warranty_until": "2029-03-01"})

	rows, err := st.DB.QueryContext(ctx, `SELECT body FROM pocket_outbox ORDER BY created_at, rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []map[string]any
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			t.Fatal(err)
		}
		var ev struct {
			Topic, Subject string
			Payload        map[string]any
		}
		if err := json.Unmarshal(b, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Topic != "home.thing_saved" {
			continue
		}
		if ev.Subject != "thing:"+itoa(id) {
			t.Errorf("subject = %q", ev.Subject)
		}
		got = append(got, ev.Payload)
	}
	if len(got) != 2 {
		t.Fatalf("thing_saved events = %d, want 2 (create and update)", len(got))
	}
	for i, want := range []string{"2028-03-01", "2029-03-01"} {
		p := got[i]
		if p["warranty_until"] != want || p["name"] != "Dishwasher" || p["home"] != "Flat" || p["user_email"] != "ana@example.com" || int64(p["thing_id"].(float64)) != id {
			t.Errorf("event %d payload = %v", i, p)
		}
	}
}
