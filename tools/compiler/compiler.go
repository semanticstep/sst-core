// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

//go:generate go run compiler.go

// The SST Compiler converts higher level ontologies provided in Turtle format into Go
// statements for early binding SST functionality.
//
// Ontology source:
//
//	The compiler reads .ttl files from the standalone module
//	    git.semanticstep.net/x/sst-ontologies
//	which exposes all ontologies via an embed.FS (ontologies.FS).
//
//	LoadDictOntologies works with any fs.FS, so it can also be pointed at a local
//	directory (e.g. os.DirFS(".")) for development or testing.
//
// Updating ontologies:
//  1. Publish changes to github.com/semanticstep/sst-ontologies.
//  2. In this module run:
//     go get github.com/semanticstep/sst-ontologies@latest
//     go mod tidy
//  3. Re-run the compiler:
//     go run ./tools/compiler
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode"

	wrfs "github.com/relab/wrfs"
	"github.com/semanticstep/sst-core/sst"
	ontologies "github.com/semanticstep/sst-ontologies"
)

// Copy of used vocabulary elements.
var (
	rdfVocabulary                = sst.Vocabulary{BaseIRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns"}
	rdfProperty                  = sst.Element{Vocabulary: rdfVocabulary, Name: "Property"}
	rdfType                      = sst.Element{Vocabulary: rdfVocabulary, Name: "type"}
	rdfsVocabulary               = sst.Vocabulary{BaseIRI: "http://www.w3.org/2000/01/rdf-schema"}
	rdfsClass                    = sst.Element{Vocabulary: rdfsVocabulary, Name: "Class"}
	rdfsDatatype                 = sst.Element{Vocabulary: rdfsVocabulary, Name: "Datatype"}
	rdfsDomain                   = sst.Element{Vocabulary: rdfsVocabulary, Name: "domain"}
	rdfsRange                    = sst.Element{Vocabulary: rdfsVocabulary, Name: "range"}
	rdfsSubClassOf               = sst.Element{Vocabulary: rdfsVocabulary, Name: "subClassOf"}
	rdfsSubPropertyOf            = sst.Element{Vocabulary: rdfsVocabulary, Name: "subPropertyOf"}
	owlVocabulary                = sst.Vocabulary{BaseIRI: "http://www.w3.org/2002/07/owl"}
	owlClass                     = sst.Element{Vocabulary: owlVocabulary, Name: "Class"}
	owlAnnotationProperty        = sst.Element{Vocabulary: owlVocabulary, Name: "AnnotationProperty"}
	owlAsymmetricProperty        = sst.Element{Vocabulary: owlVocabulary, Name: "AsymmetricProperty"}
	owlDatatypeProperty          = sst.Element{Vocabulary: owlVocabulary, Name: "DatatypeProperty"}
	owlFunctionalProperty        = sst.Element{Vocabulary: owlVocabulary, Name: "FunctionalProperty"}
	owlInverseFunctionalProperty = sst.Element{Vocabulary: owlVocabulary, Name: "InverseFunctionalProperty"}
	owlIrreflexiveProperty       = sst.Element{Vocabulary: owlVocabulary, Name: "IrreflexiveProperty"}
	owlOntologyProperty          = sst.Element{Vocabulary: owlVocabulary, Name: "OntologyProperty"}
	owlReflexiveProperty         = sst.Element{Vocabulary: owlVocabulary, Name: "ReflexiveProperty"}
	owlSymmetricProperty         = sst.Element{Vocabulary: owlVocabulary, Name: "SymmetricProperty"}
	owlTransitiveProperty        = sst.Element{Vocabulary: owlVocabulary, Name: "TransitiveProperty"}
	owlInverseOf                 = sst.Element{Vocabulary: owlVocabulary, Name: "inverseOf"} // InverseObjectProperties
	owlObjectProperty            = sst.Element{Vocabulary: owlVocabulary, Name: "ObjectProperty"}
	owlOnDatatype                = sst.Element{Vocabulary: owlVocabulary, Name: "onDatatype"}
	ssmetaVocabulary             = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/ssmeta"}
	ssmetaMainClass              = sst.Element{Vocabulary: ssmetaVocabulary, Name: "MainClass"}
	ssmetaOptionClass            = sst.Element{Vocabulary: ssmetaVocabulary, Name: "OptionClass"}
	ssmetaAbstractClass          = sst.Element{Vocabulary: ssmetaVocabulary, Name: "AbstractClass"}
	ssmetaRootProperty           = sst.Element{Vocabulary: ssmetaVocabulary, Name: "RootProperty"}
	ssmetaRootClass              = sst.Element{Vocabulary: ssmetaVocabulary, Name: "RootClass"}
	ssmetaGoAlias                = sst.Element{Vocabulary: ssmetaVocabulary, Name: "goAlias"}
	ssmetaOfType                 = sst.Element{Vocabulary: ssmetaVocabulary, Name: "ofType"}
)

var (
	VocabLci           = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/lci"}
	VocabOwl           = sst.Vocabulary{BaseIRI: "http://www.w3.org/2002/07/owl"}
	VocabRdfs          = sst.Vocabulary{BaseIRI: "http://www.w3.org/2000/01/rdf-schema"}
	VocabRdf           = sst.Vocabulary{BaseIRI: "http://www.w3.org/1999/02/22-rdf-syntax-ns"}
	VocabSh            = sst.Vocabulary{BaseIRI: "http://www.w3.org/ns/shacl"}
	VocabSso           = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/sso"}
	VocabSsmeta        = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/ssmeta"}
	VocabSsrep         = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/rep"}
	VocabSsqau         = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/qau"}
	VocabColor         = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/color"}
	VocabCountrycodes  = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/countrycodes"}
	VocabCurrencycodes = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/currencycodes"}
	VocabXsd           = sst.Vocabulary{BaseIRI: "http://www.w3.org/2001/XMLSchema"}
	VocabSkos          = sst.Vocabulary{BaseIRI: "http://www.w3.org/2004/02/skos/core"}
	VocabEed           = sst.Vocabulary{BaseIRI: "http://ontology.semanticstep.net/eed"}

	urlPackageMap = map[string]struct {
		pkg   string
		style prefixStyle
	}{
		VocabLci.BaseIRI:           {"lci", prefixFromComplete},
		VocabOwl.BaseIRI:           {"owl", prefixFromComplete},
		VocabRdfs.BaseIRI:          {"rdfs", prefixFromComplete},
		VocabRdf.BaseIRI:           {"rdf", prefixFromComplete},
		VocabSh.BaseIRI:            {"sh", prefixFromComplete},
		VocabSso.BaseIRI:           {"sso", prefixFromComplete},
		VocabSsrep.BaseIRI:         {"rep", prefixFromComplete},
		VocabSsmeta.BaseIRI:        {"ssmeta", prefixFromComplete},
		VocabSsqau.BaseIRI:         {"qau", prefixFromComplete},
		VocabColor.BaseIRI:         {"color", prefixFromComplete},
		VocabCountrycodes.BaseIRI:  {"countrycodes", prefixFromComplete},
		VocabCurrencycodes.BaseIRI: {"currencycodes", prefixFromComplete},
		VocabXsd.BaseIRI:           {"xsd", prefixFromComplete},
		VocabSkos.BaseIRI:          {"skos", prefixFromComplete},
		VocabEed.BaseIRI:           {"eed", prefixFromComplete},
	}
	urlImportsMap = map[string]map[string]struct{}{
		VocabRdfs.BaseIRI:          {"rdf": {}},
		VocabOwl.BaseIRI:           {"rdf": {}, "rdfs": {}, "xsd": {}},
		VocabLci.BaseIRI:           {"rdf": {}, "rdfs": {}, "xsd": {}, "owl": {}},
		VocabSsrep.BaseIRI:         {"rdf": {}, "rdfs": {}, "xsd": {}, "lci": {}, "ssmeta": {}},
		VocabSsqau.BaseIRI:         {"xsd": {}, "lci": {}},
		VocabColor.BaseIRI:         {"xsd": {}, "lci": {}},
		VocabCountrycodes.BaseIRI:  {"xsd": {}, "lci": {}},
		VocabCurrencycodes.BaseIRI: {"xsd": {}, "lci": {}},
		VocabSso.BaseIRI:           {"rdf": {}, "xsd": {}, "ssmeta": {}, "lci": {}, "rep": {}},
		VocabSkos.BaseIRI:          {"rdf": {}, "rdfs": {}},
	}

	neededCompileTTL = map[string]string{
		VocabXsd.BaseIRI:    "xsd",
		VocabRdf.BaseIRI:    "rdf",
		VocabRdfs.BaseIRI:   "rdfs",
		VocabOwl.BaseIRI:    "owl",
		VocabSsmeta.BaseIRI: "ssmeta",
		VocabLci.BaseIRI:    "lci",
		VocabSso.BaseIRI:    "sso",
		VocabSsrep.BaseIRI:  "rep",
		VocabSkos.BaseIRI:   "skos",
		VocabEed.BaseIRI:    "eed",
		VocabSh.BaseIRI:     "sh",
	}

	neededCompileDictTTL = map[string]string{
		VocabSsqau.BaseIRI:         "qau",
		VocabColor.BaseIRI:         "color",
		VocabCountrycodes.BaseIRI:  "countrycodes",
		VocabCurrencycodes.BaseIRI: "currencycodes",
	}
)

func lowerCaseGoName(s string) string {
	goName := strings.Builder{}
	toLower, toUpper := true, false
	for _, c := range s {
		switch c {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z':
			toLower = false
			fallthrough
		case 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
			if toLower {
				c = unicode.ToLower(c)
			} else if toUpper {
				c = unicode.ToUpper(c)
			}
			toUpper = false
			goName.WriteRune(c)
		case '-', '_':
			toLower, toUpper = false, true
			goName.WriteRune('_')
		}
	}
	return goName.String()
}

func nodeToInfoTypeName(n sst.IBNode) string {
	return lowerCaseGoName(string(n.Fragment())) + "I"
}

func maybeReplaceKnownPrefix(prefix string) string {
	switch prefix {
	case "22-rdf-syntax-ns":
		prefix = "rdf"
	case "rdf-schema":
		prefix = "rdfs"
	case "XMLSchema":
		prefix = "xsd"
	}
	return prefix
}

func NameOfVocabularyElement(ib sst.IBNode, vocab sst.Vocabulary) string {
	if ib.OwningGraph().IRI().String() != vocab.BaseIRI {
		if _, found := urlImportsMap[vocab.BaseIRI]; !found {
			return ""
		}
	}

	if ib.IsBlankNode() {
		return ""
	}

	s1 := urlPackageMap[ib.OwningGraph().IRI().String()].pkg
	s2 := sst.Element{Name: string(ib.Fragment())}.GoSimpleName()
	var s3 string
	if s1 == urlPackageMap[vocab.BaseIRI].pkg {
		s3 = s2
	} else {
		s3 = s1 + "." + s2
	}
	return s3
}

func prettyID(d sst.IBNode, vocab sst.Vocabulary) string {
	s1 := urlPackageMap[d.OwningGraph().IRI().String()]
	// if s1.pkg == "" || d.IsIRINode() && d.Fragment() == "" {
	// 	return d.IRI().String()
	// }
	// if s1.pkg == urlPackageMap[vocab.BaseIRI].pkg {
	// 	return ":" + string(d.Fragment())
	// }
	if d.IsIRINode() {
		return s1.pkg + ":" + string(d.Fragment())
	} else {
		return s1.pkg + ":" + d.ID().String()
	}
}

func searchInheritanceInterfaces(ib sst.IBNode, receiver string, inheritanceFunctions map[string]struct{}) {
	ib.ForAll(func(_ int, ibS, ibP sst.IBNode, o sst.Term) error {
		if ibS == ib { // not inverse
			if o.TermKind() != sst.TermKindIBNode && o.TermKind() != sst.TermKindTermCollection {
				return nil
			}
			if ibP.Is(rdfsSubClassOf) ||
				ibP.Is(rdfsSubPropertyOf) ||
				ibP.Is(owlOnDatatype) && (o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection) {
				o := o.(sst.IBNode)
				inheritanceFunctions[fmt.Sprintf("func (%s) %s()", receiver, kindMethodOf(sst.Element{
					Vocabulary: sst.Vocabulary{BaseIRI: o.OwningGraph().IRI().String()},
					Name:       o.Fragment(),
				}))] = struct{}{}
				searchInheritanceInterfaces(o, receiver, inheritanceFunctions)
			}
		}
		return nil
	})
}

type vocabProperties struct {
	isClass              bool
	isOptionClass        bool
	isAbstractClass      bool
	isRootClass          bool
	isProperty           bool
	isDatatypeProp       bool
	isObjectProperty     bool
	isDatatype           bool
	isIndividual         bool
	subtypeOf            []sst.IBNode
	collectionMemberType sst.IBNode
	domain               sst.IBNode
	rang                 sst.IBNode
	subPropertyOf        sst.IBNode
	subInversePropertyOf sst.IBNode
	inverseOf            sst.IBNode
	hasInverse           sst.IBNode
	mainClassSupersedure map[sst.IBNode]struct{}
}

func collectMainClassSupersedure(ib sst.IBNode, vp vocabProperties) {
	ib.ForAll(func(_ int, ibS, ibP sst.IBNode, o sst.Term) error {
		if ibS == ib { // not inverse
			if !sst.IsKindIBNode(o) {
				return nil
			}
			o := o.(sst.IBNode)
			if ibP.Is(rdfsSubClassOf) {
				collectMainClassSupersedure(o, vp)
			} else if ibP.Is(rdfType) && o.Is(ssmetaMainClass) {
				vp.mainClassSupersedure[ib] = struct{}{}
			}
		}
		return nil
	})
}

// collectPropertyInheritance() analysis the Class and Property definitions of an IBNode
// and returns the findings in a vocabProperties structure
func collectPropertyInheritance(ib sst.IBNode) vocabProperties {
	var vp vocabProperties

	ib.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s == ib { // forward triple
			if o.TermKind() != sst.TermKindIBNode {
				return nil
			}
			o := o.(sst.IBNode)
			if p.Is(rdfType) {
				if o.Is(rdfProperty) || o.Is(owlAnnotationProperty) || o.Is(owlAsymmetricProperty) ||
					o.Is(owlFunctionalProperty) || o.Is(owlInverseFunctionalProperty) ||
					o.Is(owlIrreflexiveProperty) || o.Is(owlOntologyProperty) || o.Is(owlReflexiveProperty) ||
					o.Is(owlSymmetricProperty) || o.Is(owlTransitiveProperty) {
					vp.isProperty = true
				} else if o.Is(owlDatatypeProperty) {
					vp.isDatatypeProp = true
					vp.isProperty = true
				} else if o.Is(owlObjectProperty) {
					vp.isObjectProperty = true
					vp.isProperty = true
				} else if o.Is(ssmetaRootProperty) {
					vp.isProperty = true
				} else if o.Is(rdfsDatatype) {
					vp.isDatatype = true
				} else if o.Is(rdfsClass) {
					vp.isClass = true
				} else if o.Is(owlClass) {
					vp.isClass = true
				} else if o.Is(ssmetaMainClass) {
					vp.mainClassSupersedure = map[sst.IBNode]struct{}{}
					for _, s := range vp.subtypeOf {
						collectMainClassSupersedure(s, vp)
					}
				} else if o.Is(ssmetaOptionClass) {
					vp.isOptionClass = true
					vp.isClass = true
				} else if o.Is(ssmetaRootClass) {
					vp.isRootClass = true
					vp.isClass = true
				} else if o.Is(ssmetaAbstractClass) {
					vp.isAbstractClass = true
					vp.isClass = true
				}
			}
			if p.Is(rdfsSubClassOf) {
				vp.isClass = true
				vp.subtypeOf = append(vp.subtypeOf, o)
				if vp.mainClassSupersedure != nil {
					collectMainClassSupersedure(o, vp)
				}
			}
			if p.Is(rdfsSubPropertyOf) {
				vp.isProperty = true
				vp.subPropertyOf = o
			}

			if p.Is(rdfsDomain) {
				vp.domain = o
			}

			if p.Is(rdfsRange) {
				vp.rang = o
			}
			if p.Is(owlInverseOf) {
				vp.inverseOf = o
				vp.hasInverse = o
				vp.isProperty = true
			}
			if p.Is(ssmetaOfType) {
				vp.collectionMemberType = o
			}

		} else { // inverse triple
			if p.Is(owlInverseOf) {
				vp.hasInverse = s
			}
		}
		return nil
	})
	if !vp.isClass && !vp.isProperty && !vp.isDatatype {
		vp.isIndividual = true
	}

	return vp
}

