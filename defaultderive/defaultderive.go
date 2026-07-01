// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// Package defaultderive contains the **default index mapping** to extract information from SST NamedGraphs for the Bleve index
//
// Defaultderive itself does not do any bleve operation.
// Defaultderive is used by either package remoterepository for a remote SST Repository or by package sst for a locally
// realized SST Repository(LocalFullRepository, LocalBasicRepository).
// There might be other application specific derive packages.
package defaultderive

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2"
	phrase "github.com/semanticstep/sst-core/defaultderive/analyzerphrase"
	separatedkeyword "github.com/semanticstep/sst-core/defaultderive/analyzerseparatedkeyword"
	"github.com/semanticstep/sst-core/singleton"
	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"

	// auto import will import wrong keyword package
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"go.uber.org/zap"
)

const (
	defaultDocumentType                = "-"
	partDocumentType                   = "Part"
	partSpecificationDocumentType      = "PartSpecification"
	genericPartDocumentType            = "GenericPart"
	organizationDocumentType           = "Organization"
	representationContextDocumentType  = "RepresentationContext"
	shapeFeatureDefinitionDocumentType = "ShapeFeatureDefinition"
	classificationSystemType           = "ClassificationSystem"
	classSystemType                    = "ClassSystem"
	personType                         = "Person"
	productClassType                   = "ProductClass"
	materializedPhysicalObjectType     = "MaterializedPhysicalObject"
	computerFileType                   = "ComputerFile"
	productFamilyType                  = "ProductFamily"
	breakdownSystemType                = "BreakdownSystem"
	documentsType                      = "Document"
	typeField                          = "mainType"
	additionalTypeField                = "additionalType"
	specificTypeField                  = "specificType"
	graphIDField                       = "graphID"
	graphURIField                      = "graphURI"
	directImportField                  = "directImport"
	nodeCountField                     = "nodeCount"
	mainNodeField                      = "mainNode"
	literalField                       = "literal"
	labelField                         = "label"
	phraseFieldSuffix                  = "phrase"
	labelPhraseField                   = labelField + "." + phraseFieldSuffix
	commentField                       = "comment"
	idPrefix                           = "id."
	idIDField                          = "id"
	idSUBIDField                       = "subid"
	keyFieldSuffix                     = "key"
	sortFieldSuffix                    = "sort"
	idTypeField                        = "type"
	idOwnerField                       = "idOwner"
	idOwnerKeyField                    = idOwnerField + "." + keyFieldSuffix
	idOwnerLabelField                  = idOwnerField + "." + labelField
	idOwnerLabelPhraseField            = idOwnerField + "." + labelField + "." + phraseFieldSuffix
	idOwnerCommentField                = idOwnerField + "." + commentField
	uriFieldSuffix                     = "uri"
	idOwnerURIField                    = idOwnerField + "." + uriFieldSuffix
	graphFieldSuffix                   = "graph"
	idOwnerURIGraphField               = idOwnerURIField + "." + graphFieldSuffix
	fragmentFieldSuffix                = "fragment"
	idOwnerURIFragmentField            = idOwnerURIField + "." + fragmentFieldSuffix
	partFieldPrefix                    = "part"
	partCategoryFieldSuffix            = "partCategory"
	shapeFeatureTypeFieldSuffix        = "shapeFeatureType"
	shapeFeatureSubClass               = "shapeFeatureSubClass"
	commitHash                         = "commitHash"
	commitAuthor                       = "commitAuthor"
	commitTime                         = "commitTime"
	commitMessage                      = "commitMessage"
	partCategoryField                  = partFieldPrefix + "." + partCategoryFieldSuffix
	definedByField                     = "definedBy"
	definedByPrefix                    = "definedBy."

	deleteSearchSize           = 1024
	maxBatchDocSize            = 16 * 1024 * 1024
	indexBlevePath             = "index.bleve"
	gitIgnoreFileName          = ".gitignore"
	sstIndexVersionFieldPrefix = "version.sst."
	sstIndexVersion            = "202604231430"
	ssoIndexVersionFieldPrefix = "version.sso."
	ssoIndexVersion            = "691e05692807"
)

var errBreakForAll = errors.New("break for all")

var part_category_ignored = map[string]interface{}{
	"in_process":          nil,
	"regulated":           nil,
	"safety":              nil,
	"service":             nil,
	"application_control": nil,
	"assembly_study":      nil,
	"digital_mock-up":     nil,
	"electrical_design":   nil,
	"mechanical_design":   nil,
	"preliminary_design":  nil,
	"process_planning":    nil,
	"product_support":     nil,
	"not_specific":        nil,
}

func DeriveInfo() *sst.SSTDeriveInfo {
	return &sst.SSTDeriveInfo{
		DerivePackageName:    "defaultderive",
		DerivePackageVersion: sstIndexVersion,
		DefaultIndexMapping:  defaultIndexMapping,
		DeriveDocuments:      deriveIndexFromNamedGraph,
	}
}

func documentTypes() []string {
	return []string{
		genericPartDocumentType,
		organizationDocumentType,
		representationContextDocumentType,
		shapeFeatureDefinitionDocumentType,
		classificationSystemType,
		classSystemType,
		personType,
		productClassType,
		materializedPhysicalObjectType,
		computerFileType,
		breakdownSystemType,
		productFamilyType,
		documentsType,
	}
}

