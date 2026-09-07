package meets

import (
	"encoding/base64"
	"encoding/json"
)

// keysetCursor is the decoded shape of an opaque pagination cursor: the sort
// column's value (RFC3339Nano string) and the uuid tiebreaker of the last row
// on the previous page.
type keysetCursor struct {
	SortValue string `json:"v"`
	UUID      string `json:"u"`
}

// encodeCursor packs a row's sort position into an opaque base64 token.
func encodeCursor(sortValue, uuid string) string {
	b, _ := json.Marshal(keysetCursor{SortValue: sortValue, UUID: uuid}) //nolint:errchkjson // fixed struct of two strings, Marshal cannot fail
	return base64.URLEncoding.EncodeToString(b)
}

// decodeCursor unpacks a cursor token. ok is false for anything malformed —
// callers treat a malformed cursor as "start from the beginning" rather than
// an error, per the design spec.
func decodeCursor(cursor string) (sortValue, uuid string, ok bool) {
	if cursor == "" {
		return "", "", false
	}
	b, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", false
	}
	var c keysetCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return "", "", false
	}
	if c.UUID == "" {
		return "", "", false
	}
	return c.SortValue, c.UUID, true
}
