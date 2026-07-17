// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/countrycodes"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func Test_RdfWriteNamedGraphEqual(t *testing.T) {
	t.Run("Equal Empty Graph", func(t *testing.T) {
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))

		st2 := sst.OpenStage(sst.DefaultTriplexMode)
		ng2 := st2.CreateNamedGraph(sst.IRI(ngID.URN()))

		assert.True(t, ng.Equal(ng2))
	})

	t.Run("Equal one IRI Node", func(t *testing.T) {
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		ng.CreateIRINode("Jane", lci.Person)

		st2 := sst.OpenStage(sst.DefaultTriplexMode)
		ng2 := st2.CreateNamedGraph(sst.IRI(ngID.URN()))
		ng2.CreateIRINode("Jane", lci.Person)

		assert.True(t, ng.Equal(ng2))
	})

	t.Run("Equal one blank Node", func(t *testing.T) {
		t.Skip("blankNode compare need further develop")
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		ng.CreateBlankNode(lci.Person)

		st2 := sst.OpenStage(sst.DefaultTriplexMode)
		ng2 := st2.CreateNamedGraph(sst.IRI(ngID.URN()))
		ng2.CreateBlankNode(lci.Person)

		assert.True(t, ng.Equal(ng2))
	})
	t.Run("Equal several IRI Nodes", func(t *testing.T) {
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		jane := ng.CreateIRINode("Jane", lci.Person)

		organization := ng.CreateIRINode("ECT", lci.Organization)
		organization.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor := ng.CreateIRINode("workFor", rdf.Property)
		workFor.AddStatement(rdfs.Domain, lci.Person)
		workFor.AddStatement(rdfs.Range, lci.Organization)

		jane.AddStatement(workFor, organization)

		st2 := sst.OpenStage(sst.DefaultTriplexMode)
		ng2 := st2.CreateNamedGraph(sst.IRI(ngID.URN()))
		jane2 := ng2.CreateIRINode("Jane", lci.Person)

		organization2 := ng2.CreateIRINode("ECT", lci.Organization)
		organization2.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor2 := ng2.CreateIRINode("workFor", rdf.Property)
		workFor2.AddStatement(rdfs.Domain, lci.Person)
		workFor2.AddStatement(rdfs.Range, lci.Organization)

		jane2.AddStatement(workFor2, organization2)

		assert.True(t, ng.Equal(ng2))
	})

	t.Run("Equal only compares subject triples", func(t *testing.T) {
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		jane := ng.CreateIRINode("Jane", lci.Person)

		organization := ng.CreateIRINode("ECT", lci.Organization)
		organization.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor := ng.CreateIRINode("workFor", rdf.Property)
		workFor.AddStatement(rdfs.Domain, lci.Person)
		workFor.AddStatement(rdfs.Range, lci.Organization)

		jane.AddStatement(workFor, organization)

		st2 := sst.OpenStage(sst.DefaultTriplexMode)
		ng2 := st2.CreateNamedGraph(sst.IRI(ngID.URN()))
		jane2 := ng2.CreateIRINode("Jane", lci.Person)
		ng22 := st2.CreateNamedGraph("")
		linda := ng22.CreateIRINode("Linda", lci.Person)
		linda.AddStatement(rdfs.Domain, jane2)

		organization2 := ng2.CreateIRINode("ECT", lci.Organization)
		organization2.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor2 := ng2.CreateIRINode("workFor", rdf.Property)
		workFor2.AddStatement(rdfs.Domain, lci.Person)
		workFor2.AddStatement(rdfs.Range, lci.Organization)

		jane2.AddStatement(workFor2, organization2)

		assert.True(t, ng.Equal(ng2))
	})

	t.Run("Not Equal IRI Nodes", func(t *testing.T) {
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		jane := ng.CreateIRINode("Jane", lci.Person)
		ng.CreateIRINode("Linda", lci.Person)

		organization := ng.CreateIRINode("ECT", lci.Organization)
		organization.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor := ng.CreateIRINode("workFor", rdf.Property)
		workFor.AddStatement(rdfs.Domain, lci.Person)
		workFor.AddStatement(rdfs.Range, lci.Organization)

		jane.AddStatement(workFor, organization)

		st2 := sst.OpenStage(sst.DefaultTriplexMode)
		ng2 := st2.CreateNamedGraph(sst.IRI(ngID.URN()))
		jane2 := ng2.CreateIRINode("Jane", lci.Person)

		organization2 := ng2.CreateIRINode("ECT", lci.Organization)
		organization2.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor2 := ng2.CreateIRINode("workFor", rdf.Property)
		workFor2.AddStatement(rdfs.Domain, lci.Person)
		workFor2.AddStatement(rdfs.Range, lci.Organization)

		jane2.AddStatement(workFor2, organization2)

		assert.False(t, ng.Equal(ng2))
	})

	t.Run("Not Equal Triples", func(t *testing.T) {
		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		jane := ng.CreateIRINode("Jane", lci.Person)
		ng.CreateIRINode("Linda", lci.Person)

		organization := ng.CreateIRINode("ECT", lci.Organization)
		organization.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor := ng.CreateIRINode("workFor", rdf.Property)
		workFor.AddStatement(rdfs.Domain, lci.Person)

		jane.AddStatement(workFor, organization)

		st2 := sst.OpenStage(sst.DefaultTriplexMode)
		ng2 := st2.CreateNamedGraph(sst.IRI(ngID.URN()))
		jane2 := ng2.CreateIRINode("Jane", lci.Person)

		organization2 := ng2.CreateIRINode("ECT", lci.Organization)
		organization2.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor2 := ng2.CreateIRINode("workFor", rdf.Property)
		workFor2.AddStatement(rdfs.Domain, lci.Person)
		workFor2.AddStatement(rdfs.Range, lci.Organization)

		jane2.AddStatement(workFor2, organization2)

		assert.False(t, ng.Equal(ng2))
	})
}

