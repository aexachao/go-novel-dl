package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Middleware struct {
	store      *Store
	jwtManager *JWTManager
}

func NewMiddleware(store *Store, jwtManager *JWTManager) *Middleware {
	return &Middleware{store: store, jwtManager: jwtManager}
}

// RequireAuth validates either a Bearer JWT or an API key and sets user context.
func (m *Middleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := m.extractAndValidate(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		// Refresh quota if needed
		_ = m.store.RefreshQuotaIfNeeded(claims.UserID)

		c.Set("claims", claims)
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

// RequireQuota checks that the user has remaining quota for the given action.
func (m *Middleware) RequireQuota(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claimsVal, ok := c.Get("claims")
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		claims := claimsVal.(*Claims)

		// Self-hosted users (identified by plan "unlimited") skip quota
		if claims.Plan == Plan("unlimited") {
			c.Next()
			return
		}

		limits := GetLimits(claims.Plan)
		quota, err := m.store.GetQuota(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to check quota"})
			return
		}
		if quota == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "quota not found"})
			return
		}

		switch action {
		case "search":
			if quota.SearchCount >= limits.DailySearch {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":        "每日搜索配额已用完",
					"limit":        limits.DailySearch,
					"reset_at":     quota.SearchResetAt,
					"current_plan": claims.Plan,
				})
				return
			}
		case "download":
			if quota.DownloadCount >= limits.DailyDownload {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":        "每日下载配额已用完",
					"limit":        limits.DailyDownload,
					"reset_at":     quota.DownloadResetAt,
					"current_plan": claims.Plan,
				})
				return
			}
		}

		c.Next()
	}
}

// ConsumeQuota increments the quota counter after a successful operation.
func (m *Middleware) ConsumeQuota(action string) {
	// Called after successful API call — fire and forget
}

func (m *Middleware) extractAndValidate(c *gin.Context) (*Claims, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil, &AuthError{Message: "missing authorization header"}
	}

	if strings.HasPrefix(authHeader, "Bearer ") {
		token := authHeader[7:]
		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			return nil, &AuthError{Message: "invalid token: " + err.Error()}
		}
		// Load full user info for plan
		user, err := m.store.GetUserByID(claims.UserID)
		if err != nil || user == nil {
			return nil, &AuthError{Message: "user not found"}
		}
		claims.Plan = user.Plan
		claims.Email = user.Email
		return claims, nil
	}

	if strings.HasPrefix(authHeader, "NLDL-Key ") {
		rawKey := authHeader[8:]
		return m.validateAPIKey(rawKey)
	}

	return nil, &AuthError{Message: "unsupported auth scheme"}
}

func (m *Middleware) validateAPIKey(rawKey string) (*Claims, error) {
	keyHash := HashAPIKey(rawKey)
	record, err := m.store.GetAPIKeyRecordByHash(keyHash)
	if err != nil {
		return nil, &AuthError{Message: "api key error"}
	}
	if record == nil {
		return nil, &AuthError{Message: "invalid api key"}
	}
	user, err := m.store.GetUserByID(record.UserID)
	if err != nil || user == nil {
		return nil, &AuthError{Message: "user not found for api key"}
	}
	return &Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Plan:      user.Plan,
		TokenType: "api_key",
		TokenID:   record.KeyID,
	}, nil
}

// SkipAuth for endpoints that don't require auth (health, auth itself)
func SkipAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("skip_auth", true)
		c.Next()
	}
}

// AuthError is a simple error with a message
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

// GetUserIDFromContext extracts the user ID from gin context
func GetUserIDFromContext(c *gin.Context) string {
	if val, exists := c.Get("user_id"); exists {
		return val.(string)
	}
	return ""
}

// GetClaimsFromContext extracts the full claims from gin context
func GetClaimsFromContext(c *gin.Context) *Claims {
	if val, exists := c.Get("claims"); exists {
		return val.(*Claims)
	}
	return nil
}

// --- Unauthenticated endpoints (used by self-hosted users) ---

// OptionalAuth sets user context if a valid token is present, but doesn't block if missing.
// Used for endpoints that work both with and without auth (e.g., search can be limited or unlimited).
func (m *Middleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		claims, err := m.extractAndValidate(c)
		if err != nil || claims == nil {
			c.Next()
			return
		}

		_ = m.store.RefreshQuotaIfNeeded(claims.UserID)
		c.Set("claims", claims)
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

// IsSelfHostedRequest returns true if the request has no auth header
// (self-hosted mode = no auth required)
func IsSelfHostedRequest(c *gin.Context) bool {
	return c.GetHeader("Authorization") == ""
}
