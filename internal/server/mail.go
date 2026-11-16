package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- emails to the people who book (through the owner's Pocket Mail) ----

// guestLines are the facts of a booking for the guest, in their language and zone.
func (s *Server) guestLines(ctx context.Context, b *store.Booking, lang string) [][2]string {
	loc := guestZone(b, s.ownerZone(ctx, b.UserID))
	w := func(k string) string { return word(lang, k) }
	lines := [][2]string{{w("when"), fmtWhen(b, loc, lang) + " (" + loc.String() + ")"}}
	if name := s.ownerName(ctx, b.UserID); name != "" {
		lines = append(lines, [2]string{w("with"), name})
	}
	switch b.LocationKind {
	case "video", "link":
		if b.MeetingURL != "" {
			lines = append(lines, [2]string{w("where"), w("video") + ": " + b.MeetingURL})
		}
	case "phone":
		lines = append(lines, [2]string{w("where"), fmt.Sprintf(w("phone_out"), firstNonEmpty(s.ownerName(ctx, b.UserID), "…")) + " (" + b.Phone + ")"})
	case "phone_in":
		lines = append(lines, [2]string{w("where"), fmt.Sprintf(w("phone_in"), b.Location)})
	default:
		if b.Location != "" {
			lines = append(lines, [2]string{w("where"), b.Location})
		}
	}
	if b.Price > 0 {
		lines = append(lines, [2]string{w("price"), money(b.Price, b.Currency)})
	}
	return lines
}

// mailHTML is a short, plain email: paragraphs, the facts, a button.
func mailHTML(accent, body string, lines [][2]string, button, link string) string {
	var h strings.Builder
	esc := html.EscapeString
	h.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>`)
	h.WriteString(`<body style="margin:0;padding:24px 12px;background:#f5f6f8;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1f2433">`)
	h.WriteString(`<div style="max-width:560px;margin:0 auto;background:#fff;border:1px solid #e3e6ee;border-top:5px solid ` + esc(accent) + `;border-radius:12px;padding:24px">`)
	for _, p := range paras(body) {
		h.WriteString(`<p style="margin:0 0 12px;line-height:1.55">` + esc(strings.Join(p, "\n")) + `</p>`)
	}
	if len(lines) > 0 {
		h.WriteString(`<table role="presentation" style="width:100%;border-collapse:collapse;margin-top:8px;font-size:14px">`)
		for _, l := range lines {
			h.WriteString(`<tr><td style="padding:7px 0;border-top:1px solid #eef0f4;color:#646b7d;vertical-align:top;width:30%">` + esc(l[0]) +
				`</td><td style="padding:7px 0 7px 10px;border-top:1px solid #eef0f4">` + esc(l[1]) + `</td></tr>`)
		}
		h.WriteString(`</table>`)
	}
	if link != "" {
		h.WriteString(`<p style="margin:20px 0 0"><a href="` + esc(link) + `" style="display:inline-block;background:` + esc(accent) +
			`;color:#fff;text-decoration:none;padding:10px 18px;border-radius:8px;font-weight:600">` + esc(button) + `</a></p>`)
	}
	h.WriteString(`</div></body></html>`)
	return strings.ReplaceAll(h.String(), "\n", "<br>")
}

