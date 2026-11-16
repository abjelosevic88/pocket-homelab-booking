package server

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- bookings ----

func (s *Server) ownBooking(w http.ResponseWriter, r *http.Request) (*store.Booking, bool) {
	ctx := r.Context()
	b, err := s.store.GetBooking(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "booking")
		return nil, false
	}
	return b, true
}

func (s *Server) loc(ctx context.Context) *time.Location { return s.ownerZone(ctx, actorID(ctx)) }

// dayBounds turns days in a zone into UTC stamps (to is included).
func dayBounds(from, to string, loc *time.Location) (string, string, error) {
	var a, b string
	if from != "" {
		t, err := time.ParseInLocation("2006-01-02", from, loc)
		if err != nil {
			return "", "", &userErr{"from is YYYY-MM-DD"}
		}
		a = store.Stamp(t)
	}
	if to != "" {
		t, err := time.ParseInLocation("2006-01-02", to, loc)
		if err != nil {
			return "", "", &userErr{"to is YYYY-MM-DD"}
		}
		b = store.Stamp(t.AddDate(0, 0, 1))
	}
	return a, b, nil
}

func (s *Server) handleListBookings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	loc := s.loc(ctx)
	q := r.URL.Query()
	view := firstNonEmpty(q.Get("view"), "all")
	if !oneOf(view, []string{"upcoming", "pending", "past", "cancelled", "all", "active"}) {
		writeErr(w, http.StatusBadRequest, "view is upcoming, pending, past, cancelled or all")
		return
	}
	from, to, err := dayBounds(q.Get("from"), q.Get("to"), loc)
	if err != nil {
		s.badOrFail(w, err, "bookings")
		return
	}
	f := store.BookingFilter{View: view, From: from, To: to, Query: q.Get("q"), Email: q.Get("email"), TypeID: int64(queryInt(r, "type_id")),
		Limit: queryInt(r, "limit"), Offset: queryInt(r, "offset"), Newest: queryBool(r, "newest")}
	if view == "all" {
		f.View = ""
	}
	bs, total, err := s.store.ListBookings(ctx, actorID(ctx), f)
	if err != nil {
		s.fail(w, err, "bookings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bookings": s.viewBookings(bs, loc), "total": total})
}

func (s *Server) handleGetBooking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	b, ok := s.ownBooking(w, r)
	if !ok {
		return
	}
	v := s.viewBooking(b, s.loc(ctx))
	v.Made, _ = s.store.MadeFor(ctx, b.UserID, b.ID)
	if v.Made == nil {
		v.Made = []store.Made{}
	}
	writeJSON(w, http.StatusOK, v)
}

type createIn struct {
	TypeID    int64          `json:"type_id"`
	Start     string         `json:"start"`
	Minutes   int            `json:"minutes"`
	Name      string         `json:"name"`
	Email     string         `json:"email"`
	Phone     string         `json:"phone"`
	Note      string         `json:"note"`
	Answers   []store.Answer `json:"answers"`
	TellGuest *bool          `json:"tell_guest"`
	Force     bool           `json:"force"`
	ClientKey string         `json:"client_key"`
	Status    string         `json:"status"`
}

func parseStart(v string, loc *time.Location) (time.Time, error) {
	v = strings.TrimSpace(v)
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", v, loc); err == nil {
		return t, nil
	}
	return time.Time{}, &userErr{"the start is a date and time (RFC 3339, like 2026-11-18T10:00:00+01:00)"}
}

