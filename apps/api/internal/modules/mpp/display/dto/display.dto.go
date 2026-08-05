package dto

// DisplayQuery selects which agency's screen to render.
type DisplayQuery struct {
	InstansiID string `form:"instansi_id" binding:"required,uuid"`
}

// InstansiRef labels the screen.
type InstansiRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

// CurrentCall is one counter's active call. TTSText is ready to speak —
// the TV never has to build the phrase itself.
type CurrentCall struct {
	AntrianID string `json:"antrian_id"`
	Nomor     string `json:"nomor"`
	Status    string `json:"status"`
	LoketID   string `json:"loket_id"`
	Loket     string `json:"loket"`
	TTSText   string `json:"tts_text"`
}

// NextUp is one upcoming number.
type NextUp struct {
	AntrianID string `json:"antrian_id"`
	Nomor     string `json:"nomor"`
	Layanan   string `json:"layanan"`
}

// DisplayResponse is the whole TV snapshot.
type DisplayResponse struct {
	Instansi InstansiRef   `json:"instansi"`
	Current  []CurrentCall `json:"current"`
	Next     []NextUp      `json:"next"`
}
