package meets

import (
	"encoding/json"
	"slices"
)

// Participant status values used in MeetSettings.Participants. Written by the
// ravanhealth caller (Index\AppointmentsController::upsertParticipantStatus),
// read here by the booking-conflict check.
const (
	ParticipantStatusBooked    = "booked"
	ParticipantStatusCancelled = "cancelled"
)

// Attendance status values for MeetSettings.AttendanceStatus. Written by
// ravanhealth (Admin\AppointmentsController::markPresent/markAbsent/updateSchedule)
// and the dashboard (AppointmentVisitStatusCard).
const (
	AttendanceStatusPresent = "present"
	AttendanceStatusAbsent  = "absent"
)

// MeetSettings is the typed view of the Meet.Settings JSON blob (stored as a
// raw JSON string in the settings column, exposed as google.protobuf.Struct
// over gRPC).
type MeetSettings struct {
	// MinHoursBetweenBookings is the minimum gap (hours) required
	// between two bookings of the same participant. 0/absent disables the check.
	MinHoursBetweenBookings int `json:"minHoursBetweenBookings,omitempty"`
	// Participants is the status-tracked booking history for the meet.
	Participants []ParticipantSetting `json:"participants,omitempty"`

	// Fields owned and managed by the callers (ravanhealth, dashboard)
	Eligibility []string `json:"eligibility,omitempty"`
	Capacity    int      `json:"capacity,omitempty"`
	ApprovedAt  *string  `json:"approved_at,omitempty"`
	// AttendanceStatus is AttendanceStatusPresent | AttendanceStatusAbsent (unset/omitted means not yet recorded).
	AttendanceStatus *string `json:"attendance_status,omitempty"`
	AttendanceAt     *string `json:"attendance_at,omitempty"`
	CreatedByUuid    string  `json:"created_by_uuid,omitempty"`
}

// ParticipantSetting is one entry of MeetSettings.Participants.
type ParticipantSetting struct {
	UUID        string `json:"uuid"`
	Status      string `json:"status"`                 // ParticipantStatusBooked | ParticipantStatusCancelled
	BookedAt    string `json:"booked_at,omitempty"`    // RFC3339
	CancelledAt string `json:"cancelled_at,omitempty"` // RFC3339
}

// hasActiveBooking reports whether pUuid is an active (not cancelled)
// participant of the given meet: listed in ParticipantUuids and not marked
// cancelled in settings.participants. A participant with no settings entry
// counts as active — admin-added participants never get one.
func hasActiveBooking(meet *Meet, pUuid string) bool {
	if !slices.Contains(meet.ParticipantUuids, pUuid) {
		return false
	}
	for _, p := range parseSettings(meet.Settings).Participants {
		if p.UUID == pUuid {
			return p.Status != ParticipantStatusCancelled
		}
	}
	return true
}

// parseSettings decodes the domain Meet.Settings JSON string into its typed
// view. Returns the zero value when settings are absent or malformed.
func parseSettings(settings *string) MeetSettings {
	var s MeetSettings
	if settings == nil || *settings == "" {
		return s
	}
	_ = json.Unmarshal([]byte(*settings), &s)
	return s
}
