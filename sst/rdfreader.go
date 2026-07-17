// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// A parseOption allows to customize the behavior of a reader.
type parseOption int

// Options which can configure a reader.
const (
	// base IRI to resolve relative IRIs against (for formats that support
	// relative IRIs: Turtle, RDF/XML, TriG, JSON-LD)
	base parseOption = iota

	// Strict mode determines how the reader responds to errors.
	// When true (the default), it will fail on any malformed input. When
	// false, it will try to continue parsing, discarding only the malformed
	// parts.
	// Strict

	// ErrOut
)

// tripleReader parses RDF documents (serializations of an RDF graph).
//
// For streaming parsing, use the read() method to read a single Triple
// at a time. Or, if you want to read the whole document in one go, use readAll().
//
// The reader can be instructed with numerous options. Note that not all options
// are supported by all formats. Consult the following table:
//
//	Option      Description        Value      (default)       Format support
//	------------------------------------------------------------------------------
//	Base        Base IRI           IRI        (empty IRI)     Turtle, RDF/XML
//	Strict      Strict mode        true/false (true)          TODO
//	ErrOut      Error output       io.Writer  (nil)           TODO
type tripleReader interface {
	// read parses a RDF document and return the next valid triple.
	// It returns io.EOF when the whole document is parsed.
	read() (rdfTriple, error)

	// readAll parses the entire RDF document and return all valid
	// triples, or an error.
	readAll() ([]rdfTriple, error)

	// setOption sets a parsing option to the given value. Not all options
	// are supported by all serialization formats.
	setOption(parseOption, interface{}) error
}

// newTripleReader returns a new TripleReader capable of parsing triples
// from the given io.Reader in the given serialization format.
func newTripleReader(r io.Reader, f RdfFormat) tripleReader {
	switch f {
	case RdfFormatTurtle:
		return newTTLReader(r)
	case RdfFormatTriG:
		return newTriGReader(r)
	default:
		panic(fmt.Errorf("read for serialization format %v not implemented", f))
	}
}

// These errors represent conditions that may occur during NamedGraph read function invocations.
var (
	ErrGraphNotEmpty               = errors.New("graph not empty")
	ErrIllegalTripleSubject        = errors.New("illegal triple subject")
	ErrUnsupportedLiteral          = errors.New("unsupported literal")
	ErrGenericIRINotAllowed        = errors.New("generic IRI not allowed")
	ErrContentCouldNotBeRecognized = errors.New("content could not be recognized")
	ErrMissingContent              = errors.New("missing content")
	ErrMultipleGraphsNotSupported  = errors.New("multiple graphs not supported")
	ErrStageNotFound               = errors.New("stage not found")
	ErrBlankNodeInMultipleGraphs   = errors.New("blank node label appears in multiple graph blocks")
)

// rdfReader is the interface that defines access to decoded Triple instances.
// Call to read() returns next Triple or error that indicates reading error.
// The reading loop should be terminated in io.EOF error is returned. The other kinds of
// errors may be recoverable but that is implementation specific.
type rdfReader interface {
	read() (rdfTriple, error) // read returns next triple or error.
}

// readErrorHandler defines parse error handling function that is called when reader
// returns error other than io.EOF. If the reader is recoverable then this function
// should return nil. Returning non nil error terminates the reading loop.
type readErrorHandler func(error) error

// RecoverHandler and StrictHandler provides predefined recover functions.
// RecoverHandler ignores given parse error and indicates that reading loop should continue.
// StrictHandler returns given parse error causing reading loop termination.
var (
	RecoverHandler = readErrorHandler(func(error) error { return nil })
	StrictHandler  = readErrorHandler(func(err error) error { return err })
)

var (
	rdfRest       = Element{Vocabulary: rdfVocabulary, Name: "rest"}
	rdfFirst      = Element{Vocabulary: rdfVocabulary, Name: "first"}
	rdfNil        = Element{Vocabulary: rdfVocabulary, Name: "nil"}
	owlVocabulary = Vocabulary{BaseIRI: "http://www.w3.org/2002/07/owl"}
	owlOntology   = Element{Vocabulary: owlVocabulary, Name: "Ontology"}
	owlImports    = Element{Vocabulary: owlVocabulary, Name: "imports"}
)

