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

	type syncFromArgs struct {
		TargetRepoAlias string   `json:"target_repo_alias" jsonschema:"Target repository alias that receives synced data, e.g. r1"`
		SourceRepoAlias string   `json:"source_repo_alias" jsonschema:"Source repository alias to sync from, e.g. r2"`
		Branch          string   `json:"branch,omitempty" jsonschema:"Optional branch name; omit or use * for all branches"`
		Datasets        []string `json:"datasets,omitempty" jsonschema:"Optional dataset IRIs, UUIDs, or opened dataset aliases; omit for all datasets"`
	}
	addTool(server, "repository_sync_from", "Sync data from another open repository into the target repository", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args syncFromArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.TargetRepoAlias == "" {
			return nil, nil, fmt.Errorf("target_repo_alias is required")
		}
		if args.SourceRepoAlias == "" {
			return nil, nil, fmt.Errorf("source_repo_alias is required")
		}
		result, err := sess.SyncFrom(args.TargetRepoAlias, args.SourceRepoAlias, args.Branch, args.Datasets)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
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

	type datasetArgs struct {
		RepoAlias string `json:"repo_alias" jsonschema:"Repository alias from open_local_repository or open_remote_repository, e.g. r1"`
		IRI       string `json:"iri" jsonschema:"Dataset IRI, e.g. urn:uuid:..."`
		Alias     string `json:"alias,omitempty" jsonschema:"Optional dataset alias, e.g. d1"`
	}
	addTool(server, "repository_dataset", "Open a dataset by IRI in a repository", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args datasetArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.RepoAlias == "" {
			return nil, nil, fmt.Errorf("repo_alias is required")
		}
		if args.IRI == "" {
			return nil, nil, fmt.Errorf("iri is required")
		}
		datasetAlias, err := sess.OpenDataset(args.RepoAlias, args.IRI, args.Alias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]string{
			"dataset_alias": datasetAlias,
			"repo_alias":    args.RepoAlias,
			"iri":           args.IRI,
			"message":       "dataset opened",
		})
	})

	type datasetBranchesArgs struct {
		DatasetAlias string `json:"dataset_alias" jsonschema:"Dataset alias from repository_dataset, e.g. d1"`
	}
	addTool(server, "dataset_branches", "List all branches and their commit hashes in a dataset", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args datasetBranchesArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.DatasetAlias == "" {
			return nil, nil, fmt.Errorf("dataset_alias is required")
		}
		branches, err := sess.Branches(args.DatasetAlias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]any{
			"dataset_alias": args.DatasetAlias,
			"branches":      branches,
		})
	})

	type datasetListCommitsArgs struct {
		DatasetAlias string `json:"dataset_alias" jsonschema:"Dataset alias from repository_dataset, e.g. d1"`
		Details      bool   `json:"details,omitempty" jsonschema:"If true, include full commit metadata like CLI --details"`
	}
	addTool(server, "dataset_list_commits", "Walk commit history from leaf commits and branch tips (deduplicated)", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args datasetListCommitsArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.DatasetAlias == "" {
			return nil, nil, fmt.Errorf("dataset_alias is required")
		}
		commits, err := sess.ListCommits(args.DatasetAlias, args.Details)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]any{
			"dataset_alias": args.DatasetAlias,
			"details":       args.Details,
			"commits":       commits,
			"count":         len(commits),
		})
	})

	type datasetCommitDetailsByHashArgs struct {
		DatasetAlias string `json:"dataset_alias" jsonschema:"Dataset alias from repository_dataset, e.g. d1"`
		Hash         string `json:"hash" jsonschema:"Commit hash as Base58 string"`
	}
	addTool(server, "dataset_commit_details_by_hash", "Show commit details by commit hash", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args datasetCommitDetailsByHashArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.DatasetAlias == "" {
			return nil, nil, fmt.Errorf("dataset_alias is required")
		}
		if args.Hash == "" {
			return nil, nil, fmt.Errorf("hash is required")
		}
		details, err := sess.CommitDetailsByHash(args.DatasetAlias, args.Hash)
		if err != nil {
			return nil, nil, err
		}
		details["dataset_alias"] = args.DatasetAlias
		return textResult(details)
	})

	type datasetCommitDetailsByBranchArgs struct {
		DatasetAlias string `json:"dataset_alias" jsonschema:"Dataset alias from repository_dataset, e.g. d1"`
		Branch       string `json:"branch" jsonschema:"Branch name, e.g. master"`
	}
	addTool(server, "dataset_commit_details_by_branch", "Show commit details by branch name", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args datasetCommitDetailsByBranchArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.DatasetAlias == "" {
			return nil, nil, fmt.Errorf("dataset_alias is required")
		}
		if args.Branch == "" {
			return nil, nil, fmt.Errorf("branch is required")
		}
		details, err := sess.CommitDetailsByBranch(args.DatasetAlias, args.Branch)
		if err != nil {
			return nil, nil, err
		}
		details["dataset_alias"] = args.DatasetAlias
		details["branch"] = args.Branch
		return textResult(details)
	})

	type datasetHistoryArgs struct {
		DatasetAlias string `json:"dataset_alias" jsonschema:"Dataset alias from repository_dataset, e.g. d1"`
	}
	addTool(server, "dataset_history", "Show commit history graph for the dataset (oldest first, with parents and branch tips)", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args datasetHistoryArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.DatasetAlias == "" {
			return nil, nil, fmt.Errorf("dataset_alias is required")
		}
		commits, err := sess.History(args.DatasetAlias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]any{
			"dataset_alias": args.DatasetAlias,
			"commits":       commits,
			"count":         len(commits),
		})
	})

	type datasetCheckoutBranchArgs struct {
		DatasetAlias string `json:"dataset_alias" jsonschema:"Dataset alias from repository_dataset, e.g. d1"`
		Branch       string `json:"branch" jsonschema:"Branch name to checkout, e.g. master"`
		Alias        string `json:"alias,omitempty" jsonschema:"Optional stage alias, e.g. s1"`
	}
	addTool(server, "dataset_checkout_branch", "Checkout a dataset branch into a new stage", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args datasetCheckoutBranchArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.DatasetAlias == "" {
			return nil, nil, fmt.Errorf("dataset_alias is required")
		}
		if args.Branch == "" {
			return nil, nil, fmt.Errorf("branch is required")
		}
		stageAlias, err := sess.CheckoutBranch(args.DatasetAlias, args.Branch, args.Alias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]string{
			"stage_alias":   stageAlias,
			"dataset_alias": args.DatasetAlias,
			"branch":        args.Branch,
			"message":       "stage opened",
		})
	})

	type datasetCheckoutCommitArgs struct {
		DatasetAlias string `json:"dataset_alias" jsonschema:"Dataset alias from repository_dataset, e.g. d1"`
		Hash         string `json:"hash" jsonschema:"Commit hash as Base58 string"`
		Alias        string `json:"alias,omitempty" jsonschema:"Optional stage alias, e.g. s1"`
	}
	addTool(server, "dataset_checkout_commit", "Checkout a dataset commit into a new stage", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args datasetCheckoutCommitArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.DatasetAlias == "" {
			return nil, nil, fmt.Errorf("dataset_alias is required")
		}
		if args.Hash == "" {
			return nil, nil, fmt.Errorf("hash is required")
		}
		stageAlias, err := sess.CheckoutCommit(args.DatasetAlias, args.Hash, args.Alias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]string{
			"stage_alias":   stageAlias,
			"dataset_alias": args.DatasetAlias,
			"commit":        args.Hash,
			"message":       "stage opened",
		})
	})

	type repositoryOpenStageArgs struct {
		RepoAlias string `json:"repo_alias" jsonschema:"Repository alias from open_local_repository or open_remote_repository, e.g. r1"`
		Alias     string `json:"alias,omitempty" jsonschema:"Optional stage alias, e.g. s1"`
	}
	addTool(server, "repository_open_stage", "Create an empty stage in a repository", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args repositoryOpenStageArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.RepoAlias == "" {
			return nil, nil, fmt.Errorf("repo_alias is required")
		}
		stageAlias, err := sess.OpenStage(args.RepoAlias, args.Alias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]string{
			"stage_alias": stageAlias,
			"repo_alias":  args.RepoAlias,
			"message":     "stage opened",
		})
	})

	type stageInfoArgs struct {
		StageAlias string `json:"stage_alias" jsonschema:"Stage alias from checkout or open_stage, e.g. s1"`
	}
	addTool(server, "stage_info", "Show stage info (local/referenced graphs and triple counts)", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args stageInfoArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.StageAlias == "" {
			return nil, nil, fmt.Errorf("stage_alias is required")
		}
		info, err := sess.Info(args.StageAlias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(info)
	})

	type stageNamedGraphsArgs struct {
		StageAlias string `json:"stage_alias" jsonschema:"Stage alias from checkout or open_stage, e.g. s1"`
	}
	addTool(server, "stage_named_graphs", "List named graph IRIs in a stage", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args stageNamedGraphsArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.StageAlias == "" {
			return nil, nil, fmt.Errorf("stage_alias is required")
		}
		graphs, err := sess.NamedGraphs(args.StageAlias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]any{
			"stage_alias":  args.StageAlias,
			"named_graphs": graphs,
			"count":        len(graphs),
		})
	})

	type stageValidateArgs struct {
		StageAlias string `json:"stage_alias" jsonschema:"Stage alias from checkout or open_stage, e.g. s1"`
		OutputPath string `json:"output_path,omitempty" jsonschema:"Optional path to write the validation report (.txt)"`
	}
	addTool(server, "stage_validate", "Validate a stage (rdf type and domain-range checks)", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args stageValidateArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.StageAlias == "" {
			return nil, nil, fmt.Errorf("stage_alias is required")
		}
		result, err := sess.Validate(args.StageAlias, args.OutputPath)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
	})

	type stageTriGArgs struct {
		StageAlias string `json:"stage_alias" jsonschema:"Stage alias from checkout or open_stage, e.g. s1"`
		OutputPath string `json:"output_path,omitempty" jsonschema:"Optional path to write TriG (.trig); content is always returned in the result"`
	}
	addTool(server, "stage_trig", "Export stage RDF as TriG", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args stageTriGArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.StageAlias == "" {
			return nil, nil, fmt.Errorf("stage_alias is required")
		}
		result, err := sess.TriG(args.StageAlias, args.OutputPath)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
	})

	type stageCommitArgs struct {
		StageAlias string `json:"stage_alias" jsonschema:"Stage alias from checkout or open_stage, e.g. s1"`
		Message    string `json:"message" jsonschema:"Commit message"`
		Branch     string `json:"branch,omitempty" jsonschema:"Optional branch name; empty uses repository default behavior"`
	}
	addTool(server, "stage_commit", "Commit current changes in the stage", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args stageCommitArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.StageAlias == "" {
			return nil, nil, fmt.Errorf("stage_alias is required")
		}
		if args.Message == "" {
			return nil, nil, fmt.Errorf("message is required")
		}
		result, err := sess.Commit(args.StageAlias, args.Message, args.Branch)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
	})

	type documentsArgs struct {
		RepoAlias string `json:"repo_alias" jsonschema:"Repository alias from open_local_repository or open_remote_repository, e.g. r1"`
	}
	addTool(server, "repository_documents", "List all documents in the repository", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args documentsArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.RepoAlias == "" {
			return nil, nil, fmt.Errorf("repo_alias is required")
		}
		documents, err := sess.ListDocuments(args.RepoAlias)
		if err != nil {
			return nil, nil, err
		}
		return textResult(map[string]any{
			"repo_alias": args.RepoAlias,
			"documents":  documents,
			"count":      len(documents),
		})
	})


	type documentSetArgs struct {
		RepoAlias string `json:"repo_alias" jsonschema:"Repository alias from open_local_repository or open_remote_repository, e.g. r1"`
		FilePath  string `json:"file_path" jsonschema:"Absolute or relative path to the local file to upload"`
	}
	addTool(server, "repository_document_set", "Upload a document file to the repository", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args documentSetArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.RepoAlias == "" {
			return nil, nil, fmt.Errorf("repo_alias is required")
		}
		if args.FilePath == "" {
			return nil, nil, fmt.Errorf("file_path is required")
		}
		result, err := sess.SetDocument(args.RepoAlias, args.FilePath)
		if err != nil {
			return nil, nil, err
		}
		result["repo_alias"] = args.RepoAlias
		return textResult(result)
	})

	type documentGetArgs struct {
		RepoAlias  string `json:"repo_alias" jsonschema:"Repository alias from open_local_repository or open_remote_repository, e.g. r1"`
		Hash       string `json:"hash" jsonschema:"Document hash as 44-character Base58 string"`
		OutputPath string `json:"output_path,omitempty" jsonschema:"Optional output file or directory path; defaults to hash plus extension in the current directory"`
	}
	addTool(server, "repository_document_get", "Download a document by hash to a local file", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args documentGetArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.RepoAlias == "" {
			return nil, nil, fmt.Errorf("repo_alias is required")
		}
		if args.Hash == "" {
			return nil, nil, fmt.Errorf("hash is required")
		}
		result, err := sess.GetDocument(args.RepoAlias, args.Hash, args.OutputPath)
		if err != nil {
			return nil, nil, err
		}
		result["repo_alias"] = args.RepoAlias
		return textResult(result)
	})

	type documentInfoArgs struct {
		RepoAlias string `json:"repo_alias" jsonschema:"Repository alias from open_local_repository or open_remote_repository, e.g. r1"`
		Hash      string `json:"hash" jsonschema:"Document hash as 44-character Base58 string"`
	}
	addTool(server, "repository_document_info", "Show document metadata in the repository", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args documentInfoArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.RepoAlias == "" {
			return nil, nil, fmt.Errorf("repo_alias is required")
		}
		if args.Hash == "" {
			return nil, nil, fmt.Errorf("hash is required")
		}
		result, err := sess.DocumentInfo(args.RepoAlias, args.Hash)
		if err != nil {
			return nil, nil, err
		}
		result["repo_alias"] = args.RepoAlias
		return textResult(result)
	})

	type documentDeleteArgs struct {
		RepoAlias string `json:"repo_alias" jsonschema:"Repository alias from open_local_repository or open_remote_repository, e.g. r1"`
		Hash      string `json:"hash" jsonschema:"Document hash as 44-character Base58 string"`
	}
	addTool(server, "repository_document_delete", "Delete a document by its hash", func(
		_ context.Context, _ *sdkmcp.CallToolRequest, args documentDeleteArgs,
	) (*sdkmcp.CallToolResult, any, error) {
		if args.RepoAlias == "" {
			return nil, nil, fmt.Errorf("repo_alias is required")
		}
		if args.Hash == "" {
			return nil, nil, fmt.Errorf("hash is required")
		}
		if err := sess.DeleteDocument(args.RepoAlias, args.Hash); err != nil {
			return nil, nil, err
		}
		return textResult(map[string]string{
			"repo_alias": args.RepoAlias,
			"hash":       args.Hash,
			"message":    "document deleted",
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
