// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/semanticstep/sst-core/sst"
	"github.com/blevesearch/bleve/v2"
)

func PrintRepositoryInfo(info sst.RepositoryInfo) {
	fmt.Printf("- URL: %s\n", info.URL)
	fmt.Printf("- AccessRight: %s\n", info.AccessRight.String())
	fmt.Printf("- MasterDBSize: %d\n", info.MasterDBSize)
	fmt.Printf("- DerivedDBSize: %d\n", info.DerivedDBSize)
	fmt.Printf("- DocumentDBSize: %d\n", info.DocumentDBSize)
	if info.MaxRepositorySize > 0 || info.ActualRepositorySize > 0 {
		fmt.Printf("- ActualRepositorySize: %d\n", info.ActualRepositorySize)
		fmt.Printf("- MaxRepositorySize: %d\n", info.MaxRepositorySize)
	}
	fmt.Printf("- Number of Datasets: %d\n", info.NumberOfDatasets)
	fmt.Printf("- Number of Datasets In Branch: %d\n", info.NumberOfDatasetsInBranch)
	fmt.Printf("- Number of Dataset Revisions: %d\n", info.NumberOfDatasetRevisions)
	fmt.Printf("- Number of Named Graph Revisions: %d\n", info.NumberOfNamedGraphRevisions)
	fmt.Printf("- Number of Commits: %d\n", info.NumberOfCommits)
	fmt.Printf("- Number of RepositoryLogs: %d\n", info.NumberOfRepositoryLogs)
	fmt.Printf("- Number of Documents: %d\n", info.NumberOfDocuments)
	fmt.Printf("- Is remote?: %s\n", BoolToYesNo(info.IsRemote))
	fmt.Printf("- Support Revision History?: %s\n", BoolToYesNo(info.SupportRevisionHistory))
	fmt.Printf("- Bleve Name: %s\n", info.BleveName)
	fmt.Printf("- Bleve Version: %s\n", info.BleveVersion)
	fmt.Printf("- Version Hash: %s\n", info.VersionHash)
}

func PrintStageInfo(stage sst.Stage) {
	info := stage.Info()

	fmt.Printf("- Number of local graphs: %d\n", info.NumberOfLocalGraphs)
	localGraphs := stage.NamedGraphs()
	for _, ng := range localGraphs {
		ibNodeCount := ng.IRINodeCount() + ng.BlankNodeCount()
		fmt.Printf("  - %s: %d IBNodes\n", ng.IRI(), ibNodeCount)
	}

	fmt.Printf("- Number of referenced graphs: %d\n", info.NumberOfReferencedGraphs)
	refGraphs := stage.ReferencedGraphs()
	for _, ng := range refGraphs {
		ibNodeCount := ng.IRINodeCount() + ng.BlankNodeCount()
		fmt.Printf("  - %s: %d IBNodes\n", ng.IRI(), ibNodeCount)
	}

	fmt.Printf("- Total number of triples: %d\n", info.TotalNumberOfTriples)
}

func PrintNamedGraphInfo(info sst.NamedGraphInfo) {
	fmt.Printf("- IRI: %s\n", info.Iri)
	fmt.Printf("- ID: %s\n", info.Id)
	fmt.Printf("- Is Referenced?: %s\n", BoolToYesNo(info.IsReferenced))
	fmt.Printf("- Is Empty?: %s\n", BoolToYesNo(info.IsEmpty))
	fmt.Printf("- Is Modified?: %t\n", info.IsModified)
	fmt.Printf("- Number of IRI Nodes: %d\n", info.NumberOfIRINodes)
	fmt.Printf("- Number of Blank Nodes: %d\n", info.NumberOfBlankNodes)
	fmt.Printf("- Number of Term Collections: %d\n", info.NumberOfTermCollections)
	fmt.Printf("- Number of Direct Imported Graphs: %d\n", info.NumberOfDirectImportedGraphs)
	fmt.Printf("- Number of All Imported Graphs: %d\n", info.NumberOfAllImportedGraphs)
	fmt.Printf("- Number of Subject Triples: %d\n", info.NumberOfSubjectTriples)
	fmt.Printf("- Number of Predicate Triples: %d\n", info.NumberOfPredicateTriples)
	fmt.Printf("- Number of Object Triples: %d\n", info.NumberOfObjectTriples)
	fmt.Printf("- Number of TermCollection Triples: %d\n", info.NumberOfTermCollectionTriples)

	fmt.Printf("- Commit Hash: %s\n", info.CheckedOutCommits)
	fmt.Printf("- NamedGraph Revision Hash: %s\n", info.NamedGraphRevision)
	fmt.Printf("- Dataset Revision Hash: %s\n", info.DatasetRevision)
}

func PrintCommitDetails(details *sst.CommitDetails) {
	fmt.Printf("- Commit Hash: %s\n", details.Commit)
	fmt.Printf("- Author: %s\n", details.Author)
	fmt.Printf("- Date: %s\n", details.AuthorDate.UTC().Format(time.RFC3339))
	fmt.Printf("- Message: %s\n", details.Message)

	// Dataset Revisions
	if len(details.DatasetRevisions) > 0 {
		fmt.Println("- Dataset Revisions:")
		for id, hash := range details.DatasetRevisions {
			fmt.Printf("  - %s::%s\n", id, hash)
		}
	} else {
		fmt.Println("- Dataset Revisions: {}")
	}

	// NamedGraph Revisions
	if len(details.NamedGraphRevisions) > 0 {
		fmt.Println("- NamedGraph Revisions:")
		for id, hash := range details.NamedGraphRevisions {
			fmt.Printf("  - %s::%s\n", id, hash)
		}
	} else {
		fmt.Println("- NamedGraph Revisions: {}")
	}

	// Parent Commits
	if len(details.ParentCommits) > 0 {
		fmt.Println("- Parent Commits:")
		for dsID, parentList := range details.ParentCommits {
			fmt.Printf("  - %s:\n", dsID)
			for _, parent := range parentList {
				fmt.Printf("    - %s\n", parent)
			}
		}
	} else {
		fmt.Println("- Parent Commits: {}")
	}
}