// RdfRead reads rdf from given bufio.Reader with given rdf format and creates
// a new NamedGraph from file content. The NamedGraph is created as a default named graph
// in a new Stage. This function returns the created Stage.
func RdfRead(reader *bufio.Reader, f RdfFormat, readError readErrorHandler, Mode TriplexMode) (st Stage, err error) {
	r := lineNumberReader{reader: reader}
	tripleReader := newTripleReader(&r, f)

	// Use TriG-specific processing for TriG format
	if f == RdfFormatTriG {
		trigRdr := tripleReader.(*trigReader)
		st, err = fromTriGReaderToStage(trigRdr, func(err error) error {
			return readError(newFileLineError("", r.line(), err))
		})
	} else {
		st, err = fromReaderToStage(tripleReader, func(err error) error {
			return readError(newFileLineError("", r.line(), err))
		})
	}

	if err != nil {
		if _, ok := err.(fileLineError); !ok { //nolint:errorlint
			err = newFileLineError("", r.line(), err)
		}
	}
	return
}

// fromReaderToStage reads RDF data from the provided rdfReader and converts it into a Stage.
// It handles errors using the provided readErrorHandler and processes the RDF triples to
// create nodes and statements in the Stage.
//
// Parameters:
//   - rdr: An rdfReader that provides the RDF triples to be read.
//   - readError: A readErrorHandler that handles errors encountered during reading.
//
// Returns:
//   - Stage: The constructed Stage containing the RDF data.
//   - error: An error if any occurred during the reading or processing of the RDF data.
func fromReaderToStage(
	rdr rdfReader, readError readErrorHandler,
) (Stage, error) {
	newStage := OpenStage(DefaultTriplexMode)

	blankNodes := map[string]IBNode{}
	context := readToStageContext{
		blankNodes: blankNodes,
	}

	skipAOntology := false
	// type Triple struct {
	// 	Subj RdfTypeSubject
	// 	Pred RdfTypePredicate
	// 	Obj  RdfTypeObject
	// }
	for triple, err := rdr.read(); !errors.Is(err, io.EOF); triple, err = rdr.read() {
		// fmt.Printf("triple %s %s %s\n", triple.Subj, triple.Pred, triple.Obj)
		if err != nil {
			err = readError(err)
			if err != nil {
				return newStage, err
			}
			continue
		}
		sub := triple.Subj
		s, err := convertTripleSubjectInStage(sub, newStage, context)
		if err != nil {
			err = readError(fmt.Errorf("%w from triple %v", err, triple))
			if err != nil {
				return newStage, err
			}
			continue
		}

		// skip a owl:Ontology in ttl file
		if !skipAOntology {
			if triple.Pred.String() == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" &&
				triple.Obj.String() == "http://www.w3.org/2002/07/owl#Ontology" {
				skipAOntology = true
				continue
			}
		}

		pred := triple.Pred
		p := iriToNodeInStage(pred.(IRI), newStage)
		obj := triple.Obj
		o, err := convertTripleObjectInStage(obj, newStage, context)
		if err != nil {
			err = readError(fmt.Errorf("%w from triple %v", err, triple))
			if err != nil {
				return newStage, err
			}
			continue
		}
		err = s.AddStatementAlt(p, o)
		if err != nil {
			err = readError(fmt.Errorf("%w from triple %v", err, triple))
			if err != nil {
				return newStage, err
			}
			continue
		}
		GlobalLogger.Debug("addStatement", zap.String("subject", s.iriOrID()),
			zap.String("predicate", p.iriOrID()), zap.String("object", termToString(o)))
	}

	extractCollections(newStage)

	if graphs := newStage.NamedGraphs(); len(graphs) > 0 {
		if err := processImports(newStage, graphs[0]); err != nil {
			return newStage, err
		}
	}

	return newStage, nil
}

