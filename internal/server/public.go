package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/avail"
	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- the booking pages as people outside see them ----
//
// /b/<handle> shows a month and the free times of the day picked;
// /b/<handle>/book asks who they are; /m/<token> is the guest's own page
// (add to calendar, move, cancel); /i/<token> is a personal link;
// /p/<handle> lists a person's pages. They are rendered here with no
// JavaScript: a robot that does not wait or fills the hidden field is let
// go without its booking counting.

//go:embed site/*
var siteFS embed.FS

var siteTmpl = template.Must(template.New("site").Funcs(template.FuncMap{
	"safeCSS": func(s string) template.CSS {
		if reHex.MatchString(s) {
			return template.CSS(s)
		}
		return template.CSS("#087f5b")
	},
	"paras": paras,
	"has":   func(m map[string]bool, k string) bool { return m[k] },
	"str":   strconv.Itoa,
}).ParseFS(siteFS, "site/*.html"))

type calDay struct {
	Day      int
	URL      string
	Free     bool
	Selected bool
	Today    bool
	Blank    bool
}

type siteSlot struct {
	Label string
	URL   string
	Seats int
}

type lenOpt struct {
	Label string
	URL   string
	On    bool
}

type siteField struct {
	store.Question
	Value   string
	Checked map[string]bool
	Error   string
	Input   string
}

type personPage struct {
	Title, Emoji, Desc, URL, Length string
}

type sitePage struct {
	Lang     string
	W        map[string]string
	Kind     string // pick | details | manage | cancel | moveto | person | notfound | message
	Title    string
	Accent   string
	Embed    bool
	Preview  bool
	Branding bool
	Type     *store.Type
	Owner    string
	Desc     string
	Where    string
	Price    string
	Notice   string
	Bad      bool
	Message  string

	// pick
	MonthLabel string
	PrevURL    string
	NextURL    string
	DayNames   []string
	Weeks      [][]calDay
	DayLabel   string
	Slots      []siteSlot
	TZ         string
	TZs        []string
	TZForm     map[string]string
	Lengths    []lenOpt
	Minutes    int
	MinLabel   string
	Moving     bool
	ForWhom    string

	// details
	When          string
	Action        string
	Hidden        map[string]string
	Fields        []siteField
	Name, Email   string
	Phone         string
	AskPhone      string
	AskWhere      bool
	WhereVal      string
	Errs          map[string]string
	Stamp         string
	SubmitLabel   string
	NameFixed     bool
	EmailFixed    bool
	BackURL       string
	PayNote       string
	ShowPayNote   bool
	StatusTitle   string
	StatusText    string
	Lines         [][2]string
	MeetingURL    string
	ICSURL        string
	MoveURL       string
	CancelURL     string
	CanChange     bool
	BookAgainURL  string
	Pages         []personPage
	PersonIntro   string
	ConfirmAction string
	ManageURL     string
}

// ---- host switch ----

func hostOnly(h string) string {
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	return strings.ToLower(strings.TrimSuffix(h, "."))
}

func urlHost(u string) string {
	if p, err := url.Parse(u); err == nil {
		return strings.ToLower(p.Hostname())
	}
	return ""
}

