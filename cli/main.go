// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// The SST Core “Command Line Interface” tool CLI is a low level tool
// for debugging and testing the SST-Core and -Repositories.
package main

import (
	cli "github.com/semanticstep/sst-core/cli/cmd"
	"github.com/semanticstep/sst-core/cli/cmd/utils"
)

// main function to execute the CLI commands
// go build -o ./cli/sst ./cli/main.go
func main() {
	utils.LoadEnvSstCLI()
	cli.Execute()
}
