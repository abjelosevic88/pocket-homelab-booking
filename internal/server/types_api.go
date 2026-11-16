package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- booking pages ----

var (
	reHandle = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,60}$`)
	reHex    = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

// slugify turns a title into a web address part: "Uvodni razgovor" → "uvodni-razgovor".
func slugify(s string) string {
	s = slugLetters.Replace(strings.ToLower(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		out = "book"
	}
	return out
}

var slugLetters = strings.NewReplacer("č", "c", "ć", "c", "š", "s", "ž", "z", "đ", "dj", "ß", "ss", "æ", "ae", "ø", "o", "å", "a",
	"ä", "a", "ö", "o", "ü", "u", "á", "a", "à", "a", "â", "a", "é", "e", "è", "e", "ê", "e", "ë", "e", "í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "ú", "u", "ù", "u", "û", "u", "ñ", "n", "ç", "c", "ł", "l", "ń", "n", "ś", "s", "ź", "z", "ż", "z", "ő", "o", "ű", "u")

const handleChars = "abcdefghijkmnpqrstuvwxyz23456789"

// newHandle is the title plus four random characters (box-wide unique).
func (s *Server) newHandle(ctx context.Context, title string) string {
	base := slugify(title)
	for i := 0; i < 20; i++ {
		var b [4]byte
		_, _ = rand.Read(b[:])
		suf := make([]byte, 4)
		for j := range b {
			suf[j] = handleChars[int(b[j])%len(handleChars)]
		}
		h := base + "-" + string(suf)
		if !s.store.HandleTaken(ctx, h, 0) {
			return h
		}
	}
	return base + "-" + store.RandomToken(4)
}

// typeIn is the body of create and update; absent keys stay as they are.
type typeIn struct {
	Template     string            `json:"template"`
	Minutes      int               `json:"minutes"`
	Handle       *string           `json:"handle"`
	Title        *string           `json:"title"`
	Description  *string           `json:"description"`
	Emoji        *string           `json:"emoji"`
	Status       *string           `json:"status"`
	Durations    *[]int            `json:"durations"`
	LocationKind *string           `json:"location_kind"`
	Location     *string           `json:"location"`
	PlaceID      *string           `json:"place_id"`
	ScheduleID   *int64            `json:"schedule_id"`
	NoticeMin    *int              `json:"notice_min"`
	DaysAhead    *int              `json:"days_ahead"`
	BufferBefore *int              `json:"buffer_before"`
	BufferAfter  *int              `json:"buffer_after"`
	StepMin      *int              `json:"step_min"`
	MaxPerDay    *int              `json:"max_per_day"`
	Capacity     *int              `json:"capacity"`
	Confirm      *string           `json:"confirm"`
	AskPhone     *string           `json:"ask_phone"`
	Questions    *[]store.Question `json:"questions"`
	Price        *int64            `json:"price"`
	Currency     *string           `json:"currency"`
	PayNote      *string           `json:"pay_note"`
	ReminderH    *int              `json:"reminder_h"`
	Language     *string           `json:"language"`
	Accent       *string           `json:"accent"`
	InviteOnly   *bool             `json:"invite_only"`
	After        *store.After      `json:"after"`
	Thanks       *string           `json:"thanks"`
}

// apply checks and copies what was sent onto t (a userErr says what is wrong).
func (s *Server) applyType(ctx context.Context, uid int64, t *store.Type, in typeIn) error {
	bad := func(msg string) error { return &userErr{msg} }
	if in.Title != nil {
		t.Title = clip(strings.TrimSpace(*in.Title), 120)
		if t.Title == "" {
			return bad("give the page a name, like “Intro call”")
		}
	}
	if in.Handle != nil {
		h := strings.ToLower(strings.TrimSpace(*in.Handle))
		if !reHandle.MatchString(h) {
			return bad("the address uses small letters, digits and dashes (2 to 61)")
		}
		if s.store.HandleTaken(ctx, h, t.ID) {
			return bad("that address is taken")
		}
		t.Handle = h
	}
	if in.Description != nil {
		t.Description = clip(strings.TrimSpace(*in.Description), 5000)
	}
	if in.Emoji != nil {
		t.Emoji = clip(strings.TrimSpace(*in.Emoji), 16)
	}
	if in.Status != nil {
		if !oneOf(*in.Status, []string{store.TypeDraft, store.TypeOpen, store.TypePaused}) {
			return bad("a page is a draft, open or paused")
		}
		t.Status = *in.Status
	}
	if in.Durations != nil {
		var ds []int
		seen := map[int]bool{}
		for _, d := range *in.Durations {
			if d < 5 || d > 24*60 {
				return bad("a booking lasts 5 minutes to 24 hours")
			}
			if !seen[d] {
				seen[d] = true
				ds = append(ds, d)
			}
		}
		if len(ds) == 0 || len(ds) > 6 {
			return bad("give one to six lengths")
		}
		t.Durations = ds
	}
	if in.LocationKind != nil {
		if !oneOf(*in.LocationKind, store.LocationKinds) {
			return bad("unknown kind of place")
		}
		t.LocationKind = *in.LocationKind
	}
	if in.Location != nil {
		t.Location = clip(strings.TrimSpace(*in.Location), 500)
	}
	if in.PlaceID != nil {
		t.PlaceID = clip(strings.TrimSpace(*in.PlaceID), 40)
	}
	if t.LocationKind == "link" && t.Location != "" {
		if u, ok := cleanURL(t.Location); ok {
			t.Location = u
		} else {
			return bad("the video link is a web address (https://…)")
		}
	}
	if in.ScheduleID != nil {
		if *in.ScheduleID > 0 {
			if _, err := s.store.GetSchedule(ctx, uid, *in.ScheduleID); err != nil {
				return bad("pick one of your weekly hours")
			}
		}
		t.ScheduleID = *in.ScheduleID
	}
	ints := []struct {
		p        *int
		dst      *int
		min, max int
		msg      string
	}{
		{in.NoticeMin, &t.NoticeMin, 0, 60 * 24 * 60, "the notice is 0 to 60 days"},
		{in.DaysAhead, &t.DaysAhead, 0, 730, "people can book up to two years ahead"},
		{in.BufferBefore, &t.BufferBefore, 0, 24 * 60, "a break is up to a day"},
		{in.BufferAfter, &t.BufferAfter, 0, 24 * 60, "a break is up to a day"},
		{in.StepMin, &t.StepMin, 0, 24 * 60, "times start every 0 to 1440 minutes"},
		{in.MaxPerDay, &t.MaxPerDay, 0, 500, "at most 500 a day"},
		{in.Capacity, &t.Capacity, 1, 1000, "1 to 1000 people at a time"},
		{in.ReminderH, &t.ReminderH, 0, 24 * 7, "a reminder goes 0 to 168 hours before"},
	}
	for _, x := range ints {
		if x.p != nil {
			if *x.p < x.min || *x.p > x.max {
				return bad(x.msg)
			}
			*x.dst = *x.p
		}
	}
	if in.StepMin != nil && *in.StepMin > 0 && *in.StepMin < 5 {
		return bad("times start at least 5 minutes apart")
	}
	if in.Confirm != nil {
		if !oneOf(*in.Confirm, []string{"auto", "manual"}) {
			return bad("bookings are on at once (auto) or after you say yes (manual)")
		}
		t.Confirm = *in.Confirm
	}
	if in.AskPhone != nil {
		if !oneOf(*in.AskPhone, []string{"no", "optional", "required"}) {
			return bad("the phone is not asked, optional or required")
		}
		t.AskPhone = *in.AskPhone
	}
	if in.Questions != nil {
		qs, err := cleanQuestions(*in.Questions)
		if err != nil {
			return err
		}
		t.Questions = qs
	}
	if in.Price != nil {
		if *in.Price < 0 || *in.Price > 100_000_000 {
			return bad("the price is not right")
		}
		t.Price = *in.Price
	}
	if in.Currency != nil {
		c := cleanCurrency(*in.Currency)
		if c == "" {
			return bad("the currency is three letters, like EUR")
		}
		t.Currency = c
	}
	if in.PayNote != nil {
		t.PayNote = clip(strings.TrimSpace(*in.PayNote), 2000)
	}
	if in.Language != nil {
		if _, ok := siteWords[*in.Language]; !ok {
			return bad("that language is not offered yet")
		}
		t.Language = *in.Language
	}
	if in.Accent != nil {
		if *in.Accent != "" && !reHex.MatchString(*in.Accent) {
			return bad("the colour is like #087f5b")
		}
		t.Accent = *in.Accent
	}
	if in.InviteOnly != nil {
		t.InviteOnly = *in.InviteOnly
	}
	if in.After != nil {
		t.After = *in.After
	}
	if in.Thanks != nil {
		t.Thanks = clip(strings.TrimSpace(*in.Thanks), 2000)
	}
	if t.LocationKind == "ask" {
		for _, q := range t.Questions {
			if q.ID == "where" {
				return bad("the question id “where” is kept for “Where should we meet?”")
			}
		}
	}
	return nil
}

// cleanQuestions checks the extra questions and gives each an id.
func cleanQuestions(in []store.Question) ([]store.Question, error) {
	if len(in) > 30 {
		return nil, &userErr{"at most 30 questions"}
	}
	out := []store.Question{}
	used := map[string]bool{}
	for i, q := range in {
		q.Label = clip(strings.TrimSpace(q.Label), 300)
		q.Help = clip(strings.TrimSpace(q.Help), 500)
		if q.Label == "" {
			return nil, &userErr{fmt.Sprintf("question %d has no words", i+1)}
		}
		if q.Type == "" {
			q.Type = "text"
		}
		if !oneOf(q.Type, store.QuestionTypes) {
			return nil, &userErr{"unknown kind of question: " + q.Type}
		}
		var opts []string
		for _, o := range q.Options {
			if o = clip(strings.TrimSpace(o), 200); o != "" && !oneOf(o, opts) {
				opts = append(opts, o)
			}
		}
		q.Options = opts
		if (q.Type == "choice" || q.Type == "checkboxes") && len(opts) < 2 {
			return nil, &userErr{fmt.Sprintf("“%s” needs at least two choices", q.Label)}
		}
		if q.Type != "choice" && q.Type != "checkboxes" {
			q.Options = nil
		}
		id := strings.TrimSpace(q.ID)
		if id == "" || used[id] || len(id) > 20 || id == "name" || id == "email" || id == "phone" || id == "where" {
			for n := len(out) + 1; ; n++ {
				id = fmt.Sprintf("q%d", n)
				if !used[id] {
					break
				}
			}
		}
		used[id] = true
		q.ID = id
		out = append(out, q)
	}
	return out, nil
}

// typeTemplates are ready-made pages to start from.
var typeTemplates = map[string]store.Type{
	"intro": {Title: "Intro call", Emoji: "📞", Durations: []int{30}, LocationKind: "video",
		Description: "A short call to get to know each other and see how I can help.",
		Questions:   []store.Question{{ID: "q1", Type: "long", Label: "What would you like to talk about?"}}},
	"consult": {Title: "Consultation", Emoji: "💼", Durations: []int{60, 90}, LocationKind: "video", BufferAfter: 15,
		Description: "A full session on your question, with time for a plan.",
		Questions:   []store.Question{{ID: "q1", Type: "long", Label: "What should we work on?", Required: true}}},
	"meeting": {Title: "Meeting", Emoji: "🤝", Durations: []int{30, 60}, LocationKind: "ask"},
	"viewing": {Title: "Viewing", Emoji: "🏠", Durations: []int{30}, LocationKind: "place", BufferAfter: 15, AskPhone: "required",
		Description: "Come and see it in person.",
		Questions:   []store.Question{{ID: "q1", Type: "number", Label: "How many people will come?"}}},
	"lesson": {Title: "Lesson", Emoji: "🎓", Durations: []int{45, 60}, LocationKind: "place", AskPhone: "optional",
		Questions: []store.Question{{ID: "q1", Type: "choice", Label: "Your level", Options: []string{"Beginner", "Some experience", "Advanced"}}}},
	"appointment": {Title: "Appointment", Emoji: "✂️", Durations: []int{30}, LocationKind: "place", AskPhone: "required", Confirm: "auto"},
	"class": {Title: "Group class", Emoji: "🧘", Durations: []int{60}, LocationKind: "place", Capacity: 8,
		Description: "A class for a small group. Book your place."},
	"phone": {Title: "Phone call", Emoji: "☎️", Durations: []int{15, 30}, LocationKind: "phone", AskPhone: "required"},
}

// handleTemplates lists the ready-made pages.
func templateList() []map[string]any {
	order := []string{"intro", "consult", "meeting", "phone", "viewing", "lesson", "appointment", "class"}
	out := []map[string]any{}
	for _, k := range order {
		t := typeTemplates[k]
		out = append(out, map[string]any{"key": k, "title": t.Title, "emoji": t.Emoji, "durations": t.Durations, "location_kind": t.LocationKind,
			"capacity": max(t.Capacity, 1), "description": t.Description})
	}
	return out
}

// newType is a page with the person's defaults (and a template's content).
func (s *Server) newType(ctx context.Context, uid int64, st store.Settings, tmpl string) *store.Type {
	t := &store.Type{Status: store.TypeOpen, Durations: []int{30}, LocationKind: "video", NoticeMin: 240, DaysAhead: 60, Capacity: 1,
		Confirm: "auto", AskPhone: "no", Questions: []store.Question{}, Currency: firstNonEmpty(st.Currency, "EUR"), ReminderH: 24,
		Language: firstNonEmpty(st.Language, "en"), After: store.After{Event: true}}
	if x, ok := typeTemplates[tmpl]; ok {
		t.Title, t.Emoji, t.Description, t.Durations, t.LocationKind = x.Title, x.Emoji, x.Description, append([]int(nil), x.Durations...), x.LocationKind
		t.BufferAfter, t.Capacity = x.BufferAfter, max(x.Capacity, 1)
		t.Questions = append([]store.Question{}, x.Questions...)
		if x.AskPhone != "" {
			t.AskPhone = x.AskPhone
		}
	}
	if sc, err := s.store.DefaultSchedule(ctx, uid, zone(st).String()); err == nil {
		t.ScheduleID = sc.ID
	}
	return t
}

func (s *Server) handleListTypes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ts, err := s.store.ListTypes(ctx, actorID(ctx))
	if err != nil {
		s.fail(w, err, "pages")
		return
	}
	out := make([]*typeView, 0, len(ts))
	for _, t := range ts {
		out = append(out, s.viewType(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"types": out, "templates": templateList()})
}

func (s *Server) handleCreateType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in typeIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st, _ := s.store.GetSettings(ctx, uid)
	t := s.newType(ctx, uid, st, in.Template)
	if app := linkApp(ctx); app != "" {
		t.Status, t.Source = store.TypeDraft, app
		in.Status = nil
	}
	if in.Minutes > 0 && in.Durations == nil {
		in.Durations = &[]int{in.Minutes}
	}
	if t.Title == "" && in.Title == nil {
		in.Title = ptr("Intro call")
	}
	if err := s.applyType(ctx, uid, t, in); err != nil {
		s.badOrFail(w, err, "page")
		return
	}
	if t.Handle == "" {
		t.Handle = s.newHandle(ctx, t.Title)
	}
	if err := s.store.CreateType(ctx, uid, t); err != nil {
		s.fail(w, err, "create page")
		return
	}
	t, _ = s.store.GetType(ctx, uid, t.ID)
	writeJSON(w, http.StatusCreated, s.viewType(t))
}

func (s *Server) handleGetType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, err := s.store.GetType(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "page")
		return
	}
	writeJSON(w, http.StatusOK, s.viewType(t))
}

func (s *Server) handleUpdateType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	t, err := s.store.GetType(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "page")
		return
	}
	if app := linkApp(ctx); app != "" && t.Source != app {
		writeErr(w, http.StatusForbidden, "this Pocket Core link may only change the pages its app made")
		return
	}
	var in typeIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.applyType(ctx, uid, t, in); err != nil {
		s.badOrFail(w, err, "page")
		return
	}
	if err := s.store.UpdateType(ctx, t); err != nil {
		s.fail(w, err, "save page")
		return
	}
	t, _ = s.store.GetType(ctx, uid, t.ID)
	writeJSON(w, http.StatusOK, s.viewType(t))
}

func (s *Server) handleDeleteType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteType(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete page")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDuplicateType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	t, err := s.store.GetType(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "page")
		return
	}
	t.ID, t.Title, t.Status, t.Source = 0, clip(t.Title+" (copy)", 120), store.TypeDraft, ""
	t.Handle = s.newHandle(ctx, t.Title)
	if err := s.store.CreateType(ctx, uid, t); err != nil {
		s.fail(w, err, "copy page")
		return
	}
	t, _ = s.store.GetType(ctx, uid, t.ID)
	writeJSON(w, http.StatusCreated, s.viewType(t))
}

func (s *Server) handleOrderTypes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.store.MoveType(ctx, actorID(ctx), in.IDs); err != nil {
		s.fail(w, err, "order")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCheckHandle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("handle")))
	ok := reHandle.MatchString(h) && !s.store.HandleTaken(ctx, h, int64(queryInt(r, "id")))
	writeJSON(w, http.StatusOK, map[string]any{"handle": h, "free": ok})
}

// handleSlots lists a page's free times (owner, other apps).
func (s *Server) handleSlots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, err := s.store.GetType(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "page")
		return
	}
	days := queryInt(r, "days")
	if days <= 0 {
		days = 7
	}
	if days > 62 {
		writeErr(w, http.StatusBadRequest, "at most 62 days at a time")
		return
	}
	minutes := queryInt(r, "minutes")
	if minutes <= 0 {
		minutes = t.Minutes()
	}
	if !t.Lasts(minutes) {
		writeErr(w, http.StatusBadRequest, "this page does not offer that length")
		return
	}
	from := r.URL.Query().Get("from")
	if from != "" && !validDate(from) {
		writeErr(w, http.StatusBadRequest, "from is YYYY-MM-DD")
		return
	}
	got, loc, warn, err := s.slotsFor(ctx, t, minutes, from, days)
	if err != nil {
		s.fail(w, err, "times")
		return
	}
	show := loadZone(r.URL.Query().Get("tz"), loc)
	type slot struct {
		Start string `json:"start"`
		End   string `json:"end"`
		Seats int    `json:"seats"`
	}
	type day struct {
		Date  string `json:"date"`
		Slots []slot `json:"slots"`
	}
	out := []day{}
	for _, d := range got {
		dd := day{Date: d.Date, Slots: []slot{}}
		for _, x := range d.Slots {
			dd.Slots = append(dd.Slots, slot{x.Start.In(show).Format(time.RFC3339), x.End.In(show).Format(time.RFC3339), x.Seats})
		}
		out = append(out, dd)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tz": show.String(), "minutes": minutes, "days": out, "calendar_warning": warn})
}

// badOrFail answers a userErr with 400, anything else as a failure.
func (s *Server) badOrFail(w http.ResponseWriter, err error, what string) {
	if ue, ok := err.(*userErr); ok {
		writeErr(w, http.StatusBadRequest, ue.msg)
		return
	}
	if err == store.ErrTaken {
		writeErr(w, http.StatusConflict, "that time is taken")
		return
	}
	s.fail(w, err, what)
}
