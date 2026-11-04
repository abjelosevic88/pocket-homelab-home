package server

import (
	"strings"

	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// suggestion is a piece of upkeep offered for a thing; nothing is saved
// until the person picks it.
type suggestion struct {
	Title     string `json:"title"`
	EveryN    int    `json:"every_n"`
	EveryUnit string `json:"every_unit"`
	SoonDays  int    `json:"soon_days"`
	Notes     string `json:"notes"`
}

func sg(title string, n int, unit string, soon int, notes string) suggestion {
	return suggestion{Title: title, EveryN: n, EveryUnit: unit, SoonDays: soon, Notes: notes}
}

// templatesByKeyword: the first keyword found in a thing's name wins.
var templatesByKeyword = []struct {
	words []string
	list  []suggestion
}{
	{[]string{"boiler", "furnace", "kotao", "kombi", "gas heater"}, []suggestion{
		sg("Annual boiler service", 1, "year", 30, "A qualified engineer checks the burner, flue, safety devices and gas pressure. Keep the certificate."),
		sg("Check the boiler pressure", 1, "month", 3, "Usually 1–1.5 bar when cold. Top up through the filling loop if it drops."),
		sg("Bleed the radiators", 1, "year", 14, "Before the heating season: let the air out until water comes, then check the pressure again."),
	}},
	{[]string{"heat pump", "toplotna"}, []suggestion{
		sg("Heat pump service", 1, "year", 30, "Refrigerant, electrics and the outdoor unit checked by an installer."),
		sg("Clean the outdoor unit", 6, "month", 14, "Clear leaves and dirt from the fins and around the unit."),
		sg("Clean the indoor filters", 3, "month", 7, ""),
	}},
	{[]string{"air condition", "aircon", "a/c", "klima", "split"}, []suggestion{
		sg("Clean the AC filters", 3, "month", 7, "Slide out the filters, vacuum or rinse them, let them dry."),
		sg("AC service and disinfection", 1, "year", 30, "Before summer: coils cleaned, drain checked, refrigerant pressure."),
	}},
	{[]string{"water heater", "bojler", "geyser", "hot water"}, []suggestion{
		sg("Flush the water heater", 1, "year", 14, "Drain a few litres from the bottom valve to clear sediment."),
		sg("Check the anode rod", 2, "year", 30, "Replace it when it is worn down; it stops the tank rusting through."),
		sg("Test the pressure relief valve", 1, "year", 14, ""),
	}},
	{[]string{"chimney", "fireplace", "stove", "wood burner", "dimnjak", "kamin", "peć"}, []suggestion{
		sg("Chimney sweep", 1, "year", 30, "Before the heating season. Keep the sweep's certificate."),
		sg("Check the stove door seal", 1, "year", 14, ""),
	}},
	{[]string{"dishwasher", "perilica posuđa", "mašina za suđe", "masina za sudje"}, []suggestion{
		sg("Clean the dishwasher filter", 1, "month", 3, "Twist out the filter at the bottom, rinse it under the tap."),
		sg("Descale and clean the dishwasher", 3, "month", 7, "Run an empty hot cycle with cleaner; check the salt and rinse aid."),
		sg("Clean the spray arms", 6, "month", 7, ""),
	}},
	{[]string{"washing machine", "washer", "veš mašina", "ves masina", "perilica rublja"}, []suggestion{
		sg("Clean the washing machine filter", 3, "month", 7, "The small door at the bottom front; have a towel and a shallow tray ready."),
		sg("Hot clean cycle", 1, "month", 3, "Empty drum at 90 °C with cleaner, then leave the door open."),
		sg("Clean the detergent drawer and door seal", 1, "month", 3, ""),
	}},
	{[]string{"dryer", "sušilica", "susilica", "tumble"}, []suggestion{
		sg("Clean the lint filter", 1, "week", 1, "After every load ideally; a clogged filter is a fire risk."),
		sg("Clean the condenser or vent duct", 6, "month", 14, ""),
	}},
	{[]string{"fridge", "refrigerator", "frižider", "frizider", "freezer", "zamrzivač"}, []suggestion{
		sg("Clean the fridge coils", 1, "year", 14, "Unplug, pull it out and vacuum the coils at the back or bottom."),
		sg("Check the door seals", 1, "year", 14, "A sheet of paper should be held tight when the door shuts on it."),
		sg("Defrost the freezer", 1, "year", 14, "When the ice is thicker than about 5 mm."),
		sg("Replace the water filter", 6, "month", 14, "Only for fridges with a water or ice dispenser."),
	}},
	{[]string{"hood", "extractor", "napa", "aspirator"}, []suggestion{
		sg("Clean the cooker hood filter", 3, "month", 7, "Metal filters go in the dishwasher; carbon filters are replaced."),
		sg("Replace the carbon filter", 1, "year", 14, ""),
	}},
	{[]string{"oven", "rerna", "pećnica", "cooker"}, []suggestion{
		sg("Clean the oven", 3, "month", 7, "Or run the self-clean cycle."),
	}},
	{[]string{"coffee", "espresso", "kettle", "kafe", "kava"}, []suggestion{
		sg("Descale", 1, "month", 3, "More often with hard water."),
	}},
	{[]string{"smoke", "alarm", "detector", "dimni"}, []suggestion{
		sg("Test the smoke alarms", 1, "month", 3, "Press the test button on each one."),
		sg("Replace the alarm batteries", 1, "year", 14, "Unless they are sealed ten-year alarms."),
		sg("Replace the smoke alarms", 10, "year", 60, "Alarms wear out; the date is printed on the back."),
	}},
	{[]string{"carbon monoxide", "co alarm", "co detector"}, []suggestion{
		sg("Test the CO alarm", 1, "month", 3, ""),
		sg("Replace the CO alarm", 7, "year", 60, ""),
	}},
	{[]string{"extinguisher", "aparat", "fire blanket"}, []suggestion{
		sg("Check the fire extinguisher", 1, "year", 30, "The gauge in the green, the pin and seal in place; serviced as required where you live."),
	}},
	{[]string{"roof", "krov", "tiles", "crijep"}, []suggestion{
		sg("Inspect the roof", 1, "year", 30, "After winter: loose or broken tiles, flashing around the chimney, moss."),
		sg("Clean the gutters", 6, "month", 14, "In late autumn and spring; check the downpipes run free."),
	}},
	{[]string{"gutter", "oluk", "žlijeb"}, []suggestion{
		sg("Clean the gutters", 6, "month", 14, "In late autumn and spring; check the downpipes run free."),
	}},
	{[]string{"window", "prozor", "door", "vrata"}, []suggestion{
		sg("Check the seals and oil the hinges", 1, "year", 14, ""),
		sg("Clean the window drain holes", 1, "year", 14, ""),
	}},
	{[]string{"lawn", "mower", "kosilica", "garden", "vrt"}, []suggestion{
		sg("Service the lawnmower", 1, "year", 30, "Before the season: oil, spark plug, sharpen the blade, air filter."),
		sg("Drain or stabilise the fuel for winter", 1, "year", 14, ""),
	}},
	{[]string{"outdoor tap", "garden tap", "hose", "sprinkler", "irrigation"}, []suggestion{
		sg("Winterise the outdoor taps", 1, "year", 21, "Close the inside valve, open the tap to drain, before the first frost."),
	}},
	{[]string{"septic", "sewer", "septička"}, []suggestion{
		sg("Empty the septic tank", 2, "year", 60, ""),
	}},
	{[]string{"water filter", "softener", "omekšivač", "filter za vodu"}, []suggestion{
		sg("Replace the water filter cartridge", 6, "month", 14, ""),
		sg("Top up the softener salt", 1, "month", 3, ""),
	}},
	{[]string{"solar", "photovolt"}, []suggestion{
		sg("Clean and check the solar panels", 1, "year", 30, "Look for dirt, cracks and loose cables; compare the yield with last year."),
	}},
	{[]string{"fuse", "electrical panel", "breaker", "osigurač", "razvodna"}, []suggestion{
		sg("Test the RCD switches", 6, "month", 14, "Press the T button on each residual-current device; it should trip at once."),
		sg("Electrical safety inspection", 10, "year", 90, "By an electrician; sooner in an older house or after a renovation."),
	}},
	{[]string{"car charger", "wallbox", "ev charger"}, []suggestion{
		sg("Check the charger cable and test its RCD", 1, "year", 14, ""),
	}},
	{[]string{"pool", "bazen", "hot tub", "jacuzzi"}, []suggestion{
		sg("Test the water", 1, "week", 1, "pH and chlorine."),
		sg("Clean the filter", 1, "month", 3, ""),
	}},
}

// templatesByCategory are offered when no keyword matches.
var templatesByCategory = map[string][]suggestion{
	"heating":    {sg("Service the heating", 1, "year", 30, ""), sg("Bleed the radiators", 1, "year", 14, "")},
	"cooling":    {sg("Clean the filters", 3, "month", 7, ""), sg("Service", 1, "year", 30, "")},
	"water":      {sg("Check for leaks", 6, "month", 14, "Under sinks, around the toilet, the water meter with all taps off."), sg("Descale taps and shower heads", 6, "month", 7, "")},
	"electrical": {sg("Test the RCD switches", 6, "month", 14, "")},
	"appliance":  {sg("Clean the filter", 3, "month", 7, ""), sg("Check and clean", 1, "year", 14, "")},
	"kitchen":    {sg("Clean and descale", 3, "month", 7, "")},
	"structure":  {sg("Inspect for damp and cracks", 1, "year", 30, ""), sg("Clean the gutters", 6, "month", 14, "")},
	"outdoor":    {sg("Seasonal check", 6, "month", 14, "")},
	"safety":     {sg("Test it", 1, "month", 3, ""), sg("Replace the batteries", 1, "year", 14, "")},
	"other":      {sg("Check it", 1, "year", 14, "")},
}

// templatesFor offers upkeep for a thing by its name, else its category.
func templatesFor(t *store.Thing) []suggestion {
	name := strings.ToLower(t.Name + " " + t.Model)
	for _, k := range templatesByKeyword {
		for _, w := range k.words {
			if strings.Contains(name, w) {
				return k.list
			}
		}
	}
	if l, ok := templatesByCategory[t.Category]; ok {
		return l
	}
	return templatesByCategory["other"]
}
