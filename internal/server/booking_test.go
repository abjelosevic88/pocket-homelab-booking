package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// alicePage makes Alice a page open every day of the week, all day, with
// no notice, so tests always find free times.
func alicePage(t *testing.T, a *app, extra map[string]any) map[string]any {
	t.Helper()
	sc := a.ok(a.alice, "GET", "/api/v1/schedules", nil)
	scs := sc["schedules"].([]any)
	sid := int64(num(scs[0].(map[string]any), "id"))
	hours := map[string]any{}
	for _, d := range []string{"0", "1", "2", "3", "4", "5", "6"} {
		hours[d] = [][2]string{{"08:00", "20:00"}}
	}
	a.ok(a.alice, "PUT", "/api/v1/schedules/"+itoa(sid), map[string]any{"hours": hours})
	body := map[string]any{"title": "Intro call", "durations": []int{30}, "notice_min": 0, "days_ahead": 30, "schedule_id": sid, "status": "open"}
	for k, v := range extra {
		body[k] = v
	}
	return a.ok(a.alice, "POST", "/api/v1/types", body)
}

func firstSlot(t *testing.T, a *app, tok, typeID string) string {
	t.Helper()
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	s := a.ok(tok, "GET", "/api/v1/types/"+typeID+"/slots?days=3&from="+tomorrow, nil)
	days := s["days"].([]any)
	if len(days) == 0 {
		t.Fatalf("no free times: %v", s)
	}
	return str(days[0].(map[string]any)["slots"].([]any)[0].(map[string]any), "start")
}

func TestPerPersonIsolation(t *testing.T) {
	a := newApp(t, nil)
	ty := alicePage(t, a, nil)
	tid := id(ty)
	start := firstSlot(t, a, a.alice, tid)
	b := a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Ana", "email": "ana@example.com"})
	bid := id(b)

	// Bob sees none of it, and cannot touch it
	a.expect(404, a.bob, "GET", "/api/v1/types/"+tid, nil)
	a.expect(404, a.bob, "PUT", "/api/v1/types/"+tid, map[string]any{"title": "x"})
	a.expect(404, a.bob, "GET", "/api/v1/types/"+tid+"/slots", nil)
	a.expect(404, a.bob, "GET", "/api/v1/bookings/"+bid, nil)
	a.expect(404, a.bob, "POST", "/api/v1/bookings/"+bid+"/cancel", map[string]any{})
	a.expect(404, a.bob, "DELETE", "/api/v1/bookings/"+bid, nil)
	a.expect(400, a.bob, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Mallory"})
	a.expect(400, a.bob, "POST", "/api/v1/invites", map[string]any{"type_id": n(tid)})
	if l := a.ok(a.bob, "GET", "/api/v1/bookings?view=all", nil)["bookings"].([]any); len(l) != 0 {
		t.Fatalf("Bob sees Alice's bookings: %v", l)
	}
	if l := a.ok(a.bob, "GET", "/api/v1/lookup?email=ana@example.com", nil)["bookings"].([]any); len(l) != 0 {
		t.Fatalf("Bob's lookup finds Alice's booking")
	}
	if ts := a.ok(a.bob, "GET", "/api/v1/types", nil)["types"].([]any); len(ts) != 0 {
		t.Fatal("Bob sees Alice's pages")
	}
	// Bob's own hours are his own
	a.ok(a.bob, "GET", "/api/v1/schedules", nil)
	var n1, n2 int
	_ = a.st.DB.QueryRow(`SELECT COUNT(1) FROM schedules WHERE user_id = ?`, a.ids["Alice"]).Scan(&n1)
	_ = a.st.DB.QueryRow(`SELECT COUNT(1) FROM schedules WHERE user_id = ?`, a.ids["Bob"]).Scan(&n2)
	if n1 != 1 || n2 != 1 {
		t.Fatalf("schedules per person: alice %d bob %d", n1, n2)
	}
	// Alice still has it all
	a.expect(200, a.alice, "GET", "/api/v1/bookings/"+bid, nil)
}

func TestNoDoubleBookingAndMove(t *testing.T) {
	a := newApp(t, nil)
	tid := id(alicePage(t, a, nil))
	start := firstSlot(t, a, a.alice, tid)
	b := a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Ana", "email": "ana@example.com"})
	// the same time again is taken
	a.expect(409, a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Emir"})
	// the owner may book over it on purpose
	a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Emir", "force": true})
	// the slot is gone from the free times
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	s := a.ok(a.alice, "GET", "/api/v1/types/"+tid+"/slots?days=1&from="+tomorrow, nil)
	for _, d := range s["days"].([]any) {
		for _, sl := range d.(map[string]any)["slots"].([]any) {
			if str(sl.(map[string]any), "start") == start {
				t.Fatal("a booked time is still offered")
			}
		}
	}
	// move 30 minutes later
	st, _ := time.Parse(time.RFC3339, start)
	later := st.Add(time.Hour).Format(time.RFC3339)
	m := a.ok(a.alice, "POST", "/api/v1/bookings/"+id(b)+"/move", map[string]any{"start": later})
	if str(m, "start") == start || str(m, "moved_from") == "" {
		t.Fatalf("not moved: %v", m)
	}
	// cancel, then the time is free again
	a.ok(a.alice, "POST", "/api/v1/bookings/"+id(b)+"/cancel", map[string]any{"message": "sorry"})
	a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": later, "name": "Lejla"})
}

func TestIdempotentClientKey(t *testing.T) {
	a := newApp(t, nil)
	tid := id(alicePage(t, a, nil))
	start := firstSlot(t, a, a.alice, tid)
	body := map[string]any{"type_id": n(tid), "start": start, "name": "Ana", "client_key": "crm:deal:7"}
	b1 := a.ok(a.alice, "POST", "/api/v1/bookings", body)
	b2 := a.ok(a.alice, "POST", "/api/v1/bookings", body)
	if id(b1) != id(b2) {
		t.Fatalf("the same key made two bookings: %s %s", id(b1), id(b2))
	}
}

func TestGroupCapacity(t *testing.T) {
	a := newApp(t, nil)
	tid := id(alicePage(t, a, map[string]any{"capacity": 2, "durations": []int{60}}))
	start := firstSlot(t, a, a.alice, tid)
	a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "One"})
	a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Two"})
	a.expect(409, a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Three"})
}

func TestManualConfirm(t *testing.T) {
	a := newApp(t, nil)
	tid := id(alicePage(t, a, map[string]any{"confirm": "manual"}))
	start := firstSlot(t, a, a.alice, tid)
	b := a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Ana", "status": "pending"})
	if str(b, "status") != store.StatusPending {
		t.Fatalf("status %v", b["status"])
	}
	if s := a.ok(a.alice, "GET", "/api/v1/summary", nil); num(s, "pending") != 1 {
		t.Fatalf("summary pending: %v", s["pending"])
	}
	c := a.ok(a.alice, "POST", "/api/v1/bookings/"+id(b)+"/confirm", map[string]any{})
	if str(c, "status") != store.StatusConfirmed {
		t.Fatalf("not confirmed: %v", c["status"])
	}
	a.expect(400, a.alice, "POST", "/api/v1/bookings/"+id(b)+"/confirm", map[string]any{})
}

// ---- the public pages ----

var reStamp = regexp.MustCompile(`name="stamp" value="([^"]+)"`)

func get(t *testing.T, u string) (int, string, string) {
	t.Helper()
	cl := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 test")
	res, err := cl.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b), res.Header.Get("Location")
}

