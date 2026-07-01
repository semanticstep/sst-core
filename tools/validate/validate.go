// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// Package validate contains the code to validate SST data contained in a Stage.
package validate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/ssmeta"
	"github.com/semanticstep/sst-core/vocabularies/xsd"
	"go.uber.org/zap"
)

var (
	ErrRDFTypeMissing = &tripleFormatterError{message: "rdf:type missing", format: "missing%*s%*s%*s rdf:type"}
	ErrDomainMismatch = &tripleFormatterError{message: "domain mismatch", format: "domain mismatch%*s%*s%*s"}
	ErrRangeMismatch  = &tripleFormatterError{message: "range mismatch", format: "range mismatch%*s%*s%*s"}
	errBreakFor       = errors.New("break for")
)

type ValidationName string

// copyStageInto copies all local NamedGraphs from src into dst by serialising
// each graph to Turtle and reading it back into dst.  The temporary stages
// created during copying are discarded afterwards, so only dst is modified.
func copyStageInto(dst, src sst.Stage) error {
	for _, ng := range src.NamedGraphs() {
		if ng == nil || !ng.IsValid() {
			continue
		}
		var buf bytes.Buffer
		if err := ng.RdfWrite(&buf, sst.RdfFormatTurtle); err != nil {
			return err
		}
		tempStage, err := sst.RdfRead(bufio.NewReader(&buf), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			return err
		}
		_, err = dst.MoveAndMerge(context.Background(), tempStage)
		if err != nil {
			return err
		}
	}
	return nil
}

// ValidateAll runs all validation kinds on the given stage and returns a report.
// validationStage contains the data to be validated.
// referenceStages contains additional data that might be referenced from the validationStage
// and might be needed to perform the validation. For example, a property with its domain
// and range might be defined in a reference stage and used in the validationStage.
// Neither validationStage nor referenceStages are modified.
func ValidateAll(validationStage sst.Stage, referenceStages ...sst.Stage) (*ValidateReport, error) {
	// Remember original named graphs from validationStage
	originalGraphs := validationStage.NamedGraphs()
	originalIRIs := make([]sst.IRI, len(originalGraphs))
	for i, ng := range originalGraphs {
		originalIRIs[i] = ng.IRI()
	}

	// Build a working stage so we never mutate the inputs.
	workingStage := sst.OpenStage(sst.DefaultTriplexMode)

	if err := copyStageInto(workingStage, validationStage); err != nil {
		return nil, err
	}
	for _, refStage := range referenceStages {
		if refStage == nil || !refStage.IsValid() {
			continue
		}
		if err := copyStageInto(workingStage, refStage); err != nil {
			return nil, err
		}
	}

	kinds := []ValidationKind{KindRdfType, KindDomainRange, KindFunctionalProperty}
	report := NewReport(kinds...)
	for _, iri := range originalIRIs {
		graph := workingStage.NamedGraph(iri)
		if graph == nil || !graph.IsValid() {
			continue
		}
		for _, s := range kinds {
			switch s {
			case KindRdfType:
				err := RdfType(graph, &report, outputLog{graph})
				if err != nil {
					return &report, err
				}
			case KindDomainRange:
				err := DomainAndRange(graph, &report, outputLog{graph})
				if err != nil {
					return &report, err
				}
			case KindFunctionalProperty:
				err := FunctionalProperty(graph, &report, outputLog{graph})
				if err != nil {
					return &report, err
				}
			}
		}
	}
	return &report, nil
}

type predicateObject struct {
	p sst.IBNode
	o sst.Term
}

func elementInformerString(ei sst.ElementInformer) string {
	if ei == nil {
		return "<nil>"
	}
	el := ei.VocabularyElement()
	if pfx, found := sst.NamespaceToPrefix(el.Vocabulary.BaseIRI); found {
		return pfx + ":" + el.Name
	}
	return "<" + el.Vocabulary.BaseIRI + "#" + el.Name + ">"
}

func ibNodeString(graph sst.NamedGraph, n sst.IBNode) string {
	if n == nil || !n.IsValid() {
		return "<nil>"
	}
	s := graphPrefixedFragment(graph, n)
	if s != "" {
		return s
	}
	if n.IsBlankNode() {
		return "_:" + n.ID().String()
	}
	return "<" + string(n.IRI()) + ">"
}