func Test_RdfWriteCheckTriple(t *testing.T) {
	// testName := filepath.Join("./testdata/" + t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		// create a NamedGraph and get it
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))

		jane := ng.CreateIRINode("Jane", lci.Person)
		assert.True(t, jane.CheckTriple(rdf.Type, lci.Person))

		blankOrganization := ng.CreateBlankNode(lci.Organization)
		assert.True(t, blankOrganization.CheckTriple(rdf.Type, lci.Organization))

		blankOrganization.AddStatement(rdfs.Label, sst.String("ECT"))
		assert.True(t, blankOrganization.CheckTriple(rdfs.Label, sst.String("ECT")))
		assert.False(t, blankOrganization.CheckTriple(rdfs.Label, sst.String("FCT")))

		workFor := ng.CreateIRINode("workFor", rdf.Property)
		assert.True(t, workFor.CheckTriple(rdf.Type, rdf.Property))

		workFor.AddStatement(rdfs.Domain, lci.Person)
		assert.True(t, workFor.CheckTriple(rdfs.Domain, lci.Person))
		assert.False(t, workFor.CheckTriple(rdfs.Domain, lci.Organization))

		workFor.AddStatement(rdfs.Range, lci.Organization)
		assert.True(t, workFor.CheckTriple(rdfs.Range, lci.Organization))

		jane.AddStatement(workFor, blankOrganization)
		assert.True(t, jane.CheckTriple(workFor, blankOrganization))

		// ng.Dump()
		// writeToFile(ng, testName)
	})
}

func Test_RdfWriteStaticStage(t *testing.T) {
	sst.AtomicLevel.SetLevel(zap.DebugLevel)
	t.Run("write", func(t *testing.T) {
		dicStage := sst.StaticDictionary()
		vocabNgs := dicStage.NamedGraphs()
		for _, ng := range vocabNgs {
			ng.ForIRINodes(func(d sst.IBNode) error {
				if d.Fragment() != "" {
					e := d.InVocabulary()
					if e == nil {
						if d.PrefixedFragment() == "rdf:http://www.w3.org/1999/02/22-rdf-syntax-ns#first" {
							d.PrefixedFragment()
						}
					}
				}
				return nil
			})
		}
	})
}

