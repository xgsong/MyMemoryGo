// Package service defines domain services that orchestrate business logic.
package service

import (
	"crypto/sha256"
	"encoding/hex"
)

// CalculateChecksum generates a checksum for memory content.
// This is used for change detection and cache invalidation.
func CalculateChecksum(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
