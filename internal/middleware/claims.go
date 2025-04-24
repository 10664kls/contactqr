package middleware

import (
	"context"

	"aidanwoods.dev/go-paseto"
	"github.com/10664kls/contactqr/internal/auth"
	"github.com/labstack/echo/v4"
)

func SetContextClaimsFromToken(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token, ok := c.Get("token").(*paseto.Token)
		if !ok {
			return next(c)
		}

		savedReq := c.Request()
		savedCtx := contextClaimsFromToken(savedReq.Context(), token)
		newReq := savedReq.WithContext(savedCtx)
		c.SetRequest(newReq)

		return next(c)
	}
}

func contextClaimsFromToken(ctx context.Context, token *paseto.Token) context.Context {
	return auth.ContextWithClaims(ctx, parseTokenToClaims(token))
}

func parseTokenToClaims(token *paseto.Token) *auth.Claims {
	if token == nil {
		return &auth.Claims{}
	}

	claims := new(auth.Claims)
	token.Get("profile", claims)

	return claims
}