func checkRangeDomain(ib sst.IBNode, vocab sst.Vocabulary) vocabProperties {
	vp := collectPropertyInheritance(ib)
	return vp
}

func joinSortedNodes(ibs []sst.IBNode, suffix string, vocab sst.Vocabulary) string {
	var out strings.Builder
	sort.Slice(ibs, func(i, j int) bool {
		return ibs[i].IRI().String() < ibs[j].IRI().String()
	})
	for _, s := range ibs {
		d := NameOfVocabularyElement(s, vocab)
		if d != "" {
			if out.Len() > 0 {
				fprint(&out, ", ")
			}
			fprint(&out, d)
			fprint(&out, suffix)
		}
	}
	return out.String()
}

type prefixStyle int

const (
	prefixFromSuffix prefixStyle = iota
	prefixFromPrefix
	prefixFromComplete
)

func (p prefixStyle) rangeString() string {
	return ([]string{"[2:]", "[:3]", ""})[p]
}

func (p prefixStyle) prefix(pkg string) string {
	return ([]func() string{
		func() string { return pkg[2:] },
		func() string { return pkg[:3] },
		func() string { return pkg },
	})[p]()
}

type namespaceToPrefix struct {
	pkg, vocabulary string
	style           prefixStyle
}

type vocabData struct {
	imports           []string
	namespaceToPrefix []namespaceToPrefix
}

