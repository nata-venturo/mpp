// Package service reads the MPP configuration keys the queue engine
// depends on (BR-04 number format, check-in window, TTS phrasing).
//
// Every getter falls back to a hard default, so an empty system_config
// table is a working system rather than a broken one. Admin CRUD over
// these keys is a later delivery-plan phase; this is the read side.
package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/config/repository"
	"github.com/ndollem/mpp/apps/api/internal/shared/dbx"
	"github.com/ndollem/mpp/apps/api/pkg/logger"
)

// Config keys, mirroring the `config_key` values documented on
// mpp.system_config (migration 000005).
const (
	KeyNumberFormat  = "number_format"
	KeyCheckinWindow = "checkin_window"
	KeyTTSText       = "tts_text"
)

// NumberFormat controls how a sequence becomes a printed queue number
// (BR-04): `{prefix}` and `{seq}` placeholders, `pad` zero-pads the seq.
// Default `{prefix}-{seq}` with pad 3 → "A-014".
type NumberFormat struct {
	Pattern string `json:"pattern"`
	Pad     int    `json:"pad"`
}

// CheckinWindow decides how long a QR token stays valid.
//
//	mode "end_of_day"  → valid until 23:59:59 local on the booking date
//	mode "fixed_hours" → valid for `hours` from the booking date's start
type CheckinWindow struct {
	Mode  string `json:"mode"`
	Hours int    `json:"hours"`
}

// TTSText holds the Indonesian announcement template. Placeholders:
// `{nomor}` (spoken digits) and `{loket}` (spoken counter).
type TTSText struct {
	Template string `json:"template"`
}

var (
	defaultNumberFormat  = NumberFormat{Pattern: "{prefix}-{seq}", Pad: 3}
	defaultCheckinWindow = CheckinWindow{Mode: "end_of_day"}
	defaultTTSText       = TTSText{Template: "Nomor antrian {nomor}, silakan menuju {loket}"}
)

type ConfigService struct {
	repo *repository.ConfigRepository
	loc  *time.Location
}

func NewConfigService(repo *repository.ConfigRepository, loc *time.Location) *ConfigService {
	return &ConfigService{repo: repo, loc: loc}
}

// Location is the operating timezone used for all local-day arithmetic.
func (s *ConfigService) Location() *time.Location {
	return s.loc
}

// NumberFormat resolves BR-04's pattern for one agency.
func (s *ConfigService) NumberFormat(ctx context.Context, q dbx.Querier, instansiID *string) NumberFormat {
	out := defaultNumberFormat
	s.decode(ctx, q, instansiID, KeyNumberFormat, &out)
	if out.Pattern == "" {
		out.Pattern = defaultNumberFormat.Pattern
	}
	if out.Pad < 0 {
		out.Pad = 0
	}
	return out
}

// CheckinWindow resolves the QR validity rule for one agency.
func (s *ConfigService) CheckinWindow(ctx context.Context, q dbx.Querier, instansiID *string) CheckinWindow {
	out := defaultCheckinWindow
	s.decode(ctx, q, instansiID, KeyCheckinWindow, &out)
	if out.Mode == "" {
		out.Mode = defaultCheckinWindow.Mode
	}
	return out
}

// TTSText resolves the announcement template for one agency (FR-CFG-03).
func (s *ConfigService) TTSText(ctx context.Context, q dbx.Querier, instansiID *string) TTSText {
	out := defaultTTSText
	s.decode(ctx, q, instansiID, KeyTTSText, &out)
	if out.Template == "" {
		out.Template = defaultTTSText.Template
	}
	return out
}

// decode overlays a stored value onto `target`, leaving the defaults in
// place when the key is absent or malformed. A bad row must not take the
// queue down, so it is logged and ignored.
func (s *ConfigService) decode(ctx context.Context, q dbx.Querier, instansiID *string, key string, target any) {
	raw, err := s.repo.Get(ctx, q, instansiID, key)
	if err != nil || len(raw) == 0 {
		return
	}

	if err := json.Unmarshal(raw, target); err != nil {
		logger.Warn("Ignoring malformed system_config value",
			logger.String("config_key", key), logger.Err(err))
	}
}
