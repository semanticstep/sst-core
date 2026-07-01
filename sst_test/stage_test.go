// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// check typeof field after MoveAndMerge
func Test_Stage_Merge(t *testing.T) {
	sst.AtomicLevel.SetLevel(zap.DebugLevel)
	fileName1 := filepath.Join("./testdata/" + "Test_Stage_Merge1")
	fileName2 := filepath.Join("./testdata/" + "Test_Stage_Merge2")

	defer os.Remove(fileName1 + ".sst")
	defer os.Remove(fileName2 + ".sst")

	// read TTL1 and TTL2 and write them into sst files,
	// then MoveAndMerge TTL2's Stage to TTL1's Stage and check the typeof field of IBNode of TTL2
	t.Run("write", func(t *testing.T) {
		ng1 := readTTLFile(fileName1 + ".ttl")

		ng2 := readTTLFile(fileName2 + ".ttl")

		out, err := os.Create(fileName1 + ".sst")
		if err != nil {
			log.Panic(err)
		}
		defer out.Close()
		err = ng1.SstWrite(out)
		if err != nil {
			log.Panic(err)
		}

		out2, err := os.Create(fileName2 + ".sst")
		if err != nil {
			log.Panic(err)
		}
		defer out2.Close()
		err = ng2.SstWrite(out2)
		if err != nil {
			log.Panic(err)
		}

		john := ng2.GetIRINodeByFragment("John")
		assert.True(t, john.TypeOf().Is(lci.Person))

		_, err = ng1.Stage().MoveAndMerge(context.TODO(), ng2.Stage())
		if err != nil {
			panic(err)
		}

		assert.Equal(t, 2, len(ng1.Stage().NamedGraphs()))
		assert.Equal(t, 4, len(ng1.Stage().ReferencedGraphs()))

		assert.Equal(t, 2, ng1.Stage().NamedGraph(ng1.IRI()).IRINodeCount())
		readedMainA := ng1.Stage().NamedGraph(ng1.IRI()).GetIRINodeByFragment("mainA")
		assert.NotNil(t, readedMainA)
		readedMainA.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s != readedMainA { // skip inverses
				return nil
			}
			assert.Equal(t, s.Fragment(), "mainA")
			assert.True(t, p.Is(rep.MappingSource))
			assert.Equal(t, o.(sst.IBNode).Fragment(), "Comp1")
			return nil
		})

		assert.Equal(t, 4, ng1.Stage().NamedGraph(sst.IRI(ng2.IRI())).IRINodeCount())
		readedJohn := ng1.Stage().NamedGraph(sst.IRI(ng2.IRI())).GetIRINodeByFragment("John")
		assert.NotNil(t, readedJohn)
		readedJohn.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s != readedMainA { // skip inverses
				return nil
			}
			assert.Equal(t, s.Fragment(), "John")
			assert.True(t, p.Is(rdf.Type))
			assert.True(t, o.(sst.IBNode).Is(lci.Person))
			return nil
		})

		readedLinda := ng1.Stage().NamedGraph(sst.IRI(ng2.IRI())).GetIRINodeByFragment("Linda")
		assert.NotNil(t, readedLinda)
		readedLinda.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s != readedMainA { // skip inverses
				return nil
			}
			assert.Equal(t, s.Fragment(), "Linda")
			assert.True(t, p.Is(rdf.Type))
			assert.True(t, o.(sst.IBNode).Is(lci.Person))
			return nil
		})

		readedCom1 := ng1.Stage().NamedGraph(sst.IRI(ng2.IRI())).GetIRINodeByFragment("Comp1")
		assert.NotNil(t, readedCom1)
		readedCom1.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			if s != readedCom1 { // skip inverses
				return nil
			}
			assert.Equal(t, s.Fragment(), "Comp1")
			if p.Is(rdf.Type) {
				assert.True(t, o.(sst.IBNode).Is(lci.Organization))
			} else if p.Is(sst.IRI("http:example.com#employees")) {
				assert.True(t, o.(sst.IBNode).IsTermCollection())
				found := 0
				o.(sst.TermCollection).ForMembers(func(index int, li sst.Term) {
					if li.(sst.IBNode).Fragment() == "John" {
						found++
					} else if li.(sst.IBNode).Fragment() == "Linda" {
						found++
					}
				})
				assert.Equal(t, 2, found)
			} else if p.Is(sst.IRI("http:example.com#strings")) {
				found := 0
				o.(sst.LiteralCollection).ForMembers(func(index int, li sst.Literal) {
					switch li.(sst.String) {
					case "st1":
						found++
					case "st2":
						found++
					}
				})
				assert.Equal(t, 2, found)
			} else if p.Is(lci.HasPart) {
				assert.True(t, o.(sst.IBNode).IsBlankNode())
				o.(sst.IBNode).ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
					if p.Is(rdf.Type) {
						assert.True(t, o.(sst.IBNode).Is(lci.Organization))
					} else if p.Is(lci.IntegerAsLiteral) {
						assert.Equal(t, o.(sst.Integer), sst.Integer(14))
					}
					return nil
				})
			}

			return nil
		})

		john2AfterMove := ng1.Stage().NamedGraph(sst.IRI(ng2.IRI())).GetIRINodeByFragment("John")
		assert.True(t, john2AfterMove.TypeOf().Is(lci.Person))
	})

	// read sst file 2 then MoveAndMerge the Stage to an empty Stage
	// then check the typeof field of IBNode of sst2
	t.Run("read_then_merge_to_empty_Stage ", func(t *testing.T) {
		in, err := os.Open(fileName2 + ".sst")
		if err != nil {
			log.Panic(err)
		}
		defer in.Close()

		ng2, err := sst.SstRead(bufio.NewReader(in), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		john := ng2.GetIRINodeByFragment("John")

		stage2 := sst.OpenStage(sst.DefaultTriplexMode)
		stage2.MoveAndMerge(context.TODO(), ng2.Stage())
		john2 := stage2.NamedGraph(sst.IRI(ng2.IRI())).GetIRINodeByFragment("John")

		assert.True(t, john.TypeOf().Is(lci.Person))
		assert.True(t, john2.TypeOf().Is(lci.Person))
	})

	// read sst file 2 then MoveAndMerge the Stage to sst file 1 Stage
	// the check typeof field of IBNode of sst2
	t.Run("read_then_merge_to_sst1_Stage ", func(t *testing.T) {
		in1, err := os.Open(fileName1 + ".sst")
		if err != nil {
			log.Panic(err)
		}
		defer in1.Close()
		ng1, err := sst.SstRead(bufio.NewReader(in1), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}

		in2, err := os.Open(fileName2 + ".sst")
		if err != nil {
			log.Panic(err)
		}
		defer in2.Close()
		ng2, err := sst.SstRead(bufio.NewReader(in2), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		john := ng2.GetIRINodeByFragment("John")

		ng1.Stage().MoveAndMerge(context.TODO(), ng2.Stage())
		john2 := ng1.Stage().NamedGraph(sst.IRI(ng2.IRI())).GetIRINodeByFragment("John")

		assert.True(t, john.TypeOf().Is(lci.Person))
		assert.True(t, john2.TypeOf().Is(lci.Person))
	})

}

func Test_Stage_Reload_Typeof(t *testing.T) {
	sst.AtomicLevel.SetLevel(zap.DebugLevel)
	fileName2 := filepath.Join("./testdata/" + "Test_Stage_Merge2")

	t.Run("write", func(t *testing.T) {
		ng2 := readTTLFile(fileName2 + ".ttl")
		john := ng2.GetIRINodeByFragment("John")
		assert.True(t, john.TypeOf().Is(lci.Person))

		stage2 := reloadPersisted(t, ng2.Stage())

		john2 := stage2.NamedGraph(sst.IRI(ng2.IRI())).GetIRINodeByFragment("John")
		assert.True(t, john2.TypeOf().Is(lci.Person))
	})
}

// mainNg imports importedNg but not use it.
func Test_StageReload_NoUsingImportedNG_TypeOf(t *testing.T) {
	sst.AtomicLevel.SetLevel(zap.DebugLevel)

	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		importedNG := st.CreateNamedGraph(sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN()))
		b2 := importedNG.CreateIRINode("", lci.Person)
		assert.True(t, b2.TypeOf().Is(lci.Person))

		mainNG := st.CreateNamedGraph(sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN()))
		mainNG.CreateIRINode("", lci.Activity)

		mainNG.AddImport(importedNG)

		st = reloadPersisted(t, st)
		if assert.Len(t, st.NamedGraph(mainNG.IRI()).DirectImports(), 1) {
			readedImportedNG := st.NamedGraph(mainNG.IRI()).DirectImports()[0]
			assert.Equal(t, 2, readedImportedNG.IRINodeCount())
			var iTypeOf sst.IBNode
			assert.NoError(t, readedImportedNG.ForIRINodes(func(d sst.IBNode) error {
				if d.Fragment() != "" {
					fmt.Println(d.Fragment())
					iTypeOf = d.TypeOf()
					if assert.NotEqual(t, nil, iTypeOf) {
						assert.True(t, iTypeOf.Is(lci.Person))
					}
				}
				return nil
			}))
			assert.NotEqual(t, nil, iTypeOf)
		}
	})
}

