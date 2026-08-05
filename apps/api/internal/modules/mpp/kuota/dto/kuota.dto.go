package dto

// AvailabilityQuery is the public availability filter. `date` is a local
// calendar day (YYYY-MM-DD), not an instant — quota is per operating day.
type AvailabilityQuery struct {
	InstansiID string `form:"instansi_id" binding:"required,uuid"`
	LayananID  string `form:"layanan_id" binding:"omitempty,uuid"`
	Date       string `form:"date" binding:"required,datetime=2006-01-02"`
}

// AvailabilityResponse reports the remaining seats for one day.
type AvailabilityResponse struct {
	Date      string `json:"date"`
	Kuota     int    `json:"kuota"`
	Terpakai  int    `json:"terpakai"`
	Remaining int    `json:"remaining"`
}
