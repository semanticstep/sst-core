// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestStage_NamedGraphIDs(t *testing.T) {
	type fields struct {
		testStage stage
	}
	u := uuid.MustParse
	tests := []struct {
		name   string
		fields fields
		want   []uuid.UUID
	}{
		{
			name: "no imports",
			fields: fields{
				testStage: stage{
					localGraphs: map[uuid.UUID]*namedGraph{
						uuid.MustParse("e00b52a1-9831-4b0f-be50-fd121187708d"): {
							id: uuid.MustParse("e00b52a1-9831-4b0f-be50-fd121187708d"),
						},
					},
					referencedGraphs: map[string]*namedGraph{},
				},
			},
			want: []uuid.UUID{u("e00b52a1-9831-4b0f-be50-fd121187708d")},
		},
		{
			name: "single import",
			fields: fields{
				testStage: stage{
					localGraphs: map[uuid.UUID]*namedGraph{
						uuid.MustParse("c8a6c627-a886-439a-8f0c-dd78e1c1bbd8"): {
							id: uuid.MustParse("c8a6c627-a886-439a-8f0c-dd78e1c1bbd8"),
						},
						uuid.MustParse("fa5741c3-fef3-4d5d-a7e1-ed853a535dea"): {
							id: uuid.MustParse("fa5741c3-fef3-4d5d-a7e1-ed853a535dea"),
						},
					},
					referencedGraphs: map[string]*namedGraph{},
				},
			},
			want: []uuid.UUID{u("c8a6c627-a886-439a-8f0c-dd78e1c1bbd8"), u("fa5741c3-fef3-4d5d-a7e1-ed853a535dea")},
		},
		{
			name: "2 nesting levels",
			fields: fields{
				testStage: stage{
					localGraphs: map[uuid.UUID]*namedGraph{
						uuid.MustParse("6a353684-175e-4f83-bec9-259f7b39ab24"): {
							id: uuid.MustParse("6a353684-175e-4f83-bec9-259f7b39ab24"),
						},
						uuid.MustParse("193d7489-b725-4a18-9094-be296e15872d"): {
							id: uuid.MustParse("193d7489-b725-4a18-9094-be296e15872d"),
						},
						uuid.MustParse("e0fd0b19-3cd7-414a-945a-f9cd899d732f"): {
							id: uuid.MustParse("e0fd0b19-3cd7-414a-945a-f9cd899d732f"),
						},
					},
					referencedGraphs: map[string]*namedGraph{},
				},
			},
			want: []uuid.UUID{u("6a353684-175e-4f83-bec9-259f7b39ab24"), u("193d7489-b725-4a18-9094-be296e15872d"), u("e0fd0b19-3cd7-414a-945a-f9cd899d732f")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fields.testStage
			var ids []uuid.UUID
			for _, v := range s.NamedGraphs() {
				ids = append(ids, v.ID())
			}
			assert.True(t, compareUUIDSlicesUnordered(tt.want, ids))
		})
	}
}
func compareUUIDSlicesUnordered(slice1, slice2 []uuid.UUID) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	// Use maps to count occurrences of each UUID
	countMap1 := make(map[uuid.UUID]int)
	countMap2 := make(map[uuid.UUID]int)

	for _, u := range slice1 {
		countMap1[u]++
	}
	for _, u := range slice2 {
		countMap2[u]++
	}

	// Compare the two maps
	for key, count1 := range countMap1 {
		if count2, found := countMap2[key]; !found || count1 != count2 {
			return false
		}
	}

	return true
}