type vocabMapPkg struct {
	pkg     string
	imprt   string
	entries []string
}

type sstToGo struct {
	ibs sst.IBNode
	vp  vocabProperties
}

func compileSSTtoGO(graph sst.NamedGraph, output string, vocab sst.Vocabulary, vocabMaps *[]vocabMapPkg, data *vocabData) {
	pkg := urlPackageMap[vocab.BaseIRI]

	if graph == nil {
		fmt.Printf("skipping: %s\n", pkg.pkg)
		return
	}

	err := graph.Stage().ForUndefinedIBNodes(func(d sst.IBNode) error {
		if d.Fragment() != "" { // skip default IBNodes representing the whole NG
			fmt.Printf("WARNING: Unknown Node %s\n", prettyID(d, vocab))
		}
		err := d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			visited := map[sst.IBNode]struct{}{}
			var forInternNode func(s sst.IBNode, level int) error
			forInternNode = func(s sst.IBNode, level int) error {
				if _, found := visited[s]; found {
					return nil
				}
				visited[s] = struct{}{}
				if s.IsUuidFragment() {
					d := s
					return d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
						if s == d {
							return nil
						}
						fmt.Printf("%*s  referenced by predicate %s\n", level, " ", prettyID(p, vocab))
						fmt.Printf("%*s  referenced by subject %s\n", level, " ", prettyID(s, vocab))
						return forInternNode(s, level+2)
					})
				}
				return nil
			}
			if o == d {
				fmt.Printf("  referenced by predicate %s\n", prettyID(p, vocab))
				err := forInternNode(p, 2)
				if err != nil {
					return err
				}
				fmt.Printf("  referenced by subject %s\n", prettyID(s, vocab))
				err = forInternNode(s, 2)
				if err != nil {
					return err
				}
			}
			if p == d {
				fmt.Printf("  referenced by subject %s\n", prettyID(s, vocab))
				err := forInternNode(s, 2)
				if err != nil {
					return err
				}
				if o, ok := o.(sst.IBNode); ok {
					fmt.Printf("  referenced by object %s\n", prettyID(o, vocab))
					err := forInternNode(o, 2)
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	out, err := os.Create(output)
	if err != nil {
		log.Panic(err)
	}
	defer func() {
		_ = out.Close()
	}()
	writeHeader(out)
	fprintf(out, "package %s\n", pkg.pkg)
	fprintf(out, "\n")
	fprint(out, "import (\n\t\"reflect\"\n")
	fprint(out, "\t\"github.com/semanticstep/sst-core/sst\"\n")
	imports := make([]string, 0, len(urlImportsMap[vocab.BaseIRI]))
	for i := range urlImportsMap[vocab.BaseIRI] {
		imports = append(imports, i)
	}

	sort.Strings(imports)

	for _, i := range imports {
		fprintf(out, "\t\"github.com/semanticstep/sst-core/vocabularies/%s\"\n", i)
	}
	fprint(out, ")\n\ntype pkg struct{ sst.ElementInfo }\n\n")

	prefix := maybeReplaceKnownPrefix(strings.TrimSuffix(path.Base(vocab.BaseIRI), "#"))
	vocabularyPKG := strings.ToUpper(prefix)
	fprintf(out, "var %sVocabulary = sst.Vocabulary{BaseIRI: \"%s\"}\n\n", vocabularyPKG, vocab.BaseIRI)
	fprint(out, "var (\n\tPKG = sst.ElementPkg(pkg{})\n\t_   = PKG\n)\n")

	ibFragments := make([]string, 0, graph.IRINodeCount())

	graph.ForIRINodes(func(d sst.IBNode) error {
		ibFragments = append(ibFragments, d.Fragment())
		return nil
	})

	sort.Slice(ibFragments, func(i, j int) bool {
		return strings.Compare(string(ibFragments[i]), string(ibFragments[j])) < 0
	})

	vocabMap := vocabMapPkg{
		pkg:     pkg.pkg,
		entries: make([]string, 0, len(ibFragments)),
	}

	var sstToGos []sstToGo

	// collect all information
	for _, f := range ibFragments {
		ibS := graph.GetIRINodeByFragment(f)
		vp := checkRangeDomain(ibS, vocab)
		sstToGos = append(sstToGos, sstToGo{ibS, vp})
	}

	var alreadyPrintedInverseOfNode []sst.IBNode

	// print out all information
	for _, element := range sstToGos {
		ibS := element.ibs
		vp := element.vp
		var s0 string

		// check if this node has a goAlias
		goAlias := ibS.GetObjects(ssmetaGoAlias)
		if len(goAlias) > 0 {
			s0 = string(goAlias[0].(sst.String))
		} else {
			s0 = string(ibS.Fragment())
		}

		if s0 == "." || s0 == "" {
			continue
		} // Why this happens for owl:versionInfo "0.01" ??

		// elemVarName
		elemVarName := sst.Element{Name: s0}.GoSimpleName()

		// s4
		s4 := lowerCaseGoName(s0) + "I"
		s5 := s4[0 : len(s4)-1]
		fprintf(out, "\n// Definitions of %s.%s\n\n", prefix, s0)
		fprintf(
			out,
			"type Is%s interface {\n\tsst.ElementInformer\n\t%s()\n}\n\n",
			elemVarName,
			typeMethodOf(sst.Element{
				Vocabulary: sst.Vocabulary{BaseIRI: ibS.OwningGraph().IRI().String()},
				Name:       ibS.Fragment(),
			}),
		)
		fprintf(
			out,
			"type Kind%s interface {\n\tsst.ElementInformer\n\t%s()\n}\n\n",
			elemVarName,
			kindMethodOf(sst.Element{
				Vocabulary: sst.Vocabulary{BaseIRI: ibS.OwningGraph().IRI().String()},
				Name:       ibS.Fragment(),
			}),
		)
		fprintf(out, "type %s struct{ sst.ElementInfo }\n\n", s4)
		inheritanceFunctions := map[string]struct{}{
			fmt.Sprintf("func (%s) %s()", s4, typeMethodOf(sst.Element{
				Vocabulary: sst.Vocabulary{BaseIRI: ibS.OwningGraph().IRI().String()},
				Name:       ibS.Fragment(),
			})): {},
			fmt.Sprintf("func (%s) %s()", s4, kindMethodOf(sst.Element{
				Vocabulary: sst.Vocabulary{BaseIRI: ibS.OwningGraph().IRI().String()},
				Name:       ibS.Fragment(),
			})): {},
		}
		inheritanceFunctions[fmt.Sprintf("func (%s) %s()", s4, kindMethodOf(sst.Element{
			Vocabulary: rdfsVocabulary, Name: "Resource",
		}))] = struct{}{}
		if vp.isIndividual {
			inheritanceFunctions[fmt.Sprintf("func (%s) %s()", s4, kindMethodOf(sst.Element{
				Vocabulary: owlVocabulary, Name: "Thing",
			}))] = struct{}{}
		}
		searchInheritanceInterfaces(ibS, s4, inheritanceFunctions)
		sortedInheritanceFunctions := make([]string, 0, len(inheritanceFunctions))
		for k := range inheritanceFunctions {
			sortedInheritanceFunctions = append(sortedInheritanceFunctions, k)
		}
		sort.Strings(sortedInheritanceFunctions)
		sortedInheritanceMaxLen := 0
		for _, k := range sortedInheritanceFunctions {
			if sortedInheritanceMaxLen < len(k) {
				sortedInheritanceMaxLen = len(k)
			}
		}
		for _, k := range sortedInheritanceFunctions {
			fprintf(out, "%-*s {}\n", sortedInheritanceMaxLen, k)
		}
		fprintf(out, "\nvar %sKindInterface reflect.Type\n\n", s5)
		fprintf(out, "var %s = %s{sst.ElementInfo{\n", elemVarName, s4)

		vocabMap.entries = append(vocabMap.entries, elemVarName)
		fprintf(out, "\tElement: sst.Element{Vocabulary: %sVocabulary, Name: \"%s\"},\n", vocabularyPKG, ibS.Fragment())
		if vp.domain != nil {
			if d := NameOfVocabularyElement(vp.domain, vocab); d != "" {
				fprintf(out, "\tADomain:           %s,\n", d)
			}
		}
		if vp.rang != nil {
			if r := NameOfVocabularyElement(vp.rang, vocab); r != "" {
				fprintf(out, "\tARange:            %s,\n", r)
			}
		}
		if vp.collectionMemberType != nil {
			if r := NameOfVocabularyElement(vp.collectionMemberType, vocab); r != "" {
				fprintf(out, "\tACollectionMemberType:            %s,\n", r)
			}
		}

		if len(vp.subtypeOf) > 0 {
			subtypes := joinSortedNodes(vp.subtypeOf, "", vocab)
			if subtypes != "" {
				fprintf(out, "\tASubtypeOf:        []sst.ElementInformer{%s},\n", subtypes)
			}
		}
		if vp.subPropertyOf != nil {
			if s := NameOfVocabularyElement(vp.subPropertyOf, vocab); s != "" {
				fprintf(out, "\tASubPropertyOf:    %s,\n", s)
			}
		}
		if vp.inverseOf != nil && !slices.Contains(alreadyPrintedInverseOfNode, ibS) {
			if i := NameOfVocabularyElement(vp.inverseOf, vocab); i != "" {
				alreadyPrintedInverseOfNode = append(alreadyPrintedInverseOfNode, vp.inverseOf)
				fprintf(out, "\tAnInverseOf:       %s,\n", i)
			}
		}
		fprintf(out, "\n\tAClass: %t, AnOptionClass: %t, AnAbstractClass: %t, ARootClass: %t,\n",
			vp.isClass, vp.isOptionClass, vp.isAbstractClass, vp.isRootClass)
		fprintf(out, "\tAProperty: %t, ADatatypeProperty: %t, AnObjectProperty: %t, \n",
			vp.isProperty, vp.isDatatypeProp, vp.isObjectProperty)
		fprintf(out, "\tADatatype: %t, AnIndividual: %t,\n", vp.isDatatype, vp.isIndividual)
		if vp.mainClassSupersedure != nil {
			fprint(out, "\tAMainClassSupersedure: map[sst.Element]struct{}{")
			var supersedure []sst.IBNode
			for m := range vp.mainClassSupersedure {
				supersedure = append(supersedure, m)
			}
			fprint(out, joinSortedNodes(supersedure, ".Element: {}", vocab))
			fprint(out, "},\n")
		}
		fprintf(out, "}}\n")
		if vp.inverseOf != nil && vp.inverseOf.OwningGraph() == ibS.OwningGraph() {
			fprintf(out, "\nfunc (%s) InverseOf() sst.ElementInformer { return %s }\n", nodeToInfoTypeName(vp.inverseOf), elemVarName)
		}
	}
	vocabMap.imprt = path.Dir(output)
	*vocabMaps = append(*vocabMaps, vocabMap)
	data.imports = append(data.imports, vocabMap.imprt)
	data.namespaceToPrefix = append(data.namespaceToPrefix, namespaceToPrefix{
		pkg:        pkg.pkg,
		vocabulary: vocabularyPKG,
		style:      pkg.style,
	})
}

func dictSSTtoGO(graph sst.NamedGraph, output string, vocab sst.Vocabulary, data *vocabData) {
	pkg := urlPackageMap[vocab.BaseIRI]
	if graph == nil {
		fmt.Printf("skipping dict: %s\n", pkg.pkg)
		return
	}
	out, err := os.Create(output)
	if err != nil {
		log.Panic(err)
	}
	defer func() {
		_ = out.Close()
	}()
	writeHeader(out)
	fprintf(out, "package %s\n", pkg.pkg)
	fprintf(out, "\n")
	fprint(out, "import \"github.com/semanticstep/sst-core/sst\"\n\ntype pkg struct{ sst.ElementInfo }\n\n")
	prefix := maybeReplaceKnownPrefix(strings.TrimSuffix(path.Base(vocab.BaseIRI), "#"))
	vocabularyPKG := strings.ToUpper(prefix)
	fprintf(out, "var %sVocabulary = sst.Vocabulary{BaseIRI: \"%s\"}\n\n", vocabularyPKG, vocab.BaseIRI)
	fprint(out, "var (\n\tPKG = sst.ElementPkg(pkg{})\n\t_   = PKG\n)\n\nvar (\n")
	ibFragments := make([]string, 0, graph.IRINodeCount())
	graph.ForIRINodes(func(d sst.IBNode) error {
		ibFragments = append(ibFragments, d.Fragment())
		return nil
	})
	sort.Slice(ibFragments, func(i, j int) bool {
		return strings.Compare(string(ibFragments[i]), string(ibFragments[j])) < 0
	})
	for _, f := range ibFragments {
		ibS := graph.GetIRINodeByFragment(f)
		s0 := string(ibS.Fragment())
		if s0 == "." || s0 == "" {
			continue
		}
		elemVarName := sst.Element{Name: s0}.GoSimpleName()
		fprintf(out,
			"\t%s = sst.Element{Vocabulary: %sVocabulary, Name: \"%s\"}\n",
			elemVarName,
			vocabularyPKG,
			ibS.Fragment(),
		)
	}
	fprintf(out, ")\n")
	data.imports = append(data.imports, path.Dir(output))
	data.namespaceToPrefix = append(data.namespaceToPrefix, namespaceToPrefix{
		pkg:        pkg.pkg,
		vocabulary: vocabularyPKG,
		style:      pkg.style,
	})
}

func main() {
	fmt.Printf("Ontology to SST Vocabulary Compiler\n")
	pkgI, err := os.Stat("vocabularies")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Chdir("../..")
			if err != nil {
				log.Panic(err)
			}
		} else {
			log.Panic(err)
		}
	} else if !pkgI.IsDir() {
		err = os.Chdir("../..")
		if err != nil {
			log.Panic(err)
		}
	}

	dict, err := LoadDictOntologies(ontologies.FS, "ontologies", log.Default())
	if err != nil {
		log.Panic(err)
	}

	// *.sst
	ontologies, err := generateDict(dict)
	if err != nil {
		log.Panic(err)
	}

	if err != nil {
		log.Panic(err)
	}

	// generate vocabulary.go
	var vocabMaps []vocabMapPkg
	var data vocabData

	for k, v := range neededCompileTTL {
		tempVocab := sst.Vocabulary{BaseIRI: k}
		compileSSTtoGO(ontologies[tempVocab], "vocabularies/"+v+"/vocabulary.go", tempVocab, &vocabMaps, &data)
	}

	for k, v := range neededCompileDictTTL {
		tempVocab := sst.Vocabulary{BaseIRI: k}
		dictSSTtoGO(ontologies[tempVocab], "vocabularies/"+v+"/dictionary.go", tempVocab, &data)
	}

	// vocabularymap.go
	err = writeVocabMap(vocabMaps, data)
	if err != nil {
		log.Panic(err)
	}
}

