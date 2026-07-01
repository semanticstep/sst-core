// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"context"
)

// Dataset is the representation of an RDF Dataset and all it's revisions within an SST Repository.
// Like an RDF Dataset an SST Dataset consists of a default NamedGraph and all the other NamedGraphs that are either directly or indirectly imported.
// Note: For SST also the default graph is a NamedGraph.
// Note: In OWL2, direct imported NamedGraphs are provided by owl:imports.
// An example of an indirect import is when NamedGraph A imports NamedGraph B which then imports NamedGraph C.
// In this case NamedGraph A directly imports NamedGraph B and indirectly imports NamedGraph C.
// All imports of NamedGraph within a Dataset must form zero, one or several directed acyclic graphs.
// SST ensures this by checking for cyclic imports during [NamedGraph.AddImport] for the current Stage.
// Inside the SST API imports are handled by the special methods [NamedGraph.DirectImports], [NamedGraph.AddImport], and [NamedGraph.RemoveImport].
//
// The set of DatasetRevisions of a Dataset form a directed acyclic graph following their parents with either 0 (if new), one (if it is a revision of previous one) or 2 (if merging).
// This parent information is imbedded in commit structure.
//
// Unique BranchNames within a Dataset can be stored for particular DatasetRevisions.
// BranchNames are set by SetBranch and removed by RemoveBranch.
// BranchNames by default are not synchronized between different Datasets.
//
// An SST Dataset can be retrieve from an SST Repository by methods such as [Repository.CreateDataset] or [Repository.OpenDataset].
// The latest revision of a Dataset of any branch is loaded into a new Stage by method [Dataset.StageForBranch].
// A specific revision of a Dataset is loaded into a new Stage by method [Dataset.StageAtCommit].
// All these methods ensure that the default NamedGraphs and other direct and indirect imported NamedGraphs are loaded in the revisions that corresponds to the specified branch or commit.
type Dataset interface {
	// IRI returns the IRI of the default NamedGraph of this [Dataset].
	IRI() IRI

	// Branches returns all available branch Names and corresponding commitHashes.
	// A SST Repository that does not support history returns an empty map.
	// This method can also be used to check if this Dataset is deleted or not.
	// If a Dataset is deleted, all branches are removed and thus an empty map is returned.
	Branches(ctx context.Context) (map[string]Hash, error)

	// LeafCommits returns all leaf commits of this Dataset that are not identified by a branch Name.
	// A SST Repository that does not support history returns an empty slice.
	LeafCommits(ctx context.Context) ([]Hash, error)

	// CommitDetailsByHash provides detail information about a commit identified by
	// commitID in scope of a Dataset. If the commitID is not know for the
	// Dataset an error ErrCommitNotFound is returned.
	CommitDetailsByHash(ctx context.Context, commit Hash) (*CommitDetails, error)

	// CommitDetailsByBranch provides detail information about a commit identified by
	// branch in scope of a Dataset. If the branch is not know for the
	// Dataset an error errBranchNotFound is returned.
	CommitDetailsByBranch(ctx context.Context, branch string) (*CommitDetails, error)

	// SetBranchCommit moves or sets a branch Name to the revision of this Dataset that was created in the specified commit.
	// If this branch Name was set before to another revision of this Dataset, it is removed from that revision.
	SetBranchCommit(ctx context.Context, commit Hash, branch string) error

	// SetBranchRevision moves or sets a branch Name to the revision of this Dataset identified by the given datasetRevision.
	// If this branch Name was set before to another revision of this Dataset, it is removed from that revision.
	SetBranchRevision(ctx context.Context, datasetRevision Hash, branch string) error

	// RemoveBranch deletes a dataset branch from the repository.
	RemoveBranch(ctx context.Context, branch string) error

	// Repository returns [Repository] to which the [Dataset] belongs.
	Repository() Repository

	// CheckoutCommit returns a new Stage that is based on the given commit but is not
	// related to any branch optionally using provided options.
	CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error)

	// CheckoutBranch returns a Stage that operates on the receiver Dataset for the commit that is
	// specified by the tip of the given branch and optionally using provided options.
	CheckoutBranch(ctx context.Context, branchName string, mode TriplexMode) (Stage, error)

	// CheckoutRevision returns a new Stage that is based on the given DatasetRevision.
	// This is useful when a DatasetRevision exists that is not associated with a branch name.
	CheckoutRevision(ctx context.Context, datasetRevision Hash, mode TriplexMode) (Stage, error)

	// CommitForRevision returns the commit hash that created the given dataset revision hash.
	// If the dataset revision is not found in this repository, ErrDatasetRevisionNotFound is returned.
	CommitForRevision(ctx context.Context, datasetRevision Hash) (Hash, error)

	// FindCommonParentRevision searches for a common ancestor of commit revision1 and commit revision2
	// of this Dataset and returns its common parent commit revision Hash.
	// For this, the two revisions must belong to the same Dataset and exist in the the same Repository.
	// Only then, it is possible to compare the two revisions of the same Dataset.
	// This method traverses the commit revision history of both revisions in the Repository
	// by following the commit tree that contain information about the parents.
	// This method returns either revision1 or revision2 or another revision of this Dataset in the same Repository.
	// This method returns Hash of empty string and nil if there is no common parent revision.
	// Note: This method is used for three-way diff and merge.
	FindCommonParentRevision(context context.Context, revision1, revision2 Hash) (parent Hash, err error)
}
