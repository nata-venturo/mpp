package domain

import "time"

// Pemohon is the citizen who registered. PII is deliberately minimal
// (name + contact); NIK is only ever stored hashed, and only when a
// service requires it.
type Pemohon struct {
	ID      string
	Name    string
	Phone   *string
	Email   *string
	NIKHash *string
}

// Booking is a scheduled registration, before on-site check-in.
type Booking struct {
	ID             string
	PemohonID      string
	InstansiID     string
	JenisLayananID string
	Tanggal        time.Time
	Channel        string // WHATSAPP | WEB
	QRToken        *string
	QRExpiresAt    *time.Time
	Status         string // BOOKED | CHECKED_IN | EXPIRED | CANCELLED
	CheckedInAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Detail is a booking joined with the names the confirm screen and the
// kiosk need, so neither has to make a second catalog call.
type Detail struct {
	Booking

	PemohonName    string
	PemohonPhone   *string
	InstansiName   string
	InstansiPrefix string
	LayananName    string
}
