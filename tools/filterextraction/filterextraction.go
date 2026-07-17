// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// Package filterextraction split a bigger NamedGraph to smaller ones
// according to the needs of SST Ontologies and to support SST Repositories.
package filterextraction

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict" // register vocabulary map
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/google/uuid"
	"go.uber.org/zap"
)


// returns a map
// key:    predicate
// value:  subject collection of this predicate
func CollectNodeAsPredicateUsages(graph sst.NamedGraph) (map[sst.IBNode]map[sst.IBNode]struct{}, error) {
	usages := map[sst.IBNode]map[sst.IBNode]struct{}{}
	err := graph.ForAllIBNodes(func(d sst.IBNode) error {
		return d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s == d && p.OwningGraph() == graph {
				u, found := usages[p]
				if !found {
					u = map[sst.IBNode]struct{}{}
					usages[p] = u
				}
				u[d] = struct{}{}
			}
			return nil
		})
	})
	return usages, err
}

type nodeGroup struct {
	rootNodeType  sst.ElementInformer
	group         map[sst.IBNode]struct{}
	outsideNodes  map[sst.IBNode]struct{}
	ID            uuid.UUID
	importedGraph sst.NamedGraph
}

func listMap(group nodeGroup) {
	sst.GlobalLogger.Debug("filterextract node list", zap.String("groupID", group.ID.String()))
	for ib := range group.group {
		if ib.IsBlankNode() {
			sst.GlobalLogger.Debug("filterextract group node", zap.String("nodeID", ib.ID().String()), zap.String("groupID", group.ID.String()))
		} else {
			sst.GlobalLogger.Debug("filterextract group node", zap.String("fragment", ib.Fragment()), zap.String("groupID", group.ID.String()))
		}
	}
	for ib := range group.outsideNodes {
		sst.GlobalLogger.Debug("filterextract outside ref", zap.String("fragment", ib.Fragment()), zap.String("groupID", group.ID.String()))
	}
}