// handleCreateBooking books someone in (the owner, or an app through Core).
func (s *Server) handleCreateBooking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in createIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	t, err := s.store.GetType(ctx, uid, in.TypeID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "pick one of your booking pages")
		return
	}
	loc := s.loc(ctx)
	start, err := parseStart(in.Start, loc)
	if err != nil {
		s.badOrFail(w, err, "booking")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email != "" {
		if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
			writeErr(w, http.StatusBadRequest, "that email address does not look right")
			return
		}
	}
	if strings.TrimSpace(in.Name) == "" && email == "" {
		writeErr(w, http.StatusBadRequest, "say who it is for: a name or an email")
		return
	}
	src := "owner"
	if app := linkApp(ctx); app != "" {
		src = app
		in.Force = false // an app books free times only
	}
	if in.Status != "" && !oneOf(in.Status, []string{store.StatusPending, store.StatusConfirmed}) {
		writeErr(w, http.StatusBadRequest, "a new booking is pending or confirmed")
		return
	}
	tell := in.TellGuest == nil || *in.TellGuest
	answers := []store.Answer{}
	for _, a := range in.Answers {
		if v := clip(strings.TrimSpace(a.Value), 5000); v != "" {
			answers = append(answers, store.Answer{ID: clip(a.ID, 20), Label: clip(firstNonEmpty(a.Label, a.ID), 300), Value: v})
		}
	}
	status := firstNonEmpty(in.Status, store.StatusConfirmed) // the owner (or an app for them) said so: no need to confirm
	b, err := s.book(ctx, bookReq{Type: t, Start: start, Minutes: in.Minutes, Name: in.Name, Email: email, Phone: in.Phone, Answers: answers,
		GuestTZ: loc.String(), Language: t.Language, Source: src, Force: in.Force, TellGuest: tell, Note: clip(strings.TrimSpace(in.Note), 5000),
		ClientKey: clip(strings.TrimSpace(in.ClientKey), 120), Status: status})
	if err != nil {
		s.badOrFail(w, err, "booking")
		return
	}
	writeJSON(w, http.StatusCreated, s.viewBooking(b, loc))
}

// handleUpdateBooking changes the owner's own note, the guest's details or the price.
func (s *Server) handleUpdateBooking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	b, ok := s.ownBooking(w, r)
	if !ok {
		return
	}
	var in struct {
		OwnerNote *string `json:"owner_note"`
		Name      *string `json:"name"`
		Email     *string `json:"email"`
		Phone     *string `json:"phone"`
		Price     *int64  `json:"price"`
		Location  *string `json:"location"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.OwnerNote != nil {
		b.OwnerNote = clip(strings.TrimSpace(*in.OwnerNote), 20000)
	}
	if in.Name != nil {
		b.Name = clip(strings.TrimSpace(*in.Name), 200)
	}
	if in.Email != nil {
		e := strings.ToLower(strings.TrimSpace(*in.Email))
		if e != "" {
			if a, err := mail.ParseAddress(e); err != nil || a.Address != e {
				writeErr(w, http.StatusBadRequest, "that email address does not look right")
				return
			}
		}
		b.Email = e
	}
	if in.Phone != nil {
		b.Phone = clip(strings.TrimSpace(*in.Phone), 40)
	}
	if in.Price != nil && *in.Price >= 0 {
		b.Price = *in.Price
	}
	if in.Location != nil {
		b.Location = clip(strings.TrimSpace(*in.Location), 500)
	}
	if err := s.store.UpdateBooking(ctx, b); err != nil {
		s.fail(w, err, "save booking")
		return
	}
	writeJSON(w, http.StatusOK, s.viewBooking(b, s.loc(ctx)))
}

func (s *Server) handleDeleteBooking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	b, ok := s.ownBooking(w, r)
	if !ok {
		return
	}
	if b.Status == store.StatusPending || b.Status == store.StatusConfirmed {
		if b.End().After(time.Now()) {
			writeErr(w, http.StatusBadRequest, "cancel the booking first; then it can be deleted")
			return
		}
	}
	if err := s.store.DeleteBooking(ctx, b.UserID, b.ID); err != nil {
		s.fail(w, err, "delete booking")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type actionIn struct {
	Message   string `json:"message"`
	TellGuest *bool  `json:"tell_guest"`
	Start     string `json:"start"`
	Minutes   int    `json:"minutes"`
	Force     bool   `json:"force"`
	Paid      *bool  `json:"paid"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

func (in actionIn) tell() bool { return in.TellGuest == nil || *in.TellGuest }

// action reads the body, runs fn and answers with the booking.
func (s *Server) action(w http.ResponseWriter, r *http.Request, fn func(ctx context.Context, b *store.Booking, in actionIn) error) {
	ctx := r.Context()
	b, ok := s.ownBooking(w, r)
	if !ok {
		return
	}
	var in actionIn
	if r.ContentLength != 0 {
		if err := decode(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
	}
	if err := fn(ctx, b, in); err != nil {
		s.badOrFail(w, err, "booking")
		return
	}
	b, _ = s.store.GetBooking(ctx, b.UserID, b.ID)
	writeJSON(w, http.StatusOK, s.viewBooking(b, s.loc(ctx)))
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, func(ctx context.Context, b *store.Booking, in actionIn) error { return s.confirm(ctx, b, in.tell()) })
}

func (s *Server) handleDecline(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, func(ctx context.Context, b *store.Booking, in actionIn) error {
		return s.decline(ctx, b, in.Message, in.tell())
	})
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, func(ctx context.Context, b *store.Booking, in actionIn) error {
		return s.cancel(ctx, b, "owner", in.Message, in.tell())
	})
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, func(ctx context.Context, b *store.Booking, in actionIn) error {
		start, err := parseStart(in.Start, s.loc(ctx))
		if err != nil {
			return err
		}
		force := in.Force && linkApp(ctx) == ""
		return s.move(ctx, b, start, in.Minutes, "owner", force, in.tell())
	})
}

