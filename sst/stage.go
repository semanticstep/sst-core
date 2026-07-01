// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"go.uber.org/zap"
)

var (
	ErrNamedGraphNotFound                              = errors.New("named graph not found")
	errReferencedGraphAlreadyExists                    = errors.New("external graph already exists")
	errNamedGraphAlreadyExists                         = errors.New("NamedGraph already exists in the Stage")
	errCannotMoveReferencedGraphsWithNamedGraphContent = errors.New("can not move external graphs if named graph with content exist")
	ErrStageDeletedOrInvalid                           = errors.New("stage deleted or invalid")
	ErrStageNotLinkToRepository                        = errors.New("stage does not link to a repository")
	ErrCommitWasNotCreated                             = errors.New("commit was not created")
	ErrPreCommitConditionFailed                        = errors.New("pre-commit condition failed")
)

// TriplexMode is a parameter for all functions and methods that create a new Stage, including
// [OpenStage], [RepositoryOpenStage].
// The TriplexMode parameter defines which triplexes to manage in the internal SST memory structure for a given RDF triple, consisting of a subject, a predicate and an object.
// For every RDF triple between 1 to 3 triplexes are created:
// * the subject triplex is always created and contains the predicate and object for a given subject
// * the optional predicate triplex is created and contains the subject and object for a given predicate.
// * the optional object triplex is created and contains the predicate and subject of a given object.
// Which triplex mode to use is a question on how to balance between performance and memory requirement:
// * to use only subject triplexes is most memory efficient
// * to use only subject and object triplexes is suitable for many applications that require to traverse RDF graphs both in forward and inverse directions
// * to use all three kinds of triplexes together is optimal for typical SST applications that uses a lot of application specific sub-properties that are also individuals.
// For the SST version 1.0 only the DefaultTriplexMode is available that uses all three kinds together.
// The triplex structure is internal to SST and can't be accessed, modified or manipulated by application other than by this parameter.
type TriplexMode int

const (
	// The default TriplexMode that manages subject, predicate and object triplex in the internal SST memory structure.
	DefaultTriplexMode TriplexMode = iota
)

type stage struct {
	repo Repository

	// the key of these maps should be same type
	localGraphs      map[uuid.UUID]*namedGraph
	referencedGraphs map[string]*namedGraph

	// initial value is 1
	assignedNamedGraphNumber int
}

// A Stage is the representation of one or more NamedGraphs in memory together with the IBNodes and triples.
// A Stage might either work on persistent data when it is linked to a SST Repository or ephemeral data when it is not
// linked to a SST Repository.
// A particular revision of a Dataset from a SST Repository can be loaded into a Stage, resulting in a memory representation of the default NamedGraph
// and all other directly and indirectly imported NamedGraphs.
// A Stage can contain particular NamedGraph in only one revision or with no revision information when it is created anew.
// To work on different revisions of the same NamedGraphs, these NamedGraphs need to be opened in different Stages.
// Modified or new data in a Stage that is linked to a SST Repository can be committed to the SST Repository.
//
// Note: A Stage corresponds to what is known as checkout data in Git, where only one version of a source file can exist.
type Stage interface {
	// Creates a NamedGraph in this Stage whose name is the provided IRI.
	// Note that the NamedGraph IRI must be unique within the Stage and also unique within the linked Repository.
	// The ID of the created NamedGraph will be a hash UUID (version 5) of the IRI of this NamedGraph.
	// A created NamedGraph is in local state(NamedGraph.IsReferenced() will return false).
	// e.g.
	// ng := stage.CreateNamedGraph(sst.IRI("http://example.com/test1"))
	CreateNamedGraph(iri IRI) NamedGraph

	// NamedGraph returns the local NamedGraph for the specified IRI.
	// If the NamedGraph with the IRI is not used in the Stage then the nil value is returned.
	NamedGraph(iri IRI) NamedGraph

	// NamedGraphs returns a slice of all local NamedGraphs in this Stage.
	// The returned slice does not contain Referenced NamedGraphs.
	// The returned slice might be empty.
	NamedGraphs() []NamedGraph

	// ReferencedGraph returns the Referenced NamedGraph for the specified base URI.
	// If the NamedGraph with the baseIRI is not used in the Stage then the nil value is returned.
	ReferencedGraph(iri IRI) NamedGraph

	// ReferencedGraphs returns a slice of all referenced NamedGraph in this Stage.
	// The returned slice does not contain loaded NamedGraphs.
	// The returned slice might be empty.
	ReferencedGraphs() []NamedGraph

	// ForNamedGraphs loops through all NamedGraphs in this Stage that are not in Referenced state
	// and invokes the callback function c for each of them.
	// In case the callback function c returns an error, the loop terminates and this method
	// is returning this error.
	ForNamedGraphs(c func(ng NamedGraph) error) error

	// ForReferencedNamedGraphs loops through all referenced NamedGraphs in this Stage
	// and invokes the callback function c for each of them.
	// In case the callback function c returns an error, the loop terminates and this method
	// is returning this error.
	ForReferencedNamedGraphs(c func(ng NamedGraph) error) error

	// IsValid returns true if the Stage is valid and represents the non-zero value of Stage type.
	// Methods such as MoveAndMerge() results in invalid Stages.
	// References to invalid Stages should be removed so that they can be taken care by garbage collector.
	IsValid() bool

	// Repository returns the SST [Repository] this Stage is linked to.
	Repository() Repository

	// Commit is saving all changes of the [NamedGraph]s and [IBNode]s in this Stage into the linked repository.
	// For each modified NamedGraph a new NamedGraphRevision is created.
	// Corresponding DatasetRevisions are created for each new NamedGraphRevisions or if one of the imported NamedGraphs has changed.
	// if branchName is an empty string, then the branchNames for each modified Dataset in the target repository is not changed.
	// if branchName is not an empty string, then for each modified Dataset with a branchName is set respectively moved
	// to the new Dataset Revision. Note: The specified branchName might be a new one.
	// If the stage is ephemeral (not linked to a repository), error sst.ErrStageNotLinkToRepository is returned.
	// Other errors may be returned from the underlying file system or connection to a remote repository.
	// If this method is successful, a Hash value is returned that identifies the commit together with the new DatasetRevisions.
	Commit(ctx context.Context, message string, branchName string) (Hash, []uuid.UUID, error)

	// MoveAndMerge tries to automatically move and merges all NamedGraphs and referenced graphs from the source Stage (`from`) into the target Stage (`to`).
	// After merging, the source Stage is cleared of all local and referenced graphs.
	// If a source NamedGraph does not exist in the target Stage, it is taken as it is.
	// If a source NamedGraph does exist in the target Stage, this method tries to merge these two NamedGraphs.
	// For merging two NamedGraph revisions, this method tries to find a common ancestor in the revision history in the Repository of the target Stage.
	// If a common ancestor is found:
	// 	1. if fromNG is a xxx-parent of the toNG, keep te toNG, and skip the fromNG
	// 	2. if the toNG is a xxx-parent of the fromNG, replace the toNG by the the fromNG
	// 	3. TBD: otherwise, use a common parent to perform a 3-diff merge
	// If no common ancestor is found, the fromNG is merged into the toNG by adding all IBNodes and triples from the fromNG into the toNG.
	// Returns:
	//   *MoveAndMergeReport - A report with detailed statistics about moved, merged, and deleted graphs/nodes.
	//   error               - An error if the operation fails.
	//
	// Typical usage:
	//   report, err := stage.MoveAndMerge(ctx, fromStage)
	//   if err != nil {
	//       // handle error
	//   }
	//   fmt.Println(report)
	MoveAndMerge(ctx context.Context, from Stage) (*MoveAndMergeReport, error)

	// AlignHistory copies the repository pointer and checkout metadata
	// from the given stage to this stage. For each local NamedGraph in the given stage
	// that has a matching NamedGraph in this stage, the checkout values
	// (checkedOutCommits, checkedOutNGRevisions, checkedOutDSRevisions) are copied.
	// If a matching NamedGraph is referenced in this stage but was local in the source
	// stage, it is converted to a local NamedGraph and marked as deleted, so that
	// commit reflects the removal.
	// This allows a stage created from e.g. RdfRead to be committed to the same
	// repository as the source stage.
	AlignHistory(from Stage)

	// IBNodeByVocabulary locates the IBNode with the IRI for the specified vocabulary element.
	IBNodeByVocabulary(t Elementer) (IBNode, error)

	// Close() ensures that all memory allocated by the stage is released and can be taken away by garbage collection.
	// After calling this method, the stage can no longer be used and it is invalid.
	// TODO: ensure all underlying objects become invalid
	Close()

	// WriteSstFilesDirectory persists the NamedGraphs of this Stage into the
	// directory represented by stageDir.
	//
	// Each NamedGraph is written to its own file. The file name is the
	// base64 URL-safe encoding (without padding) of the graph's IRI base
	// portion, i.e. the part before the last '#'. If the IRI contains no '#',
	// the whole IRI is encoded. This encoding keeps file names filesystem-safe
	// and allows the original base IRI to be recovered.
	//
	// Parameters:
	//   - stageDir: An fs.FS referring to an existing directory where the graph
	//     files will be created.
	//
	// Returns:
	//   - err: nil on success, or ErrDirectoryExpectedAsBasePath if stageDir
	//     does not denote a directory. I/O failures during graph writing are
	//     reported via panic.
	WriteSstFilesDirectory(stageDir fs.FS) (err error)

	// Info returns a StageInfo struct containing information about the Stage.
	// The information include the number of local graphs, the number of referenced graphs,
	// and the total number of triples across all local graphs.
	//
	// Returns:
	// - StageInfo: A struct containing the following fields:
	//   - NumberOfLocalGraphs: The number of local named graphs in the stage.
	//   - NumberOfReferencedGraphs: The number of referenced named graphs in the stage.
	//   - TotalNumberOfTriples: The total number of triples across all local named graphs.
	//
	// The function performs the following steps:
	// 1. Initializes counters for local graphs, referenced graphs, and total triples.
	// 2. Counts the number of local graphs by getting the length of the localGraphs map.
	// 3. Counts the number of referenced graphs by getting the length of the referencedGraphs map.
	// 4. Iterates over the localGraphs map and sums the number of triples from each graph's Info method.
	Info() StageInfo

	// Dump prints the repository information and dumps all localGraphs to the standard error output.
	// It logs the index of each graph before dumping it.
	Dump()

	// RdfWrite writes all NamedGraphs in this Stage to the given writer in the specified RDF format.
	// Currently supports RdfFormatTriG which allows exporting multiple named graphs in a single file.
	// For RdfFormatTurtle, only the first NamedGraph will be written (use NamedGraph.RdfWrite instead).
	RdfWrite(w io.Writer, format RdfFormat) error

	numberNGs()
	removeEmptyReferencedGraphs()
	referencedGraphByURI(baseURI string) *namedGraph
	addMissingIBNodeTriplexes() error
	getLoadedGraphByID(id uuid.UUID) (*namedGraph, error)
	assertAccess() error

	moveReferencedGraphsFrom(from *stage) error
	getFirstLoadedGraph() (ng *namedGraph)
	modifiedDatasets() map[uuid.UUID]struct{}
}

