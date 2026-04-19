package handlers

import (
	"database/sql"
	"fmt"
	"sync"
)

var ensureActivityLogSchemaOnce sync.Once
var ensureActivityLogSchemaErr error

// EnsureActivityLogSchema memastikan tabel activity_logs tersedia dan kompatibel.
func EnsureActivityLogSchema(db *sql.DB) error {
	ensureActivityLogSchemaOnce.Do(func() {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS activity_logs (
				id BIGSERIAL PRIMARY KEY,
				admin_id INT REFERENCES admins(id) ON DELETE SET NULL,
				admin_nama TEXT NOT NULL DEFAULT 'System',
				action TEXT NOT NULL,
				entity_type TEXT,
				entity_id INT,
				entity_name TEXT,
				details JSONB,
				created_at TIMESTAMP NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_activity_logs_created_at ON activity_logs(created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_activity_logs_action ON activity_logs(action)`,
			`CREATE INDEX IF NOT EXISTS idx_activity_logs_entity_type ON activity_logs(entity_type)`,
			`CREATE INDEX IF NOT EXISTS idx_activity_logs_admin_id ON activity_logs(admin_id)`,
			`ALTER TABLE activity_logs ALTER COLUMN admin_id DROP NOT NULL`,
			`UPDATE activity_logs al
			   SET admin_id = NULL
			 WHERE admin_id IS NOT NULL
			   AND NOT EXISTS (SELECT 1 FROM admins a WHERE a.id = al.admin_id)`,
			`DO $$
			BEGIN
				IF EXISTS (
					SELECT 1
					FROM information_schema.table_constraints
					WHERE table_name = 'activity_logs'
					  AND constraint_name = 'activity_logs_admin_id_fkey'
					  AND constraint_type = 'FOREIGN KEY'
				) THEN
					ALTER TABLE activity_logs DROP CONSTRAINT activity_logs_admin_id_fkey;
				END IF;
			END
			$$`,
			`ALTER TABLE activity_logs
			   ADD CONSTRAINT activity_logs_admin_id_fkey
			   FOREIGN KEY (admin_id)
			   REFERENCES admins(id)
			   ON DELETE SET NULL`,
			`DO $$
			BEGIN
				IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'logs')
				   AND NOT EXISTS (SELECT 1 FROM activity_logs) THEN
					INSERT INTO activity_logs (admin_nama, action, entity_type, entity_id, entity_name, details, created_at)
					SELECT 'Legacy', COALESCE(action, 'UNKNOWN'), 'book', book_id, NULL,
						jsonb_build_object('legacy_detail', detail, 'legacy_user_id', user_id),
						COALESCE(created_at, NOW())
					FROM logs;
				END IF;
			END
			$$`,
		}

		for _, stmt := range statements {
			if _, err := db.Exec(stmt); err != nil {
				ensureActivityLogSchemaErr = fmt.Errorf("gagal memastikan skema activity logs: %w", err)
				return
			}
		}
	})

	return ensureActivityLogSchemaErr
}
