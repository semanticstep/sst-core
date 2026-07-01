// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package interactive

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/sst"
)

// buildVerboseEntryGroups constructs full verbose log display blocks in original order.
func buildVerboseEntryGroups(repo sst.Repository, ctx context.Context, logs []sst.RepositoryLogEntry) [][]string {
	var entryGroups [][]string

	// Step 1: Collect all commit_ids for batch lookup
	var ids []sst.Hash
	for _, entry := range logs {
		if entry.Fields["type"] == "commit" {
			if h, err := sst.StringToHash(entry.Fields["commit_id"]); err == nil {
				ids = append(ids, h)
			}
		}
	}

	// Step 2: Fetch commit details in batch
	detailMap := make(map[string]*sst.CommitDetails)
	if len(ids) > 0 {
		detailsList, err := repo.CommitDetails(ctx, ids)
		if err != nil {
			fmt.Println(utils.ExplainCLIError("fetch commit details", err))
		} else {
			for i, h := range ids {
				base58ID := h.String()
				detailMap[base58ID] = detailsList[i]
			}
		}
	}

	// Step 3: Build log entries grouped by type
	for _, entry := range logs {
		var lines []string
		firstLine := fmt.Sprintf("#%09d", entry.LogKey)
		t := entry.Fields["type"]

		switch t {
		case "init":
			firstLine += " Init"
			lines = append(lines, firstLine)
			lines = append(lines, fmt.Sprintf("  Author:    %s", entry.Fields["author"]))
			lines = append(lines, fmt.Sprintf("  Message:   %s", entry.Fields["message"]))
			lines = append(lines, fmt.Sprintf("  Time:      %s", entry.Fields["timestamp"]))

		case "commit":
			base58ID := entry.Fields["commit_id"]
			detail, ok := detailMap[base58ID]
			if !ok {
				firstLine += fmt.Sprintf(" Type: Commit %s (details not found)", base58ID)
				lines = append(lines, firstLine)
				entryGroups = append(entryGroups, lines)
				continue
			}
			firstLine += fmt.Sprintf(" Commit %s", base58ID)
			lines = append(lines, firstLine)
			lines = append(lines, fmt.Sprintf("  Author:    %s", detail.Author))
			lines = append(lines, fmt.Sprintf("  Message:   %s", detail.Message))
			lines = append(lines, fmt.Sprintf("  Time:      %s", detail.AuthorDate.UTC().Format(time.RFC3339)))
			if branch := entry.Fields["branch"]; branch != "" {
				lines = append(lines, fmt.Sprintf("  Branch:    %s", branch))
			}
			for dsID, revHash := range detail.DatasetRevisions {
				lines = append(lines, fmt.Sprintf("  Dataset Revision: %s / %s", dsID.String(), revHash.String()))
			}

		case "upload_document":
			hash := entry.Fields["hash"]
			firstLine += fmt.Sprintf(" Upload %s", hash)
			lines = append(lines, firstLine)
			lines = append(lines, fmt.Sprintf("  Author:    %s", entry.Fields["author"]))
			lines = append(lines, fmt.Sprintf("  Mimetype:  %s", entry.Fields["mime_type"]))
			if sz := entry.Fields["size_bytes"]; sz != "" {
				if n, err := strconv.ParseInt(sz, 10, 64); err == nil {
					lines = append(lines, fmt.Sprintf("  Size:      %s (%s bytes)", utils.HumanBytes(n), sz))
				}
			}
			lines = append(lines, fmt.Sprintf("  Message:   %s", entry.Fields["message"]))
			lines = append(lines, fmt.Sprintf("  Time:      %s", entry.Fields["timestamp"]))

		case "download_document":
			hash := entry.Fields["hash"]
			firstLine += fmt.Sprintf(" Download %s", hash)
			lines = append(lines, firstLine)
			lines = append(lines, fmt.Sprintf("  Author:    %s", entry.Fields["author"]))
			lines = append(lines, fmt.Sprintf("  Mimetype:  %s", entry.Fields["mime_type"]))
			if sz := entry.Fields["size_bytes"]; sz != "" {
				if n, err := strconv.ParseInt(sz, 10, 64); err == nil {
					lines = append(lines, fmt.Sprintf("  Size:      %s (%s bytes)", utils.HumanBytes(n), sz))
				}
			}
			lines = append(lines, fmt.Sprintf("  Message:   %s", entry.Fields["message"]))
			lines = append(lines, fmt.Sprintf("  Time:      %s", entry.Fields["timestamp"]))

		case "delete_document":
			hash := entry.Fields["hash"]
			firstLine += fmt.Sprintf(" Delete Document %s", hash)
			lines = append(lines, firstLine)
			lines = append(lines, fmt.Sprintf("  Author:    %s", entry.Fields["author"]))
			lines = append(lines, fmt.Sprintf("  Mimetype:  %s", entry.Fields["mime_type"]))
			if sz := entry.Fields["size_bytes"]; sz != "" {
				if n, err := strconv.ParseInt(sz, 10, 64); err == nil {
					lines = append(lines, fmt.Sprintf("  Size:      %s (%s bytes)", utils.HumanBytes(n), sz))
				}
			}
			lines = append(lines, fmt.Sprintf("  Message:   %s", entry.Fields["message"]))
			lines = append(lines, fmt.Sprintf("  Time:      %s", entry.Fields["timestamp"]))

		case "set_branch":
			firstLine += " Set Branch"
			lines = append(lines, firstLine)
			lines = append(lines, fmt.Sprintf("  Author:    %s", entry.Fields["author"]))
			lines = append(lines, fmt.Sprintf("  Branch:    %s", entry.Fields["branch"]))
			lines = append(lines, fmt.Sprintf("  Dataset:   %s", entry.Fields["dataset"]))
			lines = append(lines, fmt.Sprintf("  Dataset Revision:  %s", entry.Fields["ds_revision"]))
			lines = append(lines, fmt.Sprintf("  Time:      %s", entry.Fields["timestamp"]))

		case "remove_branch":
			firstLine += " Remove Branch"
			lines = append(lines, firstLine)
			lines = append(lines, fmt.Sprintf("  Author:    %s", entry.Fields["author"]))
			lines = append(lines, fmt.Sprintf("  Branch:    %s", entry.Fields["branch"]))
			lines = append(lines, fmt.Sprintf("  Dataset:   %s", entry.Fields["dataset"]))
			lines = append(lines, fmt.Sprintf("  Dataset Revision:  %s", entry.Fields["ds_revision"]))
			lines = append(lines, fmt.Sprintf("  Time:      %s", entry.Fields["timestamp"]))

		case "sync_from":
			firstLine += " Sync From"
			lines = append(lines, firstLine)
			lines = append(lines, fmt.Sprintf("  Author:    %s", entry.Fields["author"]))
			lines = append(lines, fmt.Sprintf("  From:      %s", entry.Fields["from_repo_url"]))
			lines = append(lines, fmt.Sprintf("  Time:      %s", entry.Fields["timestamp"]))
			if countStr := entry.Fields["affected_count"]; countStr != "" {
				if count, err := strconv.Atoi(countStr); err == nil {
					for i := 0; i < count; i++ {
						idx := strconv.Itoa(i)
						lines = append(lines, fmt.Sprintf("  Dataset:   %s", entry.Fields["dataset_"+idx]))
						lines = append(lines, fmt.Sprintf("  Branch:    %s", entry.Fields["branch_"+idx]))
						lines = append(lines, fmt.Sprintf("  Dataset Revision:  %s", entry.Fields["ds_revision_"+idx]))
					}
				}
			}

		default:
			firstLine += fmt.Sprintf(" Type: %s (unknown)", strings.Title(strings.ReplaceAll(t, "_", " ")))
			lines = append(lines, firstLine)
			keys := make([]string, 0, len(entry.Fields))
			for k := range entry.Fields {
				if k == "type" {
					continue
				}
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				lines = append(lines, fmt.Sprintf("  %s: %s", strings.Title(k), entry.Fields[k]))
			}
		}

		entryGroups = append(entryGroups, lines)
	}

	return entryGroups
}

