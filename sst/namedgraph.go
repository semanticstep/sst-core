// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"sort"
	"strings"
	"unsafe"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrIRINodeNotFound                     = errors.New("iri node not found")
	ErrDuplicatedFragment                  = errors.New("duplicated fragment")
	errAlreadyContainsNodes                = errors.New("already contains nodes")
	errCannotMoveNamedGraphWithImports     = errors.New("can not move named graph with imports")
	ErrStagesAreNotTheSame                 = errors.New("stages are not the same")
	ErrNamedGraphAlreadyImported           = errors.New("namedGraph already imported")
	ErrNamedGraphNotImported               = errors.New("namedGraph is not imported")
	ErrNamedGraphImportCycle               = errors.New("namedGraph import cycle")
	ErrNothingToCommit                     = errors.New("nothing to commit")
	ErrNamedGraphInvalid                   = errors.New("NamedGraph is invalid")
	ErrNamedGraphAlreadyDeleted            = errors.New("NamedGraph is deleted")
	ErrInternNodeNotFound                  = errors.New("intern node not found")
	ErrInternNodesNotAccessible            = errors.New("intern nodes not accessible")
	ErrNamedGraphIsImported                = errors.New("this NamedGraph is imported by others")
	ErrIBNodeInNamedGraphIsUsedElseWhere   = errors.New("this NamedGraph is referenced by others")
	ErrReferencedNamedGraphCanNotBeDeleted = errors.New("referenced NamedGraph can not be deleted")
)

type (
	// namedGraph is NamedGraph implementation.
	// NamedGraph states:
	// 1. Created by
	//    Stage.CreateNamedGraph (no revision info)
	//    Stage.CreateNamedGraphByIRI (no revision info)
	//    Dataset.CheckoutCommit (has revision info)
	//    Dataset.CheckoutBranch (has revision info)
	//    ...
	// 2. Modified by
	//    NamedGraph.CreateIRINode
	//    NamedGraph.CreateBlankNode
	//    NamedGraph.AddStatement
	//    Stage.MoveAndMerge
	//    ...
	// 3. Committed by
	//    Stage.Commit
	namedGraph struct {
		stage *stage
		// 0 means unset;
		// -1 means pre-defined prefix;
		// 1+ means assigned prefix number
		ngNumber       int
		id             uuid.UUID
		stringNodes    map[string]*ibNodeString
		uuidNodes      map[uuid.UUID]*ibNodeUuid
		baseIRI        string
		triplexStorage []triplex // all triplexes of this namedGraph
		triplexKinds   []uint    // packed triplexKind flags.
		flags          namedGraphFlags
		directImports  map[uuid.UUID]*namedGraph
		isImportedBy   map[uuid.UUID]*namedGraph

		// checkedOutCommits - to indicate which commit this NG belonging
		// usages:
		// 0. can be got by Info() and Revision()
		// 1. can be set by SetCommitAndRevisions
		// 2. used in FindCommonParentRevision
		// 3. set by checkoutCommon
		// 4. used in Stage.Commit() to be filled in parent part, and updated after commit
		//
		// length of checkedOutCommits can be 0, 1 or more:
		//  case 0: this NG is new created and not been committed yet
		// 	case 1: this NG is loaded from Repository or committed once and has not been merged with other NGs
		// 	case more: this NG has been merged with other NGs, and other NGs checkoutCommit is stored here
		//             ONLY used for parent commits storing
		//
		// if this NG is new created and not been committed, checkedOutCommits value is emptyHash
		// if this NG is committed already, checkedOutCommits value will be set to the latest commit Hash
		checkedOutCommits []Hash

		// checkedOutNGRevisions - NGRevision(s) corresponding to checkedOutCommits.
		// Length matches checkedOutCommits (0, 1, or more).
		// For merged NGs this holds the NG-Revision of each parent commit.
		checkedOutNGRevisions []Hash

		// checkedOutDSRevisions - Dataset Revision(s) corresponding to checkedOutCommits.
		// Length matches checkedOutCommits (0, 1, or more).
		// For merged NGs this holds the DS-Revision of each parent commit.
		checkedOutDSRevisions []Hash

		// checkedOutCommit, checkedOutNGRevisions and checkedOutDSRevisions only correct when this namedGraph is not modified
		// after checkout. If modified, these values will be updated during commit.
		// checkedOutCommit, checkedOutNGRevisions and checkedOutDSRevisions only correct when this namedGraph is not modified
		// after checkout. If modified, these values will be updated during commit.
	}
)

