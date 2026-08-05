package crypto

import (
	"crypto/rand"
	"encoding/base64"
)

// RandomToken returns a URL-safe, unpadded base64 string carrying
// numBytes of cryptographic randomness. Used for opaque handles such as
// the MPP check-in QR token, where the value must be unguessable but
// carries no data of its own.
func RandomToken(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