const destDictDir = "dict"

func writeVocabMap(vocabMaps []vocabMapPkg, data vocabData) error {
	vocabmapF, err := os.Create("vocabularies/dict/vocabularymap.go")
	if err != nil {
		return err
	}
	vocabmap := bufio.NewWriter(vocabmapF)
	defer vocabmapF.Close()

	writeHeader(vocabmap)
	fprintf(vocabmap, "package vocabularies\n")
	fprintf(vocabmap, "\n")
	fprint(vocabmap, "import (\n")
	fprintf(vocabmap, "\t\"embed\"\n")
	fprintf(vocabmap, "\t\"github.com/semanticstep/sst-core/sst\"\n")
	sort.Strings(data.imports)
	var prevVocabImport string
	for _, vocabImport := range data.imports {
		if prevVocabImport != vocabImport {
			fprintf(vocabmap, "\t\"github.com/semanticstep/sst-core/%s\"\n", vocabImport)
		}
		prevVocabImport = vocabImport
	}
	fprint(vocabmap, ")\n\n")

	fprint(vocabmap, "//go:embed *.sst\nvar dictFS embed.FS\n\n")

	fprint(vocabmap, "var keys = map[sst.Element]sst.ElementInformer{")

	// sort to get stable results
	sort.Slice(vocabMaps, func(i, j int) bool {
		return vocabMaps[i].pkg < vocabMaps[j].pkg
	})

	for _, vmPkg := range vocabMaps {
		// sort to get stable results
		sort.Strings(vmPkg.entries)
		if len(vmPkg.entries) != 0 {
			fprintln(vocabmap)
			vocabMapEntryMaxLen := 0
			for _, e := range vmPkg.entries {
				if vocabMapEntryMaxLen < len(e) {
					vocabMapEntryMaxLen = len(e)
				}
			}
			for _, e := range vmPkg.entries {
				fprintf(vocabmap, "\t%[1]s.%[2]s.Element:%*s%[1]s.%[2]s,\n", vmPkg.pkg, e, vocabMapEntryMaxLen-len(e)+1, " ")
			}
		}
	}

	fprintln(vocabmap, "}")
	fprintln(vocabmap, "")
	fprintln(vocabmap, "func init() {")
	// fprintln(vocabmap, "\tfor _, j := range keys {")
	// fprintln(vocabmap, "\t\tif j.InverseOf() != nil {")
	// fprintln(vocabmap, "\t\t\ttempInvereOfNode := j.InverseOf()")
	// fprintln(vocabmap, "\t\t\ttempInvereOfNode.SetInverseOf(j) ")
	// fprintln(vocabmap, "\t\t}")
	// fprintln(vocabmap, "\t}")

	fprintln(vocabmap, "\tsst.RegisterDictionary(keys, dictFS, map[string]string{")
	// sort to get stable results
	sort.Slice(data.namespaceToPrefix, func(i, j int) bool {
		return data.namespaceToPrefix[i].pkg < data.namespaceToPrefix[j].pkg
	})
	if len(data.namespaceToPrefix) > 0 {
		namespaceMaxLen := 0
		for _, nspfx := range data.namespaceToPrefix {
			len := len(nspfx.pkg) + len(nspfx.vocabulary)
			if namespaceMaxLen < len {
				namespaceMaxLen = len
			}
		}
		for _, nspfx := range data.namespaceToPrefix {
			len := len(nspfx.pkg) + len(nspfx.vocabulary)
			fprintf(
				vocabmap,
				"\t\t%[1]s.%[2]sVocabulary.BaseIRI:%[3]*s%[1]s.PKG%[5]s,\n",
				nspfx.pkg,
				nspfx.vocabulary,
				namespaceMaxLen-len+1, " ",
				nspfx.style.rangeString(),
			)
		}
		fprintln(vocabmap, "\t})")
	}
	fprintln(vocabmap, "}")
	return vocabmap.Flush()
}

