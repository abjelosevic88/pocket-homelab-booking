// Package avail works out the free times of a booking page: the weekly
// hours (or the hours of a day that differs) cut into times of the
// booking's length, less what is already taken, the notice and the limits.
package avail

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// Span is a stretch of a day, "HH:MM" to "HH:MM" ("24:00" = midnight after).
type Span [2]string

// Interval is a time that is taken.
type Interval struct {
	Start, End time.Time
}

// Seat is a booking of the same page at a time that can take several
// people (a class, a tour): it takes one place rather than the time.
type Seat struct {
	Start time.Time
	Count int
}

// Input is everything one page's times depend on.
type Input struct {
	Loc       *time.Location
	Hours     map[string][]Span // by weekday, "0" = Sunday
	Overrides map[string][]Span // by day YYYY-MM-DD; present (even empty) = this day instead
	FromDay   string            // first day to look at, YYYY-MM-DD in Loc
	Days      int               // how many days

	Minutes   int // the booking's length
	Step      int // a time starts every … minutes (0 = Minutes)
	BufBefore int // free minutes needed before
	BufAfter  int // and after

	Earliest time.Time // not sooner (now + notice)
	Latest   time.Time // not later (now + days ahead); zero = no limit

	MaxPerDay int            // 0 = no limit
	PerDay    map[string]int // this page's bookings per day (in Loc)

	Capacity int    // people per time; 1 = the time is taken by one booking
	Seats    []Seat // this page's bookings when Capacity > 1
	Busy     []Interval
}

// Slot is one free time.
type Slot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Seats int       `json:"seats"` // places left
}

// Day is one day's free times.
type Day struct {
	Date  string `json:"date"`
	Slots []Slot `json:"slots"`
}

// Clock reads "HH:MM" as minutes after midnight (-1 when it is not a time).
func Clock(v string) int {
	h, m, ok := strings.Cut(strings.TrimSpace(v), ":")
	if !ok || len(m) != 2 || len(h) == 0 || len(h) > 2 {
		return -1
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || mm < 0 || mm > 59 || hh > 24 || (hh == 24 && mm > 0) {
		return -1
	}
	return hh*60 + mm
}

// SpansFor returns a day's hours: the day's own, else its weekday's.
func (in *Input) SpansFor(day time.Time) []Span {
	if sp, ok := in.Overrides[day.Format("2006-01-02")]; ok {
		return sp
	}
	return in.Hours[strconv.Itoa(int(day.Weekday()))]
}

// At is minute m of a day on the wall clock in loc (so a day that changes
// to or from summer time still starts its hours at the right time).
func At(day time.Time, m int, loc *time.Location) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), m/60, m%60, 0, 0, loc)
}

// Compute returns the free times, day by day (days without any are left out).
func Compute(in Input) []Day {
	loc := in.Loc
	if loc == nil {
		loc = time.Local
	}
	if in.Minutes <= 0 {
		return nil
	}
	step := in.Step
	if step <= 0 {
		step = in.Minutes
	}
	capacity := max(in.Capacity, 1)
	first, err := time.ParseInLocation("2006-01-02", in.FromDay, loc)
	if err != nil {
		return nil
	}
	busy := append([]Interval(nil), in.Busy...)
	sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })
	seats := map[int64]int{}
	for _, s := range in.Seats {
		seats[s.Start.Unix()] += max(s.Count, 1)
	}
	var out []Day
	for i := 0; i < in.Days; i++ {
		day := time.Date(first.Year(), first.Month(), first.Day()+i, 0, 0, 0, 0, loc)
		date := day.Format("2006-01-02")
		if !in.Latest.IsZero() && At(day, 0, loc).After(in.Latest) {
			break
		}
		left := -1
		if in.MaxPerDay > 0 {
			left = in.MaxPerDay - in.PerDay[date]
			if left <= 0 {
				continue
			}
		}
		var slots []Slot
		for _, sp := range in.SpansFor(day) {
			a, b := Clock(sp[0]), Clock(sp[1])
			if a < 0 || b <= a {
				continue
			}
			for m := a; m+in.Minutes <= b; m += step {
				start := At(day, m, loc)
				end := start.Add(time.Duration(in.Minutes) * time.Minute)
				if start.Before(in.Earliest) || (!in.Latest.IsZero() && start.After(in.Latest)) {
					continue
				}
				taken := seats[start.Unix()]
				if taken >= capacity {
					continue
				}
				from := start.Add(-time.Duration(in.BufBefore) * time.Minute)
				to := end.Add(time.Duration(in.BufAfter) * time.Minute)
				if overlaps(busy, from, to) {
					continue
				}
				if capacity > 1 && taken == 0 && overlapsSeats(in.Seats, start, end, in.Minutes) {
					// another time of this same page overlaps: one group at a time
					continue
				}
				slots = append(slots, Slot{Start: start, End: end, Seats: capacity - taken})
			}
		}
		if len(slots) == 0 {
			continue
		}
		sort.Slice(slots, func(i, j int) bool { return slots[i].Start.Before(slots[j].Start) })
		slots = dedupe(slots)
		out = append(out, Day{Date: date, Slots: slots})
	}
	return out
}

func overlaps(busy []Interval, from, to time.Time) bool {
	for _, b := range busy {
		if !b.Start.Before(to) {
			break
		}
		if b.End.After(from) {
			return true
		}
	}
	return false
}

// overlapsSeats: a group already booked at another start that runs into this time.
func overlapsSeats(seats []Seat, start, end time.Time, minutes int) bool {
	for _, s := range seats {
		sEnd := s.Start.Add(time.Duration(minutes) * time.Minute)
		if !s.Start.Equal(start) && s.Start.Before(end) && sEnd.After(start) {
			return true
		}
	}
	return false
}

func dedupe(slots []Slot) []Slot {
	out := slots[:0]
	for i, s := range slots {
		if i > 0 && s.Start.Equal(slots[i-1].Start) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// Free reports whether one exact time is free (used when booking, inside the
// transaction, with what is taken right then).
func Free(in Input, start time.Time) (Slot, bool) {
	in.FromDay = start.In(in.Loc).Format("2006-01-02")
	in.Days = 1
	for _, d := range Compute(in) {
		for _, s := range d.Slots {
			if s.Start.Equal(start) {
				return s, true
			}
		}
	}
	return Slot{}, false
}