func defaultIndexMapping() *mapping.IndexMappingImpl {
	im := bleve.NewIndexMapping()
	im.TypeField = typeField
	im.DocValuesDynamic = false
	addDerivedDocumentMapping(im, defaultDocumentType, nil)
	for _, dt := range documentTypes() {
		addDerivedDocumentMapping(im, dt, func(
			documentMapping,
			phraseIndexDM, keywordDM, sKeywordIndexDM, keywordStoreDM, _, indexOnlyDM *mapping.DocumentMapping,
			keywordFM *mapping.FieldMapping,
		) {
			definedByMapping := bleve.NewDocumentMapping()
			for _, dm := range [...]*mapping.DocumentMapping{documentMapping, definedByMapping} {
				dm.AddSubDocumentMapping(additionalTypeField, indexOnlyDM)
				dm.AddSubDocumentMapping(mainNodeField, keywordDM)
				addLabelSubDocumentMapping(dm, phraseIndexDM)
				idMapping := bleve.NewDocumentMapping()
				idMapping.AddSubDocumentMapping(keyFieldSuffix, sKeywordIndexDM)
				idMapping.AddSubDocumentMapping(phraseFieldSuffix, phraseIndexDM)
				idMapping.AddSubDocumentMapping(idTypeField, keywordDM)
				idMapping.AddSubDocumentMapping(sortFieldSuffix, keywordDM)
				idOwnerMapping := bleve.NewDocumentMapping()
				idOwnerMapping.AddFieldMapping(newUnSortableTextFieldMapping())
				idOwnerMapping.AddSubDocumentMapping(keyFieldSuffix, sKeywordIndexDM)
				addLabelSubDocumentMapping(idOwnerMapping, phraseIndexDM)
				idOwnerURIMapping := bleve.NewDocumentMapping()
				idOwnerURIMapping.AddFieldMapping(keywordFM)
				idOwnerURIMapping.AddSubDocumentMapping(graphFieldSuffix, keywordStoreDM)
				idOwnerURIMapping.AddSubDocumentMapping(fragmentFieldSuffix, keywordStoreDM)
				idOwnerMapping.AddSubDocumentMapping(uriFieldSuffix, idOwnerURIMapping)
				idMapping.AddSubDocumentMapping(idOwnerField, idOwnerMapping)
				idMapping.AddFieldMapping(newSortableTextFieldMapping())
				dm.AddSubDocumentMapping(idIDField, idMapping)
			}
			documentMapping.AddSubDocumentMapping(definedByField, definedByMapping)
			if dt == genericPartDocumentType {
				partMapping := bleve.NewDocumentMapping()
				partMapping.AddSubDocumentMapping(partCategoryFieldSuffix, keywordDM)
				documentMapping.AddSubDocumentMapping(partFieldPrefix, partMapping)
			}
			if dt == shapeFeatureDefinitionDocumentType {
				documentMapping.AddSubDocumentMapping(shapeFeatureTypeFieldSuffix, keywordDM)
				documentMapping.AddSubDocumentMapping(shapeFeatureSubClass, keywordDM)
			}
			documentMapping.AddSubDocumentMapping(commitHash, keywordDM)
			documentMapping.AddSubDocumentMapping(commitAuthor, keywordDM)
			documentMapping.AddSubDocumentMapping(commitTime, keywordDM)
			documentMapping.AddSubDocumentMapping(commitMessage, keywordDM)
			documentMapping.AddSubDocumentMapping(specificTypeField, keywordDM)
		})
	}
	pseudoMapping := bleve.NewDocumentMapping()
	pseudoMapping.AddFieldMapping(versionField(sstIndexVersionFieldPrefix, sstIndexVersion))
	pseudoMapping.AddFieldMapping(versionField(ssoIndexVersionFieldPrefix, ssoIndexVersion))
	im.AddDocumentMapping("pseudo", pseudoMapping)
	return im
}

func addDerivedDocumentMapping(
	im *mapping.IndexMappingImpl,
	doctype string,
	extraMappings func(
		documentMapping *mapping.DocumentMapping,
		phraseIndexDM, keywordDM, sKeywordIndexDM, keywordStoreDM, numericDM, indexOnlyDM *mapping.DocumentMapping,
		keywordFM *mapping.FieldMapping,
	),
) {
	documentMapping := bleve.NewDocumentMapping()

	pm := bleve.NewTextFieldMapping()
	pm.Analyzer, pm.Index, pm.Store, pm.DocValues = phrase.Name, true, false, true
	phraseIndexDM := bleve.NewDocumentMapping()
	phraseIndexDM.AddFieldMapping(pm)

	keywordFM := bleve.NewTextFieldMapping()
	keywordFM.Analyzer, keywordFM.DocValues = keyword.Name, false
	keywordDM := bleve.NewDocumentMapping()
	keywordDM.AddFieldMapping(keywordFM)

	skim := bleve.NewTextFieldMapping()
	skim.Analyzer, skim.Index, skim.Store, skim.DocValues = separatedkeyword.Name, true, false, false
	sKeywordIndexDM := bleve.NewDocumentMapping()
	sKeywordIndexDM.AddFieldMapping(skim)

	ksm := bleve.NewTextFieldMapping()
	ksm.Analyzer, ksm.Index, ksm.Store, ksm.DocValues = keyword.Name, false, true, false
	keywordStoreDM := bleve.NewDocumentMapping()
	keywordStoreDM.AddFieldMapping(ksm)

	numericDM := bleve.NewDocumentMapping()
	numericDM.AddFieldMapping(bleve.NewNumericFieldMapping())

	iom := bleve.NewTextFieldMapping()
	iom.Store, iom.DocValues = true, false
	indexOnlyDM := bleve.NewDocumentMapping()
	indexOnlyDM.AddFieldMapping(iom)

	documentMapping.AddSubDocumentMapping(typeField, keywordDM)
	documentMapping.AddSubDocumentMapping(directImportField, keywordDM)
	documentMapping.AddSubDocumentMapping(graphIDField, keywordDM)
	documentMapping.AddSubDocumentMapping(graphURIField, keywordDM)
	documentMapping.AddSubDocumentMapping(nodeCountField, numericDM)
	documentMapping.AddSubDocumentMapping(literalField, indexOnlyDM)
	if extraMappings != nil {
		extraMappings(
			documentMapping, phraseIndexDM, keywordDM, sKeywordIndexDM, keywordStoreDM, numericDM, indexOnlyDM, keywordFM,
		)
	}
	im.AddDocumentMapping(doctype, documentMapping)
}