func checkSubPropertyOfRangeDomain(d sst.IBNode) (RangeObject, DomainObject sst.IBNode) {
	var rdfsSubPropertyOfNode sst.IBNode
	d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s == d { // forward triple
			if o.TermKind() != sst.TermKindIBNode {
				return nil
			}
			o := o.(sst.IBNode)

			if p.Is(rdfsRange) && RangeObject == nil {
				RangeObject = o
			}

			if p.Is(rdfsDomain) && DomainObject == nil {
				DomainObject = o
			}

			if p.Is(rdfsSubPropertyOf) {
				rdfsSubPropertyOfNode = o
				RangeObject, DomainObject = checkSubPropertyOfRangeDomain(rdfsSubPropertyOfNode)
			}
		}
		return nil
	})
	return RangeObject, DomainObject
}

func generateDict(dict sst.Stage) (map[sst.Vocabulary]sst.NamedGraph, error) {
	graphs := make(map[sst.Vocabulary]sst.NamedGraph)
	var err error

	var namedGraphIRIs []sst.IRI
	for _, ng := range dict.NamedGraphs() {
		namedGraphIRIs = append(namedGraphIRIs, ng.IRI())
	}

	// =>	SyncInverseOf() // ensure that inverses are sync for a "pair of ObjectProperties"
	for _, namedGraphID := range namedGraphIRIs {
		if currentNamedGraph := dict.NamedGraph(namedGraphID); currentNamedGraph != nil {
			currentNamedGraph.ForIRINodes(func(d sst.IBNode) error {
				d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if s == d { // forward triple
						if o.TermKind() != sst.TermKindIBNode {
							return nil
						}
						o := o.(sst.IBNode)
						if p.Is(owlInverseOf) {
							// do not need err check, if duplicate triple insert, will ignored in AddStatement function
							o.AddStatementAlt(p, s)
						}
					}
					return nil
				})
				return nil
			})
		}
	}

	// =>	SyncSubProtertyOf() // ensure that for a "pair of ObjectProperties" the subProperties are in sync as well
	for _, namedGraphIRI := range namedGraphIRIs {
		if currentNamedGraph := dict.NamedGraph(namedGraphIRI); currentNamedGraph != nil {
			currentNamedGraph.ForIRINodes(func(d sst.IBNode) error {
				var tempSubPropertyOfPredicate sst.IBNode
				var tempSubPropertyOf sst.IBNode
				var tempInverseOf sst.IBNode
				var tempSubPropertyOfObject sst.IBNode

				d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if s == d { // forward triple
						if o.TermKind() != sst.TermKindIBNode {
							return nil
						}
						o := o.(sst.IBNode)

						if p.Is(rdfsSubPropertyOf) {
							tempSubPropertyOfPredicate = p
							tempSubPropertyOf = o
							o.ForAll(func(_ int, secondSub, secondPred sst.IBNode, secondObj sst.Term) error {
								if secondSub == o {
									if secondObj.TermKind() != sst.TermKindIBNode {
										return nil
									}
									oo := secondObj.(sst.IBNode)
									if secondPred.Is(owlInverseOf) {
										tempSubPropertyOfObject = oo
									}
								}
								return nil
							})
						}
						if p.Is(owlInverseOf) {
							tempInverseOf = o
						}
					}
					return nil
				})
				if tempSubPropertyOf != nil && tempInverseOf != nil && tempSubPropertyOfObject != nil {
					tempInverseOf.AddStatementAlt(tempSubPropertyOfPredicate, tempSubPropertyOfObject)
				}
				return nil
			})

		}
	}

	// =>	SyncDomainRange()
	// ensure that domain and ranges are in sync for a hierachy of "pairs of ObjectProperties"
	for _, namedGraphIRI := range namedGraphIRIs {
		if currentNamedGraph := dict.NamedGraph(namedGraphIRI); currentNamedGraph != nil {
			currentNamedGraph.ForIRINodes(func(d sst.IBNode) error {
				var tempInverseOf sst.IBNode
				var tempInverseOfRangeObject sst.IBNode
				var tempInverseOfDomainObject sst.IBNode

				d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if s == d { // forward triple
						if o.TermKind() != sst.TermKindIBNode {
							return nil
						}
						o := o.(sst.IBNode)
						if p.Is(owlInverseOf) {
							tempInverseOf = o
						}
						if p.Is(rdfsDomain) {
							tempInverseOfDomainObject = o
						}

						if p.Is(rdfsRange) {
							tempInverseOfRangeObject = o
						}
					}
					return nil
				})
				if tempInverseOf != nil && tempInverseOfRangeObject != nil {
					if len(tempInverseOf.GetObjects(rdfsDomain)) == 0 {
						tempInverseOf.AddStatement(rdfsDomain, tempInverseOfRangeObject)
					}
				}
				if tempInverseOf != nil && tempInverseOfDomainObject != nil {
					if len(tempInverseOf.GetObjects(rdfsRange)) == 0 {
						tempInverseOf.AddStatement(rdfsRange, tempInverseOfDomainObject)
					}
				}
				return nil
			})
		}
	}

	for _, namedGraphIRI := range namedGraphIRIs {
		if currentNamedGraph := dict.NamedGraph(namedGraphIRI); currentNamedGraph != nil {
			currentNamedGraph.ForIRINodes(func(d sst.IBNode) error {
				var tempInverseOfRangeObject sst.IBNode
				var tempInverseOfDomainObject sst.IBNode
				// var rdfsSubPropertyOfNode sst.IBNode

				if d != nil {
					tempInverseOfRangeObject, tempInverseOfDomainObject = checkSubPropertyOfRangeDomain(d)
				}

				if tempInverseOfDomainObject != nil {
					if len(d.GetObjects(rdfsDomain)) == 0 {
						d.AddStatement(rdfsDomain, tempInverseOfDomainObject)
					}
				}
				if tempInverseOfRangeObject != nil {
					if len(d.GetObjects(rdfsRange)) == 0 {
						d.AddStatement(rdfsRange, tempInverseOfRangeObject)
					}
				}
				return nil
			})
		}
	}

	// generate graphs for compileSSTtoGO
	for _, namedGraphIRI := range namedGraphIRIs {
		if currentNamedGraph := dict.NamedGraph(namedGraphIRI); currentNamedGraph != nil {
			graphs[sst.Vocabulary{BaseIRI: currentNamedGraph.IRI().String()}] = currentNamedGraph
		}
	}
	err = os.RemoveAll("vocabularies/" + destDictDir)
	if err != nil {
		log.Panic(err)
	}
	err = os.MkdirAll("vocabularies/"+destDictDir, 0o777)
	if err != nil {
		log.Panic(err)
	}
	err = dict.WriteToSstFilesWithBaseURL(wrfs.DirFS("vocabularies/" + destDictDir))

	if err != nil {
		log.Panic(err)
	}

	return graphs, err
}

