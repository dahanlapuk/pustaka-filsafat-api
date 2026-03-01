package models

import (
	"encoding/json"
	"time"
)

type ActivityLog struct {
	ID         int             `json:"id"`
	AdminID    *int            `json:"admin_id"`
	AdminNama  string          `json:"admin_nama"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   *int            `json:"entity_id"`
	EntityName *string         `json:"entity_name"`
	Details    json.RawMessage `json:"details,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Action types
const (
	ActionCreate         = "CREATE"
	ActionUpdate         = "UPDATE"
	ActionDelete         = "DELETE"
	ActionDeleteRequest  = "DELETE_REQUEST"
	ActionDeleteApprove  = "DELETE_APPROVE"
	ActionPositionChange = "POSITION_CHANGE"
	ActionInventoryCheck = "INVENTORY_CHECK"
	ActionLogin          = "LOGIN"
	ActionLogout         = "LOGOUT"
)

// Entity types
const (
	EntityBook     = "BOOK"
	EntityCategory = "CATEGORY"
	EntityPosisi   = "POSISI"
	EntityLoan     = "LOAN"
	EntityAdmin    = "ADMIN"
)