func findVocabularyTopPredicate(p sst.IBNode) (sst.ElementInformer, error) {
	var superProperty sst.IBNode
	err := p.ForAll(func(_ int, ts, tp sst.IBNode, to sst.Term) error {
		if p == ts && tp.Is(rdfs.SubPropertyOf) && sst.IsKindIBNode(to) {
			superProperty = to.(sst.IBNode)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if superProperty != nil {
		return findVocabularyTopPredicate(superProperty)
	}
	dt := p.InVocabulary()
	return findVocabularyTopProperty(dt), nil
}

func findVocabularyTopProperty(dt sst.ElementInformer) sst.ElementInformer {
	// sst.GlobalLogger.Debug(fmt.Sprintf(" %s", dt.Element().GoSimpleName() ))
	if dt.IsObjectProperty() {
		// sst.GlobalLogger.Debug(fmt.Sprintf("."))
		dtSuper := dt.SubPropertyOf()
		if dtSuper != nil {
			return findVocabularyTopProperty(dtSuper)
		}
	}
	return dt
}

// process part-whole relationships, see Mereology
func loopNodes(graph sst.NamedGraph, ib sst.IBNode, group nodeGroup) error {
	// sst.GlobalLogger.Debug(fmt.Sprintf("LoopNode =: %s\n", ib.Fragment()))
	group.group[ib] = struct{}{}
	if ib.IsBlankNode() {
		sst.GlobalLogger.Debug("filterextract add node to group", zap.String("nodeID", ib.ID().String()), zap.String("groupID", group.ID.String()))
	} else {
		sst.GlobalLogger.Debug("filterextract add node to group", zap.String("fragment", ib.Fragment()), zap.String("groupID", group.ID.String()))
	}
	err := ib.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		// sst.GlobalLogger.Debug(fmt.Sprintf("FindTop:"))
		dt, err := findVocabularyTopPredicate(p)
		if err != nil {
			return err
		}
		// sst.GlobalLogger.Debug(fmt.Sprintf(" = %s\n", dt.VocabularyElement().GoSimpleName()))
		switch dt.(type) {
		case lci.IsPartOf:
			if o == ib {
				if p.OwningGraph() == graph {
					group.group[p] = struct{}{}
					sst.GlobalLogger.Debug("filterextract add node to group", zap.String("fragment", ib.Fragment()), zap.String("groupID", group.ID.String()))
				}
				err := loopNodes(graph, s, group)
				if err != nil {
					return err
				}
			}
		case lci.IsHasPart:
			if s == ib {
				if p.OwningGraph() == graph {
					group.group[p] = struct{}{}
					sst.GlobalLogger.Debug("filterextract add node to group", zap.String("fragment", ib.Fragment()), zap.String("groupID", group.ID.String()))
				}
				err := loopNodes(graph, o.(sst.IBNode), group)
				if err != nil {
					return err
				}
			}
		default:
		}
		return nil
	})
	return err
}

func processRootNodes(graph sst.NamedGraph, rootIB sst.IBNode) (nodeGroup, error) {
	returnedGroup := nodeGroup{
		group: map[sst.IBNode]struct{}{},
	}
	sst.GlobalLogger.Debug("filterextract root node", zap.String("fragment", rootIB.Fragment()))
	err := loopNodes(graph, rootIB, returnedGroup)
	if err != nil {
		return returnedGroup, err
	}
	groupFragments := make([]string, 0, len(returnedGroup.group))
	var groupFragmentsLen int
	for d := range returnedGroup.group {
		var df string
		if d.IsBlankNode() {
			df = ""
		} else {
			df = d.Fragment()
		}
		groupFragments = append(groupFragments, df)
		groupFragmentsLen += len(df)
	}
	sort.Slice(groupFragments, func(j, k int) bool {
		return groupFragments[j] < groupFragments[k]
	})
	groupFragmentBytes := make([]byte, 0, groupFragmentsLen)
	for _, f := range groupFragments {
		groupFragmentBytes = append(groupFragmentBytes, ([]byte)(f)...)
	}
	returnedGroup.ID = uuid.NewSHA1(uuid.NameSpaceURL, groupFragmentBytes)
	returnedGroup.outsideNodes = map[sst.IBNode]struct{}{}
	for ib := range returnedGroup.group {
		err := collectOutsideNodes(returnedGroup, ib)
		if err != nil {
			return returnedGroup, err
		}
	}
	listMap(returnedGroup)
	// next: separate group in its own NG
	return returnedGroup, nil
}

func SearchRootNodes(graph sst.NamedGraph) ([]nodeGroup, error) {
	grouppedNodes := map[sst.IBNode]uuid.UUID{}
	var groups []nodeGroup
	var err error
	err = graph.ForAllIBNodes(func(d sst.IBNode) error {
		tempType := d.TypeOf()
		if tempType != nil {
			switch rt := tempType.InVocabulary().(type) {
			// KindPart may change to KindGenericPart
			case sso.KindPart,
				lci.KindOrganization,
				sso.KindShapeFeatureDefinition:
				sst.GlobalLogger.Info("IBNode is KindPart/KindOrganization/KindShapeFeatureDefinition", zap.String("IRI", d.IRI().String()))
				groups, err = appendGroups(graph, d, rt, groups, grouppedNodes)
				return err
			case rep.KindRepresentationContext:
				sst.GlobalLogger.Info("IBNode is KindRepresentationContext", zap.String("IRI", d.IRI().String()))

				var repItemCnt int
				var placementsOnly bool
				var hasExtGeomModel bool
				var is3DGeometricContext bool
				err = d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					switch p.InVocabulary().(type) {
					case rep.IsContextOfItems:
						if o != d {
							return nil
						}
						if _, ok := s.TypeOf().InVocabulary().(sso.KindExternalGeometricModel); ok {
							hasExtGeomModel = true
						}
						if repItemCnt == 0 {
							placementsOnly, err = hasPlacementsOnly(s)
							if err != nil {
								return err
							}
						}
						repItemCnt++
					case rep.IsRepresentationsInContext:
						if s != d {
							return nil
						}
						if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
							o := o.(sst.IBNode)
							if _, ok := o.TypeOf().InVocabulary().(sso.KindExternalGeometricModel); ok {
								hasExtGeomModel = true
							}
							if repItemCnt == 0 {
								placementsOnly, err = hasPlacementsOnly(o)
								if err != nil {
									return err
								}
							}
						}
						repItemCnt++

					case rep.IsCoordinateSpaceDimension:
						if s != d {
							return nil
						}
						if val, ok := o.(sst.Integer); ok && val == 3 {
							is3DGeometricContext = true
						}

					}
					return nil
				})
				if err != nil {
					return err
				}
				if (repItemCnt != 1 || !placementsOnly) && !hasExtGeomModel && is3DGeometricContext {
					groups, err = appendGroups(graph, d, rt, groups, grouppedNodes)
					return err
				}
				// if is3DGeometricContext {
				// 	groups, err = appendGroups(graph, d, rt, groups, grouppedNodes)
				// 	return err
				// }
			}
		}
		return nil
	})
	sort.Slice(groups, func(i, j int) bool {
		g1, g2 := groups[i], groups[j]
		return rootNodeTypeToPriority(g1.rootNodeType) < rootNodeTypeToPriority(g2.rootNodeType)
	})
	return groups, err
}

