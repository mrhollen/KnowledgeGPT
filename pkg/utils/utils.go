package utils

import (
	"crypto/rand"
	"fmt"
)

func GenerateUUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	// Set the version to 4
	b[6] = (b[6] & 0x0f) | 0x40
	// Set the variant to RFC 4122
	b[8] = (b[8] & 0x3f) | 0x80

	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return uuid, nil
}