func writeHeader(f io.StringWriter) {
	_, err := f.WriteString(`// Code generated by tools/compiler/compiler.go DO NOT EDIT.

// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

`)
	if err != nil {
		log.Panic(err)
	}
}

// copied from sst-core starts
const (
	typeMethodPrefix = "AsIs"
	kindMethodPrefix = "AsKind"
	hexDigitsUpper   = "0123456789ABCDEF"
)

// typeMethodOf generates a method name string for the given Elementer,
// using a prefix that indicates a "type" method. The resulting name is
// escaped to be a valid Go identifier and encodes the vocabulary base IRI
// and element name, ensuring uniqueness and avoiding invalid characters.
//
// Example output: "TypeMethod_http_3A_2F_2Fontology_2Esemanticstep_2Enet_2Flci_23AbstractIndividual"
func typeMethodOf(e sst.Elementer) string {
	return escapedMethod(e, typeMethodPrefix)
}

// kindMethodOf generates a method name string for the given Elementer,
// using a prefix that indicates a "kind" method. The resulting name is
// escaped to be a valid Go identifier and encodes the vocabulary base IRI
// and element name, ensuring uniqueness and avoiding invalid characters.
//
// Example output: "KindMethod_http_3A_2F_2Fontology_2Esemanticstep_2Enet_2Flci_23AbstractIndividual"
func kindMethodOf(e sst.Elementer) string {
	return escapedMethod(e, kindMethodPrefix)
}