func (s *Server) handleDone(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, func(ctx context.Context, b *store.Booking, in actionIn) error {
		return s.mark(ctx, b, store.StatusDone)
	})
}

func (s *Server) handleNoShow(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, func(ctx context.Context, b *store.Booking, in actionIn) error {
		return s.mark(ctx, b, store.StatusNoShow)
	})
}

// mark says how a booking went (after its start).
func (s *Server) mark(ctx context.Context, b *store.Booking, status string) error {
	if b.Status == store.StatusCancelled || b.Status == store.StatusDeclined || b.Status == store.StatusPending {
		return &userErr{"only a booking that was on can be marked"}
	}
	if b.Start().After(time.Now()) {
		return &userErr{"this booking has not started yet"}
	}
	if b.Status == status {
		return nil
	}
	b.Status = status
	if err := s.store.UpdateBooking(ctx, b); err != nil {
		return err
	}
	topic := "booking.done"
	if status == store.StatusNoShow {
		topic = "booking.no_show"
	}
	s.publish(ctx, b.UserID, topic, b.Title, s.eventPayload(ctx, b, nil))
	return nil
}

// handlePaid marks a booking paid (or not); paid can also go into Pocket Money.
func (s *Server) handlePaid(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, func(ctx context.Context, b *store.Booking, in actionIn) error {
		paid := in.Paid == nil || *in.Paid
		b.Paid = paid
		b.PaidAt = ""
		if paid {
			b.PaidAt = store.Stamp(time.Now())
		}
		return s.store.UpdateBooking(ctx, b)
	})
}

// handleEmailGuest writes to the guest from the owner's Pocket Mail.
func (s *Server) handleEmailGuest(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, func(ctx context.Context, b *store.Booking, in actionIn) error {
		if b.Email == "" {
			return &userErr{"this booking has no email address"}
		}
		body := strings.TrimSpace(in.Body)
		if body == "" {
			return &userErr{"write the message first"}
		}
		subject := firstNonEmpty(strings.TrimSpace(in.Subject), b.Title)
		lang := langOK(b.Language)
		lines := s.guestLines(ctx, b, lang)
		text := body + "\n\n"
		for _, l := range lines {
			text += l[0] + ": " + l[1] + "\n"
		}
		text += "\n" + word(lang, "m_manage") + "\n" + s.manageURL(b) + "\n"
		return s.sendMail(ctx, b.UserID, b.Email, b.Name, clip(subject, 200), text,
			mailHTML("#087f5b", body, lines, word(lang, "m_open"), s.manageURL(b)), nil)
	})
}

