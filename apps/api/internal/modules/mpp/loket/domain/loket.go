package domain

import "time"

// Loket is a physical service counter belonging to an agency.
type Loket struct {
	ID         string
	InstansiID string
	Code       string
	Name       *string
	Status     string // OPEN | CLOSED | BREAK
	LastIdleAt *time.Time
	IsActive   bool
}

// DisplayName prefers the human name and falls back to the code, so the
// TV and TTS never announce an empty counter.
func (l *Loket) DisplayName() string {
	if l.Name != nil && *l.Name != "" {
		return *l.Name
	}
	return l.Code
}
