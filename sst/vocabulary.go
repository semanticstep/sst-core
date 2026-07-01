// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"embed"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"sync"
	"unicode"
)

// registerDictionaryProvider registers dictionaryProvider by provided dictFS fs.
func registerDictionaryProvider(dictFS fs.FS) {
	tempDictionaryStage := &dictionaryStage{}
	tempDictionaryStage.stageOnce.Do(func() {
		st, err := ReadStageFromSstFiles(dictFS, DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		repo, err := OpenLocalFlatFileSystemRepository(dictFS)
		if err != nil {
			panic(err)
		}

		st.(*stage).repo = repo
		tempDictionaryStage.Stage = st
		staticDictionaryStage = tempDictionaryStage
	})
}

type dictionaryStage struct {
	Stage
	stageOnce sync.Once
}

var staticDictionaryStage Dictionary

var ErrInvalidDictionaryStage = errors.New("invalid dictionary stage")

func (d *dictionaryStage) Vocabulary(v Vocabulary) (NamedGraph, error) {
	baseURI, err := NewIRI(v.BaseIRI)
	if err != nil {
		return nil, err
	}
	return d.NamedGraph(baseURI), nil
}

func (d *dictionaryStage) Element(e Element) (IBNode, error) {
	graph, err := d.Vocabulary(e.Vocabulary)
	if graph == nil || err != nil {
		return nil, err
	}
	return graph.GetIRINodeByFragment((e.Name)), nil
}

// The Dictionary interface is an extension of the Stage interface to provides access to the build in ontologies of SST in read-only mode.
// The single Dictionary object can be accessed by StaticDictionary().
// Dictionary provides access to dictionary [NamedGraph]s and [IBNode]s within the dictionary stage.
// The dictionary stage is distinct from application data stages and is read-only.
// It is linked to a read-only flat [Repository] backed by the embedded dictionary SST files,
// which can be accessed via the [Stage.Repository] method.
// It is completely managed by SST internally.
//
// Example:
//
//	lenIBNode, _ := StaticDictionary().Element(ssqau.Length)
type Dictionary interface {
	Stage
	// Vocabulary returns a NamedGraph for the given Vocabulary
	// that belongs to the read-only dictionary stage.
	Vocabulary(v Vocabulary) (NamedGraph, error)
	// Element returns an IBNode for the given VocabularyElement
	// that is located in a NamedGraph that belongs to the
	// read-only dictionary stage.
	Element(e Element) (IBNode, error)
}

// StaticDictionary returns the read-only dictionary stage.
func StaticDictionary() Dictionary {
	return staticDictionaryStage
}

var ErrElementInformerNotFound = errors.New("element informer not found")

var (
	vocabularyMap        map[Element]ElementInformer
	namespaceToPrefixMap map[string]string
)

// RegisterDictionary initializes the vocabulary map and namespace-to-prefix map,
// and registers the dictionary provider.
//
// Parameters:
//   - vm: Map where the key is an Element and the value is an ElementInformer.
//   - dictFS: Filesystem, that is typically embedded in the SST source code, containing the dictionary Repository as SST files.
//   - np: Map where the key is a namespace and the value is its corresponding prefix.
func RegisterDictionary(vm map[Element]ElementInformer, dictFS embed.FS, np map[string]string) {
	if vocabularyMap == nil {
		vocabularyMap = vm
	}
	namespaceToPrefixMap = np
	registerDictionaryProvider(dictFS)
}

// func addFromNamespaceToPrefixMap(np map[string]string) {
// 	for ns, pfx := range namespaceToPrefixMap {
// 		np[ns] = pfx
// 	}
// }

// NamespaceToPrefix returns prefix by given IRI.
// This also can be used to determine if the NameGraph is from the dictionary stage or not.
func NamespaceToPrefix(iri string) (prefix string, found bool) {
	if namespaceToPrefixMap == nil {
		GlobalLogger.Warn("Namespace to prefix map not set, please check if you have registered Dictionary")
	}
	prefix, found = namespaceToPrefixMap[iri]
	return
}

type (
	// The ElementInformer defines the methods that are available for
	// IRI node specific extensions of VocabularyElement.
	// This interface embeds the Elementer and Node interfaces.
	ElementInformer interface {
		Node
		Elementer
		IsClass() bool            // IsClass returns true if the element is an owl:Class.
		IsProperty() bool         // IsProperty returns true if the element is an rdf:Property or owl:DatatypeProperty or owl:ObjectProperty.
		IsDatatypeProperty() bool // IsDatatypeProperty returns true if the element is an owl:DatatypeProperty.
		IsObjectProperty() bool   // IsObjectProperty returns true if the element is an owl:ObjectProperty.
		IsDatatype() bool         // IsDatatype returns true if the element is an rdfs:Datatype.
		IsIndividual() bool       // IsIndividual returns true if the element is neither a class nor a property nor a datatype.

		// IsMainClass returns true if the target is directly or indirectly of the mainClass type.
		IsMainClass(mainClass Element) bool
		// Domain returns the rdfs:domain of a property.
		Domain() ElementInformer
		// Range returns the rdfs:range of a property.
		Range() ElementInformer
		// SubtypeOf returns the superclasses from zero or more rdfs:subClassOf statements.
		SubtypeOf() []ElementInformer
		// SubPropertyOf returns the super-property from rdfs:subPropertyOf statement, or nil.
		SubPropertyOf() ElementInformer
		// InverseOf returns the owl:inverseOf of a property, if available.
		InverseOf() ElementInformer
		// CollectionMember returns the collection member type.
		CollectionMember() ElementInformer
	}

	// A Vocabulary represents an externally defined ontology
	// that is made available for compile time checks
	// by early binding access constants and methods.
	// For this the externally defined ontology is compiled into a corresponding
	// GO package.
	// A Vocabulary contains VocabularyElements for the IRI nodes defined in the ontology.
	// Example: the package ssqau represents the Vocabulary for the quantities and units defined in ISO 80000.
	Vocabulary struct {
		BaseIRI string // the unique base URI of the externally defined ontology
	}
	// An Element represents an IRI node of a Vocabulary for early binding access.
	// For this purpose the SST Ontology Compiler generates a global variable
	// for each IRI node defined in the selected ontologies.
	// Element implements the Elementer interface.
	// Example: the physical quantity qau:Length as defined in ISO 80000.
	Element struct {
		Vocabulary Vocabulary
		Name       string
	}

	// A ElementInfo provides additional ontological information for an Element.
	// The fields of ElementInfo are not intended to be directly accessed by an application;
	// instead the methods for Element and ElementInformer are to be used.
	// ElementInfo and it's extensions are only generated for selected externally defined ontologies
	// that are not only treated as reference data but that contain upper level ontologies.
	// For such ontologies, the SST Ontology compiler generates global variables of types that
	// embeds ElementInfo structures but with no additional fields
	// so that compile time checks become available.
	// For these extended ElementInfo structures
	// the SST Ontology compiler also generates additional methods that
	// implement the ElementInformer interface.
	ElementInfo struct {
		Element
		AClass            bool // true if the element is an owl:Class.
		AnOptionClass     bool // true if the element is a ssmeta:OptionClass.
		AnAbstractClass   bool // true if the element is a ssmeta:AbstractClass.
		ARootClass        bool // true if the element is a ssmeta:RootClass.
		AProperty         bool // true if the element is an rdf:Property.
		ADatatypeProperty bool // true if the element is an owl:DatatypeProperty.
		AnObjectProperty  bool // true if the element is an owl:ObjectProperty.
		ADatatype         bool // true if the element is an rdfs:Datatype.
		AnIndividual      bool // true if the element is neither a class nor a property nor a datatype.

		// AMainClassSupersedure provides the main classes this element supersedes.
		// AMainClassSupersedure is set to empty map[Element]struct{} in case the
		// element denotes a main class but does not supersedes any other element.
		// AMainClassSupersedure is set to nil if this element is not a main class.
		AMainClassSupersedure map[Element]struct{}

		ADomain               ElementInformer   // the rdfs:domain of a property.
		ARange                ElementInformer   // the rdfs:range of a property.
		ACollectionMemberType ElementInformer   // the collection member type.
		ASubtypeOf            []ElementInformer // the superclasses from zero or more rdfs:subClassOf statements.

		ASubPropertyOf ElementInformer // the super-property from rdfs:subPropertyOf statement, or nil.
		AnInverseOf    ElementInformer // the owl:inverseOf of a property, if available.
	}
)

// ElementInformer looks up the ontological information for an element identified
// by name within this vocabulary. It returns ErrElementInformerNotFound if the
// element is not registered in the global vocabulary map.
func (v Vocabulary) ElementInformer(name string) (ElementInformer, error) {
	t := Element{Vocabulary: v, Name: name}
	if dt, found := vocabularyMap[t]; found {
		return dt, nil
	}
	return nil, ErrElementInformerNotFound
}

// VocabularyElement returns the Element identity itself.
// It satisfies the [Elementer] interface.
func (e Element) VocabularyElement() Element {
	return e
}

// ElementPkg takes an ElementInformer and gets its type info and removes the type name part from the type's string.
// Then it gives back the package name of the ElementInformer.
func ElementPkg(t ElementInformer) string {
	tt := reflect.TypeOf(t)
	return strings.TrimSuffix(tt.String(), "."+tt.Name())
}

// IsClass returns true if the element is an owl:Class.
func (i ElementInfo) IsClass() bool {
	return i.AClass
}

// IsProperty returns true if the element is an rdf:Property.
func (i ElementInfo) IsProperty() bool {
	return i.AProperty
}

// IsDatatypeProperty returns true if the element is an owl:DatatypeProperty.
func (i ElementInfo) IsDatatypeProperty() bool {
	return i.ADatatypeProperty
}

// IsObjectProperty returns true if the element is an owl:ObjectProperty.
func (i ElementInfo) IsObjectProperty() bool {
	return i.AnObjectProperty
}

// IsDatatype returns true if the element is an rdfs:Datatype.
func (i ElementInfo) IsDatatype() bool {
	return i.ADatatype
}

// IsIndividual returns true if the element is neither a class nor a property nor a datatype.
func (i ElementInfo) IsIndividual() bool {
	return i.AnIndividual
}

// IsMainClass returns true if the target is directly or indirectly of the mainClass type.
func (i ElementInfo) IsMainClass(prevMainClass Element) bool {
	if i.AMainClassSupersedure == nil {
		return false
	}
	if prevMainClass == (Element{}) {
		return true
	}
	_, superseded := i.AMainClassSupersedure[prevMainClass]
	return superseded
}

// Domain returns the rdfs:domain of a property.
func (i ElementInfo) Domain() ElementInformer {
	return i.ADomain
}

// Range returns the rdfs:range of a property.
func (i ElementInfo) Range() ElementInformer {
	return i.ARange
}

// CollectionMember returns the collection member type.
func (i ElementInfo) CollectionMember() ElementInformer {
	return i.ACollectionMemberType
}

// SubtypeOf returns the superclasses from zero or more rdfs:subClassOf statements.
func (i ElementInfo) SubtypeOf() []ElementInformer {
	return i.ASubtypeOf
}

// SubPropertyOf returns the super-property from rdfs:subPropertyOf statement, or nil.
func (i ElementInfo) SubPropertyOf() ElementInformer {
	return i.ASubPropertyOf
}

// InverseOf returns the owl:inverseOf of a property, if available.
func (i ElementInfo) InverseOf() ElementInformer {
	return i.AnInverseOf
}

// convertFragmentToGoName converts an RDF fragment (e.g., "part-of-speech")
// into a valid Go exported identifier (e.g., "PartOfSpeech").
func convertFragmentToGoName(s string) string {
	goName := strings.Builder{}
	toUpper := true
	for _, c := range s {
		switch c {
		case '_', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
			'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
			if toUpper {
				c = unicode.ToUpper(c)
			}
			toUpper = false
			goName.WriteRune(c)
		case '-':
			toUpper = true
		}
	}
	s1 := goName.String()
	s1 = capitalizeCommonInitialism(s1, "Id", "ID")
	s1 = capitalizeCommonInitialism(s1, "Iri", "IRI")
	s1 = capitalizeCommonInitialism(s1, "Url", "URL")
	s1 = capitalizeCommonInitialism(s1, "Uri", "URI")
	s1 = capitalizeCommonInitialism(s1, "Uuid", "UUID")
	s1 = capitalizeCommonInitialism(s1, "Uid", "UID")
	return s1
}

// capitalizeCommonInitialism replaces orig with replacement in s, but only
// when orig appears at the end of s or is followed by an uppercase letter.
// It is used to turn "Url" into "URL", "Iri" into "IRI", etc.
func capitalizeCommonInitialism(s, orig, replacement string) string {
	if pos := strings.Index(s, orig); pos >= 0 {
		if pos+len(orig) == len(s) || unicode.IsUpper(rune(s[pos+len(orig)])) {
			return s[:pos] + replacement + s[pos+len(orig):]
		}
	}
	return s
}

// GoSimpleName provides the name of the corresponding GO variable for that element.
func (e Element) GoSimpleName() string {
	return convertFragmentToGoName(e.Name)
}

// ResourceID returns the RDF resource identifier of the element as IRI string.
func (e Element) IRI() IRI {
	iri, err := NewIRI(e.Vocabulary.BaseIRI + "#" + e.Name)
	if err != nil {
		panic(err)
	}
	return iri
}

// TermKind returns TermKindIBNode, indicating that every vocabulary element
// is represented as an IRI node in RDF terms.
func (Element) TermKind() TermKind {
	return TermKindIBNode
}

// IsTermCollection returns false; a vocabulary element is never a collection.
func (Element) IsTermCollection() bool {
	return false
}

// IRI is a specialization of IRI for the purpose of SST Vocabularies.
type IRI string

// TermKind returns TermKindIBNode, indicating that an IRI is treated as an
// IRI node in RDF terms.
func (IRI) TermKind() TermKind {
	return TermKindIBNode
}

// IsTermCollection returns false; a plain IRI is never a collection.
func (IRI) IsTermCollection() bool {
	return false
}

// IRI returns the IRI value itself.
func (i IRI) IRI() IRI { return IRI(i) }

// VocabularyElement returns an Element whose vocabulary is derived from the
// base portion of this IRI and whose name is the fragment portion.
// It satisfies the [Elementer] interface.
func (i IRI) VocabularyElement() Element {
	prefix, suffix := IRI(i).Split()
	return Element{
		Vocabulary: Vocabulary{BaseIRI: prefix},
		Name:       suffix,
	}
}
