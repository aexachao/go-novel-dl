package auth

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	store      *Store
	jwtManager *JWTManager
}

func NewHandler(store *Store, jwtManager *JWTManager) *Handler {
	return &Handler{store: store, jwtManager: jwtManager}
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token    string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresAt int64  `json:"expires_at"`
	User     UserResponse `json:"user"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Plan  Plan   `json:"plan"`
}

type APIKeyResponse struct {
	Key       string    `json:"key"`
	KeyID     string    `json:"key_id"`
	CreatedAt time.Time `json:"created_at"`
	Note      string    `json:"note"`
}

type QuotaResponse struct {
	Search       QuotaCounter `json:"search"`
	Download     QuotaCounter  `json:"download"`
	Plan         Plan         `json:"plan"`
	Limits       QuotaLimits  `json:"limits"`
}

type QuotaCounter struct {
	Used     int       `json:"used"`
	Limit    int       `json:"limit"`
	ResetAt  time.Time `json:"reset_at"`
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Check if user exists
	existing, _ := h.store.GetUserByEmail(req.Email)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被注册"})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user, err := h.store.CreateUser(req.Email, hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user: " + err.Error()})
		return
	}

	token, err := h.jwtManager.GenerateAccessToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		Token:     token,
		TokenType: "access",
		ExpiresAt: time.Now().Add(JWTTokenExpiration).Unix(),
		User: UserResponse{
			ID:    user.ID,
			Email: user.Email,
			Plan:  user.Plan,
		},
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := h.store.GetUserByEmail(req.Email)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	if !CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	token, err := h.jwtManager.GenerateAccessToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token:     token,
		TokenType: "access",
		ExpiresAt: time.Now().Add(JWTTokenExpiration).Unix(),
		User: UserResponse{
			ID:    user.ID,
			Email: user.Email,
			Plan:  user.Plan,
		},
	})
}

func (h *Handler) GetMe(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	quota, _ := h.store.GetQuota(userID)
	limits := GetLimits(user.Plan)

	resp := QuotaResponse{
		Plan:   user.Plan,
		Limits: limits,
	}
	if quota != nil {
		resp.Search = QuotaCounter{
			Used:    quota.SearchCount,
			Limit:   limits.DailySearch,
			ResetAt: quota.SearchResetAt,
		}
		resp.Download = QuotaCounter{
			Used:    quota.DownloadCount,
			Limit:   limits.DailyDownload,
			ResetAt: quota.DownloadResetAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  UserResponse{ID: user.ID, Email: user.Email, Plan: user.Plan},
		"quota": resp,
	})
}

func (h *Handler) CreateAPIKey(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	keyID, rawKey := GenerateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}

	keyHash := HashAPIKey(rawKey)
	if err := h.store.StoreAPIKey(user.ID, keyID, keyHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store api key"})
		return
	}

	// Return the raw key only once - it cannot be retrieved again
	c.JSON(http.StatusCreated, APIKeyResponse{
		Key:       rawKey,
		KeyID:     keyID,
		CreatedAt: time.Now(),
		Note:      "请妥善保管此 Key，刷新页面后将无法再次显示",
	})
}

func (h *Handler) ListAPIKeys(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	records, err := h.store.ListAPIKeys(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list api keys"})
		return
	}

	keys := make([]APIKeyResponse, 0, len(records))
	for _, r := range records {
		keys = append(keys, APIKeyResponse{
			Key:       "", // Never return the actual key
			KeyID:     r.KeyID,
			CreatedAt: r.CreatedAt,
			Note:      "Key 仅在创建时显示一次",
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": keys})
}

func (h *Handler) DeleteAPIKey(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	keyID := c.Param("key_id")

	if userID == "" || keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key_id is required"})
		return
	}

	if err := h.store.DeleteAPIKey(keyID, userID); err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete api key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API Key 已删除"})
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "go-novel-dl-auth"})
}