func ibNodeListString(graph sst.NamedGraph, nodes []sst.IBNode) string {
	if len(nodes) == 0 {
		return "(none)"
	}
	var parts []string
	for _, n := range nodes {
		parts = append(parts, ibNodeString(graph, n))
	}
	return strings.Join(parts, ", ")
}

func ibNodeTypesString(graph sst.NamedGraph, n sst.IBNode) string {
	var types []sst.IBNode
	_ = n.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if n != s {
			return nil
		}
		if p.Is(rdf.Type) && (o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection) {
			types = append(types, o.(sst.IBNode))
		}
		return nil
	})
	return ibNodeListString(graph, types)
}

func FunctionalProperty(graph sst.NamedGraph, report *ValidateReport, log Logger) error {
	const validationName = ValidationName("FunctionalProperty")
	err := log.LogForGraph(InfoEnterLevel, (graph), validationName)
	if err != nil {
		return err
	}

	inverseFunctionalPropMap := make(map[predicateObject]int)

	err = graph.ForAllIBNodes(func(d sst.IBNode) error {
		err := log.Log(InfoLevel, d)
		if err != nil {
			return err
		}
		// skip NG Node
		if d.IsIRINode() && d.Fragment() == "" {
			return nil
		}

		functionalPropMap := make(map[sst.IBNode]int)
		sst.GlobalLogger.Debug("functional map empty")

		err = d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if d != s {
				return nil
			}
			sst.GlobalLogger.Debug("", zap.String("subj", ibNodeString(graph, s)), zap.String("pred", ibNodeString(graph, p)))
			// if p is dictionary vocabulary, replace it, so it can get all infomation from vocabulary
			if p.InVocabulary() != nil {
				p, err = sst.StaticDictionary().IBNodeByVocabulary(p.InVocabulary())
				if err != nil {
					return err
				}
			}
			// sst.DebugIBNode(p)
			if isFunctionalProperty(p, functionalPropMap) {
				for key, val := range functionalPropMap {
					if val > 1 {
						err := log.Log(ErrorLevel, d, ErrDomainMismatch, p, o)
						if err != nil {
							return err
						}
						report.Error(string(graph.IRI()), Finding{
							Kind:    KindFunctionalProperty,
							Rule:    RulePredicateFunctionalProperty,
							Message: "FunctionalProperty " + key.PrefixedFragment() + " must be used only once for a subject",
							S:       ibNodeValuesToLogString(graph, d),
							P:       ibNodeValuesToLogString(graph, p),
							O:       valuesToLogString(graph, o),
						})
					}
				}
			} else {
				sst.GlobalLogger.Debug("Not a functionalProperty", zap.String("", p.Fragment()))
			}

			if isInverseFunctionalProperty(p, o, inverseFunctionalPropMap) {
				for key, val := range inverseFunctionalPropMap {
					if val > 1 {
						err := log.Log(ErrorLevel, d, ErrDomainMismatch, p, o)
						if err != nil {
							return err
						}
						report.Error(string(graph.IRI()), Finding{
							Kind:    KindFunctionalProperty,
							Rule:    RulePredicateInverseFunctionalProperty,
							Message: "InverseFunctionalProperty " + key.p.PrefixedFragment() + " must be used only once for an object",
							S:       ibNodeValuesToLogString(graph, d),
							P:       ibNodeValuesToLogString(graph, p),
							O:       valuesToLogString(graph, o),
						})
					}
				}
			} else {
				sst.GlobalLogger.Debug("Not a InverseFunctionalProperty", zap.String("", p.Fragment()))
			}
			return nil
		})

		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return log.LogForGraph(InfoLeaveLevel, (graph), validationName)
}