// buildSimpleEntryGroups constructs non-verbose (summary-style) log display blocks.
func buildSimpleEntryGroups(logs []sst.RepositoryLogEntry) [][]string {
	var entryGroups [][]string

	for _, entry := range logs {
		var lines []string
		firstLine := fmt.Sprintf("#%09d", entry.LogKey)
		t := entry.Fields["type"]

		switch t {
		case "commit":
			if cid := entry.Fields["commit_id"]; cid != "" {
				firstLine += fmt.Sprintf(" Commit %s", cid)
			} else {
				firstLine += " Commit"
			}

		case "upload_document":
			if hash := entry.Fields["hash"]; hash != "" {
				firstLine += fmt.Sprintf(" Upload %s", hash)
			} else {
				firstLine += " Upload"
			}

		case "download_document":
			if hash := entry.Fields["hash"]; hash != "" {
				firstLine += fmt.Sprintf(" Download %s", hash)
			} else {
				firstLine += " Download"
			}

		case "delete_document":
			if hash := entry.Fields["hash"]; hash != "" {
				firstLine += fmt.Sprintf(" Delete Document %s", hash)
			} else {
				firstLine += " Delete Document"
			}

		case "set_branch":
			firstLine += " Set Branch"

		case "remove_branch":
			firstLine += " Remove Branch"

		case "sync_from":
			firstLine += " Sync From"
			if fromURL := entry.Fields["from_repo_url"]; fromURL != "" {
				firstLine += fmt.Sprintf(" %s", fromURL)
			}

		case "init":
			firstLine += " Init"

		default:
			firstLine += fmt.Sprintf(" Unknown Type: %s", t)
		}

		lines = append(lines, firstLine)
		entryGroups = append(entryGroups, lines)
	}

	return entryGroups
}
