package gooyai

import (
	"context"
	"strconv"
)

// ListIncomingCalls returns one page of the tenant's calls; a nil filter
// means no filters. Calls are read-only in the Partner API.
func (c *Client) ListIncomingCalls(ctx context.Context, f *ListCallsFilter) (*CallPage, error) {
	if f == nil {
		f = &ListCallsFilter{}
	}
	q := query{}.page(f.PageQuery).
		str("from_number", f.FromNumber).
		str("to_number", f.ToNumber).
		int("model_id", f.ModelID).
		str("start_date", f.StartDate).
		str("end_date", f.EndDate).
		bool("call_forward", f.CallForward).
		bool("should_recall_user", f.ShouldRecallUser)

	var page CallPage
	if err := c.get(ctx, "/incoming-calls", q.values(), &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetIncomingCall returns one call. ErrNotFound covers both a missing call
// and one belonging to another tenant.
func (c *Client) GetIncomingCall(ctx context.Context, id int64) (*IncomingCall, error) {
	var call IncomingCall
	if err := c.get(ctx, "/incoming-calls/"+strconv.FormatInt(id, 10), nil, &call); err != nil {
		return nil, err
	}
	return &call, nil
}
