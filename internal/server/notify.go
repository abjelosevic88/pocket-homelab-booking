package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
)

// ---- telling the owner, reminding the guest ----

// tellOwner sends the owner a push about one booking: new | cancelled | moved.
func (s *Server) tellOwner(ctx context.Context, b *store.Booking, kind string) {
	st, _ := s.store.GetSettings(ctx, b.UserID)
	loc := zone(st)
	when := fmtWhen(b, loc, "en")
	var title, body string
	switch kind {
	case "new":
		title = "New booking: " + b.Title
		body = fmt.Sprintf("%s booked %s.", who(b), when)
		if b.Status == store.StatusPending {
			title = "Booking to confirm: " + b.Title
			body = fmt.Sprintf("%s asks for %s. It waits for your yes.", who(b), when)
		}
	case "cancelled":
		title = "Cancelled: " + b.Title
		body = fmt.Sprintf("%s cancelled %s.", who(b), when)
		if b.CancelReason != "" {
			body += " “" + clip(b.CancelReason, 200) + "”"
		}
	case "moved":
		title = "Moved: " + b.Title
		body = fmt.Sprintf("%s moved their booking to %s.", who(b), when)
	}
	s.sendMessage(ctx, b.UserID, st, title, body, s.bookingLink(b), "info")
}

// sendMessage goes to the person's message URL (Apprise, ntfy or a webhook)
// and, through Pocket Core, to their phones with Pocket Hub.
func (s *Server) sendMessage(ctx context.Context, uid int64, st store.Settings, title, body, link, prio string) bool {
	sent := false
	if st.NotifyURL != "" {
		n := notify.New(st.NotifyURL, st.NotifyFormat)
		if err := n.Send(ctx, notify.Message{Title: title, Body: body, Priority: prio, Tags: []string{"calendar"}, Link: link}); err != nil {
			s.log.Warn("notify failed", "user", uid, "err", err)
		} else {
			sent = true
		}
	}
	if st.PushPhone && s.core.Configured() {
		if u, err := s.store.GetUser(ctx, uid); err == nil {
			res, err := s.core.Notify(ctx, u.Email, core.PushMessage{Title: title, Body: body, URL: link, Source: "booking", Thread: "booking"})
			if err == nil && res != nil && res.Sent > 0 {
				sent = true
			} else if err != nil {
				s.log.Debug("push", "user", uid, "err", err)
			}
		}
	}
	return sent
}

// remind emails guests whose booking is near (each page's own lead time).
func (s *Server) remind(ctx context.Context) {
	due, err := s.store.DueReminders(ctx, time.Now().Add(72*time.Hour))
	if err != nil {
		return
	}
	types := map[int64]*store.Type{}
	for _, b := range due {
		t, ok := types[b.TypeID]
		if !ok {
			t, _ = s.store.GetType(ctx, b.UserID, b.TypeID)
			types[b.TypeID] = t
		}
		hours := 24
		if t != nil {
			hours = t.ReminderH
		}
		if hours <= 0 {
			s.store.MarkReminded(ctx, b.ID)
			continue
		}
		if time.Until(b.Start()) > time.Duration(hours)*time.Hour {
			continue
		}
		// a booking made after its reminder time needs none
		if store.ParseStamp(b.CreatedAt).After(b.Start().Add(-time.Duration(hours) * time.Hour)) {
			s.store.MarkReminded(ctx, b.ID)
			continue
		}
		s.store.MarkReminded(ctx, b.ID)
		st, _ := s.store.GetSettings(ctx, b.UserID)
		if !st.TellGuest || b.Email == "" {
			continue
		}
		if err := s.mailGuest(ctx, b, "reminder", ""); err != nil && !isNotConnected(err) {
			s.log.Warn("booking: reminder failed", "booking", b.ID, "err", err)
		}
	}
}

// pushes runs every minute; at each person's own hour it tells them the
// day's bookings and the ones waiting for a yes.
func (s *Server) pushes(ctx context.Context) {
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	for _, uid := range ids {
		st, err := s.store.GetSettings(ctx, uid)
		if err != nil || !st.NotifyDigest {
			continue
		}
		now := time.Now().In(zone(st))
		if now.Hour() != st.NotifyHour || now.Minute() > 10 {
			continue
		}
		if !s.once(ctx, fmt.Sprintf("push:%d:%s", uid, now.Format("2006-01-02"))) {
			continue
		}
		s.dailyCheck(ctx, uid, st, now)
	}
}

// once is true the first time it is asked for a key.
func (s *Server) once(ctx context.Context, key string) bool {
	if _, _, done := s.store.CacheGet(ctx, key); done {
		return false
	}
	_ = s.store.CachePut(ctx, key, "1")
	return true
}

// dailyCheck builds and sends one person's morning message.
func (s *Server) dailyCheck(ctx context.Context, uid int64, st store.Settings, now time.Time) (string, bool) {
	loc := zone(st)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	today, _, _ := s.store.ListBookings(ctx, uid, store.BookingFilter{View: "active", From: store.Stamp(day), To: store.Stamp(day.AddDate(0, 0, 1)), Limit: 50})
	pending := s.store.CountBookings(ctx, uid, store.BookingFilter{View: "pending"})
	if len(today) == 0 && pending == 0 {
		return "", false
	}
	var lines []string
	if len(today) > 0 {
		var parts []string
		for _, b := range today {
			parts = append(parts, fmt.Sprintf("%s %s (%s)", b.Start().In(loc).Format("15:04"), who(b), b.Title))
		}
		lines = append(lines, fmt.Sprintf("Today: %s.", strings.Join(parts, ", ")))
	}
	if pending > 0 {
		lines = append(lines, fmt.Sprintf("%d %s for your yes.", pending, plural(pending, "booking waits", "bookings wait")))
	}
	body := strings.Join(lines, "\n")
	return body, s.sendMessage(ctx, uid, st, "Your bookings", body, s.base()+"/", "info")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, _ := s.store.GetSettings(ctx, actorID(ctx))
	if !s.sendMessage(ctx, actorID(ctx), st, "Pocket Booking", notifyTestBody, s.base()+"/", "info") {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "nothing could be sent: add a notification address, or connect Pocket Core and Pocket Hub"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleCheckNow sends the morning message now (Settings → Notifications).
func (s *Server) handleCheckNow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, _ := s.store.GetSettings(ctx, uid)
	body, sent := s.dailyCheck(ctx, uid, st, time.Now().In(zone(st)))
	writeJSON(w, http.StatusOK, map[string]any{"sent": sent, "text": body})
}

const notifyTestBody = "Notifications work. Each new booking, a cancellation or a move, and (if you want it) a morning note about the day's bookings will arrive here."
