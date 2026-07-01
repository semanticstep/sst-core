// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const envSstCLIFile = ".env.sst-cli"

// LoadEnvSstCLI loads variables from .env.sst-cli if present.
// Search order: current working directory, then user home.
// Existing environment variables are never overwritten.
//
// File format: one KEY=VALUE per line; # comments and blank lines ignored;
// values may be quoted with " or '.
//
// Required for remote auth: SST_OIDC_REALM_URL, SST_OIDC_CLIENT_ID, SST_OIDC_CLIENT_SECRET.
// Optional: SST_USERNAME and SST_PASSWORD (both must be set for non-interactive login).
func LoadEnvSstCLI() {
	for _, path := range envSstCLIPaths() {
		if err := loadEnvFile(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			// Non-fatal: missing or unreadable file in one location should not block the CLI.
			continue
		}
		return
	}
}

func envSstCLIPaths() []string {
	var paths []string
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, envSstCLIFile))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, envSstCLIFile))
	}
	return paths
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		value = unquoteEnvValue(value)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func unquoteEnvValue(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}