// ---- personal links ----

func (s *Server) inviteJSON(iv *store.Invite) map[string]any {
	return map[string]any{"id": iv.ID, "type_id": iv.TypeID, "type": iv.TypeTitle, "name": iv.Name, "email": iv.Email, "note": iv.Note,
		"url": s.inviteURL(iv), "used": iv.BookingID > 0, "booking_id": iv.BookingID, "expires_at": iv.ExpiresAt, "source": iv.Source,
		"created_at": iv.CreatedAt}
}

func (s *Server) handleListInvites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := s.store.ListInvites(ctx, actorID(ctx), queryInt(r, "limit"))
	if err != nil {
		s.fail(w, err, "links")
		return
	}
	out := []map[string]any{}
	for _, iv := range list {
		out = append(out, s.inviteJSON(iv))
	}
	writeJSON(w, http.StatusOK, map[string]any{"invites": out})
}

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		TypeID int64  `json:"type_id"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		Note   string `json:"note"`
		Days   int    `json:"days"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	var t *store.Type
	if in.TypeID > 0 {
		var err error
		if t, err = s.store.GetType(ctx, uid, in.TypeID); err != nil {
			writeErr(w, http.StatusBadRequest, "pick one of your booking pages")
			return
		}
	} else {
		ts, _ := s.store.ListTypes(ctx, uid)
		for _, x := range ts {
			if x.Status == store.TypeOpen {
				t = x
				break
			}
		}
		if t == nil {
			writeErr(w, http.StatusBadRequest, "there is no open booking page to make a link for")
			return
		}
	}
	if t.Status == store.TypeDraft {
		writeErr(w, http.StatusBadRequest, "open the booking page first; it is still a draft")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email != "" {
		if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
			writeErr(w, http.StatusBadRequest, "that email address does not look right")
			return
		}
	}
	days := in.Days
	if days <= 0 {
		days = 14
	}
	if days > 365 {
		days = 365
	}
	iv := &store.Invite{TypeID: t.ID, Name: clip(strings.TrimSpace(in.Name), 200), Email: email, Note: clip(strings.TrimSpace(in.Note), 2000),
		ExpiresAt: store.Stamp(time.Now().AddDate(0, 0, days)), Source: firstNonEmpty(linkApp(ctx), "owner"), TypeTitle: t.Title}
	if err := s.store.CreateInvite(ctx, uid, iv); err != nil {
		s.fail(w, err, "link")
		return
	}
	writeJSON(w, http.StatusCreated, s.inviteJSON(iv))
}

func (s *Server) handleDeleteInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteInvite(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "link")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- for the rest of the box ----

func summaryLine(today, pending int) string {
	var parts []string
	if today > 0 {
		parts = append(parts, fmt.Sprintf("%d %s today", today, plural(today, "booking", "bookings")))
	}
	if pending > 0 {
		parts = append(parts, fmt.Sprintf("%d %s for you to confirm", pending, plural(pending, "waits", "wait")))
	}
	if len(parts) == 0 {
		return "No bookings today"
	}
	return upperFirst(strings.Join(parts, ", "))
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	loc := s.loc(ctx)
	now := time.Now().In(loc)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	today, _, err := s.store.ListBookings(ctx, uid, store.BookingFilter{View: "active", From: store.Stamp(day), To: store.Stamp(day.AddDate(0, 0, 1)), Limit: 50})
	if err != nil {
		s.fail(w, err, "summary")
		return
	}
	pendingList, pending, _ := s.store.ListBookings(ctx, uid, store.BookingFilter{View: "pending", Limit: 10})
	upcoming := s.store.CountBookings(ctx, uid, store.BookingFilter{View: "upcoming"})
	week := s.store.CountBookings(ctx, uid, store.BookingFilter{View: "upcoming", To: store.Stamp(day.AddDate(0, 0, 7))})
	nextList, _, _ := s.store.ListBookings(ctx, uid, store.BookingFilter{View: "upcoming", From: store.Stamp(time.Now()), Limit: 1})
	var next any
	if len(nextList) > 0 {
		next = s.viewBooking(nextList[0], loc)
	}
	ts, _ := s.store.ListTypes(ctx, uid)
	open := 0
	for _, t := range ts {
		if t.Status == store.TypeOpen {
			open++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"upcoming": upcoming, "pending": pending, "today": len(today), "this_week": week, "types_open": open,
		"types": len(ts), "next": next, "today_list": s.viewBookings(today, loc), "pending_list": s.viewBookings(pendingList, loc),
		"line": summaryLine(len(today), pending), "url": s.base() + "/", "tz": loc.String()})
}