func addLabelSubDocumentMapping(dm, phraseDM *mapping.DocumentMapping) {
	labelMapping := bleve.NewDocumentMapping()
	labelMapping.AddFieldMapping(newUnSortableTextFieldMapping())
	labelMapping.AddSubDocumentMapping(phraseFieldSuffix, phraseDM)
	dm.AddSubDocumentMapping(labelField, labelMapping)
}

func newUnSortableTextFieldMapping() *mapping.FieldMapping {
	fm := bleve.NewTextFieldMapping()
	fm.DocValues = false
	return fm
}

func newSortableTextFieldMapping() *mapping.FieldMapping {
	fm := bleve.NewTextFieldMapping()
	fm.DocValues = true
	fm.Analyzer = keyword.Name
	return fm
}

func versionField(versionFieldPrefix, version string) *mapping.FieldMapping {
	versionField := bleve.NewBooleanFieldMapping()
	versionField.Name = versionFieldPrefix + version
	versionField.DocValues, versionField.Index, versionField.Store = false, false, false
	return versionField
}

// For the returned value:
// id: this is the id of namedgraph, should be handled by core
// document: the index is for the bleve document mapping field, the value is for the bleve document mapping value
// preConditionQueries: used for idowner, but purpose is not clear
func deriveIndexFromNamedGraph(r sst.Repository, g sst.NamedGraph) (gID string,
	graphDocument map[string]interface{}, preConditionQueries []query.Query, err error) {
	var eImportIDs []string

	// top is special varibale used for recursion, real purpose is not clear, now all rest usages use it as true
	top := true
	if top {
		gImports := g.DirectImports()
		eImportIDs = make([]string, 0, len(gImports))
		for _, i := range gImports {
			eImportIDs = append(eImportIDs, i.ID().String())
		}
		sort.Strings(eImportIDs)
		gID = g.ID().String()
	}
	literalSet := map[string]struct{}{}
	typeSubdocument := [...]map[string]interface{}{{}, {}}
	var documentType [2]string
	var nodeCnt int
	// cp := sstjson.NewSstToJsonSstConversionParameters()

	documentPartofMap := map[string]string{}
	documentMapInitialized := false

	err = g.ForAllIBNodes(func(d sst.IBNode) error {
		nodeCnt++
		var ok bool
		var recognizedType bool
		var recognizedSubtype bool
		if d.TypeOf() != nil {
			switch d.TypeOf().InVocabulary().(type) {
			case sso.KindGenericPart:
				if documentType[0], ok = overrideDocumentType(documentType[0], genericPartDocumentType); !ok {
					break
				}
				typeSubdocument[0][mainNodeField] = string(d.Fragment())
				recognizedType = true
			case sso.KindBreakdownSystem:
				if documentType[0], ok = overrideDocumentType(documentType[0], breakdownSystemType); !ok {
					break
				}
				typeSubdocument[0][mainNodeField] = string(d.Fragment())
				recognizedType = true
			case lci.KindOrganization:
				if documentType[0], ok = overrideDocumentType(documentType[0], organizationDocumentType); ok {
					typeSubdocument[0][mainNodeField] = string(d.Fragment())
				}
				recognizedType = true
			case rep.KindRepresentationContext:
				if documentType[1], ok = overrideDocumentType(documentType[1], representationContextDocumentType); ok {
					typeSubdocument[1][mainNodeField] = string(d.Fragment())
				}
				recognizedType = true
			case sso.KindShapeFeatureDefinition:
				if documentType[1], ok = overrideDocumentType(documentType[1], shapeFeatureDefinitionDocumentType); !ok {
					break
				}
				typeSubdocument[1][mainNodeField] = string(d.Fragment())
				_, _, o := d.GetTriple(0)
				switch o.TermKind() {
				case sst.TermKindIBNode:
					typeSubdocument[1][shapeFeatureSubClass] = strings.TrimPrefix(o.(sst.IBNode).PrefixedFragment(), "sso:")
				}
				recognizedType = true
			case sso.KindClassificationSystem:
				if documentType[1], ok = overrideDocumentType(documentType[1], classificationSystemType); ok {
					typeSubdocument[1][mainNodeField] = string(d.Fragment())
				}
				recognizedType = true
			case sso.KindClassSystem:
				if documentType[1], ok = overrideDocumentType(documentType[1], classSystemType); ok {
					typeSubdocument[1][mainNodeField] = string(d.Fragment())
				}
				recognizedType = true
			case lci.KindPerson:
				if documentType[1], ok = overrideDocumentType(documentType[1], personType); ok {
					typeSubdocument[1][mainNodeField] = string(d.Fragment())
				}
				recognizedType = true
			case sso.KindProductClass:
				// if documentType[1], ok = overrideDocumentType(documentType[1], productClassType); ok {
				// 	typeSubdocument[1][mainNodeField] = string(d.Fragment())
				// }
				recognizedSubtype = true
			case lci.KindMaterializedPhysicalObject:
				if documentType[1], ok = overrideDocumentType(documentType[1], materializedPhysicalObjectType); ok {
					typeSubdocument[1][mainNodeField] = string(d.Fragment())
				}
				recognizedType = true
			case lci.KindComputerFile:
				if documentType[1], ok = overrideDocumentType(documentType[1], computerFileType); ok {
					typeSubdocument[1][mainNodeField] = string(d.Fragment())
				}
				recognizedType = true
			case sso.KindProductFamily:
				if documentType[1], ok = overrideDocumentType(documentType[1], productFamilyType); ok {
					typeSubdocument[1][mainNodeField] = string(d.Fragment())
				}
				recognizedType = true
			case lci.KindInformationObject:
				if !documentMapInitialized {
					initializeDocumentPartofMap(documentPartofMap, g)
					documentMapInitialized = true
				}

				if _, ok := documentPartofMap[d.Fragment()]; !ok {
					if documentType[1], ok = overrideDocumentType(documentType[1], documentsType); ok {
						typeSubdocument[1][mainNodeField] = string(d.Fragment())
					}
					recognizedType = true
				}
			}
		}
		var ids map[sst.IBNode]map[string]interface{}
		var additionalTypes map[string]struct{}
		// typeId := ""
		// typeLabel := ""
		specificType := ""
		err := d.ForAll(func(index int, s, p sst.IBNode, o sst.Term) error {
			if s != d {
				return nil
			}
			if recognizedType {
				type idMap = map[string]interface{}
				var idmap idMap
				pv := p.InVocabulary()
				// str := cp.FindRootProperty(p, map[string]string{})
				// if str == "sso:id" {
				// 	pv = sso.ID
				// }
				switch pv.(type) {
				case rdf.IsType:
					switch o.TermKind() {
					case sst.TermKindIBNode:
						o := o.(sst.IBNode)
						if o.IsValid() {
							if o.InVocabulary() == nil {
								additionalType := o.IRI().String()
								var err error
								additionalTypes, err = addAdditionalTypes(
									typeSubdocument, additionalTypes, g.Stage(), additionalType,
								)
								if err != nil {
									return err
								}

								// get specific type from singleton
								if specificType == "" {
									specificType = "unknown"
									edmsystemtype := singleton.GetInstance()

									if _, exists := edmsystemtype.TypeMap[o.OwningGraph().Info().Id.String()]; exists {
										classList := edmsystemtype.TypeMap[o.OwningGraph().Info().Id.String()]

										if _, exists := classList[o.Fragment()]; exists {
											class := classList[o.Fragment()]

											if class.Id != "" || class.Label != "" {
												specificType = class.Id

												if class.Label != "" {
													specificType = class.Label
												}
											}
										}
									}

									addDataElementValue(typeSubdocument[0], specificTypeField, specificType)
								}

							} else if documentType[0] == genericPartDocumentType {
								typeName := o.PrefixedFragment()
								category := strings.TrimPrefix(typeName, "sso:PartCategory_")
								if len(category) < len(typeName) {
									if _, exist := part_category_ignored[category]; !exist {
										addDataElementValue(typeSubdocument[0], partCategoryField, category)
									}
								}
							} else if documentType[1] == shapeFeatureDefinitionDocumentType {
								typeName := o.PrefixedFragment()
								category := strings.TrimPrefix(typeName, "sso:ShapeFeatureType_")
								if len(category) < len(typeName) {
									addDataElementValue(typeSubdocument[0], shapeFeatureTypeFieldSuffix, category)
								}
							}
						}
					}

				case rdfs.KindLabel:
					setDataElement(g, typeSubdocument[0], labelField, o)
					setDataElement(g, typeSubdocument[0], labelPhraseField, o)
				case rdfs.KindComment:
					setDataElement(g, typeSubdocument[0], commentField, o)
				case sso.KindID:
					// Dynamic is asked to be deleted
					// pv = sst.UnDynamic(pv)
					idmap, ids = getOrCreateID(ids, p, func() idMap { return idMap{} })
					setDataElement(g, idmap, idIDField, o)
					switch v := idmap[idIDField].(type) {
					case []string:
						for _, v := range v {
							addDataElementValue(idmap, keyFieldSuffix, v)
							addDataElementValue(idmap, phraseFieldSuffix, v)
						}
					case string:
						addDataElementValue(idmap, keyFieldSuffix, v)
						addDataElementValue(idmap, phraseFieldSuffix, v)
					}
					var idType string
					if _, ok := pv.(sso.IsID); !ok {
						ide := pv.VocabularyElement()
						if prefix, found := sst.NamespaceToPrefix(ide.Vocabulary.BaseIRI); found {
							idType = prefix + ":" + ide.Name
						} else {
							idType = "<" + ide.Vocabulary.BaseIRI + "#" + ide.Name + ">"
						}
					}
					addDataElementValue(idmap, idTypeField, idType)
					var idOwner sst.IBNode
					err := p.ForAll(func(index int, si, pi sst.IBNode, oi sst.Term) error {
						if p == si && pi.Is(sso.IDOwner) && oi.TermKind() == sst.TermKindIBNode {
							idOwner = oi.(sst.IBNode)
							return errBreakForAll
						}
						return nil
					})
					if err != nil && !errors.Is(err, errBreakForAll) {
						return err
					}
					if idOwner != nil {
						setDataElement(g, idmap, idOwnerURIField, idOwner)
						addDataElementValue(idmap, idOwnerURIGraphField, idOwner.OwningGraph().IRI().String())
						addDataElementValue(idmap, idOwnerURIFragmentField, string(idOwner.Fragment()))
						err := addIDOwnerValues(g, idmap, idOwner)
						if err != nil {
							return err
						}
					}
					// if idOwner.IsValid() {
					// 	g.setDataElement(id, idOwnerURIField, idOwner)
					// 	addDataElementValue(id, idOwnerURIGraphField, idOwner.OwningGraph().IRI().String())
					// 	addDataElementValue(id, idOwnerURIFragmentField, string(idOwner.Fragment()))
					// 	err := g.addIDOwnerValues(id, idOwner)
					// 	if err != nil {
					// 		return err
					// 	}
					// }
					for _, k := range []string{
						idOwnerField, idOwnerKeyField, idOwnerLabelField, idOwnerLabelPhraseField,
						idOwnerCommentField, idOwnerURIField, idOwnerURIGraphField, idOwnerURIFragmentField,
					} {
						if _, found := idmap[k]; !found {
							idmap[k] = ""
						}
					}
				case lci.IsIsDefinedBy:
					switch o.TermKind() {
					case sst.TermKindIBNode:
						o := o.(sst.IBNode)
						if o.IsValid() {
							//
							// g := o.OwningGraph()
							var definedBy map[string]interface{}
							// This recursion is not something that needs to be directly be supported by the official API methods/functions.
							// err := deriveDocuments(g,
							// 	func(_ string, doc map[string]interface{}, _ []query.Query) error {
							// 		definedBy = doc
							// 		return nil
							// 	}, false,
							// )
							// if err != nil {
							// 	return err
							// }
							for k, v := range definedBy {
								typeSubdocument[0][definedByPrefix+k] = v
							}
						}
					}
				default:
					// only punning node will arrive here ??
					if p.IsValid() {
						if p.InVocabulary() == nil {
							sst.GlobalLogger.Debug("punning node",
								zap.String("base", p.OwningGraph().IRI().String()),
								zap.String("fragment", string(p.Fragment())),
							)

							// loading punning node
							ds, err := r.Dataset(context.TODO(), p.OwningGraph().IRI())
							if err != nil {
								sst.GlobalLogger.Debug("loading punning node failed",
									zap.String("graphID", gID),
									zap.String("iri", p.OwningGraph().IRI().String()),
									zap.Error(err),
								)
								return nil
							}

							stage, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
							if err != nil {
								sst.GlobalLogger.Debug("loading punning node stage failed",
									zap.String("iri", p.OwningGraph().IRI().String()),
									zap.Error(err),
								)
								return nil
							}

							ng := stage.NamedGraph(p.OwningGraph().IRI())
							node := ng.GetIRINodeByFragment(p.Fragment())
							if node == nil {
								sst.GlobalLogger.Debug("punning node fragment does not exist",
									zap.String("iri", p.OwningGraph().IRI().String()),
									zap.String("fragment", string(p.Fragment())),
								)
								return nil
							}

							// try to check this node is property or not, but does not work

							// if node.TypeOf() != nil {
							// 	switch node.TypeOf().InVocabulary().(type) {
							// 	case owl.IsObjectProperty:
							// 		fmt.Println("is a property")
							// 	}
							// }

							err = node.ForAll(func(index int, s, p2 sst.IBNode, o2 sst.Term) error {
								if s != node {
									return nil
								}

								pv := p2.InVocabulary()
								switch pv.(type) {
								case rdf.IsType:
									// maybe can check isproperty here
								case sso.KindIDOwner:
									// will o2 be string directly
									// here we treat o2 as a IBNode
									if o2.(sst.IBNode).IsValid() {
										if o2.(sst.IBNode).InVocabulary() == nil {
											sst.GlobalLogger.Debug("idOwner node",
												zap.String("base", o2.(sst.IBNode).OwningGraph().IRI().String()),
												zap.String("fragment", string(o2.(sst.IBNode).Fragment())),
											)

											// is possible atBase2 != atBase ?
											// here we just consider atBase2 == atBase
											if o2.(sst.IBNode).OwningGraph().IRI().String() == p.OwningGraph().IRI().String() {
												node2 := ng.GetIRINodeByFragment(o2.(sst.IBNode).Fragment())
												if node2 == nil {
													sst.GlobalLogger.Debug("idOwner fragment node does not exist",
														zap.String("iri", p.OwningGraph().IRI().String()),
														zap.String("fragment", string(o2.(sst.IBNode).Fragment())),
													)
													return nil
												}

												// does recursion needed here ?
												err = node2.ForAll(func(index int, s, p sst.IBNode, o sst.Term) error {
													if s != node2 {
														return nil
													}

													pv := p.InVocabulary()
													switch pv.(type) {
													case sso.KindID:
														// we just extract id here
														sst.GlobalLogger.Debug("idOwner literal", zap.String("value", fmt.Sprintf("%v", o.(sst.Literal))))
														setDataElement(g, typeSubdocument[0], idOwnerField, o)
													}

													return nil
												})
											}
										}
									}
								case rdfs.IsSubPropertyOf:
									// is possible o2 be self defined node which type is sso:id
									if o2.(sst.IBNode).Is(sso.ID) {
										sst.GlobalLogger.Debug("id literal", zap.String("value", fmt.Sprintf("%v", o.(sst.Literal))))
										setDataElement(g, typeSubdocument[0], idIDField, o)
									}
								}

								return nil
							})

						}
					}

				}
			}

			if recognizedSubtype {
				type idMap = map[string]interface{}
				var idmap idMap = idMap{}
				pv := p.InVocabulary()
				switch pv.(type) {
				case sso.KindID:
					setDataElement(g, idmap, idIDField, o)
					switch v := idmap[idIDField].(type) {
					case []string:
						for _, v := range v {
							addDataElementValue(typeSubdocument[0], idSUBIDField, v)
						}
					case string:
						addDataElementValue(typeSubdocument[0], idSUBIDField, v)
					}
				}
			}

			addLiteralValues(literalSet, o)
			return nil
		})
		if err != nil {
			return err
		}
		if recognizedType {
			sortedIdentifications := sortIDMap(ids)
			identificationInclusions := map[string]struct{}{}
			for _, pn := range sortedIdentifications {
				for k, v := range pn {
					switch v := v.(type) {
					case []string:
						for _, vv := range v {
							if vv != "" {
								identificationInclusions[k] = struct{}{}
								break
							}
						}
					case string:
						if v != "" {
							identificationInclusions[k] = struct{}{}
						}
					}
				}
			}
			for _, pn := range sortedIdentifications {
				for k, v := range pn {
					if _, include := identificationInclusions[k]; include {
						switch v := v.(type) {
						case []string:
							dataElementKey := k
							if dataElementKey != idIDField {
								dataElementKey = idPrefix + k
							}
							for _, vv := range v {
								addDataElementValue(typeSubdocument[0], dataElementKey, vv)
							}
						case string:
							dataElementKey := k
							if dataElementKey != idIDField {
								dataElementKey = idPrefix + k
							}
							addDataElementValue(typeSubdocument[0], dataElementKey, v)
						}
					}
				}
			}
		}
		return nil
	})
	if documentType[0] == "" {
		documentType[0] = documentType[1]
		for k, v := range typeSubdocument[1] {
			if _, found := typeSubdocument[0][k]; !found {
				typeSubdocument[0][k] = v
			}
		}
	}
	if documentType[0] == "" {
		documentType[0] = defaultDocumentType
	}

	if top {
		graphDocument = map[string]interface{}{
			typeField:         documentType[0],
			graphIDField:      gID,
			graphURIField:     g.IRI().String(),
			directImportField: eImportIDs,
			nodeCountField:    nodeCnt,
			literalField:      sortStringSet(literalSet),
			// partCategoryField: "",
		}
	} else {
		graphDocument = map[string]interface{}{
			typeField: documentType[0],
		}
	}

	if documentType[0] != defaultDocumentType {
		for k, v := range typeSubdocument[0] {
			graphDocument[k] = v
		}
		var idOwner string
		var idOwners []string
		switch v := typeSubdocument[0][idPrefix+idOwnerField].(type) {
		case []string:
			if len(v) > 0 {
				idOwner = v[0]
			}
			idOwners = v
		case string:
			idOwner = v
			idOwners = []string{v}
		}
		switch v := typeSubdocument[0][idIDField].(type) {
		case []string:
			if len(v) <= len(idOwners) {
				for i, v := range v {
					preConditionQueries = appendIDPreCondition(preConditionQueries, documentType[0], idOwners[i], v)
				}
			} else {
				for i, idowner := range idOwners {
					preConditionQueries = appendIDPreCondition(preConditionQueries, documentType[0], idowner, v[i])
				}
				for i := len(idOwners); i < len(v); i++ {
					preConditionQueries = appendIDPreCondition(preConditionQueries, documentType[0], "", v[i])
				}
			}
		case string:
			preConditionQueries = appendIDPreCondition(preConditionQueries, documentType[0], idOwner, v)
		}

		if graphDocument[idIDField] != nil {
			re := regexp.MustCompile(`\d+`)

			var sortFieldValue string

			switch v := graphDocument[idIDField].(type) {
			case []string:
				if len(v) > 0 {
					sortFieldValue = re.ReplaceAllStringFunc(v[0], func(match string) string {
						return fmt.Sprintf("%010s", match)
					})
				}
			case string:
				sortFieldValue = re.ReplaceAllStringFunc(v, func(match string) string {
					return fmt.Sprintf("%010s", match)
				})
			}

			graphDocument[idIDField+"."+sortFieldSuffix] = sortFieldValue
		}
	}
	return gID, graphDocument, preConditionQueries, err
}