func escapedMethod(e sst.Elementer, prefix string) string {
	el := e.VocabularyElement()
	// The hash symbol "#" here is for generating correct method names.
	// If there is no hash symbol, the method name will be generated as
	// AsKind_http_3A_2F_2Fontology_2Esemanticstep_2Enet_2FlciAbstractIndividual
	// lci and AbstractIndividual are concatenated together.
	// If there is a hash symbol, the method name will be generated as
	// AsKind_http_3A_2F_2Fontology_2Esemanticstep_2Enet_2Flci_23AbstractIndividual
	mParts := []string{el.Vocabulary.BaseIRI + "#", el.Name}
	// mParts := []string{el.Vocabulary.BaseIRI, el.Name}
	var escCnt int
	var appendUnderscore int
	for _, unescaped := range mParts {
		for _, c := range unescaped {
			if ('0' > c || c > '9') && ('a' > c || c > 'z') && ('A' > c || c > 'Z') {
				escCnt++
			}
			if appendUnderscore == 0 {
				appendUnderscore = 1
				if ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') {
					appendUnderscore = 2
				}
			}
		}
	}
	if appendUnderscore > 0 {
		appendUnderscore--
	}
	// also add "#" here
	escaped := make([]byte, 0, len(prefix)+appendUnderscore+len(el.Vocabulary.BaseIRI+"#")+len(el.Name)+(escCnt<<1))
	// escaped := make([]byte, 0, len(prefix)+appendUnderscore+len(el.Vocabulary.BaseIRI)+len(el.Name)+(escCnt<<1))
	escaped = append(escaped, prefix...)
	if appendUnderscore > 0 {
		escaped = append(escaped, '_')
	}
	for _, unescaped := range mParts {
		for _, c := range unescaped {
			if ('0' > c || c > '9') && ('a' > c || c > 'z') && ('A' > c || c > 'Z') {
				escaped = append(escaped, '_', hexDigitsUpper[c>>4], hexDigitsUpper[c&0xf])
				continue
			}
			escaped = append(escaped, byte(c))
		}
	}
	return string(escaped)
}

