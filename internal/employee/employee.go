package employee

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/10664kls/contactqr/internal/auth"
	"github.com/10664kls/contactqr/internal/pager"
	"go.uber.org/zap"
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

func (s *Service) ListEmployees(ctx context.Context, req *EmployeeQuery) (*ListEmployeesResult, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "ListEmployees"),
		zap.String("username", claims.Code),
		zap.Any("req", req),
	)

	if !claims.IsHR {
		return nil, rpcStatus.Error(
			codes.PermissionDenied,
			"You are not allowed to access theses employees.",
		)
	}

	employees, err := listEmployees(ctx, s.db, req)
	if err != nil {
		zlog.Error("failed to list employees", zap.Error(err))
		return nil, err
	}

	var pageToken string
	if l := len(employees); l > 0 && l == int(pager.Size(req.PageSize)) {
		last := employees[l-1]
		pageToken = pager.EncodeCursor(&pager.Cursor{
			ID:   strconv.FormatInt(last.ID, 10),
			Time: last.CreatedAt,
		})
	}

	return &ListEmployeesResult{
		Employees:     employees,
		NextPageToken: pageToken,
	}, nil
}

func (s *Service) GetEmployeeByID(ctx context.Context, id int64) (*Employee, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "GetEmployeeByID"),
		zap.String("username", claims.Code),
		zap.Int64("id", id),
	)

	if !claims.IsHR {
		return nil, rpcStatus.Error(
			codes.PermissionDenied,
			"You are not allowed to access this employee or (it may not exist)",
		)
	}

	employee, err := getEmployee(ctx, s.db, &EmployeeQuery{ID: id})
	if errors.Is(err, ErrEmployeeNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this employee or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get employee by id", zap.Error(err))
		return nil, err
	}

	return employee, nil
}

func (s *Service) GetMyEmployeeProfile(ctx context.Context) (*Employee, error) {
	claims := auth.ClaimsFromContext(ctx)

	zlog := s.zlog.With(
		zap.String("method", "GetMyEmployeeProfile"),
		zap.String("username", claims.Code),
	)

	employee, err := getEmployee(ctx, s.db, &EmployeeQuery{ID: claims.ID})
	if errors.Is(err, ErrEmployeeNotFound) {
		return nil, rpcStatus.Error(codes.PermissionDenied, "You are not allowed to access this employee or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get employee by id", zap.Error(err))
		return nil, err
	}

	return employee, nil
}

type Employee struct {
	ID             int64     `json:"id"`
	ManagerID      int64     `json:"managerId"`
	DepartmentID   int64     `json:"departmentId"`
	PositionID     int64     `json:"positionId"`
	CompanyID      int64     `json:"companyId"`
	Code           string    `json:"code"`
	DisplayName    string    `json:"displayName"`
	DepartmentName string    `json:"departmentName"`
	PositionName   string    `json:"positionName"`
	CompanyName    string    `json:"companyName"`
	Email          string    `json:"emailAddress"`
	Phone          string    `json:"phoneNumber"`
	Mobile         string    `json:"mobileNumber"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (e *Employee) SetPhone(phone string) {
	e.Phone = phone
}

func (e *Employee) SetMobile(mobile string) {
	e.Mobile = mobile
}

type ListEmployeesResult struct {
	Employees     []*Employee `json:"employees"`
	NextPageToken string      `json:"nextPageToken"`
}
