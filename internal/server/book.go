package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/avail"
	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- making, moving and calling off bookings ----
//
// Every booking — from a public page, a personal link, the owner's own
// "Book someone in", or another app through Core — goes through book():
// the time is checked and stored in one step (no two people get the same
// time), then the owner is told, the guest gets their email, the event and
// everything set to happen by itself is made in the other apps, and an
// event goes on the Core bus.

// bookReq is one request for a time.
type bookReq struct {
	Type      *store.Type
	Start     time.Time
	Minutes   int
	Name      string
	Email     string
	Phone     string
	Answers   []store.Answer
	GuestTZ   string
	Language  string
	Source    string // page | invite | owner | import | <app slug>
	InviteID  int64
	IPHash    string
	ClientKey string
	Force     bool // the owner may book over a taken time or outside the hours
	TellGuest bool
	Note      string // the owner's own note
	Status    string // "" = by the page's rule
}

// jitsiRoom is a fresh video room for one booking.
func jitsiRoom(title string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		}
		return -1
	}, title)
	if len(slug) > 24 {
		slug = slug[:24]
	}
	return "https://meet.jit.si/Pocket" + slug + hex.EncodeToString(b[:])
}

// book checks the time and stores the booking, then does what follows.
func (s *Server) book(ctx context.Context, rq bookReq) (*store.Booking, error) {
	t := rq.Type
	if rq.Minutes <= 0 {
		rq.Minutes = t.Minutes()
	}
	if !rq.Force && !t.Lasts(rq.Minutes) {
		return nil, &userErr{"this page does not offer that length"}
	}
	if rq.ClientKey != "" {
		if b, err := s.store.BookingByClientKey(ctx, t.UserID, rq.ClientKey); err == nil {
			return b, nil
		}
	}
	st, _ := s.store.GetSettings(ctx, t.UserID)
	status := store.StatusConfirmed
	if t.Confirm == "manual" {
		status = store.StatusPending
	}
	if rq.Status != "" {
		status = rq.Status
	}
	b := &store.Booking{TypeID: t.ID, Title: t.Title, Emoji: t.Emoji, Status: status, StartAt: store.Stamp(rq.Start),
		EndAt: store.Stamp(rq.Start.Add(time.Duration(rq.Minutes) * time.Minute)), Minutes: rq.Minutes, GuestTZ: rq.GuestTZ,
		Name: clip(strings.TrimSpace(rq.Name), 200), Email: strings.ToLower(strings.TrimSpace(rq.Email)), Phone: clip(strings.TrimSpace(rq.Phone), 40),
		Answers: rq.Answers, LocationKind: t.LocationKind, Price: t.Price, Currency: t.Currency, Source: rq.Source, InviteID: rq.InviteID,
		Language: firstNonEmpty(rq.Language, t.Language, "en"), IPHash: rq.IPHash, ClientKey: rq.ClientKey, OwnerNote: rq.Note}
	switch t.LocationKind {
	case "video":
		b.MeetingURL = jitsiRoom(t.Title)
	case "link":
		b.MeetingURL = t.Location
	case "phone_in", "place":
		b.Location = t.Location
	case "ask":
		for _, a := range rq.Answers {
			if a.ID == "where" {
				b.Location = a.Value
			}
		}
	}
	var cal []avail.Interval
	if !rq.Force && st.CheckCalendar {
		var err error
		cal, err = s.calendarBusy(ctx, t.UserID, st, rq.Start.Add(-24*time.Hour), rq.Start.Add(48*time.Hour), false)
		if err != nil {
			s.log.Warn("booking: could not check Pocket Calendar", "err", err)
		}
	}
	in, err := s.baseInput(ctx, t, st, rq.Minutes, time.Now())
	if err != nil {
		return nil, err
	}
	s.bookMu.Lock()
	err = s.store.CreateBooking(ctx, t.UserID, b, func(tx *sql.Tx) error {
		if rq.Force {
			return nil
		}
		return stillFree(ctx, tx, t, in, rq.Start, 0, cal)
	})
	s.bookMu.Unlock()
	if errors.Is(err, store.ErrConflict) && rq.ClientKey != "" {
		return s.store.BookingByClientKey(ctx, t.UserID, rq.ClientKey)
	}
	if err != nil {
		return nil, err
	}
	s.afterBooked(ctx, t, b, st, rq.TellGuest)
	return b, nil
}

// afterBooked runs the follow-ups of a new booking. They run in the
// background: the person who booked does not wait for the other apps.
func (s *Server) afterBooked(ctx context.Context, t *store.Type, b *store.Booking, st store.Settings, tellGuest bool) {
	s.publish(ctx, b.UserID, "booking.booked", b.Title, s.eventPayload(ctx, b, nil))
	bg := context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(bg, 3*time.Minute)
		defer cancel()
		if st.NotifyEach {
			s.tellOwner(ctx, b, "new")
		}
		if tellGuest && st.TellGuest && b.Email != "" {
			kind := "booked"
			if b.Status == store.StatusPending {
				kind = "asked"
			}
			if err := s.mailGuest(ctx, b, kind, ""); err != nil && !isNotConnected(err) {
				s.log.Warn("booking: guest email failed", "booking", b.ID, "err", err)
			}
		}
		if b.Status == store.StatusConfirmed {
			s.autoMake(ctx, t, b)
		}
	}()
}