func appendGroups(
	graph sst.NamedGraph,
	rootNode sst.IBNode,
	rt sst.ElementInformer,
	inGroups []nodeGroup,
	grouppedNodes map[sst.IBNode]uuid.UUID,
) ([]nodeGroup, error) {
	groups := inGroups
	nodegroup, err := processRootNodes(graph, rootNode)
	if err != nil {
		return groups, err
	}
	nodegroup.rootNodeType = rt
	groups = append(groups, nodegroup)
	// Check if nodes in a group were not already assigned to another group
	// if that is the case print a warning and move node from node list
	// and consider it as outside node
	for d := range nodegroup.group {
		if gID, found := grouppedNodes[d]; found {
			sst.GlobalLogger.Warn("filterextract node already assigned to group",
				zap.String("fragment", d.Fragment()),
				zap.String("existingGroupID", gID.String()),
			)
			delete(nodegroup.group, d)
			nodegroup.outsideNodes[d] = struct{}{}
			sst.GlobalLogger.Debug("filterextract add node to outsideNodes",
				zap.String("fragment", d.Fragment()),
				zap.String("groupID", nodegroup.ID.String()),
			)
		} else {
			grouppedNodes[d] = nodegroup.ID
		}
	}
	return groups, nil
}

func hasPlacementsOnly(d sst.IBNode) (bool, error) {
	var notOnlyPlacements bool
	err := d.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if d == s {
			if _, ok := p.InVocabulary().(rep.KindItem); ok {
				if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
					o := o.(sst.IBNode)
					if _, ok := o.TypeOf().InVocabulary().(rep.KindPlacement); !ok {
						notOnlyPlacements = true
					}
					return nil
				}
				notOnlyPlacements = true
			}
		}
		return nil
	})
	return !notOnlyPlacements, err
}

func rootNodeTypeToPriority(rt sst.ElementInformer) int {
	switch rt.(type) {
	case sso.KindPart,
		lci.KindOrganization:
		return 0
	default:
		return 100
	}
}

