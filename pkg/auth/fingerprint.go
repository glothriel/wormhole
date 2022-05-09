package auth

import (
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"strings"
)

// Fingerprint allows presenting public key in a format, that can be interpreted by human
func Fingerprint(cert *rsa.PublicKey) string {
	h := sha256.New()
	_, _ = h.Write(cert.N.Bytes())
	stringParts := []string{}
	for _, singleByte := range h.Sum(nil)[:8] {
		stringParts = append(stringParts, fmt.Sprintf("%d", singleByte))
	}
	return strings.Join(stringParts, "::")
}
