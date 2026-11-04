package server

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// seedDemo fills the owner's account with two homes, their appliances,
// upkeep in every state, two years of log and meter readings (DEMO_DATA,
// for `make run`, the CI smoke test and the screenshots). Dates are
// relative to today, so the demo always looks current.
func (s *Server) seedDemo(ctx context.Context, uid int64) error {
	// with AUTH_MODE=none this runs inside the first request; finish even
	// when that client goes away
	ctx = context.WithoutCancel(ctx)
	if hs, _ := s.store.ListHomes(ctx, uid); len(hs) > 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(42))
	today := store.Today()
	day := func(n int) string { return store.AddDays(today, n) }
	i64 := func(v int64) *int64 { return &v }
	f64 := func(v float64) *float64 { return &v }
	iptr := func(v int) *int { return &v }

	house := store.Home{Name: "Linden Street house", Kind: "house", Address: "Linden Street 14", YearBuilt: iptr(1987), AreaM2: f64(148), Currency: "EUR", MoneyTag: "house",
		Notes: "Main water valve: in the cellar, left of the stairs. Gas shut-off: outside, by the meter box."}
	cabin := store.Home{Name: "Lake cabin", Kind: "cottage", Address: "Birch Point 3", YearBuilt: iptr(1972), AreaM2: f64(56), Currency: "EUR", MoneyTag: "cabin", Sort: 1}
	for _, h := range []*store.Home{&house, &cabin} {
		if err := s.store.CreateHome(ctx, uid, h); err != nil {
			return err
		}
	}

	pros := map[string]*store.Pro{
		"plumber":  {Name: "Marko Petrović", Trade: "Plumber", Phone: "+387 61 204 118", Email: "marko@example.com", Rating: 5, Notes: "Comes the same week. Cash or card."},
		"electric": {Name: "Elena Kovač", Trade: "Electrician", Phone: "+387 62 553 902", Rating: 4},
		"sweep":    {Name: "Clean Flue", Trade: "Chimney sweep", Company: "Clean Flue d.o.o.", Phone: "+387 33 610 445", Website: "https://example.com", Rating: 5},
		"roof":     {Name: "Ivan Horvat", Trade: "Roofer", Phone: "+387 65 118 330", Rating: 4},
		"hvac":     {Name: "CoolAir Service", Trade: "Heating and cooling", Company: "CoolAir", Phone: "+387 33 220 771", Email: "service@example.com", Rating: 4},
	}
	for _, k := range []string{"plumber", "electric", "sweep", "roof", "hvac"} {
		if err := s.store.CreatePro(ctx, uid, pros[k]); err != nil {
			return err
		}
	}

	type th struct {
		key  string
		home *store.Home
		t    store.Thing
	}
	things := []th{
		{"boiler", &house, store.Thing{Name: "Gas boiler", Category: "heating", Room: "Utility room", Brand: "Vaillant", Model: "ecoTEC plus 832", Serial: "21163500100", InstalledOn: day(-3700), Price: i64(215000), Vendor: "Thermo Centar", WarrantyUntil: day(-50), ManualURL: "https://example.com/manuals/ecotec-plus.pdf", Notes: "Filling loop under the boiler; pressure 1.2 bar cold."}},
		{"ac", &house, store.Thing{Name: "Heat pump air conditioner", Category: "cooling", Room: "Living room", Brand: "Daikin", Model: "Perfera FTXM35R", InstalledOn: day(-880), PurchasedOn: day(-890), Price: i64(139000), Vendor: "CoolAir", WarrantyUntil: day(945)}},
		{"heater", &house, store.Thing{Name: "Water heater", Category: "water", Room: "Bathroom", Brand: "Ariston", Model: "Velis Evo 80 L", InstalledOn: day(-2050), Price: i64(32900)}},
		{"dishwasher", &house, store.Thing{Name: "Dishwasher", Category: "kitchen", Room: "Kitchen", Brand: "Bosch", Model: "SMV4HVX31E", Serial: "FD 0102 00456", PurchasedOn: day(-690), Price: i64(64900), Vendor: "Tehnomarket", WarrantyUntil: day(40)}},
		{"washer", &house, store.Thing{Name: "Washing machine", Category: "appliance", Room: "Bathroom", Brand: "Miele", Model: "WWD 120 WCS", PurchasedOn: day(-1150), Price: i64(89900), WarrantyUntil: day(310)}},
		{"fridge", &house, store.Thing{Name: "Fridge freezer", Category: "kitchen", Room: "Kitchen", Brand: "Liebherr", Model: "CNsfd 5723", PurchasedOn: day(-2400), Price: i64(79900)}},
		{"hood", &house, store.Thing{Name: "Cooker hood", Category: "kitchen", Room: "Kitchen", Brand: "Elica", Model: "Hidden 60"}},
		{"smoke", &house, store.Thing{Name: "Smoke alarms", Category: "safety", Room: "Hallway", Brand: "Ei Electronics", Model: "Ei650 (5 pieces)", InstalledOn: day(-1500), Notes: "Hallway, both bedrooms, attic, cellar stairs. Sealed ten-year batteries."}},
		{"roof", &house, store.Thing{Name: "Roof and gutters", Category: "structure", Room: "Outside", Notes: "Clay tiles, last re-roofed 2009. Spare tiles in the garage loft."}},
		{"fuse", &house, store.Thing{Name: "Fuse box", Category: "electrical", Room: "Hallway", Brand: "Schneider", Model: "Resi9"}},
		{"mower", &house, store.Thing{Name: "Lawn mower", Category: "outdoor", Room: "Garden shed", Brand: "Honda", Model: "HRG 466", PurchasedOn: day(-1900), Price: i64(44900)}},
		{"stove", &cabin, store.Thing{Name: "Wood stove", Category: "heating", Room: "Living room", Brand: "Jøtul", Model: "F 118", InstalledOn: day(-5200)}},
		{"pump", &cabin, store.Thing{Name: "Water pump", Category: "water", Room: "Cellar", Brand: "Grundfos", Model: "SCALA1 3-45", InstalledOn: day(-1300), WarrantyUntil: day(-1300 + 730)}},
		{"taps", &cabin, store.Thing{Name: "Outdoor taps", Category: "outdoor", Room: "Outside"}},
	}
	tid := map[string]*store.Thing{}
	for i := range things {
		t := &things[i].t
		t.HomeID = things[i].home.ID
		if err := s.store.CreateThing(ctx, uid, t); err != nil {
			return err
		}
		tid[things[i].key] = t
	}

	type jb struct {
		thing, pro string
		title      string
		n          int
		unit       string
		dueIn      int
		soon       int
		est        int64
		notes      string
		paused     bool
		once       bool
	}
	jobs := []jb{
		{"boiler", "hvac", "Annual boiler service", 1, "year", 9, 30, 12000, "Ask them to check the expansion vessel too.", false, false},
		{"boiler", "", "Check the boiler pressure", 1, "month", 12, 3, 0, "1–1.5 bar when cold.", false, false},
		{"boiler", "", "Bleed the radiators", 1, "year", -4, 14, 0, "", false, false},
		{"ac", "", "Clean the AC filters", 3, "month", -6, 7, 0, "", false, false},
		{"ac", "hvac", "AC service and disinfection", 1, "year", 190, 30, 9000, "", false, false},
		{"heater", "", "Flush the water heater", 1, "year", 47, 14, 0, "", false, false},
		{"heater", "plumber", "Check the anode rod", 2, "year", 320, 30, 6000, "", false, false},
		{"dishwasher", "", "Clean the dishwasher filter", 1, "month", 2, 3, 0, "", false, false},
		{"washer", "", "Clean the washing machine filter", 3, "month", 25, 7, 0, "", false, false},
		{"fridge", "", "Clean the fridge coils", 1, "year", 120, 14, 0, "", false, false},
		{"hood", "", "Clean the cooker hood filter", 3, "month", 0, 7, 0, "Metal filters go in the dishwasher.", false, false},
		{"smoke", "", "Test the smoke alarms", 1, "month", 0, 3, 0, "", false, false},
		{"roof", "roof", "Clean the gutters", 6, "month", 16, 14, 8000, "", false, false},
		{"roof", "roof", "Inspect the roof", 1, "year", 150, 30, 0, "", false, false},
		{"fuse", "", "Test the RCD switches", 6, "month", 64, 14, 0, "", false, false},
		{"mower", "", "Service the lawnmower", 1, "year", 140, 30, 4500, "", true, false},
		{"stove", "sweep", "Chimney sweep", 1, "year", -12, 30, 8500, "Book before the first cold weekend.", false, false},
		{"taps", "", "Winterise the outdoor taps", 1, "year", 11, 21, 0, "Close the valve in the cellar, open the taps to drain.", false, false},
		{"pump", "plumber", "Replace the pressure switch", 0, "month", 33, 14, 7500, "It clicks on and off; Marko has the part ordered.", false, true},
	}
	jid := map[string]*store.Job{}
	for _, d := range jobs {
		t := tid[d.thing]
		j := store.Job{HomeID: t.HomeID, ThingID: &t.ID, Title: d.title, Notes: d.notes, EveryN: d.n, EveryUnit: d.unit, DueOn: day(d.dueIn), SoonDays: d.soon, Paused: d.paused, Source: "demo"}
		if d.once {
			j.EveryN = 0
		}
		if d.pro != "" {
			j.ProID = &pros[d.pro].ID
		}
		if d.est > 0 {
			j.EstCost = i64(d.est)
		}
		if j.EveryN > 0 {
			back := map[string]int{"day": 1, "week": 7, "month": 30, "year": 365}[j.EveryUnit] * j.EveryN
			j.LastDone = store.AddDays(j.DueOn, -back)
			if j.LastDone > today {
				j.LastDone = ""
			}
			j.DoneCount = 1 + rng.Intn(4)
		}
		if err := s.store.CreateJob(ctx, uid, &j); err != nil {
			return err
		}
		jid[d.title] = &j
	}

	// the log: two years of work, with costs
	type rc struct {
		ago        int
		thing, pro string
		job        string
		kind       string
		title      string
		cost       int64
		notes      string
	}
	recs := []rc{
		{356, "boiler", "hvac", "Annual boiler service", "maintenance", "Annual boiler service", 11500, "Burner cleaned, flue gas fine, expansion vessel topped up to 1 bar."},
		{721, "boiler", "hvac", "Annual boiler service", "maintenance", "Annual boiler service", 11000, ""},
		{512, "boiler", "plumber", "", "repair", "Replaced the boiler pressure relief valve", 9800, "It was dripping into the tundish."},
		{365 + 4, "boiler", "", "Bleed the radiators", "maintenance", "Bled the radiators", 0, ""},
		{98, "ac", "", "Clean the AC filters", "maintenance", "Cleaned the AC filters", 0, ""},
		{190, "ac", "", "Clean the AC filters", "maintenance", "Cleaned the AC filters", 0, ""},
		{175, "ac", "hvac", "AC service and disinfection", "maintenance", "AC service and disinfection", 8500, ""},
		{890, "ac", "hvac", "", "purchase", "Heat pump air conditioner installed", 139000, "Includes installation and the outdoor bracket."},
		{318, "heater", "", "Flush the water heater", "maintenance", "Flushed the water heater", 0, "A lot of limescale came out."},
		{412, "heater", "plumber", "Check the anode rod", "maintenance", "Replaced the anode rod", 5400, ""},
		{690, "dishwasher", "", "", "purchase", "New dishwasher", 64900, "The old Whirlpool gave up (pump)."},
		{241, "dishwasher", "plumber", "", "repair", "Fixed the dishwasher drain hose", 3500, "Kinked behind the cabinet."},
		{28, "dishwasher", "", "Clean the dishwasher filter", "maintenance", "Cleaned the dishwasher filter", 0, ""},
		{66, "washer", "", "Clean the washing machine filter", "maintenance", "Cleaned the washing machine filter", 0, "Found a coin and two hair clips."},
		{430, "washer", "", "", "repair", "Replaced the washing machine door seal", 6900, "Ordered from Miele, fitted myself."},
		{245, "fridge", "", "Clean the fridge coils", "maintenance", "Cleaned the fridge coils", 0, ""},
		{92, "hood", "", "Clean the cooker hood filter", "maintenance", "Cleaned the cooker hood filter", 0, ""},
		{30, "smoke", "", "Test the smoke alarms", "inspection", "Tested the smoke alarms", 0, "All five fine."},
		{61, "smoke", "", "Test the smoke alarms", "inspection", "Tested the smoke alarms", 0, ""},
		{168, "roof", "roof", "Clean the gutters", "maintenance", "Gutters cleaned", 8000, "Two brackets re-fixed above the terrace."},
		{350, "roof", "roof", "Clean the gutters", "maintenance", "Gutters cleaned", 7500, ""},
		{215, "roof", "roof", "Inspect the roof", "inspection", "Roof inspection after the storm", 0, "Four tiles replaced from the spares."},
		{300, "roof", "roof", "", "repair", "Re-sealed the chimney flashing", 32000, "Leak above the attic stairs."},
		{120, "fuse", "electric", "", "improvement", "New circuit and sockets for the home office", 41000, ""},
		{302, "fuse", "", "Test the RCD switches", "inspection", "Tested the RCD switches", 0, ""},
		{225, "mower", "", "Service the lawnmower", "maintenance", "Lawnmower service", 3900, "Blade sharpened, oil, spark plug."},
		{377, "stove", "sweep", "Chimney sweep", "maintenance", "Chimney swept", 8500, ""},
		{742, "stove", "sweep", "Chimney sweep", "maintenance", "Chimney swept", 8000, ""},
		{354, "taps", "", "Winterise the outdoor taps", "maintenance", "Drained the outdoor taps", 0, ""},
		{405, "pump", "plumber", "", "repair", "Replaced the pump's pressure vessel", 18500, ""},
		{150, "", "plumber", "", "improvement", "New bathroom tap and shower head", 21000, ""},
		{460, "", "", "", "improvement", "Painted the garden fence", 6400, "Three tins of stain, one weekend."},
		{80, "", "", "", "other", "Cleaned the windows", 0, ""},
	}
	for _, d := range recs {
		home := house.ID
		r := store.Record{HomeID: home, Date: day(-d.ago), Kind: d.kind, Title: d.title, Notes: d.notes, Cost: d.cost, Currency: "EUR", Source: "demo"}
		if d.thing != "" {
			t := tid[d.thing]
			r.ThingID, r.HomeID = &t.ID, t.HomeID
		}
		if d.pro != "" {
			r.ProID = &pros[d.pro].ID
		}
		if j, ok := jid[d.job]; ok {
			r.JobID = &j.ID
		}
		if err := s.store.CreateRecord(ctx, uid, &r); err != nil {
			return err
		}
	}

	// papers kept with things
	for _, a := range []store.Attachment{
		{OwnerKind: "thing", OwnerID: tid["boiler"].ID, Kind: "url", Title: "Installation and service manual", URL: "https://example.com/manuals/ecotec-plus.pdf"},
		{OwnerKind: "thing", OwnerID: tid["dishwasher"].ID, Kind: "url", Title: "Receipt and warranty card", URL: "https://example.com/receipts/dishwasher.pdf"},
		{OwnerKind: "thing", OwnerID: tid["ac"].ID, Kind: "url", Title: "Daikin Onecta app", URL: "https://example.com/onecta"},
	} {
		a := a
		if err := s.store.CreateAttachment(ctx, uid, &a); err != nil {
			return err
		}
	}

	// meters: readings on (about) the first of each month, two years back
	type mt struct {
		home   *store.Home
		m      store.Meter
		start  float64
		month  func(m int) float64 // usage in month m (1–12)
		months int
		every  int
	}
	meters := []mt{
		{&house, store.Meter{Name: "Electricity", Kind: "electricity", Unit: "kWh", PricePerUnit: f64(0.24), Number: "EL-4471023", RemindDay: 1}, 48210,
			func(m int) float64 { return 300 + 70*math.Cos(float64(m-1)/12*2*math.Pi) }, 24, 1},
		{&house, store.Meter{Name: "Gas", Kind: "gas", Unit: "m³", PricePerUnit: f64(0.92), Number: "G4-88120", RemindDay: 1}, 21880,
			func(m int) float64 { return math.Max(18, 125+115*math.Cos(float64(m-1)/12*2*math.Pi)) }, 24, 1},
		{&house, store.Meter{Name: "Water", Kind: "water", Unit: "m³", PricePerUnit: f64(2.15), Number: "W-30455", RemindDay: 1}, 1302,
			func(m int) float64 { return 8.5 + 2.5*math.Sin(float64(m-4)/12*2*math.Pi) }, 24, 1},
		{&cabin, store.Meter{Name: "Electricity", Kind: "electricity", Unit: "kWh", PricePerUnit: f64(0.26), Number: "EL-9920017"}, 11420,
			func(m int) float64 { return 60 + 40*math.Sin(float64(m-3)/12*2*math.Pi) }, 24, 3},
	}
	t0, _ := store.ParseDate(today)
	for _, d := range meters {
		m := d.m
		m.HomeID = d.home.ID
		if err := s.store.CreateMeter(ctx, uid, &m); err != nil {
			return err
		}
		v := d.start
		for back := d.months; back >= 0; back -= d.every {
			at := t0.AddDate(0, -back, 0)
			date := fmt.Sprintf("%04d-%02d-%02d", at.Year(), int(at.Month()), 1+rng.Intn(3))
			if date > today {
				continue
			}
			if back != d.months {
				for k := 0; k < d.every; k++ {
					month := int(at.AddDate(0, -k-1, 0).Month())
					v += d.month(month) * (0.9 + rng.Float64()*0.2)
				}
			}
			rd := store.Reading{MeterID: m.ID, ReadOn: date, Value: math.Round(v*10) / 10, Source: "demo"}
			if err := s.store.CreateReading(ctx, uid, &rd); err != nil {
				return err
			}
		}
	}
	s.log.Info("demo data seeded", "user", uid)
	return nil
}
