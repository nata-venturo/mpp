package domain

import "time"

// Instansi is an agency operating inside the MPP building.
type Instansi struct {
	ID             string
	CompanyID      string
	Name           string
	Slug           string
	Prefix         string
	Description    *string
	LogoURL        *string
	OperatingHours []byte // raw JSONB, passed through to the client
	QueueMode      string // FIFO | BOOKING_PRIORITY
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Layanan is a service type under an agency.
type Layanan struct {
	ID                     string
	InstansiID             string
	Name                   string
	Description            *string
	EstimasiDurasiMenit    int
	RequiresFOVerification bool
	IsActive               bool
	Syarat                 []SyaratDokumen
}

// SyaratDokumen is one document requirement attached to a service.
type SyaratDokumen struct {
	ID             string
	JenisLayananID string
	Name           string
	IsRequired     bool
	Notes          *string
	Sort           int
}
