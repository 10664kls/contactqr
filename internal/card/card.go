package card

import (
	"context"
	"database/sql"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	e164 "github.com/nyaruka/phonenumbers"
	"go.uber.org/zap"
	edPb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	rpcStatus "google.golang.org/grpc/status"
)

type Service struct {
	db   *sql.DB
	zlog *zap.Logger
}

func NewService(_ context.Context, db *sql.DB, zlog *zap.Logger) (*Service, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}
	if zlog == nil {
		return nil, errors.New("zlog is nil")
	}

	return &Service{
		db:   db,
		zlog: zlog,
	}, nil
}

func (s *Service) CreateCard(ctx context.Context, in *CardReq) (*Card, error) {
	zlog := s.zlog.With(
		zap.String("method", "CreateCard"),
		zap.Any("req", in),
	)

	if err := in.Validate(); err != nil {
		return nil, err
	}

	card := newCard(in)
	if err := createCard(ctx, s.db, card); err != nil {
		zlog.Error("failed to create card", zap.Error(err))
		return nil, err
	}
	return card, nil
}

type CardReq struct {
	DisplayName string      `json:"displayName"`
	Department  string      `json:"department"`
	JobTitle    string      `json:"jobTitle"`
	Company     string      `json:"company"`
	Email       string      `json:"emailAddress"`
	Phone       PhoneNumber `json:"phone"`
	Mobile      PhoneNumber `json:"mobile"`

	employeeID string
}

type PhoneNumber struct {
	// ISO Alpha-2 code: "LA", "TH", "US", etc.
	Country string `json:"country"`

	// Phone number in E.164 format.
	Number string `json:"number"`
}

func (r *CardReq) Validate() error {
	violations := make([]*edPb.BadRequest_FieldViolation, 0)

	r.DisplayName = strings.TrimSpace(r.DisplayName)
	if r.DisplayName == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "displayName",
			Description: "displayName must not be empty",
		})
	}

	r.Department = strings.TrimSpace(r.Department)
	if r.Department == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "department",
			Description: "department must not be empty",
		})
	}

	r.JobTitle = strings.TrimSpace(r.JobTitle)
	if r.JobTitle == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "jobTitle",
			Description: "jobTitle must not be empty",
		})
	}

	r.Company = strings.TrimSpace(r.Company)
	if r.Company == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "company",
			Description: "company must not be empty",
		})
	}

	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "emailAddress",
			Description: "emailAddress must not be empty",
		})
	}
	if _, err := mail.ParseAddress(r.Email); err != nil {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "emailAddress",
			Description: "emailAddress must be a valid email address",
		})
	}

	r.Phone.Number = strings.TrimSpace(r.Phone.Number)
	r.Mobile.Number = strings.TrimSpace(r.Mobile.Number)
	if r.Phone.Number == "" && r.Mobile.Number == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "phoneNumber and mobileNumber",
			Description: "phoneNumber and mobileNumber both must not be empty. At least one of them must not be empty",
		})
	}

	if r.Phone.Number != "" {
		r.Phone.Country = strings.TrimSpace(r.Phone.Country)
		if r.Phone.Country == "" {
			violations = append(violations, &edPb.BadRequest_FieldViolation{
				Field:       "phone.country",
				Description: "phone country must not be empty",
			})
		}

		phone, err := e164.Parse(r.Phone.Number, r.Phone.Country)
		if err != nil {
			violations = append(violations, &edPb.BadRequest_FieldViolation{
				Field:       "phone.number",
				Description: "phone number must be a valid number",
			})
		}
		if !e164.IsValidNumber(phone) {
			violations = append(violations, &edPb.BadRequest_FieldViolation{
				Field:       "phone.number",
				Description: "phone number must be a valid number",
			})
		}
		r.Phone.Number = e164.Format(phone, e164.INTERNATIONAL)
	}

	if r.Mobile.Number != "" {
		r.Mobile.Country = strings.TrimSpace(r.Mobile.Country)
		if r.Mobile.Country == "" {
			violations = append(violations, &edPb.BadRequest_FieldViolation{
				Field:       "mobile.country",
				Description: "mobile country must not be empty",
			})
		}

		mobile, err := e164.Parse(r.Mobile.Number, r.Mobile.Country)
		if err != nil {
			violations = append(violations, &edPb.BadRequest_FieldViolation{
				Field:       "mobile.number",
				Description: "mobile number must be a valid number",
			})
		}
		if !e164.IsValidNumber(mobile) {
			violations = append(violations, &edPb.BadRequest_FieldViolation{
				Field:       "mobile.number",
				Description: "mobile number must be a valid number",
			})
		}
		r.Mobile.Number = e164.Format(mobile, e164.INTERNATIONAL)
	}

	if len(violations) > 0 {
		s, _ := rpcStatus.New(
			codes.InvalidArgument,
			"Card is not valid or incomplete. Please check the errors and try again, see details for more information.",
		).WithDetails(&edPb.BadRequest{FieldViolations: violations})
		return s.Err()
	}

	return nil
}