func RdfType(graph sst.NamedGraph, report *ValidateReport, log Logger) error {
	const validationName = ValidationName("ValidateRdfType")
	err := log.LogForGraph(InfoEnterLevel, (graph), validationName)
	if err != nil {
		return err
	}
	err = graph.ForAllIBNodes(func(d sst.IBNode) error {
		err := log.Log(InfoLevel, d)
		if err != nil {
			return err
		}

		var types []sst.IBNode
		err = d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if d != s {
				return nil
			}
			if p.Is(rdf.Type) && (o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection) {
				types = append(types, o.(sst.IBNode))
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}

		// skip NG Node
		if d.IsIRINode() && d.Fragment() == "" {
			return nil
		}

		// skip TermCollection rdf:type checking
		if d.IsTermCollection() {
			return nil
		}

		if len(types) == 0 {
			err := log.Log(ErrorLevel, d, ErrRDFTypeMissing)
			if err != nil {
				return err
			}
			report.Error(string(graph.IRI()), Finding{
				Kind:    KindRdfType,
				Rule:    RuleRdfTypeMissing,
				Message: ibNodeValuesToLogString(graph, d) + " has no rdf:type",
				S:       ibNodeValuesToLogString(graph, d),
			})
		} else {
			isMainClass := false
			isPropertyType := false

			// skip class definitions
			for _, t := range types {
				if t.Is(owl.Class) {
					return nil
				}
			}

			for _, t := range types {
				if isMainClassNode(t) {
					isMainClass = true
					break
				}
			}

			// a rdf:Property, owl:ObjectProperty or owl:DatatypeProperty together with a statement
			// that the subject is a rdfs:subPropertyOf of a property defined in one of the SST Vocabularies or any subProperty of these.
			// The answer has to be found within the current dataset, otherwise it is an error.
			isPropertyType = isValidProperty(d)

			if isMainClass {
				sst.GlobalLogger.Debug("isMainClass found", zap.String("node", ibNodeValuesToLogString(graph, d)))
			}

			if isPropertyType {
				sst.GlobalLogger.Debug("isPropertyType found", zap.String("node", ibNodeValuesToLogString(graph, d)))
			}

			if !isMainClass && !isPropertyType {
				err := log.Log(ErrorLevel, d, ErrRDFTypeMissing)
				if err != nil {
					return err
				}
				report.Error(string(graph.IRI()), Finding{
					Kind:    KindRdfType,
					Rule:    RuleRdfTypeWrong,
					Message: ibNodeValuesToLogString(graph, d) + " does not contain a type that is either property or main class",
					S:       ibNodeValuesToLogString(graph, d),
				})
			}

		}
		return nil
	})
	if err != nil {
		return err
	}
	return log.LogForGraph(InfoLeaveLevel, (graph), validationName)
}

// value pass.
func isFunctionalProperty(subj sst.IBNode, all map[sst.IBNode]int) bool {
	isFunctionalPropBool := false
	subj.ForAll(func(_ int, s, predPred sst.IBNode, o sst.Term) error {
		if subj == s {
			if predPred.Is(rdf.Type) && o.TermKind() == sst.TermKindIBNode {
				if o.(sst.IBNode).Is(owl.FunctionalProperty) {
					_, ok := all[subj]
					if ok {
						all[subj]++
					} else {
						all[subj] = 1
					}
					// sst.DebugIBNode(subj)
					sst.GlobalLogger.Debug("functional map modified", zap.String("", subj.Fragment()), zap.Int("", all[subj]))
					isFunctionalPropBool = true
					return nil
				}
			}
		}
		return nil
	})

	// if !isFunctionalPropBool {
	subj.ForAll(func(_ int, s, predPred sst.IBNode, oo sst.Term) (err error) {
		if subj == s {
			if predPred.Is(rdfs.SubPropertyOf) && oo.TermKind() == sst.TermKindIBNode {
				if oo.(sst.IBNode).InVocabulary() != nil {
					oo, err = sst.StaticDictionary().IBNodeByVocabulary(oo.(sst.IBNode).InVocabulary())
					if err != nil {
						return err
					}
				}
				isFunctionalPropBool = isFunctionalPropBool || isFunctionalProperty(oo.(sst.IBNode), all)
			}
		}
		return nil
	})
	// }

	return isFunctionalPropBool
}

func isInverseFunctionalProperty(subj sst.IBNode, obj sst.Term, all map[predicateObject]int) bool {
	isInverseFunctionalPropBool := false
	subj.ForAll(func(_ int, s, predPred sst.IBNode, o sst.Term) error {
		if subj == s {
			if predPred.Is(rdf.Type) && o.TermKind() == sst.TermKindIBNode {
				if o.(sst.IBNode).Is(owl.InverseFunctionalProperty) {
					_, ok := all[predicateObject{subj, obj}]
					if ok {
						all[predicateObject{subj, obj}]++
					} else {
						all[predicateObject{subj, obj}] = 1
					}

					sst.GlobalLogger.Debug("inverseFunctional map modified",
						zap.String("", subj.Fragment()), zap.String("", obj.(sst.IBNode).Fragment()),
						zap.Int("", all[predicateObject{subj, obj}]))
					isInverseFunctionalPropBool = true
					return nil
				}
			}
		}
		return nil
	})

	// if !isInverseFunctionalPropBool {
	subj.ForAll(func(_ int, s, predPred sst.IBNode, oo sst.Term) (err error) {
		if subj == s {
			if predPred.Is(rdfs.SubPropertyOf) && oo.TermKind() == sst.TermKindIBNode {
				if oo.(sst.IBNode).InVocabulary() != nil {
					oo, err = sst.StaticDictionary().IBNodeByVocabulary(oo.(sst.IBNode).InVocabulary())
					if err != nil {
						return err
					}
				}
				isInverseFunctionalPropBool = isInverseFunctionalPropBool || isInverseFunctionalProperty(oo.(sst.IBNode), obj, all)
			}
		}
		return nil
	})
	// }

	return isInverseFunctionalPropBool
}