// mainNg imports importedNg but and use it.
func Test_StageReload_UsingImportedNG_TypeOf(t *testing.T) {
	sst.AtomicLevel.SetLevel(zap.DebugLevel)

	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		importedNG := st.CreateNamedGraph(sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363").URN()))
		b2 := importedNG.CreateIRINode("", lci.Person)
		assert.True(t, b2.TypeOf().Is(lci.Person))

		mainNG := st.CreateNamedGraph(sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362").URN()))
		mainIB := mainNG.CreateIRINode("", lci.Activity)

		mainNG.AddImport(importedNG)
		mainIB.AddStatement(b2, sst.String("abc"))

		st = reloadPersisted(t, st)
		if assert.Len(t, st.NamedGraph(mainNG.IRI()).DirectImports(), 1) {
			readedImportedNG := st.NamedGraph(mainNG.IRI()).DirectImports()[0]
			assert.Equal(t, 2, readedImportedNG.IRINodeCount())
			var iTypeOf sst.IBNode
			assert.NoError(t, readedImportedNG.ForIRINodes(func(d sst.IBNode) error {
				if d.Fragment() != "" {
					fmt.Println(d.Fragment())
					iTypeOf = d.TypeOf()
					if assert.NotEqual(t, nil, iTypeOf) {
						assert.True(t, iTypeOf.Is(lci.Person))
					}
				}
				return nil
			}))
			assert.NotEqual(t, nil, iTypeOf)
		}
	})
}

func reloadPersisted(t *testing.T, st sst.Stage) sst.Stage {
	stagePath := t.TempDir()
	assert.NoError(t, st.WriteSstFilesDirectory(fs.DirFS(stagePath)))
	st, err := sst.ReadStageFromSstFiles(fs.DirFS(stagePath), sst.DefaultTriplexMode)
	assert.NoError(t, err)
	return st
}
