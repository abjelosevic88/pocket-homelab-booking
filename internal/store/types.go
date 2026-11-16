package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

const typeCols = `id, user_id, handle, title, description, emoji, status, durations, location_kind, location, place_id,
	COALESCE(schedule_id, 0), notice_min, days_ahead, buffer_before, buffer_after, step_min, max_per_day, capacity, confirm, ask_phone,
	questions, price, currency, pay_note, reminder_h, language, accent, invite_only, after, thanks, source, position, created_at, updated_at`

func scanType(row interface{ Scan(...any) error }) (*Type, error) {
	var t Type
	var durations, questions, after string
	var inviteOnly int
	if err := row.Scan(&t.ID, &t.UserID, &t.Handle, &t.Title, &t.Description, &t.Emoji, &t.Status, &durations, &t.LocationKind, &t.Location, &t.PlaceID,
		&t.ScheduleID, &t.NoticeMin, &t.DaysAhead, &t.BufferBefore, &t.BufferAfter, &t.StepMin, &t.MaxPerDay, &t.Capacity, &t.Confirm, &t.AskPhone,
		&questions, &t.Price, &t.Currency, &t.PayNote, &t.ReminderH, &t.Language, &t.Accent, &inviteOnly, &after, &t.Thanks, &t.Source, &t.Position,
		&t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	t.InviteOnly = inviteOnly == 1
	_ = json.Unmarshal([]byte(durations), &t.Durations)
	_ = json.Unmarshal([]byte(questions), &t.Questions)
	_ = json.Unmarshal([]byte(after), &t.After)
	if len(t.Durations) == 0 {
		t.Durations = []int{30}
	}
	if t.Questions == nil {
		t.Questions = []Question{}
	}
	if t.Capacity < 1 {
		t.Capacity = 1
	}
	return &t, nil
}

// ListTypes returns a person's booking pages (in their order) with counts.
func (s *Store) ListTypes(ctx context.Context, uid int64) ([]*Type, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+typeCols+` FROM types WHERE user_id = ? ORDER BY position, id`, uid)
	if err != nil {
		return nil, err
	}
	var out []*Type
	for rows.Next() {
		t, err := scanType(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	now := Stamp(time.Now())
	for _, t := range out {
		s.countType(ctx, t, now)
	}
	return out, nil
}

func (s *Store) countType(ctx context.Context, t *Type, now string) {
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1), COALESCE(SUM(CASE WHEN start_at >= ? AND status IN ('pending','confirmed') THEN 1 ELSE 0 END), 0)
		FROM bookings WHERE type_id = ?`, now, t.ID).Scan(&t.Bookings, &t.Upcoming)
	_ = s.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(count), 0) FROM views WHERE type_id = ?`, t.ID).Scan(&t.Views)
}

// GetType returns one of a person's booking pages.
func (s *Store) GetType(ctx context.Context, uid, id int64) (*Type, error) {
	t, err := scanType(s.DB.QueryRowContext(ctx, `SELECT `+typeCols+` FROM types WHERE id = ? AND user_id = ?`, id, uid))
	if err != nil {
		return nil, notFound(err)
	}
	s.countType(ctx, t, Stamp(time.Now()))
	return t, nil
}

// TypeByHandle finds a page by its public address (any owner).
func (s *Store) TypeByHandle(ctx context.Context, handle string) (*Type, error) {
	t, err := scanType(s.DB.QueryRowContext(ctx, `SELECT `+typeCols+` FROM types WHERE handle = ?`, strings.ToLower(handle)))
	if err != nil {
		return nil, notFound(err)
	}
	return t, nil
}

