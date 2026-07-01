// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import (
	"fmt"
	"os"
)

var (
	originalStdout *os.File = os.Stdout // Store the original stdout
	mutedStdout    *os.File             // Store the muted stdout file handle
)

// MuteStdout redirects stdout to a null device, effectively silencing fmt.Printf and similar output.
func MuteStdout() {
	// Close previous muted stdout if exists (defensive programming)
	if mutedStdout != nil {
		mutedStdout.Close()
		mutedStdout = nil
	}
	// Open DevNull in write mode to allow writing
	var err error
	mutedStdout, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		// If opening fails, panic as this is critical for functionality
		// In practice, this should never fail on any OS
		panic(fmt.Sprintf("Failed to open null device: %v", err))
	}
	os.Stdout = mutedStdout
}

// RestoreStdout restores stdout to its original destination.
func RestoreStdout() {
	if mutedStdout != nil {
		mutedStdout.Close()
		mutedStdout = nil
	}
	os.Stdout = originalStdout
}