func post(t *testing.T, u string, v url.Values) (int, string, string) {
	t.Helper()
	cl := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequest("POST", u, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 test")
	res, err := cl.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b), res.Header.Get("Location")
}

func TestPublicBookingFlow(t *testing.T) {
	a := newApp(t, nil)
	a.s.formMinAge = 0
	ty := alicePage(t, a, map[string]any{"questions": []map[string]any{{"type": "long", "label": "What about?", "required": true}}})
	h := str(ty, "handle")
	start := firstSlot(t, a, a.alice, id(ty))
	if code, body, _ := get(t, a.url+"/b/"+h); code != 200 || !strings.Contains(body, "Intro call") {
		t.Fatalf("page: %d", code)
	}
	code, body, _ := get(t, a.url+"/b/"+h+"/book?d=30&start="+url.QueryEscape(start))
	if code != 200 {
		t.Fatalf("details: %d", code)
	}
	stamp := reStamp.FindStringSubmatch(body)[1]
	v := url.Values{"start": {start}, "d": {"30"}, "stamp": {stamp}, "name": {"Ana Marić"}, "email": {"ana@example.com"}}
	// a required question left empty
	if code, _, _ := post(t, a.url+"/b/"+h+"/book", v); code != http.StatusUnprocessableEntity {
		t.Fatalf("missing answer accepted: %d", code)
	}
	v.Set("q1", "A new website")
	code, _, loc := post(t, a.url+"/b/"+h+"/book", v)
	if code != 303 || !strings.HasPrefix(loc, "/m/") {
		t.Fatalf("booking: %d %s", code, loc)
	}
	tok := strings.TrimPrefix(strings.SplitN(loc, "?", 2)[0], "/m/")
	// the guest's page, their calendar file
	if code, body, _ := get(t, a.url+"/m/"+tok); code != 200 || !strings.Contains(body, "Ana Marić") {
		t.Fatalf("manage page: %d", code)
	}
	if _, ics, _ := get(t, a.url+"/m/"+tok+"/ics"); !strings.Contains(ics, "BEGIN:VEVENT") {
		t.Fatal("no calendar file")
	}
	// the same time again: sent back to pick another
	code, _, loc = post(t, a.url+"/b/"+h+"/book", v)
	if code != 303 || !strings.Contains(loc, "taken=1") {
		t.Fatalf("double booking from the page: %d %s", code, loc)
	}
	// a robot filling the hidden field is let go without a booking
	v.Set("website", "http://spam")
	v.Set("start", time.Now().Add(72*time.Hour).Truncate(time.Hour).Format(time.RFC3339))
	post(t, a.url+"/b/"+h+"/book", v)
	if l := a.ok(a.alice, "GET", "/api/v1/bookings?view=all", nil)["bookings"].([]any); len(l) != 1 {
		t.Fatalf("want 1 booking, have %d", len(l))
	}
	// the guest cancels
	if code, _, _ := post(t, a.url+"/m/"+tok+"/cancel", url.Values{"reason": {"ill"}}); code != 303 {
		t.Fatalf("cancel: %d", code)
	}
	b := a.ok(a.alice, "GET", "/api/v1/bookings?view=cancelled", nil)["bookings"].([]any)
	if len(b) != 1 || str(b[0].(map[string]any), "cancelled_by") != "guest" {
		t.Fatalf("not cancelled by the guest: %v", b)
	}
	// a draft is not public
	a.ok(a.alice, "PUT", "/api/v1/types/"+id(ty), map[string]any{"status": "draft"})
	if code, _, _ := get(t, a.url+"/b/"+h); code != 404 {
		t.Fatalf("draft is public: %d", code)
	}
}