func PrintNamedGraphDetails(alias string, namedGraph sst.NamedGraph) {
	fmt.Printf("NamedGraph '%s' (IRI '%s') opened successfully.\n", alias, namedGraph.IRI())
}

func PrintIBNodeDetails(alias string, node sst.IBNode) {
	fmt.Printf("IBNode '%s' (IRI '%s') opened successfully.\n", alias, node.IRI())
}

func ListCommitHistoryHashOnly(dataset sst.Dataset, commit sst.Hash, visited map[string]bool, ctx context.Context) {
	// Skip the commit if it has already been visited
	if visited[commit.String()] {
		return
	}

	// Mark the commit as visited
	visited[commit.String()] = true

	// Print only the commit hash
	fmt.Printf("- %s\n", commit.String())

	// Retrieve commit details to access parent commits
	commitDetails, err := dataset.CommitDetailsByHash(ctx, commit)
	if err != nil {
		fmt.Println(ExplainCLIError("get commit details", err))
		return
	}

	// Recursively print the history of parent commits
	for _, parent := range commitDetails.ParentCommits[dataset.IRI()] {
		ListCommitHistoryHashOnly(dataset, parent, visited, ctx)
	}
}

func ListCommitHistoryDetailed(dataset sst.Dataset, commit sst.Hash, visited map[string]bool, ctx context.Context) {
	// Skip the commit if it has already been visited
	if visited[commit.String()] {
		return
	}

	// Mark the commit as visited
	visited[commit.String()] = true

	// Retrieve the details of the current commit
	commitDetails, err := dataset.CommitDetailsByHash(ctx, commit)
	if err != nil {
		fmt.Println(ExplainCLIError("get commit details", err))
		return
	}

	// Print detailed commit information
	fmt.Println()
	PrintCommitDetails(commitDetails)

	// Recursively print the history of parent commits
	for _, parent := range commitDetails.ParentCommits[dataset.IRI()] {
		ListCommitHistoryDetailed(dataset, parent, visited, ctx)
	}
}

// ListCommitsEntryHashes returns commit hashes used as roots when walking history like
// interactive `listcommits`: all [Dataset.LeafCommits], then unique branch tips from
// [Dataset.Branches] (branch names sorted). The same commit hash appears at most once.
func ListCommitsEntryHashes(ctx context.Context, ds sst.Dataset) ([]sst.Hash, error) {
	seen := make(map[string]struct{})
	var roots []sst.Hash

	leaves, err := ds.LeafCommits(ctx)
	if err != nil {
		return nil, err
	}
	for _, h := range leaves {
		k := h.String()
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		roots = append(roots, h)
	}

	branches, err := ds.Branches(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(branches))
	for n := range branches {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		h := branches[n]
		k := h.String()
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		roots = append(roots, h)
	}
	return roots, nil
}

func PrintSearchResult(result *bleve.SearchResult) {
	whitelist := map[string]bool{
		"id":      true,
		"label":   true,
		"comment": true,
		"type":    true,
	}

	fmt.Printf("- %d matches, showing %d through %d, took %v\n",
		result.Total, len(result.Hits), len(result.Hits), result.Took)

	for i, hit := range result.Hits {
		fmt.Printf("  - %3d. %s (score: %.6f)\n", i+1, hit.ID, hit.Score)

		for field, val := range hit.Fields {
			if !whitelist[field] {
				continue
			}

			fmt.Printf("      - %s: %v\n", field, val)
		}
	}
}

// PrintSearchResultAllFields prints all fields from search results
func PrintSearchResultAllFields(result *bleve.SearchResult) {
	fmt.Printf("- %d matches, showing %d through %d, took %v\n",
		result.Total, len(result.Hits), len(result.Hits), result.Took)

	for i, hit := range result.Hits {
		fmt.Printf("  - %3d. %s (score: %.6f)\n", i+1, hit.ID, hit.Score)

		// Print all fields
		for field, val := range hit.Fields {
			fmt.Printf("      - %s: %v\n", field, val)
		}
	}
}

func PrintDocumentInfo(doc *sst.DocumentInfo) {
	fmt.Printf("- Hash: %s\n", doc.Hash.String())
	fmt.Printf("  - MIME TYPE: %s\n", doc.MIMEType)
	fmt.Printf("  - AUTHOR:    %s\n", doc.Author)
	fmt.Printf("  - TIMESTAMP: %s\n", doc.Timestamp.UTC().Format(time.RFC3339))
	fmt.Printf("  - SIZE:      %s (%d bytes)\n", HumanBytes(doc.Size), doc.Size)
}

func PrintDocumentList(docs []sst.DocumentInfo) {
	if len(docs) == 0 {
		fmt.Println("No documents found in repository.")
		return
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Timestamp.After(docs[j].Timestamp)
	})

	for _, doc := range docs {
		fmt.Printf("- Hash: %s\n", doc.Hash.String())
		fmt.Printf("  - MIME TYPE: %s\n", doc.MIMEType)
		fmt.Printf("  - AUTHOR:    %s\n", doc.Author)
		fmt.Printf("  - TIMESTAMP: %s\n", doc.Timestamp.UTC().Format(time.RFC3339))
		fmt.Printf("  - SIZE:      %s (%d bytes)\n", HumanBytes(doc.Size), doc.Size)
	}
}
