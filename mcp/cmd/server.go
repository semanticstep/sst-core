// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer registers SST MCP tools on a new MCP server.
func NewServer(sess *Session) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "sst-mcp",
		Version: "0.1.0",
	}, nil)

	type statusArgs struct{}
	addTool(server, "status", "Show open SST resources in this MCP session", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, _ statusArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		return textResult(sess.Status())
	})

	type openArgs struct {
		Path  string `json:"path" jsonschema:"Absolute or relative path to local SST repository directory"`
		Alias string `json:"alias,omitempty" jsonschema:"Optional repository alias, e.g. r1"`
	}
	addTool(server, "open_local_repository", "Open a local SST repository", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args openArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.Path == "" {
			return nil, nil, fmt.Errorf("path is required")
		}
		id, err := sess.OpenLocalRepository(args.Path, args.Alias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]string{
			"repo_alias": id,
			"path":    args.Path,
			"type":    "local",
			"message": "repository opened",
		})
	})

	type openRemoteArgs struct {
		URL   string `json:"url" jsonschema:"Remote repository URL, e.g. host:443 or host:443#repo-name"`
		Alias string `json:"alias,omitempty" jsonschema:"Optional repository alias, e.g. r1"`
	}
	addTool(server, "open_remote_repository", "Open a remote SST repository (requires .env.sst-cli auth)", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args openRemoteArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.URL == "" {
			return nil, nil, fmt.Errorf("url is required")
		}
		id, err := sess.OpenRemoteRepository(args.URL, args.Alias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]string{
			"repo_alias": id,
			"url":     args.URL,
			"type":    "remote",
			"message": "repository opened",
		})
	})

	type closeArgs struct {
		RepoAlias string `json:"repo_alias" jsonschema:"Repository alias to close, e.g. r1"`
	}
	addTool(server, "repository_close", "Close a repository and release its session handle", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args closeArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.RepoAlias == "" {
			return nil, nil, fmt.Errorf("repo_alias is required")
		}
		if err := sess.CloseRepository(args.RepoAlias); err != nil {
			return nil, nil, err
		}
		return textResult(map[string]string{
			"repo_alias": args.RepoAlias,
			"message":    "repository closed",
		})
	})

	type datasetsArgs struct {
		RepoAlias string `json:"repo_alias" jsonschema:"Repository alias from open_local_repository or open_remote_repository, e.g. r1"`
	}
	addTool(server, "repository_datasets", "List dataset IRIs in a repository", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args datasetsArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.RepoAlias == "" {
			return nil, nil, fmt.Errorf("repo_alias is required")
		}
		datasets, err := sess.ListDatasets(args.RepoAlias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]any{
			"repo_alias": args.RepoAlias,
			"datasets":   datasets,
		})
	})

	return server
}

func addTool[T any](
	server *sdkmcp.Server,
	name, description string,
	handler func(context.Context, *sdkmcp.CallToolRequest, T) (*sdkmcp.CallToolResult, any, error),
) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        name,
		Description: description,
	}, handler)
}

func textResult(v any) (*sdkmcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: string(b)},
		},
	}, v, nil
}

// RunStdio runs the MCP server over stdin/stdout.
func RunStdio(ctx context.Context, sess *Session) error {
	server := NewServer(sess)
	return server.Run(ctx, &sdkmcp.StdioTransport{})
}
