package store

import (
	"encoding/json"
)

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Booking page statuses.
const (
	TypeDraft  = "draft"  // only the owner sees it (a preview)
	TypeOpen   = "open"   // people can book
	TypePaused = "paused" // shows "not taking bookings right now"
)

// Booking statuses.
const (
	StatusPending   = "pending"   // waits for the owner to confirm
	StatusConfirmed = "confirmed" // on
	StatusDeclined  = "declined"  // the owner said no to a pending one
	StatusCancelled = "cancelled" // called off (by the guest or the owner)
	StatusDone      = "done"      // it happened
	StatusNoShow    = "no_show"   // they did not come
)

// Active reports whether a booking still holds its time.
func Active(status string) bool {
	return status == StatusPending || status == StatusConfirmed || status == StatusDone || status == StatusNoShow
}

// LocationKinds are where a booking happens.
var LocationKinds = []string{"video", "link", "phone", "phone_in", "place", "ask"}

// QuestionTypes are the kinds of extra question a page can ask (name and
// email are always asked).
var QuestionTypes = []string{"text", "long", "choice", "checkboxes", "yesno", "number", "url", "date"}

// Question is one extra question on a booking page.
type Question struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Label    string   `json:"label"`
	Help     string   `json:"help,omitempty"`
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"`
}

// After is what is made by itself when someone books.
type After struct {
	Event   bool `json:"event"`   // the booking into Pocket Calendar (moved and removed with it)
	Contact bool `json:"contact"` // the guest into Pocket Contacts (unless they are there)
	Deal    bool `json:"deal"`    // a lead in Pocket CRM
	Task    bool `json:"task"`    // a task to get ready, due the day before
	Note    bool `json:"note"`    // a note for the meeting in Pocket Notes
	Invoice bool `json:"invoice"` // a draft invoice in Pocket Invoicing (pages with a price)
}

// Type is one booking page.
type Type struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"-"`
	Handle       string     `json:"handle"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Emoji        string     `json:"emoji"`
	Status       string     `json:"status"`
	Durations    []int      `json:"durations"`
	LocationKind string     `json:"location_kind"`
	Location     string     `json:"location"`
	PlaceID      string     `json:"place_id"`
	ScheduleID   int64      `json:"schedule_id"`
	NoticeMin    int        `json:"notice_min"`
	DaysAhead    int        `json:"days_ahead"`
	BufferBefore int        `json:"buffer_before"`
	BufferAfter  int        `json:"buffer_after"`
	StepMin      int        `json:"step_min"`
	MaxPerDay    int        `json:"max_per_day"`
	Capacity     int        `json:"capacity"`
	Confirm      string     `json:"confirm"`
	AskPhone     string     `json:"ask_phone"`
	Questions    []Question `json:"questions"`
	Price        int64      `json:"price"`
	Currency     string     `json:"currency"`
	PayNote      string     `json:"pay_note"`
	ReminderH    int        `json:"reminder_h"`
	Language     string     `json:"language"`
	Accent       string     `json:"accent"`
	InviteOnly   bool       `json:"invite_only"`
	After        After      `json:"after"`
	Thanks       string     `json:"thanks"`
	Source       string     `json:"source"`
	Position     int        `json:"position"`
	CreatedAt    string     `json:"created_at"`
	UpdatedAt    string     `json:"updated_at"`

	// counted, not stored
	Upcoming int `json:"upcoming"`
	Bookings int `json:"bookings"`
	Views    int `json:"views"`
}

// Minutes is the page's first (usual) length.
func (t *Type) Minutes() int {
	if len(t.Durations) > 0 && t.Durations[0] > 0 {
		return t.Durations[0]
	}
	return 30
}

// Lasts reports whether m is one of the page's lengths.
func (t *Type) Lasts(m int) bool {
	for _, d := range t.Durations {
		if d == m {
			return true
		}
	}
	return false
}

// Span is a stretch of a day, "HH:MM" to "HH:MM".
type Span [2]string

// Override is a day that is not like its weekday (no hours = closed).
type Override struct {
	Day   string `json:"day"`
	Hours []Span `json:"hours"`
	Note  string `json:"note,omitempty"`
}

