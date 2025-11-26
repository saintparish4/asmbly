package models

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateTraceID creates a cryptographically random 128-bit trace ID.
// Returns a 32-character lowercase hex string (e.g., "a1b2c3d4e5f6...").
//
// This uses crypto/rand for true randomness suitable for distributed systems,
// ensuring trace IDs are globally unique across all services.
func GenerateTraceID() string {
	b := make([]byte, 16) // 128 bits = 16 bytes
	_, err := rand.Read(b)
	if err != nil {
		// crypto/rand.Read only fails on catastrophic system errors
		// In practice, this should never happen on modern systems
		panic("failed to generate random trace ID: " + err.Error())
	}
	return hex.EncodeToString(b) // 16 bytes → 32 hex chars
}

// GenerateSpanID creates a cryptographically random 64-bit span ID.
// Returns a 16-character lowercase hex string (e.g., "1a2b3c4d5e6f7a8b").
//
// This uses crypto/rand for true randomness suitable for distributed systems,
// ensuring span IDs are unique within a trace.
func GenerateSpanID() string {
	b := make([]byte, 8) // 64 bits = 8 bytes
	_, err := rand.Read(b)
	if err != nil {
		// crypto/rand.Read only fails on catastrophic system errors
		panic("failed to generate random span ID: " + err.Error())
	}
	return hex.EncodeToString(b) // 8 bytes → 16 hex chars
}

// IsValidTraceID validates that a trace ID is properly formatted:
// - Exactly 32 characters
// - All characters are lowercase hexadecimal (0-9, a-f)
func IsValidTraceID(id string) bool {
	if len(id) != 32 {
		return false
	}
	return isHex(id)
}

// IsValidSpanID validates that a span ID is properly formatted:
// - Exactly 16 characters
// - All characters are lowercase hexadecimal (0-9, a-f)
func IsValidSpanID(id string) bool {
	if len(id) != 16 {
		return false
	}
	return isHex(id)
}

// isHex checks if a string contains only hexadecimal characters (0-9, a-f, A-F).
// We accept both uppercase and lowercase for flexibility.
func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