func isMainClassNode(t sst.IBNode) bool {
	if tElementInfo := t.InVocabulary(); tElementInfo != nil {
		return tElementInfo.IsMainClass(sst.Element{})
	}

	isMainClass := false
	t.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if t != s {
			return nil
		}
		if p.Is(rdf.Type) && o.TermKind() == sst.TermKindIBNode {
			if o.(sst.IBNode).Is(ssmeta.MainClass) {
				isMainClass = true
				return errBreakFor
			}
		}
		if p.Is(rdfs.SubClassOf) && o.TermKind() == sst.TermKindIBNode {
			oo := o.(sst.IBNode)
			if isMainClassNode(oo) {
				isMainClass = true
				return errBreakFor
			}
		}
		return nil
	})
	return isMainClass
}

func isValidProperty(subj sst.IBNode) bool {
	isPropertyBool := false
	subj.ForAll(func(_ int, s, predPred sst.IBNode, o sst.Term) error {
		if subj == s {
			if predPred.Is(rdf.Type) && o.TermKind() == sst.TermKindIBNode {
				if o.(sst.IBNode).Is(rdf.Property) || o.(sst.IBNode).Is(owl.ObjectProperty) || o.(sst.IBNode).Is(owl.DatatypeProperty) {
					subj.ForAll(func(_ int, s, predPred sst.IBNode, oo sst.Term) error {
						if subj == s {
							if predPred.Is(rdfs.SubPropertyOf) && oo.TermKind() == sst.TermKindIBNode {
								oVoc := oo.(sst.IBNode).InVocabulary()
								if oVoc != nil {
									if oVoc.IsProperty() {
										isPropertyBool = true
										return nil
									}
								} else {
									isPropertyBool = isValidProperty(oo.(sst.IBNode))
									return nil
								}
							}
						}
						return nil
					})
				}
				return nil
			}
		}
		return nil
	})
	return isPropertyBool
}

func literalCheck(rang sst.ElementInformer, li sst.Literal) (expected string, real string, ok bool) {
	expected = elementInformerString(rang)

	// rdfs:Literal matches any literal
	if _, isLiteral := rang.(rdfs.KindLiteral); isLiteral {
		return expected, real, true
	}

	switch li.(type) {
	case sst.LangString:
		_, ok = rang.(rdf.KindLangString)
		real = "rdf:langString"
	case sst.String:
		_, ok = rang.(xsd.KindString)
		real = "xsd:string"
	case sst.Double:
		_, ok = rang.(xsd.KindDouble)
		real = "xsd:double"
	case sst.Float:
		_, ok = rang.(xsd.KindFloat)
		real = "xsd:float"
	case sst.Integer:
		_, ok = rang.(xsd.KindInteger)
		real = "xsd:integer"
	case sst.Int:
		_, ok = rang.(xsd.KindInt)
		real = "xsd:int"
	case sst.Long:
		_, ok = rang.(xsd.KindLong)
		real = "xsd:long"
	case sst.Short:
		_, ok = rang.(xsd.KindShort)
		real = "xsd:short"
	case sst.Byte:
		_, ok = rang.(xsd.KindByte)
		real = "xsd:byte"
	case sst.Boolean:
		_, ok = rang.(xsd.KindBoolean)
		real = "xsd:boolean"
	default:
		dt := li.DataType()
		if dt != nil && dt.IsValid() {
			real = dt.PrefixedFragment()
			if real == "" {
				real = string(dt.IRI())
			}
		}
		if real == "" {
			real = "(unknown)"
		}
	}

	return
}