func (to *stage) numberNGs() {
	// for local graphs
	for _, ng := range to.localGraphs {
		if ng.ngNumber != 0 {
			// do nothing
		} else {
			// check if the ng is in dictionary stage
			ds, ok := staticDictionaryStage.(*dictionaryStage)

			if ok {
				if _, found := ds.Stage.(*stage).localGraphs[ng.id]; found {
					ng.ngNumber = -1
				} else {
					ng.ngNumber = to.assignedNamedGraphNumber
					to.assignedNamedGraphNumber++
				}
			} else {
				ng.ngNumber = to.assignedNamedGraphNumber
				to.assignedNamedGraphNumber++
			}

		}
	}

	// for referenced graphs
	for _, ng := range to.referencedGraphs {
		if ng.ngNumber != 0 {
			// do nothing
		} else {
			// check if the ng is in dictionary stage
			ds, ok := staticDictionaryStage.(*dictionaryStage)

			if ok {
				if _, found := ds.Stage.(*stage).localGraphs[ng.id]; found {
					ng.ngNumber = -1
				} else {
					ng.ngNumber = to.assignedNamedGraphNumber
					to.assignedNamedGraphNumber++
				}
			} else {
				ng.ngNumber = to.assignedNamedGraphNumber
				to.assignedNamedGraphNumber++
			}

		}
	}
}

func (to *stage) getFirstLoadedGraph() (ng *namedGraph) {
	if len(to.localGraphs) == 1 {
		for _, v := range to.localGraphs {
			return v
		}
	}

	return nil
}

func (to *stage) Dump() {
	fmt.Fprintln(os.Stderr, "stage.Repository:", to.Repository())
	count := 0
	for _, val := range to.localGraphs {
		fmt.Fprintf(os.Stderr, "localGraph %d ", count)
		val.Dump()
		count++
		fmt.Fprintln(os.Stderr)
	}
	for _, val := range to.referencedGraphs {
		fmt.Fprintf(os.Stderr, "referencedGraph %d ", count)
		val.Dump()
		count++
		fmt.Fprintln(os.Stderr)
	}
}

func (to *stage) stageRepo() string {
	if to.repo != nil {
		return to.repo.URL()
	} else {
		return "<ephemeral stage>"
	}
}

type StageInfo struct {
	NumberOfLocalGraphs      int
	NumberOfReferencedGraphs int
	// total count of all three kinds of triples of local NamedGraphs?
	// or separated count?
	TotalNumberOfTriples int
}

func (to *stage) Info() StageInfo {
	numberOfLocalGraphs := 0
	numberOfReferencedGraphs := 0
	totalNumberOfTriples := 0

	numberOfLocalGraphs = len(to.localGraphs)
	numberOfReferencedGraphs = len(to.referencedGraphs)

	for _, val := range to.localGraphs {
		totalNumberOfTriples += val.Info().NumberOfSubjectTriples
	}

	return StageInfo{
		NumberOfLocalGraphs:      numberOfLocalGraphs,
		NumberOfReferencedGraphs: numberOfReferencedGraphs,
		TotalNumberOfTriples:     totalNumberOfTriples,
	}
}

