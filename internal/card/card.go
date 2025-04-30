package card

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/10664kls/contactqr/internal/auth"
	"github.com/10664kls/contactqr/internal/employee"
	"github.com/10664kls/contactqr/internal/pager"
	"github.com/google/uuid"
	e164 "github.com/nyaruka/phonenumbers"
	"go.uber.org/zap"
	edPb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	rpcStatus "google.golang.org/grpc/status"
)

type Service struct {
	employee *employee.Service
	db       *sql.DB
	zlog     *zap.Logger
}

func NewService(_ context.Context, db *sql.DB, zlog *zap.Logger, employee *employee.Service) (*Service, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}
	if zlog == nil {
		return nil, errors.New("zlog is nil")
	}

	if employee == nil {
		return nil, errors.New("employee is nil")
	}

	return &Service{
		db:       db,
		zlog:     zlog,
		employee: employee,
	}, nil
}

func (s *Service) CreateBusinessCard(ctx context.Context, in *CardReq) (*Card, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "CreateBusinessCard"),
		zap.Any("req", in),
		zap.String("username", claims.Code),
	)

	if err := in.Validate(); err != nil {
		return nil, err
	}

	employee, err := s.employee.GetMyEmployeeProfile(ctx)
	if err != nil {
		return nil, err
	}

	employee.SetPhone(in.Phone.Number)
	employee.SetMobile(in.Mobile.Number)
	card := newCardFromEmployee(employee)
	if err := createCard(ctx, s.db, card); err != nil {
		zlog.Error("failed to create card", zap.Error(err))
		return nil, err
	}
	return card, nil
}

func (s *Service) UpdateBusinessCard(ctx context.Context, in *CardReq) (*Card, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "UpdateBusinessCard"),
		zap.Any("req", in),
		zap.String("username", claims.Code),
	)

	if err := in.Validate(); err != nil {
		return nil, err
	}

	employee, err := s.employee.GetMyEmployeeProfile(ctx)
	if err != nil {
		return nil, err
	}

	card, err := getCard(ctx, s.db, &CardQuery{
		EmployeeID: employee.ID,
		ID:         in.ID,
	})
	if errors.Is(err, ErrCardNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this card or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get card by id", zap.Error(err))
		return nil, err
	}

	employee.SetPhone(in.Phone.Number)
	employee.SetMobile(in.Mobile.Number)
	card.UpdateFromEmployee(employee)
	if err := updateCard(ctx, s.db, card); err != nil {
		zlog.Error("failed to update card", zap.Error(err))
		return nil, err
	}

	return card, nil
}

type ListCardsResult struct {
	Cards         []*Card `json:"businessCards"`
	NextPageToken string  `json:"nextPageToken"`
}

func (s *Service) ListBusinessCards(ctx context.Context, req *CardQuery) (*ListCardsResult, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "ListBusinessCards"),
		zap.Any("req", req),
		zap.String("username", claims.Code),
	)

	if !claims.IsHR {
		return nil, rpcStatus.Error(
			codes.PermissionDenied,
			"You are not allowed to access theses business cards.",
		)
	}

	cards, err := listCards(ctx, s.db, req)
	if err != nil {
		zlog.Error("failed to list business cards", zap.Error(err))
		return nil, err
	}

	var pageToken string
	if l := len(cards); l > 0 && l == int(pager.Size(req.PageSize)) {
		last := cards[l-1]
		pageToken = pager.EncodeCursor(&pager.Cursor{
			ID:   last.ID,
			Time: last.CreatedAt,
		})
	}

	return &ListCardsResult{
		Cards:         cards,
		NextPageToken: pageToken,
	}, nil
}

func (s *Service) GetBusinessCardByID(ctx context.Context, id string) (*Card, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "GetBusinessCardByID"),
		zap.String("username", claims.Code),
		zap.String("id", id),
	)

	if !claims.IsHR {
		return nil, rpcStatus.Error(
			codes.PermissionDenied,
			"You are not allowed to access this card or (it may not exist)",
		)
	}

	card, err := getCard(ctx, s.db, &CardQuery{
		ID: id,
	})
	if errors.Is(err, ErrCardNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this card or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get card by id", zap.Error(err))
		return nil, err
	}

	return card, nil
}