func TestStage_AlignHistory(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "repo")
	repo, err := CreateLocalRepository(repoDir, "test@example.com", "test", true)
	assert.NoError(t, err)
	defer repo.Close()

	from := OpenStage(DefaultTriplexMode).(*stage)
	from.repo = repo

	// Create local graph in from stage
	fromNg := from.CreateNamedGraph(IRI("http://example.com/test")).(*namedGraph)
	fromNg.checkedOutCommits = []Hash{{1, 2, 3}}
	fromNg.checkedOutNGRevisions = []Hash{{4, 5, 6}}
	fromNg.checkedOutDSRevisions = []Hash{{7, 8, 9}}

	// Create referenced graph in from stage
	fromRefNg := from.referencedGraphByURI("http://example.com/ref")
	fromRefNg.checkedOutCommits = []Hash{{10, 11, 12}}
	fromRefNg.checkedOutNGRevisions = []Hash{{13, 14, 15}}
	fromRefNg.checkedOutDSRevisions = []Hash{{16, 17, 18}}

	// Create local graph in from stage that will match a referenced graph in to stage
	fromLocalForRef := from.CreateNamedGraph(IRI("http://example.com/ref2")).(*namedGraph)
	fromLocalForRef.checkedOutCommits = []Hash{{19, 20, 21}}
	fromLocalForRef.checkedOutNGRevisions = []Hash{{22, 23, 24}}
	fromLocalForRef.checkedOutDSRevisions = []Hash{{25, 26, 27}}

	to := OpenStage(DefaultTriplexMode).(*stage)

	// Create matching local graph in to stage
	toNg := to.CreateNamedGraph(IRI("http://example.com/test")).(*namedGraph)

	// Create referenced graph in to stage that matches a local graph in from stage
	toRefForLocal := to.referencedGraphByURI("http://example.com/ref2")

	// Create a non-matching local graph in to stage
	toOtherNg := to.CreateNamedGraph(IRI("http://example.com/other")).(*namedGraph)

	to.AlignHistory(from)

	// Verify repo was copied
	assert.Equal(t, from.repo, to.repo)

	// Verify matching local graph got checkout values
	assert.Equal(t, fromNg.checkedOutCommits, toNg.checkedOutCommits)
	assert.Equal(t, fromNg.checkedOutNGRevisions, toNg.checkedOutNGRevisions)
	assert.Equal(t, fromNg.checkedOutDSRevisions, toNg.checkedOutDSRevisions)

	// Verify referenced graph in to stage that matched a local graph in from stage
	// was converted to local, marked as deleted, and received checkout values
	toRefConverted, ok := to.localGraphs[toRefForLocal.id]
	assert.True(t, ok, "Referenced graph should be converted to local")
	assert.True(t, toRefConverted.flags.deleted, "Converted graph should be marked as deleted")
	assert.False(t, toRefConverted.flags.isReferenced, "Converted graph should remain local (not referenced)")
	assert.Equal(t, fromLocalForRef.checkedOutCommits, toRefConverted.checkedOutCommits)
	assert.Equal(t, fromLocalForRef.checkedOutNGRevisions, toRefConverted.checkedOutNGRevisions)
	assert.Equal(t, fromLocalForRef.checkedOutDSRevisions, toRefConverted.checkedOutDSRevisions)
	_, stillReferenced := to.referencedGraphs["http://example.com/ref2"]
	assert.False(t, stillReferenced, "Converted graph should be removed from referencedGraphs")

	// Verify non-matching graph was not modified
	assert.Empty(t, toOtherNg.checkedOutCommits)
	assert.Equal(t, []Hash{}, toOtherNg.checkedOutNGRevisions)
	assert.Equal(t, []Hash{}, toOtherNg.checkedOutDSRevisions)

	// Verify that modifying to's commits doesn't affect from's commits
	toNg.checkedOutCommits[0] = Hash{99, 99, 99}
	assert.NotEqual(t, fromNg.checkedOutCommits[0], toNg.checkedOutCommits[0])

	// Verify missing local graph in from stage is created and deleted in to stage
	fromMissingNg := from.CreateNamedGraph(IRI("http://example.com/missing")).(*namedGraph)
	fromMissingNg.checkedOutCommits = []Hash{{28, 29, 30}}
	fromMissingNg.checkedOutNGRevisions = []Hash{{31, 32, 33}}
	fromMissingNg.checkedOutDSRevisions = []Hash{{34, 35, 36}}

	to.AlignHistory(from)

	toMissingNg := to.localGraphs[fromMissingNg.id]
	assert.NotNil(t, toMissingNg, "Missing graph should be created in to stage")
	assert.True(t, toMissingNg.flags.deleted, "Missing graph should be marked as deleted")
	assert.Equal(t, fromMissingNg.checkedOutCommits, toMissingNg.checkedOutCommits)
	assert.Equal(t, fromMissingNg.checkedOutNGRevisions, toMissingNg.checkedOutNGRevisions)
	assert.Equal(t, fromMissingNg.checkedOutDSRevisions, toMissingNg.checkedOutDSRevisions)
}

type noError struct{ assert.TestingT }
