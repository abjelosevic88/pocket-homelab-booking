package server

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- your bookings, out and in ----

// exportDoc is one person's hours, pages and bookings, as JSON.
type exportDoc struct {
	App       string            `json:"app"`
	Version   string            `json:"version"`
	Exported  string            `json:"exported_at"`
	Schedules []*store.Schedule `json:"schedules"`
	Types     []exportType      `json:"types"`
	Bookings  []exportBooking   `json:"bookings"`
}

type exportType struct {
	store.Type
	ScheduleName string `json:"schedule_name"`
}

type exportBooking struct {
	store.Booking
	Start string `json:"start"`
	End   string `json:"end"`
	Key   string `json:"key"`
}

func (s *Server) export(ctx context.Context, uid int64) exportDoc {
	doc := exportDoc{App: "pocket-booking", Version: Version, Exported: store.Now(), Schedules: []*store.Schedule{}, Types: []exportType{}, Bookings: []exportBooking{}}
	scs, _ := s.store.ListSchedules(ctx, uid)
	names := map[int64]string{}
	for _, sc := range scs {
		names[sc.ID] = sc.Name
		doc.Schedules = append(doc.Schedules, sc)
	}
	ts, _ := s.store.ListTypes(ctx, uid)
	for _, t := range ts {
		doc.Types = append(doc.Types, exportType{Type: *t, ScheduleName: names[t.ScheduleID]})
	}
	for off := 0; ; off += 1000 {
		bs, _, _ := s.store.ListBookings(ctx, uid, store.BookingFilter{Limit: 1000, Offset: off})
		for _, b := range bs {
			key := firstNonEmpty(b.ClientKey, "pocket:"+b.Token)
			doc.Bookings = append(doc.Bookings, exportBooking{Booking: *b, Start: b.StartAt, End: b.EndAt, Key: key})
		}
		if len(bs) < 1000 {
			break
		}
	}
	return doc
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", `attachment; filename="pocket-booking-export.json"`)
	writeJSON(w, http.StatusOK, s.export(r.Context(), actorID(r.Context())))
}

// handleImport takes a Pocket Booking export (again and again: nothing is
// doubled), or past bookings from Calendly, Cal.com or a spreadsheet as CSV.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var data []byte
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeErr(w, http.StatusBadRequest, "the file is too big")
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "no file")
			return
		}
		defer f.Close()
		if data, err = io.ReadAll(f); err != nil {
			writeErr(w, http.StatusBadRequest, "could not read the file")
			return
		}
	} else {
		b, err := s.readBody(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "the file is too big")
			return
		}
		data = b
	}
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		var doc exportDoc
		if err := json.Unmarshal(data, &doc); err != nil || doc.App != "pocket-booking" {
			writeErr(w, http.StatusBadRequest, "that is not a Pocket Booking export")
			return
		}
		writeJSON(w, http.StatusOK, s.importPocket(ctx, uid, doc))
		return
	}
	out, err := s.importCSV(ctx, uid, data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) importPocket(ctx context.Context, uid int64, doc exportDoc) map[string]any {
	added := map[string]int{"schedules": 0, "types": 0, "bookings": 0}
	skipped := 0
	mine, _ := s.store.ListSchedules(ctx, uid)
	byName := map[string]int64{}
	for _, sc := range mine {
		byName[sc.Name] = sc.ID
	}
	for _, sc := range doc.Schedules {
		if _, ok := byName[sc.Name]; ok {
			continue
		}
		n := &store.Schedule{Name: sc.Name, TZ: sc.TZ, Hours: sc.Hours, Overrides: sc.Overrides, IsDefault: false}
		if s.store.SaveSchedule(ctx, uid, n) == nil {
			byName[n.Name] = n.ID
			added["schedules"]++
		}
	}
	ts, _ := s.store.ListTypes(ctx, uid)
	typeBy := map[string]int64{}
	for _, t := range ts {
		typeBy[t.Handle] = t.ID
		typeBy["title:"+t.Title] = t.ID
	}
	oldType := map[int64]int64{}
	for _, et := range doc.Types {
		if id, ok := typeBy[et.Handle]; ok {
			oldType[et.ID] = id
			continue
		}
		if id, ok := typeBy["title:"+et.Title]; ok {
			oldType[et.ID] = id
			continue
		}
		t := et.Type
		old := t.ID
		t.ID, t.Source, t.ScheduleID = 0, "", byName[et.ScheduleName]
		if s.store.HandleTaken(ctx, t.Handle, 0) {
			t.Handle = s.newHandle(ctx, t.Title)
		}
		if s.store.CreateType(ctx, uid, &t) == nil {
			oldType[old] = t.ID
			typeBy[t.Handle] = t.ID
			typeBy["title:"+t.Title] = t.ID
			added["types"]++
		}
	}
	for _, eb := range doc.Bookings {
		key := firstNonEmpty(eb.Key, eb.ClientKey)
		if key == "" {
			skipped++
			continue
		}
		if _, err := s.store.BookingByClientKey(ctx, uid, key); err == nil {
			skipped++
			continue
		}
		b := eb.Booking
		b.ID, b.Token, b.ClientKey, b.TypeID = 0, "", key, oldType[eb.TypeID]
		b.StartAt, b.EndAt = firstNonEmpty(eb.Start, b.StartAt), firstNonEmpty(eb.End, b.EndAt)
		b.InviteID, b.Reminded = 0, true
		if b.StartAt == "" || b.EndAt == "" {
			skipped++
			continue
		}
		if err := s.store.CreateBooking(ctx, uid, &b, nil); err != nil {
			skipped++
			continue
		}
		added["bookings"]++
	}
	return map[string]any{"added": added, "skipped": skipped}
}

