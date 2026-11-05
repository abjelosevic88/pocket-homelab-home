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

// plant adds a plant for a person in a new home and returns the plant and
// its care by kind.
func (a *app) plant(tok, name string, body map[string]any) (map[string]any, map[string]map[string]any) {
	a.t.Helper()
	h := a.ok(tok, "POST", "/api/v1/homes", map[string]any{"name": "Flat"})
	if body == nil {
		body = map[string]any{}
	}
	body["home_id"], body["name"] = num(h, "id"), name
	p := a.ok(tok, "POST", "/api/v1/plants", body)
	return p, careOf(p)
}

func careOf(p map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, c := range p["care"].([]any) {
		j := c.(map[string]any)
		out[j["care"].(string)] = j
	}
	return out
}

func TestPlantDefaultsAndWatering(t *testing.T) {
	a := newApp(t, nil)
	today := store.Today()
	p, care := a.plant(a.alice, "Monstera", map[string]any{"room": "Living room", "light": "bright", "last_watered": store.AddDays(today, -7)})
	if p["kind"] != "foliage" {
		t.Fatalf("kind guessed from the name = %v", p["kind"])
	}
	w, f, r := care["water"], care["fertilise"], care["repot"]
	if w == nil || num(w, "every_n") != 7 || w["every_unit"] != "day" || w["due_on"] != today || w["status"] != "due" {
		t.Fatalf("water: %v", w)
	}
	if f == nil || num(f, "every_n") != 1 || f["every_unit"] != "month" || r == nil || num(r, "every_n") != 2 || r["every_unit"] != "year" {
		t.Fatalf("fertilise %v, repot %v", f, r)
	}
	pid := id(p)
	// it is on the one due list, the overview and the plants list
	due := a.ok(a.alice, "GET", "/api/v1/due", nil)
	if num(due["counts"].(map[string]any), "due") != 1 || due["due"].([]any)[0].(map[string]any)["plant_name"] != "Monstera" {
		t.Fatalf("due: %v", due["counts"])
	}
	o := a.ok(a.alice, "GET", "/api/v1/overview", nil)
	if num(o["plants"].(map[string]any), "water_due") != 1 || len(o["attention"].([]any)) != 1 {
		t.Fatalf("overview plants: %v, attention %d", o["plants"], len(o["attention"].([]any)))
	}
	if ps := a.list(a.alice, "/api/v1/plants"); len(ps) != 1 || ps[0].(map[string]any)["status"] != "due" {
		t.Fatalf("plants: %v", ps)
	}
	// "Watered"
	res := a.ok(a.alice, "POST", "/api/v1/plants/"+pid+"/done", map[string]any{})
	rec := res["record"].(map[string]any)
	if rec["title"] != "Watered Monstera" || rec["kind"] != "plant" || rec["plant_name"] != "Monstera" || rec["date"] != today {
		t.Fatalf("log entry: %v", rec)
	}
	if j := res["job"].(map[string]any); j["due_on"] != store.AddDays(today, 7) || j["last_done"] != today {
		t.Fatalf("after watering: %v", j)
	}
	if pl := res["plant"].(map[string]any); pl["last_watered"] != today {
		t.Fatalf("last watered: %v", pl["last_watered"])
	}
	// the day page has it for Pocket Day
	d := a.ok(a.alice, "GET", "/api/v1/day/"+today, nil)
	pc := d["plant_care"].([]any)
	if len(pc) != 1 || pc[0].(map[string]any)["label"] != "Watered" || pc[0].(map[string]any)["plants"].([]any)[0] != "Monstera" {
		t.Fatalf("day plant care: %v", pc)
	}
	// care that is not on the schedule is only logged
	res = a.ok(a.alice, "POST", "/api/v1/plants/"+pid+"/done", map[string]any{"care": "mist", "notes": "dry week"})
	if res["job"] != nil || res["record"].(map[string]any)["title"] != "Misted Monstera" {
		t.Fatalf("mist: %v", res)
	}
	a.expect(400, a.alice, "POST", "/api/v1/plants/"+pid+"/done", map[string]any{"care": "sing"})
	// search finds it
	if s := a.ok(a.alice, "GET", "/api/v1/search?q=monst", nil); len(s["plants"].([]any)) != 1 {
		t.Fatalf("search: %v", s["plants"])
	}
}