func Test_RdfWriteRdfFirstNodes(t *testing.T) {
	testName := filepath.Join("./testdata/" + t.Name())
	defer removeFolder(testName + ".sst")
	defer removeFolder(testName + ".ttl")
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		// create a NamedGraph and get it
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))

		jane := ng.CreateIRINode("Jane", lci.Person)

		jane.AddStatement(rdf.Bag, sst.String("ECT"))

		jane.ForAll(func(_ int, s, p sst.IBNode, o sst.Term) error {
			fmt.Println(p.PrefixedFragment(), p.InVocabulary().IRI())
			return nil
		})

		// ng.Dump()
		writeToFile(ng, testName)
	})
}

func Test_RdfWriteBlankNodes(t *testing.T) {
	t.Skip("sst file may be different in each run")
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		// create a NamedGraph and get it
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))

		jane := ng.CreateIRINode("Jane", lci.Person)

		blankOrganization := ng.CreateBlankNode(lci.Organization)
		blankOrganization.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor := ng.CreateIRINode("workFor", rdf.Property)
		workFor.AddStatement(rdfs.Domain, lci.Person)
		workFor.AddStatement(rdfs.Range, lci.Organization)

		jane.AddStatement(workFor, blankOrganization)

		// ng.Dump()
		writeToFile(ng, testName)
	})
	t.Run("read", func(t *testing.T) {
		generatedFiles := []string{
			testName,
		}

		compareFiles(t, generatedFiles)
	})
}

func Test_RdfWriteAllNormalIBNodes(t *testing.T) {
	t.Skip("sst file may be different in each run")
	testName := filepath.Join("./testdata/" + t.Name())
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)

		ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")

		// create a NamedGraph and get it
		ng := st.CreateNamedGraph(sst.IRI(ngID.URN()))

		// subject local, predicate referenced, object local
		john := ng.CreateIRINode("John", lci.Person)
		jane := ng.CreateIRINode("Jane", lci.Person)
		adam := ng.CreateIRINode("Adam", lci.Person)
		Oliver := ng.CreateIRINode("Oliver", lci.Person)

		dct := ng.CreateIRINode("DCT", lci.Organization)
		blankOrganization := ng.CreateBlankNode(lci.Organization)
		blankOrganization.AddStatement(rdfs.Label, sst.String("ECT"))

		workFor := ng.CreateIRINode("workFor", rdf.Property)
		liveIn := ng.CreateIRINode("liveIn", rdf.Property)
		hasFriend := ng.CreateIRINode("hasFriend", rdf.Property)

		col := ng.CreateCollection(jane, adam, Oliver)

		lc1 := sst.NewLiteralCollection(sst.Integer(255), sst.Integer(255), sst.Integer(255))
		white := ng.CreateIRINode("white", rep.ColourRGB)
		white.AddStatement(rep.Rgb, lc1)

		// subject local, predicate local, object local TermCollection
		john.AddStatement(hasFriend, col)

		// subject local, predicate referenced, object referenced
		workFor.AddStatement(rdfs.Domain, lci.Person)
		workFor.AddStatement(rdfs.Range, lci.Organization)

		// subject local, predicate local, object local
		john.AddStatement(workFor, dct)

		// subject local, predicate local, object referenced
		jane.AddStatement(liveIn, countrycodes.Cn)
		jane.AddStatement(workFor, blankOrganization)

		// ng.Dump()
		writeToFile(ng, testName)
	})
	t.Run("read", func(t *testing.T) {
		generatedFiles := []string{
			testName,
		}

		compareFiles(t, generatedFiles)
	})
}
