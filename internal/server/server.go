package server

import (
	"errors"
	"net/http"

	"github.com/10664kls/contactqr/internal/auth"
	"github.com/10664kls/contactqr/internal/card"
	"github.com/10664kls/contactqr/internal/employee"
	"github.com/labstack/echo/v4"
	edPb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	rpcStatus "google.golang.org/grpc/status"
)

type Server struct {
	employee *employee.Service
	card     *card.Service
	auth     *auth.Auth
}

func NewServer(emp *employee.Service, card *card.Service, auth *auth.Auth) (*Server, error) {
	if emp == nil {
		return nil, errors.New("employee service is nil")
	}
	if card == nil {
		return nil, errors.New("card service is nil")
	}
	if auth == nil {
		return nil, errors.New("auth service is nil")
	}

	return &Server{
		employee: emp,
		card:     card,
		auth:     auth,
	}, nil
}

func (s *Server) Install(e *echo.Echo, mws ...echo.MiddlewareFunc) error {
	if e == nil {
		return errors.New("echo is nil")
	}

	v1 := e.Group("/v1")
	v1.POST("/auth/login", s.login)
	v1.POST("/auth/token", s.refreshToken)
	v1.GET("/auth/profile", s.authProfile, mws...)

	v1.GET("/employees", s.listEmployees, mws...)
	v1.GET("/employees/:id", s.getEmployeeByID, mws...)
	v1.GET("/employees/me/profile", s.getMyEmployeeProfile, mws...)

	v1.POST("/business-cards", s.createBusinessCard, mws...)
	v1.PUT("/business-cards/:id", s.updateBusinessCard, mws...)
	v1.GET("/business-cards/me", s.listMyBusinessCards, mws...)
	v1.GET("/business-cards/me/approval", s.listMyApprovalBusinessCards, mws...)
	v1.GET("/business-cards/me/approval/:id", s.getMyApprovalBusinessCardByID, mws...)
	v1.GET("/business-cards/me/:id", s.getMyBusinessCardByID, mws...)
	v1.GET("/business-cards", s.listBusinessCards, mws...)
	v1.GET("/business-cards/:id", s.getBusinessCardByID, mws...)

	v1.POST("/business-cards/approve", s.approveBusinessCard, mws...)
	v1.POST("/business-cards/reject", s.rejectBusinessCard, mws...)
	v1.POST("/business-cards/publish", s.publishBusinessCard, mws...)

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

func badParam() error {
	s, _ := rpcStatus.New(codes.InvalidArgument, "Request parameters must be a valid type.").
		WithDetails(&edPb.ErrorInfo{
			Reason: "BINDING_ERROR",
			Domain: "http",
		})

	return s.Err()
}

func (s *Server) listEmployees(c echo.Context) error {
	req := new(employee.EmployeeQuery)
	if err := c.Bind(req); err != nil {
		return badParam()
	}

	ctx := c.Request().Context()
	employees, err := s.employee.ListEmployees(ctx, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, employees)
}

func (s *Server) getEmployeeByID(c echo.Context) error {
	req := new(employee.EmployeeQuery)
	if err := c.Bind(req); err != nil {
		return badParam()
	}

	ctx := c.Request().Context()

	employee, err := s.employee.GetEmployeeByID(ctx, req.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, echo.Map{
		"employee": employee,
	})
}

func (s *Server) getMyEmployeeProfile(c echo.Context) error {
	ctx := c.Request().Context()
	employee, err := s.employee.GetMyEmployeeProfile(ctx)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, echo.Map{
		"employeeProfile": employee,
	})
}

func (s *Server) createBusinessCard(c echo.Context) error {
	req := new(card.CardReq)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	card, err := s.card.CreateBusinessCard(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"card": card,
	})
}

func (s *Server) updateBusinessCard(c echo.Context) error {
	req := new(card.CardReq)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	card, err := s.card.UpdateBusinessCard(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"card": card,
	})
}

func (s *Server) listMyBusinessCards(c echo.Context) error {
	req := new(card.CardQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	cards, err := s.card.ListMyBusinessCards(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, cards)
}

func (s *Server) getMyBusinessCardByID(c echo.Context) error {
	req := new(card.CardQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	card, err := s.card.GetMyBusinessCardByID(ctx, req.ID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"card": card,
	})
}

func (s *Server) listBusinessCards(c echo.Context) error {
	req := new(card.CardQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	cards, err := s.card.ListBusinessCards(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, cards)
}

func (s *Server) getBusinessCardByID(c echo.Context) error {
	req := new(card.CardQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	card, err := s.card.GetBusinessCardByID(ctx, req.ID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"card": card,
	})
}

func (s *Server) listMyApprovalBusinessCards(c echo.Context) error {
	req := new(card.CardQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	cards, err := s.card.ListMyApprovalBusinessCards(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, cards)
}

func (s *Server) login(c echo.Context) error {
	req := new(auth.LoginReq)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	token, err := s.auth.Login(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, token)
}

func (s *Server) refreshToken(c echo.Context) error {
	req := new(auth.NewTokenReq)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	token, err := s.auth.RefreshToken(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, token)
}

func (s *Server) authProfile(c echo.Context) error {
	ctx := c.Request().Context()
	profile, err := s.auth.Profile(ctx)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"profile": profile,
	})
}

func (s *Server) approveBusinessCard(c echo.Context) error {
	req := new(card.ApproveBusinessCardReq)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	card, err := s.card.ApproveBusinessCard(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"card": card,
	})
}

func (s *Server) rejectBusinessCard(c echo.Context) error {
	req := new(card.RejectBusinessCardReq)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	card, err := s.card.RejectBusinessCard(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"card": card,
	})
}

func (s *Server) publishBusinessCard(c echo.Context) error {
	req := new(card.PublishBusinessCardReq)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	card, err := s.card.PublishBusinessCard(ctx, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"card": card,
	})
}

func (s *Server) getMyApprovalBusinessCardByID(c echo.Context) error {
	req := new(card.CardQuery)
	if err := c.Bind(req); err != nil {
		return badJSON()
	}

	ctx := c.Request().Context()
	card, err := s.card.GetMyApprovalBusinessCardByID(ctx, req.ID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"card": card,
	})
}
