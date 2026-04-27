package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckDevicePermission_SuperAdmin(t *testing.T) {
	tests := []struct {
		name         string
		operation    DeviceOperation
		environment  string
		expected     bool
	}{
		{"SuperAdmin can read prod", OpDeviceRead, EnvProd, true},
		{"SuperAdmin can modify prod", OpDeviceModify, EnvProd, true},
		{"SuperAdmin can restart prod", OpDeviceRestart, EnvProd, true},
		{"SuperAdmin can delete prod", OpDeviceDelete, EnvProd, true},
		{"SuperAdmin can read dev", OpDeviceRead, EnvDev, true},
		{"SuperAdmin can modify dev", OpDeviceModify, EnvDev, true},
		{"SuperAdmin can restart dev", OpDeviceRestart, EnvDev, true},
		{"SuperAdmin can read test", OpDeviceRead, EnvTest, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := contextWithUser("testuser", []string{RoleSuperAdmin}, []string{PermAll})
			result := CheckDevicePermission(ctx, tt.operation, tt.environment)
			if result != tt.expected {
				t.Errorf("CheckDevicePermission() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckDevicePermission_Operator(t *testing.T) {
	tests := []struct {
		name         string
		operation    DeviceOperation
		environment  string
		expected     bool
	}{
		{"Operator can read prod", OpDeviceRead, EnvProd, true},
		{"Operator cannot modify prod", OpDeviceModify, EnvProd, false},
		{"Operator cannot restart prod", OpDeviceRestart, EnvProd, false},
		{"Operator cannot delete prod", OpDeviceDelete, EnvProd, false},
		{"Operator can read dev", OpDeviceRead, EnvDev, true},
		{"Operator can modify dev", OpDeviceModify, EnvDev, true},
		{"Operator can restart dev", OpDeviceRestart, EnvDev, true},
		{"Operator can delete dev", OpDeviceDelete, EnvDev, true},
		{"Operator can read test", OpDeviceRead, EnvTest, true},
		{"Operator can modify test", OpDeviceModify, EnvTest, true},
		{"Operator can restart test", OpDeviceRestart, EnvTest, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := contextWithUser("testuser", []string{RoleOperator}, []string{PermDeploy, PermConfigManage, PermExecute, PermRead})
			result := CheckDevicePermission(ctx, tt.operation, tt.environment)
			if result != tt.expected {
				t.Errorf("CheckDevicePermission() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckDevicePermission_Developer(t *testing.T) {
	tests := []struct {
		name         string
		operation    DeviceOperation
		environment  string
		expected     bool
	}{
		{"Developer can read prod", OpDeviceRead, EnvProd, true},
		{"Developer cannot modify prod", OpDeviceModify, EnvProd, false},
		{"Developer cannot restart prod", OpDeviceRestart, EnvProd, false},
		{"Developer cannot delete prod", OpDeviceDelete, EnvProd, false},
		{"Developer can read dev", OpDeviceRead, EnvDev, true},
		{"Developer cannot modify dev", OpDeviceModify, EnvDev, false},
		{"Developer cannot restart dev", OpDeviceRestart, EnvDev, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := contextWithUser("testuser", []string{RoleDeveloper}, []string{PermRead, PermTestDeploy})
			result := CheckDevicePermission(ctx, tt.operation, tt.environment)
			if result != tt.expected {
				t.Errorf("CheckDevicePermission() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckDevicePermission_Auditor(t *testing.T) {
	tests := []struct {
		name         string
		operation    DeviceOperation
		environment  string
		expected     bool
	}{
		{"Auditor can read prod", OpDeviceRead, EnvProd, true},
		{"Auditor cannot modify prod", OpDeviceModify, EnvProd, false},
		{"Auditor cannot restart prod", OpDeviceRestart, EnvProd, false},
		{"Auditor can read dev", OpDeviceRead, EnvDev, true},
		{"Auditor cannot modify dev", OpDeviceModify, EnvDev, false},
		{"Auditor cannot restart dev", OpDeviceRestart, EnvDev, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := contextWithUser("testuser", []string{RoleAuditor}, []string{PermRead, PermAuditRead})
			result := CheckDevicePermission(ctx, tt.operation, tt.environment)
			if result != tt.expected {
				t.Errorf("CheckDevicePermission() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckDevicePermission_NoUser(t *testing.T) {
	ctx := context.Background()
	result := CheckDevicePermission(ctx, OpDeviceRead, EnvProd)
	if result != false {
		t.Errorf("CheckDevicePermission() with no user = %v, want false", result)
	}
}

func TestRequireRole_Middleware(t *testing.T) {
	tests := []struct {
		name           string
		userRoles      []string
		requiredRoles  []string
		expectedStatus int
	}{
		{"SuperAdmin passes Operator requirement", []string{RoleSuperAdmin}, []string{RoleOperator}, http.StatusOK},
		{"Operator passes Operator requirement", []string{RoleOperator}, []string{RoleOperator}, http.StatusOK},
		{"Developer fails Operator requirement", []string{RoleDeveloper}, []string{RoleOperator}, http.StatusForbidden},
		{"No user fails", []string{}, []string{RoleOperator}, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		 handler := RequireRole(tt.requiredRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			if len(tt.userRoles) > 0 {
				ctx := contextWithUser("testuser", tt.userRoles, []string{})
				req = req.WithContext(ctx)
			}

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("RequireRole() status = %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

// Helper function to create context with user
func contextWithUser(username string, roles []string, permissions []string) context.Context {
	user := &User{
		Username:    username,
		Roles:       roles,
		Permissions: permissions,
	}
	return context.WithValue(context.Background(), userContextKey, user)
}
