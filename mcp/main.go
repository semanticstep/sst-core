// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// SST-MCP exposes SST Core operations to AI clients via the Model Context Protocol.
//
// Build: go build -o mcp/sst-mcp ./mcp/main.go

// MCP Inspector:
// go build -o mcp/sst-mcp.exe ./mcp/main.go
// npx @modelcontextprotocol/inspector ./mcp/sst-mcp
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
	mcp "github.com/semanticstep/sst-core/mcp/cmd"
)

func main() {
	utils.LoadEnvSstCLI()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	sess := mcp.NewSession()
	if err := mcp.RunStdio(ctx, sess); err != nil {
		log.Fatal(err)
	}
}