func initializeDocumentPartofMap(documentPartofMap map[string]string, g sst.NamedGraph) {
	err := g.ForAllIBNodes(func(d sst.IBNode) error {
		isDocumentNode := false

		if d.TypeOf() != nil {
			switch d.TypeOf().InVocabulary().(type) {
			case lci.KindInformationObject:
				isDocumentNode = true
			}
		}

		if isDocumentNode {
			err := d.ForAll(func(index int, s, p sst.IBNode, o sst.Term) error {
				if s != d {
					return nil
				}

				pv := p.InVocabulary()
				switch pv.(type) {
				case lci.IsHasPart:
					switch o.TermKind() {
					case sst.TermKindIBNode:
						o := o.(sst.IBNode)
						documentPartofMap[o.Fragment()] = s.Fragment()
					}
				}

				return nil
			})

			return err
		}

		return nil
	})

	if err != nil {
		sst.GlobalLogger.Warn("initialize document map failed", zap.Error(err))
	}
}

func overrideDocumentType(documentType, override string) (string, bool) {
	if documentType != "" {
		return defaultDocumentType, false
	}
	return override, true
}

func appendIDPreCondition(preConditionQueries []query.Query, documentType, owner, id string) []query.Query {
	q1 := bleve.NewTermQuery(documentType)
	q1.SetField(typeField)
	q2 := bleve.NewMatchQuery(id)
	q2.SetField(idPrefix + keyFieldSuffix)
	if owner != "" {
		q3 := bleve.NewMatchQuery(owner)
		q3.SetField(idPrefix + idOwnerKeyField)
		return append(preConditionQueries, bleve.NewConjunctionQuery(q1, q2, q3))
	}
	return append(preConditionQueries, bleve.NewConjunctionQuery(q1, q2))
}

