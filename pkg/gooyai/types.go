package gooyai

import (
	"encoding/json"
	"fmt"
)

// Pagination is data.pagination on list endpoints. Filters affect Total and
// TotalPages.
type Pagination struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"total_pages"`
}

// PageQuery is the page/limit pair every paginated endpoint accepts.
// Zero values are omitted so Gooyai's defaults (page 1, limit 10) apply.
type PageQuery struct {
	Page  int
	Limit int // 1..100
}

// MaxLimit is the largest page size Gooyai accepts; more is a 400.
const MaxLimit = 100

// jsonNull is the raw JSON null literal.
const jsonNull = "null"

// Bool decodes a JSON boolean that Gooyai may send as true/false or as a
// 0/1 number: the call flags are documented as booleans but come back as
// MySQL tinyints.
type Bool bool

// UnmarshalJSON accepts true, false, null and any JSON number (non-zero is
// true).
func (b *Bool) UnmarshalJSON(data []byte) error {
	switch s := string(data); s {
	case "true":
		*b = true
	case "false", jsonNull:
		*b = false
	default:
		var n float64
		if err := json.Unmarshal(data, &n); err != nil {
			return fmt.Errorf("gooyai: cannot decode %s as a boolean", s)
		}
		*b = n != 0
	}
	return nil
}

// IncomingCall is one call the AI agent answered.
type IncomingCall struct {
	ID               int64  `json:"id"`
	FromNumber       string `json:"from_number"`
	ToNumber         string `json:"to_number"`
	StartAt          string `json:"start_at"` // ISO 8601, UTC
	EndAt            string `json:"end_at"`
	DurationSeconds  int    `json:"duration_seconds"`
	RecordingURL     string `json:"recording_url"`
	CallForward      Bool   `json:"call_forward"`
	ShouldRecallUser Bool   `json:"should_recall_user"`
	// Analytics is flexible JSON (e.g. {"sentiment":"neutral","resolved":true});
	// its keys vary, so it's kept as a generic map.
	Analytics map[string]any `json:"analytics"`
	// Chats is the transcript. Documented as flexible JSON; the documented
	// example is [{"role","content"}], so it's kept raw and decoded by the
	// caller (see ChatMessages).
	Chats json.RawMessage `json:"chats"`
	Model *AIModel        `json:"model"`
}

// ChatMessage is one transcript turn in the documented Chats shape.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatMessages decodes Chats in the documented [{"role","content"}] shape.
// ok is false when Chats is absent or in some other shape, so callers can
// fall back instead of failing the whole call.
func (c *IncomingCall) ChatMessages() (msgs []ChatMessage, ok bool) {
	if len(c.Chats) == 0 || string(c.Chats) == jsonNull {
		return nil, false
	}
	if err := json.Unmarshal(c.Chats, &msgs); err != nil {
		return nil, false
	}
	return msgs, true
}

// AIModel is the Gooyai agent that handled a call.
type AIModel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListCallsFilter filters GET /incoming-calls. Zero values are not sent:
// Gooyai rejects unknown query parameters, and an empty one would filter.
type ListCallsFilter struct {
	PageQuery
	FromNumber string // partial match
	ToNumber   string // partial match
	ModelID    int64
	StartDate  string // ISO date, inclusive
	EndDate    string // ISO date, inclusive
	// CallForward / ShouldRecallUser are tri-state: nil means "don't filter".
	CallForward      *bool
	ShouldRecallUser *bool
}

// CallPage is one page of incoming calls.
type CallPage struct {
	Calls      []IncomingCall `json:"calls"`
	Pagination Pagination     `json:"pagination"`
}

// Attribute is a provider/slot attribute in its flattened form: one selected
// value per entry.
type Attribute struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	DataType         string `json:"data_type"`
	Value            string `json:"value"`
	AttributeValueID int64  `json:"attribute_value_id"`
}

// CatalogAttribute is an attribute with its allowed values: the catalog,
// booking options, and booking attributes all use this grouped form.
type CatalogAttribute struct {
	ID       int64            `json:"id"`
	Name     string           `json:"name"`
	DataType string           `json:"data_type"`
	Values   []AttributeValue `json:"values"`
}

// AttributeValue is one allowed value of a CatalogAttribute.
type AttributeValue struct {
	ID    int64  `json:"id"`
	Value string `json:"value"`
}

// AttributeSelection is how attributes are written:
// {"attribute_id":6,"attribute_value_ids":[42]}.
type AttributeSelection struct {
	AttributeID       int64   `json:"attribute_id"`
	AttributeValueIDs []int64 `json:"attribute_value_ids"`
}

// ServiceProvider is a schedulable person (doctor, therapist) in Gooyai.
// Providers are managed in Gooyai's panel; the API can only read them.
type ServiceProvider struct {
	ID              int64       `json:"id"`
	TenantID        int64       `json:"tenant_id"`
	FirstName       string      `json:"first_name"`
	LastName        string      `json:"last_name"`
	Specialty       string      `json:"specialty"`
	Phone           string      `json:"phone"`
	ProfileImageURL *string     `json:"profile_image_url"`
	IsActive        bool        `json:"is_active"`
	CreatedAt       string      `json:"created_at"`
	UpdatedAt       string      `json:"updated_at"`
	Attributes      []Attribute `json:"attributes"`
}

// ListProvidersFilter filters GET /providers.
type ListProvidersFilter struct {
	PageQuery
	IsActive *bool // nil means "don't filter"
	Search   string
}

// ProviderPage is one page of providers.
type ProviderPage struct {
	Providers  []ServiceProvider `json:"providers"`
	Pagination Pagination        `json:"pagination"`
}

// Slot statuses. SlotBooked is read-only: it can't be set on create/update.
const (
	SlotAvailable    = "available"
	SlotNotAvailable = "not_available"
	SlotBooked       = "booked"
)

// AvailabilitySlot is an open schedule window for one provider on one date.
type AvailabilitySlot struct {
	ID                int64         `json:"id"`
	ServiceProviderID int64         `json:"service_provider_id"`
	SlotDate          string        `json:"slot_date"`  // YYYY-MM-DD
	Status            string        `json:"status"`     // SlotAvailable / SlotNotAvailable / SlotBooked
	StartTime         string        `json:"start_time"` // HH:mm:ss
	EndTime           string        `json:"end_time"`   // HH:mm:ss
	IntervalMinutes   int           `json:"interval_minutes"`
	Description       string        `json:"description"`
	CreatedAt         string        `json:"created_at"`
	UpdatedAt         string        `json:"updated_at"`
	Attributes        []Attribute   `json:"attributes"`
	TimeSegments      []TimeSegment `json:"time_segments"`
}

// TimeSegment is one interval_minutes-long piece of a slot.
type TimeSegment struct {
	StartTime string `json:"start_time"` // HH:mm:ss
	EndTime   string `json:"end_time"`
	Status    string `json:"status"`
}

// ListSlotsFilter filters GET /providers/:id/availability-slots.
type ListSlotsFilter struct {
	Date      string // YYYY-MM-DD, a single day
	StartDate string // YYYY-MM-DD
	EndDate   string // YYYY-MM-DD
	StartTime string // HH:mm
	EndTime   string // HH:mm
}

// CreateSlotRequest is the body of POST /providers/:id/availability-slots.
// The slot length must be divisible by IntervalMinutes (default 30), and it
// must not overlap an existing slot (409).
type CreateSlotRequest struct {
	SlotDate        string               `json:"slot_date"`  // YYYY-MM-DD
	StartTime       string               `json:"start_time"` // HH:mm
	EndTime         string               `json:"end_time"`   // HH:mm
	IntervalMinutes int                  `json:"interval_minutes,omitempty"`
	Status          string               `json:"status,omitempty"`
	Description     string               `json:"description,omitempty"` // max 255
	Attributes      []AttributeSelection `json:"attributes,omitempty"`
}

// UpdateSlotRequest is the body of PATCH .../availability-slots/:slotId.
// Nil fields are left unchanged. A non-nil Attributes replaces the slot's —
// it's a pointer so that &[]AttributeSelection{} sends "attributes":[] and
// clears them, which a plain slice with omitempty would silently drop.
type UpdateSlotRequest struct {
	Status      *string               `json:"status,omitempty"`
	SlotDate    *string               `json:"slot_date,omitempty"`
	StartTime   *string               `json:"start_time,omitempty"`
	EndTime     *string               `json:"end_time,omitempty"`
	Description *string               `json:"description,omitempty"`
	Attributes  *[]AttributeSelection `json:"attributes,omitempty"`
}

// Booking statuses.
const (
	BookingActive   = "active"
	BookingCanceled = "canceled"
)

// Booking is an appointment in a provider's slot.
type Booking struct {
	ID                 int64              `json:"id"`
	ServiceProviderID  int64              `json:"service_provider_id"`
	AvailabilitySlotID int64              `json:"availability_slot_id"`
	CustomerName       string             `json:"customer_name"`
	CustomerPhone      string             `json:"customer_phone"`
	SlotDate           string             `json:"slot_date"`  // YYYY-MM-DD
	StartTime          string             `json:"start_time"` // HH:mm:ss
	EndTime            string             `json:"end_time"`
	Status             string             `json:"status"` // BookingActive / BookingCanceled
	Attributes         []CatalogAttribute `json:"attributes"`
	CreatedAt          string             `json:"created_at"`
	UpdatedAt          string             `json:"updated_at"`
}

// ListBookingsFilter filters GET /providers/:id/bookings.
type ListBookingsFilter struct {
	PageQuery
	Status string // BookingActive / BookingCanceled; empty means all
}

// BookingPage is one page of bookings.
type BookingPage struct {
	Bookings   []Booking  `json:"bookings"`
	Pagination Pagination `json:"pagination"`
}

// CreateBookingRequest is the body of POST /providers/:id/bookings.
// Times must be on a :00 or :30 mark and fall inside an available segment of
// the slot; ServiceProviderID must equal the provider in the path.
type CreateBookingRequest struct {
	ServiceProviderID  int64                `json:"service_provider_id"`
	AvailabilitySlotID int64                `json:"availability_slot_id"`
	StartTime          string               `json:"start_time"`     // HH:mm
	EndTime            string               `json:"end_time"`       // HH:mm
	CustomerName       string               `json:"customer_name"`  // max 128
	CustomerPhone      string               `json:"customer_phone"` // max 32
	Attributes         []AttributeSelection `json:"attributes,omitempty"`
}
