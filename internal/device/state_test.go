// Package device tests
// NOTE: These are DEV TESTS - may use httptest mocks for fast local development.
// For QA tests with real environment, see manager_integration_test.go

package device

import "testing"

func TestState_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     State
		to       State
		expected bool
	}{
		// Valid transitions
		{"pending to authenticated", StatePending, StateAuthenticated, true},
		{"authenticated to registered", StateAuthenticated, StateRegistered, true},
		{"registered to active", StateRegistered, StateActive, true},
		{"active to maintenance", StateActive, StateMaintenance, true},
		{"active to suspended", StateActive, StateSuspended, true},
		{"active to retire", StateActive, StateRetire, true},
		{"maintenance to active", StateMaintenance, StateActive, true},
		{"maintenance to suspended", StateMaintenance, StateSuspended, true},
		{"suspended to active", StateSuspended, StateActive, true},
		{"suspended to retire", StateSuspended, StateRetire, true},

		// Invalid transitions
		{"pending to active", StatePending, StateActive, false},
		{"pending to registered", StatePending, StateRegistered, false},
		{"authenticated to active", StateAuthenticated, StateActive, false},
		{"authenticated to pending", StateAuthenticated, StatePending, false},
		{"registered to maintenance", StateRegistered, StateMaintenance, false},
		{"active to pending", StateActive, StatePending, false},
		{"active to authenticated", StateActive, StateAuthenticated, false},
		{"maintenance to retire", StateMaintenance, StateRetire, false},
		{"suspended to maintenance", StateSuspended, StateMaintenance, false},
		{"retire to anything", StateRetire, StateActive, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			if result != tt.expected {
				t.Errorf("State(%s).CanTransitionTo(%s) = %v, want %v",
					tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StatePending, "pending"},
		{StateAuthenticated, "authenticated"},
		{StateRegistered, "registered"},
		{StateActive, "active"},
		{StateMaintenance, "maintenance"},
		{StateSuspended, "suspended"},
		{StateRetire, "retire"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if result := tt.state.String(); result != tt.expected {
				t.Errorf("State(%s).String() = %s, want %s", tt.state, result, tt.expected)
			}
		})
	}
}
