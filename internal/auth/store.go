package auth

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/glebarez/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open auth db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping auth db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate auth db: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		plan TEXT NOT NULL DEFAULT 'free',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS api_keys (
		id TEXT PRIMARY KEY,
		key_id TEXT UNIQUE NOT NULL,
		key_hash TEXT NOT NULL,
		user_id TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	CREATE TABLE IF NOT EXISTS quotas (
		user_id TEXT PRIMARY KEY,
		search_count INTEGER NOT NULL DEFAULT 0,
		search_reset_at DATETIME NOT NULL,
		download_count INTEGER NOT NULL DEFAULT 0,
		download_reset_at DATETIME NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	CREATE INDEX IF NOT EXISTS idx_api_keys_key_id ON api_keys(key_id);
	CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
	`
	_, err := s.db.Exec(schema)
	return err
}

// --- User operations ---

func (s *Store) CreateUser(email, passwordHash string) (*User, error) {
	id := generateID()
	now := time.Now()
	_, err := s.db.Exec(
		`INSERT INTO users (id, email, password_hash, plan, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, email, passwordHash, PlanFree, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	user := &User{ID: id, Email: email, Plan: PlanFree, CreatedAt: now, UpdatedAt: now}
	if err := s.initQuota(user.ID); err != nil {
		return nil, err
	}
	return user, nil
}

// CreateGuestUser creates a guest account with a placeholder password hash.
// The account is only accessible via pre-generated JWT (no password login).
func (s *Store) CreateGuestUser(email string) (*User, error) {
	id := generateID()
	now := time.Now()
	// Placeholder hash — guest account cannot be logged into with a password
	placeholderHash := "$2a$10$guest.account.placeholder.hash.for.token.only"
	_, err := s.db.Exec(
		`INSERT INTO users (id, email, password_hash, plan, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, email, placeholderHash, PlanFree, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert guest user: %w", err)
	}
	user := &User{ID: id, Email: email, Plan: PlanFree, CreatedAt: now, UpdatedAt: now}
	if err := s.initQuota(user.ID); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) GetUserByEmail(email string) (*User, error) {
	row := s.db.QueryRow(`SELECT id, email, password_hash, plan, created_at, updated_at FROM users WHERE email = ?`, email)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Plan, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (s *Store) GetUserByID(id string) (*User, error) {
	row := s.db.QueryRow(`SELECT id, email, password_hash, plan, created_at, updated_at FROM users WHERE id = ?`, id)
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Plan, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (s *Store) UpdateUserPlan(userID string, plan Plan) error {
	_, err := s.db.Exec(`UPDATE users SET plan = ?, updated_at = ? WHERE id = ?`, plan, time.Now(), userID)
	return err
}

// --- API Key operations ---

func (s *Store) StoreAPIKey(userID, keyID, keyHash string) error {
	id := generateID()
	_, err := s.db.Exec(
		`INSERT INTO api_keys (id, key_id, key_hash, user_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, keyID, keyHash, userID, time.Now(),
	)
	return err
}

func (s *Store) GetAPIKeyRecordByKeyID(keyID string) (*APIKeyRecord, error) {
	row := s.db.QueryRow(`SELECT id, key_id, key_hash, user_id, created_at FROM api_keys WHERE key_id = ?`, keyID)
	var r APIKeyRecord
	err := row.Scan(&r.ID, &r.KeyID, &r.KeyHash, &r.UserID, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api key record: %w", err)
	}
	return &r, nil
}

func (s *Store) GetAPIKeyRecordByHash(keyHash string) (*APIKeyRecord, error) {
	row := s.db.QueryRow(`SELECT id, key_id, key_hash, user_id, created_at FROM api_keys WHERE key_hash = ?`, keyHash)
	var r APIKeyRecord
	err := row.Scan(&r.ID, &r.KeyID, &r.KeyHash, &r.UserID, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}
	return &r, nil
}

func (s *Store) ListAPIKeys(userID string) ([]APIKeyRecord, error) {
	rows, err := s.db.Query(`SELECT id, key_id, key_hash, user_id, created_at FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []APIKeyRecord
	for rows.Next() {
		var r APIKeyRecord
		if err := rows.Scan(&r.ID, &r.KeyID, &r.KeyHash, &r.UserID, &r.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, r)
	}
	return keys, nil
}

func (s *Store) DeleteAPIKey(keyID, userID string) error {
	_, err := s.db.Exec(`DELETE FROM api_keys WHERE key_id = ? AND user_id = ?`, keyID, userID)
	return err
}

// --- Quota operations ---

func (s *Store) initQuota(userID string) error {
	now := time.Now()
	reset := now.Add(24 * time.Hour).Truncate(24 * time.Hour)
	_, err := s.db.Exec(
		`INSERT INTO quotas (user_id, search_count, search_reset_at, download_count, download_reset_at) VALUES (?, 0, ?, 0, ?)`,
		userID, reset, reset,
	)
	return err
}

func (s *Store) GetQuota(userID string) (*Quota, error) {
	row := s.db.QueryRow(`SELECT user_id, search_count, search_reset_at, download_count, download_reset_at FROM quotas WHERE user_id = ?`, userID)
	var q Quota
	err := row.Scan(&q.UserID, &q.SearchCount, &q.SearchResetAt, &q.DownloadCount, &q.DownloadResetAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (s *Store) RefreshQuotaIfNeeded(userID string) error {
	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE quotas
		SET search_count = CASE WHEN search_reset_at <= ? THEN 0 ELSE search_count END,
		    search_reset_at = CASE WHEN search_reset_at <= ? THEN ? ELSE search_reset_at END,
		    download_count = CASE WHEN download_reset_at <= ? THEN 0 ELSE download_count END,
		    download_reset_at = CASE WHEN download_reset_at <= ? THEN ? ELSE download_reset_at END
		WHERE user_id = ?`,
		now, now, now.Add(24*time.Hour).Truncate(24*time.Hour),
		now, now, now.Add(24*time.Hour).Truncate(24*time.Hour),
		userID,
	)
	return err
}

func (s *Store) IncrementSearch(userID string) error {
	return s.incrementCounter(userID, "search")
}

func (s *Store) IncrementDownload(userID string) error {
	return s.incrementCounter(userID, "download")
}

func (s *Store) incrementCounter(userID, counter string) error {
	_, err := s.db.Exec(
		fmt.Sprintf(`UPDATE quotas SET %s_count = %s_count + 1 WHERE user_id = ?`, counter, counter),
		userID,
	)
	return err
}

// --- Helpers ---

func generateID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano()/1e6, randomString(8))
}