func (s *Server) dayItem(b *store.Booking, loc *time.Location) map[string]any {
	v := s.viewBooking(b, loc)
	return map[string]any{"kind": "booking", "id": b.ID, "time": v.Time, "end": v.EndTime, "start": v.Start, "title": b.Title + " — " + who(b),
		"type": b.Title, "emoji": b.Emoji, "who": who(b), "email": b.Email, "status": b.Status, "location": whereText(b),
		"meeting_url": b.MeetingURL, "url": v.URL, "minutes": b.Minutes, "date": v.Date}
}

func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	date := chiParam(r, "date")
	loc := s.loc(ctx)
	d, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "the date is YYYY-MM-DD")
		return
	}
	on, _, err := s.store.ListBookings(ctx, uid, store.BookingFilter{View: "active", From: store.Stamp(d), To: store.Stamp(d.AddDate(0, 0, 1)), Limit: 200})
	if err != nil {
		s.fail(w, err, "day")
		return
	}
	made, _, _ := s.store.ListBookings(ctx, uid, store.BookingFilter{Made: [2]string{d.UTC().Format(time.RFC3339Nano), d.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano)}, Limit: 200})
	items, madeItems := []map[string]any{}, []map[string]any{}
	for _, b := range on {
		items = append(items, s.dayItem(b, loc))
	}
	for _, b := range made {
		madeItems = append(madeItems, s.dayItem(b, loc))
	}
	text := ""
	if len(on) > 0 {
		text = fmt.Sprintf("%d %s", len(on), plural(len(on), "booking", "bookings"))
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "count": len(on), "summary": text, "items": items, "made": madeItems})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	loc := s.loc(ctx)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	items, bookings, types := []map[string]any{}, []*bookingView{}, []*typeView{}
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "bookings": bookings, "types": types})
		return
	}
	lim := queryInt(r, "limit")
	if lim <= 0 || lim > 100 {
		lim = 20
	}
	lq := strings.ToLower(q)
	ts, _ := s.store.ListTypes(ctx, uid)
	for _, t := range ts {
		if len(types) < lim && strings.Contains(strings.ToLower(t.Title+" "+t.Description+" "+t.Handle), lq) {
			v := s.viewType(t)
			types = append(types, v)
			items = append(items, map[string]any{"kind": "type", "id": t.ID, "title": t.Title,
				"subtitle": fmt.Sprintf("Booking page · %d min · %d upcoming", t.Minutes(), t.Upcoming), "url": v.Link})
		}
	}
	bs, _, err := s.store.ListBookings(ctx, uid, store.BookingFilter{Query: q, Limit: lim, Newest: true})
	if err != nil {
		s.fail(w, err, "search")
		return
	}
	for _, b := range bs {
		v := s.viewBooking(b, loc)
		bookings = append(bookings, v)
		items = append(items, map[string]any{"kind": "booking", "id": b.ID, "title": who(b) + " — " + b.Title,
			"subtitle": fmtWhen(b, loc, "en") + " · " + statusWord(b.Status), "date": v.Start, "url": v.URL})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "bookings": bookings, "types": types})
}

