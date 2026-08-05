package dto

// PemohonRequest is the applicant block of a booking request. Only name
// and a contact number are mandatory — NIK is collected solely when the
// chosen service requires it, and is never stored in the clear.
type PemohonRequest struct {
	Name  string  `json:"name" binding:"required,min=2,max=255"`
	Phone string  `json:"phone" binding:"required,min=6,max=20"`
	Email *string `json:"email" binding:"omitempty,email"`
	NIK   *string `json:"nik" binding:"omitempty,min=8,max=32"`
}

// CreateBookingRequest is the public registration payload.
type CreateBookingRequest struct {
	InstansiID string         `json:"instansi_id" binding:"required,uuid"`
	LayananID  string         `json:"layanan_id" binding:"required,uuid"`
	Tanggal    string         `json:"tanggal" binding:"required,datetime=2006-01-02"`
	Pemohon    PemohonRequest `json:"pemohon" binding:"required"`
}

// BookingResponse is what the citizen gets back on create. Timestamps are
// UTC; `tanggal` is a local calendar day.
type BookingResponse struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	InstansiID  string  `json:"instansi_id"`
	LayananID   string  `json:"layanan_id"`
	Tanggal     string  `json:"tanggal"`
	Channel     string  `json:"channel"`
	QRToken     *string `json:"qr_token"`
	QRExpiresAt *string `json:"qr_expires_at"`
	CreatedAt   string  `json:"created_at"`
}

// CatalogRef is the minimal agency/service reference embedded in a
// booking detail, so the confirm screen needs no extra catalog call.
type CatalogRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix,omitempty"`
}

// BookingDetailResponse backs GET /booking/{id} — the confirm screen and
// the "re-open my QR" flow.
type BookingDetailResponse struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Tanggal     string     `json:"tanggal"`
	Channel     string     `json:"channel"`
	QRToken     *string    `json:"qr_token"`
	QRExpiresAt *string    `json:"qr_expires_at"`
	CheckedInAt *string    `json:"checked_in_at"`
	PemohonName string     `json:"pemohon_name"`
	Instansi    CatalogRef `json:"instansi"`
	Layanan     CatalogRef `json:"layanan"`
	CreatedAt   string     `json:"created_at"`
}
