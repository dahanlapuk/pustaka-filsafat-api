package models

import "time"

type Admin struct {
	ID                int        `json:"id"`
	Nama              string     `json:"nama"`
	Nickname          *string    `json:"nickname"`
	Email             string     `json:"email"`
	PasswordHash      string     `json:"-"` // Never expose
	Role              string     `json:"role"`
	Title             *string    `json:"title"`
	IsSuperadmin      bool       `json:"is_superadmin"`
	PasswordChangedAt *time.Time `json:"password_changed_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

type AdminLoginInput struct {
	AdminID  int    `json:"admin_id"`
	Password string `json:"password"`
}

type AdminProfileUpdate struct {
	Nama     string `json:"nama"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
}

type AdminPasswordChange struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type AdminResponse struct {
	ID                int        `json:"id"`
	Nama              string     `json:"nama"`
	Nickname          *string    `json:"nickname"`
	Email             string     `json:"email"`
	Role              string     `json:"role"`
	Title             *string    `json:"title"`
	IsSuperadmin      bool       `json:"is_superadmin"`
	PasswordChangedAt *time.Time `json:"password_changed_at"`
	PasswordExpired   bool       `json:"password_expired"`
}