// -------- Report Types --------

// MergeActionType defines the type of action performed during the MoveAndMerge process.
// Typical values include merging graphs, redirecting graphs, changing graph types, marking as modified, updating triples, skipping, or other custom actions.
type MergeActionType string

const (
	ActionMergeGraph       MergeActionType = "merge_graph"
	ActionRedirectGraph    MergeActionType = "redirect_graph"
	ActionChangeToLocal    MergeActionType = "change_to_local"
	ActionMarkModified     MergeActionType = "mark_modified"
	ActionUpdateTriples    MergeActionType = "update_triples"
	ActionSkipSameRevision MergeActionType = "skip_same_revision"
	ActionThreeWayMerge    MergeActionType = "three_way_merge"
	ActionOther            MergeActionType = "other"
)

// MergeAction records a single operation performed during MoveAndMerge, including its type, target graph, description, result, timestamp, and optional metadata.
type MergeAction struct {
	Type        MergeActionType `json:"type"`
	GraphID     string          `json:"graph_id,omitempty"`
	BaseIRI     string          `json:"base_iri,omitempty"`
	Description string          `json:"description,omitempty"`
	Result      string          `json:"result,omitempty"` // success / skipped / failed
	Timestamp   time.Time       `json:"timestamp"`
	// Metadata    map[string]any  `json:"metadata,omitempty"` //
}

// MoveAndMergeReport provides a summary and detailed log of all actions performed during a MoveAndMerge operation.
// It includes a list of actions, statistics, warnings, timing and Stage's information about the operation.
type MoveAndMergeReport struct {
	Actions      []MergeAction  `json:"actions"`
	BetweenRepos bool           `json:"between_repos"`
	StageFrom    string         `json:"stage_from,omitempty"`
	StageTo      string         `json:"stage_to,omitempty"`
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
	Duration     time.Duration  `json:"duration"`
	Stats        map[string]int `json:"stats"`              // count of each kind of action
	Warnings     []string       `json:"warnings,omitempty"` // collect warnings
}

func (r *MoveAndMergeReport) AddAction(act MergeActionType, ng *namedGraph, desc, result string, meta map[string]any) {
	if r == nil {
		return
	}
	a := MergeAction{
		Type:        act,
		Description: desc,
		Result:      result,
		Timestamp:   time.Now(),
		// Metadata:    meta,
	}
	if ng != nil {
		a.GraphID = ng.id.String()
		a.BaseIRI = ng.baseIRI
	}

	r.Actions = append(r.Actions, a)
	if r.Stats == nil {
		r.Stats = make(map[string]int)
	}
	r.Stats[string(act)]++
}

func (r *MoveAndMergeReport) start(stageFrom, stageTo string, between bool) {
	r.BetweenRepos = between
	r.StageFrom = stageFrom
	r.StageTo = stageTo
	r.StartTime = time.Now()
}

func (r *MoveAndMergeReport) finish() {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
}

// String returns the string of JSON representation of the MoveAndMergeReport.
func (r *MoveAndMergeReport) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) //
	enc.SetIndent("", "  ")  //
	if err := enc.Encode(r); err != nil {
		return fmt.Sprintf(`{"error": "%v"}`, err)
	}
	return buf.String()
}