func TestInviteBooksOnce(t *testing.T) {
	a := newApp(t, nil)
	a.s.formMinAge = 0
	ty := alicePage(t, a, map[string]any{"invite_only": true})
	h := str(ty, "handle")
	if code, _, _ := get(t, a.url+"/b/"+h); code != 404 {
		t.Fatalf("a page for personal links only is open to all: %d", code)
	}
	iv := a.ok(a.alice, "POST", "/api/v1/invites", map[string]any{"type_id": n(id(ty)), "name": "Daniel", "email": "d@example.com"})
	u, _ := url.Parse(str(iv, "url"))
	itok := strings.TrimPrefix(u.Path, "/i/")
	_, _, loc := get(t, a.url+"/i/"+itok)
	if !strings.Contains(loc, "i="+itok) {
		t.Fatalf("invite redirect: %s", loc)
	}
	start := firstSlot(t, a, a.alice, id(ty))
	_, body, _ := get(t, a.url+"/b/"+h+"/book?i="+itok+"&d=30&start="+url.QueryEscape(start))
	if !strings.Contains(body, "d@example.com") {
		t.Fatal("email not filled in")
	}
	stamp := reStamp.FindStringSubmatch(body)[1]
	v := url.Values{"start": {start}, "d": {"30"}, "stamp": {stamp}, "name": {"Daniel"}, "email": {"other@example.com"}, "i": {itok}}
	if code, _, loc := post(t, a.url+"/b/"+h+"/book", v); code != 303 || !strings.HasPrefix(loc, "/m/") {
		t.Fatalf("invite booking: %d %s", code, loc)
	}
	bs := a.ok(a.alice, "GET", "/api/v1/bookings?view=all", nil)["bookings"].([]any)
	if len(bs) != 1 || str(bs[0].(map[string]any), "email") != "d@example.com" || str(bs[0].(map[string]any), "source") != "invite" {
		t.Fatalf("invite booking: %v", bs)
	}
	// used: the personal link leads to the booking now, and cannot book again
	if _, _, loc := get(t, a.url+"/i/"+itok); !strings.HasPrefix(loc, "/m/") {
		t.Fatalf("used invite: %s", loc)
	}
	if code, body, _ := get(t, a.url+"/b/"+h+"?i="+itok); code != 200 || !strings.Contains(body, "already been used") {
		t.Fatal("a used link books again")
	}
}

