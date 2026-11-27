package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// What Pocket Search, Pocket Day and Pocket Hub ask Home (the capabilities
// value in links.go): one route each, in the shape every Pocket app answers
// in. They read what /search, /day and the first page read, as the
// request's person; only the capture adds something, one upkeep job made
// the way POST /jobs makes it.

// The kinds in plain words, as the pages name them.
var (
	recordKindWords = map[string]string{"maintenance": "upkeep", "repair": "repair", "improvement": "improvement", "inspection": "check", "purchase": "purchase",
		"plant": "plant care", "other": "other"}
	meterKindWords = map[string]string{"electricity": "Electricity", "water": "Water", "gas": "Gas", "heating": "Heating", "solar": "Solar", "oil": "Heating oil"}
)

// upperFirst starts a line with a capital letter.
func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 || unicode.IsUpper(r) {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// dots joins the parts of a line that are there: "House · Cellar · last done 3 days ago".
func dots(parts ...string) string {
	out := parts[:0:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

// hasWord reports whether a search has a letter or a digit to look for.
func hasWord(q string) bool {
	return strings.IndexFunc(q, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
}

// relDay says a date next to today: "today", "tomorrow", "in 3 days",
// "yesterday", "5 days ago", or "on 2027-03-01" further away.
func relDay(date, today string) string {
	if date == "" {
		return ""
	}
	d := store.DaysBetween(today, date)
	switch {
	case d == 0:
		return "today"
	case d == 1:
		return "tomorrow"
	case d == -1:
		return "yesterday"
	case d > 1 && d <= 31:
		return fmt.Sprintf("in %d days", d)
	case d < -1 && d >= -31:
		return fmt.Sprintf("%d days ago", -d)
	}
	return "on " + date
}

// lateOrWhen is when open upkeep is due, for a widget row: "3 days late",
// "today", "tomorrow", "in 2 days".
func lateOrWhen(due, today string) string {
	if d := store.DaysBetween(today, due); d < 0 {
		return fmt.Sprintf("%d %s late", -d, dayWord(-d))
	}
	return relDay(due, today)
}

// dueWords is when a job is due, for a line of details: "3 days late",
// "due today", "due in 12 days", "paused", "done".
func dueWords(j store.Job, today string) string {
	switch {
	case j.Status == store.StatusDone:
		return "done"
	case j.Status == store.StatusPaused:
		return "paused"
	case j.DueOn < today:
		return lateOrWhen(j.DueOn, today)
	}
	return "due " + relDay(j.DueOn, today)
}

// jobTitle is a job as the lists show it: plant care with its plant
// ("Water Monstera"), other upkeep by its own title.
func jobTitle(j store.Job) string {
	if j.PlantID != nil {
		return jobName(j)
	}
	return j.Title
}

// jobPage opens a job in the app: the plant's page for plant care.
func jobPage(j store.Job) string {
	if j.PlantID != nil {
		return "/plants/" + itoa(*j.PlantID)
	}
	return "/upkeep/" + itoa(j.ID)
}

// countWords: "1 job", "3 jobs".
func countWords(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// number is a reading without trailing zeros: "1600", "12.35".
func number(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// activeHomes counts the homes someone still looks after: with more than
// one, a line names the home.
func activeHomes(homes []store.Home) int {
	n := 0
	for _, h := range homes {
		if !h.Archived {
			n++
		}
	}
	return n
}

// ---------- search ----------

// pocketSearch answers Pocket Search: what /search finds, as one list with
// the closest matches first: a name or title that has the words, then
// what has them in its notes or details.
func (s *Server) pocketSearch(r *http.Request, q string, limit int) ([]core.SearchItem, error) {
	if !hasWord(q) {
		return nil, nil
	}
	ctx := r.Context()
	f := s.find(ctx, actorID(ctx), q, limit)
	today := store.Today()
	lq := strings.ToLower(q)
	score := func(title string) float64 {
		if strings.Contains(strings.ToLower(title), lq) {
			return 3
		}
		return 1
	}
	out := []core.SearchItem{}
	for _, h := range f.homes {
		// as the list of homes says it: "2 appliances · 1 overdue"
		things, late := "", ""
		if h.Things > 0 {
			things = countWords(h.Things, "appliance", "appliances")
		}
		if h.Overdue > 0 {
			late = fmt.Sprintf("%d overdue", h.Overdue)
		}
		out = append(out, core.SearchItem{ID: "home:" + itoa(h.ID), Title: h.Name, Kind: "home", URL: "/homes/" + itoa(h.ID), Snippet: dots(h.Address, things, late), Score: score(h.Name)})
	}
	for _, t := range f.things {
		next := ""
		switch {
		case t.NextDue != "" && t.NextDue < today:
			next = "upkeep " + lateOrWhen(t.NextDue, today)
		case t.NextDue != "":
			next = "next due " + relDay(t.NextDue, today)
		}
		last := ""
		if t.LastDone != "" {
			last = "last done " + relDay(t.LastDone, today)
		}
		out = append(out, core.SearchItem{ID: "appliance:" + itoa(t.ID), Title: t.Name, Kind: "appliance", URL: "/things/" + itoa(t.ID),
			Snippet: dots(t.HomeName, t.Room, next, last), Score: score(t.Name)})
	}
	for _, p := range f.plants {
		care := ""
		if p.Archived {
			care = "no longer here"
		} else {
			for _, c := range p.Care {
				if c.Status != store.StatusPaused {
					care = careTitle(c.Care) + " " + lateOrWhen(c.DueOn, today)
					break
				}
			}
		}
		out = append(out, core.SearchItem{ID: "plant:" + itoa(p.ID), Title: p.Name, Kind: "plant", URL: "/plants/" + itoa(p.ID),
			Snippet: dots(p.HomeName, p.Species, p.Room, care), Score: score(p.Name)})
	}
	for _, j := range f.jobs {
		last := ""
		if j.LastDone != "" {
			last = "last done " + relDay(j.LastDone, today)
		}
		title := jobTitle(j)
		out = append(out, core.SearchItem{ID: "job:" + itoa(j.ID), Title: title, Kind: "job", Date: j.DueOn, URL: jobPage(j),
			Snippet: dots(j.HomeName, firstNonEmpty(j.ThingName, j.PlantName), dueWords(j, today), last), Score: score(title)})
	}
	for _, rc := range f.recs {
		out = append(out, core.SearchItem{ID: "record:" + itoa(rc.ID), Title: rc.Title, Kind: "record", Date: rc.Date, URL: "/log/" + itoa(rc.ID),
			Snippet: dots(rc.HomeName, firstNonEmpty(rc.ThingName, rc.PlantName), rc.ProName, "done "+relDay(rc.Date, today)), Score: score(rc.Title)})
	}
	for _, p := range f.pros {
		// what the list of pros shows; the phone number is only on the pro's page
		company, work := p.Company, ""
		if company == p.Name {
			company = ""
		}
		if p.Jobs > 0 {
			work = countWords(p.Jobs, "job", "jobs") + ", last " + relDay(p.LastWork, today)
		}
		out = append(out, core.SearchItem{ID: "pro:" + itoa(p.ID), Title: p.Name, Kind: "pro", URL: "/pros/" + itoa(p.ID), Snippet: dots(p.Trade, company, work), Score: score(p.Name)})
	}
	for _, m := range f.meters {
		last, date := "", ""
		if m.Last != nil {
			last, date = "last read "+relDay(m.Last.ReadOn, today)+": "+strings.TrimSpace(number(m.Last.Value)+" "+m.Unit), m.Last.ReadOn
		}
		out = append(out, core.SearchItem{ID: "meter:" + itoa(m.ID), Title: m.Name, Kind: "meter", Date: date, URL: "/meters/" + itoa(m.ID),
			Snippet: dots(m.HomeName, meterKindWords[m.Kind], last), Score: score(m.Name)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---------- day ----------

// readingTitle: "Read the electricity meter", as the daily message says it.
func readingTitle(meter string) string {
	name := strings.ToLower(strings.TrimSpace(meter))
	if !strings.Contains(name, "meter") {
		name += " meter"
	}
	return "Read the " + name
}

// pocketDay answers Pocket Day from what /day reads: the work logged that
// day (with its cost), house upkeep that was due that day and is still
// open, the meters read and the plant care, one line per kind of care.
// Plant care that was only due is left out: watering would crowd the day.
// Days here are plain dates, so the time zone is not used.
func (s *Server) pocketDay(r *http.Request, date string, _ *time.Location) (*core.Day, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	d, err := s.dayOf(ctx, uid, date)
	if err != nil {
		return nil, err
	}
	homes, err := s.store.ListHomes(ctx, uid)
	if err != nil {
		return nil, err
	}
	home := func(name string) string {
		if activeHomes(homes) > 1 {
			return name
		}
		return ""
	}
	day := &core.Day{Date: date}
	done, due, read := 0, 0, 0
	for _, rc := range d.recs {
		if rc.PlantID != nil && rc.Kind == "plant" {
			continue // in the plant care below
		}
		it := core.DayItem{ID: "record:" + itoa(rc.ID), Title: rc.Title, Sub: dots(firstNonEmpty(rc.ThingName, rc.PlantName), rc.ProName, home(rc.HomeName)),
			Kind: recordKindWords[rc.Kind], Done: true, Note: rc.Notes, URL: "/log/" + itoa(rc.ID)}
		if rc.Cost > 0 {
			it.Amount, it.Currency = rc.Cost, rc.Currency
		}
		day.Items = append(day.Items, it)
		done++
	}
	for _, j := range d.due {
		if j.PlantID != nil {
			continue
		}
		day.Items = append(day.Items, core.DayItem{ID: "job:" + itoa(j.ID), Title: j.Title, Sub: dots(j.ThingName, home(j.HomeName)), Kind: "due", URL: jobPage(j)})
		due++
	}
	for _, rd := range d.readings {
		meter, _ := rd["meter"].(string)
		unit, _ := rd["unit"].(string)
		value, _ := rd["value"].(float64)
		id, _ := rd["meter_id"].(int64)
		sub := strings.TrimSpace(number(value) + " " + unit)
		if usage, _ := rd["usage"].(*float64); usage != nil {
			days, _ := rd["days"].(int)
			sub += " · " + strings.TrimSpace(number(*usage)+" "+unit) + " used"
			if days > 0 {
				sub += " in " + countWords(days, "day", "days")
			}
		}
		day.Items = append(day.Items, core.DayItem{ID: "meter:" + itoa(id), Title: readingTitle(meter), Sub: sub, Kind: "reading", URL: "/meters/" + itoa(id)})
		read++
	}
	plants, watered := map[int64]bool{}, true
	for _, c := range d.care {
		names, _ := c["plants"].([]string)
		ids, _ := c["plant_ids"].([]int64)
		kind, _ := c["care"].(string)
		label, _ := c["label"].(string)
		it := core.DayItem{ID: "care:" + firstNonEmpty(kind, "other"), Kind: "plant care", Done: true, URL: "/plants"}
		if len(ids) == 1 {
			it.Title, it.URL = careDoneTitle(kind, names[0]), "/plants/"+itoa(ids[0])
		} else {
			it.Title = label + ": " + strings.Join(names, ", ")
		}
		day.Items = append(day.Items, it)
		for _, id := range ids {
			plants[id] = true
		}
		watered = watered && kind == "water"
	}
	if len(day.Items) == 0 {
		return nil, nil
	}
	var parts []string
	if done > 0 {
		parts = append(parts, countWords(done, "job", "jobs")+" done")
	}
	if due > 0 {
		if done > 0 {
			parts = append(parts, fmt.Sprintf("%d due", due))
		} else {
			parts = append(parts, countWords(due, "job", "jobs")+" due")
		}
	}
	if read > 0 {
		parts = append(parts, countWords(read, "meter", "meters")+" read")
	}
	if n := len(plants); n > 0 {
		if watered {
			parts = append(parts, countWords(n, "plant", "plants")+" watered")
		} else {
			parts = append(parts, countWords(n, "plant", "plants")+" looked after")
		}
	}
	day.Summary = strings.Join(parts, ", ")
	return day, nil
}

// ---------- widget ----------

// pocketWidget answers Pocket Hub: house upkeep that needs doing, as on the
// first page (late first, the longest wait on top, then due today, then due
// soon), then the plants that need water today. The line is empty when
// nothing is due; someone without a home gets nothing to show.
func (s *Server) pocketWidget(r *http.Request) (*core.WidgetData, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	homes, err := s.store.ListHomes(ctx, uid)
	if err != nil {
		return nil, err
	}
	if len(homes) == 0 {
		return nil, nil
	}
	many := activeHomes(homes) > 1
	today := store.Today()
	jobs, err := s.store.ListJobs(ctx, uid, store.JobFilter{Scope: "house", Until: store.AddDays(today, 60)})
	if err != nil {
		return nil, err
	}
	water, err := s.store.ListJobs(ctx, uid, store.JobFilter{Scope: "plants", Until: today})
	if err != nil {
		return nil, err
	}
	w := &core.WidgetData{URL: "/"}
	counts := map[string]int{}
	for _, j := range jobs {
		switch j.Status {
		case store.StatusOverdue, store.StatusDue, store.StatusSoon:
			counts[j.Status]++
			where := j.ThingName
			if many {
				where = dots(j.ThingName, j.HomeName)
			}
			w.Items = append(w.Items, core.WidgetItem{ID: "job:" + itoa(j.ID), Title: j.Title, Detail: dots(lateOrWhen(j.DueOn, today), where), URL: jobPage(j)})
		}
	}
	thirsty := 0
	for _, j := range water {
		if j.Care != "water" || (j.Status != store.StatusOverdue && j.Status != store.StatusDue) {
			continue
		}
		thirsty++
		where := ""
		if many {
			where = j.HomeName
		}
		w.Items = append(w.Items, core.WidgetItem{ID: "plant:" + itoa(*j.PlantID), Title: jobName(j), Detail: dots(lateOrWhen(j.DueOn, today), where), URL: jobPage(j)})
	}
	var parts []string
	noun := false // the first count about upkeep says "job"
	for _, c := range []struct{ status, words string }{{store.StatusOverdue, "overdue"}, {store.StatusDue, "due today"}, {store.StatusSoon, "due soon"}} {
		n := counts[c.status]
		if n == 0 {
			continue
		}
		if !noun {
			parts, noun = append(parts, countWords(n, "job", "jobs")+" "+c.words), true
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", n, c.words))
		}
	}
	if thirsty == 1 {
		parts = append(parts, "1 plant needs water")
	} else if thirsty > 1 {
		parts = append(parts, fmt.Sprintf("%d plants need water", thirsty))
	}
	w.Line = strings.Join(parts, ", ")
	w.Metrics = []core.Metric{{Label: "Overdue", Value: strconv.Itoa(counts[store.StatusOverdue])}, {Label: "Due today", Value: strconv.Itoa(counts[store.StatusDue])},
		{Label: "Due soon", Value: strconv.Itoa(counts[store.StatusSoon])}, {Label: "To water", Value: strconv.Itoa(thirsty)}}
	return w, nil
}

// ---------- capture ----------

// captureHome finds the home a quick line is for and what is left to write
// down. The line may start with one of the person's homes' names (capitals
// do not matter; the longest name wins, a home still looked after before an
// archived one). "Cabin: fix the gate latch" and "Cabin, fix the gate latch"
// use the name as a label, so it is dropped; "Cabin roof leaks" is a
// sentence about it and stays whole. Without a name the line goes to the
// only home (the only one still looked after, or the only one at all).
func captureHome(homes []store.Home, line string) (*store.Home, string) {
	byName := make([]*store.Home, 0, len(homes))
	var active []*store.Home
	for i := range homes {
		byName = append(byName, &homes[i])
		if !homes[i].Archived {
			active = append(active, &homes[i])
		}
	}
	// ListHomes has the archived ones last; keep that among equal lengths
	sort.SliceStable(byName, func(i, j int) bool { return len(byName[i].Name) > len(byName[j].Name) })
	text := []rune(line)
	for _, h := range byName {
		name := []rune(strings.TrimSpace(h.Name))
		if len(name) == 0 || len(text) <= len(name) || !strings.EqualFold(string(text[:len(name)]), string(name)) {
			continue
		}
		rest := strings.TrimSpace(string(text[len(name)+1:]))
		switch sep := text[len(name)]; {
		case rest == "":
		case sep == ':' || sep == ',':
			return h, upperFirst(rest)
		case unicode.IsSpace(sep):
			return h, upperFirst(line)
		}
	}
	switch {
	case len(active) == 1:
		return active[0], upperFirst(line)
	case len(active) == 0 && len(homes) == 1:
		return &homes[0], upperFirst(line)
	}
	return nil, line
}

// pocketCapture keeps a quick line from Pocket Hub as a one-off upkeep job,
// due on the given day or today, made the way POST /jobs makes it (the
// calendar sync picks it up). The same request_id again answers with the
// job made the first time and adds nothing; the receipt is written in the
// job's own transaction.
func (s *Server) pocketCapture(r *http.Request, in core.CaptureInput) (*core.CaptureResult, error) {
	ctx, uid := r.Context(), actorID(r.Context())
	op, err := core.NewOperation(fmt.Sprintf("home:user:%d:%s", uid, core.CapturePath), in.RequestID, in)
	if err != nil {
		return nil, err
	}
	var res core.CaptureResult
	replay := func() (*core.CaptureResult, error) {
		found, err := op.Replay(ctx, s.store.DB, &res)
		switch {
		case errors.Is(err, core.ErrOperationConflict):
			return nil, fmt.Errorf("%w: this was already kept with other words", core.ErrCapture)
		case err != nil || !found:
			return nil, err
		}
		return &res, nil
	}
	if kept, err := replay(); kept != nil || err != nil {
		return kept, err
	}

	// the first line is the job, anything after it the notes
	line, notes := in.Title, in.Text
	if line == "" {
		line, notes, _ = strings.Cut(in.Text, "\n")
	}
	line, notes = strings.Join(strings.Fields(line), " "), strings.TrimSpace(notes)
	homes, err := s.store.ListHomes(ctx, uid)
	if err != nil {
		return nil, err
	}
	if len(homes) == 0 {
		return nil, fmt.Errorf("%w: add a home in Pocket Home first", core.ErrCapture)
	}
	home, title := captureHome(homes, line)
	if home == nil {
		example := homes[0].Name
		for _, h := range homes {
			if !h.Archived {
				example = h.Name
				break
			}
		}
		return nil, fmt.Errorf("%w: start with the home's name, e.g. \"%s: fix the gate latch\"", core.ErrCapture, example)
	}
	if len(title) > 160 { // a long line: the start is the title, all of it is in the notes
		cut := title[:157]
		for !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
		title, notes = strings.TrimSpace(cut)+"…", strings.TrimSpace(title+"\n"+notes)
	}
	due, once := firstNonEmpty(in.Due, store.Today()), 0
	shown := title
	if activeHomes(homes) > 1 {
		shown = home.Name + ": " + title
	}
	j, msg, err := s.newJobWith(ctx, uid, jobInput{HomeID: &home.ID, Title: &title, Notes: &notes, EveryN: &once, DueOn: &due}, map[string]bool{},
		func(tx *sql.Tx, j *store.Job) error {
			res = core.CaptureResult{ID: "job:" + itoa(j.ID), Title: shown, URL: "/upkeep/" + itoa(j.ID)}
			return op.Save(ctx, tx, res)
		})
	switch {
	case msg != "":
		return nil, fmt.Errorf("%w: %s", core.ErrCapture, msg)
	case err != nil:
		// the same line sent twice at once: the other one kept it
		if kept, rerr := replay(); kept != nil || rerr != nil {
			return kept, rerr
		}
		return nil, err
	}
	s.syncCalendarSoon(uid)
	return &core.CaptureResult{ID: "job:" + itoa(j.ID), Title: shown, URL: "/upkeep/" + itoa(j.ID)}, nil
}
