package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-home/internal/config"
	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// fakeCapCore is a Pocket Core that vouches for link tokens
// pcx_<alice|bob>_<search|day|widget|capture|read|write> from Pocket Hub
// and remembers the manifest Home registers with.
type fakeCapCore struct {
	*httptest.Server
	mu       sync.Mutex
	manifest map[string]any
}

func newCapApp(t *testing.T) (*app, *fakeCapCore) {
	t.Helper()
	fc := &fakeCapCore{}
	scopes := map[string]string{"search": core.ScopeSearch, "day": core.ScopeDay, "widget": core.ScopeWidget, "capture": core.ScopeCapture,
		"read": "home:read", "write": "home:write"}
	fc.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/apps/register":
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			fc.mu.Lock()
			fc.manifest = m
			fc.mu.Unlock()
			io.WriteString(w, `{}`)
		case "/api/v1/auth/introspect":
			var in struct{ Token string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			parts := strings.Split(in.Token, "_")
			if len(parts) != 3 || parts[0] != "pcx" || scopes[parts[2]] == "" {
				io.WriteString(w, `{"active":false}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "kind": "delegated", "audience": "home", "scope": scopes[parts[2]],
				"user": map[string]any{"id": 1, "sub": parts[1] + "@example.com", "email": parts[1] + "@example.com", "name": parts[1], "role": "member"},
				"app":  map[string]any{"id": 9, "slug": "hub", "name": "Pocket Hub"}})
		default:
			io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(fc.Close)
	return newApp(t, func(c *config.Config) { c.CoreURL, c.CoreRegistrationKey = fc.URL, "key" }), fc
}

// items are the rows of a capability answer.
func items(t *testing.T, m map[string]any) []map[string]any {
	t.Helper()
	l, ok := m["items"].([]any)
	if !ok {
		t.Fatalf("no items list in %v", m)
	}
	out := []map[string]any{}
	for _, v := range l {
		out = append(out, v.(map[string]any))
	}
	return out
}

// item finds the row with that title.
func item(t *testing.T, m map[string]any, title string) map[string]any {
	t.Helper()
	for _, it := range items(t, m) {
		if it["title"] == title {
			return it
		}
	}
	t.Fatalf("no %q in %v", title, m["items"])
	return nil
}

// capture sends a quick line as Pocket Hub would.
func capture(a *app, tok, requestID, text string, more map[string]any) (int, map[string]any) {
	a.t.Helper()
	body := map[string]any{"request_id": requestID, "kind": "job", "text": text}
	for k, v := range more {
		body[k] = v
	}
	code, m, _ := a.call(tok, "POST", "/api/v1/pocket/capture", body)
	return code, m
}

// jobsOf is someone's open upkeep, plant care left out.
func (a *app) jobsOf(tok string) []map[string]any {
	a.t.Helper()
	out := []map[string]any{}
	for _, v := range a.list(tok, "/api/v1/jobs?scope=house") {
		out = append(out, v.(map[string]any))
	}
	return out
}

func TestPocketSearch(t *testing.T) {
	a := newApp(t, nil)
	ids := a.seed(a.alice)
	today := store.Today()
	a.ok(a.alice, "PUT", "/api/v1/pros/"+ids["pro"], map[string]any{"name": "Marko", "trade": "Plumber", "phone": "+387 61 000 111", "company": "Boiler Fix"})
	res := a.ok(a.alice, "GET", "/api/v1/pocket/search?q=boiler", nil)
	all := items(t, res)
	if len(all) != 4 || all[0]["kind"] != "appliance" || all[0]["title"] != "Gas boiler" || all[0]["id"] != "appliance:"+ids["thing"] || all[0]["url"] != "/things/"+ids["thing"] ||
		all[0]["snippet"] != "House · Cellar · upkeep 3 days late · last done today" {
		t.Fatalf("the boiler leads what its name finds: %v", all)
	}
	// the log entry about the boiler comes after what has the word in its title
	if all[1]["title"] != "Boiler service" || all[2]["title"] != "Fixed the valve" {
		t.Fatalf("order: %v", all)
	}
	job := item(t, res, "Boiler service")
	if job["kind"] != "job" || job["id"] != "job:"+ids["job"] || job["url"] != "/upkeep/"+ids["job"] || job["date"] != store.AddDays(today, -3) ||
		job["snippet"] != "House · Gas boiler · 3 days late" {
		t.Fatalf("job: %v", job)
	}
	// the pro's company has the word; the phone number stays on the pro's page
	pro := item(t, res, "Marko")
	if pro["kind"] != "pro" || pro["id"] != "pro:"+ids["pro"] || pro["url"] != "/pros/"+ids["pro"] || pro["snippet"] != "Plumber · Boiler Fix" || pro["score"] != nil && num(pro, "score") != 1 {
		t.Fatalf("pro: %v", pro)
	}
	rec := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=valve", nil), "Fixed the valve")
	if rec["kind"] != "record" || rec["id"] != "record:"+ids["record"] || rec["url"] != "/log/"+ids["record"] || rec["date"] != today ||
		rec["snippet"] != "House · Gas boiler · done today" {
		t.Fatalf("record: %v", rec)
	}
	a.ok(a.alice, "POST", "/api/v1/meters/"+ids["meter"]+"/readings", map[string]any{"value": 1234.5, "read_on": store.AddDays(today, -2)})
	m := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=electric", nil), "Electricity")
	if m["kind"] != "meter" || m["id"] != "meter:"+ids["meter"] || m["url"] != "/meters/"+ids["meter"] || m["snippet"] != "House · Electricity · last read 2 days ago: 1234.5 kWh" {
		t.Fatalf("meter: %v", m)
	}
	p := a.ok(a.alice, "POST", "/api/v1/plants", map[string]any{"home_id": mustNum(ids["home"]), "name": "Monstera", "room": "Hall", "last_watered": store.AddDays(today, -7)})
	pl := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=monstera", nil), "Monstera")
	if pl["kind"] != "plant" || pl["id"] != "plant:"+id(p) || pl["url"] != "/plants/"+id(p) || pl["snippet"] != "House · Hall · Water today" {
		t.Fatalf("plant: %v", pl)
	}
	if w := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=monstera", nil), "Water Monstera"); w["kind"] != "job" || w["url"] != "/plants/"+id(p) {
		t.Fatalf("plant care: %v", w)
	}
	if h := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=house", nil), "House"); h["kind"] != "home" || h["url"] != "/homes/"+ids["home"] || h["snippet"] != "1 appliance · 1 overdue" {
		t.Fatalf("home: %v", h)
	}
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=boiler&limit=1", nil)); len(got) != 1 || got[0]["title"] != "Gas boiler" {
		t.Fatalf("limit 1: %v", got)
	}
	for _, q := range []string{"", "%25", "??", "*", "__"} {
		if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q="+q, nil)); len(got) != 0 {
			t.Fatalf("no words %q: %v", q, got)
		}
	}
	// another person's things never appear
	for _, q := range []string{"boiler", "valve", "marko", "electric", "monstera", "house"} {
		if got := items(t, a.ok(a.bob, "GET", "/api/v1/pocket/search?q="+q, nil)); len(got) != 0 {
			t.Fatalf("bob finds alice's %q: %v", q, got)
		}
	}
	a.expect(401, "", "GET", "/api/v1/pocket/search?q=boiler", nil)
}

func TestPocketDay(t *testing.T) {
	a := newApp(t, nil)
	ids := a.seed(a.alice)
	today := store.Today()
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2001-01-01", nil); d["date"] != "2001-01-01" || d["summary"] != nil || len(items(t, d)) != 0 {
		t.Fatalf("an empty day: %v", d)
	}
	a.expect(400, a.alice, "GET", "/api/v1/pocket/day/2026-13-01", nil)

	// the boiler service is done today, with a cost; a gutter job is due today
	a.ok(a.alice, "POST", "/api/v1/jobs/"+ids["job"]+"/done", map[string]any{"cost": 12000, "pro_id": mustNum(ids["pro"]), "money": false})
	gutter := a.ok(a.alice, "POST", "/api/v1/jobs", map[string]any{"home_id": mustNum(ids["home"]), "title": "Clean the gutters", "due_on": today})
	a.ok(a.alice, "POST", "/api/v1/meters/"+ids["meter"]+"/readings", map[string]any{"value": 1000, "read_on": store.AddDays(today, -30)})
	a.ok(a.alice, "POST", "/api/v1/meters/"+ids["meter"]+"/readings", map[string]any{"value": 1300})
	for _, name := range []string{"Monstera", "Pothos"} {
		p := a.ok(a.alice, "POST", "/api/v1/plants", map[string]any{"home_id": mustNum(ids["home"]), "name": name, "last_watered": store.AddDays(today, -7)})
		a.ok(a.alice, "POST", "/api/v1/plants/"+id(p)+"/done", map[string]any{})
	}
	d := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+today, nil)
	if d["summary"] != "2 jobs done, 1 due, 1 meter read, 2 plants watered" || len(items(t, d)) != 5 {
		t.Fatalf("day: %v", d)
	}
	svc := item(t, d, "Boiler service")
	if svc["done"] != true || num(svc, "amount") != 12000 || svc["currency"] != "EUR" || svc["kind"] != "upkeep" || svc["sub"] != "Gas boiler · Marko" || !strings.HasPrefix(svc["url"].(string), "/log/") {
		t.Fatalf("job done: %v", svc)
	}
	if v := item(t, d, "Fixed the valve"); v["done"] != true || num(v, "amount") != 9800 || v["kind"] != "repair" || v["url"] != "/log/"+ids["record"] {
		t.Fatalf("log entry: %v", v)
	}
	if g := item(t, d, "Clean the gutters"); g["done"] != nil || g["kind"] != "due" || g["id"] != "job:"+id(gutter) || g["url"] != "/upkeep/"+id(gutter) || g["amount"] != nil {
		t.Fatalf("due: %v", g)
	}
	if r := item(t, d, "Read the electricity meter"); r["sub"] != "1300 kWh · 300 kWh used in 30 days" || r["url"] != "/meters/"+ids["meter"] {
		t.Fatalf("reading: %v", r)
	}
	if c := item(t, d, "Watered: Pothos, Monstera"); c["done"] != true || c["kind"] != "plant care" || c["url"] != "/plants" {
		t.Fatalf("plant care: %v", c)
	}
	// done, the gutters leave the day's open upkeep
	a.ok(a.alice, "POST", "/api/v1/jobs/"+id(gutter)+"/done", map[string]any{"log": false})
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+today, nil); d["summary"] != "2 jobs done, 1 meter read, 2 plants watered" {
		t.Fatalf("after the gutters: %v", d["summary"])
	}
	// only due: the summary names the jobs
	a.ok(a.alice, "POST", "/api/v1/jobs", map[string]any{"home_id": mustNum(ids["home"]), "title": "Bleed the radiators", "due_on": store.AddDays(today, 5)})
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+store.AddDays(today, 5), nil); d["summary"] != "1 job due" || len(items(t, d)) != 1 {
		t.Fatalf("a day with upkeep due: %v", d)
	}
	// someone else's day is empty
	if d := a.ok(a.bob, "GET", "/api/v1/pocket/day/"+today, nil); d["summary"] != nil || len(items(t, d)) != 0 {
		t.Fatalf("bob's day: %v", d)
	}
}

func TestPocketWidget(t *testing.T) {
	a := newApp(t, nil)
	if w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil); w["line"] != nil || len(items(t, w)) != 0 || w["metrics"] != nil {
		t.Fatalf("without a home: %v", w)
	}
	ids := a.seed(a.alice)
	today := store.Today()
	soon := a.ok(a.alice, "POST", "/api/v1/jobs", map[string]any{"home_id": mustNum(ids["home"]), "title": "Smoke alarm batteries", "due_on": store.AddDays(today, 2)})
	a.ok(a.alice, "POST", "/api/v1/jobs", map[string]any{"home_id": mustNum(ids["home"]), "title": "Paint the fence", "due_on": store.AddDays(today, 200)})
	for _, name := range []string{"Monstera", "Pothos"} {
		a.ok(a.alice, "POST", "/api/v1/plants", map[string]any{"home_id": mustNum(ids["home"]), "name": name, "last_watered": store.AddDays(today, -7)})
	}
	w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)
	rows := items(t, w)
	if w["line"] != "1 job overdue, 1 due soon, 2 plants need water" || w["url"] != "/" || len(rows) != 4 {
		t.Fatalf("widget: %v", w)
	}
	if rows[0]["title"] != "Boiler service" || rows[0]["detail"] != "3 days late · Gas boiler" || rows[0]["url"] != "/upkeep/"+ids["job"] ||
		rows[1]["title"] != "Smoke alarm batteries" || rows[1]["detail"] != "in 2 days" || rows[1]["id"] != "job:"+id(soon) ||
		rows[2]["title"] != "Water Monstera" || rows[2]["detail"] != "today" || !strings.HasPrefix(rows[2]["url"].(string), "/plants/") {
		t.Fatalf("rows: %v", rows)
	}
	ms := w["metrics"].([]any)
	if len(ms) != 4 || ms[0].(map[string]any)["label"] != "Overdue" || ms[0].(map[string]any)["value"] != "1" || ms[2].(map[string]any)["value"] != "1" ||
		ms[3].(map[string]any)["label"] != "To water" || ms[3].(map[string]any)["value"] != "2" {
		t.Fatalf("metrics: %v", ms)
	}
	// with a second home, the rows say which
	a.ok(a.alice, "POST", "/api/v1/homes", map[string]any{"name": "Cabin"})
	if rows := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)); rows[0]["detail"] != "3 days late · Gas boiler · House" || rows[2]["detail"] != "today · House" {
		t.Fatalf("two homes: %v", rows)
	}
	// all done: nothing to say
	a.ok(a.alice, "POST", "/api/v1/jobs/"+ids["job"]+"/done", map[string]any{"log": false})
	a.ok(a.alice, "POST", "/api/v1/jobs/"+id(soon)+"/done", map[string]any{"log": false})
	a.ok(a.alice, "POST", "/api/v1/plants/done", map[string]any{})
	if w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil); w["line"] != nil || len(items(t, w)) != 0 {
		t.Fatalf("nothing due: %v", w)
	}
	// someone else's widget
	if w := a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil); len(items(t, w)) != 0 || w["line"] != nil {
		t.Fatalf("bob's widget: %v", w)
	}
}

func TestPocketCapture(t *testing.T) {
	a := newApp(t, nil)
	keep := func(tok, requestID, text string, more map[string]any) map[string]any {
		t.Helper()
		code, m := capture(a, tok, requestID, text, more)
		if code != 201 {
			t.Fatalf("capture %q → %d %v", text, code, m)
		}
		return a.ok(tok, "GET", "/api/v1/jobs/"+strings.TrimPrefix(m["id"].(string), "job:"), nil)["job"].(map[string]any)
	}
	refuse := func(tok, requestID, text, want string) {
		t.Helper()
		if code, m := capture(a, tok, requestID, text, nil); code != 400 || m["error"] != want {
			t.Fatalf("capture %q → %d %v, want 400 %q", text, code, m, want)
		}
	}
	refuse(a.alice, "req-0000000000000000", "Fix the gate latch", "add a home in Pocket Home first")

	// one home: the line is a one-off job there
	house := a.ok(a.alice, "POST", "/api/v1/homes", map[string]any{"name": "House"})
	code, first := capture(a, a.alice, "req-0000000000000001", "fix the gate latch", map[string]any{"due": "2026-12-05"})
	if code != 201 || first["title"] != "Fix the gate latch" || first["url"] != "/upkeep/"+strings.TrimPrefix(first["id"].(string), "job:") {
		t.Fatalf("capture: %d %v", code, first)
	}
	j := a.ok(a.alice, "GET", "/api/v1/jobs/"+strings.TrimPrefix(first["id"].(string), "job:"), nil)["job"].(map[string]any)
	if num(j, "home_id") != num(house, "id") || num(j, "every_n") != 0 || j["due_on"] != "2026-12-05" || j["title"] != "Fix the gate latch" || j["source"] != "web" {
		t.Fatalf("the job: %v", j)
	}
	// the same request again: the same answer, nothing added; other words are refused
	if code, again := capture(a, a.alice, "req-0000000000000001", "fix the gate latch", map[string]any{"due": "2026-12-05"}); code != 201 || again["id"] != first["id"] ||
		again["title"] != first["title"] || again["url"] != first["url"] || len(a.jobsOf(a.alice)) != 1 {
		t.Fatalf("a retry: %d %v, jobs %v", code, again, a.jobsOf(a.alice))
	}
	refuse(a.alice, "req-0000000000000001", "fix the gate hinge", "this was already kept with other words")
	if r := keep(a.alice, "req-0000000000000002", "Order salt for the softener\nthe blue bags,\n25 kg", nil); r["due_on"] != store.Today() || r["title"] != "Order salt for the softener" ||
		r["notes"] != "the blue bags,\n25 kg" {
		t.Fatalf("no due means today, more lines are notes: %v", r)
	}
	long := strings.Repeat("ž", 120)
	if r := keep(a.alice, "req-0000000000000003", long, nil); len(r["title"].(string)) > 160 || !strings.HasSuffix(r["title"].(string), "…") || r["notes"] != "Ž"+long[2:] {
		t.Fatalf("a long line: %v", r)
	}

	// several homes: the name in front says which
	cabin := a.ok(a.alice, "POST", "/api/v1/homes", map[string]any{"name": "Cabin"})
	lake := a.ok(a.alice, "POST", "/api/v1/homes", map[string]any{"name": "Cabin by the lake"})
	for i, c := range []struct {
		text, title string
		home        map[string]any
	}{
		{"Cabin: fix the gate latch", "Fix the gate latch", cabin},
		{"CABIN, sweep the chimney", "Sweep the chimney", cabin},
		{"cabin roof leaks", "Cabin roof leaks", cabin},
		{"house:   new doormat", "New doormat", house},
		{"Cabin by the lake: drain the pipes", "Drain the pipes", lake},
	} {
		if r := keep(a.alice, "req-10000000000000"+itoa(int64(10+i)), c.text, nil); num(r, "home_id") != num(c.home, "id") || r["title"] != c.title {
			t.Fatalf("%q → home %v %q, want home %v %q", c.text, r["home_id"], r["title"], c.home["id"], c.title)
		}
	}
	if code, m := capture(a, a.alice, "req-0000000000000004", "Cabin: oil the hinges", nil); code != 201 || m["title"] != "Cabin: Oil the hinges" {
		t.Fatalf("the answer names the home: %d %v", code, m)
	}
	before := len(a.jobsOf(a.alice))
	refuse(a.alice, "req-0000000000000005", "Fix the gate latch", `start with the home's name, e.g. "House: fix the gate latch"`)
	refuse(a.alice, "req-0000000000000006", "Cabins are nice", `start with the home's name, e.g. "House: fix the gate latch"`)
	if len(a.jobsOf(a.alice)) != before {
		t.Fatal("a refused line was kept")
	}
	// archived homes no longer count: the line goes to the one still looked after
	a.ok(a.alice, "PUT", "/api/v1/homes/"+id(cabin), map[string]any{"archived": true})
	a.ok(a.alice, "PUT", "/api/v1/homes/"+id(lake), map[string]any{"archived": true})
	if r := keep(a.alice, "req-0000000000000007", "Replace the hall bulb", nil); num(r, "home_id") != num(house, "id") {
		t.Fatalf("with one home left: %v", r)
	}

	// what the route checks itself
	a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "short", "text": "x"})
	a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-0000000000000009", "kind": "plant", "text": "x"})
	a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-0000000000000009", "text": "x", "due": "tomorrow"})

	// per person: bob's line never reaches alice's homes, a request id is his own
	refuse(a.bob, "req-0000000000000001", "House: fix the gate latch", "add a home in Pocket Home first")
	flat := a.ok(a.bob, "POST", "/api/v1/homes", map[string]any{"name": "Flat"})
	mine := len(a.jobsOf(a.alice))
	if r := keep(a.bob, "req-0000000000000001", "House: fix the gate latch", nil); num(r, "home_id") != num(flat, "id") || r["title"] != "House: fix the gate latch" {
		t.Fatalf("bob's line: %v", r)
	}
	if len(a.jobsOf(a.bob)) != 1 || len(a.jobsOf(a.alice)) != mine {
		t.Fatalf("bob's jobs %d, alice's %d (was %d)", len(a.jobsOf(a.bob)), len(a.jobsOf(a.alice)), mine)
	}
}

// A link token with one pocket:* scope reaches its one route and nothing else.
func TestPocketLinkTokens(t *testing.T) {
	a, _ := newCapApp(t)
	ids := a.seed(a.alice)
	today := store.Today()
	line := func(requestID string) map[string]any {
		return map[string]any{"request_id": requestID, "kind": "job", "text": "Fix the gate latch"}
	}
	for _, c := range []struct {
		tok, method, path string
		body              any
		want              int
	}{
		{"pcx_alice_search", "GET", "/api/v1/pocket/search?q=boiler", nil, 200},
		{"pcx_alice_day", "GET", "/api/v1/pocket/day/" + today, nil, 200},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/widget", nil, 200},
		{"pcx_alice_capture", "POST", "/api/v1/pocket/capture", line("req-0000000000000001"), 201},
		{"pcx_alice_widget", "GET", "/api/v1/auth/me", nil, 200},
		// another capability's route
		{"pcx_alice_search", "GET", "/api/v1/pocket/day/" + today, nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_search", "POST", "/api/v1/pocket/capture", line("req-0000000000000002"), 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/search?q=boiler", nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/search?q=boiler", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/day/" + today, nil, 403},
		{"pcx_alice_widget", "POST", "/api/v1/pocket/capture", line("req-0000000000000003"), 403},
		{"pcx_alice_capture", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_capture", "GET", "/api/v1/pocket/search?q=boiler", nil, 403},
		// the rest of the app
		{"pcx_alice_widget", "GET", "/api/v1/overview", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/due", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/search?q=boiler", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/things", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/pros/" + ids["pro"], nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/day/" + today, nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/records", nil, 403},
		{"pcx_alice_capture", "POST", "/api/v1/jobs", map[string]any{"title": "x"}, 403},
		{"pcx_alice_capture", "POST", "/api/v1/records", map[string]any{"title": "x"}, 403},
		{"pcx_alice_capture", "POST", "/api/v1/jobs/" + ids["job"] + "/done", map[string]any{}, 403},
		{"pcx_alice_capture", "PUT", "/api/v1/jobs/" + ids["job"], map[string]any{"title": "x"}, 403},
		{"pcx_alice_capture", "DELETE", "/api/v1/jobs/" + ids["job"], nil, 403},
		{"pcx_alice_capture", "GET", "/api/v1/settings", nil, 403},
		// the app's own scopes do not open the new routes
		{"pcx_alice_read", "GET", "/api/v1/pocket/search?q=boiler", nil, 403},
		{"pcx_alice_read", "GET", "/api/v1/pocket/day/" + today, nil, 403},
		{"pcx_alice_read", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_write", "POST", "/api/v1/pocket/capture", line("req-0000000000000004"), 403},
		{"pcx_alice_read", "GET", "/api/v1/search?q=boiler", nil, 200},
		{"pcx_nobody", "GET", "/api/v1/pocket/widget", nil, 401},
	} {
		if code, m, _ := a.call(c.tok, c.method, c.path, c.body); code != c.want {
			t.Errorf("%s %s %s → %d, want %d (%v)", c.tok, c.method, c.path, code, c.want, m)
		}
	}
	// the token acts for its person only
	if got := items(t, a.ok("pcx_alice_search", "GET", "/api/v1/pocket/search?q=boiler", nil)); len(got) == 0 || got[0]["title"] != "Gas boiler" {
		t.Fatalf("alice through the link: %v", got)
	}
	if got := items(t, a.ok("pcx_bob_search", "GET", "/api/v1/pocket/search?q=boiler", nil)); len(got) != 0 {
		t.Fatalf("bob through the link finds alice's boiler: %v", got)
	}
	if w := a.ok("pcx_bob_widget", "GET", "/api/v1/pocket/widget", nil); len(items(t, w)) != 0 {
		t.Fatalf("bob's widget through the link: %v", w)
	}
	if d := a.ok("pcx_bob_day", "GET", "/api/v1/pocket/day/"+today, nil); len(items(t, d)) != 0 {
		t.Fatalf("bob's day through the link: %v", d)
	}
	if code, m := capture(a, "pcx_bob_capture", "req-0000000000000001", "Fix the gate latch", nil); code != 400 || m["error"] != "add a home in Pocket Home first" {
		t.Fatalf("bob has no home: %d %v", code, m)
	}
	// only the one allowed capture landed, made as the app that sent it; a retry adds nothing
	if code, m := capture(a, "pcx_alice_capture", "req-0000000000000001", "Fix the gate latch", nil); code != 201 {
		t.Fatalf("retry through the link: %d %v", code, m)
	}
	var latch []map[string]any
	for _, j := range a.jobsOf(a.alice) {
		if j["title"] == "Fix the gate latch" {
			latch = append(latch, j)
		}
	}
	if len(latch) != 1 || latch[0]["source"] != "hub" || num(latch[0], "every_n") != 0 {
		t.Fatalf("alice's jobs: %v", a.jobsOf(a.alice))
	}
}

func TestPocketLinkScopes(t *testing.T) {
	req := func(m, p string) *http.Request { return httptest.NewRequest(m, p, nil) }
	for _, c := range []struct {
		scope string
		r     *http.Request
		want  bool
	}{
		{core.ScopeSearch, req("GET", "/api/v1/pocket/search?q=x"), true},
		{core.ScopeDay, req("GET", "/api/v1/pocket/day/2026-01-02"), true},
		{core.ScopeWidget, req("GET", "/api/v1/pocket/widget"), true},
		{core.ScopeCapture, req("POST", "/api/v1/pocket/capture"), true},
		{core.ScopeWidget, req("GET", "/api/v1/auth/me"), true},
		{core.ScopeSearch, req("POST", "/api/v1/pocket/search"), false},
		{core.ScopeDay, req("GET", "/api/v1/pocket/day/today"), false},
		{core.ScopeDay, req("GET", "/api/v1/day/2026-01-02"), false},
		{core.ScopeWidget, req("GET", "/api/v1/due"), false},
		{core.ScopeCapture, req("GET", "/api/v1/pocket/capture"), false},
		{core.ScopeCapture, req("POST", "/api/v1/jobs"), false},
		{"home:read", req("GET", "/api/v1/pocket/widget"), false},
		{"home:read", req("GET", "/api/v1/pocket/search"), false},
		{"home:write", req("POST", "/api/v1/pocket/capture"), false},
	} {
		if got := linkAllows([]string{c.scope}, c.r); got != c.want {
			t.Errorf("%s %s %s = %v, want %v", c.scope, c.r.Method, c.r.URL.Path, got, c.want)
		}
	}
}

// Home tells Core what it can do, so Search, Day and Hub find it on their own.
func TestManifestCapabilities(t *testing.T) {
	a, fc := newCapApp(t)
	if err := a.s.registerWithCore(context.Background()); err != nil {
		t.Fatal(err)
	}
	fc.mu.Lock()
	m := fc.manifest
	fc.mu.Unlock()
	if m["slug"] != "home" || m["category"] != "Home" {
		t.Fatalf("manifest: %v %v", m["slug"], m["category"])
	}
	search, _ := m["search"].(map[string]any)
	day, _ := m["day"].(map[string]any)
	widget, _ := m["widget"].(map[string]any)
	cap, _ := m["capture"].(map[string]any)
	if search == nil || day == nil || day["label"] != "Home" || day["one"] != "job" || day["many"] != "jobs" || widget == nil || widget["title"] != "Home upkeep" || cap == nil {
		t.Fatalf("capabilities: %v %v %v %v", search, day, widget, cap)
	}
	if b := search["bangs"].([]any); len(b) != 2 || b[0] != "home" || b[1] != "house" {
		t.Fatalf("bangs: %v", b)
	}
	if k := cap["kinds"].([]any)[0].(map[string]any); k["kind"] != "job" || len(k["fields"].([]any)) != 1 || k["fields"].([]any)[0] != "due" {
		t.Fatalf("capture kinds: %v", cap)
	}
	offered := map[string]bool{}
	for _, o := range m["offers"].([]any) {
		offered[o.(map[string]any)["scope"].(string)] = true
	}
	for _, sc := range []string{"home:read", "home:write", core.ScopeSearch, core.ScopeDay, core.ScopeWidget, core.ScopeCapture} {
		if !offered[sc] {
			t.Errorf("%s is not offered", sc)
		}
	}
}

// The same line sent twice at once still makes one job.
func TestPocketCaptureAtOnce(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "POST", "/api/v1/homes", map[string]any{"name": "House"})
	var wg sync.WaitGroup
	got := make([]map[string]any, 4)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, got[i] = capture(a, a.alice, "req-at-once-000000001", "Fix the gate latch", nil)
		}()
	}
	wg.Wait()
	for _, m := range got {
		if m["id"] != got[0]["id"] || m["id"] == nil {
			t.Fatalf("answers differ: %v", got)
		}
	}
	if n := len(a.jobsOf(a.alice)); n != 1 {
		t.Fatalf("jobs = %d, want 1", n)
	}
}