// fromStage has
//
//	localNGs
//	referencedNGs
//
// toStage has
//
//	localNGs
//	referencedNGs
//
// logic:
// fromReferencedNGs exists in toReferencedNGs?
//
//		 yes: merge fromReferencedNGs to toReferencedNGs, still referenced in toStage, no need to change modified flag
//	  no: fromReferencedNGs exists in toLocalNGs?
//	      yes: merge fromReferencedNGs to toLocalNGs, merge opreation will set the modified flag to true
//	      no: redirect fromReferencedNGs to toStage, and update all its contained triples, no need to change modified flag
//
// fromLocalNGs exists in toLocalNGs?
//
//	yes: check if fromLocalNGs is identical to toLocalNGs
//	     yes: updates triples of toLocalNGs
//	     no:  check if they all have revisions and from same repository?
//	          yes:  check if two revisions are same?
//	                yes: keep the NG depending on the modified flag, which NG is modified, keep the modified one and skip the other one
//	                no : check if there is a common ancestor of these two revisions in the repository of toStage?
//	                     if fromNG is a xxx-parent of the toNG, keep te toNG, and skip the fromNG
//	                     if the toNG is a xxx-parent of the fromNG, replace the toNG by the the fromNG
//	                     otherwise, merge fromLocalNGs to toLocalNGs, merge operation will set the modified flag to true
//	          no: merge fromLocalNGs to toLocalNGs, merge operation	 will set the modified flag to true
//	no: fromLocalNGs exists in toReferencedNGs?
//	    yes: change the toReferencedNGs to localNGs, and merge fromLocalNGs to toLocalNGs, merge operation will set the modified flag to true
//	    no: redirect fromLocalNGs to toStage, set modified flag to true when across repositories
func (to *stage) MoveAndMerge(ctx context.Context, from Stage) (*MoveAndMergeReport, error) {
	var err error
	report := &MoveAndMergeReport{}

	err = to.assertAccess()
	if err != nil {
		return report, err
	}
	err = from.assertAccess()
	if err != nil {
		return report, err
	}
	fromImpl := from.(*stage)
	// about the repo of these two stages, there are several cases:
	// 1. both of them are not linked to any repo, then it is a move and merge between two ephemeral stages,
	//    in this case, modified flag is not needed because the toStage cannot be committed.
	// 2. one of them is linked to a repo, then it is a move and merge between an ephemeral stage and a repo-linked stage
	//    1) if the toStage is linked to a repo, modified flag should be true due to the NamedGraph in fromStage is true (because it only can be got from SstRead or RdfRead).
	//    2) if the fromStage is linked to a repo, modified flag is not needed because the toStage cannot be committed.
	// 3. both of them are linked to the same repo, then it is a move and merge between two stages linked to the same repo
	//    in this case, the modified flag should not be set.
	// 4. both of them are linked to different repos, then it is a move and merge between two stages linked to different repos
	//    in this case, the modified flag should be set to true for moved or merged local NamedGraphs.
	betweenTwoRepositories := (to.repo != nil && fromImpl.repo != nil && to.repo != fromImpl.repo)

	report.start(fromImpl.stageRepo(), to.stageRepo(), betweenTwoRepositories)
	defer report.finish()

	GlobalLogger.Debug("NewMoveAndMerge Starts")

	// Loop over all referenced NGs in fromStage and merge them to the toStage
	for _, fromNg := range fromImpl.referencedGraphs {
		// if toStage does not have the fromNG then move the fromNG over to toStage (not copy),
		// including all the IBNodes and object triples
		var toNg NamedGraph
		toNg = to.referencedGraphs[fromNg.baseIRI]

		// if not found in referenced graphs of toStage
		if g, ok := toNg.(*namedGraph); ok && g == nil {
			// if found in localGraphs of toStage
			// merge the IBNodes in the referenced fromNG to the IBNodes in the local toNG.
			if to.localGraphs[fromNg.id] != nil {
				toNg = to.localGraphs[fromNg.id]
				GlobalLogger.Debug("referencedGraph of fromStage merge to localGraph of toStage",
					zap.String("baseIRI", toNg.(*namedGraph).baseIRI))

				// REPORT
				report.AddAction(
					ActionMergeGraph,
					fromNg,
					"merge referenced(from) -> local(to)",
					"success",
					map[string]any{"path": "ref->local"},
				)

				toNg.(*namedGraph).mergeNodes(fromNg)

			} else {
				// if referencedGraphs and localGraphs of toStage both do not have the fromNG
				// redirect the fromNG to the toStage, and update all its contained triples
				GlobalLogger.Debug("redirect the referenced graph", zap.String("baseIRI", fromNg.baseIRI))

				// REPORT
				report.AddAction(
					ActionRedirectGraph,
					fromNg,
					"redirect referenced graph to toStage",
					"success",
					nil,
				)

				fromNg.stage = to
				to.referencedGraphs[fromNg.baseIRI] = fromNg
				if err = updateNgTriples(fromNg, to); err != nil {
					return report, err
				}

				// REPORT
				report.AddAction(
					ActionUpdateTriples,
					fromNg,
					"update triples after redirect (referenced)",
					"success",
					nil,
				)
			}
		} else {
			// if found in referencedGraphs of toStage
			// merge the IBNodes in the referenced fromNG to the referenced toNG.
			GlobalLogger.Debug("merge the referenced graph", zap.String("baseIRI", fromNg.baseIRI))

			// REPORT
			report.AddAction(
				ActionMergeGraph,
				fromNg,
				"merge referenced(from) -> referenced(to)",
				"success",
				map[string]any{"path": "ref->ref"},
			)

			toNg.(*namedGraph).mergeNodes(fromNg)
		}
	}

	// Loop over all local NGs in fromStage and merge them to the toStage
	for _, fromNg := range fromImpl.localGraphs {
		var toNg NamedGraph
		toNg = to.localGraphs[fromNg.id]
		GlobalLogger.Debug("processing local graph in fromStage", zap.String("baseIRI", fromNg.baseIRI))

		// if not found in localGraphs of toStage
		if g, ok := toNg.(*namedGraph); ok && g == nil {
			// if found in referencedGraphs of toStage
			if to.referencedGraphs[fromNg.baseIRI] != nil {
				// if the toNG is a referenced NG turn it from referenced NG into a local NG
				toNg = to.referencedGraphs[fromNg.baseIRI]
				toNg.(*namedGraph).setReferenced(false)
				toNg.(*namedGraph).stage.localGraphs[toNg.ID()] = toNg.(*namedGraph)
				delete(to.referencedGraphs, toNg.(*namedGraph).baseIRI)

				GlobalLogger.Debug("referencedGraph of toStage change to localGraph",
					zap.String("baseIRI", toNg.(*namedGraph).baseIRI))

				// REPORT
				report.AddAction(
					ActionChangeToLocal,
					toNg.(*namedGraph),
					"change referenced(to) -> local(to)",
					"success",
					nil,
				)

				toNg.(*namedGraph).mergeNodes(fromNg)

				// REPORT
				report.AddAction(
					ActionMergeGraph,
					fromNg,
					"merge local(from) -> local(to)",
					"success",
					map[string]any{"path": "local->local"},
				)

			} else {
				// if localGraphs and referencedGraphs of toStage both do not have the fromNG
				// redirect the fromNG to the toStage
				GlobalLogger.Debug("redirect the local graph", zap.String("baseIRI", fromNg.baseIRI))

				// REPORT
				report.AddAction(
					ActionRedirectGraph,
					fromNg,
					"redirect local graph to toStage",
					"success",
					nil,
				)

				fromNg.stage = to
				to.localGraphs[fromNg.id] = fromNg
				// only set true manually when moving across repositories
				// otherwise, use original modified flag of fromNG, which might be true or false
				if betweenTwoRepositories {
					fromNg.flags.modified = true

					// REPORT
					report.AddAction(
						ActionMarkModified,
						fromNg,
						"mark modified when moving across repositories (local redirect)",
						"success",
						map[string]any{"betweenRepos": true},
					)
				}
				if err = updateNgTriples(fromNg, to); err != nil {
					return report, err
				}

				// REPORT
				report.AddAction(
					ActionUpdateTriples,
					fromNg,
					"update triples after redirect (local)",
					"success",
					nil,
				)
			}
		} else {
			// if found in localGraphs of toStage
			// merge the IBNodes in the local fromNG to the local toNG.
			GlobalLogger.Debug("merge the local graph", zap.String("baseIRI", fromNg.baseIRI))

			if fromNg.Equal(toNg) {
				if err = updateNgTriples(toNg.(*namedGraph), to); err != nil {
					return report, err
				}

				// REPORT
				report.AddAction(
					ActionUpdateTriples,
					toNg.(*namedGraph),
					"equal revisions; just update triples",
					"success",
					nil,
				)
				if betweenTwoRepositories {
					toNg.(*namedGraph).flags.modified = true

					// REPORT
					report.AddAction(
						ActionMarkModified,
						toNg.(*namedGraph),
						"mark modified after equal-revision update across repos",
						"success",
						map[string]any{"betweenRepos": true},
					)
				}
			} else {
				// if both NG revisions have a checkoutCommit, find a parent commit in the to Repository then:
				// 1. if fromNG is a xxx-parent of the toNG, keep te toNG, and skip the fromNG
				// 2. if the toNG is a xxx-parent of the fromNG, replace the toNG by the the fromNG
				// 3. otherwise, use a common parent to perform a 3-diff merge
				if len(fromNg.checkedOutCommits) != 0 &&
					len(toNg.(*namedGraph).checkedOutCommits) != 0 &&
					!betweenTwoRepositories {
					// check the fromNG.checkedOutCommit exist in the to Repository
					// if it is not there, return an error
					ds, err := toNg.(*namedGraph).stage.repo.Dataset(ctx, fromNg.IRI())
					if err != nil {
						return report, fmt.Errorf("failed to get dataset by ID %s: %w", fromNg.ID(), err)
					}
					// if checkedOutCommit is not same, find common parent
					if fromNg.checkedOutCommits[0] != toNg.(*namedGraph).checkedOutCommits[0] {
						commonRevision, err := ds.FindCommonParentRevision(ctx, fromNg.checkedOutCommits[0], toNg.(*namedGraph).checkedOutCommits[0])
						if err != nil {
							return report, fmt.Errorf("failed to find common parent revision: %w", err)
						}
						switch commonRevision {
						case fromNg.checkedOutCommits[0]:
							// keep toNG, skip fromNG
							if err = updateNgTriples(toNg.(*namedGraph), to); err != nil {
								return report, err
							}

							// REPORT
							report.AddAction(
								ActionSkipSameRevision,
								toNg.(*namedGraph),
								"common parent == from; keep toNG, update triples",
								"success",
								nil,
							)
						case toNg.(*namedGraph).checkedOutCommits[0]:
							// replace toNG by fromNG
							fromNg.flags.modified = true
							fromNg.stage = to
							to.localGraphs[fromNg.id] = fromNg

							for _, val := range to.localGraphs {
								updateNgTriples(val, to)
							}
							for _, val := range to.referencedGraphs {
								updateNgTriples(val, to)
							}

							// REPORT
							report.AddAction(
								ActionUpdateTriples,
								fromNg,
								"common parent == to; replace toNG with fromNG and update all triples",
								"success",
								nil,
							)
						default:
							// local->local merge
							GlobalLogger.Debug("merge the local graph", zap.String("baseIRI", fromNg.baseIRI))
							toNg.(*namedGraph).mergeNodes(fromNg)

							// REPORT
							report.AddAction(
								ActionMergeGraph,
								fromNg,
								"merge local(from) -> local(to)",
								"success",
								map[string]any{"path": "local->local"},
							)
						}
					} else { // if checkedOutCommit is same, check which one is modified
						if fromNg.flags.modified != toNg.(*namedGraph).flags.modified {
							if fromNg.flags.modified {
								// replace toNG by fromNG
								fromNg.stage = to
								to.localGraphs[fromNg.id] = fromNg

								for _, val := range to.localGraphs {
									if err = updateNgTriples(val, to); err != nil {
										return report, err
									}
								}
								for _, val := range to.referencedGraphs {
									if err = updateNgTriples(val, to); err != nil {
										return report, err
									}
								}

								// REPORT
								report.AddAction(
									ActionUpdateTriples,
									fromNg,
									"fromNG is modified; replace toNG with fromNG and update all triples",
									"success",
									nil,
								)
							} else if toNg.(*namedGraph).flags.modified {
								// keep toNG, skip fromNG
								if err = updateNgTriples(toNg.(*namedGraph), to); err != nil {
									return report, err
								}

								// REPORT
								report.AddAction(
									ActionSkipSameRevision,
									toNg.(*namedGraph),
									"toNG is modified; keep toNG, update triples",
									"success",
									nil,
								)
							}
						}
					}

				} else {
					// local->local merge
					GlobalLogger.Debug("merge the local graph", zap.String("baseIRI", fromNg.baseIRI))
					toNg.(*namedGraph).mergeNodes(fromNg)

					// REPORT
					report.AddAction(
						ActionMergeGraph,
						fromNg,
						"merge local(from) -> local(to)",
						"success",
						map[string]any{"path": "local->local"},
					)
				}
			}
		}
	}

	// update loadedGraphs' directImports
	for _, ng := range to.localGraphs {
		for importedID, importedNg := range ng.directImports {
			if _, found := to.localGraphs[importedID]; found {
				ng.directImports[importedID] = to.localGraphs[importedID]
			} else if _, found := to.referencedGraphs[importedNg.baseIRI]; found {
				ng.directImports[importedID] = to.referencedGraphs[importedNg.baseIRI]
			}
		}
	}

	// update loadedGraphs' isImportedBy
	for _, ng := range to.localGraphs {
		for isImportedNGID, isImportedNg := range ng.isImportedBy {
			if _, found := to.localGraphs[isImportedNGID]; found {
				ng.isImportedBy[isImportedNGID] = to.localGraphs[isImportedNGID]
			} else if _, found := to.referencedGraphs[isImportedNg.baseIRI]; found {
				ng.isImportedBy[isImportedNGID] = to.referencedGraphs[isImportedNg.baseIRI]
			}
		}
	}

	// update referencedGraphs' directImports
	for _, ng := range to.referencedGraphs {
		for importedID, importedNg := range ng.directImports {
			if _, found := to.localGraphs[importedID]; found {
				ng.directImports[importedID] = to.localGraphs[importedID]
			} else if _, found := to.referencedGraphs[importedNg.baseIRI]; found {
				ng.directImports[importedID] = to.referencedGraphs[importedNg.baseIRI]
			}
		}
	}

	// update referencedGraphs' isImportedBy
	for _, ng := range to.referencedGraphs {
		for isImportedNGID, isImportedNg := range ng.isImportedBy {
			if _, found := to.localGraphs[isImportedNGID]; found {
				ng.isImportedBy[isImportedNGID] = to.localGraphs[isImportedNGID]
			} else if _, found := to.referencedGraphs[isImportedNg.baseIRI]; found {
				ng.isImportedBy[isImportedNGID] = to.referencedGraphs[isImportedNg.baseIRI]
			}
		}
	}

	// clear from Stage
	for k := range fromImpl.localGraphs {
		delete(fromImpl.localGraphs, k)
	}
	fromImpl.localGraphs = nil

	for k := range fromImpl.referencedGraphs {
		delete(fromImpl.referencedGraphs, k)
	}
	fromImpl.referencedGraphs = nil

	GlobalLogger.Debug("NewMoveAndMerge Ends")

	return report, nil
}

