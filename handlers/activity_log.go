package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"pustaka-filsafat/models"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// LogActivity - helper untuk mencatat aktivitas
func LogActivity(db *sql.DB, adminID *int, adminNama, action, entityType string, entityID *int, entityName *string, details interface{}) error {
	if err := EnsureActivityLogSchema(db); err != nil {
		return err
	}

	if adminNama == "" {
		adminNama = "System"
	}

	// Be defensive against legacy/wrong admin IDs so logging never breaks business flow.
	if adminID != nil {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM admins WHERE id = $1)`, *adminID).Scan(&exists)
		if isInvalidStatementNameError(err) {
			err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM admins WHERE id = $1)`, *adminID).Scan(&exists)
		}
		if err != nil || !exists {
			adminID = nil
		}
	}

	var detailsJSON []byte
	var err error

	if details != nil {
		detailsJSON, err = json.Marshal(details)
		if err != nil {
			detailsJSON = nil
		}
	}

	_, err = db.Exec(`
		INSERT INTO activity_logs (admin_id, admin_nama, action, entity_type, entity_id, entity_name, details)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, adminID, adminNama, action, entityType, entityID, entityName, detailsJSON)
	if isInvalidStatementNameError(err) {
		_, err = db.Exec(`
			INSERT INTO activity_logs (admin_id, admin_nama, action, entity_type, entity_id, entity_name, details)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, adminID, adminNama, action, entityType, entityID, entityName, detailsJSON)
	}

	return err
}

// GetActivityLogs - ambil semua activity logs
func GetActivityLogs(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := EnsureActivityLogSchema(db); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Schema activity logs belum siap", "debug": err.Error()})
		}

		limit := c.QueryInt("limit", 50)
		offset := c.QueryInt("offset", 0)
		entityType := c.Query("entity_type")
		action := c.Query("action")

		query := `
			SELECT id, admin_id, admin_nama, action, entity_type, entity_id, entity_name, details, created_at
			FROM activity_logs
			WHERE 1=1
		`
		args := []interface{}{}
		argCount := 0

		if entityType != "" {
			argCount++
			query += fmt.Sprintf(" AND entity_type = $%d", argCount)
			args = append(args, entityType)
		}

		if action != "" {
			argCount++
			query += fmt.Sprintf(" AND action = $%d", argCount)
			args = append(args, action)
		}

		query += " ORDER BY created_at DESC"
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)

		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, offset)

		rows, err := db.Query(query, args...)
		if isInvalidStatementNameError(err) {
			rows, err = db.Query(query, args...)
			if isInvalidStatementNameError(err) {
				fallbackQuery := buildActivityLogsFallbackQuery(limit, offset, entityType, action)
				rows, err = db.Query(fallbackQuery)
			}
		}
		if err != nil {
			log.Printf("[GetActivityLogs] query error: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil logs", "debug": err.Error()})
		}
		defer rows.Close()

		var logs []models.ActivityLog
		for rows.Next() {
			var log models.ActivityLog
			var details sql.NullString
			err := rows.Scan(
				&log.ID, &log.AdminID, &log.AdminNama, &log.Action,
				&log.EntityType, &log.EntityID, &log.EntityName, &details, &log.CreatedAt,
			)
			if err != nil {
				continue
			}
			if details.Valid {
				log.Details = json.RawMessage(details.String)
			}
			logs = append(logs, log)
		}

		if err := rows.Err(); err != nil {
			log.Printf("[GetActivityLogs] rows error: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Gagal membaca logs", "debug": err.Error()})
		}

		if logs == nil {
			logs = []models.ActivityLog{}
		}

		return c.JSON(logs)
	}
}

func buildActivityLogsFallbackQuery(limit, offset int, entityType, action string) string {
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	quote := func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}

	var b strings.Builder
	b.WriteString(`
		SELECT id, admin_id, admin_nama, action, entity_type, entity_id, entity_name, details, created_at
		FROM activity_logs
		WHERE 1=1
	`)
	if entityType != "" {
		b.WriteString(" AND entity_type = ")
		b.WriteString(quote(entityType))
	}
	if action != "" {
		b.WriteString(" AND action = ")
		b.WriteString(quote(action))
	}
	b.WriteString(" ORDER BY created_at DESC LIMIT ")
	b.WriteString(strconv.Itoa(limit))
	b.WriteString(" OFFSET ")
	b.WriteString(strconv.Itoa(offset))

	return b.String()
}

