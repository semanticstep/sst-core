// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
)

func Test_ReadCollectionIntoLocalFullRepository(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDB := uuid.MustParse("acc85dd8-4b61-4f90-934c-43ca645e73ae")
	readTTLFilePath := filepath.Join("./testdata", "stageStorageCollection_test.ttl")
	outputTTLFilePath := filepath.Join(dir, "stageStorageCollection_test_output.ttl")

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		tempNg := readTTLFile(readTTLFilePath)
		fromStage := tempNg.Stage()

		_, err = st.MoveAndMerge(context.TODO(), fromStage)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, false, fromStage.IsValid())

		st.Commit(context.TODO(), "load all ttl files", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDB.URN()))
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngB := st.NamedGraph(sst.IRI(ngIDB.URN()))
		if ngB == nil {
			panic("got nil NamedGraph")
		}
		traversalByAPI2Level(ngB)

		f, err := os.Create(outputTTLFilePath)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		err = ngB.RdfWrite(f, sst.RdfFormatTurtle)
		assert.True(t, compare(t, readTTLFilePath, outputTTLFilePath))
		if err != nil {
			log.Panic(err)
		}
	})

	t.Run("cleanup", func(t *testing.T) {
		// os.Remove("./testdata/stageStorageCollection_test_output.ttl")
	})
}

func traversalByAPI2Level(ngA sst.NamedGraph) {
	err := ngA.ForIRINodes(func(t sst.IBNode) error {
		fmt.Printf("  IBNode: %st\n", t.Fragment())
		return t.ForAll(func(_ int, st, p sst.IBNode, o sst.Term) error {
			if st != t {
				return nil
			}
			switch o.TermKind() {
			case sst.TermKindIBNode:
				var ib sst.IBNode
				ib = o.(sst.IBNode)
				if ib.IsBlankNode() {
					fmt.Printf("    %st _:%st\n", p.IRI(), ib.ID())
				} else {
					fmt.Printf("    %st %st\n", p.IRI(), ib.IRI())
				}
				secondLevelO := o.(sst.IBNode)
				secondLevelO.ForAll(func(_ int, st, p sst.IBNode, o sst.Term) error {
					if secondLevelO != st {
						return nil
					}
					if secondLevelO.IsBlankNode() {
						fmt.Printf("      IBNode: %st\n", secondLevelO.ID())
					} else {
						fmt.Printf("      IBNode: %st\n", secondLevelO.Fragment())
					}
					switch o.TermKind() {
					case sst.TermKindIBNode:
						var ib sst.IBNode
						ib = o.(sst.IBNode)
						if ib.IsBlankNode() {
							fmt.Printf("    %st _:%st\n", p.IRI(), ib.ID())
						} else {
							fmt.Printf("    %st %st\n", p.IRI(), ib.IRI())
						}
					case sst.TermKindTermCollection:
						fmt.Printf("    %st _:%st\n", p.IRI(), o.(sst.TermCollection).ID())
					case sst.TermKindLiteral:
						fmt.Printf("      %st %v^^%st\n", p.IRI(), o.(sst.Literal), o.(sst.Literal).DataType().IRI())
					default:
						fmt.Printf("      %st %st\n", p.IRI(), o)
					}
					return nil
				})
			case sst.TermKindTermCollection:
				fmt.Printf("    %st _:%st\n", p.IRI(), o.(sst.TermCollection).ID())
				secondLevelO := o.(sst.IBNode)
				secondLevelO.ForAll(func(_ int, st, p sst.IBNode, o sst.Term) error {
					if secondLevelO != st {
						return nil
					}
					if secondLevelO.IsBlankNode() {
						fmt.Printf("      IBNode: %st\n", secondLevelO.ID())
					} else {
						fmt.Printf("      IBNode: %st\n", secondLevelO.Fragment())
					}
					switch o.TermKind() {
					case sst.TermKindIBNode:
						var ib sst.IBNode
						ib = o.(sst.IBNode)
						if ib.IsBlankNode() {
							fmt.Printf("    %st _:%st\n", p.IRI(), ib.ID())
						} else {
							fmt.Printf("    %st %st\n", p.IRI(), ib.IRI())
						}
					case sst.TermKindTermCollection:
						fmt.Printf("    %st _:%st\n", p.IRI(), o.(sst.TermCollection).ID())
					case sst.TermKindLiteral:
						fmt.Printf("      %st %v^^%st\n", p.IRI(), o.(sst.Literal), o.(sst.Literal).DataType().IRI())
					default:
						fmt.Printf("      %st %st\n", p.IRI(), o)
					}
					return nil
				})
			case sst.TermKindLiteral:
				fmt.Printf("    %st %v^^%st\n", p.IRI(), o.(sst.Literal), o.(sst.Literal).DataType().IRI())
			default:
				fmt.Printf("    %st %st\n", p.IRI(), o)
			}
			return nil
		})
	})
	if err != nil {
		panic(err)
	}
}

func Test_StorageCollection_UUIDNamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		mainC.AddStatement(rdf.Bag, rep.Angle)

		stageC.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(sst.IRI(ngIDC.URN()))
		if ng == nil {
			panic("got nil NamedGraph")
		}

		ng.Dump()
	})
}

func Test_StorageCollection_UUIDNamedGraphAssert(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		assert.Equal(t, sst.IRI("urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a363"), ng.IRI())

		mainC := ng.CreateIRINode("mainC")
		assert.True(t, mainC.IsIRINode())
		assert.False(t, mainC.IsBlankNode())
		assert.False(t, mainC.IsTermCollection())

		mainC2 := ng.CreateIRINode("mainC2")
		assert.True(t, mainC.IsIRINode())
		assert.False(t, mainC.IsBlankNode())
		assert.False(t, mainC.IsTermCollection())

		col := ng.CreateCollection(mainC, mainC2)
		assert.False(t, col.(sst.IBNode).IsIRINode())
		assert.True(t, col.(sst.IBNode).IsBlankNode())
		assert.True(t, col.(sst.IBNode).IsTermCollection())

		blankC := ng.CreateBlankNode(lci.Person)
		assert.False(t, blankC.IsIRINode())
		assert.True(t, blankC.IsBlankNode())
		assert.False(t, blankC.IsTermCollection())

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		mainC.AddStatement(rdf.Bag, col)

		_, modifiedDSIDs, _ := stageC.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)

		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, modifiedDSIDs[0], ngIDC)
	})

	t.Run("CheckoutMasterBranch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(sst.IRI(ngIDC.URN()))
		if ng == nil {
			panic("got nil NamedGraph")
		}

		ng.Dump()
	})
}
