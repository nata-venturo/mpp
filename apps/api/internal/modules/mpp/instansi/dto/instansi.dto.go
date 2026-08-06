package dto

import "encoding/json"

// InstansiResponse is the public agency payload.
type InstansiResponse struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug"`
	Prefix         string          `json:"prefix"`
	Description    *string         `json:"description"`
	LogoURL        *string         `json:"logo_url"`
	OperatingHours json.RawMessage `json:"operating_hours"`
	QueueMode      string          `json:"queue_mode"`
	IsActive       bool            `json:"is_active"`
}

// SyaratDokumenResponse is one document requirement.
type SyaratDokumenResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	IsRequired bool    `json:"is_required"`
	Notes      *string `json:"notes"`
	Sort       int     `json:"sort"`
}

// LayananResponse is a service plus its requirements.
type LayananResponse struct {
	ID                     string                  `json:"id"`
	InstansiID             string                  `json:"instansi_id"`
	Name                   string                  `json:"name"`
	Description            *string                 `json:"description"`
	EstimasiDurasiMenit    int                     `json:"estimasi_durasi_menit"`
	RequiresFOVerification bool                    `json:"requires_fo_verification"`
	SyaratDokumen          []SyaratDokumenResponse `json:"syarat_dokumen"`
}