// AlignHistory copies the repository pointer and checkout metadata
// from the given stage to this stage. For each local NamedGraph in the given stage
// that has a matching NamedGraph in this stage, the checkout values
// (checkedOutCommits, checkedOutNGRevisions, checkedOutDSRevisions) are copied.
// If a matching NamedGraph is referenced in this stage but was local in the source
// stage, it is converted to a local NamedGraph and marked as deleted, so that
// commit reflects the removal.
// This allows a stage created from e.g. RdfRead to be committed to the same
// repository as the source stage.
func (to *stage) AlignHistory(from Stage) {
	fromImpl := from.(*stage)
	to.repo = fromImpl.repo

	copyMetadata := func(toNg, fromNg *namedGraph) {
		commits := make([]Hash, len(fromNg.checkedOutCommits))
		copy(commits, fromNg.checkedOutCommits)
		toNg.checkedOutCommits = commits
		ngRevs := make([]Hash, len(fromNg.checkedOutNGRevisions))
		copy(ngRevs, fromNg.checkedOutNGRevisions)
		toNg.checkedOutNGRevisions = ngRevs
		dsRevs := make([]Hash, len(fromNg.checkedOutDSRevisions))
		copy(dsRevs, fromNg.checkedOutDSRevisions)
		toNg.checkedOutDSRevisions = dsRevs
	}

	for _, fromNg := range fromImpl.localGraphs {
		if toNg, ok := to.localGraphs[fromNg.id]; ok {
			copyMetadata(toNg, fromNg)
		} else if toNg, ok := to.referencedGraphs[fromNg.baseIRI]; ok {
			// Graph was local in from but is referenced in to.
			// This means the graph content was effectively deleted.
			to.localGraphs[toNg.id] = toNg
			delete(to.referencedGraphs, toNg.baseIRI)
			copyMetadata(toNg, fromNg)
			toNg.setReferenced(false)
			toNg.Empty()
			toNg.setDeleted(true)
		} else {
			// Graph exists in from but is missing in to: create and mark as deleted
			toNg := to.CreateNamedGraph(fromNg.IRI()).(*namedGraph)
			toNg.Empty()
			toNg.setDeleted(true)
			copyMetadata(toNg, fromNg)
		}
	}

}