func processImports(stage Stage, graph NamedGraph) error {
	selfSubj := graph.GetIRINodeByFragment("")
	if selfSubj == nil {
		err := stage.addMissingIBNodeTriplexes()
		return err
	}
	var selfImportIDs []uuid.UUID
	var selfImportNodes []IBNode
	type po struct {
		predicate IBNode
		object    Term
	}
	var selfRemainingTriples []po
	tripleCount := 0
	err := selfSubj.ForAll(func(_ int, subject, predicate IBNode, object Term) error {
		if subject != selfSubj {
			return nil
		}
		tripleCount++
		if IsKindIBNode(object) {
			if predicate.Is(owlImports) {
				o := object.(IBNode)
				iid, err := uuid.Parse(o.IRI().String())
				if err == nil {
					selfImportIDs = append(selfImportIDs, iid)
				} else {
					selfImportNodes = append(selfImportNodes, o)
				}
				return nil
			} else if predicate.Is(rdfType) && object.(IBNode).Is(owlOntology) {
				return nil
			}
		}
		selfRemainingTriples = append(selfRemainingTriples, po{predicate, object})
		return nil
	})
	if err != nil {
		return err
	}
	if tripleCount == len(selfRemainingTriples) {
		return stage.addMissingIBNodeTriplexes()
	}
	var toDelete []IBNode
	for _, iid := range selfImportIDs {
		// get referenced graph
		eig := stage.referencedGraphByURI(iid.URN())
		// add import to the graph
		graph.AddImport(eig)
		eSelfNode := eig.GetIRINodeByFragment("")
		if selfSubj != nil {
			shouldDelete, err := shouldDeleteImportSelfNode(eSelfNode, selfSubj)
			if err != nil {
				return err
			}
			if shouldDelete {
				toDelete = append(toDelete, eSelfNode)
			}
		}
	}
	for _, in := range selfImportNodes {
		ib := in.IRI()
		iURI := ib.String()
		if !strings.HasSuffix(iURI, "#") {
			iURI += "#"
		}
		eig := stage.referencedGraphByURI(iURI)
		graph.AddImport(eig)
		shouldDelete, err := shouldDeleteImportSelfNode(in, selfSubj)
		if err != nil {
			return err
		}
		if shouldDelete {
			toDelete = append(toDelete, in)
		}
		// if shouldDelete {
		// 	toDelete = append(toDelete, in)
		// } else {
		// 	if in.OwningGraph() != eig {
		// 		err := eig.MoveIBNode(in, ".")
		// 		if err != nil {
		// 			return err
		// 		}
		// 	}
		// }
	}
	if err := stage.addMissingIBNodeTriplexes(); err != nil {
		return err
	}
	for _, d := range toDelete {
		d.forDeleteTriplexes(func(_ int, subject, predicate *ibNode, object Term) bool {
			if predicate.Is(owlImports) {
				return true
			} else {
				return false
			}
		})
	}
	selfSubj.(*ibNode).DeleteTriples()
	selfSubj.addAll(func() (predicate *ibNode, object Term, ok bool) {
		if len(selfRemainingTriples) > 0 {
			triple := selfRemainingTriples[0]
			selfRemainingTriples = selfRemainingTriples[1:]
			return triple.predicate.(*ibNode), triple.object, true
		}
		return
	}, false)

	return nil
}