func TestExportImportIdempotent(t *testing.T) {
	a := newApp(t, nil)
	tid := id(alicePage(t, a, nil))
	start := firstSlot(t, a, a.alice, tid)
	a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Ana"})
	doc := a.s.export(context.Background(), a.ids["Alice"])
	raw, _ := json.Marshal(doc)
	// into Bob's account twice: the second time adds nothing
	var d exportDoc
	_ = json.Unmarshal(raw, &d)
	r1 := a.s.importPocket(context.Background(), a.ids["Bob"], d)
	r2 := a.s.importPocket(context.Background(), a.ids["Bob"], d)
	if r1["added"].(map[string]int)["bookings"] != 1 || r2["added"].(map[string]int)["bookings"] != 0 {
		t.Fatalf("import: %v then %v", r1, r2)
	}
	if ts := a.ok(a.bob, "GET", "/api/v1/types", nil)["types"].([]any); len(ts) != 1 {
		t.Fatalf("Bob's pages after import: %d", len(ts))
	}
	// a Calendly CSV, twice
	csv := "Invitee Name,Invitee Email,Event Type Name,Start Date & Time,End Date & Time,Canceled\n" +
		"Ana Marić,ana@example.com,30 Minute Meeting,2026-01-12 10:00 am,2026-01-12 10:30 am,false\n" +
		"Emir,emir@example.com,30 Minute Meeting,2026-01-13 11:00 am,2026-01-13 11:30 am,true\n"
	c1, err := a.s.importCSV(context.Background(), a.ids["Bob"], []byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := a.s.importCSV(context.Background(), a.ids["Bob"], []byte(csv))
	if c1["added"].(map[string]int)["bookings"] != 2 || c2["added"].(map[string]int)["bookings"] != 0 {
		t.Fatalf("csv: %v then %v", c1, c2)
	}
}

func TestDaySearchLookup(t *testing.T) {
	a := newApp(t, nil)
	tid := id(alicePage(t, a, nil))
	start := firstSlot(t, a, a.alice, tid)
	a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(tid), "start": start, "name": "Ana Marić", "email": "Ana@Example.com"})
	day := start[:10]
	d := a.ok(a.alice, "GET", "/api/v1/day/"+day, nil)
	if num(d, "count") != 1 || len(d["items"].([]any)) != 1 {
		t.Fatalf("day: %v", d)
	}
	s := a.ok(a.alice, "GET", "/api/v1/search?q=mari", nil)
	if len(s["items"].([]any)) == 0 {
		t.Fatalf("search found nothing: %v", s)
	}
	l := a.ok(a.alice, "GET", "/api/v1/lookup?email=ana@example.com", nil)
	if num(l, "count") != 1 || str(l, "next") != day {
		t.Fatalf("lookup: %v", l)
	}
}

func TestLinkScopes(t *testing.T) {
	cases := []struct {
		method, path, scope string
		ok                  bool
	}{
		{"GET", "/api/v1/summary", "booking:read", true},
		{"GET", "/api/v1/bookings", "booking:read", true},
		{"GET", "/api/v1/bookings/4", "booking:read", true},
		{"GET", "/api/v1/types/4/slots", "booking:read", true},
		{"GET", "/api/v1/day/2026-11-18", "booking:read", true},
		{"GET", "/api/v1/lookup", "booking:read", true},
		{"POST", "/api/v1/bookings", "booking:read", false},
		{"POST", "/api/v1/bookings", "booking:write", true},
		{"POST", "/api/v1/bookings/4/cancel", "booking:write", true},
		{"POST", "/api/v1/bookings/4/move", "booking:write", true},
		{"POST", "/api/v1/bookings/4/decline", "booking:write", false},
		{"POST", "/api/v1/bookings/4/make", "booking:write", false},
		{"DELETE", "/api/v1/bookings/4", "booking:write", false},
		{"POST", "/api/v1/invites", "booking:write", true},
		{"PUT", "/api/v1/types/4", "booking:write", true},
		{"DELETE", "/api/v1/types/4", "booking:write", false},
		{"PUT", "/api/v1/schedules/1", "booking:write", false},
		{"GET", "/api/v1/settings", "booking:read", false},
		{"GET", "/api/v1/export.json", "booking:read", false},
		{"GET", "/api/v1/auth/me", "", true},
	}
	for _, c := range cases {
		r, _ := http.NewRequest(c.method, c.path, nil)
		scopes := []string{}
		if c.scope != "" {
			scopes = []string{c.scope}
		}
		if got := linkAllows(scopes, r); got != c.ok {
			t.Errorf("%s %s with %q: %v, want %v", c.method, c.path, c.scope, got, c.ok)
		}
	}
}
