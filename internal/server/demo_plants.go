package server

import (
	"context"
	"math/rand"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// seedDemoPlants adds house plants with their care and months of history:
// a couple to water today, one a little late, one that died.
func (s *Server) seedDemoPlants(ctx context.Context, uid int64, rng *rand.Rand, house, cabin *store.Home) error {
	today := store.Today()
	day := func(n int) string { return store.AddDays(today, n) }
	type pl struct {
		home     *store.Home
		p        store.Plant
		ago      int            // acquired, days ago
		lastAgo  map[string]int // care → days since it was last done
		notes    map[int]string // the n-th watering back gets a note
		repotAgo int
		gone     int // archived this many days ago (0 = alive)
	}
	plants := []pl{
		{house, store.Plant{Name: "Monstera", Species: "Monstera deliciosa", Kind: "foliage", Room: "Living room", Light: "bright", Pot: "24 cm terracotta, moss pole",
			Notes: "Wipe the leaves now and then; it drinks more in summer."}, 1100, map[string]int{"water": 7, "fertilise": 12, "rotate": 3, "pests": 20},
			map[int]string{2: "New leaf unfurling.", 9: "Cut off a yellow leaf at the bottom."}, 380, 0},
		{house, store.Plant{Name: "Golden pothos", Species: "Epipremnum aureum", Kind: "foliage", Room: "Kitchen", Light: "medium", Pot: "Hanging pot above the counter"}, 820,
			map[string]int{"water": 9, "fertilise": 33, "rotate": 10, "pests": 25}, map[int]string{5: "Took three cuttings for the office."}, 0, 0},
		{house, store.Plant{Name: "Snake plant", Species: "Dracaena trifasciata", Kind: "succulent", Room: "Bedroom", Light: "low", Pot: "18 cm ceramic, gritty soil"}, 1500,
			map[string]int{"water": 8, "fertilise": 40, "rotate": 12}, nil, 0, 0},
		{house, store.Plant{Name: "Basil", Species: "Ocimum basilicum", Kind: "herb", Room: "Kitchen windowsill", Light: "direct", Notes: "From the supermarket; split into three pots."}, 64,
			map[string]int{"water": 2, "prune": 9, "fertilise": 20}, map[int]string{4: "Pesto night."}, 0, 0},
		{house, store.Plant{Name: "Fiddle-leaf fig", Species: "Ficus lyrata", Kind: "foliage", Room: "Living room", Light: "bright", Pot: "30 cm plastic in a basket",
			Notes: "Hates being moved. Keep it away from the radiator."}, 640, map[string]int{"water": 3, "fertilise": 26, "rotate": 6, "pests": 8}, nil, 0, 0},
		{house, store.Plant{Name: "Peace lily", Species: "Spathiphyllum", Kind: "flowering", Room: "Bathroom", Light: "low", Notes: "Droops when thirsty, perks up within the hour."}, 430,
			map[string]int{"water": 6, "fertilise": 10, "prune": 4, "pests": 15}, map[int]string{3: "Two new flowers."}, 0, 0},
		{house, store.Plant{Name: "Boston fern", Species: "Nephrolepis exaltata", Kind: "fern", Room: "Bathroom", Light: "medium", Pot: "On a tray of wet pebbles"}, 300,
			map[string]int{"water": 1, "mist": 3, "fertilise": 18, "pests": 30}, nil, 0, 0},
		{house, store.Plant{Name: "Moth orchid", Species: "Phalaenopsis", Kind: "orchid", Room: "Bedroom", Light: "bright", Pot: "Clear pot, bark", Notes: "A birthday present."}, 520,
			map[string]int{"water": 4, "fertilise": 22, "mist": 2}, nil, 200, 0},
		{cabin, store.Plant{Name: "Jade plant", Species: "Crassula ovata", Kind: "succulent", Room: "Living room", Light: "direct", Notes: "The neighbours water it when we are away."}, 2000,
			map[string]int{"water": 12, "fertilise": 50, "rotate": 20}, nil, 0, 0},
		{house, store.Plant{Name: "Calathea", Species: "Calathea orbifolia", Kind: "foliage", Room: "Hallway", Light: "low", Notes: "Crispy edges from the start; the hallway was too dry."}, 260,
			map[string]int{"water": 40, "fertilise": 70}, nil, 0, 38},
	}
	for _, d := range plants {
		p := d.p
		p.HomeID, p.AcquiredOn = d.home.ID, day(-d.ago)
		if err := s.store.CreatePlant(ctx, uid, &p); err != nil {
			return err
		}
		for _, c := range careFor(p.Kind, p.Light) {
			j := careJob(&p, c, today)
			j.Source = "demo"
			every := intervalDays(j)
			last, known := d.lastAgo[c.Care]
			if c.Care == "repot" && d.repotAgo > 0 {
				last, known = d.repotAgo, true
			}
			if !known {
				// no history: first due somewhere within one round
				j.DueOn = day(1 + rng.Intn(max(every, 2)))
				if err := s.store.CreateJob(ctx, uid, &j); err != nil {
					return err
				}
				continue
			}
			if err := s.store.CreateJob(ctx, uid, &j); err != nil {
				return err
			}
			// history: back from the last time, one round (give or take) apart
			back := min(d.ago, 150)
			if every <= 3 {
				back = min(back, 60)
			}
			if c.Care == "repot" {
				back = last
			}
			var dates []string
			for n, ago := 0, last; ago <= back; n++ {
				dates = append(dates, day(-ago))
				step := every
				if every >= 4 {
					step += rng.Intn(3) - 1
				} else if every > 1 {
					step += rng.Intn(2)
				}
				ago += max(step, 1)
			}
			for n, date := range dates {
				r := store.Record{HomeID: p.HomeID, PlantID: &p.ID, JobID: &j.ID, Date: date, Kind: "plant", Title: careDoneTitle(c.Care, p.Name), Currency: d.home.Currency, Source: "demo"}
				if c.Care == "water" {
					r.Notes = d.notes[n]
				}
				if c.Care == "repot" {
					r.Notes, r.Cost = "One size up, fresh soil.", 1890
				}
				if err := s.store.CreateRecord(ctx, uid, &r); err != nil {
					return err
				}
			}
			j.LastDone, j.DoneCount = dates[0], len(dates)
			j.DueOn = store.AddInterval(j.LastDone, j.EveryN, j.EveryUnit)
			if err := s.store.UpdateJob(ctx, uid, &j); err != nil {
				return err
			}
		}
		if d.gone > 0 {
			p.Archived, p.GoneReason, p.GoneOn = true, "died", day(-d.gone)
			if err := s.store.UpdatePlant(ctx, uid, &p); err != nil {
				return err
			}
		}
	}
	return nil
}
