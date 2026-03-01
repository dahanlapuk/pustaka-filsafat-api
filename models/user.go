package models

import "time"

type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // Never expose password
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}