// NamedGraph is the memory representation of an RDF named graph that contain IBNode's.
// A NamedGraph is either referenced or not.
// The purpose of a referenced NamedGraph is to hold the IBNodes that are only referenced from other NamedGraphs
// in the same Stage, but there are no subject statements recorded for these IBNodes.
// Referenced NamedGraphs cannot be changed directly, cannot be saved/committed, cannot be written out (SstWrite/RdfWrite).
// If a NamedGraph is not referenced, each IBNode in it must have at least one subject triple (eg. rdf.type).
// A NamedGraph should contain at least one subject triple, otherwise it can not be usefully written into a rdf file.
type NamedGraph interface {
	// Base returns the base IRI by which the NamedGraph is identified.
	// The IRIs of the IBNodes of the NamedGraph are constructed by concatenation the base IRI with IBNode.Fragment(), i.e. "urn:uuid:" + ID().
	IRI() IRI

	// ID returns named graph identifier as uuid.UUID.
	// If the IRI of this NamedGraph is of type a urn:uuid, the returned value is that uuid; typically a random uuid (version 4).
	// Otherwise, if the IRI is of any other type, the returned value is a NameSpaceURL hash uuid of that IRI (version 5).
	//
	// Deprecated: Use IRI() instead. NamedGraph is identified by its IRI.
	ID() uuid.UUID

	// IsValid indicates that this NamedGraph and its Stage are valid; so this NamedGraph is neither deleted nor its Stage is closed.
	IsValid() bool

	// A NamedGraph in a Stage, so one that is contained in Stage.NamedGraphs() is either in a referenced or a local state.
	// This state is indicated by IsReferenced(). If the returned value is false, this NamedGraph is in a local state.
	// IsReferenced returns true if this NamedGraph contains only IBNodes without any subject triple.
	// This happens when these IBNodes are only referenced by triples in other NamedGraphs within the same Stage,
	// and the owning NamedGraph is not imported.
	// As a consequence, a NamedGraph in referenced state is not written in to SST Repository during commit.
	IsReferenced() bool

	// IsEmpty() returns true if this NamedGraph does not contain any triples.
	// Note: Each NamedGraph has an implicit IBNode with empty fragment, representing the whole NamedGraph.
	IsEmpty() bool

	// IsModified() returns the modified state of this NamedGraph.
	IsModified() bool

	// BlankNodeCount returns the number of blank nodes in this graph
	// or error if blank nodes are not accessible by this graph.
	BlankNodeCount() int

	// IRINodeCount returns the number of IRI nodes in this graph.
	IRINodeCount() int

	// IRINodeCount returns the number of TermCollection in this graph.
	TermCollectionCount() int

	// ForAllIBNodes invokes callback function c for each of its IBNode for a given named graph.
	// In case the callback function c returns an error, the loop terminates and this method
	// is returning this error.
	ForAllIBNodes(c ForEachNode) error

	// ForIRINodes calls given callback function c for reach IRI node.
	// In case the callback function c returns an error, the loop terminates and this method
	// is returning this error.
	ForIRINodes(c ForEachNode) error

	// ForBlankNodes calls given callback function c for each blank node.
	// In case the callback function c returns an error, the loop terminates and this method
	// is returning this error.
	ForBlankNodes(c ForEachNode) error

	// ForTermCollection calls given callback function c for each TermCollection.
	// In case the callback function c returns an error, the loop terminates and this method
	// is returning this error.
	ForTermCollection(c func(TermCollection) error) error

	// ForUndefinedIBNodes is checking for IBNodes in this NamedGraph that do not have
	// any subject triple and therefore we do not know their rdf:type and invoke the callback function for them.
	// Note 1: The NG Node has a default rdf:type owl:Ontology and therefore is not considered undefined.
	// Note 2: Every IBNode must be involve in at least one triple to exist; otherwise it can't be
	//         represent in a rdf file. SST allows the existence of IBNodes that do not show up in any triple within that NamedGraph.
	//         This is not possible in any standard RDF formats e.g. Turtle.
	ForUndefinedIBNodes(c func(IBNode) error) error

	// GetSortedIBNodes returns all sorted IBNodes for this NamedGraph.
	// The result contains first all IRI Nodes sorted by their fragments and then all
	// blank Nodes sorted by their usages of IRI Nodes either directly or indirectly.
	// TBD: what will happen if the blank Nodes are not referenced by any IRI Nodes?
	GetSortedIBNodes() []IBNode

	// RdfWrite writes given NamedGraph to an rdf file with given name and of given format.
	// Currently, only the RDF format Turtle is supported.
	// The resulting file will contain a special NamedGraph-Node with empty fragment that represents the whole NamedGraph and that is of type owl:Ontology.
	// In addition, the NamedGraph-Node might have further owl:imports triples for the imported NamedGraphs.
	RdfWrite(w io.Writer, format RdfFormat) error

	// Stage returns the stage this named graph belongs to.
	Stage() Stage

	// CreateIRINode creates a new IRI IBNode with optional triples of rdf:type for the indicated types.
	// The parameter fragment indicates the fragment of the IRI Node to create.
	// If this value is an empty string, a new random UUID for the fragment is created.
	// Note: The special Node that represents the complete the NamedGraph also has an empty fragment, but it is created only implicitly and always
	// available when needed. This special NamedGraph-Node can be accessed by GetIRINodeByFragment() with empty string for the fragment parameter.
	CreateIRINode(fragment string, types ...Term) IBNode

	// CreateBlankNode creates a new blank node within the NamedGraph.
	// The returned blank node is not (yet) associated with any triple.
	CreateBlankNode(types ...Term) IBNode

	// CreateCollection returns a TermCollection, which is created by given members in the namedGraph.
	// A TermCollection with no member is treated in SST by rdf:nil.
	// a TermCollection is (at least in Turtle) treated as a BlankNode
	CreateCollection(members1 ...Term) TermCollection

	// GetIRINodeByFragment returns IBNode which is an IRI node with given fragment.
	GetIRINodeByFragment(fragment string) IBNode

	// GetBlankNodeByID returns IBNode which is a blank node with given id.
	GetBlankNodeByID(id uuid.UUID) IBNode

	// AddImport adds the NamedGraph ng to this NamedGraph.
	// When exporting to rdf/ttl, this is indicated by owl:imports.
	// Returns ErrNamedGraphAlreadyImported if ng is already imported by this NamedGraph.
	// Returns ErrNamedGraphImportCycle if adding the import would create a cycle.
	// Returns ErrStagesAreNotTheSame if ng belongs to a different Stage.
	AddImport(ng NamedGraph) error

	// RemoveImport removes the NamedGraph ng from this NamedGraph.
	// Returns ErrNamedGraphNotImported if ng is not imported by this NamedGraph.
	// Returns ErrStagesAreNotTheSame if ng belongs to a different Stage.
	RemoveImport(ng NamedGraph) error

	// DirectImports returns directly imported NamedGraphs.
	DirectImports() []NamedGraph

	// Diff compares this NamedGraph with graph2 and returns their differences as a
	// slice of DiffTriple. The comparison direction is "graph2 minus this":
	//   - TripleAdded (+1)   : the triple exists in graph2 but not in this.
	//   - TripleRemoved (-1) : the triple exists in this but not in graph2.
	//   - TripleEqual (0)    : the triple exists in both.
	//
	// The returned slice groups import-level changes first, followed by content-level
	// (triple) changes. For each changed owl:imports entry, synthetic DiffTriple rows
	// are emitted with the IRI and DatasetRevision hash combined in the Obj field:
	//
	//   Added import:   "+ <baseIRI> owl:imports <iri> #<hash>"
	//   Removed import: "- <baseIRI> owl:imports <iri> #<hash>"
	//   Modified import (same IRI, different revision):
	//                   "- <baseIRI> owl:imports <iri> #<oldHash>"
	//                   "+                             #<newHash>"
	// An error is returned if the two NamedGraph IRIs differ or if serialization fails.
	Diff(graph2 NamedGraph) ([]DiffTriple, error)

	// FindCommonParentRevision searches for a common ancestor of this and the other NamedGraph and returns this.
	// For this, the two NamedGraphs must have the same IRI and needs to be opened in different Stages
	// that are both linked to the same Repository. Only then, it is possible to compare the two revisions of the
	// same NamedGraph.
	// This method traverses the revision history of both NamedGraphs in the Repository
	// by following the commit tree that contain information about the parents.
	// This method returns either this NamedGraph or the other NamedGraph or a 3rd NamedGraph
	// with the same IRI in a new 3rd Stage of the same Repository.
	// This method returns nil if there is no common parent revision.
	// Note: This method is used for three-way diff and merge.
	FindCommonParentRevision(ctx context.Context, other NamedGraph) NamedGraph

	// MoveIBNode moves the specified IBNode `s` from its owning NamedGraph to the receiver NamedGraph.
	// If an IBNode with the same fragment already exists in the target graph, it will result in panic.
	// If fragment is non-empty, it will be used for the new IRI of Node.
	// TODO: The NG Node (fragment="") cannot be moved, it will result in panic.
	// The function checks access permissions and ensures both graphs belong to the same stage before moving.
	MoveIBNode(s IBNode, fragment string)

	// SstWrite writes NamedGraph into given writer using SST file format.
	SstWrite(w io.Writer) error

	// Empty() removes all IBNodes that are not referenced from other NamedGraphs in the same Stage.
	// Unlike the Delete() method, Empty() keeps the NamedGraph valid.
	Empty()

	// Delete() tries to remove a NamedGraph if it is not imported by any other NamedGraphs in the same Stage.
	// If the NamedGraph is imported by any other NamedGraphs in the same Stage, the error ErrNamedGraphIsImported is returned.
	// Otherwise, all IBNodes in this NamedGraph that are not referenced by IBNodes of other NamedGraphs are deleted.
	// If this NamedGraph still contains IBNodes that are referenced by IBNodes of other NamedGraphs, in the same Stage,
	// then this NamedGraph is turned into a referenced one. Otherwise, this NamedGraph is completely removed from the
	// Stage and NamedGraph.IsValid() will return false.
	// Note: If only the content of the NamedGraph to be removed, use Empty() method.
	//       When committing this Stage, a new DatasetRevision of this deleted NamedGraph is created, and the corresponding NamedGraphRevision
	//       filed is set to HashNil(). There is no new entry created in NamedGraphRevision bucket.
	//       Then, when this Dataset is accessed in future, Repository.Dataset() method could got this Dataset.
	//       But if Dataset.CheckoutCommit() or Dataset.CheckoutBranch() is called to load the commit that deleting this NamedGraph,
	//       error ErrDatasetHasBeenDeleted is returned.
	//       The history of this deleted NamedGraph is still kept and could be accessed by CheckoutCommit/CheckoutBranch its previous commits.
	Delete() error

	// Equal compares this NamedGraph with another NamedGraph for structural and content equality.
	// It compares the NamedGraph IRIs, all IRIs and blankNodes with all their subject triples.
	// Returns true if the graphs are equal, false otherwise.
	// Note: The two NamedGraphs need to be in different Stages to be identical.
	//       The memory addresses of IBNodes in the two NamedGraphs are different; different Stages don't share memory.
	// TBD: add blankNodes comparison
	Equal(ng NamedGraph) bool

	// Info returns a NamedGraphInfo struct containing statistics about this NamedGraph.
	// The info includes the number of direct imported graphs, the number of indirect imported graphs,
	// and the number of triples in the named graph.
	//
	// Returns:
	// - NamedGraphInfo: A struct containing the following fields:
	//   - NumberOfDirectImportedGraphs: The number of directly imported named graphs.
	//   - NumberOfIndirectImportedGraphs: The number of indirectly imported named graphs.
	//   - NumberOfTriples: The total number of triples in the named graph.
	//
	// The function performs the following steps:
	// 1. Initializes counters for direct imported graphs, indirect imported graphs, and triples.
	// 2. Counts the number of direct imported graphs by getting the length of the directImports map.
	// 3. Defines a recursive function to count the number of indirect imported graphs.
	// 4. Iterates over the directImports map and applies the recursive function to count indirect imports.
	// 5. Iterates over the triplexStorage slice and counts the number of triples.
	Info() NamedGraphInfo

	// SetCommits sets 0, 1, or many CheckoutCommit values that are to be used for further Commit operations
	// on the owning Stage resulting in new parent commits.
	// Note: This method is only to be used for NamedGraph in the fromStage in MoveAndMerge method or when manually merging is performed.
	//       In all other cases, SST takes care of all the correct parent commits when making a new commit.
	SetCommits(Commits ...Hash)

	allocateTriplexes(count int)

	// createAllocatedNode creates or retrieves an IBNode of the given type in this NamedGraph
	// and reserves the specified number of triplex slots for it.
	// The node is positioned at triplexStart in the graph's triplex storage.
	// It panics if the graph is nil, if the fragment is duplicated, or on any other fatal error.
	createAllocatedNode(fragment string, ibFlag ibNodeFlag, triplexStart int, allocatedTriplexCnt int) (IBNode, int)

	getNgNumber() int

	// method for debugging purpose that prints out the content of this NamedGraph into os.Stderr.
	// Dump() will print all triples in a list format.
	// Example:
	// TTL format:
	// :b750eaf5-55ff-4679-bb82-2cb01c22d1b2	a	lci:Individual ;
	// 	    rdfs:label	"HelloWorld" .
	// Output:
	// 	2025/10/21 13:35:41 NamedGraph ID: eedb4dec-f20c-4a44-aeb4-57bc3f684798
	// 	2025/10/21 13:35:41 graph BaseURI: urn:uuid:eedb4dec-f20c-4a44-aeb4-57bc3f684798
	// 	2025/10/21 13:35:41 graph flags:  isReferenced: false
	// 	2025/10/21 13:35:41              trackPredicates: true    modified: true
	// 	2025/10/21 13:35:41 directImports ( 0 ) :
	// 	2025/10/21 13:35:41 stringNodes ( 1 ) :
	// 	2025/10/21 13:35:41 0
	// 	2025/10/21 13:35:41 ib: &{{<nil>} 0xc000b1e000 0 0 0}
	// 	2025/10/21 13:35:41 typedResource.typeOf: <nil>
	// 	2025/10/21 13:35:41 ib.triplexStart: 0 ib.triplexEnd: 0
	// 	2025/10/21 13:35:41 ib.flags: 0
	// 	2025/10/21 13:35:41 ib frag:
	// 	2025/10/21 13:35:41
	// 	2025/10/21 13:35:41 uuidNodes ( 1 ) :
	// 	2025/10/21 13:35:41 0
	// 	2025/10/21 13:35:41 ib: &{{0xc000bf3aa0} 0xc000b1e000 0 6 1}
	// 	2025/10/21 13:35:41 typedResource.typeOf: &{{<nil>} 0xc000b1e380 0 6 0}
	// 	2025/10/21 13:35:41 ib.triplexStart: 0 ib.triplexEnd: 6
	// 	2025/10/21 13:35:41 ib.flags: 1
	// 	2025/10/21 13:35:41 ib id: b750eaf5-55ff-4679-bb82-2cb01c22d1b2
	// 	2025/10/21 13:35:41
	// 	2025/10/21 13:35:41 graph triplexStorage ( 6 ):
	// 	2025/10/21 13:35:41 0
	// 	2025/10/21 13:35:41  subjectTriplex
	// 	2025/10/21 13:35:41  http://www.w3.org/1999/02/22-rdf-syntax-ns#type http://ontology.semanticstep.net/lci#Individual
	// 	2025/10/21 13:35:41 1
	// 	2025/10/21 13:35:41  subjectTriplex
	// 	2025/10/21 13:35:41  http://www.w3.org/2000/01/rdf-schema#label HelloWorld^^http://www.w3.org/2001/XMLSchema#string
	Dump()

	// method for debugging purpose that prints out all IBNodes with their triples into standard output.
	// PrintTriples() will print all triples in Subject-Predicate-Object format.
	// Example:
	// TTL format:
	// :b750eaf5-55ff-4679-bb82-2cb01c22d1b2	a	lci:Individual ;
	// 	    rdfs:label	"HelloWorld" .
	// Output:
	// 	IBNode: urn:uuid:eedb4dec-f20c-4a44-aeb4-57bc3f684798#b750eaf5-55ff-4679-bb82-2cb01c22d1b2
	// 	http://www.w3.org/1999/02/22-rdf-syntax-ns#type http://ontology.semanticstep.net/lci#Individual
	// 	IBNode: urn:uuid:eedb4dec-f20c-4a44-aeb4-57bc3f684798#b750eaf5-55ff-4679-bb82-2cb01c22d1b2
	// 	http://www.w3.org/2000/01/rdf-schema#label "HelloWorld"
	PrintTriples()
}

