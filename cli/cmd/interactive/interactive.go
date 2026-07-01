// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
)

// interactiveCmd starts an interactive mode
var InteractiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Start an interactive mode for repository operations",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Entering SST CLI tool in interactive mode. Type 'q' to quit, 'help' to see available commands.")
		StartInteractiveMode()
	},
}

// StartInteractiveMode starts the interactive command loop
func StartInteractiveMode() {
	// Get user home directory for history file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	historyFile := filepath.Join(homeDir, ".sst_cli_history")

	// Create auto-completer
	completer := NewCompleter()

	// Configure readline with history support and auto-completion
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "sst > ",
		HistoryFile:     historyFile,
		HistoryLimit:    1000, // Store up to 1000 commands
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		// Fallback to basic input if readline fails
		startBasicInteractiveMode()
		return
	}
	defer rl.Close()

	for {
		// Read user input with readline (supports history navigation)
		line, err := rl.Readline()
		if err != nil {
			// Handle EOF (Ctrl+D) and interrupt (Ctrl+C)
			if err == readline.ErrInterrupt {
				fmt.Println("\nUse 'q' to quit.")
				continue
			}
			if err == io.EOF {
				fmt.Println("\nExiting SST CLI tool.")
				break
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)

		// Skip empty lines
		if input == "" {
			continue
		}

		// Exit interactive mode
		if input == "q" {
			fmt.Println("Exiting SST CLI tool.")
			break
		}

		// Save command to history
		rl.SaveHistory(input)

		// Safe command execution with panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[ERROR] Unexpected internal error: %v\n", r)
				}
			}()
			handleInteractiveCommand(input)
		}()
	}
}

// startBasicInteractiveMode is a fallback mode without readline features
func startBasicInteractiveMode() {
	fmt.Println("Warning: Command history is not available. Using basic input mode.")
	fmt.Println("Entering SST CLI tool in interactive mode. Type 'q' to quit, 'help' to see available commands.")

	reader := bufio.NewReader(os.Stdin)
	// Simple input loop without history
	for {
		fmt.Print("sst > ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}
		input = strings.TrimSpace(input)

		if input == "q" {
			fmt.Println("Exiting SST CLI tool.")
			break
		}

		if input == "" {
			continue
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[ERROR] Unexpected internal error: %v\n", r)
				}
			}()
			handleInteractiveCommand(input)
		}()
	}
}

// handleInteractiveCommand handles input commands in interactive mode
func handleInteractiveCommand(input string) {
	form, topLevel, alias, command, args, err := parseInput(input)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	switch form {
	case formEmpty:
		return
	case formTopLevel:
		if err := dispatchTopLevelCommand(topLevel, args); err != nil {
			fmt.Println(formatTopLevelError(err))
		}
	case formAliasCommand:
		if err := dispatchAliasCommand(alias, command, args); err != nil {
			fmt.Println(formatAliasCmdError(err))
		}
	}
}
