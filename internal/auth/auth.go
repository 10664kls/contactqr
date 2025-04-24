package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"aidanwoods.dev/go-paseto"
	sq "github.com/Masterminds/squirrel"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	edPb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	rpcStatus "google.golang.org/grpc/status"
)

var ErrUserNotFound = errors.New("user not found")

type Auth struct {
	db   *sql.DB
	aKey paseto.V4SymmetricKey
	rKey paseto.V4SymmetricKey
	zlog *zap.Logger
}

func NewAuth(_ context.Context, db *sql.DB, aKey, rKey paseto.V4SymmetricKey, zlog *zap.Logger) (*Auth, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}
	if zlog == nil {
		return nil, errors.New("zlog is nil")
	}

	return &Auth{
		db:   db,
		aKey: aKey,
		rKey: rKey,
		zlog: zlog,
	}, nil
}

func (s *Auth) Profile(ctx context.Context) (*User, error) {
	zlog := s.zlog.With(
		zap.String("method", "Profile"),
	)

	claims := ClaimsFromContext(ctx)
	user, err := getUserByUsername(ctx, s.db, claims.Code)
	if errors.Is(err, ErrUserNotFound) {
		zlog.Info("failed to get user", zap.Error(err))
		return nil, rpcStatus.Error(codes.PermissionDenied, "Your are not allowed to access this user or (it may not exist)")
	}
	if err != nil {
		zlog.Error("failed to get user", zap.Error(err))
		return nil, err
	}

	return user, nil
}

func (s *Auth) Login(ctx context.Context, in *LoginReq) (*Token, error) {
	zlog := s.zlog.With(
		zap.String("method", "Login"),
	)

	if err := in.Validate(); err != nil {
		return nil, err
	}

	user, err := getUserByUsername(ctx, s.db, in.Username)
	if errors.Is(err, ErrUserNotFound) {
		zlog.Info("failed to get user", zap.Error(err))
		return nil, rpcStatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check your username and password and try again.")
	}
	if err != nil {
		zlog.Error("failed to get user", zap.Error(err))
		return nil, err
	}

	if passed, err := user.Compare(in.Password); err != nil || !passed {
		zlog.Info("failed to compare password", zap.Error(err))
		return nil, rpcStatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check your username and password and try again.")
	}

	token, err := s.genToken(user)
	if err != nil {
		zlog.Error("failed to generate token", zap.Error(err))
		return nil, err
	}

	return token, nil
}

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r *LoginReq) Validate() error {
	violations := make([]*edPb.BadRequest_FieldViolation, 0)

	r.Username = strings.TrimSpace(r.Username)
	if r.Username == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "username",
			Description: "username must not be empty",
		})
	}

	r.Password = strings.TrimSpace(r.Password)
	if r.Password == "" {
		violations = append(violations, &edPb.BadRequest_FieldViolation{
			Field:       "password",
			Description: "password must not be empty",
		})
	}

	if len(violations) > 0 {
		s, _ := rpcStatus.New(
			codes.InvalidArgument,
			"Credentials are not valid or incomplete. Please check the errors and try again, see details for more information.",
		).WithDetails(&edPb.BadRequest{FieldViolations: violations})
		return s.Err()
	}

	return nil
}

type NewTokenReq struct {
	Token string `json:"token"`
}