// autoMake makes what the page says should happen by itself.
func (s *Server) autoMake(ctx context.Context, t *store.Type, b *store.Booking) {
	if t == nil {
		return
	}
	c := s.clients(ctx, b.UserID)
	run := func(on bool, target string) {
		if !on {
			return
		}
		if _, err := s.make(ctx, c, t, b, makeIn{Target: target}, true); err != nil {
			s.log.Debug("booking: auto make", "target", target, "booking", b.ID, "err", err)
		}
	}
	run(t.After.Event, "event")
	run(t.After.Contact, "contact")
	run(t.After.Deal, "deal")
	run(t.After.Task, "task")
	run(t.After.Note, "note")
	run(t.After.Invoice && b.Price > 0, "invoice")
}

// eventPayload is a booking as an event for the Core bus.
func (s *Server) eventPayload(ctx context.Context, b *store.Booking, extra map[string]any) map[string]any {
	loc := s.ownerZone(ctx, b.UserID)
	v := s.viewBooking(b, loc)
	p := map[string]any{"booking_id": b.ID, "type_id": b.TypeID, "type": b.Title, "title": b.Title + " — " + who(b), "start": v.Start, "end": v.End,
		"minutes": b.Minutes, "status": b.Status, "name": b.Name, "email": b.Email, "phone": b.Phone, "location": whereText(b),
		"meeting_url": b.MeetingURL, "url": v.URL, "source": b.Source}
	for k, x := range extra {
		p[k] = x
	}
	return p
}

// ---- changes ----

// confirm says yes to a booking that waited.
func (s *Server) confirm(ctx context.Context, b *store.Booking, tellGuest bool) error {
	if b.Status != store.StatusPending {
		return &userErr{"only a booking that waits for you can be confirmed"}
	}
	b.Status = store.StatusConfirmed
	if err := s.store.UpdateBooking(ctx, b); err != nil {
		return err
	}
	s.publish(ctx, b.UserID, "booking.confirmed", b.Title, s.eventPayload(ctx, b, nil))
	st, _ := s.store.GetSettings(ctx, b.UserID)
	bg := context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(bg, 3*time.Minute)
		defer cancel()
		if tellGuest && st.TellGuest && b.Email != "" {
			_ = s.mailGuest(ctx, b, "confirmed", "")
		}
		if t, err := s.store.GetType(ctx, b.UserID, b.TypeID); err == nil {
			s.autoMake(ctx, t, b)
		}
	}()
	return nil
}

// decline says no to a booking that waited.
func (s *Server) decline(ctx context.Context, b *store.Booking, message string, tellGuest bool) error {
	if b.Status != store.StatusPending {
		return &userErr{"only a booking that waits for you can be declined"}
	}
	b.Status, b.CancelReason, b.CancelledBy = store.StatusDeclined, clip(strings.TrimSpace(message), 2000), "owner"
	if err := s.store.UpdateBooking(ctx, b); err != nil {
		return err
	}
	s.publish(ctx, b.UserID, "booking.declined", b.Title, s.eventPayload(ctx, b, map[string]any{"reason": b.CancelReason}))
	st, _ := s.store.GetSettings(ctx, b.UserID)
	if tellGuest && st.TellGuest && b.Email != "" {
		s.background(ctx, func(ctx context.Context) { _ = s.mailGuest(ctx, b, "declined", message) })
	}
	return nil
}

// cancel calls a booking off (by the guest or the owner) and removes its
// calendar event.
func (s *Server) cancel(ctx context.Context, b *store.Booking, by, message string, tellGuest bool) error {
	if b.Status != store.StatusPending && b.Status != store.StatusConfirmed {
		return &userErr{"this booking is not on any more"}
	}
	if by == "guest" && b.Start().Before(time.Now()) {
		return &userErr{"this booking has already started"}
	}
	b.Status, b.CancelReason, b.CancelledBy = store.StatusCancelled, clip(strings.TrimSpace(message), 2000), by
	if err := s.store.UpdateBooking(ctx, b); err != nil {
		return err
	}
	s.publish(ctx, b.UserID, "booking.cancelled", b.Title, s.eventPayload(ctx, b, map[string]any{"by": by, "reason": b.CancelReason}))
	st, _ := s.store.GetSettings(ctx, b.UserID)
	s.background(ctx, func(ctx context.Context) {
		s.syncEvent(ctx, b, true)
		if by == "guest" && st.NotifyEach {
			s.tellOwner(ctx, b, "cancelled")
		}
		if tellGuest && st.TellGuest && b.Email != "" {
			_ = s.mailGuest(ctx, b, "cancelled", message)
		}
	})
	return nil
}