func TestPlantSchedule(t *testing.T) {
	a := newApp(t, nil)
	// a schedule of its own, then changed: water stretched, mist added, repot removed
	p, care := a.plant(a.alice, "Snake plant", map[string]any{"light": "low", "care": []map[string]any{{"care": "water", "every_n": 3, "every_unit": "week"}, {"care": "repot", "every_n": 3, "every_unit": "year"}}})
	if p["kind"] != "succulent" || len(care) != 2 || num(care["water"], "every_n") != 3 {
		t.Fatalf("own schedule: kind %v care %v", p["kind"], care)
	}
	pid := id(p)
	p = a.ok(a.alice, "PUT", "/api/v1/plants/"+pid, map[string]any{"care": []map[string]any{{"care": "water", "every_n": 4}, {"care": "mist", "every_n": 1, "every_unit": "week"}, {"care": "repot", "every_n": 0}}})
	care = careOf(p)
	if len(care) != 2 || num(care["water"], "every_n") != 4 || care["water"]["every_unit"] != "week" || care["mist"] == nil || care["repot"] != nil {
		t.Fatalf("changed schedule: %v", care)
	}
	a.expect(400, a.alice, "PUT", "/api/v1/plants/"+pid, map[string]any{"care": []map[string]any{{"care": "dance", "every_n": 1}}})
	a.expect(400, a.alice, "POST", "/api/v1/plants", map[string]any{"name": "x", "light": "dark"})
	// it died: archived, its care paused and off the due list
	p = a.ok(a.alice, "PUT", "/api/v1/plants/"+pid, map[string]any{"archived": true, "gone_reason": "died"})
	if p["gone_on"] != store.Today() || p["status"] != "paused" {
		t.Fatalf("archived: %v", p)
	}
	for _, c := range p["care"].([]any) {
		if c.(map[string]any)["status"] != "paused" {
			t.Fatalf("care of an archived plant: %v", c)
		}
	}
	if n := len(a.list(a.alice, "/api/v1/plants")); n != 0 {
		t.Fatalf("archived plant still listed: %d", n)
	}
	if n := len(a.list(a.alice, "/api/v1/plants?archived=1")); n != 1 {
		t.Fatalf("archived=1 lists %d", n)
	}
	// deleted: its care and its care log go
	a.ok(a.alice, "POST", "/api/v1/plants/"+pid+"/done", map[string]any{"care": "water"})
	a.expect(204, a.alice, "DELETE", "/api/v1/plants/"+pid, nil)
	if js := a.list(a.alice, "/api/v1/jobs?scope=plants&all=1"); len(js) != 0 {
		t.Fatalf("care left behind: %v", js)
	}
	if recs := a.ok(a.alice, "GET", "/api/v1/records?scope=plants", nil)["records"].([]any); len(recs) != 0 {
		t.Fatalf("care log left behind: %v", recs)
	}
}

func TestWaterAllDue(t *testing.T) {
	a := newApp(t, nil)
	today := store.Today()
	h := a.ok(a.alice, "POST", "/api/v1/homes", map[string]any{"name": "Flat"})
	mk := func(name string, ago int) string {
		return id(a.ok(a.alice, "POST", "/api/v1/plants", map[string]any{"home_id": num(h, "id"), "name": name, "kind": "foliage", "last_watered": store.AddDays(today, -ago)}))
	}
	thirsty, late, fine := mk("Pothos", 7), mk("Ficus", 9), mk("Calathea", 2)
	res := a.ok(a.alice, "POST", "/api/v1/plants/done", map[string]any{})
	if num(res, "count") != 2 {
		t.Fatalf("water all due: %v", res)
	}
	for _, pid := range []string{thirsty, late} {
		pl := a.ok(a.alice, "GET", "/api/v1/plants/"+pid, nil)["plant"].(map[string]any)
		if pl["last_watered"] != today {
			t.Fatalf("plant %s not watered: %v", pid, pl)
		}
	}
	if pl := a.ok(a.alice, "GET", "/api/v1/plants/"+fine, nil)["plant"].(map[string]any); pl["last_watered"] == today {
		t.Fatal("a plant that was not due got watered")
	}
	// picked ones, any state
	res = a.ok(a.alice, "POST", "/api/v1/plants/done", map[string]any{"plant_ids": []any{mustNum(fine)}})
	if num(res, "count") != 1 {
		t.Fatalf("water picked: %v", res)
	}
}