// update all IBNode triples in fromNg by the content in toStage
func updateNgTriples(fromNg *namedGraph, to *stage) error {
	err := fromNg.forAllIBNodes(func(fromIBNode *ibNode) error {
		return updateIBNodeTriples(fromIBNode, to)
	})
	if err != nil {
		return wrapError(err)
	}
	return nil
}

// update all triples of an IBNode in fromNg by the content in toStage
func updateIBNodeTriples(fromIBNode *ibNode, to *stage) error {
	return fromIBNode.forAllTriplexes(func(index int, tx triplex, k triplexKind) (triplex, error) {
		GlobalLogger.Debug("update IBNode triple", zap.String("fromIBNode", fromIBNode.iriOrID()))
		p := tx.p

		newP := findIBNode(p, to)

		var o *ibNode
		var newO *ibNode
		switch resourceTypeRecursive(tx.t) {
		case resourceTypeIBNode:
			o = tx.t.asIBNode()
			newO = findIBNode(o, to)
		}

		// update txp
		if newP != nil {
			GlobalLogger.Debug("Use newP", zap.String("fragment", p.iriOrID()))
			tx.p = newP
		}
		// update tx.t
		if newO != nil {
			GlobalLogger.Debug("Use newO", zap.String("fragment", o.iriOrID()))
			tx.t = &newO.typedResource
		}

		return tx, nil
	})
}

// look for p ibNode in toStage and return it
// if not found, return nil
// if found, return new found p ibNode
func findIBNode(p *ibNode, to *stage) *ibNode {
	var newP *ibNode

	switch p.ibNodeType().(type) {
	case *ibNodeString:
		pNgId := p.ng.id
		// found Ng in toStage
		var pNgNew *namedGraph
		found := false
		for _, ng := range to.localGraphs {
			if ng.id == pNgId {
				pNgNew = ng
				found = true
			}
		}
		if !found {
			for _, ng := range to.referencedGraphs {
				if ng.id == pNgId {
					pNgNew = ng
					found = true
				}
			}
		}

		// continue looking for IBNodes, if found, use the found one
		if found {
			newPStringNode, existed := pNgNew.stringNodes[p.asStringIBNode().fragment]
			if existed {
				newP = newPStringNode.asIBNode()
			}
		} else {
			// do nothing, keep original
		}

	case *ibNodeUuid:
		pNgId := p.ng.id
		// found Ng in toStage
		var pNgNew *namedGraph
		found := false
		for _, ng := range to.localGraphs {
			if ng.id == pNgId {
				pNgNew = ng
				found = true
			}
		}
		if !found {
			for _, ng := range to.referencedGraphs {
				if ng.id == pNgId {
					pNgNew = ng
					found = true
				}
			}
		}

		// continue looking for IBNodes
		if found {
			newPUuidNode, existed := pNgNew.uuidNodes[p.asUuidIBNode().id]
			if existed {
				newP = newPUuidNode.asIBNode()
			}
		} else {
			// do nothing
		}
	default:
		panic(fmt.Sprintf("findIBNode: unknown IBNode type %T", p.ibNodeType()))
	}

	return newP
}

// merge all IBNodes in the fromNG with the IBNodes in toNG.
func (toNg *namedGraph) mergeNodes(fromNg *namedGraph) {
	// var err error
	fromNg.forAllIBNodes(func(fromIBNode *ibNode) error {
		var found bool
		var toIBNode IBNode
		switch fromIBNode.ibNodeType().(type) {
		case *ibNodeString:
			toIBNode, found = toNg.stringNodes[fromIBNode.Fragment()]
		case *ibNodeUuid:
			toIBNode, found = toNg.uuidNodes[fromIBNode.asUuidIBNode().id]
		}
		// If the IBNode(include blankNodes) does not exist, then move it over with all it's triples and remove it from the remaining fromNG
		if !found {
			GlobalLogger.Debug("move IBNode", zap.String("fromIBNode", fromIBNode.iriOrID()))

			copyNodeToNamedGraph(toNg, fromIBNode)

			if fromIBNode.IsUuidFragment() {
				toNg.uuidNodes[fromIBNode.asUuidIBNode().id] = fromIBNode.asUuidIBNode()
			} else {
				toNg.stringNodes[fromIBNode.Fragment()] = fromIBNode.asStringIBNode()
			}
			updateIBNodeTriples(fromIBNode, toNg.stage)
		} else {
			// merge the fromNode with the toNode: This is done by moving over all triples from the fromNode to the toNode.
			// While doing so all usages in subject and predicate triples of the from-IBNode in the remaining toStage needs to be updated,
			// as the toNode will cease to exist afterwards
			GlobalLogger.Debug("choose existed IBNode", zap.String("toIBNode", toIBNode.iriOrID()))
			fromIBNode.forAllTriplexes(func(index int, tx triplex, k triplexKind) (triplex, error) {
				p := tx.p

				newP := findIBNode(p, toNg.stage)

				var o *ibNode
				var newO *ibNode
				switch resourceTypeRecursive(tx.t) {
				case resourceTypeIBNode:
					o = tx.t.asIBNode()
					newO = findIBNode(o, toNg.stage)
				}

				// update txp
				if newP != nil {
					GlobalLogger.Debug("Use newP", zap.String("fragment", p.iriOrID()))
					tx.p = newP
				}

				// update tx.t
				if newO != nil {
					GlobalLogger.Debug("Use newO", zap.String("fragment", o.iriOrID()))
					tx.t = &newO.typedResource
				}

				pString := p.iriOrID()
				oString := termToString(triplexToObject(tx))
				GlobalLogger.Debug("addTriplex", zap.String("sub", toIBNode.iriOrID()), zap.String("pred", pString), zap.String("obj", oString))

				// add triplex to toIBNode if this triplex is not existed before
				switch resourceTypeRecursive(tx.t) {
				case resourceTypeIBNode:
					o = tx.t.asIBNode()
					if toIBNode.CheckTriple(tx.p, o) {
						GlobalLogger.Debug("skip existed triplex", zap.String("sub", toIBNode.iriOrID()), zap.String("pred", pString), zap.String("obj", oString))
						return tx, nil
					}
				case resourceTypeLiteral:
					li := tx.t.asLiteral()
					if toIBNode.CheckTriple(tx.p, li) {
						GlobalLogger.Debug("skip existed Literal triplex", zap.String("sub", toIBNode.iriOrID()), zap.String("pred", pString), zap.String("obj", oString))
						return tx, nil
					}
				case resourceTypeLiteralCollection:
					lic := tx.t.asLiteralCollection()
					if toIBNode.CheckTriple(tx.p, lic) {
						GlobalLogger.Debug("skip existed LiterCollection triplex", zap.String("sub", toIBNode.iriOrID()), zap.String("pred", pString), zap.String("obj", oString))
						return tx, nil
					}
				}

				switch ib := toIBNode.(type) {
				case *ibNode:
					addTriplexAtOrAfter(ib, 0, tx, k)
					if ib.typeOf == nil {
						ib.typeOf = fromIBNode.typeOf
					}
				case *ibNodeString:
					addTriplexAtOrAfter(&ib.ibNode, 0, tx, k)
					if ib.typeOf == nil {
						ib.typeOf = fromIBNode.typeOf
					}
				case *ibNodeUuid:
					addTriplexAtOrAfter(&ib.ibNode, 0, tx, k)
					if ib.typeOf == nil {
						ib.typeOf = fromIBNode.typeOf
					}
				default:
					return tx, fmt.Errorf("unrecognized IBNode type: %T", ib)
				}
				// }
				return tx, nil
			})

		}

		return nil
	})
	for importedID, ng := range fromNg.directImports {
		toNg.directImports[importedID] = ng
	}
	for isImportedID, ng := range fromNg.isImportedBy {
		toNg.isImportedBy[isImportedID] = ng
	}

	toNg.checkedOutCommits = append(toNg.checkedOutCommits, fromNg.checkedOutCommits...)
	toNg.checkedOutNGRevisions = append(toNg.checkedOutNGRevisions, fromNg.checkedOutNGRevisions...)
	toNg.checkedOutDSRevisions = append(toNg.checkedOutDSRevisions, fromNg.checkedOutDSRevisions...)
	toNg.flags.modified = true
}