// sendMail sends an email for the owner through their Pocket Mail.
func (s *Server) sendMail(ctx context.Context, uid int64, to, toName, subject, text, htmlBody string, ics []byte) error {
	if s.cfg.DemoData {
		s.log.Debug("demo: email not sent", "to", to, "subject", subject)
		return nil
	}
	c := s.clients(ctx, uid)
	sib, err := c.conn(ctx, appMail, scopeMailSend)
	if err != nil {
		return err
	}
	body := map[string]any{"from": c.st.MailFrom, "to": []map[string]string{{"name": toName, "addr": to}}, "subject": subject,
		"body_text": text, "body_html": htmlBody}
	if len(ics) > 0 {
		body["attachments"] = []map[string]string{{"filename": "invite.ics", "mime": "text/calendar", "data": base64.StdEncoding.EncodeToString(ics)}}
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	var out map[string]any
	return sib.client.Do(ctx, http.MethodPost, "/api/v1/send", nil, body, &out)
}

// mailGuest tells the guest about their booking: kind is booked | asked |
// confirmed | declined | cancelled | moved | reminder.
func (s *Server) mailGuest(ctx context.Context, b *store.Booking, kind, message string) error {
	if b.Email == "" {
		return nil
	}
	lang := langOK(b.Language)
	w := func(k string) string { return word(lang, k) }
	owner := firstNonEmpty(s.ownerName(ctx, b.UserID), "—")
	subject := fmt.Sprintf(w("m_"+kind), b.Title)
	intro := w("m_" + kind + "_body")
	if strings.Contains(intro, "%s") {
		intro = fmt.Sprintf(intro, owner)
	}
	if m := strings.TrimSpace(message); m != "" {
		intro += "\n\n" + fmt.Sprintf(w("m_message"), owner) + "\n" + m
	}
	link := s.manageURL(b)
	var ics []byte
	switch kind {
	case "booked", "confirmed", "moved":
		ics = s.icsFor(ctx, b, "REQUEST")
	case "cancelled", "declined":
		ics = s.icsFor(ctx, b, "CANCEL")
	}
	lines := s.guestLines(ctx, b, lang)
	text := intro + "\n\n"
	for _, l := range lines {
		text += l[0] + ": " + l[1] + "\n"
	}
	button := w("m_open")
	if kind != "cancelled" && kind != "declined" {
		text += "\n" + w("m_manage") + "\n" + link + "\n"
	} else {
		button, link = w("book_again"), ""
		if t, err := s.store.GetType(ctx, b.UserID, b.TypeID); err == nil && t.Status == store.TypeOpen && !t.InviteOnly {
			link = s.typeURL(t)
			text += "\n" + w("book_again") + ": " + link + "\n"
		}
	}
	accent := "#087f5b"
	if t, err := s.store.GetType(ctx, b.UserID, b.TypeID); err == nil && t.Accent != "" {
		accent = t.Accent
	}
	return s.sendMail(ctx, b.UserID, b.Email, b.Name, subject, text, mailHTML(accent, intro, lines, button, link), ics)
}

// ---- calendar files ----

func icsEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`).Replace(s)
}

// icsFold folds long lines at 75 octets as the standard asks.
func icsFold(line string) string {
	if len(line) <= 75 {
		return line
	}
	var b strings.Builder
	cur := 0
	for _, r := range line {
		n := len(string(r))
		if cur+n > 75 {
			b.WriteString("\r\n ")
			cur = 1
		}
		b.WriteRune(r)
		cur += n
	}
	return b.String()
}

// icsFor is the booking as a calendar file (method REQUEST or CANCEL; "" = a plain file).
func (s *Server) icsFor(ctx context.Context, b *store.Booking, method string) []byte {
	owner := s.ownerName(ctx, b.UserID)
	ownerEmail := ""
	if u, err := s.store.GetUser(ctx, b.UserID); err == nil {
		ownerEmail = u.Email
	}
	stamp := func(t time.Time) string { return t.UTC().Format("20060102T150405Z") }
	desc := strings.TrimSpace(joinNonEmpty("\n", b.MeetingURL, s.manageURL(b)))
	status, seq := "CONFIRMED", 0
	if b.Status == store.StatusPending {
		status = "TENTATIVE"
	}
	if method == "CANCEL" {
		status = "CANCELLED"
	}
	if b.MovedFrom != "" {
		seq = 1
	}
	if method == "CANCEL" {
		seq += 1
	}
	title := b.Title
	if owner != "" {
		title += " — " + owner
	}
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Pocket Booking//EN", "CALSCALE:GREGORIAN"}
	if method != "" {
		lines = append(lines, "METHOD:"+method)
	}
	lines = append(lines, "BEGIN:VEVENT",
		fmt.Sprintf("UID:booking-%d-%s@pocket-booking", b.ID, b.Token[:min(8, len(b.Token))]),
		"DTSTAMP:"+stamp(time.Now()), "DTSTART:"+stamp(b.Start()), "DTEND:"+stamp(b.End()),
		"SUMMARY:"+icsEscape(title), "STATUS:"+status, fmt.Sprintf("SEQUENCE:%d", seq))
	if desc != "" {
		lines = append(lines, "DESCRIPTION:"+icsEscape(desc))
	}
	if loc := whereText(b); loc != "" {
		lines = append(lines, "LOCATION:"+icsEscape(loc))
	}
	if b.MeetingURL != "" {
		lines = append(lines, "URL:"+b.MeetingURL)
	}
	if ownerEmail != "" && method != "" {
		lines = append(lines, "ORGANIZER;CN="+icsEscape(firstNonEmpty(owner, ownerEmail))+":mailto:"+ownerEmail)
		if b.Email != "" {
			lines = append(lines, "ATTENDEE;CN="+icsEscape(firstNonEmpty(b.Name, b.Email))+";ROLE=REQ-PARTICIPANT;PARTSTAT=ACCEPTED:mailto:"+b.Email)
		}
	}
	if method != "CANCEL" {
		lines = append(lines, "BEGIN:VALARM", "ACTION:DISPLAY", "DESCRIPTION:"+icsEscape(b.Title), "TRIGGER:-PT30M", "END:VALARM")
	}
	lines = append(lines, "END:VEVENT", "END:VCALENDAR")
	var out strings.Builder
	for _, l := range lines {
		out.WriteString(icsFold(l) + "\r\n")
	}
	return []byte(out.String())
}

// paras splits text into paragraphs, and each into lines.
func paras(s string) [][]string {
	var out [][]string
	for _, p := range strings.Split(strings.ReplaceAll(strings.TrimSpace(s), "\r\n", "\n"), "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, strings.Split(p, "\n"))
		}
	}
	return out
}
