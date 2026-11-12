package httpapi

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

var (
	emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

type user struct {
	ID               uuid.UUID `json:"id"`
	Email            string    `json:"email"`
	PhoneNumber      string    `json:"phone_number"`
	FullName         *string   `json:"full_name,omitempty"`
	Role             string    `json:"role"`
	SubscriptionTier string    `json:"subscription_tier"`
	CreatedAt        time.Time `json:"created_at"`
}

type authClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
	userID uuid.UUID
}

type registerRequest struct {
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
	FullName    string `json:"full_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) register(c *gin.Context) {
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid JSON request")
		return
	}
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	request.PhoneNumber = strings.TrimSpace(request.PhoneNumber)
	request.FullName = strings.TrimSpace(request.FullName)
	if !emailPattern.MatchString(request.Email) || len(request.Email) > 255 {
		respondError(c, http.StatusBadRequest, "a valid email is required")
		return
	}
	if !phonePattern.MatchString(request.PhoneNumber) {
		respondError(c, http.StatusBadRequest, "phone_number must be in E.164 format")
		return
	}
	if len(request.Password) < 12 || len(request.Password) > 72 {
		respondError(c, http.StatusBadRequest, "password must be between 12 and 72 bytes")
		return
	}
	if len(request.FullName) > 100 {
		respondError(c, http.StatusBadRequest, "full_name must be at most 100 characters")
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not create account")
		return
	}

	var created user
	err = h.db.QueryRow(c.Request.Context(), `
		INSERT INTO users (email, phone_number, password_hash, full_name)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, email, phone_number, full_name, role, subscription_tier, created_at`,
		request.Email, request.PhoneNumber, string(passwordHash), request.FullName,
	).Scan(&created.ID, &created.Email, &created.PhoneNumber, &created.FullName, &created.Role,
		&created.SubscriptionTier, &created.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(c, http.StatusConflict, "email or phone_number is already registered")
			return
		}
		if errors.As(err, &pgErr) {
			log.Printf("account registration database error: SQLSTATE %s", pgErr.Code)
		} else {
			log.Printf("account registration database operation failed")
		}
		respondError(c, http.StatusInternalServerError, "could not create account")
		return
	}

	token, err := h.issueToken(created.ID, created.Role)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not create session")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": created, "token": token})
}

func (h *Handler) login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid JSON request")
		return
	}
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))

	var authenticated user
	var passwordHash string
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, email, phone_number, full_name, role, subscription_tier,
		       created_at, password_hash
		FROM users WHERE email = $1`, request.Email,
	).Scan(&authenticated.ID, &authenticated.Email, &authenticated.PhoneNumber,
		&authenticated.FullName, &authenticated.Role, &authenticated.SubscriptionTier,
		&authenticated.CreatedAt, &passwordHash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(request.Password)) != nil {
		respondError(c, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := h.issueToken(authenticated.ID, authenticated.Role)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not create session")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": authenticated, "token": token})
}

func (h *Handler) getMe(c *gin.Context) {
	claims := claimsFromContext(c)
	var current user
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, email, phone_number, full_name, role, subscription_tier, created_at
		FROM users WHERE id = $1`, claims.userID,
	).Scan(&current.ID, &current.Email, &current.PhoneNumber, &current.FullName,
		&current.Role, &current.SubscriptionTier, &current.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(c, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not load user")
		return
	}
	c.JSON(http.StatusOK, current)
}

func (h *Handler) issueToken(id uuid.UUID, role string) (string, error) {
	now := time.Now()
	claims := authClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.config.JWTTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.config.JWTSecret)
}
