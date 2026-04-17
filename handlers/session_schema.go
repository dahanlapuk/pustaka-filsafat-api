package handlers

import (
	"database/sql"
	"fmt"
	"sync"
)

var ensureSessionSchemaOnce sync.Once
var ensureSessionSchemaErr error

// EnsureAuthSessionSchema memastikan tabel dan index sesi auth tersedia.
func EnsureAuthSessionSchema(db *sql.DB) error {
	ensureSessionSchemaOnce.Do(func() {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS admin_sessions (
				id BIGSERIAL PRIMARY KEY,
				admin_id INT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
				token_hash TEXT NOT NULL UNIQUE,
				issued_at TIMESTAMP NOT NULL,
				expires_at TIMESTAMP NOT NULL,
				invalidated_at TIMESTAMP,
				created_at TIMESTAMP DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_admin_sessions_admin_active
				ON admin_sessions(admin_id, invalidated_at)`,
			`CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires_at
				ON admin_sessions(expires_at)`,
		}

		for _, stmt := range statements {
			if _, err := db.Exec(stmt); err != nil {
				ensureSessionSchemaErr = fmt.Errorf("gagal memastikan skema auth session: %w", err)
				return
			}
		}
	})

	return ensureSessionSchemaErr
}