var csvTimeLayouts = []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02 3:04 pm", "2006-01-02 3:04pm", "2006-01-02T15:04",
	"01/02/2006 15:04", "01/02/2006 3:04 pm", "02.01.2006 15:04", "2006-01-02 03:04:05 PM", "Jan 2, 2006 3:04 PM", "January 2, 2006 3:04 PM"}

func parseCSVTime(v string, loc *time.Location) (time.Time, bool) {
	v = strings.TrimSpace(v)
	for _, l := range csvTimeLayouts {
		if t, err := time.ParseInLocation(l, v, loc); err == nil {
			return t, true
		}
		if t, err := time.ParseInLocation(l, strings.ToLower(v), loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// importCSV reads past bookings exported from Calendly ("Invitee Name",
// "Invitee Email", "Event Type Name", "Start Date & Time", "End Date &
// Time", "Canceled"), Cal.com or any sheet with similar columns. Each
// kind of booking becomes a draft page; nothing is booked twice.
func (s *Server) importCSV(ctx context.Context, uid int64, data []byte) (map[string]any, error) {
	rd := csv.NewReader(bytes.NewReader(data))
	rd.FieldsPerRecord = -1
	rd.LazyQuotes = true
	recs, err := rd.ReadAll()
	if err != nil || len(recs) < 2 {
		return nil, errors.New("that does not look like a CSV with a header row and bookings")
	}
	col := map[string]int{}
	find := func(key string, words ...string) {
		for i, h := range recs[0] {
			h = strings.ToLower(strings.TrimSpace(h))
			for _, w := range words {
				if _, done := col[key]; !done && (h == w || strings.Contains(h, w)) {
					col[key] = i
				}
			}
		}
	}
	find("name", "invitee name", "attendee name", "name")
	find("email", "invitee email", "attendee email", "email")
	find("type", "event type name", "event type", "title", "service", "event")
	find("start", "start date & time", "start time", "starttime", "start")
	find("end", "end date & time", "end time", "endtime", "end")
	find("cancel", "canceled", "cancelled", "status")
	find("phone", "phone")
	find("where", "location")
	if _, ok := col["start"]; !ok {
		return nil, errors.New("the file needs a column with the start (like “Start Date & Time”)")
	}
	loc := s.ownerZone(ctx, uid)
	st, _ := s.store.GetSettings(ctx, uid)
	get := func(rec []string, k string) string {
		if i, ok := col[k]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	types := map[string]*store.Type{}
	ts, _ := s.store.ListTypes(ctx, uid)
	for _, t := range ts {
		types[strings.ToLower(t.Title)] = t
	}
	added, skipped, newTypes := 0, 0, 0
	for _, rec := range recs[1:] {
		start, ok := parseCSVTime(get(rec, "start"), loc)
		if !ok {
			skipped++
			continue
		}
		end, ok := parseCSVTime(get(rec, "end"), loc)
		if !ok || !end.After(start) {
			end = start.Add(30 * time.Minute)
		}
		title := firstNonEmpty(get(rec, "type"), "Imported booking")
		t := types[strings.ToLower(title)]
		if t == nil {
			t = s.newType(ctx, uid, st, "")
			t.Title, t.Status, t.Durations = clip(title, 120), store.TypeDraft, []int{int(end.Sub(start).Minutes())}
			if t.Durations[0] < 5 {
				t.Durations = []int{30}
			}
			t.Handle = s.newHandle(ctx, t.Title)
			if err := s.store.CreateType(ctx, uid, t); err != nil {
				skipped++
				continue
			}
			types[strings.ToLower(title)] = t
			newTypes++
		}
		email := strings.ToLower(get(rec, "email"))
		key := fmt.Sprintf("csv:%s:%s:%s", store.Stamp(start), email, strings.ToLower(title))
		if _, err := s.store.BookingByClientKey(ctx, uid, key); err == nil {
			skipped++
			continue
		}
		status := store.StatusConfirmed
		cancel := strings.ToLower(get(rec, "cancel"))
		switch {
		case cancel == "true" || cancel == "yes" || strings.HasPrefix(cancel, "cancel"):
			status = store.StatusCancelled
		case end.Before(time.Now()):
			status = store.StatusDone
		}
		b := &store.Booking{TypeID: t.ID, Title: t.Title, Emoji: t.Emoji, Status: status, StartAt: store.Stamp(start), EndAt: store.Stamp(end),
			Minutes: int(end.Sub(start).Minutes()), GuestTZ: loc.String(), Name: clip(get(rec, "name"), 200), Email: email, Phone: clip(get(rec, "phone"), 40),
			Location: clip(get(rec, "where"), 500), LocationKind: "place", Source: "import", ClientKey: key, Language: t.Language, Reminded: true}
		if err := s.store.CreateBooking(ctx, uid, b, nil); err != nil {
			skipped++
			continue
		}
		added++
	}
	return map[string]any{"added": map[string]int{"bookings": added, "types": newTypes}, "skipped": skipped}, nil
}
