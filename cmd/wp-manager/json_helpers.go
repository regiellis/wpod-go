package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
)

// Generate a random string of given length (A-Za-z0-9)
func generateRandomStringSafe(length int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, bigInt(len(chars)))
		if err != nil {
			result[i] = chars[0]
		} else {
			result[i] = chars[n.Int64()]
		}
	}
	return string(result)
}

func bigInt(n int) *big.Int {
	return big.NewInt(int64(n))
}

// Validate and sanitize instance name
func sanitizeInstanceName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("instance_name cannot be empty")
	}
	if strings.ContainsAny(trimmed, "/\\:*?\"<>| ") {
		return "", errors.New("instance_name contains invalid characters")
	}
	return trimmed, nil
}

// Validate and sanitize parent directory
func sanitizeParentDirectory(dir string) (string, error) {
	if dir == "" {
		return ".", nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("invalid parent_directory: %w", err)
	}
	return abs, nil
}

// Validate and sanitize custom salts
func sanitizeCustomSalts(salts map[string]string) map[string]string {
	validKeys := []string{"AUTH_KEY", "SECURE_AUTH_KEY", "LOGGED_IN_KEY", "NONCE_KEY", "AUTH_SALT", "SECURE_AUTH_SALT", "LOGGED_IN_SALT", "NONCE_SALT"}
	out := make(map[string]string)
	for _, k := range validKeys {
		v, ok := salts[k]
		if ok && v != "" {
			out[k] = v
		} else {
			out[k] = generateRandomStringSafe(64)
		}
	}
	return out
}

// Validate and sanitize extra env
func sanitizeExtraEnv(extra map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range extra {
		if k == "" || strings.ContainsAny(k, " \t\n\r=") {
			continue // skip invalid keys
		}
		out[k] = v
	}
	return out
}

// Validate and sanitize boolean
func sanitizeBool(val *bool, def bool) bool {
	if val == nil {
		return def
	}
	return *val
}

// Validate and sanitize int (port, etc)
func sanitizeInt(val *int, def int) int {
	if val == nil || *val <= 0 {
		return def
	}
	return *val
}

// Validate and sanitize string with default
func sanitizeString(val *string, def string) string {
	if val == nil || strings.TrimSpace(*val) == "" {
		return def
	}
	return strings.TrimSpace(*val)
}

// Validate and sanitize map[string]string with default
func sanitizeMap(val *map[string]string, def map[string]string) map[string]string {
	if val == nil {
		return def
	}
	return *val
}