func shouldDeleteImportSelfNode(iSelfNode IBNode, selfSubj IBNode) (bool, error) {
	var keep bool
	err := iSelfNode.ForAll(func(_ int, subject, predicate IBNode, object Term) error {
		switch object.TermKind() {
		case TermKindIBNode:
			if (subject == selfSubj && object == iSelfNode && predicate.Is(owlImports)) ||
				(subject == iSelfNode && predicate.Is(rdfType) && object.(IBNode).Is(owlOntology)) {
				return nil
			}
			keep = true
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return !keep, nil
}

// dump NamedGraph
func (g *namedGraph) Dump() {
	fmt.Fprintln(os.Stderr, "NamedGraph ID:", g.id)
	fmt.Fprintln(os.Stderr, "graph BaseURI:", g.baseIRI)
	fmt.Fprintln(os.Stderr, "graph flags: ", "isReferenced:", g.flags.isReferenced)
	fmt.Fprintln(os.Stderr, "             trackPredicates:", g.flags.trackPredicates, "   modified:", g.flags.modified)
	fmt.Fprintln(os.Stderr, "directImports", "(", len(g.directImports), ") :")
	for key := range g.directImports {
		fmt.Fprintln(os.Stderr, key)
	}

	count := 0
	fmt.Fprintln(os.Stderr, "stringNodes", "(", len(g.stringNodes), ") :")
	for _, value := range g.stringNodes {
		fmt.Fprintf(os.Stderr, "%d ", count)
		count++
		value.ibNode.dump(false)
	}
	fmt.Fprintln(os.Stderr)

	count = 0
	fmt.Fprintln(os.Stderr, "uuidNodes", "(", len(g.uuidNodes), ") :")
	for _, value := range g.uuidNodes {
		fmt.Fprintf(os.Stderr, "%d ", count)
		count++
		value.ibNode.dump(false)
	}
	fmt.Fprintln(os.Stderr)

	fmt.Fprintln(os.Stderr, "graph triplexStorage", "(", len(g.triplexStorage), "):")
	for i, triplex := range g.triplexStorage {
		if triplex.p != nil {
			fmt.Fprintf(os.Stderr, "%d ", i)
			switch triplexKindAtAbs(g, triplexOffset(i)) {
			case subjectTriplexKind:
				fmt.Fprintf(os.Stderr, " subjectTriplex ")
			case objectTriplexKind:
				fmt.Fprintf(os.Stderr, " objectTriplex ")
			case predicateTriplexKind:
				fmt.Fprintf(os.Stderr, " predicateTriplex ")
			}
			triplex.dump()
		}
	}
}

func (t *ibNode) dump(simple bool) {
	if !simple {
		fmt.Fprintln(os.Stderr, "ib:", t)
		fmt.Fprintln(os.Stderr, "typedResource.typeOf:", t.typedResource.typeOf)
		fmt.Fprintln(os.Stderr, "ib.triplexStart:", t.triplexStart, "ib.triplexEnd:", t.triplexEnd)
		fmt.Fprintln(os.Stderr, "ib.flags:", t.flags)
	}
	switch t := t.ibNodeType().(type) {
	case *ibNodeUuid:
		fmt.Fprintln(os.Stderr, "ib id:", t.id)
	case *ibNodeString:
		fmt.Fprintln(os.Stderr, "ib frag:", t.fragment)
	default:
		panic(t)
	}
}

func (t triplex) dump() {
	p := t.p
	o := triplexToObject(t)
	switch o.TermKind() {
	case TermKindIBNode, TermKindTermCollection:
		o := o.(IBNode)
		switch o.ibNodeType().(type) {
		case *ibNodeUuid:
			fmt.Fprintf(os.Stderr, " %s %s\n", p.IRI(), o.(*ibNode).asUuidIBNode().id)
		case *ibNodeString:
			fmt.Fprintf(os.Stderr, " %s %s\n", p.IRI(), o.IRI())
		default:
			panic("not known type")
		}

	case TermKindLiteral:
		dumpLiteral(p, o.(Literal))
	case TermKindLiteralCollection:
		o := o.(*literalCollection)
		fmt.Fprintf(os.Stderr, " %s %v^^%s\n", p.IRI(), o.Values(), o.Member(0).DataType().IRI())
	}
}

func dumpLiteral(p *ibNode, l Literal) {
	switch o := l.(type) {
	case String, Boolean, Integer, Double:
		if p.IsIRINode() {
			fmt.Fprintf(os.Stderr, " %s %v^^%s\n", p.IRI(), o, o.DataType().IRI())
		} else {
			fmt.Fprintf(os.Stderr, " %s %v^^%s\n", p.ID(), o, o.DataType().IRI())
		}
	case LangString:
		switch p.ibNodeType().(type) {
		case *ibNodeUuid:
			fmt.Fprintf(os.Stderr, " %s %v%v^^%s\n", p.ID(), o.Val, o.LangTag, o.DataType().IRI())
		case *ibNodeString:
			fmt.Fprintf(os.Stderr, " %s %v%v^^%s\n", p.IRI(), o.Val, o.LangTag, o.DataType().IRI())
		default:
			panic("not known type")
		}

	}
}

// fromTriGReaderToStage reads TriG data from the provided trigReader and converts it into a Stage.
// It handles multiple named graphs and properly assigns triples to their respective graphs.
func fromTriGReaderToStage(
	rdr *trigReader, readError readErrorHandler,
) (Stage, error) {
	newStage := OpenStage(DefaultTriplexMode)

	blankNodes := map[string]IBNode{}
	context := readToStageContext{
		blankNodes: blankNodes,
	}

	for triple, err := rdr.read(); !errors.Is(err, io.EOF); triple, err = rdr.read() {
		if err != nil {
			err = readError(err)
			if err != nil {
				return newStage, err
			}
			continue
		}

		// Get the graph context for this triple (captured when triple was parsed)
		currentGraphIRI := string(rdr.graphForNextTriple)

		sub := triple.Subj
		s, err := convertTripleSubjectInTriG(sub, currentGraphIRI, newStage, context)
		if err != nil {
			err = readError(fmt.Errorf("%w from triple %v", err, triple))
			if err != nil {
				return newStage, err
			}
			continue
		}

		pred := triple.Pred
		p := iriToNodeInTriG(pred.(IRI), currentGraphIRI, newStage)
		obj := triple.Obj
		o, err := convertTripleObjectInTriG(obj, currentGraphIRI, newStage, context)
		if err != nil {
			err = readError(fmt.Errorf("%w from triple %v", err, triple))
			if err != nil {
				return newStage, err
			}
			continue
		}
		err = s.AddStatementAlt(p, o)
		if err != nil {
			err = readError(fmt.Errorf("%w from triple %v", err, triple))
			if err != nil {
				return newStage, err
			}
			continue
		}
		GlobalLogger.Debug("addStatement", zap.String("graph", currentGraphIRI), zap.String("subject", s.iriOrID()),
			zap.String("predicate", p.iriOrID()), zap.String("object", termToString(o)))
	}

	extractCollections(newStage)

	// Process imports for each graph
	for _, ng := range newStage.NamedGraphs() {
		if err := processImports(newStage, ng); err != nil {
			return newStage, err
		}
	}

	return newStage, nil
}

// convertTripleSubjectInTriG converts an RDF subject to an IBNode.
// For TriG, the graph is determined by the graphIRI parameter, and the NamedGraph
// is created if it doesn't exist (as a local modifiable graph).
func convertTripleSubjectInTriG(sub rdfTypeSubject, graphIRI string, st Stage, context readToStageContext) (IBNode, error) {
	switch sub.rdfTermType() {
	case rdfTermIRI:
		base, fragment := sub.(IRI).Split()
		// Split returns fragment with leading '#', strip it
		fragment = strings.TrimPrefix(fragment, "#")
		// base is the namespace IRI, may be used for validation
		_ = base

		// Get or create the NamedGraph for this IRI
		var ng NamedGraph
		if graphIRI == "" {
			graphs := st.NamedGraphs()
			if len(graphs) > 0 {
				ng = graphs[0]
			} else {
				ng = st.CreateNamedGraph("")
			}
		} else {
			ng = st.NamedGraph(IRI(graphIRI))
			if ng == nil {
				ng = st.CreateNamedGraph(IRI(graphIRI))
			}
		}

		// if fragment is empty, this IBNode is the NG Node of this NamedGraph
		if fragment == "" {
			ib := ng.GetIRINodeByFragment("")
			if ib == nil {
				// Create the NG node if it doesn't exist
				var err error
				ib, err = ng.(*namedGraph).createIRIStringNode("")
				if err != nil {
					return nil, err
				}
			}
			return ib, nil
		}
		// get the IBNode by Fragment
		return nodeForFragment(ng, fragment), nil
	case rdfTermBlank:
		return blankToIBNodeInTriG(sub.(rdfTypeBlank), graphIRI, st, context)
	case rdfTermLiteral:
		fallthrough
	default:
		return nil, fmt.Errorf("with subject %v: %w", sub, ErrIllegalTripleSubject)
	}
}

// iriToNodeInTriG converts an IRI to an IBNode.
// For TriG, this uses the same logic as iriToNodeInStage to properly handle
// external vocabularies as referencedGraphs.
func iriToNodeInTriG(iri IRI, graphIRI string, st Stage) IBNode {
	base, fragment := iri.Split()
	// Split returns fragment with leading '#', strip it
	fragment = strings.TrimPrefix(fragment, "#")

	// For predicates and external references, use the stage's vocabulary handling
	if fragment == "" {
		// This is a complete IRI (like a predicate from external vocabulary)
		return iriToNodeInStage(iri, st)
	}

	// Check if the base matches the current graph's IRI
	if graphIRI != "" && base == graphIRI {
		ng := st.NamedGraph(IRI(graphIRI))
		if ng == nil {
			ng = st.CreateNamedGraph(IRI(graphIRI))
		}
		return nodeForFragment(ng, fragment)
	}

	// For external references (different base), use the standard stage lookup
	return iriToNodeInStage(iri, st)
}

// convertTripleObjectInTriG converts an RDF object to a Term.
// For TriG, the graph is determined by the graphIRI parameter.
func convertTripleObjectInTriG(obj rdfTypeObject, graphIRI string, stage Stage, context readToStageContext) (Term, error) {
	switch obj.rdfTermType() {
	case rdfTermIRI:
		o := iriToNodeInTriG(obj.(IRI), graphIRI, stage)
		return o, nil
	case rdfTermBlank:
		return blankToIBNodeInTriG(obj.(rdfTypeBlank), graphIRI, stage, context)
	case rdfTermLiteral:
		l := obj.(rdfTypeLiteral)
		return convertLiteral(l)
	default:
		panic(obj)
	}
}

// blankToIBNodeInTriG creates or retrieves a blank node in the appropriate graph for TriG.
// For TriG, blank nodes are scoped to the current named graph.
// If a blank node label has already been used in a different graph block, an error is returned.
func blankToIBNodeInTriG(blank rdfTypeBlank, graphIRI string, stage Stage, context readToStageContext) (IBNode, error) {
	// Get or create the NamedGraph for this blank node
	var bGraph NamedGraph
	if graphIRI == "" {
		graphs := stage.NamedGraphs()
		if len(graphs) > 0 {
			bGraph = graphs[0]
		} else {
			bGraph = stage.CreateNamedGraph("")
		}
	} else {
		bGraph = stage.NamedGraph(IRI(graphIRI))
		if bGraph == nil {
			bGraph = stage.CreateNamedGraph(IRI(graphIRI))
		}
	}

	blankLabel := blank.String()
	if d, found := context.blankNodes[blankLabel]; found {
		if d.OwningGraph() != bGraph {
			return nil, fmt.Errorf("blank node label %q appears in graph %q and graph %q: %w",
				blankLabel, d.OwningGraph().IRI(), bGraph.IRI(), ErrBlankNodeInMultipleGraphs)
		}
		return d, nil
	}
	var d IBNode
	blankUUID, err := uuid.Parse(blankLabel)
	if err == nil {
		d, err = bGraph.(*namedGraph).createBlankUuidNode(blankUUID)
		if err != nil {
			panic(err)
		}
	} else {
		d = bGraph.CreateBlankNode()
	}

	context.blankNodes[blankLabel] = d
	return d, nil
}