func ExtractImportedGraphs(
	graph sst.NamedGraph,
	nodegroups []nodeGroup,
	predicateUsages map[sst.IBNode]map[sst.IBNode]struct{},
) error {
	for i, nodegroup := range nodegroups {
		currentNodeGroupNG := graph.Stage().CreateNamedGraph(sst.IRI(nodegroup.ID.URN()))
		sst.GlobalLogger.Debug("filterextract create named graph", zap.String("iri", currentNodeGroupNG.IRI().String()))

		graph.AddImport(currentNodeGroupNG)

		sst.GlobalLogger.Debug("filterextract add import",
			zap.String("from", graph.IRI().String()),
			zap.String("to", currentNodeGroupNG.IRI().String()),
		)

		nodegroup.importedGraph = currentNodeGroupNG

		// handle group nodes
		for n := range nodegroup.group {
			if n.IsBlankNode() {
				currentNodeGroupNG.MoveIBNode(n, "")
			} else {
				currentNodeGroupNG.MoveIBNode(n, n.Fragment())
			}
		}

		nodegroups[i] = nodegroup
	}
	for _, nodegroup := range nodegroups {
		for {
			var outsideNodeAdded bool
			for d := range nodegroup.outsideNodes {
				if d.OwningGraph() == graph {
					if d.IsBlankNode() {
						nodegroup.importedGraph.MoveIBNode(d, "")
						sst.GlobalLogger.Debug("filterextract move node to imported graph",
							zap.String("nodeID", d.ID().String()),
							zap.String("graphIRI", nodegroup.importedGraph.IRI().String()),
						)
					} else {
						nodegroup.importedGraph.MoveIBNode(d, d.Fragment())
						sst.GlobalLogger.Debug("filterextract move node to imported graph",
							zap.String("fragment", d.Fragment()),
							zap.String("graphIRI", nodegroup.importedGraph.IRI().String()),
						)
					}
					nodegroup.group[d] = struct{}{}
					if d.IsBlankNode() {
						sst.GlobalLogger.Debug("filterextract add node to group", zap.String("nodeID", d.ID().String()), zap.String("groupID", nodegroup.ID.String()))
					} else {
						sst.GlobalLogger.Debug("filterextract add node to group", zap.String("fragment", d.Fragment()), zap.String("groupID", nodegroup.ID.String()))
					}
					delete(nodegroup.outsideNodes, d)
					prevOutsideNodeCount := len(nodegroup.outsideNodes)
					err := collectOutsideNodes(nodegroup, d)
					if err != nil {
						return err
					}
					if len(nodegroup.outsideNodes) != prevOutsideNodeCount {
						outsideNodeAdded = true
					}
				}
			}
			if !outsideNodeAdded {
				break
			}
		}
	}
	var injectedGraph sst.NamedGraph
	for _, nodegroup := range nodegroups {
		importedGraphs := map[sst.NamedGraph]struct{}{}
		for outsideNode := range nodegroup.outsideNodes {
			if ng := outsideNode.OwningGraph(); ng != nil && !ng.IsReferenced() {
				if _, found := importedGraphs[ng]; found {
					continue
				}
				var err error
				if outsideNode.Fragment() != "first" {
					for _, val := range nodegroup.importedGraph.DirectImports() {
						if val.IRI() == ng.IRI() {
							err = sst.ErrNamedGraphAlreadyImported
						}
					}

					for _, val := range ng.DirectImports() {
						if val.IRI() == nodegroup.importedGraph.IRI() {
							err = sst.ErrNamedGraphImportCycle
						}
					}

					if err == nil {
						nodegroup.importedGraph.AddImport(ng)
					}
				}
				if err != nil {
					if !errors.Is(err, sst.ErrNamedGraphImportCycle) && !errors.Is(err, sst.ErrNamedGraphAlreadyImported) && !errors.Is(err, sst.ErrStagesAreNotTheSame) {
						sst.GlobalLogger.Debug("filterextract failed import",
							zap.String("from", ng.IRI().String()),
							zap.String("to", nodegroup.importedGraph.IRI().String()),
						)
						// return err
						panic(err)
					}
					injectedGraph, err = moveNamedGraphNodeToInjectGraph(
						nodegroup.importedGraph,
						outsideNode, injectedGraph,
						predicateUsages,
					)
				} else {
					sst.GlobalLogger.Debug("filterextract add import",
						zap.String("from", ng.IRI().String()),
						zap.String("to", nodegroup.importedGraph.IRI().String()),
					)
					importedGraphs[ng] = struct{}{}
				}
			}
		}
	}
	return nil
}

