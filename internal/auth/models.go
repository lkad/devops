package auth

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleOperator   Role = "operator"
	RoleDeveloper  Role = "developer"
	RoleAuditor    Role = "auditor"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Role        Role      `json:"role"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	Group       string    `json:"group"`
	LDAPDN      string    `json:"ldapDn,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
	User      *User  `json:"user"`
}