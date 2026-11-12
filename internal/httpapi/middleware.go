package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const claimsContextKey = "auth_claims"

func (h *Handler) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		scheme, tokenString, ok := strings.Cut(header, " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || tokenString == "" {
			respondError(c, http.StatusUnauthorized, "bearer token required")
			return
		}

		claims := &authClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return h.config.JWTSecret, nil
		})
		if err != nil || !token.Valid {
			respondError(c, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			respondError(c, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		claims.userID = userID
		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

func (h *Handler) requireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := claimsFromContext(c)
		var role string
		err := h.db.QueryRow(c.Request.Context(),
			`SELECT role FROM users WHERE id = $1`, claims.userID,
		).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(c, http.StatusUnauthorized, "user not found")
			return
		}
		if err != nil {
			respondError(c, http.StatusInternalServerError, "could not verify administrator role")
			return
		}
		if role != "super_admin" {
			respondError(c, http.StatusForbidden, "super administrator access required")
			return
		}
		c.Next()
	}
}

func claimsFromContext(c *gin.Context) *authClaims {
	claims, _ := c.Get(claimsContextKey)
	return claims.(*authClaims)
}
