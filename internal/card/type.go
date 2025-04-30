package card

import (
	"database/sql/driver"
	"fmt"
	"strconv"
)

type status int

const (
	StatusUnspecified status = iota
	StatusPending
	StatusApproved
	StatusRejected
	StatusPublished
)

var statusNames = map[status]string{
	StatusUnspecified: "UNSPECIFIED",
	StatusPending:     "PENDING",
	StatusApproved:    "APPROVED",
	StatusRejected:    "REJECTED",
	StatusPublished:   "PUBLISHED",
}

var statusValues = map[string]status{
	"PENDING":     StatusPending,
	"APPROVED":    StatusApproved,
	"REJECTED":    StatusRejected,
	"PUBLISHED":   StatusPublished,
	"UNSPECIFIED": StatusUnspecified,
}

func (s status) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

func (s *status) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	data = data[1 : len(data)-1]
	if t, ok := statusValues[string(data)]; ok {
		*s = t
	}

	if t, err := strconv.Atoi(string(data)); err == nil {
		*s = status(t)
		return nil
	}
	return nil
}

func (s *status) Scan(src any) error {
	if src == nil {
		return nil
	}

	switch src := src.(type) {
	case string:
		if t, ok := statusValues[src]; ok {
			*s = t
		}

	case []byte:
		if t, ok := statusValues[string(src)]; ok {
			*s = t
		}
	}

	return nil
}

func (s status) Value() (driver.Value, error) {
	return s.String(), nil
}

func (s status) String() string {
	if t, ok := statusNames[s]; ok {
		return t
	}
	return fmt.Sprintf("Status(%d)", s)
}