func (g *namedGraph) SetCommits(Commits ...Hash) {
	g.checkedOutCommits = Commits
}

func (g *namedGraph) PrintTriples() {
	traversed := make(map[*ibNode]struct{})
	err := g.ForAllIBNodes(func(t IBNode) error {
		t.printTriples(0, traversed)
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func (g *namedGraph) getNgNumber() int {
	return g.ngNumber
}

// Note: Might be used in future
// func (g *namedGraph) IsParentRevision(ctx context.Context, child NamedGraph) bool {
// 	childImp := child.(*namedGraph)

// 	if g.stage.repo == nil || childImp.stage.repo == nil ||
// 		g.stage.repo != childImp.stage.repo {
// 		return false
// 	}

// 	if len(g.checkedOutCommits) == 0 {
// 		GlobalLogger.Error("This NameGraph has no commit Hash", zap.String("NamedGraph", string(g.IRI())))
// 		return false
// 	}

// 	if len(childImp.checkedOutCommits) == 0 {
// 		GlobalLogger.Error("This NameGraph has no commit Hash", zap.String("NamedGraph", string(childImp.IRI())))
// 		return false
// 	}

// 	ds, err := g.stage.repo.Dataset(ctx, g.id.URN())
// 	if err != nil {
// 		GlobalLogger.Error("Dataset error", zap.Error(err))
// 		return false
// 	}

// 	commitDetail, err := ds.CommitDetailsByHash(ctx, childImp.checkedOutCommits[0])
// 	if err != nil {
// 		GlobalLogger.Error("CommitDetailsByHash error", zap.Error(err))
// 		return false
// 	}

// 	var checkParent func(*CommitDetails, map[Hash]struct{}, Hash) bool
// 	checkParent = func(cd *CommitDetails, alreadyChecked map[Hash]struct{}, lookFor Hash) bool {
// 		for _, currentCommitHash := range cd.ParentCommits[g.id] {
// 			if currentCommitHash == lookFor {
// 				return true
// 			}
// 			newCd, err := ds.CommitDetailsByHash(ctx, currentCommitHash)
// 			if err != nil {
// 				GlobalLogger.Error("CommitDetailsByHash error", zap.Error(err))
// 				return false
// 			}
// 			if checkParent(newCd, alreadyChecked, lookFor) {
// 				return true
// 			}
// 		}
// 		return false
// 	}

// 	return checkParent(commitDetail, map[Hash]struct{}{}, g.checkedOutCommits[0])
// }

func (g *namedGraph) FindCommonParentRevision(ctx context.Context, other NamedGraph) NamedGraph {
	otherNg := other.(*namedGraph)

	if g.stage.repo == nil || otherNg.stage.repo == nil ||
		g.stage.repo != otherNg.stage.repo {
		return nil
	}

	if len(g.checkedOutCommits) == 0 {
		GlobalLogger.Error("This NameGraph has no commit Hash", zap.String("NamedGraph", string(g.IRI())))
		return nil
	}

	if len(otherNg.checkedOutCommits) == 0 {
		GlobalLogger.Error("This NameGraph has no commit Hash", zap.String("NamedGraph", string(other.IRI())))
		return nil
	}

	if g.baseIRI != otherNg.baseIRI {
		return nil
	}
	ds, err := g.stage.repo.Dataset(ctx, IRI(g.baseIRI))
	if err != nil {
		GlobalLogger.Error("Dataset error", zap.Error(err))
		return nil
	}

	parentHash, err := ds.FindCommonParentRevision(ctx, g.checkedOutCommits[0], otherNg.checkedOutCommits[0])
	if err != nil {
		GlobalLogger.Error("Dataset error", zap.Error(err))
		return nil
	}

	if parentHash != HashNil() {
		switch parentHash {
		case g.checkedOutCommits[0]:
			return g
		case otherNg.checkedOutCommits[0]:
			return otherNg
		default:
			st, err := ds.CheckoutCommit(ctx, parentHash, DefaultTriplexMode)
			if err != nil {
				GlobalLogger.Error("Dataset error", zap.Error(err))
				return nil
			}
			return st.NamedGraph(g.IRI())
		}
	}

	return nil
}

func (g *namedGraph) Equal(ng NamedGraph) bool {
	ngIml := ng.(*namedGraph)

	if g.baseIRI != ngIml.baseIRI {
		return false
	}

	if len(g.stringNodes) != len(ngIml.stringNodes) {
		return false
	}
	if len(g.uuidNodes) != len(ngIml.uuidNodes) {
		return false
	}

	if len(g.triplexStorage) != len(ngIml.triplexStorage) {
		return false
	}

	isSame := true
	g.forAllIBNodes(func(fromIBNode *ibNode) error {
		switch ib := fromIBNode.ibNodeType().(type) {
		case *ibNodeString:
			_, found := ngIml.stringNodes[ib.fragment]
			if !found {
				isSame = false
				return errBreak
			}
		case *ibNodeUuid:
			_, found := ngIml.uuidNodes[ib.id]
			if !found {
				isSame = false
				return errBreak
			}
		}
		return nil
	})
	if !isSame {
		return false
	}

	err := g.forAllIBNodes(func(fromIBNode *ibNode) error {
		var found bool
		var toIBNode IBNode
		switch fromIBNode.ibNodeType().(type) {
		case *ibNodeString:
			toIBNode, found = ngIml.stringNodes[fromIBNode.Fragment()]
		case *ibNodeUuid:
			toIBNode, found = ngIml.uuidNodes[fromIBNode.asUuidIBNode().id]
		}
		if !found {
			panicf("IBNode %s not found in target NamedGraph %s during Equal", fromIBNode.iriOrID(), ngIml.IRI())
		}

		if !fromIBNode.Equal(toIBNode) {
			return errBreak
		}

		return nil
	})
	return err == nil
}

func (s *ibNode) Equal(ib IBNode) bool {
	isBaseIRIAndFragmentEqual := compareIBNodes(s, ib)
	if isBaseIRIAndFragmentEqual != 0 {
		return false
	}

	err := s.forAllTriplexes(func(index int, tx triplex, k triplexKind) (out triplex, err error) {
		// only compare subject triples
		if k != subjectTriplexKind {
			return tx, nil
		}
		hasTriple := false
		originPString := tx.p.iriOrID()
		originOString := termToString(triplexToObject(tx))

		ib.forAllTriplexes(func(index int, toTx triplex, toK triplexKind) (out triplex, err error) {
			// only look for subject triples
			if toK != subjectTriplexKind {
				return toTx, nil
			}
			toPString := toTx.p.iriOrID()
			toOString := termToString(triplexToObject(toTx))

			if originPString == toPString && originOString == toOString {
				hasTriple = true
			}
			if hasTriple {
				return toTx, errBreak
			}

			return toTx, nil
		})

		if !hasTriple {
			return tx, errBreak
		}

		return tx, nil
	})

	return err == nil
}

// iriToUUID converts a NamedGraph base IRI into a UUID for internal storage.
// If the IRI is a urn:uuid, the embedded UUID is parsed and returned directly.
// Otherwise, a deterministic type 5 UUID is derived from the IRI string using
// uuid.NameSpaceURL, ensuring that the same custom IRI always maps to the same UUID.
func iriToUUID(baseIRI IRI) (uuid.UUID, error) {
	s := strings.TrimSuffix(string(baseIRI), "#")
	if after, ok := strings.CutPrefix(s, "urn:uuid:"); ok {
		return uuid.Parse(after)
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(s)), nil
}

func newNamedGraphIRI(stage *stage, baseIRI IRI, isReferenced bool, trackPredicates bool) *namedGraph {
	var err error
	if baseIRI == "" {
		// generate a random uuid for baseIRI
		newUUID := uuid.New()
		baseIRI = IRI(newUUID.URN())
	} else {
		baseIRI, err = NewIRI(string(baseIRI))
		if err != nil {
			panic(err)
		}
	}

	baseIRI = IRI(strings.TrimRight(string(baseIRI), "#"))
	newUUID, err := iriToUUID(baseIRI)
	if err != nil {
		panic(err)
	}

	graph := namedGraph{
		baseIRI: baseIRI.String(),
		flags: namedGraphFlags{
			isReferenced:    isReferenced,
			trackPredicates: trackPredicates,
			// if this is a referenced NG, the modified should be false.
			// if this is a local NG, the modified flag should be true.
			modified: !isReferenced,
		},
		stage:                 stage,
		id:                    newUUID,
		stringNodes:           map[string]*ibNodeString{},
		uuidNodes:             map[uuid.UUID]*ibNodeUuid{},
		triplexStorage:        []triplex{},
		triplexKinds:          []uint{},
		directImports:         map[uuid.UUID]*namedGraph{},
		isImportedBy:          map[uuid.UUID]*namedGraph{},
		checkedOutNGRevisions: []Hash{},
		checkedOutDSRevisions: []Hash{},
		checkedOutCommits:     []Hash{},
	}
	return &graph
}

func (g *namedGraph) Stage() Stage {
	return g.stage
}

// NamedGraphInfo struct is the return type of the NamedGraph method Info().
// The data contained in this struct represents the state of a NamedGraph at the time when Info() was invoked.
// NamedGraphInfo is not used for any other purpose; changes on the data in NamedGraphInfo do not have any effect on NamedGraph.
type NamedGraphInfo struct {
	// same as NamedGraph method IRI()
	Iri IRI
	// same as NamedGraph method ID()
	Id uuid.UUID
	// same as NamedGraph method IsReferenced()
	IsReferenced bool
	// same as NamedGraph method IsEmpty()
	IsEmpty bool
	// same as NamedGraph method IsModified()
	IsModified bool
	// same as NamedGraph method IRINodeCount()
	NumberOfIRINodes int
	// same as NamedGraph method BlankNodeCount()
	NumberOfBlankNodes int
	// same as NamedGraph method TermCollectionCount()
	NumberOfTermCollections int
	// the number of direct imported NamedGraphs by this NamedGraph
	NumberOfDirectImportedGraphs int
	// the number of direct and indirect imported NamedGraphs by this NamedGraph
	NumberOfAllImportedGraphs int

	// the number of subject triples in current Triplex structure in memory for this NamedGraph
	NumberOfSubjectTriples int

	// the number of predicate triples in current Triplex structure in memory for this NamedGraph
	NumberOfPredicateTriples int

	// the number of object triples in current Triplex structure in memory for this NamedGraph
	NumberOfObjectTriples int

	// the number of term collection triples in current Triplex structure in memory for this NamedGraph
	NumberOfTermCollectionTriples int

	// the commits of this NamedGraph
	// If the NamedGraph is a new one, then there is no Commit (yet).
	// If the NamedGraph was checked out from an SST Repository with history then there is exactly one NamedGraph.
	// After one or several MoveAndMerge invocations there might be several Commit entries.
	// After a successful invocation of the Stage method Commit() on a repository with history support; there will be exactly one entry.
	CheckedOutCommits []Hash

	// CheckedOutNGRevisions holds the NamedGraph Revision Hash for each entry in Commits.
	// Length matches Commits. For merged NGs this contains the NG-Revision of each parent.
	CheckedOutNGRevisions []Hash

	// CheckedOutDSRevisions holds the Dataset Revision Hash for each entry in Commits.
	// Length matches Commits. For merged NGs this contains the DS-Revision of each parent.
	CheckedOutDSRevisions []Hash

	// NamedGraphRevision is the actual SHA256 hash of this NamedGraph's SST serialization.
	//
	// Behavior:
	//  - If IsModified is true: the hash is calculated on-the-fly from the current
	//    in-memory content when Info() is called.
	//  - If IsModified is false: the hash reflects the most recent checkout or
	//    commit revision stored in the repository.
	//  - If this is a new empty NamedGraph or it has been deleted: HashNil().
	NamedGraphRevision Hash

	// DatasetRevision is the actual SHA256 hash of this NamedGraph's Dataset Revision.
	// It is computed as sha256(NamedGraphRevision + all-imported-NamedGraph-Revisions...).
	//
	// Behavior:
	//  - If IsModified is true: the hash is calculated on-the-fly from the current
	//    in-memory content (including recursively calculated revisions of modified imports)
	//    when Info() is called.
	//  - If IsModified is false: the hash reflects the most recent checkout or
	//    commit revision stored in the repository.
	//  - If this is a new empty NamedGraph: HashNil().
	DatasetRevision Hash
}

func (g *namedGraph) Info() NamedGraphInfo {
	numberOfDirectImportedGraphs := 0
	numberOfAllImportedGraphs := 0
	numberOfSubjectTriples := 0
	numberOfPredicateTriples := 0
	numberOfObjectTriples := 0
	numberOfTermCollectionTriples := 0

	numberOfDirectImportedGraphs = len(g.directImports)

	var countFunc func(ng *namedGraph)
	countFunc = func(ng *namedGraph) {
		for _, tempNg := range ng.directImports {
			numberOfAllImportedGraphs++
			countFunc(tempNg)
		}

	}
	for _, ng := range g.directImports {
		countFunc(ng)
	}

	for i, value := range g.triplexStorage {
		if value.p != nil {
			switch triplexKindAtAbs(g, triplexOffset(i)) {
			case subjectTriplexKind:
				numberOfSubjectTriples++
			case objectTriplexKind:
				numberOfObjectTriples++
			case predicateTriplexKind:
				numberOfPredicateTriples++
			}
		}
	}

	err := g.ForTermCollection(func(tc TermCollection) error {
		numberOfTermCollectionTriples += tc.MemberCount()
		return nil
	})
	if err != nil {
		panic(err)
	}

	var ngRev, dsRev Hash
	if g.flags.modified {
		ngRev, dsRev = g.calculateCurrentRevisions()
	} else {
		ngRev = g.lastCheckedOutNGRevision()
		dsRev = g.lastCheckedOutDSRevision()
	}

	return NamedGraphInfo{
		Iri:                           g.IRI(),
		Id:                            g.ID(),
		IsReferenced:                  g.IsReferenced(),
		IsEmpty:                       g.IsEmpty(),
		IsModified:                    g.IsModified(),
		NumberOfIRINodes:              g.IRINodeCount(),
		NumberOfBlankNodes:            g.BlankNodeCount(),
		NumberOfTermCollections:       g.TermCollectionCount(),
		NumberOfDirectImportedGraphs:  numberOfDirectImportedGraphs,
		NumberOfAllImportedGraphs:     numberOfAllImportedGraphs,
		NumberOfSubjectTriples:        numberOfSubjectTriples,
		NumberOfPredicateTriples:      numberOfPredicateTriples,
		NumberOfObjectTriples:         numberOfObjectTriples,
		NumberOfTermCollectionTriples: numberOfTermCollectionTriples,
		CheckedOutCommits:             g.checkedOutCommits,
		CheckedOutNGRevisions:         g.checkedOutNGRevisions,
		CheckedOutDSRevisions:         g.checkedOutDSRevisions,
		NamedGraphRevision:            ngRev,
		DatasetRevision:               dsRev,
	}
}

func (g *namedGraph) lastCheckedOutNGRevision() Hash {
	if len(g.checkedOutNGRevisions) > 0 {
		return g.checkedOutNGRevisions[len(g.checkedOutNGRevisions)-1]
	}
	return emptyHash
}

func (g *namedGraph) lastCheckedOutDSRevision() Hash {
	if len(g.checkedOutDSRevisions) > 0 {
		return g.checkedOutDSRevisions[len(g.checkedOutDSRevisions)-1]
	}
	return emptyHash
}

// calculateCurrentRevisions computes the actual NamedGraphRevision and DatasetRevision
// based on the current in-memory content. This mirrors the logic in commitNewVersion
// without persisting to the repository.
func (g *namedGraph) calculateCurrentRevisions() (ngRev Hash, dsRev Hash) {
	if g.flags.deleted || g.IsEmpty() {
		ngRev = emptyHash
	} else {
		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)
		err := g.SstWrite(bw)
		if err != nil {
			panic(err)
		}
		_ = bw.Flush()
		ngRev = sha256.Sum256(buf.Bytes())
	}

	dsHasher := sha256.New()
	dsHasher.Write(ngRev[:])
	for _, importedNG := range g.allImports() {
		importImpl := importedNG.(*namedGraph)
		var importNGRev Hash
		if importImpl.flags.modified {
			importNGRev, _ = importImpl.calculateCurrentRevisions()
		} else {
			importNGRev = importImpl.lastCheckedOutNGRevision()
		}
		dsHasher.Write(importNGRev[:])
	}
	dsRev = BytesToHash(dsHasher.Sum(nil))
	return
}

func (g *namedGraph) ID() uuid.UUID {
	return g.id
}

func (g *namedGraph) IRI() IRI {
	// the NamedGraph.IRI() method must not contain a trailing "#"
	// return IRI(strings.TrimRight(g.baseIRI, "#"))
	return *(*IRI)(unsafe.Pointer(&g.baseIRI))
}

func (g *namedGraph) nodesReferencedByOthers() map[*ibNode]struct{} {
	referencedIBNodeMap := make(map[*ibNode]struct{})
	for _, ib := range g.stringNodes {
		for i, triplex := range g.triplexStorage[ib.triplexStart:ib.triplexEnd] {
			if triplex.p != nil {
				switch (&ib.ibNode).triplexKindAt(i) {
				case subjectTriplexKind:
					continue
				case objectTriplexKind, predicateTriplexKind:
					if triplex.t.asIBNode().ng != g {
						referencedIBNodeMap[&ib.ibNode] = struct{}{}
					}
				}
			}
		}
	}

	return referencedIBNodeMap
}

func (g *namedGraph) Empty() {
	nodesReferencedByOthers := g.nodesReferencedByOthers()

	for _, ib := range g.stringNodes {
		if _, found := nodesReferencedByOthers[&ib.ibNode]; found {
			ib.ibNode.deleteSubjectTriplex()
		} else {
			ib.ibNode.Delete()
		}
	}

	for _, ib := range g.uuidNodes {
		ib.ibNode.Delete()
	}

	if len(g.uuidNodes) != 0 {
		panicf("NamedGraph %s Empty: %d uuidNodes remain after deletion", g.IRI(), len(g.uuidNodes))
	}

	g.flags.modified = true
}

func (g *namedGraph) Delete() error {
	if g.IsReferenced() {
		return ErrReferencedNamedGraphCanNotBeDeleted
	}

	if g.flags.deleted {
		return ErrNamedGraphAlreadyDeleted
	}

	if !g.IsValid() {
		return ErrNamedGraphInvalid
	}

	// there is another NamedGraph in the Stage that is importing the NamedGraph to be deleted.
	if len(g.isImportedBy) != 0 {
		return ErrNamedGraphIsImported
	}

	g.Empty()

	// TODO: if this NamedGraph still contains IBNodes after empty, then it is turned into a referenced NamedGraph.
	// TO be discussed: if this NamedGraph is empty, it still can be turned into a referenced state to indicate this
	// NamedGraph is not loaded anymore and can not be modified.
	g.setReferenced(true)

	// To ensure that this NamedGraph should be committed as deleted.
	g.setDeleted(true)

	return nil
}

func (g *namedGraph) setDeleted(flag bool) {
	g.flags.deleted = flag
}

func (g *namedGraph) IsValid() bool {
	return g.stage != nil
}

func (g *namedGraph) GetIRINodeByFragment(fragment string) IBNode {
	err := g.assertAccess()
	if err != nil {
		panic(err)
	}

	var IsUuidFragment bool
	var uuidFragment uuid.UUID

	if uuidFragment, err = uuid.Parse(fragment); err == nil {
		IsUuidFragment = true
	} else {
		IsUuidFragment = false
	}

	if IsUuidFragment {
		if s, found := g.uuidNodes[uuidFragment]; found {
			return &s.ibNode
		}
	} else {
		if s, found := g.stringNodes[fragment]; found {
			return &s.ibNode
		}
	}

	return nil
}

func (g *namedGraph) ForIRINodes(c ForEachNode) error {
	err := g.assertAccess()
	if err != nil {
		return err
	}

	for _, s := range g.stringNodes {
		err := c(&s.ibNode)
		if err != nil {
			return err
		}
	}

	for _, s := range g.uuidNodes {
		if s.IsIRINode() {
			err := c(&s.ibNode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *namedGraph) forIRINodes(callback func(*ibNode) error) error {
	for _, s := range g.stringNodes {
		err := callback(&s.ibNode)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *namedGraph) IRINodeCount() int {
	count := 0
	for _, val := range g.uuidNodes {
		if val.IsIRINode() {
			count++
		}
	}

	return len(g.stringNodes) + count
}

func (g *namedGraph) CreateIRINode(fragment string, types ...Term) IBNode {
	err := g.assertAccess()
	if err != nil {
		return nil
	}

	var isUuidFragment bool
	var uuidFragment uuid.UUID
	var ib IBNode

	if fragment == "" {
		isUuidFragment = true
		uuidFragment = uuid.New()
	} else if uuidFragment, err = uuid.Parse(fragment); err == nil {
		isUuidFragment = true
	} else {
		isUuidFragment = false
	}

	if isUuidFragment {
		ib, err = g.createIriUUIDNode(uuidFragment)
		if err != nil {
			panic(err)
		}
	} else {
		ib, err = g.createIRIStringNode(fragment)
		if err != nil {
			panic(err)
		}
	}

	for _, t := range types {
		ib.AddStatement(rdfType, t)
	}
	return ib
}

func (g *namedGraph) CreateBlankNode(types ...Term) IBNode {
	err := g.assertAccess()
	if err != nil {
		return nil
	}
	s := g.createBlankUUIDNode()

	for _, t := range types {
		s.AddStatement(rdfType, t)
	}

	return s
}

// createBlankUUIDNode creates a new blank node with a random type 4 UUID.
// This is the runtime creation path; the UUID is not persisted in SST files.
func (g *namedGraph) createBlankUUIDNode() *ibNode {
	d := newUuidNode(g, uuid.New(), nil, blankNodeType|uuidNode)
	g.uuidNodes[d.id] = d

	return &d.ibNode
}

func (g *namedGraph) createBlankUuidNode(id uuid.UUID) (*ibNode, error) {
	if _, found := g.uuidNodes[id]; found {
		return nil, ErrDuplicatedFragment
	}
	d := newUuidNode(g, id, nil, blankNodeType|uuidNode)
	g.uuidNodes[d.id] = d

	return &d.ibNode, nil
}

func (g *namedGraph) createIriUUIDNode(id uuid.UUID) (*ibNode, error) {
	if _, found := g.uuidNodes[id]; found {
		return nil, ErrDuplicatedFragment
	}

	d := newUuidNode(g, id, nil, iriNodeType|uuidNode)
	g.uuidNodes[d.id] = d

	return &d.ibNode, nil
}

func (g *namedGraph) createIRIStringNode(fragment string) (*ibNode, error) {
	if _, found := g.stringNodes[fragment]; found {
		return nil, ErrDuplicatedFragment
	}

	s := newStringNode(g, fragment, nil, iriNodeType|stringNode)
	g.stringNodes[fragment] = s

	return &s.ibNode, nil
}

func (g *namedGraph) GetBlankNodeByID(fragment uuid.UUID) IBNode {
	err := g.assertAccess()
	if err != nil {
		panic(err)
	}
	if s, found := g.uuidNodes[fragment]; found {
		return &s.ibNode
	}
	return nil
}

func (g *namedGraph) ForBlankNodes(c ForEachNode) error {
	err := g.assertAccess()
	if err != nil {
		return err
	}

	for _, s := range g.uuidNodes {
		if s.IsBlankNode() {
			err := c(&s.ibNode)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *namedGraph) ForTermCollection(c func(TermCollection) error) error {
	err := g.assertAccess()
	if err != nil {
		return err
	}

	for _, s := range g.uuidNodes {
		if s.typeOf != nil && *s.typedResource.typeOf == termCollectionResourceType.ibNode {
			err := c(&s.ibNode)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *namedGraph) TermCollectionCount() int {
	err := g.assertAccess()
	if err != nil {
		panic(err)
	}

	count := 0

	for _, s := range g.uuidNodes {
		if s.IsBlankNode() {
			if s.typeOf != nil && *s.typedResource.typeOf == termCollectionResourceType.ibNode {
				count++
			}
		}
	}

	return count
}

func (g *namedGraph) BlankNodeCount() int {
	count := 0
	for _, val := range g.uuidNodes {
		if val.IsBlankNode() {
			count++
		}
	}
	return count
}

func (g *namedGraph) CreateCollection(members ...Term) TermCollection {
	err := g.assertAccess()
	if err != nil {
		panic(err)
	}

	if len(members) == 0 {
		rdfNilN, err := g.stage.IBNodeByVocabulary(rdfNil)
		if err != nil {
			panic(err)
		}
		return rdfNilN.(*ibNode)
	}

	d, err := newTermCollection(g, members...)
	if err != nil {
		panic(err)
	}

	if g.uuidNodes != nil {
		g.uuidNodes[d.(*ibNode).asUuidIBNode().id] = d.(*ibNode).asUuidIBNode()
	}

	return d
}

// IsReferenced returns true is this NamedGraph is not part of the current Stage;
// meaning this NamedGraph is neither the default graph of the Dataset
// nor a direct or indirectly imported NamedGraph of the Dataset this Stage is representing.
// When this method returns TRUE, then this is indicating that only some out of potentially many IBNodes are currently be known by this NamedGraph.
func (g *namedGraph) IsReferenced() bool {
	return g.flags.isReferenced
}

func (g *namedGraph) setReferenced(referenced bool) {
	g.flags.isReferenced = referenced
}

// RemoveImport removes the NamedGraph ng from this NamedGraph.
// Panic will be called if ng is not imported by this NamedGraph.
func (g *namedGraph) RemoveImport(ng NamedGraph) error {
	if g.stage != ng.Stage() {
		return ErrStagesAreNotTheSame
	}

	if _, found := g.directImports[ng.ID()]; !found {
		return ErrNamedGraphNotImported
	}

	delete(g.directImports, ng.ID())
	delete(ng.(*namedGraph).isImportedBy, g.id)
	g.flags.modified = true
	return nil
}

func (g *namedGraph) AddImport(ng NamedGraph) error {
	if g.stage != ng.Stage() {
		return ErrStagesAreNotTheSame
	}

	if _, found := g.directImports[ng.ID()]; found {
		return ErrNamedGraphAlreadyImported
	}

	if ng.(*namedGraph).hasImported(g) {
		return ErrNamedGraphImportCycle
	}

	g.directImports[ng.ID()] = ng.(*namedGraph)
	ng.(*namedGraph).isImportedBy[g.id] = g
	g.flags.modified = true
	return nil
}

func (g *namedGraph) hasImported(target *namedGraph) bool {
	visited := make(map[uuid.UUID]bool)
	return g.dfsForDirectImports(target, visited)
}

func (g *namedGraph) dfsForDirectImports(target *namedGraph, visited map[uuid.UUID]bool) bool {
	if g.id == target.id {
		return true
	}

	visited[g.id] = true

	for _, imported := range g.directImports {
		if !visited[imported.id] {
			if imported.dfsForDirectImports(target, visited) {
				return true
			}
		}
	}

	return false
}

// DirectImports returns directly imported NamedGraphs.
//
// See also [Imports].
func (g *namedGraph) DirectImports() []NamedGraph {
	values := make([]NamedGraph, 0, len(g.directImports))
	for _, value := range g.directImports {
		values = append(values, value)
	}
	return values
}

func (g *namedGraph) allImports() []NamedGraph {
	visited := make(map[uuid.UUID]struct{}, len(g.directImports))
	out := make([]NamedGraph, 0)

	// DFS
	stack := make([]NamedGraph, 0, len(g.directImports))
	stack = append(stack, g.DirectImports()...)

	for len(stack) > 0 {
		// pop
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		id := n.ID()
		if _, ok := visited[id]; ok {
			continue
		}
		visited[id] = struct{}{}
		out = append(out, n)

		// get indirect
		stack = append(stack, n.DirectImports()...)
	}

	return out
}

func (g *namedGraph) MoveIBNode(s IBNode, mergedFragment string) {
	err := g.assertAccess()
	if err != nil {
		panic(err)
	}
	s.OwningGraph().(*namedGraph).assertSameStageGraph(g)
	s.OwningGraph().(*namedGraph).moveNodeToNamedGraph(g, s.(*ibNode), mergedFragment)
}

func (g *namedGraph) IsEmpty() bool {
	return len(g.triplexStorage) == 0
}

func (g *namedGraph) IsModified() bool {
	return g.flags.modified
}

func (g *namedGraph) assertSameStageGraph(o *namedGraph) {
	if o == nil {
		panicf("input NamedGraph is nil")
	}

	if g.stage != o.stage {
		panic(ErrStagesAreNotTheSame)
	}
}

func (g *namedGraph) assertAccess() error {
	if g.stage == nil {
		return ErrNamedGraphInvalid
	}
	return nil
}

func (g *namedGraph) mergeNodesFrom(
	from *namedGraph,
	replacements nodeCopyReplacements,
	toC map[*namedGraph]namedGraphsAndOffset,
	fromMatch fromMatchFunc,
	f func(fragment string) string,
) error {
	return from.forAllIBNodes(func(ib *ibNode) error {
		var fragment string

		switch ib.ibNodeType().(type) {
		case *ibNodeString:
			fragment = f(ib.asStringIBNode().fragment)
		case *ibNodeUuid:
			fragment = f(ib.asUuidIBNode().id.String())
		default:
			panic("not recognized IBNode type")
		}

		t := replacements.mergeNodeToNamedGraph(ib, toC, fromMatch, fragment)

		switch ib := t.ibNodeType().(type) {
		case *ibNodeString:
			g.stringNodes[fragment] = ib
		case *ibNodeUuid:
			g.uuidNodes[ib.id] = ib
		}
		return nil
	})
}

// provide a new fragment for the IBNode
func (g *namedGraph) renameIBNodeString(d *ibNodeString, newFragment string) error {
	prevFragment := d.fragment
	d.fragment = string(newFragment)
	if _, found := g.stringNodes[d.fragment]; found {
		panic(ErrDuplicatedFragment)
	}
	g.stringNodes[d.fragment] = d
	delete(g.stringNodes, prevFragment)
	return nil
}

// provide a new fragment for the IBNode
// better be a IBNode method, no namedGraph needed.
func (g *namedGraph) renameIBNodeUuid(d *ibNodeUuid, newFragment uuid.UUID) error {
	if g.IsReferenced() {
		return ErrInternNodesNotAccessible
	}

	prevFragment := d.id
	d.id = newFragment

	if _, found := g.uuidNodes[d.id]; found {
		panic(ErrDuplicatedFragment)
	}
	g.uuidNodes[d.id] = d
	delete(g.uuidNodes, prevFragment)
	return nil
}

func (g *namedGraph) deleteSubjectTriples() error {
	for _, d := range g.stringNodes {
		d.DeleteTriples()
	}
	for _, d := range g.uuidNodes {
		d.DeleteTriples()
	}
	return nil
}

func (ng *namedGraph) deleteNode(ib *ibNode) {
	switch s := ib.ibNodeType().(type) {
	case *ibNodeString:
		delete(ng.stringNodes, s.fragment)
	case *ibNodeUuid:
		delete(ng.uuidNodes, s.id)
	case nil:
	}
}

func (g *namedGraph) deleteNodesWithNoTriples() error {
	for _, d := range g.stringNodes {
		var triplexCnt int
		err := d.forAll(func(_ int, _, _ *ibNode, _ Term) error { triplexCnt++; return nil })
		if err != nil {
			return err
		}
		if triplexCnt == 0 {
			d.Delete()
		}
	}
	for _, d := range g.uuidNodes {
		var triplexCnt int
		err := d.forAll(func(_ int, _, _ *ibNode, _ Term) error { triplexCnt++; return nil })
		if err != nil {
			return err
		}
		if triplexCnt == 0 {
			d.Delete()

		}
	}
	return nil
}

func (g *namedGraph) clearGraph() {
	g.stringNodes = nil
	g.uuidNodes = nil
	g.triplexStorage, g.triplexKinds = nil, nil
}

func (g *namedGraph) moveNodeToNamedGraph(to *namedGraph, s *ibNode, mergedFragment string) error {
	if g == to {
		return nil
	}
	switch s := s.ibNodeType().(type) {
	case *ibNodeString:
		if _, exists := to.stringNodes[mergedFragment]; exists {
			panic(ErrDuplicatedFragment)
		}
		to.stringNodes[mergedFragment] = s
		delete(g.stringNodes, s.fragment)
		s.fragment = mergedFragment
	case *ibNodeUuid:
		if to.uuidNodes != nil {
			if _, exists := to.uuidNodes[s.id]; exists {
				panic(ErrDuplicatedFragment)
			}
			to.uuidNodes[s.id] = s
		}
		if g.uuidNodes != nil {
			delete(g.uuidNodes, s.id)
		}
	}
	copyNodeToNamedGraph(to, s)
	return nil
}

func (g *namedGraph) ForAllIBNodes(callback ForEachNode) error {
	err := g.assertAccess()
	if err != nil {
		return err
	}
	return g.forAllIBNodes(func(s *ibNode) error { return callback(s) })
}

// forAllIBNodes iterates over all IBNodes in the namedGraph
// and applies the provided callback function to each node. It ensures that each
// node is visited only once by keeping track of visited nodes in a map.
//
// The callback function is expected to return an error if any issue occurs during
// its execution. If the callback returns an error, the iteration stops and the
// error is propagated back to the caller.
//
// The function first iterates over all IRI nodes in the graph using the forIRINodes
// method. For each IRI node, it applies the callback and then recursively processes
// its related nodes (subject, predicate, and object) if they are blank nodes
// and belong to the same base IRI as the namedGraph.
//
// Parameters:
// - callback: A function that takes an *ibNode as input and returns an error.
//
// Returns:
//   - error: An error if the callback function returns an error during iteration,
//     otherwise nil.
func (g *namedGraph) forAllIBNodes(callback func(d *ibNode) error) error {
	var err error
	for _, val := range g.stringNodes {
		err = callback(&val.ibNode)
		if err != nil {
			return err
		}
	}

	for _, val := range g.uuidNodes {
		err = callback(&val.ibNode)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *namedGraph) GetSortedIBNodes() (sortedNodes []IBNode) {
	sortedNodes = make([]IBNode, 0, g.IRINodeCount())
	err := g.ForAllIBNodes(func(d IBNode) error {
		sortedNodes = append(sortedNodes, d)
		return nil
	})
	if err != nil {
		panic(err)
	}
	sort.Slice(sortedNodes, func(i, j int) bool {
		return lessIBNodeThan(nil, sortedNodes[i], sortedNodes[j])
	})
	return
}

// getSortedTriplesIBNodes returns all sorted IBNodes with sorted triples
func getSortedTriplesIBNodes(g *namedGraph) (sortedNodes []IBNode, err error) {
	sortedNodes = g.GetSortedIBNodes()

	for _, d := range sortedNodes {
		da := d
		_ = da.sortAndCountTriples(func(IBNode) {}, func(IBNode) {})
	}
	return
}