// TypeAny returns a page by id whoever owns it (public pages, background work).
func (s *Store) TypeAny(ctx context.Context, id int64) (*Type, error) {
	t, err := scanType(s.DB.QueryRowContext(ctx, `SELECT `+typeCols+` FROM types WHERE id = ?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return t, nil
}

// HandleTaken reports whether a page address is in use (by another page than skip).
func (s *Store) HandleTaken(ctx context.Context, handle string, skip int64) bool {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM types WHERE handle = ? AND id <> ?`, strings.ToLower(handle), skip).Scan(&n)
	return n > 0
}

func typeArgs(t *Type) []any {
	if len(t.Durations) == 0 {
		t.Durations = []int{30}
	}
	if t.Questions == nil {
		t.Questions = []Question{}
	}
	var sched any
	if t.ScheduleID > 0 {
		sched = t.ScheduleID
	}
	return []any{strings.ToLower(t.Handle), t.Title, t.Description, t.Emoji, t.Status, toJSON(t.Durations), t.LocationKind, t.Location, t.PlaceID,
		sched, t.NoticeMin, t.DaysAhead, t.BufferBefore, t.BufferAfter, t.StepMin, t.MaxPerDay, t.Capacity, t.Confirm, t.AskPhone,
		toJSON(t.Questions), t.Price, t.Currency, t.PayNote, t.ReminderH, t.Language, t.Accent, boolInt(t.InviteOnly), toJSON(t.After), t.Thanks}
}

// CreateType stores a new booking page (at the end of the list).
func (s *Store) CreateType(ctx context.Context, uid int64, t *Type) error {
	args := append([]any{uid}, typeArgs(t)...)
	args = append(args, t.Source, uid)
	res, err := s.DB.ExecContext(ctx, `INSERT INTO types (user_id, handle, title, description, emoji, status, durations, location_kind, location, place_id,
		schedule_id, notice_min, days_ahead, buffer_before, buffer_after, step_min, max_per_day, capacity, confirm, ask_phone,
		questions, price, currency, pay_note, reminder_h, language, accent, invite_only, after, thanks, source, position)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		(SELECT COALESCE(MAX(position), 0) + 1 FROM types WHERE user_id = ?))`, args...)
	if err != nil {
		return conflict(err)
	}
	t.ID, _ = res.LastInsertId()
	t.UserID = uid
	return nil
}

// UpdateType saves a booking page.
func (s *Store) UpdateType(ctx context.Context, t *Type) error {
	args := append(typeArgs(t), Now(), t.ID, t.UserID)
	res, err := s.DB.ExecContext(ctx, `UPDATE types SET handle = ?, title = ?, description = ?, emoji = ?, status = ?, durations = ?, location_kind = ?,
		location = ?, place_id = ?, schedule_id = ?, notice_min = ?, days_ahead = ?, buffer_before = ?, buffer_after = ?, step_min = ?, max_per_day = ?,
		capacity = ?, confirm = ?, ask_phone = ?, questions = ?, price = ?, currency = ?, pay_note = ?, reminder_h = ?, language = ?, accent = ?,
		invite_only = ?, after = ?, thanks = ?, updated_at = ? WHERE id = ? AND user_id = ?`, args...)
	if err != nil {
		return conflict(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MoveType puts a page at a new place in the person's list.
func (s *Store) MoveType(ctx context.Context, uid int64, ids []int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE types SET position = ? WHERE id = ? AND user_id = ?`, i+1, id, uid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteType removes a page; its bookings stay (without the page).
func (s *Store) DeleteType(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM types WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddView counts one visit to a page.
func (s *Store) AddView(ctx context.Context, uid, typeID int64) {
	_, _ = s.DB.ExecContext(ctx, `INSERT INTO views (user_id, type_id, day, count) VALUES (?, ?, ?, 1)
		ON CONFLICT(type_id, day) DO UPDATE SET count = count + 1`, uid, typeID, Today())
}

// ViewsSince counts visits per page since a day.
func (s *Store) ViewsSince(ctx context.Context, uid int64, day string) map[int64]int {
	out := map[int64]int{}
	rows, err := s.DB.QueryContext(ctx, `SELECT type_id, SUM(count) FROM views WHERE user_id = ? AND day >= ? GROUP BY type_id`, uid, day)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n int
		if rows.Scan(&id, &n) == nil {
			out[id] = n
		}
	}
	return out
}

// PageOwner finds whose public page a handle is (the page_handle setting).
func (s *Store) PageOwner(ctx context.Context, handle string) (int64, error) {
	var uid int64
	err := s.DB.QueryRowContext(ctx, `SELECT user_id FROM user_settings WHERE key = 'page_handle' AND value = ?`, toJSON(strings.ToLower(handle))).Scan(&uid)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return uid, err
}

// PageHandleTaken reports whether someone else uses that page address.
func (s *Store) PageHandleTaken(ctx context.Context, handle string, uid int64) bool {
	owner, err := s.PageOwner(ctx, handle)
	return err == nil && owner != uid
}