func TestPlantIsolation(t *testing.T) {
	a := newApp(t, nil)
	p, care := a.plant(a.alice, "Monstera", map[string]any{"last_watered": store.AddDays(store.Today(), -8)})
	pid, water := id(p), id(care["water"])
	if n := len(a.list(a.bob, "/api/v1/plants")); n != 0 {
		t.Fatalf("bob sees %d plants", n)
	}
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/v1/plants/" + pid, nil},
		{"PUT", "/api/v1/plants/" + pid, map[string]any{"name": "Mine now"}},
		{"DELETE", "/api/v1/plants/" + pid, nil},
		{"POST", "/api/v1/plants/" + pid + "/done", map[string]any{}},
		{"GET", "/api/v1/jobs/" + water, nil},
		{"PUT", "/api/v1/jobs/" + water, map[string]any{"every_n": 1}},
		{"POST", "/api/v1/jobs/" + water + "/done", map[string]any{}},
		{"POST", "/api/v1/plants/done", map[string]any{"plant_ids": []any{mustNum(pid)}}},
	} {
		a.expect(404, a.bob, c.method, c.path, c.body)
	}
	// nor hang his own rows off her plant
	bh := a.ok(a.bob, "POST", "/api/v1/homes", map[string]any{"name": "Bob's"})
	a.expect(404, a.bob, "POST", "/api/v1/jobs", map[string]any{"home_id": num(bh, "id"), "plant_id": mustNum(pid), "care": "water", "every_n": 3, "every_unit": "day"})
	a.expect(404, a.bob, "POST", "/api/v1/records", map[string]any{"home_id": num(bh, "id"), "plant_id": mustNum(pid), "title": "x"})
	a.expect(404, a.bob, "POST", "/api/v1/attachments", map[string]any{"owner_kind": "plant", "owner_id": mustNum(pid), "kind": "url", "url": "https://example.com", "title": "x"})
	// "water all due" for Bob leaves Alice's thirsty plant alone
	if res := a.ok(a.bob, "POST", "/api/v1/plants/done", map[string]any{}); num(res, "count") != 0 {
		t.Fatalf("bob watered %v plants", res["count"])
	}
	if pl := a.ok(a.alice, "GET", "/api/v1/plants/"+pid, nil)["plant"].(map[string]any); pl["status"] != "overdue" || pl["name"] != "Monstera" {
		t.Fatalf("alice's plant after bob: %v", pl)
	}
	if s := a.ok(a.bob, "GET", "/api/v1/search?q=monstera", nil); len(s["plants"].([]any)) != 0 || len(s["jobs"].([]any)) != 0 {
		t.Fatalf("bob's search finds alice's plant: %v", s)
	}
	if d := a.ok(a.bob, "GET", "/api/v1/due", nil); num(d["counts"].(map[string]any), "overdue") != 0 {
		t.Fatal("bob's due list has alice's plant")
	}
}

func TestPlantLinkScopes(t *testing.T) {
	read := []string{"home:read"}
	write := []string{"home:write"}
	for _, c := range []struct {
		scopes       []string
		method, path string
		want         bool
	}{
		{read, "GET", "/api/v1/plants", true},
		{read, "GET", "/api/v1/plants/7", true},
		{read, "GET", "/api/v1/plants/templates", true},
		{read, "POST", "/api/v1/plants/7/done", false},
		{write, "POST", "/api/v1/plants/7/done", true},
		{write, "POST", "/api/v1/plants/done", true},
		{write, "GET", "/api/v1/plants", false},
		{append(read, write...), "POST", "/api/v1/plants", false},
		{append(read, write...), "PUT", "/api/v1/plants/7", false},
		{append(read, write...), "DELETE", "/api/v1/plants/7", false},
	} {
		if got := linkAllows(c.scopes, httptest.NewRequest(c.method, c.path, nil)); got != c.want {
			t.Errorf("%v %s %s = %v, want %v", c.scopes, c.method, c.path, got, c.want)
		}
	}
}

