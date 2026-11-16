package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const bookingCols = `id, user_id, COALESCE(type_id, 0), title, emoji, status, start_at, end_at, minutes, guest_tz, name, email, phone, answers,
	location_kind, location, meeting_url, price, currency, paid, paid_at, token, source, COALESCE(invite_id, 0), cancel_reason, cancelled_by,
	moved_from, owner_note, language, reminded, ip_hash, client_key, created_at, updated_at`

func scanBooking(row interface{ Scan(...any) error }) (*Booking, error) {
	var b Booking
	var answers string
	var paid, reminded int
	if err := row.Scan(&b.ID, &b.UserID, &b.TypeID, &b.Title, &b.Emoji, &b.Status, &b.StartAt, &b.EndAt, &b.Minutes, &b.GuestTZ, &b.Name, &b.Email,
		&b.Phone, &answers, &b.LocationKind, &b.Location, &b.MeetingURL, &b.Price, &b.Currency, &paid, &b.PaidAt, &b.Token, &b.Source, &b.InviteID,
		&b.CancelReason, &b.CancelledBy, &b.MovedFrom, &b.OwnerNote, &b.Language, &reminded, &b.IPHash, &b.ClientKey, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, err
	}
	b.Paid, b.Reminded = paid == 1, reminded == 1
	_ = json.Unmarshal([]byte(answers), &b.Answers)
	if b.Answers == nil {
		b.Answers = []Answer{}
	}
	return &b, nil
}

// Start and End are the booking's times.
func (b *Booking) Start() time.Time { return ParseStamp(b.StartAt) }
func (b *Booking) End() time.Time   { return ParseStamp(b.EndAt) }

// BookingFilter narrows a list of bookings.
type BookingFilter struct {
	View   string // upcoming | pending | past | cancelled | all
	From   string // UTC stamps: starts at or after
	To     string // starts before
	Query  string
	Email  string
	TypeID int64
	Made   [2]string // created between (UTC stamps)
	Limit  int
	Offset int
	Newest bool // latest first
}