type Card struct {
	ID           string    `json:"id"`
	EmployeeID   string    `json:"employeeId"`
	DisplayName  string    `json:"displayName"`
	Department   string    `json:"department"`
	JobTitle     string    `json:"jobTitle"`
	Company      string    `json:"company"`
	Email        string    `json:"emailAddress"`
	PhoneNumber  string    `json:"phoneNumber"`
	MobileNumber string    `json:"mobileNumber"`
	Remark       string    `json:"remark"`
	Status       status    `json:"status"` // PENDING, APPROVED, REJECTED, PUBLISHED. Default: PENDING.
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	createdBy string
	updatedBy string
}

func (c *Card) Approved(by string) error {
	switch c.Status {
	case StatusApproved:
		return nil

	case StatusRejected:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in rejected status. Only pending status can be approved.")

	case StatusPublished:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in published status. Only pending status can be approved.")

	}

	c.Status = StatusApproved
	c.updatedBy = by
	c.UpdatedAt = time.Now()

	return nil
}

func (c *Card) Rejected(by, remark string) error {
	switch c.Status {
	case StatusRejected:
		return nil

	case StatusApproved:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in approved status. Only pending status can be rejected.")

	case StatusPublished:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in published status. Only pending status can be rejected.")
	}

	c.Status = StatusRejected
	c.Remark = remark
	c.updatedBy = by
	c.UpdatedAt = time.Now()

	return nil
}

func (c *Card) Published(by string) error {
	switch c.Status {
	case StatusPublished:
		return nil

	case StatusPending:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in pending status. Only approved status can be published.")

	case StatusRejected:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in rejected status. Only approved status can be published.")

	}

	c.Status = StatusPublished
	c.updatedBy = by
	c.UpdatedAt = time.Now()

	return nil
}

func (c *Card) Update(in *CardReq) error {
	switch c.Status {
	case StatusPublished:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in published status. Only pending and rejected status can be updated.")

	case StatusApproved:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in approved status. Only pending and rejected status can be updated.")

	}

	c.DisplayName = in.DisplayName
	c.PhoneNumber = in.Phone.Number
	c.MobileNumber = in.Mobile.Number
	c.Email = in.Email
	c.JobTitle = in.JobTitle
	c.Department = in.Department
	c.Company = in.Company
	c.Status = StatusPending
	c.UpdatedAt = time.Now()
	c.updatedBy = c.EmployeeID

	return nil
}

func newCard(in *CardReq) *Card {
	c := new(Card)
	now := time.Now()
	id := uuid.NewString()

	c.ID = strings.ToUpper(strings.Split(id, "-")[4])
	c.EmployeeID = in.employeeID
	c.DisplayName = in.DisplayName
	c.JobTitle = in.JobTitle
	c.Department = in.Department
	c.Company = in.Company
	c.Email = in.Email
	c.PhoneNumber = in.Phone.Number
	c.MobileNumber = in.Mobile.Number
	c.Status = StatusPending
	c.createdBy = in.employeeID
	c.updatedBy = in.employeeID
	c.CreatedAt = now
	c.UpdatedAt = now

	return c
}
