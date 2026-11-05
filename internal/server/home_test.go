package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abjelosevic88/pocket-homelab-home/internal/config"
	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

type app struct {
	t     *testing.T
	url   string
	s     *Server
	st    *store.Store
	alice string
	bob   string
	ids   map[string]int64
}

func newApp(t *testing.T, tweak func(*config.Config)) *app {
	t.Helper()
	cfg := testConfig(t.TempDir())
	cfg.BaseURL = "http://home.test"
	cfg.MaxBodyBytes = 64 << 20
	cfg.NotifyHour = 8
	if tweak != nil {
		tweak(cfg)
	}
	st := openTestStore(t, cfg)
	ctx := context.Background()
	a := &app{t: t, st: st, ids: map[string]int64{}}
	for _, u := range []*store.User{{Email: "alice@example.com", Name: "Alice", Role: "admin"}, {Email: "bob@example.com", Name: "Bob", Role: "member"}} {
		if err := st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
		tok, _, _ := st.CreateAPIToken(ctx, u.ID, "t")
		a.ids[u.Name] = u.ID
		if u.Name == "Alice" {
			a.alice = tok
		} else {
			a.bob = tok
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	a.s = New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	if err := a.s.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(a.s.Router())
	t.Cleanup(ts.Close)
	a.url = ts.URL
	return a
}

func (a *app) call(auth, method, path string, body any) (int, map[string]any, []any) {
	a.t.Helper()
	var rd io.Reader
	if body != nil {
		j, _ := json.Marshal(body)
		rd = bytes.NewReader(j)
	}
	req, _ := http.NewRequest(method, a.url+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var m map[string]any
	var l []any
	if json.Unmarshal(raw, &m) != nil {
		_ = json.Unmarshal(raw, &l)
	}
	return res.StatusCode, m, l
}

func (a *app) ok(auth, method, path string, body any) map[string]any {
	a.t.Helper()
	code, m, _ := a.call(auth, method, path, body)
	if code >= 300 {
		a.t.Fatalf("%s %s → %d %v", method, path, code, m)
	}
	return m
}

func (a *app) list(auth, path string) []any {
	a.t.Helper()
	code, m, l := a.call(auth, "GET", path, nil)
	if code != 200 {
		a.t.Fatalf("GET %s → %d %v", path, code, m)
	}
	return l
}

func (a *app) expect(want int, auth, method, path string, body any) map[string]any {
	a.t.Helper()
	code, m, _ := a.call(auth, method, path, body)
	if code != want {
		a.t.Fatalf("%s %s → %d, want %d (%v)", method, path, code, want, m)
	}
	return m
}

func num(m map[string]any, k string) float64 { v, _ := m[k].(float64); return v }
func id(m map[string]any) string             { return itoa(int64(num(m, "id"))) }

// seed gives a person a home, a boiler, upkeep, a log entry and a meter.
func (a *app) seed(tok string) map[string]string {
	h := a.ok(tok, "POST", "/api/v1/homes", map[string]any{"name": "House", "currency": "EUR"})
	t := a.ok(tok, "POST", "/api/v1/things", map[string]any{"home_id": num(h, "id"), "name": "Gas boiler", "category": "heating", "room": "Cellar"})
	j := a.ok(tok, "POST", "/api/v1/jobs", map[string]any{"thing_id": num(t, "id"), "title": "Boiler service", "every_n": 1, "every_unit": "year", "due_on": store.AddDays(store.Today(), -3)})
	r := a.ok(tok, "POST", "/api/v1/records", map[string]any{"thing_id": num(t, "id"), "title": "Fixed the valve", "kind": "repair", "cost": 9800, "money": false})
	m := a.ok(tok, "POST", "/api/v1/meters", map[string]any{"home_id": num(h, "id"), "name": "Electricity", "kind": "electricity"})
	p := a.ok(tok, "POST", "/api/v1/pros", map[string]any{"name": "Marko", "trade": "Plumber"})
	rec := r["record"].(map[string]any)
	return map[string]string{"home": id(h), "thing": id(t), "job": id(j), "record": id(rec), "meter": id(m), "pro": id(p)}
}

func TestPerPersonIsolation(t *testing.T) {
	a := newApp(t, nil)
	al := a.seed(a.alice)
	// Bob sees none of Alice's things, and each of her ids is a 404 for him
	if n := len(a.list(a.bob, "/api/v1/homes")); n != 0 {
		t.Fatalf("bob sees %d homes", n)
	}
	for _, p := range []string{"/api/v1/homes/" + al["home"], "/api/v1/things/" + al["thing"], "/api/v1/jobs/" + al["job"], "/api/v1/records/" + al["record"],
		"/api/v1/meters/" + al["meter"], "/api/v1/pros/" + al["pro"], "/api/v1/meters/" + al["meter"] + "/readings"} {
		a.expect(404, a.bob, "GET", p, nil)
	}
	a.expect(404, a.bob, "PUT", "/api/v1/things/"+al["thing"], map[string]any{"name": "Mine now"})
	a.expect(404, a.bob, "DELETE", "/api/v1/records/"+al["record"], nil)
	a.expect(404, a.bob, "POST", "/api/v1/jobs/"+al["job"]+"/done", map[string]any{})
	a.expect(404, a.bob, "POST", "/api/v1/meters/"+al["meter"]+"/readings", map[string]any{"value": 5})
	// he cannot hang his own rows off her home, thing or pro either
	a.expect(404, a.bob, "POST", "/api/v1/things", map[string]any{"home_id": mustNum(al["home"]), "name": "x"})
	bh := a.ok(a.bob, "POST", "/api/v1/homes", map[string]any{"name": "Flat"})
	a.expect(404, a.bob, "POST", "/api/v1/records", map[string]any{"home_id": num(bh, "id"), "thing_id": mustNum(al["thing"]), "title": "x"})
	a.expect(404, a.bob, "POST", "/api/v1/jobs", map[string]any{"home_id": num(bh, "id"), "pro_id": mustNum(al["pro"]), "title": "x", "due_on": store.Today()})
	a.expect(404, a.bob, "POST", "/api/v1/attachments", map[string]any{"owner_kind": "thing", "owner_id": mustNum(al["thing"]), "kind": "url", "url": "https://example.com", "title": "x"})
	// search and the overview are his alone
	s := a.ok(a.bob, "GET", "/api/v1/search?q=boiler", nil)
	if len(s["things"].([]any)) != 0 || len(s["jobs"].([]any)) != 0 {
		t.Fatalf("bob's search finds alice's boiler: %v", s)
	}
	o := a.ok(a.bob, "GET", "/api/v1/overview", nil)
	if len(o["attention"].([]any)) != 0 {
		t.Fatal("bob's overview shows alice's upkeep")
	}
	// an admin does not see others' content either
	if n := len(a.list(a.alice, "/api/v1/homes")); n != 1 {
		t.Fatalf("alice sees %d homes, want only her own", n)
	}
}

func mustNum(s string) float64 {
	var f float64
	_ = json.Unmarshal([]byte(s), &f)
	return f
}

func TestJobDoneRollsForward(t *testing.T) {
	a := newApp(t, nil)
	ids := a.seed(a.alice)
	due := a.ok(a.alice, "GET", "/api/v1/due", nil)
	if num(due["counts"].(map[string]any), "overdue") != 1 {
		t.Fatalf("want one overdue job, got %v", due["counts"])
	}
	today := store.Today()
	res := a.ok(a.alice, "POST", "/api/v1/jobs/"+ids["job"]+"/done", map[string]any{"cost": 12000, "notes": "all fine", "money": false})
	j := res["job"].(map[string]any)
	if j["due_on"] != store.AddInterval(today, 1, "year") || j["status"] != "ok" || j["last_done"] != today {
		t.Fatalf("after done: %v", j)
	}
	rec := res["record"].(map[string]any)
	if num(rec, "cost") != 12000 || rec["job_title"] != "Boiler service" || rec["thing_name"] != "Gas boiler" {
		t.Fatalf("log entry: %v", rec)
	}
	// a one-off is finished, and done once only
	once := a.ok(a.alice, "POST", "/api/v1/jobs", map[string]any{"home_id": mustNum(ids["home"]), "title": "Fix the gate", "every_n": 0, "due_on": today})
	a.ok(a.alice, "POST", "/api/v1/jobs/"+id(once)+"/done", map[string]any{"log": false})
	a.expect(409, a.alice, "POST", "/api/v1/jobs/"+id(once)+"/done", map[string]any{})
	g := a.ok(a.alice, "GET", "/api/v1/jobs/"+id(once), nil)["job"].(map[string]any)
	if g["status"] != "done" {
		t.Fatalf("one-off after done: %v", g)
	}
	// snooze and skip
	sn := a.ok(a.alice, "POST", "/api/v1/jobs/"+ids["job"]+"/snooze", map[string]any{"days": 7})
	if sn["due_on"] != store.AddDays(store.AddInterval(today, 1, "year"), 7) {
		t.Fatalf("snooze: %v", sn["due_on"])
	}
	a.expect(400, a.alice, "POST", "/api/v1/jobs/"+id(once)+"/skip", nil)
}

func TestAddInterval(t *testing.T) {
	for _, c := range [][4]string{{"2026-01-31", "1", "month", "2026-02-28"}, {"2024-01-31", "1", "month", "2024-02-29"}, {"2026-03-15", "3", "month", "2026-06-15"},
		{"2024-02-29", "1", "year", "2025-02-28"}, {"2026-11-04", "2", "week", "2026-11-18"}, {"2026-12-31", "1", "day", "2027-01-01"}} {
		n := int(mustNum(c[1]))
		if got := store.AddInterval(c[0], n, c[2]); got != c[3] {
			t.Errorf("%s + %d %s = %s, want %s", c[0], n, c[2], got, c[3])
		}
	}
}

func TestMeterUsage(t *testing.T) {
	a := newApp(t, nil)
	ids := a.seed(a.alice)
	base := store.AddDays(store.Today(), -60)
	for i, v := range []float64{1000, 1300, 1600} {
		a.ok(a.alice, "POST", "/api/v1/meters/"+ids["meter"]+"/readings", map[string]any{"read_on": store.AddDays(base, 30*i), "value": v})
	}
	// lower than the last reading without "new meter" is refused
	a.expect(400, a.alice, "POST", "/api/v1/meters/"+ids["meter"]+"/readings", map[string]any{"value": 10})
	m := a.ok(a.alice, "GET", "/api/v1/meters/"+ids["meter"], nil)
	rs := m["readings"].([]any)
	if len(rs) != 3 || num(rs[0].(map[string]any), "usage") != 300 {
		t.Fatalf("readings: %v", rs)
	}
	if pd := num(m["summary"].(map[string]any), "per_day"); pd < 9.9 || pd > 10.1 {
		t.Fatalf("per day = %v, want 10", pd)
	}
}

func TestLinkScopes(t *testing.T) {
	req := func(method, path string) *http.Request { return httptest.NewRequest(method, path, nil) }
	read := []string{"home:read"}
	write := []string{"home:write"}
	for _, c := range []struct {
		scopes []string
		r      *http.Request
		want   bool
	}{
		{nil, req("GET", "/api/v1/auth/me"), true},
		{read, req("GET", "/api/v1/overview"), true},
		{read, req("GET", "/api/v1/due"), true},
		{read, req("GET", "/api/v1/things/4"), true},
		{read, req("GET", "/api/v1/meters/2/usage"), true},
		{read, req("GET", "/api/v1/day/2026-11-04"), true},
		{read, req("POST", "/api/v1/records"), false},
		{write, req("POST", "/api/v1/records"), true},
		{write, req("POST", "/api/v1/jobs/3/done"), true},
		{write, req("POST", "/api/v1/meters/3/readings"), true},
		{write, req("GET", "/api/v1/overview"), false},
		{append(read, write...), req("DELETE", "/api/v1/things/3"), false},
		{append(read, write...), req("PUT", "/api/v1/settings"), false},
		{append(read, write...), req("GET", "/api/v1/export.json"), false},
		{append(read, write...), req("GET", "/api/v1/backup"), false},
		{append(read, write...), req("POST", "/api/v1/tokens"), false},
		{append(read, write...), req("GET", "/api/v1/links/money/accounts"), false},
	} {
		if got := linkAllows(c.scopes, c.r); got != c.want {
			t.Errorf("%v %s %s = %v, want %v", c.scopes, c.r.Method, c.r.URL.Path, got, c.want)
		}
	}
}

func TestExportImportIdempotent(t *testing.T) {
	a := newApp(t, nil)
	a.seed(a.alice)
	code, doc, _ := a.call(a.alice, "GET", "/api/v1/export.json", nil)
	if code != 200 || doc["app"] != "pocket-home" {
		t.Fatalf("export: %d %v", code, doc["app"])
	}
	first := a.ok(a.bob, "POST", "/api/v1/import", doc)
	if num(first, "homes") != 1 || num(first, "things") != 1 || num(first, "jobs") != 1 || num(first, "records") != 1 || num(first, "meters") != 1 || num(first, "pros") != 1 {
		t.Fatalf("first import: %v", first)
	}
	second := a.ok(a.bob, "POST", "/api/v1/import", doc)
	for k, v := range second {
		if v.(float64) != 0 {
			t.Fatalf("second import added %v %s", v, k)
		}
	}
	recs := a.ok(a.bob, "GET", "/api/v1/records", nil)["records"].([]any)
	if len(recs) != 1 || recs[0].(map[string]any)["thing_name"] != "Gas boiler" || recs[0].(map[string]any)["money_tx_id"] != nil {
		t.Fatalf("bob's imported log: %v", recs)
	}
	a.expect(400, a.bob, "POST", "/api/v1/import", map[string]any{"app": "pocket-still"})
}

func TestSettingsPerPerson(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"currency": "BAM", "notify_url": "http://apprise/notify/alice", "money_mode": "direct", "money_token": "secret-token-1234", "title": "Our place"})
	b := a.ok(a.bob, "GET", "/api/v1/settings", nil)
	if b["currency"] != "EUR" || b["notify_url"] != "" || b["money_token"] != "" || b["money_mode"] != "direct" && b["money_mode"] != "core" {
		t.Fatalf("bob got alice's settings: %v", b)
	}
	if b["title"] != "Our place" {
		t.Fatalf("the admin's title is box-wide, bob sees %v", b["title"])
	}
	a.ok(a.bob, "PUT", "/api/v1/settings", map[string]any{"title": "Bob's"})
	if al := a.ok(a.alice, "GET", "/api/v1/settings", nil); al["title"] != "Our place" || al["money_token"] != "secr••••1234" || al["currency"] != "BAM" {
		t.Fatalf("alice after bob's change: title %v token %v", al["title"], al["money_token"])
	}
	a.expect(400, a.alice, "PUT", "/api/v1/settings", map[string]any{"currency": "euro"})
}

func TestTemplates(t *testing.T) {
	cases := map[string]string{"Gas boiler": "Annual boiler service", "Bosch dishwasher": "Clean the dishwasher filter", "Smoke alarms": "Test the smoke alarms", "Electrical panel": "Test the RCD switches"}
	for name, want := range cases {
		if got := templatesFor(&store.Thing{Name: name, Category: "other"}); len(got) == 0 || got[0].Title != want {
			t.Errorf("%s → %v, want %s first", name, got, want)
		}
	}
	if got := templatesFor(&store.Thing{Name: "Thing", Category: "safety"}); got[0].Title != "Test it" {
		t.Errorf("category fallback: %v", got)
	}
}

func TestDemoSeeds(t *testing.T) {
	a := newApp(t, nil)
	if err := a.s.seedDemo(context.Background(), a.ids["Alice"]); err != nil {
		t.Fatal(err)
	}
	o := a.ok(a.alice, "GET", "/api/v1/overview", nil)
	if len(o["homes"].([]any)) != 2 || len(o["attention"].([]any)) == 0 || len(o["meters"].([]any)) != 4 {
		t.Fatalf("demo overview: homes %d attention %d meters %d", len(o["homes"].([]any)), len(o["attention"].([]any)), len(o["meters"].([]any)))
	}
	if pl := o["plants"].(map[string]any); num(pl, "count") != 9 || num(pl, "water_due") < 2 {
		t.Fatalf("demo plants: %v", pl)
	}
	// twice is a no-op
	if err := a.s.seedDemo(context.Background(), a.ids["Alice"]); err != nil {
		t.Fatal(err)
	}
	if n := len(a.list(a.alice, "/api/v1/homes")); n != 2 {
		t.Fatalf("demo seeded twice: %d homes", n)
	}
}