// Schedule is a set of weekly hours, with days that differ.
type Schedule struct {
	ID        int64             `json:"id"`
	UserID    int64             `json:"-"`
	Name      string            `json:"name"`
	TZ        string            `json:"tz"`
	Hours     map[string][]Span `json:"hours"` // "0" (Sunday) … "6"
	IsDefault bool              `json:"is_default"`
	Overrides []Override        `json:"overrides"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
	Pages     int               `json:"pages"` // booking pages that use it
}

// Answer is one answered question, kept with its label so it reads the
// same after the page changes.
type Answer struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// Booking is one booking.
type Booking struct {
	ID           int64    `json:"id"`
	UserID       int64    `json:"-"`
	TypeID       int64    `json:"type_id"`
	Title        string   `json:"title"`
	Emoji        string   `json:"emoji"`
	Status       string   `json:"status"`
	StartAt      string   `json:"-"` // UTC
	EndAt        string   `json:"-"`
	Minutes      int      `json:"minutes"`
	GuestTZ      string   `json:"guest_tz"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Phone        string   `json:"phone"`
	Answers      []Answer `json:"answers"`
	LocationKind string   `json:"location_kind"`
	Location     string   `json:"location"`
	MeetingURL   string   `json:"meeting_url"`
	Price        int64    `json:"price"`
	Currency     string   `json:"currency"`
	Paid         bool     `json:"paid"`
	PaidAt       string   `json:"paid_at"`
	Token        string   `json:"-"`
	Source       string   `json:"source"`
	InviteID     int64    `json:"invite_id,omitempty"`
	CancelReason string   `json:"cancel_reason"`
	CancelledBy  string   `json:"cancelled_by"`
	MovedFrom    string   `json:"moved_from"`
	OwnerNote    string   `json:"owner_note"`
	Language     string   `json:"language"`
	Reminded     bool     `json:"-"`
	IPHash       string   `json:"-"`
	ClientKey    string   `json:"-"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// Invite is a personal link that books once.
type Invite struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	TypeID    int64  `json:"type_id"`
	Token     string `json:"-"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Note      string `json:"note"`
	ExpiresAt string `json:"expires_at"`
	BookingID int64  `json:"booking_id"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
	TypeTitle string `json:"type"`
}

// Made is something made from a booking in another Pocket app.
type Made struct {
	ID        int64  `json:"id"`
	BookingID int64  `json:"booking_id"`
	Target    string `json:"target"`
	App       string `json:"app"`
	Ref       string `json:"ref"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Error     string `json:"error,omitempty"`
	Auto      bool   `json:"auto"`
	CreatedAt string `json:"created_at"`
}

// Conn is one person's connection to a sibling Pocket app.
type Conn struct {
	Mode      string `json:"mode,omitempty"`
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
}

// Settings are the box-wide values plus one person's preferences and their
// own connections.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday

	// Where and how times are shown and offered.
	TZ string `json:"tz"` // the owner's time zone ("" = the box's)

	// The person's own public page /p/<handle> listing their open pages.
	PageHandle string `json:"page_handle"`
	PageName   string `json:"page_name"`
	PageIntro  string `json:"page_intro"`

	// New booking pages start with these.
	Language string `json:"language"`
	Accent   string `json:"accent"`
	Currency string `json:"currency"`

	// Pocket Calendar: times already taken there cannot be booked, and
	// bookings go into a calendar.
	CheckCalendar bool    `json:"check_calendar"`
	BusyCalendars []int64 `json:"busy_calendars"` // empty = every calendar
	EventCalendar int64   `json:"event_calendar"` // 0 = Calendar's default

	// Emails to guests (through Pocket Mail).
	TellGuest bool   `json:"tell_guest"` // confirmation, changes, reminders
	MailFrom  string `json:"mail_from"`  // which Pocket Mail account ("" = its default)

	// Other apps.
	MoneyAccount   int64 `json:"money_account"`    // Pocket Money account payments go to (0 = the everyday one)
	MoneyIncomeCat int64 `json:"money_income_cat"` // its category
	TaskDays       int   `json:"task_days"`        // follow-up tasks are due this many days later
	AIHelp         bool  `json:"ai_help"`          // the Pocket AI buttons

	// Notifications.
	NotifyEach   bool   `json:"notify_each"`   // a push for each new booking and cancellation
	NotifyDigest bool   `json:"notify_digest"` // morning: today's bookings
	NotifyHour   int    `json:"notify_hour"`
	NotifyURL    string `json:"notify_url"`
	NotifyFormat string `json:"notify_format"` // apprise | ntfy | json
	PushPhone    bool   `json:"push_phone"`    // also to the phones with Pocket Hub (through Core)

	// Each sibling Pocket app, by slug.
	Conns map[string]Conn `json:"conns"`

	// AI: Pocket AI through Core, or an OpenAI-compatible API with the
	// person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