// return ibNode's fragment or ID as string
func (ib *ibNode) iriOrID() string {
	if ib.IsBlankNode() {
		return "_:" + ib.ID().String()
	} else {
		return ib.IRI().String()
	}
}

func (ib *ibNode) fragOrID() string {
	if ib.IsBlankNode() {
		return "_:" + ib.ID().String()
	} else {
		return ib.Fragment()
	}
}

// return a Term as string
func termToString(tm Term) string {
	var oString string
	switch tm.TermKind() {
	case TermKindIBNode, TermKindTermCollection:
		o := tm.(*ibNode)
		oString = o.iriOrID()
	case TermKindLiteral:
		o := tm.(Literal)
		oString = literalToString(o)
	case TermKindLiteralCollection:
		oString += "( "
		tm.(LiteralCollection).ForMembers(func(index int, li Literal) {
			oString += literalToString(li) + " "
		})
		oString += ")"
	}
	return oString
}

func (s *stage) CopyAndMerge(from Stage) error { return nil }

func (to *stage) CreateNamedGraph(iri IRI) NamedGraph {
	graph := newNamedGraphIRI(to, iri, false, true)
	// log.Println("create local graph:", iri)
	GlobalLogger.Debug("create local graph", zap.String("iri", iri.String()))

	if _, found := to.localGraphs[graph.id]; found {
		panic(fmt.Sprintf("CreateNamedGraph: named graph already exists for IRI %s", iri.String()))
	}

	to.localGraphs[graph.id] = graph
	_, err := graph.createIRIStringNode("")
	if err != nil {
		panic(fmt.Sprintf("CreateNamedGraph: failed to create IRI string node for %s: %v", iri.String(), err))
	}
	return graph
}

func (to *stage) NamedGraphs() []NamedGraph {
	returnedNgs := make([]NamedGraph, 0, len(to.localGraphs))
	for _, ng := range to.localGraphs {
		returnedNgs = append(returnedNgs, ng)
	}
	return returnedNgs
}

func (to *stage) ReferencedGraphs() []NamedGraph {
	returnedNgs := make([]NamedGraph, 0, len(to.referencedGraphs))
	for _, ng := range to.referencedGraphs {
		returnedNgs = append(returnedNgs, ng)
	}
	return returnedNgs
}

func (to *stage) ForNamedGraphs(c func(ng NamedGraph) error) error {
	for _, ng := range to.localGraphs {
		if err := c(ng); err != nil {
			return err
		}
	}
	return nil
}

func (to *stage) ForReferencedNamedGraphs(c func(ng NamedGraph) error) error {
	for _, ng := range to.referencedGraphs {
		if err := c(ng); err != nil {
			return err
		}
	}
	return nil
}

func (to *stage) getLoadedGraphByID(id uuid.UUID) (*namedGraph, error) {
	if graph, found := to.localGraphs[id]; found {
		if graph == nil {
			return nil, wrapError(ErrNamedGraphNotFound)
		}
		return graph, nil
	}
	return nil, wrapError(ErrNamedGraphNotFound)
}

func (to *stage) removeEmptyReferencedGraphs() {
	for baseURI, ng := range to.referencedGraphs {
		if len(ng.stringNodes) == 0 {
			delete(to.referencedGraphs, baseURI)
		} else {
			ng.setReferenced(true)
		}
	}
}

