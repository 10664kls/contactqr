package server

import (
	"errors"
	"net/http"

	"github.com/10664kls/contactqr/internal/employee"
	"github.com/labstack/echo/v4"
	edPb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	rpcStatus "google.golang.org/grpc/status"
)

type Server struct {
	employee *employee.Service
}

func NewServer(emp *employee.Service) (*Server, error) {
	if emp == nil {
		return nil, errors.New("employee service is nil")
	}

	return &Server{
		employee: emp,
	}, nil
}

func (s *Server) Install(e *echo.Echo, mws ...echo.MiddlewareFunc) error {
	if e == nil {
		return errors.New("echo is nil")
	}

	v1 := e.Group("/v1")

	v1.GET("/employees", s.listEmployees)
	v1.GET("/employees/:id", s.getEmployeeByID)

	return nil
}

func badJSON() error {
	s, _ := rpcStatus.New(codes.InvalidArgument, "Request body must be a valid JSON.").
		WithDetails(&edPb.ErrorInfo{
			Reason: "BINDING_ERROR",
			Domain: "http",
		})

	return s.Err()
}

func (s *Server) listEmployees(c echo.Context) error {
	req := new(employee.EmployeeQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	employees, err := s.employee.ListEmployees(ctx, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, employees)
}

func (s *Server) getEmployeeByID(c echo.Context) error {
	id := c.Param("id")
	ctx := c.Request().Context()

	employee, err := s.employee.GetEmployeeByID(ctx, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, echo.Map{
		"employee": employee,
	})
}
