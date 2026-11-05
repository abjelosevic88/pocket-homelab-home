package server

import (
	"math"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// The kinds of plants, the light they stand in and the care they get.
var (
	plantKinds  = []string{"foliage", "flowering", "succulent", "cactus", "herb", "fern", "orchid", "palm", "other"}
	plantLights = []string{"low", "medium", "bright", "direct"}
	goneReasons = []string{"died", "given", "other"}
)

// careNames: what the care is called on the list, and what the log says after.
var careNames = map[string][2]string{
	"water":     {"Water", "Watered"},
	"fertilise": {"Fertilise", "Fertilised"},
	"mist":      {"Mist", "Misted"},
	"rotate":    {"Rotate", "Rotated"},
	"prune":     {"Prune", "Pruned"},
	"pests":     {"Check for pests", "Checked for pests"},
	"repot":     {"Repot", "Repotted"},
}

func validCare(c string) bool { _, ok := careNames[c]; return ok }

// careTitle is the care's name on the due list ("Water").
func careTitle(c string) string {
	if n, ok := careNames[c]; ok {
		return n[0]
	}
	return c
}

// careDoneTitle is what the log says: "Watered Monstera", "Checked Monstera for pests".
func careDoneTitle(c, plant string) string {
	switch c {
	case "pests":
		return "Checked " + plant + " for pests"
	case "":
		return "Looked after " + plant
	}
	if n, ok := careNames[c]; ok {
		return n[1] + " " + plant
	}
	return c + " " + plant
}

// careSuggestion is one line of a plant's care schedule.
type careSuggestion struct {
	Care      string `json:"care"`
	Title     string `json:"title"`
	EveryN    int    `json:"every_n"`
	EveryUnit string `json:"every_unit"`
	SoonDays  int    `json:"soon_days"`
	Notes     string `json:"notes"`
}

func cs(care string, n int, unit string, soon int, notes string) careSuggestion {
	return careSuggestion{Care: care, Title: careTitle(care), EveryN: n, EveryUnit: unit, SoonDays: soon, Notes: notes}
}

// careByKind is the usual care for a kind of plant in medium light. Watering
// is "on the day" (no warning before); the rare things get a little notice.
var careByKind = map[string][]careSuggestion{
	"foliage": {cs("water", 7, "day", 0, "When the top few centimetres of soil are dry. Empty the saucer after."), cs("fertilise", 1, "month", 2, "Half-strength liquid feed in spring and summer; skip it in winter."),
		cs("rotate", 2, "week", 0, "A quarter turn so it grows evenly toward the light."), cs("repot", 2, "year", 14, "One pot size up, in spring, when roots circle the pot or come out of the bottom.")},
	"flowering": {cs("water", 5, "day", 0, "Keep the soil lightly moist while it flowers."), cs("fertilise", 2, "week", 2, "A feed for flowering plants in the growing season."),
		cs("prune", 2, "week", 2, "Pick off spent flowers and yellow leaves."), cs("repot", 2, "year", 14, "")},
	"succulent": {cs("water", 14, "day", 0, "Soak, then let the soil dry out completely. Less in winter."), cs("fertilise", 2, "month", 3, "Cactus feed in spring and summer only."),
		cs("rotate", 1, "month", 0, ""), cs("repot", 2, "year", 14, "Gritty cactus soil and a pot with a hole.")},
	"cactus": {cs("water", 21, "day", 0, "Soak, then let it dry out completely. Hardly any water in winter."), cs("fertilise", 2, "month", 3, "Cactus feed in spring and summer only."),
		cs("repot", 3, "year", 14, "Gloves or folded newspaper for the spines.")},
	"herb": {cs("water", 3, "day", 0, "Herbs on a sill dry out fast; water when the top of the soil is dry."), cs("prune", 2, "week", 2, "Pinch off the tops above a pair of leaves so it bushes out; take off flower buds."),
		cs("fertilise", 1, "month", 2, "")},
	"fern": {cs("water", 4, "day", 0, "Keep the soil moist, never soggy."), cs("mist", 3, "day", 0, "Ferns like humid air: mist the fronds, or stand it on a tray of wet pebbles."),
		cs("fertilise", 1, "month", 2, "Weak feed in spring and summer."), cs("repot", 2, "year", 14, "")},
	"orchid": {cs("water", 10, "day", 0, "Soak the pot for ten minutes when the roots turn silvery, then let it drain."), cs("fertilise", 1, "month", 2, "Orchid feed, weak."),
		cs("mist", 1, "week", 0, "Mist the aerial roots, not the flowers."), cs("repot", 2, "year", 14, "In fresh bark after it has flowered.")},
	"palm": {cs("water", 10, "day", 0, "When the top half of the soil is dry."), cs("fertilise", 1, "month", 2, "Spring and summer."),
		cs("pests", 1, "month", 3, "Look under the fronds for spider mites and scale."), cs("repot", 3, "year", 14, "")},
	"other": {cs("water", 7, "day", 0, ""), cs("fertilise", 1, "month", 2, ""), cs("repot", 2, "year", 14, "")},
}

// lightWater stretches or shortens watering by the light a plant gets: less
// light, less water.
var lightWater = map[string]float64{"low": 1.5, "medium": 1, "bright": 1, "direct": 0.75}

// careFor is the suggested care for a plant: its kind's, with watering
// matched to its light, plus a monthly pest check for the leafy ones.
func careFor(kind, light string) []careSuggestion {
	base, ok := careByKind[kind]
	if !ok {
		base = careByKind["other"]
	}
	f, ok := lightWater[light]
	if !ok {
		f = 1
	}
	out := make([]careSuggestion, 0, len(base)+1)
	for _, c := range base {
		if c.Care == "water" && c.EveryUnit == "day" {
			c.EveryN = int(math.Max(1, math.Round(float64(c.EveryN)*f)))
		}
		out = append(out, c)
	}
	if kind == "foliage" || kind == "flowering" || kind == "fern" {
		out = append(out, cs("pests", 1, "month", 3, "Look under the leaves and where they join the stem: webs, sticky spots, tiny bugs."))
	}
	return out
}

// plantKindWords guess a kind from a name or species; the first match wins.
var plantKindWords = []struct {
	kind  string
	words []string
}{
	{"flowering", []string{"christmas cactus", "easter cactus", "schlumbergera"}},
	{"cactus", []string{"cactus", "cacti", "kaktus", "opuntia", "mammillaria", "echinocactus"}},
	{"succulent", []string{"snake plant", "sansevieria", "dracaena trifasciata", "aloe", "echeveria", "jade", "crassula", "haworthia", "zz plant", "zamioculcas", "string of pearls", "sedum", "kalanchoe", "agave", "sukulent"}},
	{"herb", []string{"basil", "bosiljak", "mint", "menta", "parsley", "peršun", "persun", "rosemary", "ruzmarin", "ružmarin", "thyme", "majčina dušica", "chives", "coriander", "cilantro", "oregano", "sage", "kadulja", "žalfija", "dill", "lavender", "lavanda", "lemon balm", "matičnjak"}},
	{"orchid", []string{"orchid", "orhideja", "phalaenopsis", "dendrobium", "cymbidium"}},
	{"fern", []string{"fern", "paprat", "nephrolepis", "maidenhair", "adiantum", "asplenium"}},
	{"palm", []string{"palm", "palma", "kentia", "areca", "dypsis", "chamaedorea", "howea"}},
	{"flowering", []string{"peace lily", "spathiphyllum", "anthurium", "begonia", "african violet", "saintpaulia", "cyclamen", "ciklama", "geranium", "pelargonium", "hibiscus", "christmas cactus", "schlumbergera", "bromeliad", "gardenia", "jasmine"}},
	{"foliage", []string{"monstera", "pothos", "epipremnum", "philodendron", "ficus", "fiddle", "rubber plant", "calathea", "maranta", "prayer plant", "dracaena", "spider plant", "chlorophytum", "dieffenbachia", "aglaonema", "pilea", "peperomia", "tradescantia", "hoya", "alocasia", "croton", "ivy", "bršljan", "schefflera", "yucca", "fikus"}},
}

// guessPlantKind is the kind a plant's name or species points to, else "".
func guessPlantKind(name, species string) string {
	s := strings.ToLower(name + " " + species)
	for _, k := range plantKindWords {
		for _, w := range k.words {
			if strings.Contains(s, w) {
				return k.kind
			}
		}
	}
	return ""
}

// careJob turns a suggestion into a job for a plant, first due one interval
// after from (the day it was last done; today when unknown).
func careJob(p *store.Plant, c careSuggestion, from string) store.Job {
	if from == "" {
		from = store.Today()
	}
	return store.Job{HomeID: p.HomeID, PlantID: &p.ID, Care: c.Care, Title: careTitle(c.Care), Notes: c.Notes, EveryN: c.EveryN, EveryUnit: c.EveryUnit,
		SoonDays: c.SoonDays, DueOn: store.AddInterval(from, c.EveryN, c.EveryUnit)}
}
