package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/pocket"
	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- bookings into the other Pocket apps ----
//
// From one booking, as its owner (their own Core link or direct token):
// the event in Pocket Calendar (kept in step when the booking moves or is
// called off), the person in Pocket Contacts (unless they are there), a
// lead in Pocket CRM, a task, a meeting note, a draft invoice, the payment
// in Pocket Money and the time in Pocket Time. Each thing made is
// remembered with a link to it, and a failed try with why.

type makeIn struct {
	Target  string `json:"target"` // event | contact | deal | task | note | invoice | payment | time
	Title   string `json:"title"`
	Notes   string `json:"notes"`
	Due     string `json:"due"`
	Account int64  `json:"account_id"`
}

var makeTargets = map[string]pocketApp{"event": appCalendar, "contact": appContacts, "deal": appCRM, "task": appTasks, "note": appNotes,
	"invoice": appInvoicing, "payment": appMoney, "time": appTime}

// userErr is a reason the person can act on (400, not 502).
type userErr struct{ msg string }

func (e *userErr) Error() string { return e.msg }

// origin is the line that says where a thing came from.
func (s *Server) origin(b *store.Booking) string {
	return fmt.Sprintf("From Pocket Booking — “%s” with %s: %s", b.Title, who(b), s.bookingLink(b))
}

// detailsText lists who booked and what they wrote, as plain lines.
func detailsText(b *store.Booking, loc *time.Location) string {
	lines := []string{"When: " + fmtWhen(b, loc, "en")}
	add := func(k, v string) {
		if v = strings.TrimSpace(v); v != "" {
			lines = append(lines, k+": "+v)
		}
	}
	add("Who", b.Name)
	add("Email", b.Email)
	add("Phone", b.Phone)
	add("Where", whereText(b))
	for _, a := range b.Answers {
		add(a.Label, a.Value)
	}
	if b.Price > 0 {
		paid := "not paid yet"
		if b.Paid {
			paid = "paid"
		}
		add("Price", money(b.Price, b.Currency)+" ("+paid+")")
	}
	return strings.Join(lines, "\n")
}

// detailsMarkdown is the same as Markdown (notes, leads).
func detailsMarkdown(b *store.Booking, loc *time.Location) string {
	var out []string
	for _, l := range strings.Split(detailsText(b, loc), "\n") {
		k, v, _ := strings.Cut(l, ": ")
		out = append(out, "**"+k+":** "+v)
	}
	return strings.Join(out, "\n\n")
}

