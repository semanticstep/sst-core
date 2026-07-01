// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"reflect"
)

const (
	typeMethodPrefix  = "AsIs"
	kindMethodPrefix  = "AsKind"
	hexDigitsUpper    = "0123456789ABCDEF"
	elementInfoFldNum = 0
	kindMethodsFldNum = 1
)

// Elementer is the base interface for all types that represent a vocabulary element.
// It provides a single method to obtain the underlying [Element] identity,
// which carries the vocabulary and the element name.
//
// This interface is implemented by [Element], [ElementInfo], and the generated
// ontology constants (e.g., lci.Part, rdfs.Label).
type (
	Elementer interface {
		// VocabularyElement returns the Element identity (vocabulary + name) for this element.
		VocabularyElement() Element
	}
)

func isKind(ei ElementInformer, kind Elementer) bool {
	if ei == nil {
		return false
	}
	ev := reflect.ValueOf(ei)
	et := ev.Type()
	_, ok := et.MethodByName(kindMethodOf(kind))
	return ok
}

// typeMethodOf generates a method name string for the given Elementer,
// using a prefix that indicates a "type" method. The resulting name is
// escaped to be a valid Go identifier and encodes the vocabulary base IRI
// and element name, ensuring uniqueness and avoiding invalid characters.
//
// Example output: "TypeMethod_http_3A_2F_2Fontology_2Esemanticstep_2Enet_2Flci_23AbstractIndividual"
func typeMethodOf(e Elementer) string {
	return escapedMethod(e, typeMethodPrefix)
}

// kindMethodOf generates a method name string for the given Elementer,
// using a prefix that indicates a "kind" method. The resulting name is
// escaped to be a valid Go identifier and encodes the vocabulary base IRI
// and element name, ensuring uniqueness and avoiding invalid characters.
//
// Example output: "KindMethod_http_3A_2F_2Fontology_2Esemanticstep_2Enet_2Flci_23AbstractIndividual"
func kindMethodOf(e Elementer) string {
	return escapedMethod(e, kindMethodPrefix)
}

func escapedMethod(e Elementer, prefix string) string {
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
