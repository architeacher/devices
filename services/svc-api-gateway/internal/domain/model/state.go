package model

import (
	"fmt"
	"strings"
)

type State string

const (
	StateAvailable State = "available"
	StateInUse     State = "in-use"
	StateInactive  State = "inactive"
)

func (s State) String() string {
	return string(s)
}

func (s State) IsValid() bool {
	switch s {
	case StateAvailable, StateInUse, StateInactive:
		return true
	default:
		return false
	}
}

func ParseState(s string) (State, error) {
	state := State(strings.ToLower(strings.TrimSpace(s)))
	if !state.IsValid() {
		return "", fmt.Errorf("invalid state: %s", s)
	}

	return state, nil
}

func AllStates() []State {
	return []State{StateAvailable, StateInUse, StateInactive}
}
