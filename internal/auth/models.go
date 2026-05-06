package auth

import "time"

type Plan string

const (
	PlanFree = Plan("free")
	PlanPro  = Plan("pro")
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Plan         Plan      `json:"plan"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type APIKeyRecord struct {
	ID        string    `json:"id"`
	KeyID     string    `json:"key_id"`     // matches Claims.TokenID
	KeyHash   string    `json:"key_hash"`   // SHA256 of raw key
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Quota struct {
	UserID          string    `json:"user_id"`
	SearchCount     int       `json:"search_count"`
	SearchResetAt   time.Time `json:"search_reset_at"`
	DownloadCount   int       `json:"download_count"`
	DownloadResetAt time.Time `json:"download_reset_at"`
}

// QuotaLimits defines the limits for a given plan
type QuotaLimits struct {
	DailySearch    int
	DailyDownload  int
	MaxWorkers     int
	AllSites       bool // true = full site access, false = main sites only
}

var PlanLimits = map[Plan]QuotaLimits{
	PlanFree: {DailySearch: 50, DailyDownload: 5, MaxWorkers: 1, AllSites: false},
	PlanPro:  {DailySearch: 500, DailyDownload: 50, MaxWorkers: 3, AllSites: true},
}

func GetLimits(plan Plan) QuotaLimits {
	if limits, ok := PlanLimits[plan]; ok {
		return limits
	}
	return PlanLimits[PlanFree]
}