func sortStringSet(set map[string]struct{}) []string {
	var sorted []string
	for m := range set {
		sorted = append(sorted, m)
	}
	sort.Strings(sorted)
	return sorted
}

func addAdditionalTypes(
	typeSubdocument [2]map[string]interface{},
	additionalTypes map[string]struct{},
	gs sst.Stage,
	additionalType string,
) (map[string]struct{}, error) {
	if len(additionalTypes) == 0 {
		additionalTypes = map[string]struct{}{}
	}
	if _, found := additionalTypes[additionalType]; !found {
		additionalTypes[additionalType] = struct{}{}
		addDataElementValue(typeSubdocument[0], additionalTypeField, additionalType)
		atBase, atFragmentString := Split(additionalType)
		// es, err := sst.NewStageWithIRI(must.OK1(sst.NewIRI(atBase)))
		// if err != nil {
		// 	return additionalTypes, err
		// }
		// atg := gs.NamedGraph(es.IRI())
		es := sst.OpenStage(sst.DefaultTriplexMode)
		atg := es.CreateNamedGraph(sst.IRI(atBase))
		var err error
		err = nil
		if err != nil {
			return additionalTypes, err
		}
		atFragment := atFragmentString
		if err == nil {
			return addAdditionalTypesFromIRINode(typeSubdocument, additionalTypes, gs, atg, atFragment)
		}
		if !errors.Is(err, sst.ErrNamedGraphNotFound) {
			return additionalTypes, err
		}
		// if gs, ok := gs.(interface{ Dataset() sst.Dataset }); ok {
		atDataset, err := gs.Repository().Dataset(context.TODO(), sst.IRI(atg.ID().URN()))
		if err == nil {
			atDatasetStage, err := atDataset.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
			if err != nil {
				return additionalTypes, err
			}
			// atg := atDatasetStage.DefaultGraph()
			atg := atDatasetStage.NamedGraph(atg.IRI())
			if err != nil {
				return additionalTypes, err
			}
			return addAdditionalTypesFromIRINode(typeSubdocument, additionalTypes, atDatasetStage, atg, atFragment)
		}
		if errors.Is(err, sst.ErrDatasetNotFound) {
			err = nil
		}
		return additionalTypes, err
		// }
	}
	return additionalTypes, nil
}

