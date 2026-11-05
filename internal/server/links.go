package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// Pocket Core app links (docs/links.md in pocket-homelab-core), Home as the
// target: another Pocket app the person allowed in Core may act here for
// them with a pcx_ token, only within these scopes. Every route not listed
// in linkRoutes stays closed to it (403).

var linkOffers = []core.Offer{
	{Scope: "home:read", Name: "See your home", Description: "Read your homes, appliances, house plants, upkeep and plant care and when it is due, the log of what was done and what it cost, and meter readings — for your journal, calendar, dashboards and Pocket AI."},
	{Scope: "home:write", Name: "Log upkeep", Description: "Add log entries and meter readings, mark upkeep done and mark plants watered or cared for (a repair booked elsewhere, a smart meter). It may change only what that app logged itself."},
}

type linkRoute struct {
	methods string
	path    *regexp.Regexp
	scope   string // "" = any linked app
}

var linkRoutes = []linkRoute{
	{"GET", regexp.MustCompile(`^/api/v1/auth/me$`), ""},
	{"GET", regexp.MustCompile(`^/api/v1/(overview|due|homes|things|plants|plants/templates|jobs|records|meters|pros|search|spending|day/\d{4}-\d{2}-\d{2})$`), "home:read"},
	{"GET", regexp.MustCompile(`^/api/v1/(homes|things|plants|jobs|records|meters|pros)/\d+$`), "home:read"},
	{"GET", regexp.MustCompile(`^/api/v1/meters/\d+/(readings|usage)$`), "home:read"},
	{"POST", regexp.MustCompile(`^/api/v1/records$`), "home:write"},
	{"PUT DELETE", regexp.MustCompile(`^/api/v1/records/\d+$`), "home:write"},
	{"POST", regexp.MustCompile(`^/api/v1/meters/\d+/readings$`), "home:write"},
	{"POST", regexp.MustCompile(`^/api/v1/jobs/\d+/done$`), "home:write"},
	{"POST", regexp.MustCompile(`^/api/v1/plants/(\d+/)?done$`), "home:write"},
}

var errLinkScope = errors.New("outside the link's scope")

// linkAllows reports whether a link token with these scopes may make this request.
func linkAllows(scopes []string, r *http.Request) bool {
	for _, lr := range linkRoutes {
		if !strings.Contains(" "+lr.methods+" ", " "+r.Method+" ") || !lr.path.MatchString(r.URL.Path) {
			continue
		}
		if lr.scope == "" {
			return true
		}
		for _, sc := range scopes {
			if sc == lr.scope {
				return true
			}
		}
	}
	return false
}

// linkUser validates a pcx_ token, checks the route against its scopes and
// maps it to the person it acts for. The second value is the acting app's slug.
func (s *Server) linkUser(ctx context.Context, tok string, r *http.Request) (*store.User, string, error) {
	in, err := s.core.Introspect(ctx, tok)
	if err != nil {
		return nil, "", err
	}
	if !in.Active || !in.Delegated() || in.User == nil || in.User.Email == "" || in.App == nil || in.App.Slug == "" {
		return nil, "", http.ErrNoCookie
	}
	if !linkAllows(strings.Fields(in.Scope), r) {
		return nil, "", errLinkScope
	}
	u, err := s.coreIdentityUser(ctx, in)
	return u, in.App.Slug, err
}

const linkAppKey ctxKey = 2

// linkApp is the acting app's slug when the request came in on a link token.
func linkApp(ctx context.Context) string {
	a, _ := ctx.Value(linkAppKey).(string)
	return a
}
