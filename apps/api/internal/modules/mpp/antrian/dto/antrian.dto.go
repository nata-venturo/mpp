package dto

// PemohonRequest mirrors the booking payload's applicant block — a
// walk-in gives the same details, just at the kiosk instead of online.
type PemohonRequest struct {
	Name  string  `json:"name" binding:"required,min=2,max=255"`
	Phone string  `json:"phone" binding:"required,min=6,max=20"`
	Email *string `json:"email" binding:"omitempty,email"`
}

// WalkInRequest registers someone who turned up without a booking.
type WalkInRequest struct {
	InstansiID string         `json:"instansi_id" binding:"required,uuid"`
	LayananID  string         `json:"layanan_id" binding:"required,uuid"`
	Pemohon    PemohonRequest `json:"pemohon" binding:"required"`
}

// QueueQuery filters the waiting stream read.
type QueueQuery struct {
	LayananID string `form:"layanan_id" binding:"required,uuid"`
	Page      int    `form:"page" binding:"omitempty,min=1"`
	Limit     int    `form:"limit" binding:"omitempty,min=1,max=100"`
}

// AntrianResponse is a freshly issued ticket (check-in or walk-in).
type AntrianResponse struct {
	AntrianID   string `json:"antrian_id"`
	Nomor       string `json:"nomor"`
	NomorSeq    int    `json:"nomor_seq"`
	QueueStatus string `json:"queue_status"`
	EtaMenit    int    `json:"eta_menit"`
	InstansiID  string `json:"instansi_id"`
	LayananID   string `json:"layanan_id"`
	QueuedAt    string `json:"queued_at"`
}

// QueueItemResponse is one row of the waiting stream.
type QueueItemResponse struct {
	AntrianID string  `json:"antrian_id"`
	Nomor     string  `json:"nomor"`
	Status    string  `json:"status"`
	Source    string  `json:"source"`
	CallCount int     `json:"call_count"`
	Loket     *string `json:"loket"`
	QueuedAt  string  `json:"queued_at"`
}
