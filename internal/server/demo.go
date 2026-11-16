package server

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- demo data (DEMO_DATA=true) ----
//
// A designer who takes intro calls, paid consultations, studio visits,
// website reviews (confirmed by hand) and an evening group workshop: three
// months behind, a month ahead — enough to try everything and for the
// screenshots.

type demoPerson struct{ name, email, phone string }

var demoPeople = []demoPerson{
	{"Ana Marić", "ana@pekara-maric.ba", "+387 61 224 118"}, {"Emir Hodžić", "emir.hodzic@gmail.com", "+387 62 410 553"},
	{"Lejla Begić", "lejla@studioatlas.ba", ""}, {"Marko Jurić", "marko.juric@outlook.com", "+385 91 552 7310"},
	{"Sara Kovač", "sara@kovac-law.hr", ""}, {"Daniel Weber", "d.weber@bergholz.de", "+49 171 2203 991"},
	{"Amra Delić", "amra.delic@yahoo.com", "+387 65 118 042"}, {"Nikola Petrović", "nikola@vinarija-petrovic.rs", ""},
	{"Maja Novak", "maja.novak@gmail.com", "+386 41 380 211"}, {"Tarik Mujić", "tarik@mujic-dent.ba", "+387 61 930 774"},
	{"Ivana Babić", "ivana.babic@icloud.com", ""}, {"Jonas Lindqvist", "jonas@nordtrail.se", ""},
	{"Selma Hadžić", "selma.hadzic@gmail.com", "+387 62 077 315"}, {"Petra Horvat", "petra@horvat-design.hr", ""},
	{"Haris Omerović", "haris.omerovic@gmail.com", "+387 60 314 888"}, {"Mia Schneider", "mia.schneider@posteo.de", ""},
	{"Adnan Kurtović", "adnan@kurtovic-auto.ba", "+387 61 700 129"}, {"Lana Perić", "lana.peric@gmail.com", ""},
	{"Edin Sarić", "edin.saric@hotmail.com", "+387 63 222 901"}, {"Nina Vuković", "nina@vukovic-events.ba", ""},
}

var demoTopics = []string{
	"A new website for our bakery, with cake orders online.", "Our logo looks dated; we'd like a fresh one before spring.",
	"An online shop for handmade jewellery — where do I start?", "We need a booking site for four rooms by the river.",
	"Menus and signs for a new restaurant in Mostar.", "A landing page for our app launch in March.",
	"Help choosing colours and fonts for our law office.", "Packaging for a small wine label (3 wines).",
	"Redesign of a dental clinic website, it is slow on phones.", "A brochure for a hiking tour company.",
}

