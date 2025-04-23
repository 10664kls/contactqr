package pager

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// Size returns the size of the page.
// If the size is less than or equal to 0, it returns 20.
// If the size is greater than 200, it returns 200.
func Size(size uint64) uint64 {
	if size <= 0 {
		return 20
	}
	if size > 200 {
		return 200
	}
	return size
}

// Cursor is designed for this project only, if you need to filter or order-by
// other field than id you must change this.
type Cursor struct {
	ID   string    `json:"id"`
	Time time.Time `json:"time"`
}

// EncodeCursor encodes the cursor.
func EncodeCursor(c *Cursor) string {
	cj, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(cj)
}

// DecodeCursor decodes the cursor.
func DecodeCursor(s string) (*Cursor, error) {
	cj, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	c := &Cursor{}
	return c, json.Unmarshal(cj, c)
}