func collectOutsideNodes(group nodeGroup, ib sst.IBNode) error {
	return ib.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s == ib {
			if ng := p.OwningGraph(); ng != nil && !ng.IsReferenced() {
				if _, found := group.group[p]; !found {
					group.outsideNodes[p] = struct{}{}
					sst.GlobalLogger.Debug("filterextract add outside node to group",
						zap.String("fragment", p.Fragment()),
						zap.String("groupID", group.ID.String()),
					)
				}
			}
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				o := o.(sst.IBNode)
				if ng := o.OwningGraph(); ng != nil && !ng.IsReferenced() {
					if _, found := group.group[o]; !found {
						group.outsideNodes[o] = struct{}{}
						if o.IsBlankNode() {
							sst.GlobalLogger.Debug("filterextract add outside node to group",
								zap.String("nodeID", o.ID().String()),
								zap.String("groupID", group.ID.String()),
							)
						} else {
							sst.GlobalLogger.Debug("filterextract add outside node to group",
								zap.String("fragment", o.Fragment()),
								zap.String("groupID", group.ID.String()),
							)
						}
					}
				}
			}
		}
		return nil
	})
}

func moveNamedGraphNodeToInjectGraph(
	importingGraph sst.NamedGraph,
	ibnode sst.IBNode,
	inInjectedGraph sst.NamedGraph,
	predicateUsages map[sst.IBNode]map[sst.IBNode]struct{},
) (returnedGraph sst.NamedGraph, err error) {
	returnedGraph = inInjectedGraph
	importedGraphs := map[sst.NamedGraph]struct{}{}
	if returnedGraph == nil {
		injectedGraphImp := importingGraph.Stage().CreateNamedGraph("")
		importingGraph.AddImport(injectedGraphImp)

		returnedGraph = injectedGraphImp

		sst.GlobalLogger.Debug("filterextract injected graph created", zap.String("iri", injectedGraphImp.IRI().String()))
		importedGraphs[importingGraph] = struct{}{}
		sst.GlobalLogger.Debug("filterextract added import", zap.String("graphIRI", importingGraph.IRI().String()))
	}
	err = moveAllToGraph(returnedGraph, ibnode, importedGraphs, predicateUsages)
	if err != nil {
		panic(err)
	}
	return
}

func moveAllToGraph(
	targetGraph sst.NamedGraph,
	ibnode sst.IBNode,
	importedGraphs map[sst.NamedGraph]struct{},
	predicateUsages map[sst.IBNode]map[sst.IBNode]struct{},
) error {
	if ibnode.IsBlankNode() {
		sst.GlobalLogger.Debug("filterextract moved to injected graph", zap.String("nodeID", ibnode.ID().String()))
		targetGraph.MoveIBNode(ibnode, "")
	} else {
		sst.GlobalLogger.Debug("filterextract moved to injected graph", zap.String("fragment", ibnode.Fragment()))
		targetGraph.MoveIBNode(ibnode, ibnode.Fragment())
	}
	if p, found := predicateUsages[ibnode]; found {
		for u := range p {
			sst.GlobalLogger.Debug("filterextract predicate usage", zap.String("predicate", u.PrefixedFragment()))
			err := maybeAddImportFromUsedNode(u, targetGraph, importedGraphs)
			if err != nil {
				// return err
				panic(err)
			}
		}
	}
	return ibnode.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
		if s == ibnode {
			if ng := p.OwningGraph(); !ng.IsEmpty() && !ng.IsReferenced() {
				err := moveAllToGraph(targetGraph, p, importedGraphs, predicateUsages)
				if err != nil {
					return err
				}
			}
			if o.TermKind() == sst.TermKindIBNode || o.TermKind() == sst.TermKindTermCollection {
				o := o.(sst.IBNode)
				if ng := o.OwningGraph(); ng != nil && !ng.IsReferenced() {
					return moveAllToGraph(targetGraph, o, importedGraphs, predicateUsages)
				}
			}
		} else {
			sst.GlobalLogger.Debug("filterextract inverse predicate", zap.String("predicate", s.PrefixedFragment()))
			return maybeAddImportFromUsedNode(s, targetGraph, importedGraphs)
		}
		return nil
	})
}

