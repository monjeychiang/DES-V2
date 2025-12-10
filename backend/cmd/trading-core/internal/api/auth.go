package api

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"trading-core/pkg/db"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const userContextKey = "UserID"

// UserClaims represents JWT claims for authenticated users.
type UserClaims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func checkPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func generateToken(userID, secret string, expiresAt time.Time) (string, error) {
	claims := UserClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func parseToken(tokenStr, secret string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims.UserID, nil
	}
	return "", errors.New("invalid token claims")
}

// AuthMiddleware enforces JWT auth for protected routes.
func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":  "MISSING_TOKEN",
				"error": "missing Authorization header",
			})
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":  "INVALID_AUTH_HEADER",
				"error": "invalid Authorization header",
			})
			return
		}

		userID, err := parseToken(parts[1], secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":  "INVALID_TOKEN",
				"error": "invalid or expired token",
			})
			return
		}

		c.Set(userContextKey, userID)
		c.Next()
	}
}

// CurrentUserID returns the authenticated user ID from context.
func CurrentUserID(c *gin.Context) string {
	if v, ok := c.Get(userContextKey); ok {
		if id, okCast := v.(string); okCast {
			return id
		}
	}
	return ""
}

// registerUser handles user registration.
func (s *Server) registerUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  "INVALID_PAYLOAD",
			"error": "invalid request payload",
		})
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Username = strings.TrimSpace(req.Username)
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  "MISSING_CREDENTIALS",
			"error": "email and password are required",
		})
		return
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  "INVALID_EMAIL",
			"error": "invalid email format",
		})
		return
	}

	ctx := c.Request.Context()
	existing, err := s.DB.GetUserByEmail(ctx, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  "INTERNAL_ERROR",
			"error": err.Error(),
		})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":  "EMAIL_ALREADY_REGISTERED",
			"error": "email already registered",
		})
		return
	}

	pwHash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  "INTERNAL_ERROR",
			"error": "failed to hash password",
		})
		return
	}

	now := time.Now()
	user := db.User{
		ID:           uuid.NewString(),
		Email:        req.Email,
		PasswordHash: pwHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.DB.CreateUser(ctx, user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  "INTERNAL_ERROR",
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id":  user.ID,
		"username": req.Username,
	})
}

// loginUser handles user login.
func (s *Server) loginUser(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  "INVALID_PAYLOAD",
			"error": "invalid request payload",
		})
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":  "MISSING_CREDENTIALS",
			"error": "email and password are required",
		})
		return
	}

	ctx := c.Request.Context()
	user, err := s.DB.GetUserByEmail(ctx, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  "INTERNAL_ERROR",
			"error": err.Error(),
		})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":  "INVALID_CREDENTIALS",
			"error": "invalid credentials",
		})
		return
	}

	if err := checkPassword(user.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":  "INVALID_CREDENTIALS",
			"error": "invalid credentials",
		})
		return
	}

	expiresAt := time.Now().Add(72 * time.Hour)
	token, err := generateToken(user.ID, s.JWTSecret, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":  "INTERNAL_ERROR",
			"error": "failed to generate token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"expires_at":  expiresAt.UTC().Format(time.RFC3339),
		"user_id":     user.ID,
		"user_email":  user.Email,
	})
}