func (to *stage) addMissingIBNodeTriplexes() error {
	for _, graph := range to.localGraphs {
		if err := graph.forAllIBNodes(func(d *ibNode) error {
			type triplesToAddT struct {
				t    *ibNode
				pair triplex
			}
			var triplesToAdd []triplesToAddT

			if err := d.forAllTriplexes(func(index int, tx triplex, k triplexKind) (_ triplex, err error) {
				if k == subjectTriplexKind && tx.p.arePredicateTriplexesTracked() {
					triplesToAdd = append(triplesToAdd, triplesToAddT{t: tx.p, pair: triplex{p: d, t: tx.t}})
				}
				return
			}); err != nil {
				return err
			}
			for _, a := range triplesToAdd {
				addTriplexAtOrAfter(a.t, 0, a.pair, predicateTriplexKind)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (to *stage) assertAccess() error {
	if to.localGraphs == nil {
		return ErrStageDeletedOrInvalid
	}
	return nil
}

// Creates a new Stage for ephemeral data as it is not linked to any SST Repository.
// To make created data in an ephemeral stage persistent the data needs to be moved or copied into another
// Stage that is linked to a SST Repository by [sst.MoveAndMerge()].
func OpenStage(mode TriplexMode) Stage {
	return &stage{
		localGraphs:              map[uuid.UUID]*namedGraph{},
		referencedGraphs:         map[string]*namedGraph{},
		assignedNamedGraphNumber: 1,
	}
}

// get referencedGraph in referencedGraphs, if not exist, creates one
func (to *stage) referencedGraphByURI(baseURI string) (ng *namedGraph) {
	err := to.assertAccess()
	if err != nil {
		panic(fmt.Sprintf("referencedGraphByURI: assertAccess failed for %s: %v", baseURI, err))
	}

	baseURI = strings.TrimRight(string(baseURI), "#")

	ng, ok := to.referencedGraphs[baseURI]
	if ok {
		return ng
	}
	ng = newNamedGraphIRI(to, IRI(baseURI), true, false)
	// log.Println("create referenced graph:", baseURI)

	_, err = ng.createIRIStringNode("")
	if err != nil {
		panic(fmt.Sprintf("referencedGraphByURI: failed to create IRI string node for %s: %v", baseURI, err))
	}
	to.referencedGraphs[baseURI] = ng
	return ng
}

func (to *stage) moveReferencedGraphsFrom(from *stage) error {
	if to == from {
		return nil
	}
	for _, g := range from.localGraphs {
		if len(g.triplexStorage) > 0 {
			return errCannotMoveReferencedGraphsWithNamedGraphContent
		}
	}
	for baseURI, ng := range from.referencedGraphs {
		_, found := to.referencedGraphs[ng.baseIRI]
		if found {
			return errReferencedGraphAlreadyExists
		}
		ng.stage = to
		to.referencedGraphs[ng.baseIRI] = ng
		delete(from.referencedGraphs, baseURI)
	}
	return nil
}

func (to *stage) Commit(ctx context.Context, message string, branch string) (Hash, []uuid.UUID, error) {
	err := to.assertAccess()
	if err != nil {
		return emptyHash, nil, err
	}

	if to.repo == nil {
		return emptyHash, nil, ErrStageNotLinkToRepository
	}

	newCommitID, modifiedDataset, err := to.repo.commitNewVersion(ctx, to, message, branch)

	// only set flag back to false when commit succeeded
	if err == nil {
		for _, ng := range to.localGraphs {
			ng.flags.modified = false
		}
	}

	return newCommitID, modifiedDataset, err
}

func (to *stage) vocabularyElementToIBNode(t Element) *ibNode {
	if t.Vocabulary.BaseIRI == "" || t.Name == "" {
		panic("vocabularyElementToIBNode: Element BaseIRI/Name is empty")
	}

	var ng *namedGraph
	// look for Ng in localGraphs
	for _, val := range to.localGraphs {
		if val.baseIRI == t.Vocabulary.BaseIRI {
			ng = val
		}
	}
	// if not found, continue looking for Ng in referencedGraphs
	// if Ng is still not found, create one.
	if ng == nil {
		ng = to.referencedGraphByURI(t.Vocabulary.BaseIRI)
	}

	et := ng.GetIRINodeByFragment(t.Name)
	if et == nil {
		var IsUuidFragment bool
		var uuidFragment uuid.UUID
		var err error
		var ib *ibNode

		if uuidFragment, err = uuid.Parse(t.Name); err == nil {
			IsUuidFragment = true
		} else {
			IsUuidFragment = false
		}

		if IsUuidFragment {
			ib, err = ng.createIriUUIDNode(uuidFragment)

		} else {
			ib, err = ng.createIRIStringNode(t.Name)
		}
		if err != nil {
			panic(fmt.Sprintf("vocabularyElementToIBNode: failed to create IBNode for %s/%s: %v", t.Vocabulary.BaseIRI, t.Name, err))
		}

		return ib
	}
	return et.(*ibNode)
}

func (to *stage) Repository() Repository {
	return to.repo
}

func (to *stage) namedGraphByUUID(id uuid.UUID) NamedGraph {
	var found bool
	var graph NamedGraph

	err := to.assertAccess()
	if err != nil {
		return nil
	}

	if graph, found = to.localGraphs[id]; found {
		return graph
	}

	return nil
}

func (to *stage) NamedGraph(iri IRI) NamedGraph {
	newUUID, err := iriToUUID(iri)
	if err != nil {
		GlobalLogger.Error("NamedGraph: invalid IRI", zap.String("iri", string(iri)), zap.Error(err))
		return nil
	}

	return to.namedGraphByUUID(newUUID)
}

func (to *stage) ReferencedGraph(iri IRI) NamedGraph {
	var found bool
	var graph NamedGraph

	err := to.assertAccess()
	if err != nil {
		return nil
	}

	if graph, found = to.referencedGraphs[iri.String()]; found {
		return graph
	}

	return nil
}

func (g *namedGraph) ForUndefinedIBNodes(callback func(IBNode) error) error {
	err := g.forAllIBNodes(func(d *ibNode) error {
		err := d.forAll(func(_ int, s, predicate *ibNode, object Term) error {
			// This is a subject triple
			if s == d {
				return errBreakFor
			}
			return nil
		})
		// if d has subject triple
		if err != nil {
			if err == errBreakFor { //nolint:errorlint
				return nil
			}
			return err
		}
		// if d does not have subject triple
		return callback(d)
	})
	return err
}

// IBNodeByVocabulary locates the IBNode with the IRI for the specified vocabulary element.
func (to *stage) IBNodeByVocabulary(t Elementer) (IBNode, error) {
	err := to.assertAccess()
	if err != nil {
		return nil, err
	}

	ve := t.VocabularyElement()

	et := to.vocabularyElementToIBNode(ve)
	return et, err
}

// Close() makes the contents of the stage nil.
func (to *stage) Close() {
	for _, g := range to.localGraphs {
		for _, node := range g.stringNodes {
			node.Delete()
		}

		for _, node := range g.uuidNodes {
			node.Delete()
		}

		g.stage = nil
		g.triplexStorage = nil
		g.triplexKinds = nil
	}
	for _, ng := range to.referencedGraphs {
		if ng != nil {
			// Note: We don't delete nodes from referenced graphs here because
			// referenced nodes are read-only and cannot be modified/deleted.
			// The referenced graph structure is still cleaned up by nil'ing the fields.
			ng.stage = nil
			ng.triplexStorage = nil
			ng.triplexKinds = nil
		}
	}

	to.localGraphs, to.referencedGraphs = nil, nil
	to.repo = nil
}

// IsValid returns true if the Stage is valid and represents the non-zero value of Stage type.
// Methods such as MoveAndMerge() results in invalid Stages.
// References to invalid Stages should be removed so that they can be taken care by garbage collector.
func (to *stage) IsValid() bool {
	return to != nil && to.assertAccess() == nil
}

// modifiedDatasets returns a map of dataset UUIDs that needs to been modified in the stage according to the modified flag of NamedGraphs.
// e.g. If NG-A imports NG-B, and NG-B is modified, then both NG-A and NG-B's Dataset-Revision-Hash need to be modified.
func (to *stage) modifiedDatasets() map[uuid.UUID]struct{} {
	modifiedDatasetsMap := make(map[uuid.UUID]struct{})

	for _, ng := range to.localGraphs {
		if ng.flags.modified {
			// fmt.Println("modified NG of the stage:", ng.baseIRI)
			modifiedDatasetsMap[ng.id] = struct{}{}

			var foundAllImportedDatasets func(ng *namedGraph, modifiedDatasets map[uuid.UUID]struct{})
			foundAllImportedDatasets = func(ng *namedGraph, modifiedDatasets map[uuid.UUID]struct{}) {
				for ngID, tempNg := range ng.isImportedBy {
					if _, found := modifiedDatasets[ngID]; found {
						continue
					}
					modifiedDatasets[ngID] = struct{}{}
					// fmt.Println("modified NG added into map:", tempNg.baseIRI)
					foundAllImportedDatasets(tempNg, modifiedDatasets)
				}
			}

			foundAllImportedDatasets(ng, modifiedDatasetsMap)
		}
	}

	return modifiedDatasetsMap
}
