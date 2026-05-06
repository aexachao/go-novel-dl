package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	JWTTokenExpiration = 7 * 24 * time.Hour
	JWTLIssuer        = "go-novel-dl"
)

type Claims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Plan      Plan   `json:"plan"`
	TokenType string `json:"token_type"` // "access" or "api_key"
	TokenID   string `json:"token_id,omitempty"`
	jwtBase
}

type jwtBase struct {
	IssuedAt  int64 `json:"iat"`
	ExpiresAt int64 `json:"exp"`
	Issuer    string `json:"iss"`
}

func (c *Claims) IsAPIKey() bool   { return c.TokenType == "api_key" }
func (c *Claims) IsAccessToken() bool { return c.TokenType == "access" }
func (c *Claims) IsExpired() bool    { return time.Now().Unix() > c.ExpiresAt }

type JWTManager struct {
	secret []byte
	issuer string
}

func NewJWTManager(secret string) *JWTManager {
	if secret == "" {
		secret = "go-novel-dl-default-secret-change-in-production"
	}
	h := sha256.Sum256([]byte(secret))
	return &JWTManager{secret: h[:], issuer: JWTLIssuer}
}

func (m *JWTManager) GenerateAccessToken(user *User) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Plan:      user.Plan,
		TokenType: "access",
		jwtBase: jwtBase{
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(JWTTokenExpiration).Unix(),
			Issuer:    m.issuer,
		},
	}
	return m.sign(claims)
}

func (m *JWTManager) ValidateToken(token string) (*Claims, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Verify signature
	sig := computeHMAC256(parts[0]+"."+parts[1], m.secret)
	expectedSig := base64URLEncode(sig)
	if !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
		return nil, fmt.Errorf("invalid signature")
	}

	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	if claims.IsExpired() {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

func (m *JWTManager) sign(claims *Claims) (string, error) {
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64URLEncode(payloadBytes)
	sig := computeHMAC256(header+"."+payload, m.secret)
	signature := base64URLEncode(sig)
	return header + "." + payload + "." + signature, nil
}

func computeHMAC256(data string, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// --- Password hashing ---

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// APIKey generates a new random API key and returns raw key + key ID
func GenerateAPIKey() (keyID, rawKey string) {
	keyID = uuid.New().String()
	rawKey = fmt.Sprintf("nldl_%s_%s", keyID, randomString(32))
	return
}

func HashAPIKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return base64URLEncode(h[:])
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	r := time.Now().UnixNano()
	for i := range b {
		b[i] = letters[int(r+int64(i)*31)%len(letters)]
	}
	return string(b)
}