// DebugLogPing - endpoint debug untuk validasi insert/read activity logs
func DebugLogPing(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := EnsureActivityLogSchema(db); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Schema activity_logs gagal dipastikan", "debug": err.Error()})
		}

		var totalBefore int
		if err := db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&totalBefore); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal membaca total logs", "debug": err.Error()})
		}

		var adminID *int
		adminNama := "System"
		if raw := c.Locals("adminID"); raw != nil {
			if id, ok := raw.(int); ok {
				adminID = &id
				_ = db.QueryRow("SELECT nama FROM admins WHERE id = $1", id).Scan(&adminNama)
				if adminNama == "" {
					adminNama = "Admin"
				}
			}
		}

		action := fmt.Sprintf("DEBUG_LOG_PING_%d", time.Now().Unix())
		details := fiber.Map{"source": "debug-endpoint", "note": "manual log verification"}
		if err := LogActivity(db, adminID, adminNama, action, models.EntityAdmin, adminID, nil, details); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Insert debug log gagal", "debug": err.Error()})
		}

		var totalAfter int
		if err := db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&totalAfter); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Insert berhasil tapi gagal membaca total logs", "debug": err.Error()})
		}

		var latestID int
		var latestAction string
		var latestAt string
		if err := db.QueryRow(`
			SELECT id, action, created_at::text
			FROM activity_logs
			ORDER BY id DESC
			LIMIT 1
		`).Scan(&latestID, &latestAction, &latestAt); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Insert berhasil tapi gagal baca row terbaru", "debug": err.Error()})
		}

		return c.JSON(fiber.Map{
			"message":       "Debug log ping berhasil",
			"total_before":  totalBefore,
			"total_after":   totalAfter,
			"latest_id":     latestID,
			"latest_action": latestAction,
			"latest_at":     latestAt,
		})
	}
}

// GetLogStats - statistik aktivitas
func GetLogStats(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := EnsureActivityLogSchema(db); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Schema activity logs belum siap", "debug": err.Error()})
		}

		stats := struct {
			TotalLogs    int            `json:"total_logs"`
			TodayLogs    int            `json:"today_logs"`
			ActionCounts map[string]int `json:"action_counts"`
			TopAdmins    []struct {
				AdminNama string `json:"admin_nama"`
				Count     int    `json:"count"`
			} `json:"top_admins"`
		}{
			ActionCounts: make(map[string]int),
		}

		// Total logs
		err := db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&stats.TotalLogs)
		if isInvalidStatementNameError(err) {
			err = db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&stats.TotalLogs)
		}

		// Today's logs
		err = db.QueryRow("SELECT COUNT(*) FROM activity_logs WHERE DATE(created_at) = CURRENT_DATE").Scan(&stats.TodayLogs)
		if isInvalidStatementNameError(err) {
			err = db.QueryRow("SELECT COUNT(*) FROM activity_logs WHERE DATE(created_at) = CURRENT_DATE").Scan(&stats.TodayLogs)
		}

		// Action counts
		rows, err := db.Query(`
			SELECT action, COUNT(*) as cnt FROM activity_logs 
			GROUP BY action ORDER BY cnt DESC
		`)
		if isInvalidStatementNameError(err) {
			rows, err = db.Query(`
				SELECT action, COUNT(*) as cnt FROM activity_logs 
				GROUP BY action ORDER BY cnt DESC
			`)
		}
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var action string
				var count int
				rows.Scan(&action, &count)
				stats.ActionCounts[action] = count
			}
		}

		// Top admins
		rows2, err := db.Query(`
			SELECT admin_nama, COUNT(*) as cnt FROM activity_logs 
			GROUP BY admin_nama ORDER BY cnt DESC LIMIT 5
		`)
		if isInvalidStatementNameError(err) {
			rows2, err = db.Query(`
				SELECT admin_nama, COUNT(*) as cnt FROM activity_logs 
				GROUP BY admin_nama ORDER BY cnt DESC LIMIT 5
			`)
		}
		if rows2 != nil {
			defer rows2.Close()
			for rows2.Next() {
				var admin struct {
					AdminNama string `json:"admin_nama"`
					Count     int    `json:"count"`
				}
				rows2.Scan(&admin.AdminNama, &admin.Count)
				stats.TopAdmins = append(stats.TopAdmins, admin)
			}
		}

		return c.JSON(stats)
	}
}
