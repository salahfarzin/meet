package gooyai

import (
	"context"
	"net/http"
	"strconv"
)

// ListBookings returns one page of a provider's bookings; a nil filter
// means all statuses, default paging.
func (c *Client) ListBookings(ctx context.Context, providerID int64, f *ListBookingsFilter) (*BookingPage, error) {
	if f == nil {
		f = &ListBookingsFilter{}
	}
	q := query{}.page(f.PageQuery).str("status", f.Status)

	var page BookingPage
	if err := c.get(ctx, bookingsPath(providerID), q.values(), &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetBooking returns one booking.
func (c *Client) GetBooking(ctx context.Context, providerID, bookingID int64) (*Booking, error) {
	var b Booking
	if err := c.get(ctx, bookingPath(providerID, bookingID), nil, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// CreateBooking books a window. ErrConflict means it's already booked or
// overlaps another booking; ErrBadRequest covers an inactive provider, a
// misaligned time, or a window outside an available segment.
func (c *Client) CreateBooking(ctx context.Context, providerID int64, req *CreateBookingRequest) (*Booking, error) {
	var b Booking
	if err := c.do(ctx, http.MethodPost, bookingsPath(providerID), nil, req, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// CancelBooking cancels a booking (PATCH with no body). There is no
// reschedule: cancel and create a new booking instead.
func (c *Client) CancelBooking(ctx context.Context, providerID, bookingID int64) (*Booking, error) {
	var b Booking
	if err := c.do(ctx, http.MethodPatch, bookingPath(providerID, bookingID), nil, nil, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func bookingsPath(providerID int64) string {
	return providerPath(providerID) + "/bookings"
}

func bookingPath(providerID, bookingID int64) string {
	return bookingsPath(providerID) + "/" + strconv.FormatInt(bookingID, 10)
}