func (s *Service) GetMyBusinessCardByID(ctx context.Context, id string) (*Card, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "GetMyBusinessCardByID"),
		zap.String("username", claims.Code),
		zap.String("id", id),
	)

	card, err := getCard(ctx, s.db, &CardQuery{
		ID:         id,
		EmployeeID: claims.ID,
	})
	if errors.Is(err, ErrCardNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this card or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get card by id", zap.Error(err))
		return nil, err
	}

	return card, nil
}

func (s *Service) ListMyApprovalBusinessCards(ctx context.Context, req *CardQuery) (*ListCardsResult, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "ListMyApprovalBusinessCards"),
		zap.Any("req", req),
		zap.String("username", claims.Code),
	)

	req.managerID = claims.ID
	cards, err := listCards(ctx, s.db, req)
	if err != nil {
		zlog.Error("failed to list cards", zap.Error(err))
		return nil, err
	}

	var pageToken string
	if l := len(cards); l > 0 && l == int(pager.Size(req.PageSize)) {
		last := cards[l-1]
		pageToken = pager.EncodeCursor(&pager.Cursor{
			ID:   last.ID,
			Time: last.CreatedAt,
		})
	}

	return &ListCardsResult{
		Cards:         cards,
		NextPageToken: pageToken,
	}, nil
}

func (s *Service) GetMyApprovalBusinessCardByID(ctx context.Context, id string) (*Card, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "GetMyApprovalBusinessCardByID"),
		zap.String("username", claims.Code),
		zap.String("id", id),
	)

	card, err := getCard(ctx, s.db, &CardQuery{
		ID:        id,
		managerID: claims.ID,
	})
	if errors.Is(err, ErrCardNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this card or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get card by id", zap.Error(err))
		return nil, err
	}

	return card, nil
}

func (s *Service) ListMyBusinessCards(ctx context.Context, req *CardQuery) (*ListCardsResult, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "ListMyBusinessCards"),
		zap.Any("req", req),
		zap.String("username", claims.Code),
	)

	req.EmployeeID = claims.ID
	cards, err := listCards(ctx, s.db, req)
	if err != nil {
		zlog.Error("failed to list cards", zap.Error(err))
		return nil, err
	}

	var pageToken string
	if l := len(cards); l > 0 && l == int(pager.Size(req.PageSize)) {
		last := cards[l-1]
		pageToken = pager.EncodeCursor(&pager.Cursor{
			ID:   last.ID,
			Time: last.CreatedAt,
		})
	}

	return &ListCardsResult{
		Cards:         cards,
		NextPageToken: pageToken,
	}, nil
}

type ApproveBusinessCardReq struct {
	ID string `json:"cardId" param:"id"`
}

func (r *ApproveBusinessCardReq) Validate() error {
	violations := make([]*edPb.BadRequest_FieldViolation, 0)

	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "cardId",
			Description: "cardId must not be empty",
		})
	}

	if len(violations) > 0 {
		s, _ := rpcStatus.New(
			codes.InvalidArgument,
			"Your approval business card is not valid or incomplete. Please check the errors and try again, see details for more information.",
		).WithDetails(&edPb.BadRequest{FieldViolations: violations})
		return s.Err()
	}

	return nil
}

func (s *Service) ApproveBusinessCard(ctx context.Context, in *ApproveBusinessCardReq) (*Card, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "ApproveBusinessCard"),
		zap.String("username", claims.Code),
		zap.String("req", in.ID),
	)

	if err := in.Validate(); err != nil {
		return nil, err
	}

	card, err := getCard(ctx, s.db, &CardQuery{
		ID:        in.ID,
		managerID: claims.ID,
	})
	if errors.Is(err, ErrCardNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this card or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get card by id", zap.Error(err))
		return nil, err
	}

	if err := card.Approved(claims.Code); err != nil {
		return nil, err
	}

	if err := updateCard(ctx, s.db, card); err != nil {
		zlog.Error("failed to update card", zap.Error(err))
		return nil, err
	}

	return card, nil
}