func addIdOwner(
	typeSubdocument [2]map[string]interface{},
	gs sst.Stage,
	idOwnerType string,
) error {
	atBase, atFragmentString := Split(idOwnerType)
	es := sst.OpenStage(sst.DefaultTriplexMode)
	ng := es.CreateNamedGraph(sst.IRI(atBase))
	atFragment := atFragmentString
	atDataset, err := gs.Repository().Dataset(context.TODO(), sst.IRI(ng.ID().URN()))
	if err == nil {
		atDatasetStage, err := atDataset.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			return err
		}
		// atg := atDatasetStage.DefaultGraph()
		atg := atDatasetStage.NamedGraph(ng.IRI())

		t := atg.GetIRINodeByFragment(atFragment)
		if t != nil {
			err := t.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
				if s == t {
					if _, ok := p.InVocabulary().(sso.IsIDOwner); ok && o.TermKind() == sst.TermKindIBNode {
						o := o.(sst.IBNode)
						_, atFragmentStringSub := Split(o.IRI().String())
						tSub := atg.GetIRINodeByFragment(atFragmentStringSub)
						if tSub != nil {
							err = tSub.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
								if s == tSub {
									if _, ok := p.InVocabulary().(rdfs.KindLabel); ok && o.TermKind() == sst.TermKindLiteral {
										addDataElementValue(typeSubdocument[0], idOwnerField, string(o.(sst.String)))
									}
								}
								return nil
							})
						}
					}
				}
				return nil
			})
			return err
		}
	}
	if errors.Is(err, sst.ErrDatasetNotFound) {
		err = nil
	}
	return err
}

