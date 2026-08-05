package domain

import "time"

// Slot is one quota row: a (agency, optional service, date) triple with a
// seat count and how many of those seats are already booked.
//
// JenisLayananID nil means the row is agency-wide — it covers every
// service of that agency on that date.
type Slot struct {
	ID             string
	InstansiID     string
	JenisLayananID *string
	Tanggal        time.Time
	Kuota          int
	Terpakai       int
}

// Remaining is the number of seats still bookable (never negative).
func (s *Slot) Remaining() int {
	if s == nil {
		return 0
	}
	if s.Terpakai >= s.Kuota {
		return 0
	}
	return s.Kuota - s.Terpakai
}
