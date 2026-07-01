// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import (
	"fmt"
	"os"
	"strings"
)

// BasePath is the root directory for the .sst configuration files (relative to the current working directory).
const (
	BasePath = "cli/cmd/.sst"
)

func ValidatePath(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}
	if err != nil {
		return fmt.Errorf("unable to access path: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	return nil
}

// Converts boolean to "yes" or "no"
func BoolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func WrapText(text string, width int, indentSize int) string {
	if len(text) <= width {
		return text
	}
	words := strings.Fields(text)
	var result, line string
	indent := strings.Repeat(" ", indentSize)
	for _, word := range words {
		if len(line)+len(word) > width {
			result += line + "\n" + indent
			line = word
		} else {
			if line != "" {
				line += " "
			}
			line += word
		}
	}
	result += line
	return result
}

func CleanQuotes(s string) string {
	// Remove leading/trailing double or single quotes if present
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

// HumanBytes formats bytes into a human-friendly string.
func HumanBytes(n int64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
	)
	f := float64(n)
	switch {
	case f >= TB:
		return fmt.Sprintf("%.2f TB", f/TB)
	case f >= GB:
		return fmt.Sprintf("%.2f GB", f/GB)
	case f >= MB:
		return fmt.Sprintf("%.2f MB", f/MB)
	case f >= KB:
		return fmt.Sprintf("%.2f KB", f/KB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// EnsureOutputExt appends ext when path lacks it (case-insensitive match)
// and normalizes the extension to lowercase.
func EnsureOutputExt(path, ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + strings.ToLower(ext)
	} else {
		ext = strings.ToLower(ext)
	}
	if strings.HasSuffix(strings.ToLower(path), ext) {
		return path[:len(path)-len(ext)] + ext
	}
	return path + ext
}
