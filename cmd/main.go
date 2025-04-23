package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpPb "github.com/10664kls/contactqr/genproto/go/http/v1"
	"github.com/10664kls/contactqr/internal/employee"
	"github.com/10664kls/contactqr/internal/server"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/labstack/echo/v4"
	stdmw "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	_ "github.com/denisenkom/go-mssqldb"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	zlog, err := newLogger()
	if err != nil {
		return err
	}
	zap.ReplaceGlobals(zlog)

	db, err := sql.Open(
		"sqlserver",
		fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s&TrustServerCertificate=true",
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
			os.Getenv("DB_NAME"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create db connection: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping DB: %w", err)
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(httpLogger(zlog))
	e.Use(stdMws()...)
	e.HTTPErrorHandler = httpErr

	employeeService := must(employee.NewService(ctx, db, zlog))

	server := must(server.NewServer(employeeService))
	if err := server.Install(e); err != nil {
		return fmt.Errorf("failed to install server: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- e.Start(fmt.Sprintf(":%s", getEnv("PORT", "8089")))
	}()

	ctx, cancel = signal.NotifyContext(ctx, os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancel()

	select {
	case <-ctx.Done():
		zlog.Info("shutting down server")

		ctx, cancel := context.WithTimeout(ctx, time.Second*15)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			zlog.Error("failed to shutdown server", zap.Error(err))
			return err
		}

		zlog.Info("server shut down gracefully")

	case err := <-errCh:
		if err != http.ErrServerClosed && err != nil {
			return err
		}
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func newLogger() (*zap.Logger, error) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      false,
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	zlog, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build zap log: %w", err)
	}
	defer zlog.Sync()

	return zlog, nil
}

func httpErr(err error, c echo.Context) {
	if s, ok := status.FromError(err); ok {
		he := httpStatusPbFromRPC(s)
		jsonb, _ := protojson.Marshal(he)
		c.JSONBlob(int(he.Error.Code), jsonb)
		return
	}

	if he, ok := err.(*echo.HTTPError); ok {
		var s *status.Status
		switch he.Code {
		case http.StatusNotFound:
			s = status.New(codes.NotFound, "Not found!")

		case http.StatusTooManyRequests:
			s = status.New(codes.ResourceExhausted, "Too many requests.")

		case http.StatusInternalServerError:
			s = status.New(codes.Internal, "An internal error occurred.")

		default:
			s = status.New(codes.Unknown, "Unknown error!")
		}

		hbp := httpStatusPbFromRPC(s)
		jsonb, _ := protojson.Marshal(hbp)
		c.JSONBlob(int(hbp.Error.Code), jsonb)
		return
	}

	c.JSON(http.StatusInternalServerError, echo.Map{
		"code":    500,
		"status":  "INTERNAL_ERROR",
		"message": "An internal error occurred",
	})
}

func stdMws() []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{
		stdmw.RemoveTrailingSlash(),
		stdmw.Recover(),
		stdmw.CORSWithConfig(stdmw.CORSConfig{
			AllowOriginFunc: func(origin string) (bool, error) {
				return true, nil
			},
			AllowMethods: []string{
				http.MethodHead,
				http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete,
				http.MethodOptions,
			},
			AllowCredentials: true,
			MaxAge:           86400,
		}),
		stdmw.RateLimiter(stdmw.NewRateLimiterMemoryStore(10)),
		stdmw.Secure(),
	}
}

func httpLogger(zlog *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()

			fields := []zapcore.Field{
				zap.String("remote_ip", c.RealIP()),
				zap.String("latency", time.Since(start).String()),
				zap.String("host", req.Host),
				zap.String("request", fmt.Sprintf("%s %s", req.Method, req.RequestURI)),
				zap.Int("status", res.Status),
				zap.Int64("size", res.Size),
				zap.String("user_agent", req.UserAgent()),
			}

			id := req.Header.Get(echo.HeaderXRequestID)
			if id != "" {
				fields = append(fields, zap.String("request_id", id))
			}

			n := res.Status
			switch {
			case n >= 500:
				zlog.With(zap.Error(err)).Error("HTTP Error", fields...)

			case n >= 400:
				zlog.With(zap.Error(err)).Warn("Client Error", fields...)

			case n >= 300:
				zlog.Info("Redirection", fields...)

			default:
				zlog.Info("Success", fields...)
			}

			return nil
		}
	}
}

func httpStatusPbFromRPC(s *status.Status) *httpPb.Error {
	return &httpPb.Error{
		Error: &httpPb.Status{
			Code:    int32(runtime.HTTPStatusFromCode(s.Code())),
			Message: s.Message(),
			Status:  code.Code(s.Code()),
			Details: s.Proto().GetDetails(),
		},
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