// move puts a booking at another time (checked like a new one, without
// counting the booking itself).
func (s *Server) move(ctx context.Context, b *store.Booking, start time.Time, minutes int, by string, force, tellGuest bool) error {
	if b.Status != store.StatusPending && b.Status != store.StatusConfirmed {
		return &userErr{"this booking is not on any more"}
	}
	if by == "guest" && b.Start().Before(time.Now()) {
		return &userErr{"this booking has already started"}
	}
	t, err := s.store.GetType(ctx, b.UserID, b.TypeID)
	if err != nil {
		if !force {
			return &userErr{"the booking page of this booking is gone; move it in your calendar instead"}
		}
		t = &store.Type{ID: 0, UserID: b.UserID, Durations: []int{b.Minutes}, Capacity: 1}
	}
	if minutes <= 0 {
		minutes = b.Minutes
	}
	if !force && !t.Lasts(minutes) {
		return &userErr{"this page does not offer that length"}
	}
	st, _ := s.store.GetSettings(ctx, b.UserID)
	var cal []avail.Interval
	if !force && st.CheckCalendar {
		cal, _ = s.calendarBusy(ctx, b.UserID, st, start.Add(-24*time.Hour), start.Add(48*time.Hour), false)
	}
	old := b.StartAt
	b.MovedFrom, b.StartAt, b.EndAt, b.Minutes = old, store.Stamp(start), store.Stamp(start.Add(time.Duration(minutes)*time.Minute)), minutes
	if by == "guest" && t.Confirm == "manual" {
		b.Status = store.StatusPending
	}
	in, err := s.baseInput(ctx, t, st, minutes, time.Now())
	if err != nil && !force {
		return err
	}
	s.bookMu.Lock()
	err = s.store.MoveBooking(ctx, b, func(tx *sql.Tx) error {
		if force {
			return nil
		}
		return stillFree(ctx, tx, t, in, start, b.ID, cal)
	})
	s.bookMu.Unlock()
	if err != nil {
		return err
	}
	loc := zone(st)
	s.publish(ctx, b.UserID, "booking.moved", b.Title, s.eventPayload(ctx, b, map[string]any{"from": store.ParseStamp(old).In(loc).Format(time.RFC3339), "by": by}))
	s.background(ctx, func(ctx context.Context) {
		s.syncEvent(ctx, b, false)
		if by == "guest" && st.NotifyEach {
			s.tellOwner(ctx, b, "moved")
		}
		if tellGuest && st.TellGuest && b.Email != "" {
			_ = s.mailGuest(ctx, b, "moved", "")
		}
	})
	return nil
}

// background runs follow-up work after the answer has gone.
func (s *Server) background(ctx context.Context, fn func(ctx context.Context)) {
	bg := context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(bg, 2*time.Minute)
		defer cancel()
		fn(ctx)
	}()
}

// finishPast marks confirmed bookings whose time has passed as done.
func (s *Server) finishPast(ctx context.Context) {
	rows, err := s.store.DB.QueryContext(ctx, `SELECT id, user_id FROM bookings WHERE status = 'confirmed' AND end_at < ? LIMIT 500`,
		store.Stamp(time.Now().Add(-2*time.Hour)))
	if err != nil {
		return
	}
	type pair struct{ id, uid int64 }
	var ps []pair
	for rows.Next() {
		var p pair
		if rows.Scan(&p.id, &p.uid) == nil {
			ps = append(ps, p)
		}
	}
	rows.Close()
	for _, p := range ps {
		b, err := s.store.GetBooking(ctx, p.uid, p.id)
		if err != nil {
			continue
		}
		b.Status = store.StatusDone
		if s.store.UpdateBooking(ctx, b) == nil {
			s.publish(ctx, b.UserID, "booking.done", b.Title, s.eventPayload(ctx, b, nil))
		}
	}
}

// guestTitle names the booking for the guest: "Intro call with Aleksandar".
func (s *Server) ownerName(ctx context.Context, uid int64) string {
	st, _ := s.store.GetSettings(ctx, uid)
	if st.PageName != "" {
		return st.PageName
	}
	if u, err := s.store.GetUser(ctx, uid); err == nil {
		return firstNonEmpty(u.Name, strings.Split(u.Email, "@")[0])
	}
	return ""
}

// fmtWhen is a booking's time in words, in a zone: "Wed 18 Nov 2026, 10:00–10:30".
func fmtWhen(b *store.Booking, loc *time.Location, lang string) string {
	st, en := b.Start().In(loc), b.End().In(loc)
	return fmt.Sprintf("%s, %s–%s", dayName(st, lang), st.Format("15:04"), en.Format("15:04"))
}
