package avail

import (
	"testing"
	"time"
)

func sarajevo(t *testing.T) *time.Location {
	loc, err := time.LoadLocation("Europe/Sarajevo")
	if err != nil {
		t.Skip("no zoneinfo")
	}
	return loc
}

func base(loc *time.Location) Input {
	return Input{
		Loc:      loc,
		Hours:    map[string][]Span{"1": {{"09:00", "12:00"}}, "2": {{"09:00", "10:00"}, {"14:00", "15:00"}}},
		FromDay:  "2026-11-16", // a Monday
		Days:     2,
		Minutes:  30,
		Earliest: time.Date(2026, 11, 1, 0, 0, 0, 0, loc),
	}
}

func starts(days []Day) map[string][]string {
	out := map[string][]string{}
	for _, d := range days {
		for _, s := range d.Slots {
			out[d.Date] = append(out[d.Date], s.Start.Format("15:04"))
		}
	}
	return out
}

func TestWeeklyHoursCutIntoTimes(t *testing.T) {
	loc := sarajevo(t)
	got := starts(Compute(base(loc)))
	if len(got["2026-11-16"]) != 6 || got["2026-11-16"][0] != "09:00" || got["2026-11-16"][5] != "11:30" {
		t.Fatalf("monday: %v", got["2026-11-16"])
	}
	if want := []string{"09:00", "09:30", "14:00", "14:30"}; len(got["2026-11-17"]) != 4 || got["2026-11-17"][2] != want[2] {
		t.Fatalf("tuesday: %v", got["2026-11-17"])
	}
}

func TestStepBusyBuffersNotice(t *testing.T) {
	loc := sarajevo(t)
	in := base(loc)
	in.Days = 1
	in.Step = 15
	in.BufAfter = 15
	in.Busy = []Interval{{time.Date(2026, 11, 16, 10, 0, 0, 0, loc), time.Date(2026, 11, 16, 10, 30, 0, 0, loc)}}
	in.Earliest = time.Date(2026, 11, 16, 9, 10, 0, 0, loc)
	got := starts(Compute(in))["2026-11-16"]
	// 09:15 (ends 09:45 + 15 buffer = 10:00, fine), 09:30 would run into 10:00 with its buffer
	want := []string{"09:15", "10:30", "10:45", "11:00", "11:15", "11:30"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestDayOffAndLongerDay(t *testing.T) {
	loc := sarajevo(t)
	in := base(loc)
	in.Overrides = map[string][]Span{"2026-11-16": {}, "2026-11-17": {{"18:00", "19:00"}}}
	got := starts(Compute(in))
	if len(got["2026-11-16"]) != 0 {
		t.Fatalf("day off still has times: %v", got["2026-11-16"])
	}
	if len(got["2026-11-17"]) != 2 || got["2026-11-17"][0] != "18:00" {
		t.Fatalf("other hours: %v", got["2026-11-17"])
	}
}

func TestMaxPerDayAndLatest(t *testing.T) {
	loc := sarajevo(t)
	in := base(loc)
	in.MaxPerDay = 2
	in.PerDay = map[string]int{"2026-11-16": 2}
	got := starts(Compute(in))
	if len(got["2026-11-16"]) != 0 || len(got["2026-11-17"]) == 0 {
		t.Fatalf("limit per day: %v", got)
	}
	in = base(loc)
	in.Latest = time.Date(2026, 11, 16, 23, 0, 0, 0, loc)
	got = starts(Compute(in))
	if len(got["2026-11-17"]) != 0 {
		t.Fatalf("beyond the last day: %v", got)
	}
}

func TestGroupSeats(t *testing.T) {
	loc := sarajevo(t)
	in := base(loc)
	in.Days = 1
	in.Minutes = 60
	in.Capacity = 3
	at10 := time.Date(2026, 11, 16, 10, 0, 0, 0, loc)
	in.Seats = []Seat{{Start: at10, Count: 2}}
	days := Compute(in)
	var seats10 int
	for _, s := range days[0].Slots {
		if s.Start.Equal(at10) {
			seats10 = s.Seats
		}
	}
	if seats10 != 1 {
		t.Fatalf("10:00 should have one place left, has %d (%v)", seats10, starts(days))
	}
	in.Seats = []Seat{{Start: at10, Count: 3}}
	if _, ok := Free(in, at10); ok {
		t.Fatal("a full group time is still free")
	}
}

func TestSummerTimeChange(t *testing.T) {
	loc := sarajevo(t)
	in := Input{Loc: loc, Hours: map[string][]Span{"0": {{"09:00", "10:00"}}}, FromDay: "2026-03-29", Days: 1, Minutes: 60,
		Earliest: time.Date(2026, 3, 1, 0, 0, 0, 0, loc)}
	days := Compute(in)
	if len(days) != 1 || days[0].Slots[0].Start.In(loc).Format("15:04") != "09:00" {
		t.Fatalf("the day clocks go forward: %+v", days)
	}
}

func TestClock(t *testing.T) {
	for v, want := range map[string]int{"09:00": 540, "24:00": 1440, "9:30": 570, "25:00": -1, "10:60": -1, "x": -1, "10:5": -1} {
		if got := Clock(v); got != want {
			t.Errorf("Clock(%q) = %d, want %d", v, got, want)
		}
	}
}
