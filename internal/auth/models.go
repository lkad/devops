package auth

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleOperator   Role = "operator"
	RoleDeveloper Role = "developer"
	RoleAuditor   Role = "auditor"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Role        Role      `json:"role"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	Group       string    `json:"group"` // group for label-based access control
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

type AuthService struct {
	jwtService *JWTService
	ldapClient *LDAPClient
}

func NewAuthService(jwtService *JWTService, ldapClient *LDAPClient) *AuthService {
	return &AuthService{
		jwtService: jwtService,
		ldapClient: ldapClient,
	}
}

func (s *AuthService) Authenticate(username, password string) (*User, error) {
	return s.ldapClient.Authenticate(username, password)
}