func addDataElementValue(dataHolder map[string]interface{}, key string, val string) {
	if prevVal, found := dataHolder[key]; found {
		switch v := prevVal.(type) {
		case []string:
			dataHolder[key] = append(v, val)
		case string:
			dataHolder[key] = []string{v, val}
		}
		return
	}
	dataHolder[key] = val
}

func setDataElement(g sst.NamedGraph, dataHolder map[string]interface{}, key string, o sst.Term) {
	switch o.TermKind() {
	case sst.TermKindIBNode:
		o := o.(sst.IBNode)
		var ibID string
		if o.OwningGraph() == g {
			ibID = ":" + string(o.Fragment())
			if ibID == ":" {
				return
			}
		} else {
			ibID = o.PrefixedFragment()
		}
		addDataElementValue(dataHolder, key, ibID)
	case sst.TermKindLiteral:
		o := o.(sst.Literal)
		addDataElementValue(dataHolder, key, fmt.Sprintf("%v", o))
	case sst.TermKindLiteralCollection:
		o := o.(sst.LiteralCollection)
		o.ForMembers(func(_ int, l sst.Literal) {
			addDataElementValue(dataHolder, key, fmt.Sprintf("%v", l))
		})
	}
}

func getOrCreateID(
	ids map[sst.IBNode]map[string]interface{},
	idRef sst.IBNode,
	newID func() map[string]interface{},
) (map[string]interface{}, map[sst.IBNode]map[string]interface{}) {
	if ids == nil {
		ids = map[sst.IBNode]map[string]interface{}{}
	}
	id := ids[idRef]
	if id == nil {
		id = newID()
		ids[idRef] = id
	}
	return id, ids
}