func (s *Auth) RefreshToken(ctx context.Context, in *NewTokenReq) (*Token, error) {
	zlog := s.zlog.With(
		zap.String("method", "RefreshToken"),
		zap.Any("req", in),
	)

	rules := []paseto.Rule{
		paseto.NotExpired(),
		paseto.ValidAt(time.Now()),
	}

	parser := paseto.MakeParser(rules)
	t, err := parser.ParseV4Local(s.rKey, in.Token, nil)
	if err != nil {
		zlog.Info("failed to parse token", zap.Error(err))
		return nil, rpcStatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check your token and try again.")
	}

	claims := new(Claims)
	if err := t.Get("profile", claims); err != nil {
		zlog.Info("failed to get claims", zap.Error(err))
		return nil, rpcStatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check your token and try again.")
	}

	u, err := getUserByUsername(ctx, s.db, claims.Code)
	if errors.Is(err, ErrUserNotFound) {
		zlog.Info("failed to get user by username", zap.Error(err))
		return nil, rpcStatus.Error(codes.Unauthenticated, "Your credentials not valid. Please check your token and try again.")
	}
	if err != nil {
		zlog.Error("failed to get user by username", zap.Error(err))
		return nil, err
	}

	token, err := s.genToken(u)
	if err != nil {
		zlog.Error("failed to generate token", zap.Error(err))
		return nil, err
	}

	return token, nil
}

type Token struct {
	Access  string `json:"accessToken"`
	Refresh string `json:"refreshToken"`
}

func (s *Auth) genToken(u *User) (*Token, error) {
	now := time.Now()

	t := paseto.NewToken()
	t.SetSubject(u.Code)
	t.SetIssuedAt(now)
	t.SetNotBefore(now)
	t.SetExpiration(now.Add(time.Hour))
	t.SetFooter([]byte(now.Format(time.RFC3339)))

	if err := t.Set("profile", &Claims{
		ID:           u.ID,
		Code:         u.Code,
		DisplayName:  u.DisplayName,
		ManagerID:    u.managerID,
		PositionID:   u.positionID,
		DepartmentID: u.departmentID,
		CompanyID:    u.companyID,
		Email:        u.email,
		Phone:        u.phone,
		Mobile:       u.mobile,
	}); err != nil {
		return nil, fmt.Errorf("failed to set claims: %w", err)
	}

	accessToken := t.V4Encrypt(s.aKey, nil)

	t.SetExpiration(now.Add(time.Hour * 24 * 7))
	refreshToken := t.V4Encrypt(s.rKey, nil)

	return &Token{
		Access:  accessToken,
		Refresh: refreshToken,
	}, nil
}

type Claims struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	DisplayName  string `json:"displayName"`
	ManagerID    string `json:"managerId"`
	PositionID   string `json:"positionId"`
	DepartmentID string `json:"departmentId"`
	CompanyID    string `json:"companyId"`
	Email        string `json:"emailAddress"`
	Phone        string `json:"phoneNumber"`
	Mobile       string `json:"mobileNumber"`
}

type ctxKey int

const (
	claimsKey ctxKey = iota
)

func ClaimsFromContext(ctx context.Context) *Claims {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	if !ok {
		return &Claims{}
	}
	return claims
}

func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

type User struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	DisplayName string `json:"displayName"`

	managerID    string
	positionID   string
	departmentID string
	companyID    string
	email        string
	phone        string
	mobile       string
	password     string
}

func (u *User) Compare(password string) (bool, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}

	return bcrypt.CompareHashAndPassword(hashed, []byte(password)) == nil, nil
}

func getUserByUsername(ctx context.Context, db *sql.DB, username string) (*User, error) {
	q, args := sq.
		Select(
			"TOP 1 e.EID",
			"u.username",
			"CONCAT(e.nameeng, ' ', e.surnameeng) AS display_name",
			"e.mgrid",
			"e.bid",
			"e.depid",
			"e.poid",
			"e.Emails",
			"e.phone_number",
			"e.mobile_number",
			"u.tokenkey",
		).
		From("dbo.tb_userlogin AS u").
		InnerJoin("dbo.vm_employee AS e ON u.eid = e.EID").
		Where(
			sq.Eq{
				"u.username": username,
			},
		).
		PlaceholderFormat(sq.AtP).
		MustSql()

	row := db.QueryRowContext(ctx, q, args...)

	var u User
	err := row.Scan(
		&u.ID,
		&u.Code,
		&u.DisplayName,
		&u.managerID,
		&u.companyID,
		&u.positionID,
		&u.departmentID,
		&u.email,
		&u.phone,
		&u.mobile,
		&u.password,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}
