package dto

// LoketQuery is the filter for GET /loket.
type LoketQuery struct {
	InstansiID string `form:"instansi_id" binding:"required,uuid"`
}

// LoketResponse is the staff-facing counter payload.
type LoketResponse struct {
	ID         string `json:"id"`
	InstansiID string `json:"instansi_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	IsActive   bool   `json:"is_active"`
}
