package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/avail"
	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- which times a page can still be booked at ----
//
// A page's schedule gives the hours; the person's own bookings (any page),
// the times they are busy in Pocket Calendar, the notice, how far ahead,
// the buffers and the limit per day take times away. The same rules decide
// the times shown on the public page and, inside one transaction, whether
// a time asked for is still free.

// schedFor returns the page's schedule (else the person's default).
func (s *Server) schedFor(ctx context.Context, t *store.Type, st store.Settings) (*store.Schedule, error) {
	if t.ScheduleID > 0 {
		if sc, err := s.store.GetSchedule(ctx, t.UserID, t.ScheduleID); err == nil {
			return sc, nil
		}
	}
	return s.store.DefaultSchedule(ctx, t.UserID, zone(st).String())
}

func toAvail(sp []store.Span) []avail.Span {
	out := make([]avail.Span, 0, len(sp))
	for _, x := range sp {
		out = append(out, avail.Span(x))
	}
	return out
}

// baseInput is everything but what is taken.
func (s *Server) baseInput(ctx context.Context, t *store.Type, st store.Settings, minutes int, now time.Time) (avail.Input, error) {
	sc, err := s.schedFor(ctx, t, st)
	if err != nil {
		return avail.Input{}, err
	}
	loc := loadZone(sc.TZ, zone(st))
	in := avail.Input{Loc: loc, Hours: map[string][]avail.Span{}, Overrides: map[string][]avail.Span{},
		Minutes: minutes, Step: t.StepMin, BufBefore: t.BufferBefore, BufAfter: t.BufferAfter,
		Earliest: now.Add(time.Duration(t.NoticeMin) * time.Minute), MaxPerDay: t.MaxPerDay, Capacity: t.Capacity}
	if t.DaysAhead > 0 {
		end := now.In(loc).AddDate(0, 0, t.DaysAhead)
		in.Latest = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, loc)
	}
	for k, v := range sc.Hours {
		in.Hours[k] = toAvail(v)
	}
	for _, o := range sc.Overrides {
		in.Overrides[o.Day] = toAvail(o.Hours)
	}
	return in, nil
}

// taken fills in what is taken from the person's own bookings.
func taken(in *avail.Input, t *store.Type, bs []*store.Booking) {
	in.PerDay = map[string]int{}
	for _, b := range bs {
		if b.TypeID == t.ID {
			in.PerDay[b.Start().In(in.Loc).Format("2006-01-02")]++
			if t.Capacity > 1 {
				in.Seats = append(in.Seats, avail.Seat{Start: b.Start(), Count: 1})
				continue
			}
		}
		in.Busy = append(in.Busy, avail.Interval{Start: b.Start(), End: b.End()})
	}
}

// slotsFor works out a page's free times for some days (the public page,
// the owner's "Book someone in", other apps).
func (s *Server) slotsFor(ctx context.Context, t *store.Type, minutes int, fromDay string, days int) ([]avail.Day, *time.Location, string, error) {
	st, _ := s.store.GetSettings(ctx, t.UserID)
	now := time.Now()
	in, err := s.baseInput(ctx, t, st, minutes, now)
	if err != nil {
		return nil, nil, "", err
	}
	first, err := time.ParseInLocation("2006-01-02", fromDay, in.Loc)
	if err != nil {
		first = now.In(in.Loc)
	}
	today := now.In(in.Loc).Format("2006-01-02")
	if first.Format("2006-01-02") < today {
		first, _ = time.ParseInLocation("2006-01-02", today, in.Loc)
	}
	in.FromDay, in.Days = first.Format("2006-01-02"), days
	from := first.Add(-24 * time.Hour)
	to := first.AddDate(0, 0, days+1)
	bs, err := s.store.Busy(ctx, t.UserID, from, to)
	if err != nil {
		return nil, nil, "", err
	}
	taken(&in, t, bs)
	warn := ""
	if st.CheckCalendar {
		cal, err := s.calendarBusy(ctx, t.UserID, st, from, to, true)
		if err != nil {
			warn = err.Error()
		}
		in.Busy = append(in.Busy, cal...)
	}
	return avail.Compute(in), in.Loc, warn, nil
}

// stillFree checks one time inside the booking transaction: the person's
// bookings as they are right now (leaving out one being moved), and the
// calendar busy times fetched just before. in is built before the
// transaction opens (the database has one connection: nothing else may be
// read while it is open).
func stillFree(ctx context.Context, tx *sql.Tx, t *store.Type, in avail.Input, start time.Time, skip int64, cal []avail.Interval) error {
	day := start.In(in.Loc)
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, in.Loc).Add(-24 * time.Hour)
	bs, err := store.HeldBetween(ctx, tx, t.UserID, from, from.Add(72*time.Hour), skip)
	if err != nil {
		return err
	}
	taken(&in, t, bs)
	in.Busy = append(in.Busy, cal...)
	if _, ok := avail.Free(in, start); !ok {
		return store.ErrTaken
	}
	return nil
}