// make makes one thing from a booking and remembers it.
func (s *Server) make(ctx context.Context, c *clients, t *store.Type, b *store.Booking, in makeIn, auto bool) (*store.Made, error) {
	a, ok := makeTargets[in.Target]
	if !ok {
		return nil, &userErr{"make an event, a contact, a lead, a task, a note, an invoice, a payment or a time entry"}
	}
	if in.Target != "contact" && in.Target != "payment" {
		if prev, err := s.store.MadeOne(ctx, b.UserID, b.ID, in.Target); err == nil && (auto || in.Target == "event" || in.Target == "invoice") {
			return prev, nil // made already: one event, one invoice per booking
		}
	}
	var (
		res  map[string]any
		path string
		err  error
		m    = &store.Made{BookingID: b.ID, Target: in.Target, App: a.slug, Auto: auto}
	)
	loc := c.loc()
	switch in.Target {
	case "contact":
		return s.makeContact(ctx, c, b, m)
	case "event":
		res, path, err = s.makeEvent(ctx, c, b, in, loc)
	case "deal":
		res, path, err = s.makeDeal(ctx, c, b, in, loc)
	case "task":
		res, path, err = s.makeTask(ctx, c, b, in, loc)
	case "note":
		res, path, err = s.makeNote(ctx, c, b, in, loc)
	case "invoice":
		res, path, err = s.makeInvoice(ctx, c, b, in)
	case "payment":
		res, path, err = s.makePayment(ctx, c, b, in)
	case "time":
		res, path, err = s.makeTime(ctx, c, b, in)
	}
	if err != nil {
		if auto {
			m.Error = plainErr(a, err)
			_ = s.store.AddMade(ctx, b.UserID, m)
		}
		return nil, err
	}
	m.Ref = idOf(res)
	m.Title = firstNonEmpty(str(res, "title", "name", "fn", "display_name", "number", "description"), in.Title, b.Title+" — "+who(b))
	if u := str(res, "_url"); u != "" {
		m.URL = u
	} else if sib, _ := c.conn(ctx, a, a.scopes[len(a.scopes)-1]); sib != nil && sib.public != "" && m.Ref != "" && path != "" {
		m.URL = strings.TrimRight(sib.public, "/") + path
	}
	s.store.DropMade(ctx, b.UserID, b.ID, m.Target, true)
	if err := s.store.AddMade(ctx, b.UserID, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *clients) loc() *time.Location { return zone(c.st) }

// plainErr is a failed try in a sentence.
func plainErr(a pocketApp, err error) string {
	if isNotConnected(err) {
		return err.Error()
	}
	var ue *userErr
	if errors.As(err, &ue) {
		return ue.msg
	}
	return a.name + ": " + err.Error()
}

// ---- Pocket Calendar ----

func (s *Server) eventBody(c *clients, b *store.Booking, title string, loc *time.Location) map[string]any {
	if title == "" {
		title = b.Title + " — " + who(b)
		if b.Status == store.StatusPending {
			title = "(to confirm) " + title
		}
	}
	body := map[string]any{"title": clip(title, 200), "description": joinNonEmpty("\n\n", detailsText(b, loc), s.origin(b)),
		"start": b.Start().In(loc).Format(time.RFC3339), "end": b.End().In(loc).Format(time.RFC3339), "tz": loc.String(), "busy": true,
		"url": s.bookingLink(b), "categories": []string{"Booking"}}
	if b.MeetingURL != "" {
		body["conference"] = b.MeetingURL
		body["location"] = b.MeetingURL
	}
	if w := whereText(b); w != "" && b.MeetingURL == "" {
		body["location"] = w
	}
	if c.st.EventCalendar > 0 {
		body["calendar_id"] = c.st.EventCalendar
	}
	return body
}

func (s *Server) makeEvent(ctx context.Context, c *clients, b *store.Booking, in makeIn, loc *time.Location) (map[string]any, string, error) {
	if !store.Active(b.Status) || b.Status == store.StatusNoShow {
		return nil, "", &userErr{"this booking is not on"}
	}
	sib, err := c.conn(ctx, appCalendar, scopeCalendarWrite)
	if err != nil {
		return nil, "", err
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/events", s.eventBody(c, b, strings.TrimSpace(in.Title), loc))
	if err != nil {
		return nil, "", err
	}
	if e, ok := res["event"].(map[string]any); ok {
		res = e
	}
	if id := str(res, "event_id"); id != "" && str(res, "id") == "" {
		res["id"] = id
	}
	return res, "/event/" + idOf(res), nil
}

// syncEvent keeps the calendar event in step: moved with the booking,
// removed when it is called off.
func (s *Server) syncEvent(ctx context.Context, b *store.Booking, remove bool) {
	m, err := s.store.MadeOne(ctx, b.UserID, b.ID, "event")
	if err != nil {
		if !remove && b.Status == store.StatusConfirmed {
			// no event yet (it was waiting for a yes): make it now if the page wants one
			if t, err := s.store.GetType(ctx, b.UserID, b.TypeID); err == nil && t.After.Event {
				c := s.clients(ctx, b.UserID)
				_, _ = s.make(ctx, c, t, b, makeIn{Target: "event"}, true)
			}
		}
		return
	}
	c := s.clients(ctx, b.UserID)
	sib, err := c.conn(ctx, appCalendar, scopeCalendarWrite)
	if err != nil {
		return
	}
	path := "/api/v1/events/" + url.PathEscape(m.Ref)
	if remove {
		var out map[string]any
		if err := sib.client.Do(ctx, http.MethodDelete, path, nil, nil, &out); err != nil {
			s.log.Debug("booking: calendar event not removed", "booking", b.ID, "err", err)
			return
		}
		s.store.DropMade(ctx, b.UserID, b.ID, "event", false)
		return
	}
	var out map[string]any
	if err := sib.client.Do(ctx, http.MethodPut, path, nil, s.eventBody(c, b, "", c.loc()), &out); err != nil {
		s.log.Debug("booking: calendar event not moved", "booking", b.ID, "err", err)
	}
}

// ---- Pocket Contacts ----

func splitName(n string) (given, family string) {
	parts := strings.Fields(n)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	}
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
}

func (s *Server) makeContact(ctx context.Context, c *clients, b *store.Booking, m *store.Made) (*store.Made, error) {
	if b.Name == "" && b.Email == "" && b.Phone == "" {
		return nil, &userErr{"this booking has no name, email or phone"}
	}
	fail := func(err error) (*store.Made, error) {
		if m.Auto {
			m.Error = plainErr(appContacts, err)
			_ = s.store.AddMade(ctx, b.UserID, m)
		}
		return nil, err
	}
	if b.Email != "" {
		if sib, err := c.conn(ctx, appContacts, scopeContactsRead); err == nil {
			if v, err := getJSONFrom(ctx, sib, "/api/v1/lookup", url.Values{"email": {b.Email}}); err == nil {
				if found := rows(v, "contacts"); len(found) > 0 {
					m.Ref, m.Title = idOf(found[0]), firstNonEmpty(str(found[0], "fn", "name", "display_name"), b.Name)+" (already in your contacts)"
					if sib.public != "" && m.Ref != "" {
						m.URL = strings.TrimRight(sib.public, "/") + "/contacts/" + m.Ref
					}
					s.store.DropMade(ctx, b.UserID, b.ID, "contact", false)
					return m, s.store.AddMade(ctx, b.UserID, m)
				}
			}
		}
	}
	sib, err := c.conn(ctx, appContacts, scopeContactsWrite)
	if err != nil {
		return fail(err)
	}
	given, family := splitName(b.Name)
	card := map[string]any{"kind": "individual", "fn": b.Name, "given": given, "family": family, "note": s.origin(b)}
	if b.Email != "" {
		card["emails"] = []map[string]any{{"label": "Other", "value": b.Email, "pref": true}}
	}
	if b.Phone != "" {
		card["phones"] = []map[string]any{{"label": "Mobile", "value": b.Phone}}
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/contacts", map[string]any{"card": card})
	if err != nil {
		return fail(err)
	}
	m.Ref = idOf(res)
	m.Title = firstNonEmpty(str(res, "fn", "name", "display_name"), b.Name, b.Email)
	if sib.public != "" && m.Ref != "" {
		m.URL = strings.TrimRight(sib.public, "/") + "/contacts/" + m.Ref
	}
	s.store.DropMade(ctx, b.UserID, b.ID, "contact", false)
	return m, s.store.AddMade(ctx, b.UserID, m)
}

// ---- Pocket CRM ----

// makeDeal adds the booking to Pocket CRM as a lead; CRM keeps one deal per
// source_ref, so asking twice never makes two.
func (s *Server) makeDeal(ctx context.Context, c *clients, b *store.Booking, in makeIn, loc *time.Location) (map[string]any, string, error) {
	sib, err := c.conn(ctx, appCRM, scopeCRMWrite)
	if err != nil {
		return nil, "", err
	}
	title := firstNonEmpty(strings.TrimSpace(in.Title), who(b)+" — "+b.Title)
	desc := detailsMarkdown(b, loc)
	if n := strings.TrimSpace(in.Notes); n != "" {
		desc = n + "\n\n" + desc
	}
	body := map[string]any{"title": clip(title, 200), "source": "booking", "source_ref": fmt.Sprintf("booking:%d", b.ID),
		"next_action": b.Title + " with " + firstNonEmpty(b.Name, b.Email, "them"), "next_due": b.Start().In(loc).Format("2006-01-02"),
		"note":        fmt.Sprintf("Booked “%s” for %s", b.Title, fmtWhen(b, loc, "en")),
		"description": clip(joinNonEmpty("\n\n---\n", desc, s.origin(b)), 20000)}
	for k, v := range map[string]string{"contact_name": b.Name, "contact_email": b.Email, "contact_phone": b.Phone} {
		if v = strings.TrimSpace(v); v != "" {
			body[k] = v
		}
	}
	if b.Price > 0 {
		body["amount"] = float64(b.Price) / 100
		body["currency"] = b.Currency
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/deals", body)
	if err != nil {
		return nil, "", err
	}
	if d, ok := res["deal"].(map[string]any); ok {
		res = d
	}
	return res, "/deals/" + idOf(res), nil
}

// ---- Pocket Tasks ----

func (s *Server) makeTask(ctx context.Context, c *clients, b *store.Booking, in makeIn, loc *time.Location) (map[string]any, string, error) {
	sib, err := c.conn(ctx, appTasks, scopeTasksWrite)
	if err != nil {
		return nil, "", err
	}
	title := strings.TrimSpace(in.Title)
	due := in.Due
	if title == "" {
		if b.End().Before(time.Now()) {
			title = "Follow up with " + who(b) + " after “" + b.Title + "”"
			if due == "" && c.st.TaskDays > 0 {
				due = time.Now().AddDate(0, 0, c.st.TaskDays).In(loc).Format("2006-01-02")
			}
		} else {
			title = "Get ready: " + b.Title + " with " + who(b)
			if due == "" {
				due = b.Start().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
				if due < time.Now().In(loc).Format("2006-01-02") {
					due = b.Start().In(loc).Format("2006-01-02")
				}
			}
		}
	}
	if due != "" && !validDate(due) {
		return nil, "", &userErr{"the due day is YYYY-MM-DD"}
	}
	body := map[string]any{"title": clip(title, 200), "notes": clip(joinNonEmpty("\n\n", in.Notes, detailsText(b, loc), s.origin(b)), 20000), "url": s.bookingLink(b)}
	if due != "" {
		body["due"] = due
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/tasks", body)
	if err != nil {
		return nil, "", err
	}
	return res, "/today?task=" + idOf(res), nil
}

// ---- Pocket Notes ----

func (s *Server) makeNote(ctx context.Context, c *clients, b *store.Booking, in makeIn, loc *time.Location) (map[string]any, string, error) {
	sib, err := c.conn(ctx, appNotes, scopeNotesWrite)
	if err != nil {
		return nil, "", err
	}
	title := firstNonEmpty(strings.TrimSpace(in.Title), fmt.Sprintf("%s — %s, %s", b.Title, who(b), b.Start().In(loc).Format("2 Jan 2006")))
	content := joinNonEmpty("\n\n", in.Notes, detailsMarkdown(b, loc), "## Notes\n\n", "— "+s.origin(b))
	res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"title": clip(title, 200), "content": content})
	if err != nil {
		return nil, "", err
	}
	return res, "/note/" + idOf(res), nil
}

// ---- Pocket Invoicing ----

func (s *Server) makeInvoice(ctx context.Context, c *clients, b *store.Booking, in makeIn) (map[string]any, string, error) {
	if b.Price <= 0 {
		return nil, "", &userErr{"this booking has no price"}
	}
	name := firstNonEmpty(b.Name, b.Email)
	if name == "" {
		return nil, "", &userErr{"the invoice needs a client: this booking has no name or email"}
	}
	sib, err := c.conn(ctx, appInvoicing, scopeInvoicesWrite)
	if err != nil {
		return nil, "", err
	}
	clientID := int64(0)
	if v, err := getJSONFrom(ctx, sib, "/api/v1/clients", url.Values{"q": {name}}); err == nil {
		for _, cl := range rows(v, "items", "clients") {
			if strings.EqualFold(strings.TrimSpace(str(cl, "name")), name) || (b.Email != "" && strings.EqualFold(str(cl, "email"), b.Email)) {
				clientID = int64(num(cl, "id"))
				break
			}
		}
	}
	if clientID == 0 {
		cl, err := postJSONTo(ctx, sib, "/api/v1/clients", map[string]any{"name": name, "contact_name": b.Name, "email": b.Email, "phone": b.Phone})
		if err != nil {
			return nil, "", err
		}
		clientID = int64(num(cl, "id"))
	}
	desc := firstNonEmpty(strings.TrimSpace(in.Title), fmt.Sprintf("%s — %s", b.Title, b.Start().In(c.loc()).Format("2 Jan 2006, 15:04")))
	body := map[string]any{"client_id": clientID, "currency": b.Currency, "status": "draft",
		"notes": clip(joinNonEmpty("\n", strings.TrimSpace(in.Notes), s.origin(b)), 4000),
		"items": []map[string]any{{"description": clip(desc, 300), "quantity": 1, "unit_price": float64(b.Price) / 100}}}
	res, err := postJSONTo(ctx, sib, "/api/v1/invoices", body)
	if err != nil {
		return nil, "", err
	}
	return res, "/invoices/" + idOf(res), nil
}

// ---- Pocket Money ----

func (s *Server) makePayment(ctx context.Context, c *clients, b *store.Booking, in makeIn) (map[string]any, string, error) {
	if b.Price <= 0 {
		return nil, "", &userErr{"this booking has no price"}
	}
	conn, err := c.conn(ctx, appMoney, scopeAccountsRead, scopeTxWrite)
	if err != nil {
		return nil, "", err
	}
	accts, err := conn.client.Accounts(ctx)
	if err != nil {
		return nil, "", err
	}
	var acct *pocket.Account
	for _, id := range []int64{in.Account, c.st.MoneyAccount} {
		for i := range accts {
			if id > 0 && accts[i].ID == id {
				acct = &accts[i]
			}
		}
		if acct != nil {
			break
		}
	}
	if acct == nil {
		for i := range accts {
			if accts[i].Everyday {
				acct = &accts[i]
				break
			}
		}
	}
	if acct == nil && len(accts) > 0 {
		acct = &accts[0]
	}
	if acct == nil {
		return nil, "", &userErr{"Pocket Money has no account you can put the payment on"}
	}
	day := time.Now().In(c.loc()).Format("2006-01-02")
	if b.PaidAt != "" {
		day = store.ParseStamp(b.PaidAt).In(c.loc()).Format("2006-01-02")
	}
	tx := pocket.Tx{Kind: "income", AccountID: acct.ID, Date: day, Amount: b.Price, Currency: b.Currency,
		Description: b.Title + " — " + who(b), Payee: firstNonEmpty(b.Name, b.Email), Tags: []string{"booking"},
		Notes: s.origin(b), ExternalID: fmt.Sprintf("booking-%d", b.ID), CategoryID: c.st.MoneyIncomeCat}
	id, err := conn.client.CreateTx(ctx, tx)
	if err != nil {
		return nil, "", err
	}
	return map[string]any{"id": id, "title": fmt.Sprintf("%s on %s", money(b.Price, b.Currency), acct.Name)}, fmt.Sprintf("/transactions/%d", id), nil
}

// ---- Pocket Time ----

func (s *Server) makeTime(ctx context.Context, c *clients, b *store.Booking, in makeIn) (map[string]any, string, error) {
	if b.Start().After(time.Now()) {
		return nil, "", &userErr{"log the time once the booking has happened"}
	}
	sib, err := c.conn(ctx, appTime, scopeTimeWrite)
	if err != nil {
		return nil, "", err
	}
	title := firstNonEmpty(strings.TrimSpace(in.Title), b.Title+" — "+who(b))
	res, err := postJSONTo(ctx, sib, "/api/v1/entries", map[string]any{"title": clip(title, 200), "started_at": b.Start().UTC().Format(time.RFC3339),
		"duration_s": b.Minutes * 60, "note": joinNonEmpty("\n", in.Notes, s.origin(b)), "tags": "booking",
		"app_ref": fmt.Sprint(b.ID), "app_label": title, "app_url": s.bookingLink(b), "external_id": fmt.Sprintf("booking-%d", b.ID)})
	if err != nil {
		return nil, "", err
	}
	if e, ok := res["entry"].(map[string]any); ok {
		res = e
	}
	return res, "/entries/" + idOf(res), nil
}

// ---- the handlers ----

func (s *Server) makeFail(w http.ResponseWriter, a pocketApp, err error) {
	var ue *userErr
	switch {
	case errors.As(err, &ue), isNotConnected(err):
		writeErr(w, http.StatusBadRequest, err.Error())
	default:
		writeErr(w, http.StatusBadGateway, a.name+": "+err.Error())
	}
}

func (s *Server) handleMake(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	b, ok := s.ownBooking(w, r)
	if !ok {
		return
	}
	var in makeIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	t, _ := s.store.GetType(ctx, b.UserID, b.TypeID)
	c := s.clients(ctx, b.UserID)
	m, err := s.make(ctx, c, t, b, in, false)
	if err != nil {
		s.makeFail(w, makeTargets[in.Target], err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// handleApps says per app whether bookings can go there (for the buttons).
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	st, _ := s.store.GetSettings(ctx, u.ID)
	out := []map[string]any{}
	for _, a := range pocketApps {
		_, via, detail := s.connStatus(ctx, u, st, a)
		out = append(out, map[string]any{"slug": a.slug, "name": a.name, "reason": a.reason, "connected": via != "", "detail": detail})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAppOptions lists what to pick from in an app: Calendar's
// calendars, Places' places, Money's accounts.
func (s *Server) handleAppOptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	out := []map[string]any{}
	switch chiParam(r, "what") {
	case "calendars":
		sib, err := c.conn(ctx, appCalendar, scopeCalendarRead)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		v, err := getJSONFrom(ctx, sib, "/api/v1/calendars", nil)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "Pocket Calendar: "+err.Error())
			return
		}
		for _, x := range rows(v, "calendars") {
			out = append(out, map[string]any{"id": num(x, "id"), "name": str(x, "name", "title"), "color": str(x, "color"), "role": str(x, "role")})
		}
	case "places":
		sib, err := c.conn(ctx, appPlaces, scopePlacesRead)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		v, err := getJSONFrom(ctx, sib, "/api/v1/places", url.Values{"limit": {"500"}})
		if err != nil {
			writeErr(w, http.StatusBadGateway, "Pocket Places: "+err.Error())
			return
		}
		for _, x := range rows(v, "places", "items") {
			addr := firstNonEmpty(str(x, "address"), strings.TrimSpace(str(x, "city")+" "+str(x, "country")))
			out = append(out, map[string]any{"id": idOf(x), "name": str(x, "name", "title"), "address": addr})
		}
	case "accounts":
		conn, err := c.conn(ctx, appMoney, scopeAccountsRead)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		accts, err := conn.client.Accounts(ctx)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "Pocket Money: "+err.Error())
			return
		}
		for _, a := range accts {
			out = append(out, map[string]any{"id": a.ID, "name": a.Name, "currency": a.Currency, "everyday": a.Everyday})
		}
	default:
		writeErr(w, http.StatusNotFound, "unknown list")
		return
	}
	writeJSON(w, http.StatusOK, out)
}
