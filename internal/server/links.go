package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
)

// Pocket Core app links (docs/links.md in pocket-homelab-core), Booking as
// the target: another Pocket app the person allowed in Core may act here for
// them with a pcx_ token, only within these scopes. Every route not listed
// in linkRoutes stays closed to it (403).

var linkOffers = []core.Offer{
	{Scope: "booking:read", Name: "See your bookings and booking pages", Description: "Read who booked a time with you and when, your booking pages and their free times — for your day, search, a contact's or a deal's page, dashboards and Pocket AI."},
	{Scope: "booking:write", Name: "Book times and send booking links for you", Description: "Book someone into a free time, confirm, move or cancel a booking, make a personal booking link for someone (\"send Ana a link to book a call\"), and start a booking page as a draft. It may change only the pages that app made itself."},
}

type linkRoute struct {
	methods string
	path    *regexp.Regexp
	scope   string // "" = any linked app
}

var linkRoutes = []linkRoute{
	{"GET", regexp.MustCompile(`^/api/v1/auth/me$`), ""},
	{"GET", regexp.MustCompile(`^/api/v1/(summary|bookings|types|search|lookup|invites|day/\d{4}-\d{2}-\d{2})$`), "booking:read"},
	{"GET", regexp.MustCompile(`^/api/v1/(bookings|types)/\d+$`), "booking:read"},
	{"GET", regexp.MustCompile(`^/api/v1/types/\d+/slots$`), "booking:read"},
	{"POST", regexp.MustCompile(`^/api/v1/(bookings|invites|types)$`), "booking:write"},
	{"POST", regexp.MustCompile(`^/api/v1/bookings/\d+/(confirm|cancel|move)$`), "booking:write"},
	{"PUT", regexp.MustCompile(`^/api/v1/types/\d+$`), "booking:write"},
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
	if !in.Active || !in.Delegated() || in.Audience != "booking" || in.User == nil || in.User.Email == "" || in.App == nil || in.App.Slug == "" {
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