type RejectBusinessCardReq struct {
	Remark string `json:"remark"`
	ID     string `json:"cardId" param:"id"`
}

func (r *RejectBusinessCardReq) Validate() error {
	violations := make([]*edPb.BadRequest_FieldViolation, 0)

	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "cardId",
			Description: "cardId must not be empty",
		})
	}

	r.Remark = strings.TrimSpace(r.Remark)
	if r.Remark == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "remark",
			Description: "remark must not be empty",
		})
	}

	if len(violations) > 0 {
		s, _ := rpcStatus.New(
			codes.InvalidArgument,
			"Your reject business card is not valid or incomplete. Please check the errors and try again, see details for more information.",
		).WithDetails(&edPb.BadRequest{FieldViolations: violations})
		return s.Err()
	}

	return nil
}

func (s *Service) RejectBusinessCard(ctx context.Context, in *RejectBusinessCardReq) (*Card, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "RejectBusinessCard"),
		zap.String("username", claims.Code),
		zap.Any("req", in),
	)

	if err := in.Validate(); err != nil {
		return nil, err
	}

	card, err := getCard(ctx, s.db, &CardQuery{
		ID:        in.ID,
		managerID: claims.ID,
	})
	if errors.Is(err, ErrCardNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this card or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get card by id", zap.Error(err))
		return nil, err
	}

	if err := card.Rejected(claims.Code, in.Remark); err != nil {
		return nil, err
	}

	if err := updateCard(ctx, s.db, card); err != nil {
		zlog.Error("failed to update card", zap.Error(err))
		return nil, err
	}

	return card, nil
}

type PublishBusinessCardReq struct {
	ID string `json:"cardId" param:"id"`
}

func (r *PublishBusinessCardReq) Validate() error {
	violations := make([]*edPb.BadRequest_FieldViolation, 0)

	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "cardId",
			Description: "cardId must not be empty",
		})
	}

	if len(violations) > 0 {
		s, _ := rpcStatus.New(
			codes.InvalidArgument,
			"Your publish business card is not valid or incomplete. Please check the errors and try again, see details for more information.",
		).WithDetails(&edPb.BadRequest{FieldViolations: violations})
		return s.Err()
	}

	return nil
}

func (s *Service) PublishBusinessCard(ctx context.Context, in *PublishBusinessCardReq) (*Card, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "PublishBusinessCard"),
		zap.String("username", claims.Code),
		zap.Any("req", in),
	)

	if !claims.IsHR {
		return nil, rpcStatus.Error(
			codes.PermissionDenied,
			"You are not allowed to access this card or (it may not exist)",
		)
	}

	if err := in.Validate(); err != nil {
		return nil, err
	}

	card, err := getCard(ctx, s.db, &CardQuery{
		ID: in.ID,
	})
	if errors.Is(err, ErrCardNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this card or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get card by id", zap.Error(err))
		return nil, err
	}

	if err := card.Published(claims.Code); err != nil {
		return nil, err
	}

	if err := updateCard(ctx, s.db, card); err != nil {
		zlog.Error("failed to update card", zap.Error(err))
		return nil, err
	}

	return card, nil
}

type CardReq struct {
	ID     string      `json:"-" param:"id"`
	Phone  PhoneNumber `json:"phone"`
	Mobile PhoneNumber `json:"mobile"`
}

type PhoneNumber struct {
	// ISO Alpha-2 code: "LA", "TH", "US", etc.
	Country string `json:"country"`

	// Phone number in E.164 format.
	Number string `json:"number"`
}

func (r *CardReq) Validate() error {
	violations := make([]*edPb.BadRequest_FieldViolation, 0)

	r.Phone.Number = strings.TrimSpace(r.Phone.Number)
	if r.Phone.Number == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "phone.number",
			Description: "phone number must not be empty",
		})
	}

	r.Phone.Country = strings.TrimSpace(r.Phone.Country)
	if r.Phone.Country == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "phone.country",
			Description: "phone country must not be empty.",
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

type VCF struct {
	Content string `json:"vcf"`
}