func maybeAddImportFromUsedNode(d sst.IBNode, targetGraph sst.NamedGraph, importedGraphs map[sst.NamedGraph]struct{}) error {
	if g := d.OwningGraph(); g != (nil) && !g.IsReferenced() && g != targetGraph {
		sst.GlobalLogger.Debug("filterextract trying to import graph", zap.String("graphIRI", g.IRI().String()))
		if _, found := importedGraphs[g]; !found {
			var ng sst.NamedGraph

			ng = g.Stage().NamedGraph(targetGraph.IRI())
			if ng == nil {
				ng = g.Stage().CreateNamedGraph(targetGraph.IRI())
			}
			sst.GlobalLogger.Debug("filterextract create named graph", zap.String("iri", ng.IRI().String()))

			for _, val := range g.DirectImports() {
				if val.IRI() == targetGraph.IRI() {
					// return sst.ErrNamedGraphAlreadyImported
					// continue
					return nil
				}
			}

			for _, val := range targetGraph.DirectImports() {
				if val.IRI() == g.IRI() {
					return sst.ErrNamedGraphImportCycle
				}
			}

			g.AddImport(targetGraph)

			sst.GlobalLogger.Debug("filterextract added import", zap.String("graphIRI", g.IRI().String()))
			importedGraphs[g] = struct{}{}
		}
	}
	return nil
}

func commitData(sourceGraph sst.NamedGraph, targetRepository sst.Repository) error {
	var st sst.Stage
	// check if the sourceGraph is already saved in the targetRepository
	namespace, err := targetRepository.Dataset(context.TODO(), sourceGraph.IRI())
	// if not exist
	if err != nil {
		st = targetRepository.OpenStage(sst.DefaultTriplexMode)

	} else { // if exist
		st, err = namespace.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			return err
		}
	}

	_, err = st.MoveAndMerge(context.TODO(), sourceGraph.Stage())
	if err != nil {
		return err
	}

	// for test, print ttl out
	// for _, ng := range stage.NamedGraphs() {
	// 	f, err := os.Create(filepath.Join("../../step/testdata/ewh/stepxmlRepository", ng.ID().String()+".ttl"))
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	defer f.Close()
	// 	ng.RdfWrite(f, sst.RdfFormatTurtle)
	// }

	_, _, err = st.Commit(context.TODO(), fmt.Sprintf("filtered graph %s", sourceGraph.IRI()), sst.DefaultBranch)
	if err != nil {
		return err
	}
	return nil
}

// path is the repository path, the TTL files in the folder where the provided path is located will be checked.
func Run(repo sst.Repository, path string) {
	log.SetFlags(log.Lshortfile)
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), "_") {
			return filepath.SkipDir
		}
		if !d.IsDir() && !strings.HasPrefix(d.Name(), "_") && strings.HasSuffix(d.Name(), ".ttl") {
			sst.GlobalLogger.Debug("filterextract processing file", zap.String("path", path))
			file, err := os.Open(path)
			defer func() {
				e := file.Close()
				if err == nil {
					err = e
				}
			}()
			st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
			if err != nil {
				log.Panic(err)
			}
			graph := st.NamedGraphs()[0]
			predicateUsages, err := CollectNodeAsPredicateUsages(graph)
			if err != nil {
				log.Panic(err)
			}
			groups, err := SearchRootNodes(graph)
			if err != nil {
				log.Panic(err)
			}

			err = ExtractImportedGraphs(graph, groups, predicateUsages)
			if err != nil {
				log.Panic(err)
			}

			// for testing - write TTLs out
			// for _, ng := range graph.Stage().NamedGraphs() {
			// 	f, err := os.Create(ng.ID().String() + ".ttl")
			// 	if err != nil {
			// 		panic(err)
			// 	}
			// 	defer f.Close()

			// 	err = ng.RdfWrite(f, sst.RdfFormatTurtle)
			// 	if err != nil {
			// 		log.Panic(err)
			// 	}
			// }

			err = commitData(graph, repo)
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}
