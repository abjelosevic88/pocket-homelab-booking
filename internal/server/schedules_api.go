package server

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/avail"
	"github.com/abjelosevic88/pocket-homelab-booking/internal/store"
)

// ---- weekly hours and the days that differ ----

// cleanSpans checks a day's hours: valid times, start before end, no overlaps.
func cleanSpans(in []store.Span) ([]store.Span, error) {
	out := []store.Span{}
	for _, sp := range in {
		a, b := avail.Clock(sp[0]), avail.Clock(sp[1])
		if a < 0 || b < 0 {
			return nil, &userErr{"times are like 09:00"}
		}
		if b <= a {
			return nil, &userErr{"each stretch ends after it starts (" + sp[0] + "–" + sp[1] + ")"}
		}
		out = append(out, store.Span{strings.TrimSpace(sp[0]), strings.TrimSpace(sp[1])})
	}
	sort.Slice(out, func(i, j int) bool { return avail.Clock(out[i][0]) < avail.Clock(out[j][0]) })
	for i := 1; i < len(out); i++ {
		if avail.Clock(out[i][0]) < avail.Clock(out[i-1][1]) {
			return nil, &userErr{"two stretches of the same day overlap"}
		}
	}
	return out, nil
}

type scheduleIn struct {
	Name      *string                  `json:"name"`
	TZ        *string                  `json:"tz"`
	Hours     *map[string][]store.Span `json:"hours"`
	IsDefault *bool                    `json:"is_default"`
	Overrides *[]store.Override        `json:"overrides"`
}

func applySchedule(sc *store.Schedule, in scheduleIn) error {
	if in.Name != nil {
		sc.Name = clip(strings.TrimSpace(*in.Name), 80)
	}
	if sc.Name == "" {
		sc.Name = "Working hours"
	}
	if in.TZ != nil {
		tz := strings.TrimSpace(*in.TZ)
		if tz != "" {
			if _, err := time.LoadLocation(tz); err != nil {
				return &userErr{"unknown time zone: " + tz}
			}
		}
		sc.TZ = tz
	}
	if in.Hours != nil {
		h := map[string][]store.Span{}
		for k, v := range *in.Hours {
			if !oneOf(k, []string{"0", "1", "2", "3", "4", "5", "6"}) {
				return &userErr{"weekdays are 0 (Sunday) to 6"}
			}
			sp, err := cleanSpans(v)
			if err != nil {
				return err
			}
			if len(sp) > 0 {
				h[k] = sp
			}
		}
		sc.Hours = h
	}
	if in.IsDefault != nil {
		sc.IsDefault = *in.IsDefault
	}
	if in.Overrides != nil {
		seen := map[string]bool{}
		out := []store.Override{}
		for _, o := range *in.Overrides {
			if o.Day == "" || !validDate(o.Day) {
				return &userErr{"a day is YYYY-MM-DD"}
			}
			if seen[o.Day] {
				continue
			}
			seen[o.Day] = true
			sp, err := cleanSpans(o.Hours)
			if err != nil {
				return err
			}
			out = append(out, store.Override{Day: o.Day, Hours: sp, Note: clip(strings.TrimSpace(o.Note), 200)})
		}
		if len(out) > 1000 {
			return &userErr{"at most 1000 days that differ"}
		}
		sc.Overrides = out
	}
	return nil
}

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, _ := s.store.GetSettings(ctx, uid)
	if _, err := s.store.DefaultSchedule(ctx, uid, zone(st).String()); err != nil {
		s.fail(w, err, "hours")
		return
	}
	list, err := s.store.ListSchedules(ctx, uid)
	if err != nil {
		s.fail(w, err, "hours")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": list, "tz": zone(st).String()})
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in scheduleIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st, _ := s.store.GetSettings(ctx, uid)
	sc := &store.Schedule{TZ: zone(st).String(), Hours: store.WorkWeek(), Overrides: []store.Override{}}
	if err := applySchedule(sc, in); err != nil {
		s.badOrFail(w, err, "hours")
		return
	}
	if err := s.store.SaveSchedule(ctx, uid, sc); err != nil {
		s.fail(w, err, "save hours")
		return
	}
	sc, _ = s.store.GetSchedule(ctx, uid, sc.ID)
	writeJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	sc, err := s.store.GetSchedule(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "hours")
		return
	}
	var in scheduleIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Overrides == nil {
		sc.Overrides = nil // keep them as they are
	}
	if err := applySchedule(sc, in); err != nil {
		s.badOrFail(w, err, "hours")
		return
	}
	if err := s.store.SaveSchedule(ctx, uid, sc); err != nil {
		s.fail(w, err, "save hours")
		return
	}
	_ = s.store.CacheDelete(ctx, "busy:")
	sc, _ = s.store.GetSchedule(ctx, uid, sc.ID)
	writeJSON(w, http.StatusOK, sc)
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	list, _ := s.store.ListSchedules(ctx, uid)
	if len(list) <= 1 {
		writeErr(w, http.StatusBadRequest, "you need at least one set of hours")
		return
	}
	if err := s.store.DeleteSchedule(ctx, uid, idParam(r, "id")); err != nil {
		s.fail(w, err, "delete hours")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
