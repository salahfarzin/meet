package gooyai

import (
	"net/url"
	"strconv"
)

// query builds url.Values, skipping zero values: Gooyai rejects unknown
// parameters with a 400, and an empty one (e.g. from_number=) would filter.
type query url.Values

func (q query) str(key, v string) query {
	if v != "" {
		url.Values(q).Set(key, v)
	}
	return q
}

func (q query) int(key string, v int64) query {
	if v != 0 {
		url.Values(q).Set(key, strconv.FormatInt(v, 10))
	}
	return q
}

// bool sends "true"/"false" (Gooyai's documented boolean form); nil is skipped.
func (q query) bool(key string, v *bool) query {
	if v != nil {
		url.Values(q).Set(key, strconv.FormatBool(*v))
	}
	return q
}

func (q query) page(p PageQuery) query {
	return q.int("page", int64(p.Page)).int("limit", int64(p.Limit))
}

func (q query) values() url.Values { return url.Values(q) }
