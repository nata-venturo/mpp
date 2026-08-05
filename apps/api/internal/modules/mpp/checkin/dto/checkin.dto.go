package dto

// CheckInRequest carries the scanned QR token. It is an opaque handle —
// no PII travels in the code.
type CheckInRequest struct {
	Token string `json:"token" binding:"required,min=8,max=255"`
}

// InstansiRef / LayananRef are the labels the kiosk prints on the ticket
// without needing a second catalog call.
type InstansiRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

type LayananRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CheckInResponse is the kiosk's whole answer: the booking flipped to
// CHECKED_IN plus the queue number it just earned.
type CheckInResponse struct {
	BookingID   string      `json:"booking_id"`
	Status      string      `json:"status"`
	CheckedInAt string      `json:"checked_in_at"`
	Instansi    InstansiRef `json:"instansi"`
	Layanan     LayananRef  `json:"layanan"`

	AntrianID   string `json:"antrian_id"`
	Nomor       string `json:"nomor"`
	NomorSeq    int    `json:"nomor_seq"`
	QueueStatus string `json:"queue_status"`
	EtaMenit    int    `json:"eta_menit"`
	PemohonName string `json:"pemohon_name"`
}
