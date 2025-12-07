package tempo

import (
	"fmt"
	"os"
	"strings"
)

const (
	// KubernetesServiceAccountTokenPath is the default path for Kubernetes service account tokens
	KubernetesServiceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// ResolveBearerToken resolves the bearer token from various sources with priority:
// 1. Explicit bearerToken string (highest priority)
// 2. Token from bearerTokenFile path
// 3. Kubernetes service account token (auto-detected)
// 4. Empty string if none available (fall back to unauthenticated)
func ResolveBearerToken(bearerToken, bearerTokenFile string) (string, error) {
	// Priority 1: Explicit token string
	if bearerToken != "" {
		return strings.TrimSpace(bearerToken), nil
	}

	// Priority 2: Token from file path
	if bearerTokenFile != "" {
		token, err := readTokenFromFile(bearerTokenFile)
		if err != nil {
			return "", fmt.Errorf("failed to read token from file %s: %w", bearerTokenFile, err)
		}
		if token != "" {
			return token, nil
		}
	}

	// Priority 3: Auto-detect Kubernetes service account token
	token, err := readTokenFromFile(KubernetesServiceAccountTokenPath)
	if err != nil {
		// File doesn't exist or can't be read - this is fine, just return empty
		// This allows the client to work without authentication
		return "", nil
	}
	if token != "" {
		return token, nil
	}

	// No token available - return empty string (unauthenticated)
	return "", nil
}

// readTokenFromFile reads a token from a file path
func readTokenFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

