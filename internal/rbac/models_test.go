package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devops-toolkit/internal/auth"
	"github.com/google/uuid"
)

func TestCanAccessLabel(t *testing.T) {
	tests := []struct {
		name           string
		userGroups     []string
		labelGroup     string
		allLabelGroups []string
		want           bool
	}{
		{
			name:           "matching group",
			userGroups:     []string{"engineering", "devops"},
			labelGroup:     "engineering",
			allLabelGroups: nil,
			want:           true,
		},
		{
			name:           "no matching group",
			userGroups:     []string{"engineering", "devops"},
			labelGroup:     "operations",
			allLabelGroups: nil,
			want:           false,
		},
		{
			name:           "matches inherited group",
			userGroups:     []string{"engineering"},
			labelGroup:     "",
			allLabelGroups: []string{"engineering", "platform"},
			want:           true,
		},
		{
			name:           "empty user groups",
			userGroups:     []string{},
			labelGroup:     "engineering",
			allLabelGroups: nil,
			want:           false,
		},
		{
			name:           "empty label group and no inherited",
			userGroups:     []string{"engineering"},
			labelGroup:     "",
			allLabelGroups: nil,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanAccessLabel(tt.userGroups, tt.labelGroup, tt.allLabelGroups)
			if got != tt.want {
				t.Errorf("CanAccessLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasMatchingGroup(t *testing.T) {
	tests := []struct {
		name         string
		userGroups   []string
		entityGroups []string
		want         bool
	}{
		{
			name:         "direct match",
			userGroups:   []string{"engineering"},
			entityGroups: []string{"engineering"},
			want:         true,
		},
		{
			name:         "no match",
			userGroups:   []string{"engineering"},
			entityGroups: []string{"operations"},
			want:         false,
		},
		{
			name:         "multiple groups - partial match",
			userGroups:   []string{"engineering", "devops"},
			entityGroups: []string{"operations", "devops"},
			want:         true,
		},
		{
			name:         "empty entity groups",
			userGroups:   []string{"engineering"},
			entityGroups: []string{},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMatchingGroup(tt.userGroups, tt.entityGroups)
			if got != tt.want {
				t.Errorf("hasMatchingGroup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetUserGroupsWithInheritance(t *testing.T) {
	tests := []struct {
		name     string
		user     *auth.User
		wantLen  int
		wantErr  bool
	}{
		{
			name: "user with group",
			user: &auth.User{
				ID:       uuid.New(),
				Username: "testuser",
				Group:    "engineering",
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "user without group",
			user: &auth.User{
				ID:       uuid.New(),
				Username: "testuser",
				Group:    "",
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:     "nil user",
			user:     nil,
			wantLen:  0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getUserGroupsWithInheritance(tt.user)
			if len(got) != tt.wantLen {
				t.Errorf("getUserGroupsWithInheritance() returned %v groups, want %v", len(got), tt.wantLen)
			}
		})
	}
}

func TestExtractEntityID(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		queryParam string
		want       string
	}{
		{
			name:       "UUID in path",
			path:       "/api/devices/123e4567-e89b-12d3-a456-426614174000",
			queryParam: "",
			want:       "123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:       "entityId in query",
			path:       "/api/devices",
			queryParam: "entityId=123e4567-e89b-12d3-a456-426614174000",
			want:       "123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:       "path without UUID",
			path:       "/api/devices",
			queryParam: "",
			want:       "",
		},
		{
			name:       "non-UUID in path",
			path:       "/api/devices/list",
			queryParam: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path+"?"+tt.queryParam, nil)
			got := extractEntityID(req)
			if got != tt.want {
				t.Errorf("extractEntityID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequireLabelAccess(t *testing.T) {
	entityID := uuid.New()
	entityGroups := []string{"engineering"}

	getEntityGroups := func(entityID uuid.UUID) ([]string, error) {
		if entityID == entityID {
			return entityGroups, nil
		}
		return nil, nil
	}

	tests := []struct {
		name           string
		user           *auth.User
		entityID       uuid.UUID
		expectedStatus int
	}{
		{
			name: "matching group - allowed",
			user: &auth.User{
				ID:       uuid.New(),
				Username: "testuser",
				Roles:    []string{"developer"},
				Group:    "engineering",
			},
			entityID:       entityID,
			expectedStatus: http.StatusOK,
		},
		{
			name: "non-matching group - forbidden",
			user: &auth.User{
				ID:       uuid.New(),
				Username: "testuser",
				Roles:    []string{"developer"},
				Group:    "operations",
			},
			entityID:       entityID,
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "super admin bypasses check",
			user: &auth.User{
				ID:       uuid.New(),
				Username: "admin",
				Roles:    []string{"super_admin"},
				Group:    "unknown",
			},
			entityID:       entityID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "nil user - unauthorized",
			user:           nil,
			entityID:       entityID,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := RequireLabelAccess(getEntityGroups)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/api/devices/"+tt.entityID.String(), nil)
			if tt.user != nil {
				ctx := context.WithValue(req.Context(), auth.UserContextKey, tt.user)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("RequireLabelAccess() status = %v, want %v", rr.Code, tt.expectedStatus)
			}
		})
	}
}

func TestLabelGroup_GetAllGroups(t *testing.T) {
	parentID := uuid.New()
	g := &LabelGroup{
		ID:       uuid.New(),
		Name:     "engineering",
		ParentID: &parentID,
	}

	groups := g.GetAllGroups()
	if len(groups) != 1 {
		t.Errorf("GetAllGroups() returned %v groups, want 1 (parent lookup not implemented)", len(groups))
	}
}

func TestUserGroup_AllGroups(t *testing.T) {
	parentID := uuid.New()
	g := &UserGroup{
		ID:       uuid.New(),
		Name:     "engineering",
		ParentID: &parentID,
	}

	groups := g.AllGroups()
	if len(groups) != 1 {
		t.Errorf("AllGroups() returned %v groups, want 1 (parent lookup not implemented)", len(groups))
	}
}