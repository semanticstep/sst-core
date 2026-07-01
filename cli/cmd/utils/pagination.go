// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PaginateOutput displays a list of lines with pagination, pausing every `linesPerPage` lines.
// Users can press Enter to continue or type 'q' to exit early.
func PaginateOutput(lines []string, linesPerPage int) {
	reader := bufio.NewReader(os.Stdin)

	for i, line := range lines {
		fmt.Println(line)

		if (i+1)%linesPerPage == 0 && i+1 < len(lines) {
			fmt.Print("More lines available.\n")
			fmt.Print("Press Enter to continue, or type 'q' to quit: ")

			input, _ := reader.ReadString('\n')
			if strings.TrimSpace(input) == "q" {
				return
			}
		}
	}
}

// PaginateLogEntries paginates log entries where each entry is a group of lines.
func PaginateLogEntries(entries [][]string, pageSize int) {
	reader := bufio.NewReader(os.Stdin)

	for i := 0; i < len(entries); i += pageSize {
		end := i + pageSize
		if end > len(entries) {
			end = len(entries)
		}

		// Print each entry group
		for _, group := range entries[i:end] {
			for _, line := range group {
				fmt.Println(line)
			}
		}

		// If there are more entries, prompt user to continue
		if end < len(entries) {
			fmt.Print("\nMore entries available.\nPress Enter to continue, or type 'q' to quit: ")
			input, _ := reader.ReadString('\n')
			if input == "q\n" || input == "q\r\n" {
				break
			}
		}
	}
}