func DomainAndRange(graph sst.NamedGraph, report *ValidateReport, log Logger) error {
	const validationName = ValidationName("ValidateDomainAndRange")
	err := log.LogForGraph(InfoEnterLevel, (graph), validationName)
	if err != nil {
		return err
	}

	// Build an in-memory index of rdfs:domain, rdfs:range and rdfs:subClassOf
	// declared inside the graph itself so that late-binding validation works
	// for predicates/classes that are not part of a compiled vocabulary.
	idx := buildGraphIndex(graph)

	err = graph.ForAllIBNodes(func(d sst.IBNode) error {
		var types []sst.IBNode
		type predicateObject struct {
			p sst.IBNode
			o sst.Term
		}
		var predicateObjectTuples []predicateObject
		err = d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if d != s {
				return nil
			}

			// skip TermCollection rdf:type checking
			// if d.IsTermCollection() {
			// return nil
			// }

			if p.Is(rdf.Type) && (o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection) {
				types = append(types, o.(sst.IBNode))
				return nil
			}
			predicateObjectTuples = append(predicateObjectTuples, predicateObject{p: p, o: o})

			pElementInfo := p.InVocabulary()
			pIsProperty := false

			// check in compiled vocabulary (early binding)
			if pElementInfo != nil && pElementInfo.IsProperty() {
				pIsProperty = true
			}

			// check in graph triples (late binding)
			if !pIsProperty {
				pIsProperty = isValidProperty(p)
			}

			if !pIsProperty {
				err := log.Log(ErrorLevel, d, ErrDomainMismatch, p, o)
				if err != nil {
					return err
				}
				report.Error(string(graph.IRI()), Finding{
					Kind:    KindDomainRange,
					Rule:    RulePredicateNotProperty,
					Message: "the predicate is not a valid property, found types " + ibNodeTypesString(graph, p),
					S:       ibNodeValuesToLogString(graph, d),
					P:       ibNodeValuesToLogString(graph, p),
					O:       valuesToLogString(graph, o),
				})
			}

			return nil
		})
		if err != nil {
			return err
		}
		for _, po := range predicateObjectTuples {
			// when p is rdf.first, means the subject is the TermCollection BlankNode, so skip checking its domain
			if po.p.Is(rdf.First) {
				continue
			}
			pred := po.p.InVocabulary()
			if pred != nil {
				// === Early binding: predicate is in compiled vocabulary ===
				if domain := pred.Domain(); domain != nil {
					err := log.Log(InfoLevel, d, "domain", po.p, po.o)
					if err != nil {
						return err
					}
					var found bool
					for _, t := range types {
						if t.IsKind(domain) {
							found = true
							break
						}
					}
					if !found {
						err := log.Log(ErrorLevel, d, ErrDomainMismatch, po.p, po.o)
						if err != nil {
							return err
						}
						report.Error(string(graph.IRI()), Finding{
							Kind:    KindDomainRange,
							Rule:    RuleDomainMismatch,
							Message: "domain mismatch, expected " + elementInformerString(domain) + ", found " + ibNodeListString(graph, types),
							S:       ibNodeValuesToLogString(graph, d),
							P:       ibNodeValuesToLogString(graph, po.p),
							O:       valuesToLogString(graph, po.o),
						})
					}
				}
				if rang := pred.Range(); rang != nil {
					switch po.o.TermKind() {
					case sst.TermKindIBNode:
						o := po.o.(sst.IBNode)
						if o.OwningGraph().IsReferenced() {
							if vocab := o.InVocabulary(); vocab != nil {
								v, err := sst.StaticDictionary().Element(vocab.VocabularyElement())
								if v != nil && err == nil {
									o = v
								}
							}
						}
						if !o.OwningGraph().IsReferenced() {
							err := log.Log(InfoLevel, d, "range", po.p, po.o)
							if err != nil {
								return err
							}
							var found bool
							err = o.ForAll(func(_ int, os, op sst.IBNode, oo sst.Term) error {
								if o != os {
									return nil
								}
								if op.Is(rdf.Type) && (oo.TermKind() == sst.TermKindIBNode || oo.TermKind() == sst.TermKindTermCollection) {
									if oo.(sst.IBNode).IsKind(rang) {
										found = true
										return errBreakFor
									}
								}
								return nil
							})
							if err != nil && err != errBreakFor { // nolint:errorlint
								return err
							}
							if !found {
								err := log.Log(ErrorLevel, d, ErrRangeMismatch, po.p, o)
								if err != nil {
									return err
								}
								report.Error(string(graph.IRI()), Finding{
									Kind:    KindDomainRange,
									Rule:    RuleRangeMismatch,
									Message: "range mismatch, expected " + elementInformerString(rang) + ", found " + ibNodeTypesString(graph, o),
									S:       ibNodeValuesToLogString(graph, d),
									P:       ibNodeValuesToLogString(graph, po.p),
									O:       valuesToLogString(graph, po.o),
								})
							}
						}

					case sst.TermKindTermCollection:
						tc := po.o.(sst.TermCollection)

						_, isList := rang.(rdf.KindList)

						if !isList {
							err := log.Log(ErrorLevel, d, ErrRangeMismatch, po.p, po.o)
							if err != nil {
								return err
							}
							report.Error(string(graph.IRI()), Finding{
								Kind:    KindDomainRange,
								Rule:    RuleRangeMismatch,
								Message: "Range of predicate " + valuesToLogString(graph, po.p) + " is not a TermCollection, expected " + elementInformerString(rang),
								S:       ibNodeValuesToLogString(graph, d),
								P:       ibNodeValuesToLogString(graph, po.p),
								O:       "",
							})
							return nil
						}

						rang = rang.CollectionMember()

						tc.ForMembers(func(e int, o sst.Term) {
							foundDesiredType := false
							switch o.TermKind() {
							case sst.TermKindIBNode:
								err = o.(sst.IBNode).ForAll(func(_ int, os, op sst.IBNode, oo sst.Term) error {
									if o != os {
										return nil
									}
									if op.Is(rdf.Type) && (oo.TermKind() == sst.TermKindIBNode || oo.TermKind() == sst.TermKindTermCollection) {
										if oo.(sst.IBNode).IsKind(rang) {
											foundDesiredType = true
											return errBreakFor
										}
									}
									return nil
								})
								if !foundDesiredType {
									err := log.Log(ErrorLevel, d, ErrRangeMismatch, po.p, o)
									if err != nil {
										return
									}
									report.Error(string(graph.IRI()), Finding{
										Kind:    KindDomainRange,
										Rule:    RuleRangeMismatch,
										Message: "Term Collection member type mismatch, expected " + elementInformerString(rang) + ", found " + ibNodeTypesString(graph, o.(sst.IBNode)),
										S:       ibNodeValuesToLogString(graph, d),
										P:       ibNodeValuesToLogString(graph, po.p),
										O:       valuesToLogString(graph, o),
									})
								}
							case sst.TermKindTermCollection:
								if ibo, ok := o.(sst.IBNode); ok {
									sst.GlobalLogger.Debug("KindIBNode", zap.String("node", ibNodeString(graph, ibo)))
								}

							case sst.TermKindLiteral:
								err := log.Log(InfoLevel, d, "range", po.p, o)
								if err != nil {
									return
								}

								expected, real, ok := literalCheck(rang, o.(sst.Literal))

								if !ok {
									err := log.Log(ErrorLevel, d, ErrRangeMismatch, po.p, po.o)
									if err != nil {
										return
									}
									report.Error(string(graph.IRI()), Finding{
										Kind:    KindDomainRange,
										Rule:    RuleRangeMismatch,
										Message: "range mismatch, expected " + expected + ", found " + real,
										S:       ibNodeValuesToLogString(graph, d),
										P:       ibNodeValuesToLogString(graph, po.p),
										O:       valuesToLogString(graph, o),
									})
								}

							case sst.TermKindLiteralCollection:
								sst.GlobalLogger.Debug("KindLiteralCollection", zap.String("type", fmt.Sprintf("%T", o)))

							default:
								sst.GlobalLogger.Debug("default term", zap.String("term", fmt.Sprintf("%s", o)))
							}
						})

					case sst.TermKindLiteral:
						err := log.Log(InfoLevel, d, "range", po.p, po.o)
						if err != nil {
							return err
						}

						expected, real, ok := literalCheck(rang, po.o.(sst.Literal))

						if !ok {
							err := log.Log(ErrorLevel, d, ErrRangeMismatch, po.p, po.o)
							if err != nil {
								return err
							}
							report.Error(string(graph.IRI()), Finding{
								Kind:    KindDomainRange,
								Rule:    RuleRangeMismatch,
								Message: "range mismatch, expected " + expected + ", found " + real,
								S:       ibNodeValuesToLogString(graph, d),
								P:       ibNodeValuesToLogString(graph, po.p),
								O:       valuesToLogString(graph, po.o),
							})
						}
					case sst.TermKindLiteralCollection:
						err := log.Log(InfoLevel, d, "range", po.p, po.o)
						if err != nil {
							return err
						}
						if _, ok := rang.(rdf.KindList); !ok {
							err := log.Log(ErrorLevel, d, ErrRangeMismatch, po.p, po.o)
							if err != nil {
								return err
							}
							report.Error(string(graph.IRI()), Finding{
								Kind:    KindDomainRange,
								Rule:    RuleRangeMismatch,
								Message: "range mismatch, expected " + elementInformerString(rang) + ", found " + "literal collection",
								S:       ibNodeValuesToLogString(graph, d),
								P:       ibNodeValuesToLogString(graph, po.p),
								O:       valuesToLogString(graph, po.o),
							})
							return nil
						}

						rang = rang.CollectionMember()

						tlc := po.o.(sst.LiteralCollection)

						expected, real, ok := literalCheck(rang, tlc.Member(0))

						if !ok {
							err := log.Log(ErrorLevel, d, ErrRangeMismatch, po.p, po.o)
							if err != nil {
								return err
							}

							report.Error(string(graph.IRI()), Finding{
								Kind:    KindDomainRange,
								Rule:    RuleRangeMismatch,
								Message: "range mismatch, expected " + expected + ", found " + real,
								S:       ibNodeValuesToLogString(graph, d),
								P:       ibNodeValuesToLogString(graph, po.p),
								O:       valuesToLogString(graph, po.o),
							})
						}
					}
				}
			} else {
				// === Late binding: predicate not in compiled vocabulary ===
				if domain := domainFromGraph(idx, po.p); domain != nil {
					err := log.Log(InfoLevel, d, "domain", po.p, po.o)
					if err != nil {
						return err
					}
					var found bool
					for _, t := range types {
						if isKindFromGraph(idx, t, domain) {
							found = true
							break
						}
					}
					if !found {
						err := log.Log(ErrorLevel, d, ErrDomainMismatch, po.p, po.o)
						if err != nil {
							return err
						}
						report.Error(string(graph.IRI()), Finding{
							Kind:    KindDomainRange,
							Rule:    RuleDomainMismatch,
							Message: "domain mismatch (late binding), expected " + ibNodeString(graph, domain) + ", found " + ibNodeListString(graph, types),
							S:       ibNodeValuesToLogString(graph, d),
							P:       ibNodeValuesToLogString(graph, po.p),
							O:       valuesToLogString(graph, po.o),
						})
					}
				}
				if rang := rangeFromGraph(idx, po.p); rang != nil {
					switch po.o.TermKind() {
					case sst.TermKindIBNode:
						o := po.o.(sst.IBNode)
						err := log.Log(InfoLevel, d, "range", po.p, po.o)
						if err != nil {
							return err
						}
						var found bool
						err = o.ForAll(func(_ int, os, op sst.IBNode, oo sst.Term) error {
							if o != os {
								return nil
							}
							if op.Is(rdf.Type) && oo.TermKind() == sst.TermKindIBNode {
								if isKindFromGraph(idx, oo.(sst.IBNode), rang) {
									found = true
									return errBreakFor
								}
							}
							return nil
						})
						if err != nil && err != errBreakFor { // nolint:errorlint
							return err
						}
						if !found {
							err := log.Log(ErrorLevel, d, ErrRangeMismatch, po.p, o)
							if err != nil {
								return err
							}
							report.Error(string(graph.IRI()), Finding{
								Kind:    KindDomainRange,
								Rule:    RuleRangeMismatch,
								Message: "range mismatch (late binding), expected " + ibNodeString(graph, rang) + ", found " + ibNodeTypesString(graph, o),
								S:       ibNodeValuesToLogString(graph, d),
								P:       ibNodeValuesToLogString(graph, po.p),
								O:       valuesToLogString(graph, po.o),
							})
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return log.LogForGraph(InfoLeaveLevel, (graph), validationName)
}

func ExperimentalNamedGraphForTypeDefinitions(graph sst.NamedGraph, log Logger) error {
	const validationName = ValidationName("ValidateNamedGraphForTypeDefinitions")
	err := log.LogForGraph(InfoEnterLevel, (graph), validationName)
	if err != nil {
		return err
	}
	err = graph.ForIRINodes(func(ibS sst.IBNode) error {
		var isClass bool
		var isDatatypeProp bool
		var isObjectProperty bool
		var isIndividual bool
		var out []string
		err := ibS.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s == ibS { // not inverse
				switch o.TermKind() {
				case sst.TermKindIBNode, sst.TermKindTermCollection:
					o := o.(sst.IBNode)
					if p.Is(rdf.Type) {
						isIndividual = true
						if o.Is(owl.DatatypeProperty) {
							isDatatypeProp = true
						}
						if o.Is(owl.ObjectProperty) {
							isObjectProperty = true
						}
						if o.Is(owl.Class) {
							isClass = true
						}
					}
					if p.Is(rdfs.SubClassOf) {
						isIndividual = true
					}
					if p.Is(rdfs.SubPropertyOf) {
						isObjectProperty = true
					}
				case sst.TermKindLiteral, sst.TermKindLiteralCollection:
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if isIndividual {
			out = append(out, "individual")
		}
		if isClass {
			out = append(out, "class")
		}
		if isObjectProperty {
			out = append(out, "objectProperty")
		}
		if isDatatypeProp {
			out = append(out, "class")
		}
		return log.Log(InfoLevel, ibS, strings.Join(out, " "))
	})
	if err != nil {
		return err
	}
	return log.LogForGraph(InfoLeaveLevel, (graph), validationName)
}

func ExperimentalNamedGraphForAcyclic(graph sst.NamedGraph, log Logger) error {
	const validationName = ValidationName("ValidateNamedGraphForAcyclic")
	err := log.LogForGraph(InfoEnterLevel, (graph), validationName)
	if err != nil {
		return err
	}
	err = graph.ForIRINodes(func(ibS sst.IBNode) error {
		return ibS.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s == ibS { // not inverse
				switch o.TermKind() {
				case sst.TermKindIBNode, sst.TermKindTermCollection:
					err := log.Logf(InfoLevel, ibS, "%s %s", p.IRI(), o.(sst.IBNode).IRI())
					if err != nil {
						return err
					}
				case sst.TermKindLiteral:
					err := log.Logf(InfoLevel, ibS, "%s %q^^%s", p.IRI(), o.(sst.Literal), o.(sst.Literal).DataType().IRI())
					if err != nil {
						return err
					}
				case sst.TermKindLiteralCollection:
					err := log.Logf(InfoLevel, ibS, "%s %q^^%s\n", p.IRI(), o.(sst.LiteralCollection).Values(), o.(sst.LiteralCollection).Member(0).DataType().IRI())
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	return log.LogForGraph(InfoLeaveLevel, (graph), validationName)
}

func wave(done map[sst.IBNode]int, ib sst.IBNode, iGraph int) (int, error) {
	var count int
	_, found := done[ib]
	if !found { // not done yet?
		done[ib] = iGraph // mark done
		count++
		err := ib.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if ib.OwningGraph() == p.OwningGraph() {
				c, err := wave(done, p, iGraph)
				if err != nil {
					return err
				}
				count += c
			}
			if s == ib { // not inverse
				switch o.TermKind() {
				case sst.TermKindIBNode, sst.TermKindTermCollection:
					if ib.OwningGraph() == o.(sst.IBNode).OwningGraph() {
						c, err := wave(done, o.(sst.IBNode), iGraph)
						if err != nil {
							return err
						}
						count += c
					}
				case sst.TermKindLiteral, sst.TermKindLiteralCollection:
				}
			} else if ib.OwningGraph() == s.OwningGraph() {
				c, err := wave(done, s, iGraph)
				if err != nil {
					return err
				}
				count += c
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	return count, nil
}

func ExperimentalNamedGraphForConnectedGraph(graph sst.NamedGraph, log Logger) error {
	const validationName = ValidationName("ValidateNamedGraphForConnectedGraph")
	err := log.LogForGraph(InfoEnterLevel, (graph), validationName)
	if err != nil {
		return err
	}
	var iGraph int
	done := make(map[sst.IBNode]int)
	err = graph.ForIRINodes(func(ibS sst.IBNode) error {
		_, found := done[ibS]
		if !found {
			iGraph++
			count, err := wave(done, ibS, iGraph)
			if err != nil {
				return err
			}
			err = log.Logf(InfoLevel, ibS, "%d ", count)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = log.LogfForGraph(InfoLevel, (graph), "No of connected graphs = %d ", iGraph)
	if err != nil {
		return err
	}
	err = graph.ForIRINodes(func(ibS sst.IBNode) error {
		i, found := done[ibS]
		if !found {
			i = 0
		}
		err := log.Logf(InfoLevel, ibS, "%d", i)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return log.LogForGraph(InfoLeaveLevel, (graph), validationName)
}
