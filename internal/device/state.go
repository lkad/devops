package device

type State string

const (
	StatePending       State = "pending"
	StateAuthenticated State = "authenticated"
	StateRegistered    State = "registered"
	StateActive       State = "active"
	StateMaintenance   State = "maintenance"
	StateSuspended     State = "suspended"
	StateRetire       State = "retire"
)

var validTransitions = map[State][]State{
	StatePending:       {StateAuthenticated},
	StateAuthenticated:  {StateRegistered},
	StateRegistered:     {StateActive},
	StateActive:         {StateMaintenance, StateSuspended, StateRetire},
	StateMaintenance:    {StateActive, StateSuspended},
	StateSuspended:      {StateActive, StateRetire},
	StateRetire:        {},
}

func (s State) CanTransitionTo(next State) bool {
	transitions, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, t := range transitions {
		if t == next {
			return true
		}
	}
	return false
}

func (s State) String() string {
	return string(s)
}