func addIDOwnerValues(g sst.NamedGraph, id map[string]interface{}, idOwner sst.IBNode) error {
	return idOwner.ForAll(func(index int, s, p sst.IBNode, o sst.Term) error {
		if s != idOwner {
			return nil
		}
		switch p.InVocabulary().(type) {
		case rdfs.KindLabel:
			setDataElement(g, id, idOwnerLabelField, o)
			setDataElement(g, id, idOwnerLabelPhraseField, o)
		case rdfs.KindComment:
			setDataElement(g, id, idOwnerCommentField, o)
		case sso.KindID:
			setDataElement(g, id, idOwnerField, o)
			setDataElement(g, id, idOwnerKeyField, o)
		default:
			if p.TypeOf() != nil {
				if _, ok := p.TypeOf().InVocabulary().(sso.KindID); ok {
					setDataElement(g, id, idOwnerField, o)
					setDataElement(g, id, idOwnerKeyField, o)
				}
			}
		}
		return nil
	})
}

func addLiteralValues(literalSet map[string]struct{}, o sst.Term) {
	switch o.TermKind() {
	case sst.TermKindIBNode:
	case sst.TermKindLiteral:
		o := o.(sst.Literal)
		lv := fmt.Sprintf("%v", o)
		if len(lv) > 0 {
			literalSet[lv] = struct{}{}
		}
	case sst.TermKindLiteralCollection:
		o := o.(sst.LiteralCollection)
		o.ForMembers(func(_ int, l sst.Literal) {
			lv := fmt.Sprintf("%v", l)
			if len(lv) > 0 {
				literalSet[lv] = struct{}{}
			}
		})
	}
}

func sortIDMap(partIDs map[sst.IBNode]map[string]interface{}) []map[string]interface{} {
	var sortedPartIDs []map[string]interface{}
	for _, pn := range partIDs {
		sortedPartIDs = append(sortedPartIDs, pn)
	}
	sort.Slice(sortedPartIDs, func(i, j int) bool {
		pni, pnj := sortedPartIDs[i], sortedPartIDs[j]
		vi, vj := pni[idIDField], pnj[idIDField]
		idi, oki := vi.(string)
		idj, okj := vj.(string)
		return oki && ((okj && idi < idj) || !okj)
	})
	return sortedPartIDs
}

func Split(u string) (prefix, suffix string) {
	i := len(u)
	for i > 0 {
		r, w := utf8.DecodeLastRuneInString(u[0:i])
		if r == '/' || r == '#' {
			prefix, suffix = u[0:i], u[i:len(u)]
			break
		}
		i -= w
	}
	return prefix, suffix
}

func addAdditionalTypesFromIRINode(
	typeSubdocument [2]map[string]interface{},
	additionalTypes map[string]struct{},
	gs sst.Stage,
	atg sst.NamedGraph,
	atFragment string,
) (map[string]struct{}, error) {
	t := atg.GetIRINodeByFragment(atFragment)
	if t != nil {
		err := t.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s == t {
				//seems like a recursive invoke but the ttl we are dealing with is not that complex
				if _, ok := p.InVocabulary().(rdfs.IsSubClassOf); ok && o.TermKind() == sst.TermKindIBNode {
					o := o.(sst.IBNode)
					var err error
					additionalTypes, err = addAdditionalTypes(
						typeSubdocument, additionalTypes, gs, o.IRI().String(),
					)
					if err != nil {
						return err
					}
				}

				if _, ok := p.InVocabulary().(sso.KindID); ok && o.TermKind() == sst.TermKindLiteral {
					addDataElementValue(typeSubdocument[0], additionalTypeField, string(o.(sst.String)))
				}
			}
			return nil
		})
		return additionalTypes, err
	}

	return additionalTypes, nil
}
