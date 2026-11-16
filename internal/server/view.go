package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- how bookings and pages are shown to apps and the web app ----

// zone is a person's time zone (their setting, else the box's).
func zone(st store.Settings) *time.Location {
	if st.TZ != "" {
		if loc, err := time.LoadLocation(st.TZ); err == nil {
			return loc
		}
	}
	return time.Local
}

// loadZone reads a zone name, falling back.
func loadZone(name string, fallback *time.Location) *time.Location {
	if name = strings.TrimSpace(name); name != "" {
		if loc, err := time.LoadLocation(name); err == nil {
			return loc
		}
	}
	return fallback
}

func (s *Server) base() string { return strings.TrimRight(s.cfg.BaseURL, "/") }

// publicBase is where people outside open the booking pages.
func (s *Server) publicBase() string {
	if s.cfg.PublicURL != "" {
		return strings.TrimRight(s.cfg.PublicURL, "/")
	}
	return s.base()
}

func (s *Server) personURL(handle string) string {
	if handle == "" {
		return ""
	}
	return s.publicBase() + "/p/" + handle
}

func (s *Server) typeURL(t *store.Type) string { return s.publicBase() + "/b/" + t.Handle }
func (s *Server) typeLink(t *store.Type) string {
	return s.base() + fmt.Sprintf("/types/%d", t.ID)
}
func (s *Server) bookingLink(b *store.Booking) string {
	return s.base() + fmt.Sprintf("/bookings/%d", b.ID)
}
func (s *Server) manageURL(b *store.Booking) string { return s.publicBase() + "/m/" + b.Token }
func (s *Server) inviteURL(iv *store.Invite) string { return s.publicBase() + "/i/" + iv.Token }

// bookingView is a booking as JSON, its times in the owner's zone.
type bookingView struct {
	*store.Booking
	Start     string       `json:"start"`
	End       string       `json:"end"`
	Date      string       `json:"date"`
	Time      string       `json:"time"`
	EndTime   string       `json:"end_time"`
	URL       string       `json:"url"`
	ManageURL string       `json:"manage_url"`
	Past      bool         `json:"past"`
	Made      []store.Made `json:"made,omitempty"`
}

func (s *Server) viewBooking(b *store.Booking, loc *time.Location) *bookingView {
	st, en := b.Start().In(loc), b.End().In(loc)
	return &bookingView{Booking: b, Start: st.Format(time.RFC3339), End: en.Format(time.RFC3339), Date: st.Format("2006-01-02"),
		Time: st.Format("15:04"), EndTime: en.Format("15:04"), URL: s.bookingLink(b), ManageURL: s.manageURL(b), Past: en.Before(time.Now())}
}

func (s *Server) viewBookings(bs []*store.Booking, loc *time.Location) []*bookingView {
	out := make([]*bookingView, 0, len(bs))
	for _, b := range bs {
		out = append(out, s.viewBooking(b, loc))
	}
	return out
}

// typeView is a page as JSON.
type typeView struct {
	*store.Type
	URL     string `json:"url"`
	Link    string `json:"link"`
	Minutes int    `json:"minutes"`
}

func (s *Server) viewType(t *store.Type) *typeView {
	return &typeView{Type: t, URL: s.typeURL(t), Link: s.typeLink(t), Minutes: t.Minutes()}
}

// who is how to call the guest in titles.
func who(b *store.Booking) string {
	return firstNonEmpty(b.Name, b.Email, b.Phone, fmt.Sprintf("booking #%d", b.ID))
}

// whereText is the place of a booking in words (for emails, calendar, lists).
func whereText(b *store.Booking) string {
	switch b.LocationKind {
	case "video", "link":
		return b.MeetingURL
	case "phone":
		return "Phone call to " + firstNonEmpty(b.Phone, "you")
	case "phone_in":
		return "Phone call: " + b.Location
	}
	return b.Location
}

// guestZone is the zone the guest saw the times in (else the owner's).
func guestZone(b *store.Booking, owner *time.Location) *time.Location {
	return loadZone(b.GuestTZ, owner)
}

// money formats cents as "50.00 EUR".
func money(cents int64, cur string) string {
	if cents == 0 {
		return ""
	}
	whole, frac := cents/100, cents%100
	if frac < 0 {
		frac = -frac
	}
	if frac == 0 {
		return strings.TrimSpace(fmt.Sprintf("%d %s", whole, cur))
	}
	return strings.TrimSpace(fmt.Sprintf("%d.%02d %s", whole, frac, cur))
}

// ownerZone loads one person's zone.
func (s *Server) ownerZone(ctx context.Context, uid int64) *time.Location {
	st, _ := s.store.GetSettings(ctx, uid)
	return zone(st)
}
