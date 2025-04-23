package employee

import (
	"context"
	"database/sql"
	"errors"
	"time"

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
	zlog := s.zlog.With(
		zap.String("method", "ListEmployees"),
		zap.Any("req", req),
	)

	employees, err := listEmployees(ctx, s.db, req)
	if err != nil {
		zlog.Error("failed to list employees", zap.Error(err))
		return nil, err
	}

	var pageToken string
	if l := len(employees); l > 0 && l == int(pager.Size(req.PageSize)) {
		last := employees[l-1]
		pageToken = pager.EncodeCursor(&pager.Cursor{
			ID:   last.ID,
			Time: last.CreatedAt,
		})
	}

	return &ListEmployeesResult{
		Employees:     employees,
		NextPageToken: pageToken,
	}, nil
}

func (s *Service) GetEmployeeByID(ctx context.Context, id string) (*Employee, error) {
	zlog := s.zlog.With(
		zap.String("method", "GetEmployeeByID"),
		zap.String("id", id),
	)

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

type Employee struct {
	ID         string    `json:"id"`
	Number     string    `json:"number"`
	FirstName  string    `json:"firstName"`
	LastName   string    `json:"lastName"`
	Department string    `json:"department"`
	JobTitle   string    `json:"jobTitle"`
	Company    string    `json:"company"`
	Contact    Contact   `json:"contact"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Contact struct {
	Email  string `json:"emailAddress"`
	Phone  string `json:"phoneNumber"`
	Mobile string `json:"mobileNumber"`
}

type ListEmployeesResult struct {
	Employees     []*Employee `json:"employees"`
	NextPageToken string      `json:"nextPageToken"`
}