func (s *Service) GetMyVCFBusinessCardByID(ctx context.Context, id string) (*VCF, error) {
	// claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "GetMyVCFBusinessCardByID"),
		// zap.String("username", claims.Code),
		zap.String("id", id),
	)

	card, err := getCard(ctx, s.db, &CardQuery{
		ID: id,
		// EmployeeID: claims.ID,
	})
	if errors.Is(err, ErrCardNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this card or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get card by id", zap.Error(err))
		return nil, err
	}

	if card.Status != StatusPublished {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this card or (it may not exist)")
	}

	byt, err := genVCF(card)
	if err != nil {
		zlog.Error("failed to gen vcf", zap.Error(err))
		return nil, err
	}

	return &VCF{
		Content: base64.StdEncoding.EncodeToString(byt),
	}, nil
}

type Card struct {
	EmployeeID     int64     `json:"employeeId"`
	DepartmentID   int64     `json:"departmentId"`
	PositionID     int64     `json:"positionId"`
	CompanyID      int64     `json:"companyId"`
	ID             string    `json:"id"`
	EmployeeCode   string    `json:"employeeCode"`
	DisplayName    string    `json:"displayName"`
	Email          string    `json:"emailAddress"`
	PhoneNumber    string    `json:"phoneNumber"`
	MobileNumber   string    `json:"mobileNumber"`
	PositionName   string    `json:"positionName"`
	DepartmentName string    `json:"departmentName"`
	CompanyName    string    `json:"companyName"`
	Remark         string    `json:"remark"`
	Status         status    `json:"status"` // PENDING, APPROVED, REJECTED, PUBLISHED. Default: PENDING.
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`

	createdBy string
	updatedBy string
}

func (c *Card) Approved(by string) error {
	switch c.Status {
	case StatusApproved:
		return nil

	case StatusRejected:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in REJECTED status. Only PENDING status can be APPROVED.")

	case StatusPublished:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in PUBLISHED status. Only PENDING status can be APPROVED.")

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
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in APPROVED status. Only PENDING status can be REJECTED.")

	case StatusPublished:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in PUBLISHED status. Only PENDING status can be REJECTED.")
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
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in PENDING status. Only APPROVED status can be PUBLISHED.")

	case StatusRejected:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in REJECTED status. Only APPROVED status can be PUBLISHED.")

	}

	c.Status = StatusPublished
	c.updatedBy = by
	c.UpdatedAt = time.Now()

	return nil
}

func (c *Card) UpdateFromEmployee(in *employee.Employee) error {
	switch c.Status {
	case StatusPublished:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in PUBLISHED status. Only PENDING and REJECTED status can be updated.")

	case StatusApproved:
		return rpcStatus.Error(codes.FailedPrecondition, "Card is in APPROVED status. Only PENDING and REJECTED status can be updated.")

	}

	c.EmployeeCode = in.Code
	c.DisplayName = in.DisplayName
	c.PhoneNumber = in.Phone
	c.MobileNumber = in.Mobile
	c.Email = in.Email
	c.PositionID = in.PositionID
	c.PositionName = in.PositionName
	c.DepartmentID = in.DepartmentID
	c.DepartmentName = in.DepartmentName
	c.CompanyID = in.CompanyID
	c.CompanyName = in.CompanyName
	c.Status = StatusPending
	c.updatedBy = in.Code
	c.UpdatedAt = time.Now()

	return nil
}

func newCardFromEmployee(e *employee.Employee) *Card {
	c := new(Card)
	now := time.Now()
	id := uuid.NewString()

	c.ID = strings.ToUpper(strings.Split(id, "-")[4])
	c.EmployeeID = e.ID
	c.EmployeeCode = e.Code
	c.DisplayName = e.DisplayName
	c.PositionID = e.PositionID
	c.PositionName = e.PositionName
	c.DepartmentID = e.DepartmentID
	c.DepartmentName = e.DepartmentName
	c.CompanyID = e.CompanyID
	c.CompanyName = e.CompanyName
	c.Email = e.Email
	c.PhoneNumber = e.Phone
	c.MobileNumber = e.Mobile
	c.Status = StatusPending
	c.createdBy = e.Code
	c.updatedBy = e.Code
	c.CreatedAt = now
	c.UpdatedAt = now

	return c
}