func (s *Server) seedDemo(ctx context.Context, uid int64) error {
	ctx = context.WithoutCancel(ctx)
	if n := s.store.CountBookings(ctx, uid, store.BookingFilter{}); n > 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(42))
	st, _ := s.store.GetSettings(ctx, uid)
	st.PageHandle, st.PageName = "aleksandar", "Aleksandar Bjelošević"
	st.PageIntro = "Designer in Sarajevo. Pick what you need and a time that suits you — you'll get a confirmation by email."
	st.TZ = "Europe/Sarajevo"
	if err := s.store.SaveUserSettings(ctx, uid, st); err != nil {
		return err
	}
	loc := zone(st)

	work, err := s.store.DefaultSchedule(ctx, uid, loc.String())
	if err != nil {
		return err
	}
	now := time.Now().In(loc)
	day := func(n int) string { return now.AddDate(0, 0, n).Format("2006-01-02") }
	work.Hours = map[string][]store.Span{}
	for _, d := range []string{"1", "2", "3", "4"} {
		work.Hours[d] = []store.Span{{"09:00", "12:30"}, {"13:30", "17:00"}}
	}
	work.Hours["5"] = []store.Span{{"09:00", "13:00"}}
	work.Overrides = []store.Override{{Day: day(9), Hours: []store.Span{}, Note: "Dentist and errands"},
		{Day: day(16), Hours: []store.Span{{"12:00", "18:00"}}, Note: "Late start"}}
	if err := s.store.SaveSchedule(ctx, uid, work); err != nil {
		return err
	}
	evenings := &store.Schedule{Name: "Evening workshops", TZ: loc.String(), Hours: map[string][]store.Span{"2": {{"18:00", "20:00"}}, "4": {{"18:00", "20:00"}}},
		Overrides: []store.Override{}}
	if err := s.store.SaveSchedule(ctx, uid, evenings); err != nil {
		return err
	}

	mk := func(t *store.Type) *store.Type {
		base := s.newType(ctx, uid, st, "")
		t.Handle = s.newHandle(ctx, t.Title)
		if t.ScheduleID == 0 {
			t.ScheduleID = work.ID
		}
		if t.Status == "" {
			t.Status = store.TypeOpen
		}
		if t.Capacity == 0 {
			t.Capacity = 1
		}
		t.Confirm = firstNonEmpty(t.Confirm, "auto")
		t.AskPhone = firstNonEmpty(t.AskPhone, "no")
		t.Currency = firstNonEmpty(t.Currency, "EUR")
		t.Language = firstNonEmpty(t.Language, "en")
		if t.NoticeMin == 0 {
			t.NoticeMin = base.NoticeMin
		}
		if t.DaysAhead == 0 {
			t.DaysAhead = 45
		}
		if t.ReminderH == 0 {
			t.ReminderH = 24
		}
		if t.Questions == nil {
			t.Questions = []store.Question{}
		}
		_ = s.store.CreateType(ctx, uid, t)
		return t
	}
	intro := mk(&store.Type{Title: "Intro call", Emoji: "📞", Durations: []int{30}, LocationKind: "video",
		Description: "A free 30-minute video call to get to know each other and see whether I can help with your project.",
		Questions:   []store.Question{{ID: "q1", Type: "long", Label: "What would you like to talk about?", Required: true}},
		After:       store.After{Event: true, Contact: true, Deal: true}})
	consult := mk(&store.Type{Title: "Design consultation", Emoji: "💼", Durations: []int{60, 90}, LocationKind: "video", BufferAfter: 15,
		Price: 8000, PayNote: "Bank transfer after the session — the details come with the invoice.",
		Description: "A full session on your brand, website or packaging. We look at what you have and leave with a plan.",
		Questions: []store.Question{{ID: "q1", Type: "choice", Label: "What is it about?", Required: true, Options: []string{"Brand & logo", "Website", "Packaging", "Something else"}},
			{ID: "q2", Type: "url", Label: "Your website, if you have one"}},
		After: store.After{Event: true, Contact: true, Invoice: true, Note: true}})
	visit := mk(&store.Type{Title: "Studio visit", Emoji: "🏠", Durations: []int{45}, LocationKind: "place", BufferBefore: 15, BufferAfter: 15,
		Location: "Ferhadija 12, 71000 Sarajevo (2nd floor)", AskPhone: "required", Language: "bs",
		Description: "Come by the studio to see samples, paper and print finishes in person.",
		Questions:   []store.Question{{ID: "q1", Type: "number", Label: "Koliko vas dolazi?"}},
		After:       store.After{Event: true, Contact: true}})
	review := mk(&store.Type{Title: "Website review", Emoji: "🔍", Durations: []int{45}, LocationKind: "video", Confirm: "manual",
		Description: "I go through your website with you and write down what to fix first. I confirm each request myself.",
		Questions:   []store.Question{{ID: "q1", Type: "url", Label: "Your website", Required: true}, {ID: "q2", Type: "yesno", Label: "Do you sell online?"}},
		After:       store.After{Event: true}})
	workshop := mk(&store.Type{Title: "Evening workshop: logo basics", Emoji: "✏️", Durations: []int{120}, LocationKind: "place", Capacity: 6,
		ScheduleID: evenings.ID, Location: "Ferhadija 12, 71000 Sarajevo", Price: 2500, PayNote: "Pay in cash on the evening.",
		Description: "A small hands-on evening class: from a sketch to a simple logo. Paper and coffee are on me.",
		After:       store.After{Event: true}})
	mk(&store.Type{Title: "Mentoring", Emoji: "🌱", Durations: []int{60}, LocationKind: "video", Status: store.TypePaused,
		Description: "Monthly mentoring for junior designers. Paused until January."})

	// bookings: three months behind, a month ahead
	taken := map[string]bool{}
	type plan struct {
		t       *store.Type
		weight  int
		minutes []int
	}
	plans := []plan{{intro, 5, []int{30}}, {consult, 3, []int{60, 90}}, {visit, 2, []int{45}}, {review, 2, []int{45}}}
	pick := func() plan {
		total := 0
		for _, p := range plans {
			total += p.weight
		}
		n := rng.Intn(total)
		for _, p := range plans {
			if n < p.weight {
				return p
			}
			n -= p.weight
		}
		return plans[0]
	}
	var made []*store.Booking
	add := func(t *store.Type, start time.Time, minutes int, status string, who demoPerson) *store.Booking {
		b := &store.Booking{TypeID: t.ID, Title: t.Title, Emoji: t.Emoji, Status: status, StartAt: store.Stamp(start),
			EndAt: store.Stamp(start.Add(time.Duration(minutes) * time.Minute)), Minutes: minutes, GuestTZ: loc.String(), Name: who.name,
			Email: who.email, Phone: who.phone, Answers: []store.Answer{}, LocationKind: t.LocationKind, Price: t.Price, Currency: t.Currency,
			Source: "page", Language: t.Language, Reminded: true}
		if minutes == 90 {
			b.Price = 12000
		}
		switch t.LocationKind {
		case "video":
			b.MeetingURL = fmt.Sprintf("https://meet.jit.si/Pocket%s%06x", slugify(t.Title)[:min(8, len(slugify(t.Title)))], rng.Intn(1<<24))
		case "place":
			b.Location = t.Location
		}
		for _, q := range t.Questions {
			v := ""
			switch q.Type {
			case "long":
				v = demoTopics[rng.Intn(len(demoTopics))]
			case "choice":
				v = q.Options[rng.Intn(len(q.Options))]
			case "url":
				if rng.Intn(3) > 0 {
					v = "https://" + slugify(who.name) + ".ba"
				}
			case "number":
				v = fmt.Sprint(1 + rng.Intn(3))
			case "yesno":
				v = []string{"Yes", "No"}[rng.Intn(2)]
			}
			if v != "" {
				b.Answers = append(b.Answers, store.Answer{ID: q.ID, Label: q.Label, Value: v})
			}
		}
		if rng.Intn(6) == 0 {
			b.Source = "invite"
		}
		created := start.Add(-time.Duration(24*(1+rng.Intn(12))+rng.Intn(20)) * time.Hour)
		if created.After(time.Now()) {
			created = time.Now().Add(-time.Duration(rng.Intn(48)+1) * time.Hour)
		}
		b.CreatedAt = created.UTC().Format("2006-01-02T15:04:05.000Z")
		if status == store.StatusDone && b.Price > 0 && rng.Intn(5) > 0 {
			b.Paid, b.PaidAt = true, store.Stamp(start.Add(48*time.Hour))
		}
		if status == store.StatusCancelled {
			b.CancelledBy = []string{"guest", "guest", "owner"}[rng.Intn(3)]
			b.CancelReason = []string{"Something came up at work, sorry!", "We found someone closer, thank you.", "Ill this week.", ""}[rng.Intn(4)]
		}
		if status == store.StatusDone && rng.Intn(3) == 0 {
			b.OwnerNote = []string{"Wants a proposal by Friday.", "Budget around €3k. Send examples of bakery sites.", "Great fit — follow up in two weeks.", "Needs it before the summer season."}[rng.Intn(4)]
		}
		if err := s.store.CreateBooking(ctx, uid, b, nil); err == nil {
			made = append(made, b)
		}
		return b
	}
	for d := -90; d <= 30; d++ {
		date := now.AddDate(0, 0, d)
		wd := date.Weekday()
		if wd == time.Saturday || wd == time.Sunday || date.Format("2006-01-02") == day(9) {
			continue
		}
		n := rng.Intn(3)
		if d > 14 {
			n = rng.Intn(2)
		}
		for i := 0; i < n; i++ {
			p := pick()
			minutes := p.minutes[rng.Intn(len(p.minutes))]
			hours := []int{9, 10, 11, 14, 15}
			if wd == time.Friday {
				hours = []int{9, 10, 11}
			}
			h := hours[rng.Intn(len(hours))]
			m := []int{0, 30}[rng.Intn(2)]
			if h == 11 && m == 30 && minutes > 45 {
				m = 0
			}
			start := time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, loc)
			key := start.Format("2006-01-02T15")
			if taken[key] {
				continue
			}
			taken[key] = true
			who := demoPeople[rng.Intn(len(demoPeople))]
			status := store.StatusConfirmed
			switch {
			case start.Before(time.Now()):
				status = store.StatusDone
				switch rng.Intn(14) {
				case 0, 1:
					status = store.StatusCancelled
				case 2:
					status = store.StatusNoShow
				}
			case p.t.ID == review.ID && d < 10:
				status = store.StatusPending
			case rng.Intn(12) == 0:
				status = store.StatusCancelled
			}
			add(p.t, start, minutes, status, who)
		}
	}
	// the evening workshops: a group on some Tuesdays and Thursdays
	for d := -60; d <= 28; d++ {
		date := now.AddDate(0, 0, d)
		if (date.Weekday() != time.Tuesday && date.Weekday() != time.Thursday) || rng.Intn(3) > 0 {
			continue
		}
		start := time.Date(date.Year(), date.Month(), date.Day(), 18, 0, 0, 0, loc)
		seats := 2 + rng.Intn(5)
		if d > 0 {
			seats = 1 + rng.Intn(4)
		}
		for i := 0; i < seats; i++ {
			status := store.StatusConfirmed
			if start.Before(time.Now()) {
				status = store.StatusDone
			}
			add(workshop, start, 120, status, demoPeople[(i*7+d+60)%len(demoPeople)])
		}
	}
	// a booking for today, so the day is never empty in the screenshots
	if t := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, loc); t.After(time.Now()) && !taken[t.Format("2006-01-02T15")] {
		add(intro, t, 30, store.StatusConfirmed, demoPeople[0])
	}
	// personal links
	for i, p := range []demoPerson{demoPeople[5], demoPeople[11], demoPeople[3]} {
		iv := &store.Invite{TypeID: consult.ID, Name: p.name, Email: p.email, Note: "As we said on the phone", Source: "owner",
			ExpiresAt: store.Stamp(time.Now().AddDate(0, 0, 14-i*3))}
		_ = s.store.CreateInvite(ctx, uid, iv)
		if i == 2 && len(made) > 0 {
			for _, b := range made {
				if b.Email == p.email && b.TypeID == consult.ID {
					_, _ = s.store.DB.ExecContext(ctx, `UPDATE invites SET booking_id = ? WHERE id = ?`, b.ID, iv.ID)
					break
				}
			}
		}
	}
	// visits to the pages
	for d := -84; d <= 0; d++ {
		for _, t := range []*store.Type{intro, consult, visit, review, workshop} {
			n := rng.Intn(6)
			if t.ID == intro.ID {
				n += 3
			}
			if n > 0 {
				_, _ = s.store.DB.ExecContext(ctx, `INSERT INTO views (user_id, type_id, day, count) VALUES (?, ?, ?, ?)
					ON CONFLICT(type_id, day) DO UPDATE SET count = excluded.count`, uid, t.ID, day(d), n)
			}
		}
	}
	s.log.Info("demo data seeded", "bookings", len(made))
	return nil
}
