package handlers

import (
	"database/sql"
	"encoding/json"
	"pustaka-filsafat/models"

	"github.com/gofiber/fiber/v2"
)

// LogActivity - helper untuk mencatat aktivitas
func LogActivity(db *sql.DB, adminID *int, adminNama, action, entityType string, entityID *int, entityName *string, details interface{}) error {
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

	return err
}

// GetActivityLogs - ambil semua activity logs
func GetActivityLogs(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
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
			query += " AND entity_type = $" + string(rune('0'+argCount))
			args = append(args, entityType)
		}

		if action != "" {
			argCount++
			query += " AND action = $" + string(rune('0'+argCount))
			args = append(args, action)
		}

		query += " ORDER BY created_at DESC"
		argCount++
		query += " LIMIT $" + string(rune('0'+argCount))
		args = append(args, limit)

		argCount++
		query += " OFFSET $" + string(rune('0'+argCount))
		args = append(args, offset)

		rows, err := db.Query(query, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil logs"})
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

		if logs == nil {
			logs = []models.ActivityLog{}
		}

		return c.JSON(logs)
	}
}

// GetLogStats - statistik aktivitas
func GetLogStats(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
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
		db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&stats.TotalLogs)

		// Today's logs
		db.QueryRow("SELECT COUNT(*) FROM activity_logs WHERE DATE(created_at) = CURRENT_DATE").Scan(&stats.TodayLogs)

		// Action counts
		rows, _ := db.Query(`
			SELECT action, COUNT(*) as cnt FROM activity_logs 
			GROUP BY action ORDER BY cnt DESC
		`)
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
		rows2, _ := db.Query(`
			SELECT admin_nama, COUNT(*) as cnt FROM activity_logs 
			GROUP BY admin_nama ORDER BY cnt DESC LIMIT 5
		`)
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
