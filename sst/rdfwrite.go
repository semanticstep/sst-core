// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unsafe"
)

// ErrWriterClosed is the error returned from write() when the Triple/Quad-Writer is closed
var ErrWriterClosed = errors.New("writer is closed and cannot write anymore")

// tripleWriter serializes RDF Triples into one of the following formats:
// N-Triples, Turtle, RDF/XML.
//
// For streaming serialization, use the write() method to write a single Triple
// at a time. Or, if you want to write multiple triples in one batch, use writeAll().
// In either case; when done serializing, Close() must be called, to ensure
// that all writes are persisted, since the writer uses buffered IO.
type tripleWriter struct {
	format        RdfFormat         // Serialization format.
	w             *errWriter        // Buffered writer. Set to nil when Writer is closed.
	Namespaces    map[string]string // IRI->prefix custom mappings.
	curSubj       rdfTypeSubject    // Keep track of current subject, to enable encoding of predicate lists.
	curPred       rdfTypePredicate  // Keep track of current subject, to enable encoding of object list.
	OpenStatement bool              // True when triple statement hasn't been closed (i.e. in a predicate/object list)
	// GenerateNamespaces bool              // True to auto generate namespaces, false if you give it some custom namespaces and do not want generated ones
}

// newTripleWriter returns a new TripleWriter capable of serializing into the
// given io.Writer in the given serialization format.
func newTripleWriter(w io.Writer, f RdfFormat) *tripleWriter {
	return &tripleWriter{
		format:     f,
		w:          &errWriter{w: bufio.NewWriter(w)},
		Namespaces: make(map[string]string),
		// ns:         make(map[string]string),
		// GenerateNamespaces: true,
	}
}

// write serializes a single Triple to the io.Writer of the TripleWriter.
func (e *tripleWriter) write(ng NamedGraph, t rdfTriple) error {
	if e.w == nil {
		return ErrWriterClosed
	}
	switch e.format {
	case RdfFormatTurtle, RdfFormatTriG:
		var s, p, o string

		// object is always rendered the same
		o = e.prefixify(ng, t.Obj)

		if e.OpenStatement {
			// potentially predicate/object list
			// curSubj and curPred is set
			if rdfTermsEqual(e.curSubj, t.Subj) {
				// In predicate or object list
				if rdfTermsEqual(e.curPred, t.Pred) {
					// in object list
					s = " ,\n  "
					p = ""
				} else {
					// in predicate list
					p = e.prefixify(ng, t.Pred)

					// check if predicate introduced new prefix directive
					if e.OpenStatement {
						// in predicate list
						s = " ;\n "
						e.curPred = t.Pred
					} else {
						// previous statement closed
						e.curSubj = t.Subj
						s = e.prefixify(ng, t.Subj)
						e.curPred = t.Pred
					}
				}
			} else {
				// not in predicate/object list
				// close previous statement
				e.w.write([]byte(" .\n"))
				e.OpenStatement = false
				p = e.prefixify(ng, t.Pred)
				e.curSubj = t.Subj
				s = e.prefixify(ng, t.Subj)
				e.curPred = t.Pred
			}
		} else {
			// either first statement, or after a prefix directive
			p = e.prefixify(ng, t.Pred)
			s = e.prefixify(ng, t.Subj)
			e.curSubj = t.Subj
			e.curPred = t.Pred
		}

		// always keep statement open, in case next triple can mean predicate/object list
		e.OpenStatement = true

		e.w.write([]byte(s))
		e.w.write([]byte(" "))
		e.w.write([]byte(p))
		e.w.write([]byte(" "))
		e.w.write([]byte(o))

		if e.w.err != nil {
			return e.w.err
		}
	default:
		panic("TODO")
	}
	return nil
}