func TestPlantExportImport(t *testing.T) {
	a := newApp(t, nil)
	p, _ := a.plant(a.alice, "Basil", map[string]any{"room": "Kitchen", "light": "direct"})
	a.ok(a.alice, "POST", "/api/v1/plants/"+id(p)+"/done", map[string]any{"notes": "pesto"})
	gone, _ := a.plant(a.alice, "Calathea", nil)
	a.ok(a.alice, "PUT", "/api/v1/plants/"+id(gone), map[string]any{"archived": true, "gone_reason": "died"})
	_, doc, _ := a.call(a.alice, "GET", "/api/v1/export.json", nil)
	if len(doc["plants"].([]any)) != 2 {
		t.Fatalf("export plants: %v", doc["plants"])
	}
	first := a.ok(a.bob, "POST", "/api/v1/import", doc)
	if num(first, "plants") != 2 || num(first, "jobs") < 6 || num(first, "records") != 1 {
		t.Fatalf("first import: %v", first)
	}
	second := a.ok(a.bob, "POST", "/api/v1/import", doc)
	for k, v := range second {
		if v.(float64) != 0 {
			t.Fatalf("second import added %v %s", v, k)
		}
	}
	ps := a.list(a.bob, "/api/v1/plants?archived=1")
	if len(ps) != 2 {
		t.Fatalf("bob's plants: %v", ps)
	}
	for _, x := range ps {
		pl := x.(map[string]any)
		care := careOf(pl)
		switch pl["name"] {
		case "Basil":
			if pl["kind"] != "herb" || care["water"] == nil || pl["last_watered"] != store.Today() {
				t.Fatalf("bob's basil: %v", pl)
			}
		case "Calathea":
			if !pl["archived"].(bool) || pl["gone_reason"] != "died" || care["water"]["status"] != "paused" {
				t.Fatalf("bob's calathea: %v", pl)
			}
		}
	}
	recs := a.ok(a.bob, "GET", "/api/v1/records?scope=plants", nil)["records"].([]any)
	if len(recs) != 1 || recs[0].(map[string]any)["plant_name"] != "Basil" || recs[0].(map[string]any)["job_title"] != "Water" {
		t.Fatalf("bob's care log: %v", recs)
	}
	// alice still has hers, untouched
	if n := len(a.list(a.alice, "/api/v1/plants?archived=1")); n != 2 {
		t.Fatalf("alice has %d plants", n)
	}
}

func TestPlantTemplates(t *testing.T) {
	for name, want := range map[string]string{"Snake plant": "succulent", "Golden pothos": "foliage", "Basil": "herb", "Moth orchid": "orchid", "Boston fern": "fern",
		"Christmas cactus": "flowering", "Barrel cactus": "cactus", "Kentia palm": "palm", "Peace lily": "flowering", "Mystery": ""} {
		if got := guessPlantKind(name, ""); got != want {
			t.Errorf("%s → %q, want %q", name, got, want)
		}
	}
	water := func(kind, light string) int {
		for _, c := range careFor(kind, light) {
			if c.Care == "water" {
				return c.EveryN
			}
		}
		return 0
	}
	if water("foliage", "medium") != 7 || water("foliage", "low") != 11 || water("foliage", "direct") != 5 || water("cactus", "bright") != 21 {
		t.Errorf("watering by light: %d %d %d %d", water("foliage", "medium"), water("foliage", "low"), water("foliage", "direct"), water("cactus", "bright"))
	}
	if careDoneTitle("pests", "Ficus") != "Checked Ficus for pests" || careDoneTitle("repot", "Ficus") != "Repotted Ficus" {
		t.Error("care log titles")
	}
}

// Adding, changing and watering a plant puts home.plant_saved and
// home.plant_cared on the Core outbox.
func TestPlantEvents(t *testing.T) {
	cfg := testConfig(t.TempDir())
	cfg.CoreURL = "http://core.invalid"
	st := openTestStore(t, cfg)
	s := New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	ctx := context.Background()
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
	p, _ := a.plant(tok, "Fern", map[string]any{"kind": "fern"})
	a.ok(tok, "PUT", "/api/v1/plants/"+id(p), map[string]any{"room": "Bathroom"})
	a.ok(tok, "POST", "/api/v1/plants/"+id(p)+"/done", map[string]any{"care": "mist"})

	rows, err := st.DB.QueryContext(ctx, `SELECT body FROM pocket_outbox ORDER BY created_at, rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	topics := map[string][]map[string]any{}
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
		topics[ev.Topic] = append(topics[ev.Topic], ev.Payload)
	}
	if saved := topics["home.plant_saved"]; len(saved) != 2 || saved[1]["room"] != "Bathroom" || saved[1]["user_email"] != "ana@example.com" {
		t.Fatalf("plant_saved: %v", saved)
	}
	if cared := topics["home.plant_cared"]; len(cared) != 1 || cared[0]["care"] != "mist" || cared[0]["plant"] != "Fern" {
		t.Fatalf("plant_cared: %v", cared)
	}
	if n := len(topics["home.job_done"]); n != 0 {
		t.Fatalf("plant care also sent %d job_done", n)
	}
}

// The daily message groups plant care: one line per kind of care.
func TestPlantDailyLines(t *testing.T) {
	var p plantDue
	for _, j := range []store.Job{{Care: "water", Status: "due", PlantName: "Monstera"}, {Care: "fertilise", Status: "due", PlantName: "Ficus"},
		{Care: "water", Status: "overdue", PlantName: "Basil"}, {Care: "water", Status: "due", PlantName: "Pothos"}} {
		p.add(j)
	}
	got := p.lines()
	want := []string{"• Water — overdue: Basil", "• Water today: Monstera, Pothos", "• Fertilise today: Ficus"}
	if len(got) != len(want) {
		t.Fatalf("lines: %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}
