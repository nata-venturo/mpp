package dto

// SessionRequest opens or closes an operator's shift at a loket.
type SessionRequest struct {
	Action string `json:"action" binding:"required,oneof=open close"`
}

// SessionResponse reports the resulting shift state.
type SessionResponse struct {
	SessionID string  `json:"session_id"`
	LoketID   string  `json:"loket_id"`
	LoketName string  `json:"loket"`
	IsActive  bool    `json:"is_active"`
	OpenedAt  string  `json:"opened_at"`
	ClosedAt  *string `json:"closed_at"`
}

// CallNextRequest names the loket doing the calling. The operator's own
// counter takes the item — the pull model of BR-12.
type CallNextRequest struct {
	LoketID string `json:"loket_id" binding:"required,uuid"`
}

// AntrianActionResponse is the shared shape of every queue transition.
// TTSText is only filled on call/recall — those are the events the TV
// speaks.
type AntrianActionResponse struct {
	AntrianID   string  `json:"antrian_id"`
	Nomor       string  `json:"nomor"`
	Status      string  `json:"status"`
	LoketID     *string `json:"loket_id"`
	Loket       *string `json:"loket"`
	CallCount   int     `json:"call_count"`
	PemohonName string  `json:"pemohon_name"`
	LayananID   string  `json:"layanan_id"`
	LayananName string  `json:"layanan"`
	TTSText     string  `json:"tts_text,omitempty"`
	DurasiDetik *int    `json:"durasi_detik,omitempty"`
	DoneAt      *string `json:"done_at,omitempty"`
}