// writeAll serializes a slice of Triples to the io.Writer of the TripleWriter.
// It will ignore duplicate triples.
//
// Note that this function will modify the given slice of triples by sorting it in-place.
func (e *tripleWriter) writeAll(ts []rdfTriple) error {
	// if e.w == nil {
	// 	return ErrWriterClosed
	// }
	// switch e.format {
	// case RdfFormatTurtle:
	// 	// Sort triples by Subject, then Predicate, to maximize predicate and object lists.
	// 	sort.Sort(bySubjectThenPred(triples(ts)))

	// 	var s, p, o string

	// 	for i, t := range ts {
	// 		// object is always rendered the same
	// 		o = e.prefixify(ng, t.Obj)

	// 		if e.OpenStatement {
	// 			// potentially predicate/object list
	// 			// curSubj and curPred is set
	// 			if rdfTermsEqual(e.curSubj, t.Subj) {
	// 				// In predicate or object list
	// 				if rdfTermsEqual(e.curPred, t.Pred) {
	// 					// in object list

	// 					// check if this triple is a duplicate of the preceding triple
	// 					if i > 0 && rdfTermsEqual(t.Obj, ts[i-1].Obj) {
	// 						continue
	// 					}

	// 					s = " ,\n\t"
	// 					p = ""
	// 				} else {
	// 					// in predicate list
	// 					p = e.prefixify(ng, t.Pred)

	// 					// check if predicate introduced new prefix directive
	// 					if e.OpenStatement {
	// 						// in predicate list
	// 						s = " ;\n"
	// 						e.curPred = t.Pred
	// 					} else {
	// 						// previous statement closed
	// 						e.curSubj = t.Subj
	// 						s = e.prefixify(t.Subj)
	// 						e.curPred = t.Pred
	// 					}
	// 				}
	// 			} else {
	// 				// not in predicate/object list
	// 				// close previous statement
	// 				e.w.write([]byte(" .\n"))
	// 				e.OpenStatement = false
	// 				p = e.prefixify(t.Pred)
	// 				e.curSubj = t.Subj
	// 				s = e.prefixify(t.Subj)
	// 				e.curPred = t.Pred
	// 			}
	// 		} else {
	// 			// either first statement, or after a prefix directive
	// 			p = e.prefixify(t.Pred)
	// 			s = e.prefixify(t.Subj)
	// 			e.curSubj = t.Subj
	// 			e.curPred = t.Pred
	// 		}

	// 		// always keep statement open, in case next triple can mean predicate/object list
	// 		e.OpenStatement = true

	// 		e.w.write([]byte(s))
	// 		e.w.write([]byte("\t"))
	// 		e.w.write([]byte(p))
	// 		e.w.write([]byte("\t"))
	// 		e.w.write([]byte(o))

	// 		if e.w.err != nil {
	// 			return e.w.err
	// 		}
	// 	}
	// default:
	// 	panic("TODO")
	// }
	return nil
}

// close finalizes an s session, ensuring that any concluding tokens are
// written should it be needed (eg.g close the root tag for RDF/XML) and
// flushes the underlying buffered writer.
//
// The writer cannot write anymore when close() has been called.
func (e *tripleWriter) close() error {
	if e.OpenStatement {
		e.w.write([]byte(" .")) // Close final statement
		if e.w.err != nil {
			return e.w.err
		}
	}
	err := e.w.w.Flush()
	e.w = nil
	return err
}

// prefixify converts an rdfTerm into its prefixed form if possible.
// For IRIs, it checks if the IRI is a known type and returns a short form "a" for rdf:type.
// It attempts to split the IRI into a prefix and namespace, and uses custom namespaces if specified.
// If namespaces are to be generated, it creates a new prefix and writes the prefix declaration.
// For literals, it handles known data types normally and attempts to split unknown data types into prefix and namespace.
// It uses custom namespaces if specified or generates new prefixes if allowed.
// The function returns the prefixed form of the term or its serialized form if prefixing is not possible.
func (e *tripleWriter) prefixify(ng NamedGraph, t rdfTerm) string {
	format := e.format
	if format != RdfFormatTurtle && format != RdfFormatTriG {
		format = RdfFormatTurtle
	}

	if t.rdfTermType() == rdfTermIRI {
		if t.(IRI) == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
			return "a"
		}
		base, fragment := t.(IRI).Split()
		if fragment == "" {
			// cannot split into prefix and namespace
			return t.rdfSerialize(format)
		}

		var prefix string
		var ok bool
		// look for base in prefix map
		// in sst logic, this Namespace will be printed out always
		prefix, ok = e.Namespaces[base]
		if !ok {
			if ng != nil && ng.(*namedGraph).ngNumber != 0 && ng.(*namedGraph).ngNumber != -1 {
				prefix = fmt.Sprintf("ns%d", ng.(*namedGraph).ngNumber)
			} else {
				panic(fmt.Sprintf("prefix for %s not found, check if vocabulary is registered", base))
			}
			e.Namespaces[base] = prefix

			if e.OpenStatement {
				e.w.write([]byte(" .\n"))
			}
			e.w.write([]byte(fmt.Sprintf("@prefix %s: <%s#> .\n", prefix, base)))
			e.OpenStatement = false
		}
		return fmt.Sprintf("%s:%s", prefix, escapeLocal(fragment))
	}
	if t.rdfTermType() == rdfTermLiteral {
		switch t.(rdfTypeLiteral).DataType {
		case xsdString, xsdInteger, xsdBoolean, xsdDouble, xsdDecimal, rdfLangString:
			// serialize normally in Literal.Serialize method
			break
		default:
			first, rest := t.(rdfTypeLiteral).DataType.Split()
			if first == "" {
				return t.rdfSerialize(format)
			}

			prefix, ok := e.Namespaces[first]
			if !ok {
				custom, ok := e.Namespaces[first]
				if ok {
					// we have a custom namespace specified, use that
					prefix = custom
				} else {
					if ng != nil && ng.(*namedGraph).ngNumber != 0 && ng.(*namedGraph).ngNumber != -1 {
						prefix = fmt.Sprintf("ns%d", ng.(*namedGraph).ngNumber)
					} else {
						panic(fmt.Sprintf("prefix for %s not found, check if vocabulary is registered", first))
					}
				}
				e.Namespaces[first] = prefix
				if e.OpenStatement {
					e.w.write([]byte(" .\n"))
				}
				e.w.write([]byte(fmt.Sprintf("@prefix %s: <%s> .\n", prefix, first)))
				e.OpenStatement = false
			}
			return fmt.Sprintf("\"%s\"^^%s:%s", t.rdfSerialize(formatInternal), prefix, rest)
		}
	}
	return t.rdfSerialize(format)
}