// ends

func fprintf(w io.Writer, format string, a ...interface{}) {
	_, err := fmt.Fprintf(w, format, a...)
	if err != nil {
		log.Panic(err)
	}
}

func fprint(w io.Writer, a ...interface{}) {
	_, err := fmt.Fprint(w, a...)
	if err != nil {
		log.Panic(err)
	}
}

func fprintln(w io.Writer, a ...interface{}) {
	_, err := fmt.Fprintln(w, a...)
	if err != nil {
		log.Panic(err)
	}
}

type ErrorReporter interface {
	Println(v ...interface{})
}

func LoadDictOntologies(fsys fs.FS, baseDir string, errs ErrorReporter) (sst.Stage, error) {
	combining := sst.OpenStage(sst.DefaultTriplexMode)
	err := fs.WalkDir(fsys, baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), "_") || d.Name() == "testdata" {
			return fs.SkipDir
		}
		var errorCount int
		tooManyErrors := errors.New("too many errors")
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".ttl") {
			var err error
			file, err := fsys.Open(path)
			defer func() {
				e := file.Close()
				if err == nil {
					err = e
				}
			}()

			var st sst.Stage
			fmt.Printf("Read %s\n", path)
			st, err = sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, func(err error) error {
				errs.Println(err)
				if errorCount > 50 {
					return tooManyErrors
				}
				errorCount++
				return nil
			}, sst.DefaultTriplexMode)
			if err != nil {
				return nil
			}
			if err != nil {
				return err
			}
			_, err = combining.MoveAndMerge(context.TODO(), st)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return combining, nil
}
