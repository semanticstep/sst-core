// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import "fmt"

// AliasResult contains an alias and a confirmation function.
// If the alias was auto-generated, call Confirm() on success.
type AliasResult struct {
	Alias   string
	Confirm func() // Call this function only if the operation succeeds
}

// GetAlias checks if an alias is provided with '-a', if not, generates a default alias.
// Returns an AliasResult with the alias and a Confirm function.
// Call result.Confirm() only if the operation succeeds.
func GetAlias(args []string, resourceType string) (AliasResult, error) {
	// Try to extract alias after '-a' flag
	alias, err := extractAliasAfterFlag(args)
	if err == nil && alias != "" {
		// If alias is found after '-a', return it with a no-op Confirm function
		return AliasResult{
			Alias:   alias,
			Confirm: func() {}, // No-op for explicit aliases
		}, nil
	}

	// If '-a' is present but no alias follows, return an error
	if err != nil {
		return AliasResult{}, err
	}

	// If no '-a' is found, generate a default alias (without incrementing counter yet)
	alias = generateDefaultAliasWithoutIncrement(resourceType)
	return AliasResult{
		Alias: alias,
		Confirm: func() {
			// Increment counter only when confirmed
			aliasCounters[resourceType]++
		},
	}, nil
}

// RemoveAlias removes a specific alias string from a list of aliases (generic utility)
func RemoveAlias(list []string, alias string) []string {
	if len(list) == 0 {
		return list
	}
	out := list[:0]
	for _, a := range list {
		if a != alias {
			out = append(out, a)
		}
	}
	return out
}

// RemoveAliasFlag removes the first occurrence of '-a' flag and its value from args.
// This is useful when you need to get multiple aliases from the same args list.
func RemoveAliasFlag(args []string) []string {
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "-a" && i+1 < len(args) {
			// Skip both '-a' and its value
			i++
			continue
		}
		result = append(result, args[i])
	}
	return result
}

// ExtractOutputFlag returns the file path after '-o' if present.
func ExtractOutputFlag(args []string) (string, error) {
	for i, arg := range args {
		if arg == "-o" && i+1 < len(args) {
			return args[i+1], nil
		}
		if arg == "-o" && i+1 >= len(args) {
			return "", fmt.Errorf("error: missing path after '-o' flag")
		}
	}
	return "", nil
}

// ArgsWithoutOutputFlag removes the first occurrence of '-o' and its value from args.
func ArgsWithoutOutputFlag(args []string) []string {
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			i++
			continue
		}
		result = append(result, args[i])
	}
	return result
}

// extractAliasAfterFlag extracts the alias after a flag in the form of '-a <alias>'
func extractAliasAfterFlag(args []string) (string, error) {
	for i, arg := range args {
		if arg == "-a" && i+1 < len(args) {
			return args[i+1], nil
		}
		if arg == "-a" && i+1 >= len(args) {
			return "", fmt.Errorf("error: missing alias after '-a' flag")
		}
	}
	// Return an error if '-a' exists but no alias follows it
	return "", nil
}

// aliasCounters keeps track of the counters for each resource type
var aliasCounters = make(map[string]int)

// generateDefaultAliasWithoutIncrement generates the next alias without incrementing the counter.
// This is an internal function used by GetAlias. The counter will be incremented when
// the Confirm function from the returned AliasResult is called.
func generateDefaultAliasWithoutIncrement(resourceType string) string {
	var prefix string

	// Assign prefixes based on resource type
	switch resourceType {
	case "repository":
		prefix = "r"
	case "dataset":
		prefix = "d"
	case "stage":
		prefix = "s"
	case "namedgraph", "referencedgraph":
		prefix = "g"
	case "ibnode":
		prefix = "n"
	case "superrepository":
		prefix = "sr"
	default:
		fmt.Println("Error: Unknown resource type.")
		return ""
	}

	// Generate alias using current counter + 1 (but don't increment yet)
	nextCounter := aliasCounters[resourceType] + 1
	alias := fmt.Sprintf("%s%d", prefix, nextCounter)
	return alias
}