// escapeLocal escapes a Turtle local name (PN_LOCAL) fragment so it can be
// safely appended after a prefix (e.g. ":foo"). Characters that are only
// valid via PN_LOCAL_ESC are always backslash-escaped. '-' is escaped only
// when it appears as the first character, and '.' is escaped only when it
// appears as the last character, because both are valid inside PN_LOCAL but
// not at those positions.
// See https://www.w3.org/TR/turtle/#grammar-production-PN_LOCAL
func escapeLocal(rest string) string {
	runes := []rune(rest)
	var b bytes.Buffer
	for i, r := range runes {
		needsEscape := false
		if int(r) <= 126 && int(r) >= 33 {
			switch r {
			// Characters only valid via PN_LOCAL_ESC — must always be escaped
			case '~', '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', '/', '?', '#', '@', '%':
				needsEscape = true
			// '-' is valid in PN_CHARS but not as the first character of PN_LOCAL
			case '-':
				if i == 0 {
					needsEscape = true
				}
			// '.' is valid in the middle of PN_LOCAL but not as the last character
			case '.':
				if i == len(runes)-1 {
					needsEscape = true
				}
			}
		}
		if needsEscape {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

type triples []rdfTriple

type bySubjectThenPred triples

func (t bySubjectThenPred) Len() int {
	return len(t)
}

func (t bySubjectThenPred) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t bySubjectThenPred) Less(i, j int) bool {
	// todo implement custom compressing function which returns -1 0 1 for less, equal, greater
	// https://groups.google.com/forum/#!topic/golang-nuts/5mMdKvkxWxo
	// see also bytes.Compare
	p, q := t[i].Subj.rdfSerialize(RdfFormatTurtle), t[j].Subj.rdfSerialize(RdfFormatTurtle)
	switch {
	case p < q:
		return true
	case q < p:
		return false
	default:
		// subjects are equal, continue by comparing predicates
		return t[i].Pred.rdfSerialize(RdfFormatTurtle) < t[j].Pred.rdfSerialize(RdfFormatTurtle)
	}
}

type errWriter struct {
	w   *bufio.Writer
	err error
}

func (ew *errWriter) write(buf []byte) {
	if ew.err != nil {
		return
	}
	_, ew.err = ew.w.Write(buf)
}

const (
	termLiteralCollection = rdfTermType(iota + 1024)
	termCollection
	termInlineBlankNode
)

const lineLen = 80

var (
	// ErrUnsupportedLiteral indicates unsupported literal error.
	firstIRI = rdfFirst.IRI()
	restIRI  = rdfRest.IRI()
	nilIRI   = rdfNil.IRI()
)

func (g *namedGraph) RdfWrite(w io.Writer, format RdfFormat) (err error) {
	g.stage.numberNGs()
	writer := newTripleWriter(w, format)
	return toWriter(g, writer)
}

// RdfWrite writes all NamedGraphs in this Stage to the given writer in the specified RDF format.
// Currently supports RdfFormatTriG which allows exporting multiple named graphs in a single file.
// For RdfFormatTurtle, only the first NamedGraph will be written (use NamedGraph.RdfWrite instead).
func (s *stage) RdfWrite(w io.Writer, format RdfFormat) error {
	s.numberNGs()
	writer := newTripleWriter(w, format)

	// Get all local named graphs
	graphs := s.NamedGraphs()
	if len(graphs) == 0 {
		return nil
	}

	// Sort graphs by IRI to ensure stable output order across runs.
	sort.Slice(graphs, func(i, j int) bool {
		return graphs[i].IRI().String() < graphs[j].IRI().String()
	})

	// Only TriG format supports multiple graphs
	if format != RdfFormatTriG {
		// For non-TriG formats, write only the first graph
		return toWriter(graphs[0], writer)
	}

	// For TriG format, write all graphs with named graph blocks
	return toWriterTriG(s, graphs, writer)
}

type writerContext struct {
	writer               *tripleWriter
	blankNodes           map[IBNode]rdfTypeBlank
	collectionCnt        int
	allowCollectionTerms bool
}

// https://www.w3.org/TR/turtle/#grammar-production-PN_LOCAL_ESC
var pnLocalEsc = regexp.MustCompile(`([_~.\-!$&'()*+,;=/?#@%])`)

// Writer is the interface that allows to write rdf.Triple instances in a streaming way.
// Write() is called repeatedly for all triples that should be written.
// Close() is invoked when the writing loop finishes and allows to flush internal buffers.
func toWriter(graph NamedGraph, writer *tripleWriter) (err error) {
	defer func() {
		cErr := writer.close()
		if err == nil {
			err = cErr
		}
	}()

	sortedNodes, err := getSortedTriplesIBNodes(graph.(*namedGraph))
	if err != nil {
		return err
	}

	isTurtle := tripleWriterFormat(writer) == RdfFormatTurtle
	writer.Namespaces[owlVocabulary.BaseIRI] = "owl"
	writer.Namespaces[rdfVocabulary.BaseIRI] = "rdf"
	writer.Namespaces[rdfsVocabulary.BaseIRI] = "rdfs"
	writer.Namespaces[lciVocabulary.BaseIRI] = "lci"
	writer.Namespaces[ssoVocabulary.BaseIRI] = "sso"
	writer.Namespaces[xsdVocabulary.BaseIRI] = "xsd"

	for ns, pfx := range namespaceToPrefixMap {
		writer.Namespaces[ns] = pfx
	}

	var namespaces []string
	var namespacesMap = make(map[string]struct{})
	namespaces = append(namespaces, owlVocabulary.BaseIRI)
	namespacesMap[owlVocabulary.BaseIRI] = struct{}{}

	namespaces = append(namespaces, rdfVocabulary.BaseIRI)
	namespacesMap[rdfVocabulary.BaseIRI] = struct{}{}

	namespaces = append(namespaces, xsdVocabulary.BaseIRI)
	namespacesMap[xsdVocabulary.BaseIRI] = struct{}{}

	for _, ng := range graph.Stage().ReferencedGraphs() {
		// if the referenced graph is not added
		if _, ok := namespacesMap[string(ng.IRI())]; !ok {
			// if the referenced graph has prefix in the writer, otherwise it should start with "ns" and handled afterwards
			if _, ok := writer.Namespaces[string(ng.IRI())]; ok {
				namespaces = append(namespaces, string(ng.IRI()))
				namespacesMap[string(ng.IRI())] = struct{}{}
			}
		}
	}

	// write prefixes
	if isTurtle {
		sort.Strings(namespaces)
		for _, base := range namespaces {
			prefix := writer.Namespaces[base]
			if writer.OpenStatement {
				writer.w.write([]byte(" .\n"))
			}
			writer.w.write([]byte(fmt.Sprintf("@prefix %s: <%s#> .\n", prefix, base)))
		}
		// write current NamedGraph
		writer.w.write([]byte(fmt.Sprintf("@prefix %s: <%s#> .\n", "", graph.IRI().String())))
	}

	graphDirectImports := graph.DirectImports()

	graphImportURLs := make([]IRI, 0, len(graphDirectImports))
	for _, ng := range graphDirectImports {
		graphImportURLs = append(graphImportURLs, ng.IRI())
	}
	sort.Slice(graphImportURLs, func(i, j int) bool {
		return strings.Compare(graphImportURLs[i].String(), graphImportURLs[j].String()) < 0
	})
	if isTurtle {
		// write imports
		for _, u := range graphImportURLs {
			importIRI := u

			p, s := importIRI.Split()
			if s != "" {
				pIRI, err := NewIRI(p)
				if err != nil {
					return err
				}
				writer.prefixify(graph, pIRI)
			}
		}
		// write current graph with empty prefix
		writer.Namespaces[graph.IRI().String()] = ""
		writer.prefixify(graph, graph.IRI())
	}

	encFormat := *(*RdfFormat)(unsafe.Pointer(writer))
	wc := writerContext{
		writer:               writer,
		blankNodes:           map[IBNode]rdfTypeBlank{},
		allowCollectionTerms: encFormat == RdfFormatTurtle,
	}
	var triples []rdfTriple

	// handle other prefixes
	for _, s := range sortedNodes {
		err := s.ForAll(func(index int, ts, tp IBNode, to Term) error {
			if s == ts {
				triples, err := nodeTripleToRdfTriples(index, ts, tp, to, &wc, triples[:0])
				if err != nil {
					return err
				}
				for _, triple := range triples {
					writer.prefixify(graph, triple.Subj)
					writer.prefixify(tp.OwningGraph(), triple.Pred)
					if to.TermKind() == TermKindIBNode || to.TermKind() == TermKindTermCollection {
						writer.prefixify(to.(IBNode).OwningGraph(), triple.Obj)
					}
				}
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	writeToTripleWriter(writer, ([]byte)("\n"))
	wc.collectionCnt = 0

	// handle NG Node
	for pos, s := range sortedNodes {
		// this is the NG node
		if s.IsIRINode() && s.Fragment() == "" {
			self, _ := graph.IRI().Split()
			selfIRI, err := NewIRI(self)
			// selfIRI := graph.IRI()
			if err != nil {
				return err
			}
			rdfTypeIRI := rdfType.IRI()
			owlOntologyIRI := owlOntology.IRI()
			err = writer.write(graph, rdfTriple{Subj: selfIRI, Pred: rdfTypeIRI, Obj: owlOntologyIRI})
			if err != nil {
				return err
			}
			err = s.ForAll(func(index int, ts, tp IBNode, to Term) error {
				if s == ts { // check for forward references
					if tp.Is(rdfType) && to.TermKind() == TermKindIBNode && to.(IBNode).Is(owlOntology) {
						// do nothing
						// because this triple is written above
					} else {
						triples, err := nodeTripleToRdfTriples(index, ts, tp, to, &wc, triples[:0])
						if err != nil {
							return err
						}
						for _, triple := range triples {
							triple.Subj = selfIRI
							err := writer.write(graph, triple)
							if err != nil {
								return err
							}
						}
						return nil
					}
				}
				return nil
			})
			if err != nil {
				panic(err)
			}

			for _, u := range graphImportURLs {
				importIRIStr := strings.TrimSuffix(u.String(), "#")
				importIRI, err := NewIRI(importIRIStr)
				if err != nil {
					return err
				}
				err = writer.write(graph, rdfTriple{Subj: selfIRI, Pred: owlImports.IRI(), Obj: importIRI})
				if err != nil {
					return err
				}
			}
			sortedNodes = removeElementGeneric(sortedNodes, pos)
			break
		}
	}

	// handle other Nodes
	for _, s := range sortedNodes {
		inlineCol, err := canInlineAsCollection(s)
		if err != nil {
			return err
		}
		if inlineCol && wc.allowCollectionTerms {
			continue
		}
		inlineCol, err = canInlineAsBlankNode(s)
		if err != nil {
			return err
		}
		if inlineCol && wc.allowCollectionTerms {
			continue
		}
		err = s.ForAll(func(index int, ts, tp IBNode, to Term) error {
			if s == ts { // check for forward references
				triples, err := nodeTripleToRdfTriples(index, ts, tp, to, &wc, triples[:0])
				if err != nil {
					return err
				}
				for _, triple := range triples {
					err := writer.write(graph, triple)
					if err != nil {
						return err
					}
				}
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// remove an element from a slice
func removeElementGeneric[T any](slice []T, index int) []T {
	if index < 0 || index >= len(slice) {
		return slice
	}
	return append(slice[:index], slice[index+1:]...)
}

func nodeTripleToRdfTriples(
	index int,
	ts, tp IBNode,
	to Term,
	wc *writerContext,
	triples []rdfTriple,
) ([]rdfTriple, error) {
	if tp.Is(rdfFirst) && index != 0 {
		return triples, nil
	}
	sub, _, err := toIRIOrBlankNode(ts, wc)
	if err != nil {
		return nil, err
	}
	out, err := toRdfObject(to, wc, triples)
	if err != nil {
		return nil, err
	}
	out[0].Subj = sub
	out[0].Pred = tp.IRI()
	if tp.Is(rdfFirst) {
		col, _ := ts.AsCollection()
		outL := len(out)
		if wc.allowCollectionTerms {
			oct, err := termFromTermCollection(col, 1, wc)
			if err != nil {
				return nil, err
			}
			out = append(out, rdfTriple{Obj: oct})
		} else {
			out, err = collectionToRdfTriples(func(e func(object Term)) {
				col.ForMembers(func(index int, object Term) {
					if index != 0 {
						e(object)
					}
				})
			}, out, wc)
			if err != nil {
				return nil, err
			}
		}
		sub, _, err := toIRIOrBlankNode(ts, wc)
		if err != nil {
			return nil, err
		}
		out[outL].Subj = sub
		out[outL].Pred = restIRI
	}
	return out, nil
}

func toRdfObject(o Term, wc *writerContext, triples []rdfTriple) ([]rdfTriple, error) {
	switch o.TermKind() {
	case TermKindIBNode, TermKindTermCollection:
		o := o.(IBNode)
		inlineCol, err := canInlineAsCollection(o)
		if err != nil {
			return triples, err
		}
		var obj rdfTypeObject
		if inlineCol && wc.allowCollectionTerms {
			objCollection, _ := o.AsCollection()
			obj, err = termFromTermCollection(objCollection, 0, wc)
			if err != nil {
				return nil, err
			}
		} else {
			inlineBlank, err := canInlineAsBlankNode(o)
			if err != nil {
				return nil, err
			}
			if inlineBlank && wc.allowCollectionTerms {
				obj, err = termFromInlineBlankNode(o, wc)
			} else {
				_, obj, err = toIRIOrBlankNode(o, wc)
			}
			if err != nil {
				return nil, err
			}
		}
		return append(triples, rdfTriple{Obj: obj}), err
	case TermKindLiteral:
		l := o.(Literal)
		objLiteral, err := termFromLiteral(l)
		if err != nil {
			return triples, err
		}
		return append(triples, rdfTriple{Obj: objLiteral}), nil
	case TermKindLiteralCollection:
		c := o.(*literalCollection)
		if wc.allowCollectionTerms {
			colTerm, err := termFromLiteralCollection(*c, wc.writer)
			if err != nil {
				return nil, err
			}
			return append(triples, rdfTriple{Obj: colTerm}), nil
		}
		out, err := collectionToRdfTriples(func(e func(o Term)) {
			c.forInternalMembers(func(_ int, l Literal) { e(l.(Term)) })
		}, triples, wc)
		return out, err
	}
	panic(o.TermKind())
}

// used in rdfWrite
func termFromLiteral(l Literal) (rdfTypeLiteral, error) {
	value := l.apiValue()
	dataType := l.DataType()
	dataTypeURI := dataType.IRI()
	var err error
	var objLiteral rdfTypeLiteral
	switch dataType {
	case &literalTypeString.ibNode:
		objLiteral, err = newLiteral(string(value.(String)))
	case &literalTypeLangString.ibNode:
		objLiteral, err = rdfNewLangLiteral(value.(LangString).Val, value.(LangString).LangTag)
	case &literalTypeDouble.ibNode:
		double := float64(value.(Double))
		doubleConverted := strconv.AppendFloat(make([]byte, 0, 26), double, 'g', -1, 64)
		if bytes.IndexByte(doubleConverted, '.') < 0 && bytes.IndexByte(doubleConverted, 'e') < 0 {
			doubleConverted = append(doubleConverted, '.', '0')
		}
		objLiteral = rdfNewTypedLiteral(string(doubleConverted), dataTypeURI)
	case &literalTypeFloat.ibNode:
		f := float64(value.(Float))
		floatConverted := strconv.AppendFloat(make([]byte, 0, 26), f, 'g', -1, 32)
		if bytes.IndexByte(floatConverted, '.') < 0 && bytes.IndexByte(floatConverted, 'e') < 0 {
			floatConverted = append(floatConverted, '.', '0')
		}
		objLiteral = rdfNewTypedLiteral(string(floatConverted), dataTypeURI)
	case &literalTypeInteger.ibNode:
		integer := int64(value.(Integer))
		objLiteral = rdfNewTypedLiteral(strconv.FormatInt(integer, 10), dataTypeURI)
	case &literalTypeBoolean.ibNode:
		objLiteral = rdfNewTypedLiteral(strconv.FormatBool(bool(value.(Boolean))), dataTypeURI)
	case &literalTypeByte.ibNode:
		objLiteral = rdfNewTypedLiteral(strconv.FormatInt(int64(value.(Byte)), 10), dataTypeURI)
	case &literalTypeShort.ibNode:
		objLiteral = rdfNewTypedLiteral(strconv.FormatInt(int64(value.(Short)), 10), dataTypeURI)
	case &literalTypeInt.ibNode:
		objLiteral = rdfNewTypedLiteral(strconv.FormatInt(int64(value.(Int)), 10), dataTypeURI)
	case &literalTypeLong.ibNode:
		objLiteral = rdfNewTypedLiteral(strconv.FormatInt(int64(value.(Long)), 10), dataTypeURI)
	case &literalTypeUnsignedByte.ibNode:
		objLiteral = rdfNewTypedLiteral(strconv.FormatUint(uint64(value.(UnsignedByte)), 10), dataTypeURI)
	case &literalTypeUnsignedShort.ibNode:
		objLiteral = rdfNewTypedLiteral(strconv.FormatUint(uint64(value.(UnsignedShort)), 10), dataTypeURI)
	case &literalTypeUnsignedInt.ibNode:
		objLiteral = rdfNewTypedLiteral(strconv.FormatUint(uint64(value.(UnsignedInt)), 10), dataTypeURI)
	case &literalTypeUnsignedLong.ibNode:
		objLiteral = rdfNewTypedLiteral(strconv.FormatUint(uint64(value.(UnsignedLong)), 10), dataTypeURI)
	case &literalTypeDateTime.ibNode:
		objLiteral = rdfNewTypedLiteral(value.(TypedString).Val, dataTypeURI)
	case &literalTypeDateTimeStamp.ibNode:
		objLiteral = rdfNewTypedLiteral(value.(TypedString).Val, dataTypeURI)
	default:
		err = fmt.Errorf("with literal %v of type %v: %w", l, dataTypeURI, ErrUnsupportedLiteral)
	}
	return objLiteral, err
}

func collectionToRdfTriples(
	forMembers func(e func(o Term)),
	triples []rdfTriple,
	wc *writerContext,
) ([]rdfTriple, error) {
	outL := len(triples)
	out := append(triples, rdfTriple{})
	out[outL].Obj = nilIRI
	var err error
	forMembers(func(l Term) {
		if err != nil {
			return
		}
		rdfList, e := newCollectionBlankNode(wc)
		if e != nil {
			err = e
			return
		}
		out[outL].Obj = rdfList
		outL = len(out)
		out, err = toRdfObject(l, wc, out)
		out[outL].Subj = rdfList
		out[outL].Pred = firstIRI
		outL = len(out)
		out = append(out, rdfTriple{
			Subj: rdfList,
			Pred: restIRI,
			Obj:  nilIRI,
		})
	})
	return out, err
}

func newCollectionBlankNode(wc *writerContext) (rdfTypeBlank, error) {
	b, err := rdfNewBlank(fmt.Sprintf("l%d", wc.collectionCnt))
	wc.collectionCnt++
	return b, err
}

func toIRIOrBlankNode(s IBNode, wc *writerContext) (rdfTypeSubject, rdfTypeObject, error) {
	if s.IsBlankNode() {
		blank, found := wc.blankNodes[s]
		if found {
			return blank, blank, nil
		}
		blank, err := rdfNewBlank(fmt.Sprintf("b%d", len(wc.blankNodes)))
		if err != nil {
			return nil, nil, err
		}
		wc.blankNodes[s] = blank
		return blank, blank, nil
	}
	iri := s.IRI()
	if wc.writer.prefixify(s.OwningGraph(), iri) != iri.rdfSerialize(wc.writer.format) {
		prefix, suffix := iri.Split()
		suffix = pnLocalEsc.ReplaceAllStringFunc(suffix, func(s string) string { return s })

		iriString := prefix + "#" + suffix
		iri = *(*IRI)(unsafe.Pointer(&iriString))
	}
	return iri, iri, nil
}

func canInlineAsCollection(s IBNode) (bool, error) {
	if _, ok := s.AsCollection(); ok {
		var inlineCnt int
		err := s.ForAll(func(_ int, ts, tp IBNode, to Term) error {
			if s == ts {
				if !tp.Is(rdfFirst) {
					inlineCnt += 2
					return errBreakFor
				}
			} else {
				inlineCnt++
				if inlineCnt > 1 {
					return errBreakFor
				}
			}
			return nil
		})
		if err != nil && err != errBreakFor { // nolint:errorlint
			return false, err
		}
		return inlineCnt == 1, nil
	}
	return false, nil
}

func canInlineAsBlankNode(s IBNode) (bool, error) {
	if s.IsBlankNode() {
		var useCnt int
		err := s.ForAll(func(index int, ts, tp IBNode, to Term) error {
			if s != ts {
				useCnt++
				if useCnt > 1 {
					return errBreakFor
				}
			}
			return nil
		})
		if err != nil && err != errBreakFor { // nolint:errorlint
			return false, err
		}
		return useCnt == 1, nil
	}
	return false, nil
}

func serializeTerm(mTerm rdfTerm, twr *tripleWriter) string {
	if twr != nil {
		return twr.prefixify(nil, mTerm)
	}
	return mTerm.rdfSerialize(RdfFormatTurtle)
}

// toWriterTriG writes all NamedGraphs in TriG format.
// TriG is an extension of Turtle that supports named graphs using graph blocks.
func toWriterTriG(s *stage, graphs []NamedGraph, writer *tripleWriter) (err error) {
	defer func() {
		cErr := writer.close()
		if err == nil {
			err = cErr
		}
	}()

	// Set up common namespaces
	writer.Namespaces[owlVocabulary.BaseIRI] = "owl"
	writer.Namespaces[rdfVocabulary.BaseIRI] = "rdf"
	writer.Namespaces[rdfsVocabulary.BaseIRI] = "rdfs"
	writer.Namespaces[lciVocabulary.BaseIRI] = "lci"
	writer.Namespaces[ssoVocabulary.BaseIRI] = "sso"
	writer.Namespaces[xsdVocabulary.BaseIRI] = "xsd"

	for ns, pfx := range namespaceToPrefixMap {
		writer.Namespaces[ns] = pfx
	}

	// Collect all namespaces from all graphs
	var namespaces []string
	var namespacesMap = make(map[string]struct{})
	namespaces = append(namespaces, owlVocabulary.BaseIRI)
	namespacesMap[owlVocabulary.BaseIRI] = struct{}{}
	namespaces = append(namespaces, rdfVocabulary.BaseIRI)
	namespacesMap[rdfVocabulary.BaseIRI] = struct{}{}
	namespaces = append(namespaces, xsdVocabulary.BaseIRI)
	namespacesMap[xsdVocabulary.BaseIRI] = struct{}{}

	for _, ng := range s.ReferencedGraphs() {
		if _, ok := namespacesMap[string(ng.IRI())]; !ok {
			if _, ok := writer.Namespaces[string(ng.IRI())]; ok {
				namespaces = append(namespaces, string(ng.IRI()))
				namespacesMap[string(ng.IRI())] = struct{}{}
			}
		}
	}

	// Write prefixes at the top of the file
	sort.Strings(namespaces)
	for _, base := range namespaces {
		prefix := writer.Namespaces[base]
		if writer.OpenStatement {
			writer.w.write([]byte(" .\n"))
		}
		writer.w.write([]byte(fmt.Sprintf("@prefix %s: <%s#> .\n", prefix, base)))
	}

	// Share blank-node label scope across all graph blocks.
	// Per RDF 1.1 TriG, a blank node label represents the same blank node
	// throughout the document, so labels must be unique globally.
	wc := &writerContext{
		writer:               writer,
		blankNodes:           map[IBNode]rdfTypeBlank{},
		allowCollectionTerms: true,
	}

	// Process each named graph
	for _, graph := range graphs {
		if err := writeNamedGraphTriG(graph, writer, wc); err != nil {
			return err
		}
	}

	return nil
}

// writeNamedGraphTriG writes a single NamedGraph in TriG format with graph block.
func writeNamedGraphTriG(graph NamedGraph, writer *tripleWriter, wc *writerContext) error {
	// Close any previous statement before starting a new graph block
	if writer.OpenStatement {
		writer.w.write([]byte(" .\n"))
		writer.OpenStatement = false
	}

	// Write the graph name (IRI) followed by opening brace
	graphIRI := graph.IRI()
	writer.w.write([]byte("\n"))

	// Use the graph IRI as the named graph identifier
	// For the default prefix, we use the graph IRI directly
	graphPrefix := writer.prefixify(graph, graphIRI)
	if graphPrefix == graphIRI.rdfSerialize(RdfFormatTriG) {
		// If no prefix was created, use the full IRI in angle brackets
		writer.w.write([]byte(fmt.Sprintf("<%s> {\n", graphIRI.String())))
	} else {
		writer.w.write([]byte(fmt.Sprintf("%s {\n", graphPrefix)))
	}

	// Write the graph content using a modified version of toWriter
	if err := writeGraphContentTriG(graph, writer, wc); err != nil {
		return err
	}

	// Close the graph block
	if writer.OpenStatement {
		writer.w.write([]byte(" .\n"))
		writer.OpenStatement = false
	}
	writer.w.write([]byte("}\n"))

	return nil
}

// writeGraphContentTriG writes the triples of a NamedGraph within a TriG graph block.
func writeGraphContentTriG(graph NamedGraph, writer *tripleWriter, wc *writerContext) error {
	sortedNodes, err := getSortedTriplesIBNodes(graph.(*namedGraph))
	if err != nil {
		return err
	}

	graphDirectImports := graph.DirectImports()

	graphImportURLs := make([]IRI, 0, len(graphDirectImports))
	for _, ng := range graphDirectImports {
		graphImportURLs = append(graphImportURLs, ng.IRI())
	}
	sort.Slice(graphImportURLs, func(i, j int) bool {
		return strings.Compare(graphImportURLs[i].String(), graphImportURLs[j].String()) < 0
	})

	// Handle prefixes for import IRIs
	for _, u := range graphImportURLs {
		importIRI := u
		p, s := importIRI.Split()
		if s != "" {
			pIRI, err := NewIRI(p)
			if err != nil {
				return err
			}
			writer.prefixify(graph, pIRI)
		}
	}

	var triples []rdfTriple

	// Handle prefixes for all terms first
	for _, s := range sortedNodes {
		err := s.ForAll(func(index int, ts, tp IBNode, to Term) error {
			if s == ts {
				triples, err := nodeTripleToRdfTriples(index, ts, tp, to, wc, triples[:0])
				if err != nil {
					return err
				}
				for _, triple := range triples {
					writer.prefixify(graph, triple.Subj)
					writer.prefixify(tp.OwningGraph(), triple.Pred)
					if to.TermKind() == TermKindIBNode || to.TermKind() == TermKindTermCollection {
						writer.prefixify(to.(IBNode).OwningGraph(), triple.Obj)
					}
				}
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	writeToTripleWriter(writer, ([]byte)("\n"))

	// Handle NG Node (the node with empty fragment representing the graph itself)
	for pos, s := range sortedNodes {
		if s.IsIRINode() && s.Fragment() == "" {
			self, _ := graph.IRI().Split()
			selfIRI, err := NewIRI(self)
			if err != nil {
				return err
			}
			rdfTypeIRI := rdfType.IRI()
			owlOntologyIRI := owlOntology.IRI()
			err = writer.write(graph, rdfTriple{Subj: selfIRI, Pred: rdfTypeIRI, Obj: owlOntologyIRI})
			if err != nil {
				return err
			}
			err = s.ForAll(func(index int, ts, tp IBNode, to Term) error {
				if s == ts {
					if tp.Is(rdfType) && to.TermKind() == TermKindIBNode && to.(IBNode).Is(owlOntology) {
						// Skip, already written above
					} else {
						triples, err := nodeTripleToRdfTriples(index, ts, tp, to, wc, triples[:0])
						if err != nil {
							return err
						}
						for _, triple := range triples {
							triple.Subj = selfIRI
							err := writer.write(graph, triple)
							if err != nil {
								return err
							}
						}
						return nil
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			for _, u := range graphImportURLs {
				importIRIStr := strings.TrimSuffix(u.String(), "#")
				importIRI, err := NewIRI(importIRIStr)
				if err != nil {
					return err
				}
				err = writer.write(graph, rdfTriple{Subj: selfIRI, Pred: owlImports.IRI(), Obj: importIRI})
				if err != nil {
					return err
				}
			}

			sortedNodes = removeElementGeneric(sortedNodes, pos)
			break
		}
	}

	// Handle other Nodes
	for _, s := range sortedNodes {
		inlineCol, err := canInlineAsCollection(s)
		if err != nil {
			return err
		}
		if inlineCol && wc.allowCollectionTerms {
			continue
		}
		inlineCol, err = canInlineAsBlankNode(s)
		if err != nil {
			return err
		}
		if inlineCol && wc.allowCollectionTerms {
			continue
		}
		err = s.ForAll(func(index int, ts, tp IBNode, to Term) error {
			if s == ts {
				triples, err := nodeTripleToRdfTriples(index, ts, tp, to, wc, triples[:0])
				if err != nil {
					return err
				}
				for _, triple := range triples {
					err := writer.write(graph, triple)
					if err != nil {
						return err
					}
				}
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}