func statusWord(st string) string {
	switch st {
	case store.StatusPending:
		return "waits for you"
	case store.StatusNoShow:
		return "did not come"
	}
	return st
}

// handleLookup is one person's bookings with you, by email (Contacts, CRM).
func (s *Server) handleLookup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	loc := s.loc(ctx)
	email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
	out := map[string]any{"email": email, "bookings": []any{}, "upcoming": []any{}, "count": 0, "last": "", "next": ""}
	if email == "" {
		writeJSON(w, http.StatusOK, out)
		return
	}
	all, total, err := s.store.ListBookings(ctx, uid, store.BookingFilter{Email: email, Newest: true, Limit: 20})
	if err != nil {
		s.fail(w, err, "lookup")
		return
	}
	up, _, _ := s.store.ListBookings(ctx, uid, store.BookingFilter{Email: email, View: "upcoming", Limit: 10})
	out["bookings"], out["upcoming"], out["count"] = s.viewBookings(all, loc), s.viewBookings(up, loc), total
	if len(up) > 0 {
		out["next"] = up[0].Start().In(loc).Format("2006-01-02")
	}
	for _, b := range all {
		if b.Start().Before(time.Now()) && b.Status != store.StatusCancelled && b.Status != store.StatusDeclined {
			out["last"] = b.Start().In(loc).Format("2006-01-02")
			break
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleInsights counts bookings per week and per page, and how they went.
func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	loc := s.loc(ctx)
	weeks := queryInt(r, "weeks")
	if weeks <= 0 || weeks > 104 {
		weeks = 12
	}
	now := time.Now().In(loc)
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, -1)
	}
	start := monday.AddDate(0, 0, -7*(weeks-1))
	bs, _, err := s.store.ListBookings(ctx, uid, store.BookingFilter{From: store.Stamp(start), To: store.Stamp(monday.AddDate(0, 0, 7)), Limit: 1000})
	if err != nil {
		s.fail(w, err, "insights")
		return
	}
	type week struct {
		Start     string `json:"start"`
		Booked    int    `json:"booked"`
		Cancelled int    `json:"cancelled"`
		Minutes   int    `json:"minutes"`
	}
	ws := make([]week, weeks)
	for i := range ws {
		ws[i].Start = start.AddDate(0, 0, 7*i).Format("2006-01-02")
	}
	type perType struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Emoji string `json:"emoji"`
		Count int    `json:"count"`
	}
	per := map[int64]*perType{}
	counts := map[string]int{}
	hours := make([]int, 24)
	var earned int64
	currency := ""
	for _, b := range bs {
		i := int(b.Start().In(loc).Sub(start).Hours() / (24 * 7))
		if i < 0 || i >= weeks {
			continue
		}
		counts[b.Status]++
		if b.Status == store.StatusCancelled || b.Status == store.StatusDeclined {
			ws[i].Cancelled++
			continue
		}
		ws[i].Booked++
		ws[i].Minutes += b.Minutes
		hours[b.Start().In(loc).Hour()]++
		p := per[b.TypeID]
		if p == nil {
			p = &perType{ID: b.TypeID, Title: b.Title, Emoji: b.Emoji}
			per[b.TypeID] = p
		}
		p.Count++
		if b.Paid && b.Price > 0 && (currency == "" || currency == b.Currency) {
			earned += b.Price
			currency = b.Currency
		}
	}
	types := []*perType{}
	for _, p := range per {
		types = append(types, p)
	}
	sort.Slice(types, func(i, j int) bool { return types[i].Count > types[j].Count })
	views := 0
	for _, n := range s.store.ViewsSince(ctx, uid, start.Format("2006-01-02")) {
		views += n
	}
	writeJSON(w, http.StatusOK, map[string]any{"weeks": ws, "types": types, "statuses": counts, "hours": hours, "paid": earned, "currency": currency,
		"views": views, "from": start.Format("2006-01-02")})
}
