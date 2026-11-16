package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
)

const scheduleCols = `id, user_id, name, tz, hours, is_default, created_at, updated_at`

func scanSchedule(row interface{ Scan(...any) error }) (*Schedule, error) {
	var sc Schedule
	var hours string
	var def int
	if err := row.Scan(&sc.ID, &sc.UserID, &sc.Name, &sc.TZ, &hours, &def, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
		return nil, err
	}
	sc.IsDefault = def == 1
	_ = json.Unmarshal([]byte(hours), &sc.Hours)
	if sc.Hours == nil {
		sc.Hours = map[string][]Span{}
	}
	sc.Overrides = []Override{}
	return &sc, nil
}

// ListSchedules returns a person's schedules (the default first) with
// their days that differ and how many pages use each.
func (s *Store) ListSchedules(ctx context.Context, uid int64) ([]*Schedule, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+scheduleCols+` FROM schedules WHERE user_id = ? ORDER BY is_default DESC, name, id`, uid)
	if err != nil {
		return nil, err
	}
	var out []*Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, sc)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, sc := range out {
		if err := s.fillSchedule(ctx, sc); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) fillSchedule(ctx context.Context, sc *Schedule) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT day, hours, note FROM overrides WHERE schedule_id = ? ORDER BY day`, sc.ID)
	if err != nil {
		return err
	}
	sc.Overrides = []Override{}
	for rows.Next() {
		var o Override
		var hours string
		if err := rows.Scan(&o.Day, &hours, &o.Note); err != nil {
			rows.Close()
			return err
		}
		_ = json.Unmarshal([]byte(hours), &o.Hours)
		if o.Hours == nil {
			o.Hours = []Span{}
		}
		sc.Overrides = append(sc.Overrides, o)
	}
	rows.Close()
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM types WHERE schedule_id = ?`, sc.ID).Scan(&sc.Pages)
	return rows.Err()
}

// GetSchedule returns one of a person's schedules.
func (s *Store) GetSchedule(ctx context.Context, uid, id int64) (*Schedule, error) {
	sc, err := scanSchedule(s.DB.QueryRowContext(ctx, `SELECT `+scheduleCols+` FROM schedules WHERE id = ? AND user_id = ?`, id, uid))
	if err != nil {
		return nil, notFound(err)
	}
	return sc, s.fillSchedule(ctx, sc)
}

// DefaultSchedule is the person's default schedule (made on first use).
func (s *Store) DefaultSchedule(ctx context.Context, uid int64, tz string) (*Schedule, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, `SELECT id FROM schedules WHERE user_id = ? ORDER BY is_default DESC, id LIMIT 1`, uid).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		sc := &Schedule{Name: "Working hours", TZ: tz, Hours: WorkWeek(), IsDefault: true}
		if err := s.SaveSchedule(ctx, uid, sc); err != nil {
			return nil, err
		}
		return s.GetSchedule(ctx, uid, sc.ID)
	}
	if err != nil {
		return nil, err
	}
	return s.GetSchedule(ctx, uid, id)
}

// WorkWeek is Monday to Friday, nine to five.
func WorkWeek() map[string][]Span {
	h := map[string][]Span{}
	for _, d := range []string{"1", "2", "3", "4", "5"} {
		h[d] = []Span{{"09:00", "17:00"}}
	}
	return h
}

// SaveSchedule creates (ID 0) or updates a schedule with its days that differ.
func (s *Store) SaveSchedule(ctx context.Context, uid int64, sc *Schedule) error {
	if sc.Hours == nil {
		sc.Hours = map[string][]Span{}
	}
	hours, _ := json.Marshal(sc.Hours)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if sc.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE schedules SET is_default = 0 WHERE user_id = ?`, uid); err != nil {
			return err
		}
	}
	if sc.ID == 0 {
		res, err := tx.ExecContext(ctx, `INSERT INTO schedules (user_id, name, tz, hours, is_default) VALUES (?, ?, ?, ?, ?)`,
			uid, sc.Name, sc.TZ, string(hours), boolInt(sc.IsDefault))
		if err != nil {
			return err
		}
		sc.ID, _ = res.LastInsertId()
	} else {
		res, err := tx.ExecContext(ctx, `UPDATE schedules SET name = ?, tz = ?, hours = ?, is_default = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
			sc.Name, sc.TZ, string(hours), boolInt(sc.IsDefault), Now(), sc.ID, uid)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
	}
	if sc.Overrides != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM overrides WHERE schedule_id = ?`, sc.ID); err != nil {
			return err
		}
		sort.Slice(sc.Overrides, func(i, j int) bool { return sc.Overrides[i].Day < sc.Overrides[j].Day })
		for _, o := range sc.Overrides {
			if o.Hours == nil {
				o.Hours = []Span{}
			}
			h, _ := json.Marshal(o.Hours)
			if _, err := tx.ExecContext(ctx, `INSERT INTO overrides (user_id, schedule_id, day, hours, note) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT(schedule_id, day) DO UPDATE SET hours = excluded.hours, note = excluded.note`, uid, sc.ID, o.Day, string(h), o.Note); err != nil {
				return err
			}
		}
	}
	// a person always has exactly one default
	if _, err := tx.ExecContext(ctx, `UPDATE schedules SET is_default = 1 WHERE id = (SELECT id FROM schedules WHERE user_id = ? ORDER BY is_default DESC, id LIMIT 1)
		AND NOT EXISTS (SELECT 1 FROM schedules WHERE user_id = ? AND is_default = 1)`, uid, uid); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteSchedule removes a schedule; pages on it fall back to the default.
func (s *Store) DeleteSchedule(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM schedules WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE schedules SET is_default = 1 WHERE id = (SELECT id FROM schedules WHERE user_id = ? ORDER BY id LIMIT 1)
		AND NOT EXISTS (SELECT 1 FROM schedules WHERE user_id = ? AND is_default = 1)`, uid, uid)
	return err
}
