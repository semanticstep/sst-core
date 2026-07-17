// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Test_MoveAndMerge_BetweenTwoRepositories_SameNamedGraphIRI tests the scenario where:
//  1. Two repositories exist, each with a NamedGraph having the same IRI but different commit histories
//  2. MoveAndMerge is performed from one repository's stage to another's
//  3. This verifies that the fix (!betweenTwoRepositories check) works correctly
//     and doesn't attempt to look up datasets in the target repository that don't exist yet
func Test_MoveAndMerge_BetweenTwoRepositories_SameNamedGraphIRI(t *testing.T) {
	testName := t.Name() + "Repo"
	dir1 := filepath.Join("./testdata/", testName+"1")
	dir2 := filepath.Join("./testdata/", testName+"2")

	// Use same NamedGraph IRI in both repositories
	ngIRI := sst.IRI(uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890").URN())

	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)

	// Step 1: Create first repository with a NamedGraph and commit it
	t.Run("setup_repo1", func(t *testing.T) {
		removeFolder(dir1)

		repo1, err := sst.CreateLocalRepository(dir1, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo1.Close()

		stage1 := repo1.OpenStage(sst.DefaultTriplexMode)
		ng1 := stage1.CreateNamedGraph(ngIRI)

		// Add some content
		node1 := ng1.CreateIRINode("nodeFromRepo1")
		node1.AddStatement(rdf.Type, rep.SchematicPort)
		node1.AddStatement(rep.Angle, sst.String("value-from-repo1"))

		// Commit to repo1
		_, _, err = stage1.Commit(context.TODO(), "First commit in repo1", sst.DefaultBranch)
		assert.NoError(t, err)
	})

	// Step 2: Create second repository with a NamedGraph (same IRI) and commit it
	t.Run("setup_repo2", func(t *testing.T) {
		removeFolder(dir2)

		repo2, err := sst.CreateLocalRepository(dir2, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo2.Close()

		stage2 := repo2.OpenStage(sst.DefaultTriplexMode)
		ng2 := stage2.CreateNamedGraph(ngIRI)

		// Add some different content
		node2 := ng2.CreateIRINode("nodeFromRepo2")
		node2.AddStatement(rdf.Type, rep.Angle)
		node2.AddStatement(rdf.Bag, sst.String("value-from-repo2"))

		// Commit to repo2
		_, _, err = stage2.Commit(context.TODO(), "First commit in repo2", sst.DefaultBranch)
		assert.NoError(t, err)
	})

	// Step 3: Checkout from both repositories and perform cross-repository MoveAndMerge
	// This is where the bug would occur without the fix - before the fix, the code would
	// try to look up the dataset in the target repository, which doesn't exist there yet
	t.Run("cross_repo_move_and_merge", func(t *testing.T) {
		// Open repo1 and checkout
		repo1, err := sst.OpenLocalRepository(dir1, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo1.Close()

		ds1, err := repo1.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		stageFrom, err := ds1.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Verify stageFrom has the expected node
		ngFrom := stageFrom.NamedGraph(ngIRI)
		assert.NotNil(t, ngFrom)
		assert.NotNil(t, ngFrom.GetIRINodeByFragment("nodeFromRepo1"))

		// Open repo2 and create a fresh stage
		repo2, err := sst.OpenLocalRepository(dir2, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo2.Close()

		// Create a stage that will be the merge target
		stageTo := repo2.OpenStage(sst.DefaultTriplexMode)

		// Create a NamedGraph in the target stage with the same IRI
		ngTo := stageTo.CreateNamedGraph(ngIRI)
		ngToNode := ngTo.CreateIRINode("existingNodeInTarget")
		ngToNode.AddStatement(rdf.Type, rep.SchematicPort)

		// Verify stageTo has the expected node before merge
		assert.NotNil(t, stageTo.NamedGraph(ngIRI))
		assert.NotNil(t, stageTo.NamedGraph(ngIRI).GetIRINodeByFragment("existingNodeInTarget"))

		// Perform the cross-repository MoveAndMerge
		// Before the fix (!betweenTwoRepositories check), this would fail with:
		// "failed to get dataset by ID ...: dataset not found"
		// because the code would try to look up fromNG's dataset in repo2, which doesn't exist
		report, err := stageTo.MoveAndMerge(context.TODO(), stageFrom)
		assert.NoError(t, err, "MoveAndMerge between two repositories should not fail")
		assert.NotNil(t, report)

		// Assertions based on the merge logic:
		// 1. Report should indicate cross-repository merge
		assert.True(t, report.BetweenRepos, "Report should indicate cross-repository merge")

		// 2. Should have actions recorded
		assert.Greater(t, len(report.Actions), 0, "Report should have actions")

		// 3. Check for merge_graph action on the main NamedGraph (local->local merge)
		var foundLocalMerge bool
		for _, action := range report.Actions {
			if action.Type == "merge_graph" && action.BaseIRI == string(ngIRI) {
				foundLocalMerge = true
				assert.Equal(t, "success", action.Result)
				assert.Contains(t, action.Description, "local")
			}
		}
		assert.True(t, foundLocalMerge, "Should find local->local merge action for main graph")

		// 4. Stats should show merge_graph count >= 1
		assert.GreaterOrEqual(t, report.Stats["merge_graph"], 1, "Should have at least 1 merge_graph action")

		// 5. Verify the merge was successful by checking the content
		mergedNG := stageTo.NamedGraph(ngIRI)
		assert.NotNil(t, mergedNG, "Merged NamedGraph should exist")

		// 6. Verify nodes from both source and target are present
		assert.NotNil(t, mergedNG.GetIRINodeByFragment("nodeFromRepo1"), "Node from repo1 should exist")
		assert.NotNil(t, mergedNG.GetIRINodeByFragment("existingNodeInTarget"), "Node from target should exist")

		// 7. NamedGraph should be marked as modified (cross-repository merge)
		assert.True(t, mergedNG.IsModified(), "NamedGraph should be marked as modified after cross-repo merge")

		// 8. Verify stageFrom is now invalid (cleared after MoveAndMerge)
		assert.False(t, stageFrom.IsValid(), "Source stage should be invalid after MoveAndMerge")

		fmt.Println("MoveAndMerge Report:")
		fmt.Println(report)

		// Commit the merged result to repo2
		commitHash, modifiedDSIDs, err := stageTo.Commit(context.TODO(), "Merged from repo1", sst.DefaultBranch)
		assert.NoError(t, err)
		assert.NotEmpty(t, commitHash)
		assert.Equal(t, 1, len(modifiedDSIDs))

		fmt.Printf("Commit successful, hash: %s, modified datasets: %v\n", commitHash, modifiedDSIDs)

		// 9. Verify the dataset in repo2 now has the merged content
		ds2After, err := repo2.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		stageAfter, err := ds2After.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		ngAfter := stageAfter.NamedGraph(ngIRI)
		assert.NotNil(t, ngAfter)

		// 10. Verify both nodes exist in committed data
		assert.NotNil(t, ngAfter.GetIRINodeByFragment("nodeFromRepo1"))
		assert.NotNil(t, ngAfter.GetIRINodeByFragment("existingNodeInTarget"))
	})
}

// Test_MoveAndMerge_BetweenTwoRepositories_MultipleMerges tests multiple cross-repository
// MoveAndMerge operations without intermediate commits to ensure the fix handles all cases
func Test_MoveAndMerge_BetweenTwoRepositories_MultipleMerges(t *testing.T) {
	testName := t.Name() + "Repo"
	dir1 := filepath.Join("./testdata/", testName+"1")
	dir2 := filepath.Join("./testdata/", testName+"2")

	// Use same NamedGraph IRI in both repositories
	ngIRI := sst.IRI(uuid.MustParse("b2c3d4e5-f6a7-8901-bcde-f23456789012").URN())

	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)

	// Setup: Create both repositories with committed NamedGraphs
	t.Run("setup", func(t *testing.T) {
		removeFolder(dir1)
		removeFolder(dir2)

		// Repo 1
		repo1, err := sst.CreateLocalRepository(dir1, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		stage1 := repo1.OpenStage(sst.DefaultTriplexMode)
		ng1 := stage1.CreateNamedGraph(ngIRI)
		node1 := ng1.CreateIRINode("repo1Node")
		node1.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = stage1.Commit(context.TODO(), "Repo1 commit", sst.DefaultBranch)
		assert.NoError(t, err)
		repo1.Close()

		// Repo 2
		repo2, err := sst.CreateLocalRepository(dir2, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		stage2 := repo2.OpenStage(sst.DefaultTriplexMode)
		ng2 := stage2.CreateNamedGraph(ngIRI)
		node2 := ng2.CreateIRINode("repo2Node")
		node2.AddStatement(rdf.Type, rep.Angle)
		_, _, err = stage2.Commit(context.TODO(), "Repo2 commit", sst.DefaultBranch)
		assert.NoError(t, err)
		repo2.Close()
	})

	// Test: Multiple cross-repository merges
	t.Run("multiple_cross_repo_merges", func(t *testing.T) {
		repo1, err := sst.OpenLocalRepository(dir1, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo1.Close()

		repo2, err := sst.OpenLocalRepository(dir2, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo2.Close()

		ds1, err := repo1.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// First merge: repo1 -> repo2 stage
		stageFrom1, err := ds1.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)
		assert.True(t, stageFrom1.IsValid())

		stageTo := repo2.OpenStage(sst.DefaultTriplexMode)
		ngTo := stageTo.CreateNamedGraph(ngIRI)
		ngTo.CreateIRINode("initialNode")

		report1, err := stageTo.MoveAndMerge(context.TODO(), stageFrom1)
		assert.NoError(t, err)
		assert.NotNil(t, report1)
		assert.True(t, report1.BetweenRepos)

		// Verify first merge results
		assert.False(t, stageFrom1.IsValid(), "First source stage should be invalid after MoveAndMerge")
		assert.True(t, stageTo.NamedGraph(ngIRI).IsModified(), "NamedGraph should be modified after first merge")

		// Second merge: repo1 -> same repo2 stage (without commit in between)
		// This tests the scenario where the target stage now has checkedOutCommits
		// from the first merge but hasn't been committed yet
		stageFrom2, err := ds1.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)
		assert.True(t, stageFrom2.IsValid())

		report2, err := stageTo.MoveAndMerge(context.TODO(), stageFrom2)
		assert.NoError(t, err, "Second merge should also succeed with the fix")
		assert.NotNil(t, report2)
		assert.True(t, report2.BetweenRepos)

		// Verify second merge results
		assert.False(t, stageFrom2.IsValid(), "Second source stage should be invalid after MoveAndMerge")
		assert.True(t, stageTo.NamedGraph(ngIRI).IsModified(), "NamedGraph should still be modified after second merge")

		// Commit should succeed
		_, _, err = stageTo.Commit(context.TODO(), "Merged multiple times from repo1", sst.DefaultBranch)
		assert.NoError(t, err)
	})
}

// Test_MoveAndMerge_BetweenTwoRepositories_LocalToNew tests the scenario:
// fromLocalNGs does NOT exist in toLocalNGs -> redirect fromLocalNGs to toStage
// (Line 617-657 in stage.go logic - the "no" branch where fromLocalNGs is redirected)
func Test_MoveAndMerge_BetweenTwoRepositories_LocalToNew(t *testing.T) {
	testName := t.Name() + "Repo"
	dir1 := filepath.Join("./testdata/", testName+"1")
	dir2 := filepath.Join("./testdata/", testName+"2")

	ngIRI1 := sst.IRI(uuid.MustParse("e5f6a7b8-c9d0-1234-efab-567890123456").URN())
	ngIRI2 := sst.IRI(uuid.MustParse("f6a7b8c9-d0e1-2345-fabc-678901234567").URN())

	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)

	// Setup: Create repos with different NamedGraphs
	t.Run("setup", func(t *testing.T) {
		removeFolder(dir1)
		removeFolder(dir2)

		// Repo 1 with ngIRI1
		repo1, err := sst.CreateLocalRepository(dir1, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)

		stage1 := repo1.OpenStage(sst.DefaultTriplexMode)
		ng1 := stage1.CreateNamedGraph(ngIRI1)
		node1 := ng1.CreateIRINode("nodeFromRepo1")
		node1.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = stage1.Commit(context.TODO(), "Repo1 commit", sst.DefaultBranch)
		assert.NoError(t, err)
		repo1.Close()

		// Repo 2 with ngIRI2 (different from ngIRI1)
		repo2, err := sst.CreateLocalRepository(dir2, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)

		stage2 := repo2.OpenStage(sst.DefaultTriplexMode)
		ng2 := stage2.CreateNamedGraph(ngIRI2)
		node2 := ng2.CreateIRINode("nodeFromRepo2")
		node2.AddStatement(rdf.Type, rep.Angle)

		_, _, err = stage2.Commit(context.TODO(), "Repo2 commit", sst.DefaultBranch)
		assert.NoError(t, err)
		repo2.Close()
	})

	// Test: Cross-repo merge where fromNG doesn't exist in toStage
	t.Run("cross_repo_new_local_merge", func(t *testing.T) {
		repo1, err := sst.OpenLocalRepository(dir1, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo1.Close()

		repo2, err := sst.OpenLocalRepository(dir2, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo2.Close()

		ds1, err := repo1.Dataset(context.TODO(), ngIRI1)
		assert.NoError(t, err)

		stageFrom, err := ds1.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Verify ngIRI1 exists in source
		assert.NotNil(t, stageFrom.NamedGraph(ngIRI1))

		// Create target stage in repo2 with NO graphs (ngIRI1 doesn't exist here)
		stageTo := repo2.OpenStage(sst.DefaultTriplexMode)
		assert.Equal(t, 0, len(stageTo.NamedGraphs()), "Target stage should have no local graphs initially")

		// Perform cross-repo merge - this should redirect the local graph
		report, err := stageTo.MoveAndMerge(context.TODO(), stageFrom)
		assert.NoError(t, err)
		assert.NotNil(t, report)

		// Assertions based on redirect logic:
		// 1. Report should show redirect action
		var foundRedirect bool
		var foundMarkModified bool
		for _, action := range report.Actions {
			if action.Type == "redirect_graph" && action.BaseIRI == string(ngIRI1) {
				foundRedirect = true
				assert.Equal(t, "success", action.Result)
				assert.Contains(t, action.Description, "redirect local graph")
			}
			if action.Type == "mark_modified" && action.BaseIRI == string(ngIRI1) {
				foundMarkModified = true
				assert.Equal(t, "success", action.Result)
			}
		}
		assert.True(t, foundRedirect, "Should find redirect_graph action for ngIRI1")
		assert.True(t, foundMarkModified, "Should find mark_modified action for cross-repo redirect")

		// 2. Stats should show redirect_graph and mark_modified
		assert.GreaterOrEqual(t, report.Stats["redirect_graph"], 1, "Should have redirect_graph action")
		assert.GreaterOrEqual(t, report.Stats["mark_modified"], 1, "Should have mark_modified action")

		// 3. The graph should now exist in stageTo
		ng := stageTo.NamedGraph(ngIRI1)
		assert.NotNil(t, ng, "Graph should be redirected to target stage")

		// 4. The graph should be marked as modified (cross-repository)
		assert.True(t, ng.IsModified(), "Redirected graph should be marked as modified")

		// 5. The node should be accessible
		assert.NotNil(t, ng.GetIRINodeByFragment("nodeFromRepo1"))

		// Verify the report shows redirect action
		fmt.Println("Local->New (Redirect) Merge Report:")
		fmt.Println(report)

		// Commit should succeed
		_, _, err = stageTo.Commit(context.TODO(), "Redirected local graph from repo1", sst.DefaultBranch)
		assert.NoError(t, err)
	})
}

// Test_MoveAndMerge_BetweenTwoRepositories_MultipleGraphs tests merging multiple NamedGraphs
// from one repository to another in a single MoveAndMerge operation
func Test_MoveAndMerge_BetweenTwoRepositories_MultipleGraphs(t *testing.T) {
	testName := t.Name() + "Repo"
	dir1 := filepath.Join("./testdata/", testName+"1")
	dir2 := filepath.Join("./testdata/", testName+"2")

	ngIRI1 := sst.IRI(uuid.MustParse("c3d4e5f6-a7b8-9012-cdef-345678901234").URN())
	ngIRI2 := sst.IRI(uuid.MustParse("d4e5f6a7-b8c9-0123-defa-456789012345").URN())

	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)

	// Setup: Create repo1 with multiple NamedGraphs
	t.Run("setup", func(t *testing.T) {
		removeFolder(dir1)
		removeFolder(dir2)

		// Repo 1 with multiple graphs
		repo1, err := sst.CreateLocalRepository(dir1, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)

		stage1 := repo1.OpenStage(sst.DefaultTriplexMode)

		ng1 := stage1.CreateNamedGraph(ngIRI1)
		node1 := ng1.CreateIRINode("node1Repo1")
		node1.AddStatement(rdf.Type, rep.SchematicPort)

		ng2 := stage1.CreateNamedGraph(ngIRI2)
		node2 := ng2.CreateIRINode("node2Repo1")
		node2.AddStatement(rdf.Type, rep.Angle)

		_, _, err = stage1.Commit(context.TODO(), "Repo1 with multiple graphs", sst.DefaultBranch)
		assert.NoError(t, err)
		repo1.Close()

		// Repo 2 with one overlapping graph
		repo2, err := sst.CreateLocalRepository(dir2, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)

		stage2 := repo2.OpenStage(sst.DefaultTriplexMode)

		// Create ngIRI1 in repo2 with different content
		ng1r2 := stage2.CreateNamedGraph(ngIRI1)
		node1r2 := ng1r2.CreateIRINode("node1Repo2")
		node1r2.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = stage2.Commit(context.TODO(), "Repo2 with one graph", sst.DefaultBranch)
		assert.NoError(t, err)
		repo2.Close()
	})

	// Test: Merge multiple graphs from repo1 to repo2
	t.Run("merge_multiple_graphs", func(t *testing.T) {
		repo1, err := sst.OpenLocalRepository(dir1, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo1.Close()

		repo2, err := sst.OpenLocalRepository(dir2, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo2.Close()

		// Checkout ngIRI1 from repo1
		ds1, err := repo1.Dataset(context.TODO(), ngIRI1)
		assert.NoError(t, err)

		stageFrom1, err := ds1.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Checkout ngIRI2 from repo1
		ds2, err := repo1.Dataset(context.TODO(), ngIRI2)
		assert.NoError(t, err)

		stageFrom2, err := ds2.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Merge stageFrom2 into stageFrom1 first (so we have both graphs in one stage)
		_, err = stageFrom1.MoveAndMerge(context.TODO(), stageFrom2)
		assert.NoError(t, err)

		// Create target stage in repo2
		stageTo := repo2.OpenStage(sst.DefaultTriplexMode)

		// Create ngIRI2 in target stage so it exists
		ng2To := stageTo.CreateNamedGraph(ngIRI2)
		ng2To.CreateIRINode("existingNodeInNg2")

		// Verify initial state
		assert.Equal(t, 1, len(stageTo.NamedGraphs()), "Target should have 1 graph before cross-repo merge")

		// Perform cross-repo merge (now stageFrom1 has both ngIRI1 and ngIRI2)
		report, err := stageTo.MoveAndMerge(context.TODO(), stageFrom1)
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.True(t, report.BetweenRepos)

		// Assertions:
		// 1. Should have actions for both graphs
		var foundNgIRI1Action, foundNgIRI2Action bool
		for _, action := range report.Actions {
			if action.BaseIRI == string(ngIRI1) {
				foundNgIRI1Action = true
			}
			if action.BaseIRI == string(ngIRI2) {
				foundNgIRI2Action = true
			}
		}
		assert.True(t, foundNgIRI1Action, "Should have action for ngIRI1")
		assert.True(t, foundNgIRI2Action, "Should have action for ngIRI2")

		// 2. Both graphs should be present
		assert.NotNil(t, stageTo.NamedGraph(ngIRI1), "ngIRI1 should exist in target")
		assert.NotNil(t, stageTo.NamedGraph(ngIRI2), "ngIRI2 should exist in target")

		// 3. Should have 2 local graphs now
		assert.Equal(t, 2, len(stageTo.NamedGraphs()), "Target should have 2 graphs after merge")

		// 4. Nodes from both source graphs should be accessible
		assert.NotNil(t, stageTo.NamedGraph(ngIRI1).GetIRINodeByFragment("node1Repo1"))
		assert.NotNil(t, stageTo.NamedGraph(ngIRI2).GetIRINodeByFragment("node2Repo1"))
		assert.NotNil(t, stageTo.NamedGraph(ngIRI2).GetIRINodeByFragment("existingNodeInNg2"))

		fmt.Println("Multiple Graphs Merge Report:")
		fmt.Println(report)

		// Commit should succeed
		_, _, err = stageTo.Commit(context.TODO(), "Merged multiple graphs from repo1", sst.DefaultBranch)
		assert.NoError(t, err)
	})
}

// Test_MoveAndMerge_BetweenTwoRepositories_EqualRevisions tests merging when both repos
// have the same revision (checkedOutCommits are equal)
func Test_MoveAndMerge_BetweenTwoRepositories_EqualRevisions(t *testing.T) {
	testName := t.Name() + "Repo"
	dir1 := filepath.Join("./testdata/", testName+"1")
	dir2 := filepath.Join("./testdata/", testName+"2")

	ngIRI := sst.IRI(uuid.MustParse("f6a7b8c9-d0e1-2345-fabc-678901234567").URN())

	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)

	// Setup: Create repo1 and repo2 from same content
	t.Run("setup", func(t *testing.T) {
		removeFolder(dir1)
		removeFolder(dir2)

		// Repo 1
		repo1, err := sst.CreateLocalRepository(dir1, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)

		stage1 := repo1.OpenStage(sst.DefaultTriplexMode)
		ng1 := stage1.CreateNamedGraph(ngIRI)
		node1 := ng1.CreateIRINode("sharedNode")
		node1.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = stage1.Commit(context.TODO(), "Shared commit", sst.DefaultBranch)
		assert.NoError(t, err)
		repo1.Close()

		// Repo 2 - create identical content
		repo2, err := sst.CreateLocalRepository(dir2, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)

		stage2 := repo2.OpenStage(sst.DefaultTriplexMode)
		ng2 := stage2.CreateNamedGraph(ngIRI)
		node2 := ng2.CreateIRINode("sharedNode")
		node2.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = stage2.Commit(context.TODO(), "Shared commit", sst.DefaultBranch)
		assert.NoError(t, err)
		repo2.Close()
	})

	// Test: Merge repos with equal revisions
	t.Run("equal_revisions_merge", func(t *testing.T) {
		repo1, err := sst.OpenLocalRepository(dir1, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo1.Close()

		repo2, err := sst.OpenLocalRepository(dir2, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo2.Close()

		ds1, err := repo1.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		stageFrom, err := ds1.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get info before merge
		ngFrom := stageFrom.NamedGraph(ngIRI)
		infoFrom := ngFrom.Info()
		assert.NotNil(t, infoFrom)

		stageTo := repo2.OpenStage(sst.DefaultTriplexMode)
		ngTo := stageTo.CreateNamedGraph(ngIRI)
		ngTo.CreateIRINode("targetNode")

		report, err := stageTo.MoveAndMerge(context.TODO(), stageFrom)
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.True(t, report.BetweenRepos)

		// Assertions for equal revisions:
		// 1. Should have merge_graph action since graphs have same IRI
		var foundMerge bool
		for _, action := range report.Actions {
			if action.Type == "merge_graph" && action.BaseIRI == string(ngIRI) {
				foundMerge = true
				break
			}
		}
		assert.True(t, foundMerge, "Should have merge_graph action for equal revisions with same IRI")

		// 2. Graph should exist and be modified
		ng := stageTo.NamedGraph(ngIRI)
		assert.NotNil(t, ng)
		assert.True(t, ng.IsModified())

		// 3. Both nodes should be present after merge
		assert.NotNil(t, ng.GetIRINodeByFragment("sharedNode"))
		assert.NotNil(t, ng.GetIRINodeByFragment("targetNode"))

		fmt.Println("Equal Revisions Merge Report:")
		fmt.Println(report)

		// Commit should succeed
		_, _, err = stageTo.Commit(context.TODO(), "Merged equal revisions", sst.DefaultBranch)
		assert.NoError(t, err)
	})
}
