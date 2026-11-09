package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
)

// handleHomePlans lists the floor plans Pocket Rooms has for a home (tied to
// it there). Without Pocket Rooms the answer says why, and the page greys
// the card instead of hiding it.
func (s *Server) handleHomePlans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	h, err := s.store.GetHome(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "home")
		return
	}
	out := map[string]any{"plans": []any{}, "connected": false}
	sib, err := s.clients(ctx, uid).conn(ctx, appRooms, scopeRoomsRead)
	if err != nil {
		out["detail"] = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	var raw json.RawMessage
	if err := sib.client.Do(ctx, http.MethodGet, "/api/v1/plans", url.Values{"lean": {"1"}}, nil, &raw); err != nil {
		out["detail"] = "Pocket Rooms: " + err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	var list []map[string]any
	_ = json.Unmarshal(raw, &list)
	base := strings.TrimRight(sib.public, "/")
	plans := []map[string]any{}
	for _, p := range list {
		hid, _ := p["home_id"].(float64)
		if int64(hid) != h.ID {
			continue
		}
		id, _ := p["id"].(float64)
		area, _ := p["area_cm2"].(float64)
		rooms, _ := p["rooms_n"].(float64)
		plans = append(plans, map[string]any{"id": int64(id), "name": p["name"], "area_m2": math.Round(area/100) / 100, "rooms": int(rooms),
			"url": fmt.Sprintf("%s/plans/%d", base, int64(id))})
	}
	out["plans"], out["connected"] = plans, true
	if base != "" {
		out["new_url"] = base + "/?new=1"
	}
	writeJSON(w, http.StatusOK, out)
}
