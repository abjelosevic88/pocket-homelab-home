package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-home/internal/ai"
	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// aiFor is the person's AI: Pocket AI through Core, their own
// OpenAI-compatible key, or nil with why.
func (s *Server) aiFor(ctx context.Context, uid int64, st store.Settings) (*ai.Client, string, string) {
	email := ""
	if u, err := s.store.GetUser(ctx, uid); err == nil {
		email = u.Email
	}
	pocket, status := s.core.SelectAI(ctx, email, st.AIProvider, st.OpenAIKey != "")
	if pocket != nil {
		if !status.Ready {
			return nil, "", "Pocket AI: " + status.Detail
		}
		c := ai.New("", "", "", 150*time.Second)
		c.Pocket = pocket.WithModel(st.AIPocketModel)
		return c, "pocket", ""
	}
	if st.OpenAIKey == "" {
		return nil, "", status.Detail
	}
	return ai.New(st.OpenAIBaseURL, st.OpenAIKey, st.OpenAIModel, 150*time.Second), "openai", ""
}

func (s *Server) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, _ := s.store.GetSettings(ctx, actorID(ctx))
	cl, used, why := s.aiFor(ctx, actorID(ctx), st)
	writeJSON(w, http.StatusOK, map[string]any{"ready": cl != nil, "provider": used, "detail": why})
}

func (s *Server) handleAIModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, status := s.core.SelectAI(ctx, userFrom(ctx).Email, "pocket", false)
	if c == nil || !status.Ready {
		writeErr(w, http.StatusServiceUnavailable, status.Detail)
		return
	}
	models, err := c.Models(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, models)
}

func validAIProvider(v string) bool { return core.ValidAIProvider(v) }

func validPocketModel(v string) bool { return core.ValidInferenceModel(v) }

const aiVoice = `You are a practical, plain-spoken home maintenance adviser. You suggest upkeep for one appliance or part of a house: what to do, how often, and a one-line why. Base it on the kind of thing, its make and model if given, and common manufacturer guidance; never invent model-specific facts you are not fairly sure of, say "check the manual" instead. Safety work (gas, electrics, roofs) goes to a qualified person. No emojis, no exclamation marks.`

// handleSuggest asks the AI for upkeep for a thing: a few jobs with their
// interval, nothing saved until the person picks them.
func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	t, err := s.store.GetThing(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "thing")
		return
	}
	st, _ := s.store.GetSettings(ctx, uid)
	cl, used, why := s.aiFor(ctx, uid, st)
	if cl == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": why})
		return
	}
	have, _ := s.store.ListJobs(ctx, uid, store.JobFilter{ThingID: t.ID})
	var b strings.Builder
	fmt.Fprintf(&b, "Thing: %s\nCategory: %s\n", t.Name, t.Category)
	for _, kv := range [][2]string{{"Brand", t.Brand}, {"Model", t.Model}, {"Room", t.Room}, {"Installed", t.InstalledOn}, {"Notes", t.Notes}} {
		if kv[1] != "" {
			fmt.Fprintf(&b, "%s: %s\n", kv[0], kv[1])
		}
	}
	if len(have) > 0 {
		b.WriteString("Upkeep already planned (do not repeat):\n")
		for _, j := range have {
			fmt.Fprintf(&b, "- %s (every %d %s)\n", j.Title, j.EveryN, j.EveryUnit)
		}
	}
	b.WriteString(`
Suggest 2 to 6 pieces of recurring upkeep for it. Answer with JSON only: {"suggestions":[{"title":"short imperative title","every_n":1,"every_unit":"day|week|month|year","soon_days":14,"notes":"one or two plain sentences: how, and why"}]}`)
	var out struct {
		Suggestions []suggestion `json:"suggestions"`
	}
	if err := cl.JSON(ctx, aiVoice, b.String(), &out); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	clean := []suggestion{}
	for _, sgn := range out.Suggestions {
		sgn.Title = strings.TrimSpace(sgn.Title)
		if sgn.Title == "" || len(sgn.Title) > 160 || sgn.EveryN < 1 || sgn.EveryN > 100 || !store.ValidUnit(sgn.EveryUnit) {
			continue
		}
		if sgn.SoonDays < 0 || sgn.SoonDays > 90 {
			sgn.SoonDays = 14
		}
		clean = append(clean, sgn)
		if len(clean) == 6 {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "provider": used, "suggestions": clean})
}
