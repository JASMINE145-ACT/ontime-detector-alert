package alerts

import (
	"crypto/rand"
	"encoding/hex"
)

func generateID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

