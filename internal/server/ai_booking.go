package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- Pocket AI: a page from a sentence, a short brief before a meeting ----

func (s *Server) withAI(w http.ResponseWriter, r *http.Request) (context.Context, func(), bool) {
	select {
	case s.aiSem <- struct{}{}:
	default:
		writeErr(w, http.StatusTooManyRequests, "AI is busy with other work; try again in a moment")
		return nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 150*time.Second)
	return ctx, func() { cancel(); <-s.aiSem }, true
}

// handleAIDraft writes a booking page from a sentence ("a 45 minute guitar
// lesson at my place, beginners welcome, 25 EUR").
func (s *Server) handleAIDraft(w http.ResponseWriter, r *http.Request) {
	uid := actorID(r.Context())
	var in struct {
		Prompt string `json:"prompt"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Prompt) == "" {
		writeErr(w, http.StatusBadRequest, "say in a sentence what people should book")
		return
	}
	ctx, done, ok := s.withAI(w, r)
	if !ok {
		return
	}
	defer done()
	st, _ := s.store.GetSettings(ctx, uid)
	cl, _, why := s.aiFor(ctx, uid, st)
	if cl == nil {
		writeErr(w, http.StatusBadRequest, firstNonEmpty(why, "AI is not set up (Settings → AI)"))
		return
	}
	system := `You set up a booking page (like Calendly) from what the person says. Answer JSON only:
{"title": short name people book (2-5 words), "emoji": one emoji, "description": 1-3 plain friendly sentences for the people booking,
 "durations": [minutes, …] (1-3 lengths, the usual first), "location_kind": "video"|"phone"|"phone_in"|"place"|"ask",
 "location": the address when it is "place", else "", "price_cents": 0 or the price in cents, "currency": "EUR" or what they say,
 "capacity": 1 unless it is a group (class, tour), "confirm": "auto" or "manual" (manual only if they want to approve each),
 "ask_phone": "no"|"optional"|"required", "buffer_after": minutes between bookings (0-30),
 "questions": [{"type": "text"|"long"|"choice"|"yesno"|"number", "label": question, "required": bool, "options": [..] for choice}] (0-3 useful ones; never ask name, email or phone)}.
Write in the language the person wrote in.`
	var out struct {
		Title        string           `json:"title"`
		Emoji        string           `json:"emoji"`
		Description  string           `json:"description"`
		Durations    []int            `json:"durations"`
		LocationKind string           `json:"location_kind"`
		Location     string           `json:"location"`
		PriceCents   int64            `json:"price_cents"`
		Currency     string           `json:"currency"`
		Capacity     int              `json:"capacity"`
		Confirm      string           `json:"confirm"`
		AskPhone     string           `json:"ask_phone"`
		BufferAfter  int              `json:"buffer_after"`
		Questions    []store.Question `json:"questions"`
	}
	if err := cl.JSON(ctx, system, clip(in.Prompt, 2000), &out); err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	// keep only what the page accepts; the person checks it before saving
	if !oneOf(out.LocationKind, store.LocationKinds) {
		out.LocationKind = "video"
	}
	var ds []int
	for _, d := range out.Durations {
		if d >= 5 && d <= 600 && len(ds) < 3 {
			ds = append(ds, d)
		}
	}
	if len(ds) == 0 {
		ds = []int{30}
	}
	qs, err := cleanQuestions(out.Questions)
	if err != nil {
		qs = []store.Question{}
	}
	if !oneOf(out.Confirm, []string{"auto", "manual"}) {
		out.Confirm = "auto"
	}
	if !oneOf(out.AskPhone, []string{"no", "optional", "required"}) {
		out.AskPhone = "no"
	}
	writeJSON(w, http.StatusOK, map[string]any{"title": clip(out.Title, 120), "emoji": clip(out.Emoji, 16), "description": clip(out.Description, 2000),
		"durations": ds, "location_kind": out.LocationKind, "location": clip(out.Location, 300), "price": max(out.PriceCents, 0),
		"currency": firstNonEmpty(cleanCurrency(out.Currency), st.Currency, "EUR"), "capacity": min(max(out.Capacity, 1), 1000),
		"confirm": out.Confirm, "ask_phone": out.AskPhone, "buffer_after": min(max(out.BufferAfter, 0), 120), "questions": qs})
}

// handleAIPrep writes a short brief before a meeting: who they are, what
// they wrote, what they booked before.
func (s *Server) handleAIPrep(w http.ResponseWriter, r *http.Request) {
	b, ok := s.ownBooking(w, r)
	if !ok {
		return
	}
	ctx, done, ok := s.withAI(w, r)
	if !ok {
		return
	}
	defer done()
	st, _ := s.store.GetSettings(ctx, b.UserID)
	cl, _, why := s.aiFor(ctx, b.UserID, st)
	if cl == nil {
		writeErr(w, http.StatusBadRequest, firstNonEmpty(why, "AI is not set up (Settings → AI)"))
		return
	}
	loc := zone(st)
	var before []string
	if b.Email != "" {
		prev, _, _ := s.store.ListBookings(ctx, b.UserID, store.BookingFilter{Email: b.Email, Newest: true, Limit: 10})
		for _, p := range prev {
			if p.ID != b.ID {
				line := fmt.Sprintf("- %s: %s (%s)", p.Start().In(loc).Format("2 Jan 2006"), p.Title, statusWord(p.Status))
				if p.OwnerNote != "" {
					line += " — my note: " + clip(p.OwnerNote, 300)
				}
				before = append(before, line)
			}
		}
	}
	prompt := "The booking:\n" + detailsText(b, loc)
	if b.OwnerNote != "" {
		prompt += "\nMy own note: " + b.OwnerNote
	}
	if len(before) > 0 {
		prompt += "\n\nTheir earlier bookings with me:\n" + strings.Join(before, "\n")
	} else {
		prompt += "\n\nThis is their first booking with me."
	}
	system := `You help someone get ready for a meeting someone booked with them. In plain words and at most 120 words, say:
who it is and what they want (from their answers), anything from earlier bookings worth remembering, and 2-4 short points or questions to have ready.
No headings, a short paragraph then a short list. Do not invent facts.`
	text, err := cl.Complete(ctx, system, prompt, false)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": strings.TrimSpace(text)})
}
