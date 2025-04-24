package card

import (
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
	StatusPending:   "PENDING",
	StatusApproved:  "APPROVED",
	StatusRejected:  "REJECTED",
	StatusPublished: "PUBLISHED",
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

func (s *status) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}

	if t, err := strconv.Atoi(string(b)); err == nil {
		*s = status(t)
		return nil
	}

	b = b[1 : len(b)-1]
	if t, ok := statusValues[string(b)]; ok {
		*s = t
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

func (s status) String() string {
	if t, ok := statusNames[s]; ok {
		return t
	}
	return fmt.Sprintf("Status(%d)", s)
}
