package domain

import "time"

// Queue item lifecycle states (mpp.antrian_status). Only the ones the
// walking skeleton drives are named here.
const (
	StatusWaiting = "WAITING"
	StatusCalled  = "CALLED"
	StatusServing = "SERVING"
	StatusDone    = "DONE"
	StatusSkipped = "SKIPPED"
)

// Where a queue item came from (mpp.antrian_source).
const (
	SourceBooking = "BOOKING"
	SourceWalkIn  = "WALK_IN"
)

// Queue ordering modes (mpp.queue_mode).
const (
	ModeFIFO            = "FIFO"
	ModeBookingPriority = "BOOKING_PRIORITY"
)

// Antrian is one queue ticket for one applicant on one day.
type Antrian struct {
	ID             string
	BookingID      *string
	PemohonID      string
	InstansiID     string
	JenisLayananID string
	Nomor          string
	NomorSeq       int
	QueueDate      time.Time
	Source         string
	Status         string
	LoketID        *string
	CallCount      int
	Priority       int
	QueuedAt       time.Time
	CalledAt       *time.Time
	ServedAt       *time.Time
	DoneAt         *time.Time
}

// QueueItem is a row of the waiting stream, joined with the names the
// loket panel and the public status screen show.
type QueueItem struct {
	Antrian

	PemohonName string
	LayananName string
	LoketName   *string
}
