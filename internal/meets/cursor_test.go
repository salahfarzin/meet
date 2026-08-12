package meets

import "testing"

func TestEncodeDecodeCursorRoundTrip(t *testing.T) {
	token := encodeCursor("2026-01-15T10:30:00.123456789Z", "018f1e2a-1234-7abc-9def-0123456789ab")

	sortValue, uuid, ok := decodeCursor(token)
	if !ok {
		t.Fatalf("decodeCursor(%q) ok = false, want true", token)
	}
	if sortValue != "2026-01-15T10:30:00.123456789Z" {
		t.Errorf("sortValue = %q, want %q", sortValue, "2026-01-15T10:30:00.123456789Z")
	}
	if uuid != "018f1e2a-1234-7abc-9def-0123456789ab" {
		t.Errorf("uuid = %q, want %q", uuid, "018f1e2a-1234-7abc-9def-0123456789ab")
	}
}

func TestDecodeCursorEmpty(t *testing.T) {
	_, _, ok := decodeCursor("")
	if ok {
		t.Error("decodeCursor(\"\") ok = true, want false")
	}
}

func TestDecodeCursorMalformedBase64(t *testing.T) {
	_, _, ok := decodeCursor("not-valid-base64!!!")
	if ok {
		t.Error("decodeCursor(malformed base64) ok = true, want false")
	}
}

func TestDecodeCursorMalformedJSON(t *testing.T) {
	// Valid base64, but the decoded bytes aren't the expected JSON shape.
	_, _, ok := decodeCursor("bm90LWpzb24=") // base64("not-json")
	if ok {
		t.Error("decodeCursor(valid base64, invalid JSON) ok = true, want false")
	}
}

func TestDecodeCursorMissingUUID(t *testing.T) {
	// {"v":"2026-01-15T10:30:00Z"} with no "u" key.
	token := "eyJ2IjoiMjAyNi0wMS0xNVQxMDozMDowMFoifQ=="
	_, _, ok := decodeCursor(token)
	if ok {
		t.Error("decodeCursor(cursor missing uuid) ok = true, want false")
	}
}