func (f BookingFilter) where(uid int64) (string, []any) {
	w := []string{"user_id = ?"}
	args := []any{uid}
	now := Stamp(time.Now())
	switch f.View {
	case "upcoming":
		w = append(w, "end_at >= ? AND status IN ('pending','confirmed')")
		args = append(args, now)
	case "pending":
		w = append(w, "status = 'pending'")
	case "past":
		w = append(w, "end_at < ? AND status NOT IN ('cancelled','declined')")
		args = append(args, now)
	case "cancelled":
		w = append(w, "status IN ('cancelled','declined')")
	case "active":
		w = append(w, "status NOT IN ('cancelled','declined')")
	}
	if f.From != "" {
		w = append(w, "start_at >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		w = append(w, "start_at < ?")
		args = append(args, f.To)
	}
	if f.Made[0] != "" {
		w = append(w, "created_at >= ? AND created_at < ?")
		args = append(args, f.Made[0], f.Made[1])
	}
	if f.Email != "" {
		w = append(w, "lower(email) = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(f.Email)))
	}
	if f.TypeID > 0 {
		w = append(w, "type_id = ?")
		args = append(args, f.TypeID)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		w = append(w, "(lower(name) LIKE ? OR lower(email) LIKE ? OR lower(title) LIKE ? OR lower(answers) LIKE ? OR lower(owner_note) LIKE ? OR phone LIKE ?)")
		args = append(args, like, like, like, like, like, like)
	}
	return strings.Join(w, " AND "), args
}

// ListBookings returns a person's bookings and how many match in all.
func (s *Store) ListBookings(ctx context.Context, uid int64, f BookingFilter) ([]*Booking, int, error) {
	where, args := f.where(uid)
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM bookings WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "start_at ASC, id ASC"
	if f.Newest || f.View == "past" || f.View == "cancelled" {
		order = "start_at DESC, id DESC"
	}
	lim := f.Limit
	if lim <= 0 || lim > 1000 {
		lim = 100
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+bookingCols+` FROM bookings WHERE `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		append(args, lim, max(f.Offset, 0))...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*Booking{}
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, b)
	}
	return out, total, rows.Err()
}

// CountBookings counts a person's bookings matching a filter.
func (s *Store) CountBookings(ctx context.Context, uid int64, f BookingFilter) int {
	where, args := f.where(uid)
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM bookings WHERE `+where, args...).Scan(&n)
	return n
}

// GetBooking returns one of a person's bookings.
func (s *Store) GetBooking(ctx context.Context, uid, id int64) (*Booking, error) {
	b, err := scanBooking(s.DB.QueryRowContext(ctx, `SELECT `+bookingCols+` FROM bookings WHERE id = ? AND user_id = ?`, id, uid))
	return b, notFound(err)
}

// BookingByToken finds a booking by the guest's own link.
func (s *Store) BookingByToken(ctx context.Context, token string) (*Booking, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	b, err := scanBooking(s.DB.QueryRowContext(ctx, `SELECT `+bookingCols+` FROM bookings WHERE token = ?`, token))
	return b, notFound(err)
}

// BookingByClientKey finds the booking made earlier with the same key.
func (s *Store) BookingByClientKey(ctx context.Context, uid int64, key string) (*Booking, error) {
	b, err := scanBooking(s.DB.QueryRowContext(ctx, `SELECT `+bookingCols+` FROM bookings WHERE user_id = ? AND client_key = ?`, uid, key))
	return b, notFound(err)
}

// Busy returns a person's bookings that hold time between from and to (UTC).
func (s *Store) Busy(ctx context.Context, uid int64, from, to time.Time) ([]*Booking, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+bookingCols+` FROM bookings WHERE user_id = ? AND status IN ('pending','confirmed')
		AND start_at < ? AND end_at > ? ORDER BY start_at`, uid, Stamp(to), Stamp(from))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Booking
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func bookingArgs(b *Booking) []any {
	if b.Answers == nil {
		b.Answers = []Answer{}
	}
	var typeID, inviteID any
	if b.TypeID > 0 {
		typeID = b.TypeID
	}
	if b.InviteID > 0 {
		inviteID = b.InviteID
	}
	return []any{typeID, b.Title, b.Emoji, b.Status, b.StartAt, b.EndAt, b.Minutes, b.GuestTZ, b.Name, strings.ToLower(b.Email), b.Phone, toJSON(b.Answers),
		b.LocationKind, b.Location, b.MeetingURL, b.Price, b.Currency, boolInt(b.Paid), b.PaidAt, b.Source, inviteID, b.CancelReason, b.CancelledBy,
		b.MovedFrom, b.OwnerNote, b.Language, boolInt(b.Reminded)}
}

// ErrTaken means the time was booked in the meantime.
var ErrTaken = errors.New("that time is taken")

// CreateBooking stores a booking. With check set, it runs inside one
// transaction that first asks check whether the time is still free.
func (s *Store) CreateBooking(ctx context.Context, uid int64, b *Booking, check func(tx *sql.Tx) error) error {
	if b.Token == "" {
		b.Token = RandomToken(18)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if check != nil {
		if err := check(tx); err != nil {
			return err
		}
	}
	args := append([]any{uid}, bookingArgs(b)...)
	args = append(args, b.Token, b.IPHash, b.ClientKey)
	cols := `user_id, type_id, title, emoji, status, start_at, end_at, minutes, guest_tz, name, email, phone, answers, location_kind, location,
		meeting_url, price, currency, paid, paid_at, source, invite_id, cancel_reason, cancelled_by, moved_from, owner_note, language, reminded,
		token, ip_hash, client_key`
	q := `INSERT INTO bookings (` + cols + `) VALUES (?` + strings.Repeat(", ?", len(args)-1) + `)`
	if b.CreatedAt != "" {
		q = `INSERT INTO bookings (` + cols + `, created_at) VALUES (?` + strings.Repeat(", ?", len(args)) + `)`
		args = append(args, b.CreatedAt)
	}
	res, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		return conflict(err)
	}
	b.ID, _ = res.LastInsertId()
	b.UserID = uid
	if b.InviteID > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE invites SET booking_id = ? WHERE id = ? AND user_id = ?`, b.ID, b.InviteID, uid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HeldBetween counts, inside a transaction, the bookings holding time
// between from and to (UTC), leaving out one booking (a move) and, for a
// shared time, the ones of the same page at the same start.
func HeldBetween(ctx context.Context, tx *sql.Tx, uid int64, from, to time.Time, skip int64) ([]*Booking, error) {
	rows, err := tx.QueryContext(ctx, `SELECT `+bookingCols+` FROM bookings WHERE user_id = ? AND status IN ('pending','confirmed')
		AND start_at < ? AND end_at > ? AND id <> ?`, uid, Stamp(to), Stamp(from), skip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Booking
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateBooking saves a booking.
func (s *Store) UpdateBooking(ctx context.Context, b *Booking) error {
	args := append(bookingArgs(b), Now(), b.ID, b.UserID)
	res, err := s.DB.ExecContext(ctx, `UPDATE bookings SET type_id = ?, title = ?, emoji = ?, status = ?, start_at = ?, end_at = ?, minutes = ?, guest_tz = ?,
		name = ?, email = ?, phone = ?, answers = ?, location_kind = ?, location = ?, meeting_url = ?, price = ?, currency = ?, paid = ?, paid_at = ?,
		source = ?, invite_id = ?, cancel_reason = ?, cancelled_by = ?, moved_from = ?, owner_note = ?, language = ?, reminded = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MoveBooking moves a booking to a new time inside one transaction that
// first asks check whether it is free.
func (s *Store) MoveBooking(ctx context.Context, b *Booking, check func(tx *sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if check != nil {
		if err := check(tx); err != nil {
			return err
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE bookings SET start_at = ?, end_at = ?, minutes = ?, moved_from = ?, reminded = 0, status = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`, b.StartAt, b.EndAt, b.Minutes, b.MovedFrom, b.Status, Now(), b.ID, b.UserID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// DeleteBooking removes a booking for good.
func (s *Store) DeleteBooking(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM bookings WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DueReminders are confirmed bookings starting before `before` that have
// not had their reminder (any owner).
func (s *Store) DueReminders(ctx context.Context, before time.Time) ([]*Booking, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+bookingCols+` FROM bookings WHERE status = 'confirmed' AND reminded = 0 AND start_at > ? AND start_at <= ?
		ORDER BY start_at LIMIT 200`, Stamp(time.Now()), Stamp(before))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Booking
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// MarkReminded notes that a booking's reminder went (or will not go).
func (s *Store) MarkReminded(ctx context.Context, id int64) {
	_, _ = s.DB.ExecContext(ctx, `UPDATE bookings SET reminded = 1 WHERE id = ?`, id)
}

// RecentBookings counts bookings made from one address since a time (a guard against floods).
func (s *Store) RecentBookings(ctx context.Context, ipHash string, since time.Time) int {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM bookings WHERE ip_hash = ? AND created_at >= ?`, ipHash, since.UTC().Format(time.RFC3339Nano)).Scan(&n)
	return n
}

// ---- made ----

// AddMade remembers something made from a booking.
func (s *Store) AddMade(ctx context.Context, uid int64, m *Made) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO made (user_id, booking_id, target, app, ref, title, url, error, auto) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, m.BookingID, m.Target, m.App, m.Ref, m.Title, m.URL, m.Error, boolInt(m.Auto))
	if err != nil {
		return err
	}
	m.ID, _ = res.LastInsertId()
	m.CreatedAt = Now()
	return nil
}

// MadeFor lists what was made from a booking (newest first).
func (s *Store) MadeFor(ctx context.Context, uid, bookingID int64) ([]Made, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, booking_id, target, app, ref, title, url, error, auto, created_at FROM made
		WHERE user_id = ? AND booking_id = ? ORDER BY id DESC`, uid, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Made{}
	for rows.Next() {
		var m Made
		var auto int
		if err := rows.Scan(&m.ID, &m.BookingID, &m.Target, &m.App, &m.Ref, &m.Title, &m.URL, &m.Error, &auto, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Auto = auto == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// MadeOne returns the working thing of a kind made from a booking (the calendar event).
func (s *Store) MadeOne(ctx context.Context, uid, bookingID int64, target string) (*Made, error) {
	var m Made
	var auto int
	err := s.DB.QueryRowContext(ctx, `SELECT id, booking_id, target, app, ref, title, url, error, auto, created_at FROM made
		WHERE user_id = ? AND booking_id = ? AND target = ? AND error = '' AND ref <> '' ORDER BY id DESC LIMIT 1`, uid, bookingID, target).
		Scan(&m.ID, &m.BookingID, &m.Target, &m.App, &m.Ref, &m.Title, &m.URL, &m.Error, &auto, &m.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	m.Auto = auto == 1
	return &m, nil
}

// DropMade forgets one thing made (the event was removed) or the failed tries of a kind.
func (s *Store) DropMade(ctx context.Context, uid, bookingID int64, target string, onlyErrors bool) {
	q := `DELETE FROM made WHERE user_id = ? AND booking_id = ? AND target = ?`
	if onlyErrors {
		q += ` AND error <> ''`
	}
	_, _ = s.DB.ExecContext(ctx, q, uid, bookingID, target)
}

// ---- invites ----

const inviteCols = `i.id, i.user_id, i.type_id, i.token, i.name, i.email, i.note, i.expires_at, COALESCE(i.booking_id, 0), i.source, i.created_at, COALESCE(t.title, '')`

func scanInvite(row interface{ Scan(...any) error }) (*Invite, error) {
	var iv Invite
	err := row.Scan(&iv.ID, &iv.UserID, &iv.TypeID, &iv.Token, &iv.Name, &iv.Email, &iv.Note, &iv.ExpiresAt, &iv.BookingID, &iv.Source, &iv.CreatedAt, &iv.TypeTitle)
	return &iv, err
}

// CreateInvite stores a personal link.
func (s *Store) CreateInvite(ctx context.Context, uid int64, iv *Invite) error {
	if iv.Token == "" {
		iv.Token = RandomToken(12)
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO invites (user_id, type_id, token, name, email, note, expires_at, source) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, iv.TypeID, iv.Token, iv.Name, strings.ToLower(iv.Email), iv.Note, iv.ExpiresAt, iv.Source)
	if err != nil {
		return err
	}
	iv.ID, _ = res.LastInsertId()
	iv.UserID = uid
	iv.CreatedAt = Now()
	return nil
}

// ListInvites returns a person's personal links (newest first).
func (s *Store) ListInvites(ctx context.Context, uid int64, limit int) ([]*Invite, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+inviteCols+` FROM invites i LEFT JOIN types t ON t.id = i.type_id WHERE i.user_id = ? ORDER BY i.id DESC LIMIT ?`, uid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Invite{}
	for rows.Next() {
		iv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, iv)
	}
	return out, rows.Err()
}

// InviteByToken finds a personal link (any owner).
func (s *Store) InviteByToken(ctx context.Context, token string) (*Invite, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	iv, err := scanInvite(s.DB.QueryRowContext(ctx, `SELECT `+inviteCols+` FROM invites i LEFT JOIN types t ON t.id = i.type_id WHERE i.token = ?`, token))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return iv, err
}

// DeleteInvite removes a personal link.
func (s *Store) DeleteInvite(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM invites WHERE id = ? AND user_id = ?`, id, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
