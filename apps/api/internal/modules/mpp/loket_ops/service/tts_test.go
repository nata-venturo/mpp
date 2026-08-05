package service_test

import (
	"testing"

	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops/service"
)

func TestSpeakNomor(t *testing.T) {
	tests := []struct {
		nomor string
		want  string
	}{
		{"A-014", "A - nol satu empat"},
		{"B007", "B nol nol tujuh"},
		{"C-1", "C - satu"},
		{"A-100", "A - satu nol nol"},
	}

	for _, tc := range tests {
		t.Run(tc.nomor, func(t *testing.T) {
			if got := service.SpeakNomor(tc.nomor); got != tc.want {
				t.Fatalf("SpeakNomor(%q) = %q, want %q", tc.nomor, got, tc.want)
			}
		})
	}
}

func TestSpeakLoket(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Loket 3", "loket tiga"},
		{"Loket 12", "loket dua belas"},
		{"Loket 21", "loket dua puluh satu"},
		{"L-A3", "loket l a tiga"},
		{"", "loket"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.SpeakLoket(tc.name); got != tc.want {
				t.Fatalf("SpeakLoket(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestSpeakNumber(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "nol"},
		{7, "tujuh"},
		{10, "sepuluh"},
		{11, "sebelas"},
		{15, "lima belas"},
		{20, "dua puluh"},
		{34, "tiga puluh empat"},
		{100, "seratus"},
		{112, "seratus dua belas"},
		{250, "dua ratus lima puluh"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := service.SpeakNumber(tc.n); got != tc.want {
				t.Fatalf("SpeakNumber(%d) = %q, want %q", tc.n, got, tc.want)
			}
		})
	}
}

// The announcement the TV speaks is the whole point of the slice — it
// must read like the example in docs/04-api/websocket-events.md.
func TestBuildTTSText(t *testing.T) {
	got := service.BuildTTSText(
		"Nomor antrian {nomor}, silakan menuju {loket}", "A-014", "Loket 3")

	want := "Nomor antrian A - nol satu empat, silakan menuju loket tiga"
	if got != want {
		t.Fatalf("BuildTTSText = %q, want %q", got, want)
	}
}