// hostSwitch serves only the public pages on the public hostname; every
// other name (BASE_URL, host.docker.internal for Core and the apps acting
// through it, a LAN name) is the whole app.
func (s *Server) hostSwitch(next http.Handler) http.Handler {
	publicHost := urlHost(s.cfg.PublicURL)
	if publicHost == "" || publicHost == urlHost(s.cfg.BaseURL) {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hostOnly(r.Host) == publicHost {
			p := r.URL.Path
			ok := p == "/healthz" || p == "/favicon.svg"
			for _, pre := range []string{"/b/", "/p/", "/m/", "/i/", "/_book/"} {
				ok = ok || strings.HasPrefix(p, pre)
			}
			if !ok {
				http.NotFound(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ---- small guards ----

func (s *Server) siteSecret(ctx context.Context) []byte {
	s.secretOnce.Do(func() {
		v, _, ok := s.store.CacheGet(ctx, "site_secret")
		if !ok || len(v) < 32 {
			v = store.RandomToken(32)
			_ = s.store.CachePut(ctx, "site_secret", v)
		}
		s.secret = []byte(v)
	})
	return s.secret
}

// stamp proves the page was handed out by us, and when.
func (s *Server) stamp(ctx context.Context, typeID int64) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	m := hmac.New(sha256.New, s.siteSecret(ctx))
	fmt.Fprintf(m, "%d.%s", typeID, ts)
	return ts + "." + hex.EncodeToString(m.Sum(nil))[:24]
}

// stampOK: from us, older than the minimum, younger than six hours.
func (s *Server) stampOK(ctx context.Context, typeID int64, v string) (ok, tooFast bool) {
	ts, sig, found := strings.Cut(v, ".")
	if !found {
		return false, false
	}
	m := hmac.New(sha256.New, s.siteSecret(ctx))
	fmt.Fprintf(m, "%d.%s", typeID, ts)
	if !hmac.Equal([]byte(sig), []byte(hex.EncodeToString(m.Sum(nil))[:24])) {
		return false, false
	}
	n, _ := strconv.ParseInt(ts, 10, 64)
	age := time.Since(time.Unix(n, 0))
	return age <= 6*time.Hour, age < s.formMinAge
}

func (s *Server) ipHash(ctx context.Context, r *http.Request) string {
	ip := r.RemoteAddr
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	m := hmac.New(sha256.New, s.siteSecret(ctx))
	m.Write([]byte(ip))
	return hex.EncodeToString(m.Sum(nil))[:16]
}

var reBot = regexp.MustCompile(`(?i)bot|crawl|spider|slurp|preview|facebookexternalhit|whatsapp|telegram|curl|wget`)

func isBot(ua string) bool { return ua == "" || reBot.MatchString(ua) }

// owner is whether the signed-in person reading owns the page.
func (s *Server) owner(r *http.Request, uid int64) bool {
	if s.cfg.AuthMode == "none" {
		return true
	}
	c, err := r.Cookie("pbkg_session")
	if err != nil || c.Value == "" {
		return false
	}
	u, err := s.sessionUser(r.Context(), c.Value)
	return err == nil && u != nil && u.ID == uid
}

// ---- rendering ----

func (s *Server) handleSiteCSS(w http.ResponseWriter, r *http.Request) {
	b, _ := siteFS.ReadFile("site/book.css")
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(b)
}

func (s *Server) renderSite(w http.ResponseWriter, status int, pg *sitePage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; form-action 'self'; base-uri 'none'")
	if !pg.Embed {
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	}
	w.WriteHeader(status)
	if err := siteTmpl.ExecuteTemplate(w, "page", pg); err != nil {
		s.log.Error("site template", "err", err)
	}
}

func (s *Server) basePage(r *http.Request, t *store.Type, lang string) *sitePage {
	lang = langOK(lang)
	if t != nil {
		lang = langOK(t.Language)
	}
	pg := &sitePage{Lang: lang, W: map[string]string{}, Accent: "#087f5b", Branding: true, Embed: r.URL.Query().Get("embed") == "1"}
	for k := range siteWords["en"] {
		pg.W[k] = word(lang, k)
	}
	if t != nil {
		pg.Type, pg.Title = t, t.Title
		if t.Accent != "" {
			pg.Accent = t.Accent
		}
		pg.Owner = s.ownerName(r.Context(), t.UserID)
		pg.Desc = t.Description
		pg.Price = money(t.Price, t.Currency)
		pg.Where = s.whereLine(t, lang, pg.Owner)
	}
	return pg
}

// whereLine says where a page's bookings happen, for the guest.
func (s *Server) whereLine(t *store.Type, lang, owner string) string {
	w := func(k string) string { return word(lang, k) }
	switch t.LocationKind {
	case "video", "link":
		return w("video")
	case "phone":
		return fmt.Sprintf(w("phone_out"), firstNonEmpty(owner, "…"))
	case "phone_in":
		return fmt.Sprintf(w("phone_in"), t.Location)
	case "ask":
		return ""
	}
	return t.Location
}

func (s *Server) siteNotFound(w http.ResponseWriter, r *http.Request) {
	pg := s.basePage(r, nil, "en")
	pg.Kind, pg.Title, pg.Message = "notfound", "Pocket Booking", pg.W["notfound"]
	s.renderSite(w, http.StatusNotFound, pg)
}

func (s *Server) siteMessage(w http.ResponseWriter, r *http.Request, t *store.Type, status int, title, msg string) {
	pg := s.basePage(r, t, "en")
	pg.Kind, pg.StatusTitle, pg.Message = "message", title, msg
	s.renderSite(w, status, pg)
}

// ---- /b/<handle>: pick a time ----

// typeFor loads a page by handle and says whether this reader may book it.
func (s *Server) typeFor(w http.ResponseWriter, r *http.Request) (*store.Type, bool, bool) {
	t, err := s.store.TypeByHandle(r.Context(), chiParam(r, "handle"))
	if err != nil {
		s.siteNotFound(w, r)
		return nil, false, false
	}
	isOwner := s.owner(r, t.UserID)
	if t.Status == store.TypeDraft && !isOwner {
		s.siteNotFound(w, r)
		return nil, false, false
	}
	return t, isOwner, true
}

// inviteFrom reads a personal link from the request (?i=), if it is good.
func (s *Server) inviteFrom(ctx context.Context, r *http.Request, t *store.Type) (*store.Invite, string) {
	tok := firstNonEmpty(r.URL.Query().Get("i"), r.PostFormValue("i"))
	if tok == "" {
		return nil, ""
	}
	iv, err := s.store.InviteByToken(ctx, tok)
	if err != nil || iv.TypeID != t.ID {
		return nil, "invite_gone"
	}
	if iv.BookingID > 0 {
		return nil, "invite_used"
	}
	if iv.ExpiresAt != "" && store.ParseStamp(iv.ExpiresAt).Before(time.Now()) {
		return nil, "invite_gone"
	}
	return iv, ""
}

// keep carries the page's own query on (length, zone, personal link, move).
func keep(q url.Values, extra map[string]string) string {
	v := url.Values{}
	for _, k := range []string{"d", "tz", "i", "move", "embed"} {
		if x := q.Get(k); x != "" {
			v.Set(k, x)
		}
	}
	for k, x := range extra {
		if x == "" {
			v.Del(k)
		} else {
			v.Set(k, x)
		}
	}
	if e := v.Encode(); e != "" {
		return "?" + e
	}
	return ""
}

var commonZones = []string{"Europe/Sarajevo", "Europe/Belgrade", "Europe/Zagreb", "Europe/London", "Europe/Berlin", "Europe/Paris", "Europe/Vienna",
	"Europe/Istanbul", "Europe/Moscow", "America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles", "America/Toronto",
	"America/Sao_Paulo", "Asia/Dubai", "Asia/Kolkata", "Asia/Singapore", "Asia/Tokyo", "Australia/Sydney", "UTC"}

func (s *Server) handleTypePage(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(chiParam(r, "*"), "/")
	switch rest {
	case "":
	case "book":
		s.handleDetails(w, r)
		return
	default:
		s.siteNotFound(w, r)
		return
	}
	ctx := r.Context()
	t, isOwner, ok := s.typeFor(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	pg := s.basePage(r, t, "")
	pg.Kind, pg.Preview = "pick", t.Status == store.TypeDraft
	if q.Get("taken") == "1" {
		pg.Notice, pg.Bad = pg.W["taken"], true
	}
	// moving a booking: the guest's own token says which
	var moving *store.Booking
	if tok := q.Get("move"); tok != "" {
		if b, err := s.store.BookingByToken(ctx, tok); err == nil && b.TypeID == t.ID && (b.Status == store.StatusConfirmed || b.Status == store.StatusPending) {
			moving, pg.Moving = b, true
		}
	}
	if t.Status == store.TypePaused && moving == nil {
		s.siteMessage(w, r, t, http.StatusOK, pg.W["paused_title"], pg.W["paused"])
		return
	}
	iv, ivErr := s.inviteFrom(ctx, r, t)
	if ivErr != "" && moving == nil {
		s.siteMessage(w, r, t, http.StatusOK, t.Title, pg.W[ivErr])
		return
	}
	if t.InviteOnly && iv == nil && moving == nil && !isOwner {
		s.siteNotFound(w, r)
		return
	}
	if iv != nil && iv.Name != "" {
		pg.ForWhom = fmt.Sprintf(pg.W["for_you"], iv.Name)
	}
	st, _ := s.store.GetSettings(ctx, t.UserID)
	ownerLoc := zone(st)
	show := loadZone(q.Get("tz"), ownerLoc)
	pg.TZ = show.String()
	pg.TZs = append([]string{}, commonZones...)
	if !oneOf(pg.TZ, pg.TZs) {
		pg.TZs = append([]string{pg.TZ}, pg.TZs...)
	}
	minutes, _ := strconv.Atoi(q.Get("d"))
	if moving != nil {
		minutes = moving.Minutes
	}
	if !t.Lasts(minutes) {
		minutes = t.Minutes()
	}
	pg.Minutes, pg.MinLabel = minutes, fmt.Sprintf(pg.W["min"], minutes)
	if len(t.Durations) > 1 && moving == nil {
		for _, d := range t.Durations {
			pg.Lengths = append(pg.Lengths, lenOpt{Label: fmt.Sprintf(pg.W["min"], d), URL: "/b/" + t.Handle + keep(q, map[string]string{"d": strconv.Itoa(d)}), On: d == minutes})
		}
	}
	pg.TZForm = map[string]string{}
	for _, k := range []string{"d", "i", "move", "date", "month", "embed"} {
		if v := q.Get(k); v != "" {
			pg.TZForm[k] = v
		}
	}
	// the month shown
	now := time.Now().In(show)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, show)
	if m, err := time.ParseInLocation("2006-01", q.Get("month"), show); err == nil && !m.Before(month) {
		month = m
	}
	sel := q.Get("date")
	if d, err := time.ParseInLocation("2006-01-02", sel, show); err == nil {
		month = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, show)
	} else {
		sel = ""
	}
	// free times for the whole month, in the owner's zone, then shown in the guest's
	from := month
	if from.Before(now) {
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, show)
	}
	days := int(month.AddDate(0, 1, 0).Sub(from).Hours()/24) + 2
	got, _, _, err := s.slotsFor(ctx, t, minutes, from.In(ownerLoc).Format("2006-01-02"), days)
	if err != nil {
		s.log.Error("booking page times", "err", err)
	}
	if moving != nil {
		got = s.withOwnTime(ctx, t, st, minutes, moving, got)
	}
	byDay := map[string][]avail.Slot{}
	for _, d := range got {
		for _, sl := range d.Slots {
			k := sl.Start.In(show).Format("2006-01-02")
			byDay[k] = append(byDay[k], sl)
		}
	}
	if sel == "" {
		// open on the first day that has times
		for i := 0; i < 40; i++ {
			k := from.AddDate(0, 0, i).Format("2006-01-02")
			if len(byDay[k]) > 0 && k[:7] == month.Format("2006-01") {
				sel = k
				break
			}
		}
	}
	pg.MonthLabel = monthName(month, pg.Lang)
	if month.After(time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, show)) {
		pg.PrevURL = "/b/" + t.Handle + keep(q, map[string]string{"month": month.AddDate(0, -1, 0).Format("2006-01"), "date": ""})
	}
	if t.DaysAhead == 0 || month.AddDate(0, 1, 0).Before(now.AddDate(0, 0, t.DaysAhead+1)) {
		pg.NextURL = "/b/" + t.Handle + keep(q, map[string]string{"month": month.AddDate(0, 1, 0).Format("2006-01"), "date": ""})
	}
	// the grid, weeks starting on Monday
	for i := 1; i <= 7; i++ {
		pg.DayNames = append(pg.DayNames, upperFirst(shortDays[pg.Lang][i%7]))
	}
	var week []calDay
	lead := (int(month.Weekday()) + 6) % 7
	for i := 0; i < lead; i++ {
		week = append(week, calDay{Blank: true})
	}
	today := now.Format("2006-01-02")
	for d := month; d.Month() == month.Month(); d = d.AddDate(0, 0, 1) {
		k := d.Format("2006-01-02")
		cd := calDay{Day: d.Day(), Free: len(byDay[k]) > 0, Selected: k == sel, Today: k == today}
		if cd.Free {
			cd.URL = "/b/" + t.Handle + keep(q, map[string]string{"date": k, "month": ""})
		}
		week = append(week, cd)
		if len(week) == 7 {
			pg.Weeks = append(pg.Weeks, week)
			week = nil
		}
	}
	if len(week) > 0 {
		for len(week) < 7 {
			week = append(week, calDay{Blank: true})
		}
		pg.Weeks = append(pg.Weeks, week)
	}
	if sel != "" {
		d, _ := time.ParseInLocation("2006-01-02", sel, show)
		pg.DayLabel = dayName(d, pg.Lang)
		for _, sl := range byDay[sel] {
			ss := siteSlot{Label: sl.Start.In(show).Format("15:04")}
			if t.Capacity > 1 {
				ss.Seats = sl.Seats
			}
			start := sl.Start.In(show).Format(time.RFC3339)
			if moving != nil {
				ss.URL = "/m/" + moving.Token + "/move" + keep(url.Values{"tz": {q.Get("tz")}, "embed": {q.Get("embed")}}, map[string]string{"start": start})
			} else {
				ss.URL = "/b/" + t.Handle + "/book" + keep(q, map[string]string{"start": start, "d": strconv.Itoa(minutes)})
			}
			pg.Slots = append(pg.Slots, ss)
		}
	}
	if !isOwner && !isBot(r.UserAgent()) && moving == nil && q.Get("date") == "" && q.Get("month") == "" {
		s.store.AddView(ctx, t.UserID, t.ID)
	}
	s.renderSite(w, http.StatusOK, pg)
}

// withOwnTime adds back the booking's own time when it is being moved (it is
// not "taken" for itself) — so moving by a few minutes works.
func (s *Server) withOwnTime(ctx context.Context, t *store.Type, st store.Settings, minutes int, b *store.Booking, got []avail.Day) []avail.Day {
	in, err := s.baseInput(ctx, t, st, minutes, time.Now())
	if err != nil {
		return got
	}
	first := time.Now().In(in.Loc)
	in.FromDay = first.Format("2006-01-02")
	in.Days = max(t.DaysAhead, 1) + 1
	if in.Days > 120 {
		in.Days = 120
	}
	bs, err := s.store.Busy(ctx, t.UserID, first.Add(-24*time.Hour), first.AddDate(0, 0, in.Days+1))
	if err != nil {
		return got
	}
	var others []*store.Booking
	for _, x := range bs {
		if x.ID != b.ID {
			others = append(others, x)
		}
	}
	taken(&in, t, others)
	if st.CheckCalendar {
		cal, _ := s.calendarBusy(ctx, t.UserID, st, first.Add(-24*time.Hour), first.AddDate(0, 0, in.Days+1), true)
		in.Busy = append(in.Busy, cal...)
	}
	return avail.Compute(in)
}

// ---- /b/<handle>/book: who are you ----

var rePhone = regexp.MustCompile(`^\+?[0-9 ()./-]{5,30}$`)

func inputType(t string) string {
	switch t {
	case "number", "url", "date":
		return t
	}
	return "text"
}

func (s *Server) handleDetails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, isOwner, ok := s.typeFor(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
	}
	q := r.URL.Query()
	pg := s.basePage(r, t, "")
	pg.Kind, pg.Preview = "details", t.Status == store.TypeDraft
	if t.Status == store.TypePaused {
		s.siteMessage(w, r, t, http.StatusOK, pg.W["paused_title"], pg.W["paused"])
		return
	}
	iv, ivErr := s.inviteFrom(ctx, r, t)
	if ivErr != "" {
		s.siteMessage(w, r, t, http.StatusOK, t.Title, pg.W[ivErr])
		return
	}
	if t.InviteOnly && iv == nil && !isOwner {
		s.siteNotFound(w, r)
		return
	}
	st, _ := s.store.GetSettings(ctx, t.UserID)
	val := func(k string) string { return firstNonEmpty(r.PostFormValue(k), q.Get(k)) }
	show := loadZone(val("tz"), zone(st))
	start, err := time.Parse(time.RFC3339, val("start"))
	minutes, _ := strconv.Atoi(val("d"))
	if !t.Lasts(minutes) {
		minutes = t.Minutes()
	}
	back := "/b/" + t.Handle + keep(q, map[string]string{"start": ""})
	if err != nil {
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	pg.BackURL = "/b/" + t.Handle + keep(q, map[string]string{"start": "", "date": start.In(show).Format("2006-01-02")})
	pg.When = dayName(start.In(show), pg.Lang) + ", " + start.In(show).Format("15:04") + "–" + start.In(show).Add(time.Duration(minutes)*time.Minute).Format("15:04")
	pg.TZ, pg.Minutes, pg.MinLabel = show.String(), minutes, fmt.Sprintf(pg.W["min"], minutes)
	pg.Action = "/b/" + t.Handle + "/book"
	pg.Hidden = map[string]string{"start": start.Format(time.RFC3339), "d": strconv.Itoa(minutes), "tz": val("tz"), "i": val("i"), "embed": val("embed")}
	pg.AskPhone, pg.AskWhere = t.AskPhone, t.LocationKind == "ask"
	if t.LocationKind == "phone" && pg.AskPhone == "no" {
		pg.AskPhone = "required"
	}
	pg.SubmitLabel = pg.W["book"]
	if t.Confirm == "manual" {
		pg.SubmitLabel = pg.W["request"]
	}
	pg.PayNote, pg.ShowPayNote = t.PayNote, t.Price > 0 && t.PayNote != ""
	if iv != nil {
		pg.Name, pg.Email = iv.Name, iv.Email
		pg.NameFixed, pg.EmailFixed = false, iv.Email != ""
	}
	pg.Errs = map[string]string{}
	pg.Stamp = s.stamp(ctx, t.ID)
	if r.Method != http.MethodPost {
		pg.Fields = s.siteFields(t, url.Values{}, nil)
		s.renderSite(w, http.StatusOK, pg)
		return
	}
	// sent
	pg.Name, pg.Email, pg.Phone, pg.WhereVal = clip(strings.TrimSpace(r.PostFormValue("name")), 200), strings.ToLower(strings.TrimSpace(r.PostFormValue("email"))),
		strings.TrimSpace(r.PostFormValue("phone")), clip(strings.TrimSpace(r.PostFormValue("where")), 500)
	if iv != nil && iv.Email != "" {
		pg.Email = iv.Email
	}
	answers, errs := readAnswers(t, r.PostForm, pg.Lang)
	pg.Fields = s.siteFields(t, r.PostForm, errs)
	w8 := func(k string) string { return pg.W[k] }
	if pg.Name == "" {
		pg.Errs["name"] = w8("e_required")
	}
	if a, err := mail.ParseAddress(pg.Email); pg.Email == "" {
		pg.Errs["email"] = w8("e_required")
	} else if err != nil || a.Address != pg.Email || !strings.Contains(pg.Email, ".") {
		pg.Errs["email"] = w8("e_email")
	}
	if pg.Phone != "" && !rePhone.MatchString(pg.Phone) {
		pg.Errs["phone"] = w8("e_phone")
	} else if pg.Phone == "" && pg.AskPhone == "required" {
		pg.Errs["phone"] = w8("e_required")
	}
	if pg.AskWhere && pg.WhereVal == "" {
		pg.Errs["where"] = w8("e_required")
	}
	if len(errs) > 0 || len(pg.Errs) > 0 {
		pg.Notice, pg.Bad = w8("fix"), true
		s.renderSite(w, http.StatusUnprocessableEntity, pg)
		return
	}
	// robots: the hidden field, a page sent too fast or not handed out by us
	stampGood, tooFast := s.stampOK(ctx, t.ID, r.PostFormValue("stamp"))
	if r.PostFormValue("website") != "" || tooFast {
		pg.Kind, pg.StatusTitle, pg.Message = "message", w8("booked_title"), fmt.Sprintf(w8("booked"), pg.Email)
		s.renderSite(w, http.StatusOK, pg)
		return
	}
	if !stampGood {
		pg.Notice, pg.Bad = w8("stamp"), true
		s.renderSite(w, http.StatusUnprocessableEntity, pg)
		return
	}
	if pg.Preview {
		pg.Kind, pg.StatusTitle, pg.Message = "message", w8("preview_sent"), ""
		s.renderSite(w, http.StatusOK, pg)
		return
	}
	ip := s.ipHash(ctx, r)
	if !isOwner && s.store.RecentBookings(ctx, ip, time.Now().Add(-time.Hour)) >= 10 {
		pg.Notice, pg.Bad = w8("wait"), true
		s.renderSite(w, http.StatusTooManyRequests, pg)
		return
	}
	if pg.AskWhere {
		answers = append([]store.Answer{{ID: "where", Label: word(pg.Lang, "where_q"), Value: pg.WhereVal}}, answers...)
	}
	rq := bookReq{Type: t, Start: start, Minutes: minutes, Name: pg.Name, Email: pg.Email, Phone: pg.Phone, Answers: answers,
		GuestTZ: show.String(), Language: pg.Lang, Source: "page", IPHash: ip, TellGuest: true}
	if iv != nil {
		rq.Source, rq.InviteID = "invite", iv.ID
	}
	b, err := s.book(ctx, rq)
	if errors.Is(err, store.ErrTaken) {
		http.Redirect(w, r, "/b/"+t.Handle+keep(q, map[string]string{"start": "", "date": start.In(show).Format("2006-01-02"), "taken": "1"})+"", http.StatusSeeOther)
		return
	}
	if err != nil {
		var ue *userErr
		if errors.As(err, &ue) {
			pg.Notice, pg.Bad = ue.msg, true
			s.renderSite(w, http.StatusUnprocessableEntity, pg)
			return
		}
		s.log.Error("booking from page", "err", err)
		http.Error(w, "something went wrong; please try again", http.StatusInternalServerError)
		return
	}
	extra := map[string]string{"new": "1"}
	if q.Get("embed") == "1" || r.PostFormValue("embed") == "1" {
		extra["embed"] = "1"
	}
	http.Redirect(w, r, "/m/"+b.Token+keep(url.Values{}, extra), http.StatusSeeOther)
}

// siteFields fills the questions with what was sent.
func (s *Server) siteFields(t *store.Type, vals url.Values, errs map[string]string) []siteField {
	out := make([]siteField, 0, len(t.Questions))
	for _, q := range t.Questions {
		sf := siteField{Question: q, Checked: map[string]bool{}, Error: errs[q.ID], Input: inputType(q.Type), Value: clip(vals.Get(q.ID), 5000)}
		for _, v := range vals[q.ID] {
			sf.Checked[v] = true
		}
		out = append(out, sf)
	}
	return out
}

// readAnswers checks the extra questions; the map has an error per question id.
func readAnswers(t *store.Type, vals url.Values, lang string) ([]store.Answer, map[string]string) {
	out := []store.Answer{}
	errs := map[string]string{}
	w := func(k string) string { return word(lang, k) }
	for _, q := range t.Questions {
		raw := strings.TrimSpace(vals.Get(q.ID))
		switch q.Type {
		case "checkboxes":
			var picked []string
			for _, v := range vals[q.ID] {
				if oneOf(v, q.Options) {
					picked = append(picked, v)
				}
			}
			raw = strings.Join(picked, ", ")
		case "choice":
			if raw != "" && !oneOf(raw, q.Options) {
				errs[q.ID] = w("e_choice")
				continue
			}
		case "yesno":
			switch raw {
			case "yes":
				raw = w("yes")
			case "no":
				raw = w("no")
			case "":
			default:
				errs[q.ID] = w("e_choice")
				continue
			}
		case "number":
			if raw != "" {
				if _, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64); err != nil {
					errs[q.ID] = w("e_number")
					continue
				}
			}
		case "url":
			if raw != "" {
				if u, ok := cleanURL(raw); ok {
					raw = u
				} else if u, ok := cleanURL("https://" + raw); ok {
					raw = u
				} else {
					errs[q.ID] = w("e_url")
					continue
				}
			}
		case "date":
			if raw != "" && !validDate(raw) {
				errs[q.ID] = w("e_date")
				continue
			}
		}
		if len(raw) > 5000 {
			errs[q.ID] = w("e_long")
			continue
		}
		if raw == "" {
			if q.Required {
				errs[q.ID] = w("e_required")
			}
			continue
		}
		out = append(out, store.Answer{ID: q.ID, Label: q.Label, Value: raw})
	}
	return out, errs
}