// ---- Pocket Calendar busy times ----

type busyCall struct {
	done chan struct{}
	ivs  []avail.Interval
	err  error
}

// calendarBusy returns the times a person is busy in Pocket Calendar
// between from and to, leaving out the events Booking made itself. With
// cached set, an answer up to two minutes old is used (the public pages);
// a booking itself always asks afresh.
func (s *Server) calendarBusy(ctx context.Context, uid int64, st store.Settings, from, to time.Time, cached bool) ([]avail.Interval, error) {
	key := fmt.Sprintf("busy:%d:%s:%s", uid, from.UTC().Format("20060102T15"), to.UTC().Format("20060102T15"))
	if cached {
		if v, at, ok := s.store.CacheGet(ctx, key); ok {
			if t, err := time.Parse(time.RFC3339Nano, at); err == nil && time.Since(t) < 2*time.Minute {
				var ivs []avail.Interval
				if json.Unmarshal([]byte(v), &ivs) == nil {
					return ivs, nil
				}
			}
		}
		// one fetch at a time for the same range
		s.busyMu.Lock()
		if c, ok := s.busyFetch[key]; ok {
			s.busyMu.Unlock()
			<-c.done
			return c.ivs, c.err
		}
		c := &busyCall{done: make(chan struct{})}
		s.busyFetch[key] = c
		s.busyMu.Unlock()
		c.ivs, c.err = s.fetchBusy(ctx, uid, st, from, to)
		if c.err == nil {
			b, _ := json.Marshal(c.ivs)
			_ = s.store.CachePut(ctx, key, string(b))
		}
		s.busyMu.Lock()
		delete(s.busyFetch, key)
		s.busyMu.Unlock()
		close(c.done)
		return c.ivs, c.err
	}
	return s.fetchBusy(ctx, uid, st, from, to)
}

var calendarOff sync.Map // uid → when Calendar was last found not connected (quiet for a while)

func (s *Server) fetchBusy(ctx context.Context, uid int64, st store.Settings, from, to time.Time) ([]avail.Interval, error) {
	if v, ok := calendarOff.Load(uid); ok && time.Since(v.(time.Time)) < 5*time.Minute {
		return nil, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c := s.clients(cctx, uid)
	sib, err := c.conn(cctx, appCalendar, scopeCalendarRead)
	if err != nil {
		if isNotConnected(err) {
			calendarOff.Store(uid, time.Now())
			return nil, nil
		}
		return nil, err
	}
	q := url.Values{"from": {from.UTC().Format(time.RFC3339)}, "to": {to.UTC().Format(time.RFC3339)}, "invites": {"1"}}
	if len(st.BusyCalendars) > 0 {
		var ids []string
		for _, id := range st.BusyCalendars {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		q.Set("calendars", strings.Join(ids, ","))
	}
	v, err := getJSONFrom(cctx, sib, "/api/v1/events", q)
	if err != nil {
		return nil, fmt.Errorf("Pocket Calendar: %w", err)
	}
	mine := s.madeEventRefs(ctx, uid)
	var out []avail.Interval
	for _, e := range rows(v, "events") {
		if busy, ok := e["busy"].(bool); ok && !busy {
			continue
		}
		if st := str(e, "status"); strings.EqualFold(st, "cancelled") {
			continue
		}
		if str(e, "source") == "booking" || mine[str(e, "event_id")] || mine[str(e, "id")] {
			continue
		}
		start, ok1 := calTime(str(e, "start"))
		end, ok2 := calTime(str(e, "end"))
		if !ok1 || !ok2 {
			continue
		}
		if allDay, _ := e["all_day"].(bool); allDay {
			loc := zone(st)
			s0, _ := time.ParseInLocation("2006-01-02", str(e, "start")[:min(10, len(str(e, "start")))], loc)
			e0, err := time.ParseInLocation("2006-01-02", str(e, "end")[:min(10, len(str(e, "end")))], loc)
			if err != nil || !e0.After(s0) {
				e0 = s0.AddDate(0, 0, 1)
			}
			start, end = s0, e0
		}
		out = append(out, avail.Interval{Start: start, End: end})
	}
	return out, nil
}

func calTime(v string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", v, time.Local); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// madeEventRefs are the calendar events Booking made (so a booking never blocks itself).
func (s *Server) madeEventRefs(ctx context.Context, uid int64) map[string]bool {
	out := map[string]bool{}
	rows, err := s.store.DB.QueryContext(ctx, `SELECT ref FROM made WHERE user_id = ? AND target = 'event' AND ref <> ''`, uid)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var ref string
		if rows.Scan(&ref) == nil {
			out[ref] = true
		}
	}
	return out
}
