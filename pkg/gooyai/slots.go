package gooyai

import (
	"context"
	"net/http"
	"strconv"
)

// ListSlots returns a provider's availability slots with their time segments.
// The endpoint isn't paginated; a nil filter means no filters.
func (c *Client) ListSlots(ctx context.Context, providerID int64, f *ListSlotsFilter) ([]AvailabilitySlot, error) {
	if f == nil {
		f = &ListSlotsFilter{}
	}
	q := query{}.
		str("date", f.Date).
		str("start_date", f.StartDate).
		str("end_date", f.EndDate).
		str("start_time", f.StartTime).
		str("end_time", f.EndTime)

	var slots []AvailabilitySlot
	if err := c.get(ctx, slotsPath(providerID), q.values(), &slots); err != nil {
		return nil, err
	}
	return slots, nil
}

// CreateSlot opens a schedule window. ErrConflict means it overlaps an
// existing slot.
func (c *Client) CreateSlot(ctx context.Context, providerID int64, req *CreateSlotRequest) (*AvailabilitySlot, error) {
	var slot AvailabilitySlot
	if err := c.do(ctx, http.MethodPost, slotsPath(providerID), nil, req, &slot); err != nil {
		return nil, err
	}
	return &slot, nil
}

// UpdateSlot changes the non-nil fields of a slot.
func (c *Client) UpdateSlot(ctx context.Context, providerID, slotID int64, req *UpdateSlotRequest) (*AvailabilitySlot, error) {
	var slot AvailabilitySlot
	if err := c.do(ctx, http.MethodPatch, slotPath(providerID, slotID), nil, req, &slot); err != nil {
		return nil, err
	}
	return &slot, nil
}

// DeleteSlot removes a slot.
func (c *Client) DeleteSlot(ctx context.Context, providerID, slotID int64) error {
	return c.do(ctx, http.MethodDelete, slotPath(providerID, slotID), nil, nil, nil)
}

// BookingOptions returns the attribute values allowed for one window of a
// slot. startTime/endTime are HH:mm on a :00 or :30 mark; empty is not sent.
func (c *Client) BookingOptions(ctx context.Context, providerID, slotID int64, startTime, endTime string) ([]CatalogAttribute, error) {
	q := query{}.str("start_time", startTime).str("end_time", endTime)

	var attrs []CatalogAttribute
	if err := c.get(ctx, slotPath(providerID, slotID)+"/booking-options", q.values(), &attrs); err != nil {
		return nil, err
	}
	return attrs, nil
}

func slotsPath(providerID int64) string {
	return providerPath(providerID) + "/availability-slots"
}

func slotPath(providerID, slotID int64) string {
	return slotsPath(providerID) + "/" + strconv.FormatInt(slotID, 10)
}