// ---- /m/<token>: the guest's own booking ----

func (s *Server) handleManagePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	b, err := s.store.BookingByToken(ctx, chiParam(r, "token"))
	if err != nil {
		s.siteNotFound(w, r)
		return
	}
	t, _ := s.store.GetType(ctx, b.UserID, b.TypeID)
	rest := strings.Trim(chiParam(r, "*"), "/")
	if rest == "ics" {
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="booking.ics"`)
		_, _ = w.Write(s.icsFor(ctx, b, ""))
		return
	}
	pg := s.basePage(r, t, b.Language)
	if t == nil {
		pg.Title = b.Title
		pg.Owner = s.ownerName(ctx, b.UserID)
	}
	pg.Lang = langOK(b.Language)
	for k := range siteWords["en"] {
		pg.W[k] = word(pg.Lang, k)
	}
	st, _ := s.store.GetSettings(ctx, b.UserID)
	q := r.URL.Query()
	show := loadZone(firstNonEmpty(q.Get("tz"), b.GuestTZ), zone(st))
	live := b.Status == store.StatusPending || b.Status == store.StatusConfirmed
	pg.CanChange = live && b.Start().After(time.Now())
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
	}
	switch rest {
	case "cancel":
		if !pg.CanChange {
			s.siteMessage(w, r, t, http.StatusOK, b.Title, pg.W["too_late"])
			return
		}
		if r.Method == http.MethodPost {
			if err := s.cancel(ctx, b, "guest", r.PostFormValue("reason"), true); err != nil {
				s.siteMessage(w, r, t, http.StatusOK, b.Title, err.Error())
				return
			}
			http.Redirect(w, r, "/m/"+b.Token, http.StatusSeeOther)
			return
		}
		pg.Kind = "cancel"
	case "move":
		if !pg.CanChange || t == nil {
			s.siteMessage(w, r, t, http.StatusOK, b.Title, pg.W["too_late"])
			return
		}
		start, err := time.Parse(time.RFC3339, firstNonEmpty(r.PostFormValue("start"), q.Get("start")))
		if err != nil {
			http.Redirect(w, r, "/b/"+t.Handle+"?move="+b.Token, http.StatusSeeOther)
			return
		}
		if r.Method == http.MethodPost {
			err := s.move(ctx, b, start, b.Minutes, "guest", false, true)
			if errors.Is(err, store.ErrTaken) {
				http.Redirect(w, r, "/b/"+t.Handle+"?move="+b.Token+"&taken=1&date="+start.In(show).Format("2006-01-02"), http.StatusSeeOther)
				return
			}
			if err != nil {
				s.siteMessage(w, r, t, http.StatusOK, b.Title, err.Error())
				return
			}
			http.Redirect(w, r, "/m/"+b.Token+"?moved=1", http.StatusSeeOther)
			return
		}
		pg.Kind = "moveto"
		pg.When = dayName(start.In(show), pg.Lang) + ", " + start.In(show).Format("15:04") + "–" + start.In(show).Add(time.Duration(b.Minutes)*time.Minute).Format("15:04")
		pg.ConfirmAction = "/m/" + b.Token + "/move"
		pg.Hidden = map[string]string{"start": start.Format(time.RFC3339)}
		pg.BackURL = "/b/" + t.Handle + "?move=" + b.Token
	case "":
		pg.Kind = "manage"
	default:
		s.siteNotFound(w, r)
		return
	}
	pg.Lines = s.guestLines(ctx, b, pg.Lang)
	pg.Lines[0][1] = fmtWhen(b, show, pg.Lang) + " (" + show.String() + ")"
	if b.Name != "" {
		pg.Lines = append([][2]string{{pg.W["name"], b.Name}}, pg.Lines...)
	}
	pg.MeetingURL = b.MeetingURL
	pg.ICSURL = "/m/" + b.Token + "/ics"
	pg.CancelURL = "/m/" + b.Token + "/cancel"
	pg.ManageURL = "/m/" + b.Token
	if t != nil {
		pg.MoveURL = "/b/" + t.Handle + "?move=" + b.Token
		if t.Status == store.TypeOpen && !t.InviteOnly {
			pg.BookAgainURL = "/b/" + t.Handle
		}
		pg.PayNote, pg.ShowPayNote = t.PayNote, b.Price > 0 && !b.Paid && t.PayNote != ""
	}
	switch {
	case b.Status == store.StatusCancelled:
		pg.StatusTitle = pg.W["cancelled_title"]
		if b.CancelledBy == "owner" {
			pg.StatusText = fmt.Sprintf(pg.W["by_owner"], firstNonEmpty(pg.Owner, "—"))
		}
		if b.CancelReason != "" {
			pg.Lines = append(pg.Lines, [2]string{pg.W["reason"], b.CancelReason})
		}
	case b.Status == store.StatusDeclined:
		pg.StatusTitle = pg.W["declined_title"]
		if b.CancelReason != "" {
			pg.Lines = append(pg.Lines, [2]string{pg.W["reason"], b.CancelReason})
		}
	case !b.End().After(time.Now()):
		pg.StatusTitle = pg.W["over_title"]
	case b.Status == store.StatusPending:
		pg.StatusTitle, pg.StatusText = pg.W["pending_title"], fmt.Sprintf(pg.W["pending"], firstNonEmpty(pg.Owner, "—"))
	case q.Get("moved") == "1":
		pg.StatusTitle = pg.W["moved_title"]
	default:
		pg.StatusTitle = pg.W["booked_title"]
		if q.Get("new") == "1" {
			if b.Email != "" {
				pg.StatusText = fmt.Sprintf(pg.W["booked"], b.Email)
			} else {
				pg.StatusText = pg.W["booked_noemail"]
			}
		}
	}
	if !live {
		pg.MeetingURL = ""
	}
	s.renderSite(w, http.StatusOK, pg)
}

// ---- /i/<token>: a personal link ----

func (s *Server) handleInvitePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	iv, err := s.store.InviteByToken(ctx, chiParam(r, "token"))
	if err != nil {
		s.siteNotFound(w, r)
		return
	}
	t, err := s.store.TypeAny(ctx, iv.TypeID)
	if err != nil {
		s.siteNotFound(w, r)
		return
	}
	if iv.BookingID > 0 {
		if b, err := s.store.GetBooking(ctx, iv.UserID, iv.BookingID); err == nil {
			http.Redirect(w, r, "/m/"+b.Token, http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/b/"+t.Handle+"?i="+url.QueryEscape(iv.Token), http.StatusSeeOther)
}

// ---- /p/<handle>: a person's pages ----

func (s *Server) handlePersonPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid, err := s.store.PageOwner(ctx, chiParam(r, "handle"))
	if err != nil {
		s.siteNotFound(w, r)
		return
	}
	st, _ := s.store.GetSettings(ctx, uid)
	pg := s.basePage(r, nil, st.Language)
	pg.Kind = "person"
	pg.Owner = s.ownerName(ctx, uid)
	pg.Title = fmt.Sprintf(pg.W["pages_of"], pg.Owner)
	pg.PersonIntro = st.PageIntro
	if st.Accent != "" {
		pg.Accent = st.Accent
	}
	ts, _ := s.store.ListTypes(ctx, uid)
	for _, t := range ts {
		if t.Status != store.TypeOpen || t.InviteOnly {
			continue
		}
		var ls []string
		for _, d := range t.Durations {
			ls = append(ls, strconv.Itoa(d))
		}
		pg.Pages = append(pg.Pages, personPage{Title: t.Title, Emoji: t.Emoji, Desc: clip(t.Description, 240), URL: "/b/" + t.Handle,
			Length: strings.Join(ls, " / ") + " min"})
	}
	s.renderSite(w, http.StatusOK, pg)
}
