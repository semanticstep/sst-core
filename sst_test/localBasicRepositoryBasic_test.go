// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

// The reason why there is a separate package is about RegisterVocabularyMap.
// In sst_test_dummyvoc package, it imports package compiler, which will register a test VocabularyMap.
// In sst_test_realvoc package, it imports package vocabularies, which will register a real
// VocabularyMap that includes lci, owl...

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
)

func Test_LocalBasicRepository_UUIDNamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageC := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageC.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ng.CreateIRINode("mainC")

		mainC.AddStatement(rdf.Type, rep.SchematicPort)
		mainC.AddStatement(rdf.Bag, rep.Angle)

		_, modifiedDSIDs, _ := stageC.Commit(context.TODO(), "First commit of C", sst.DefaultBranch)
		assert.Equal(t, len(modifiedDSIDs), 1)
		assert.Equal(t, modifiedDSIDs[0], ngIDC)
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

func Test_LocalBasicRepository_IRINamedGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	testIri := "http://ontology.semanticstep.net/abc#"

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		stageB := repo.OpenStage(sst.DefaultTriplexMode)

		ng := stageB.CreateNamedGraph(sst.IRI(testIri))

		mainB := ng.CreateIRINode("mainB")

		mainB.AddStatement(rdf.Type, rep.SchematicPort)

		stageB.Commit(context.TODO(), "First commit of B", sst.DefaultBranch)
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(testIri))
		if err != nil {
			panic(err)
		}

		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ng := st.NamedGraph(sst.IRI(testIri))
		if ng == nil {
			panic("got nil NamedGraph")
		}

		ng.Dump()
	})
}

func Test_LocalBasicRepository_NGBImportNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))

		ngC := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))
		ngB.AddImport(ngC)

		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		_, modifiedDS, _ := st.Commit(context.TODO(), "Commit NGA imports NGB", sst.DefaultBranch)
		assert.Equal(t, 2, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, ngIDB, modifiedDS[0])
		assert.Equal(t, ngIDC, modifiedDS[1])
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

		ngB.Dump()

		ngC := st.NamedGraph(sst.IRI(ngIDC.URN()))
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		ngC.Dump()
		traversalByAPI(ngB)
	})
}

func Test_LocalBasicRepository_NGAImportsNGBImportsNGC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDA := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(sst.IRI(ngIDA.URN()))

		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))

		ngC := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-A imports NG-B
		// Create mainA node references mainB
		ngB.AddImport(ngC)
		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-A
		// creates mainD node references mainA
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		_, modifiedDS, _ := st.Commit(context.TODO(), "A imports B imports C", sst.DefaultBranch)

		assert.Equal(t, 3, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, ngIDA, modifiedDS[0])
		assert.Equal(t, ngIDB, modifiedDS[1])
		assert.Equal(t, ngIDC, modifiedDS[2])
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsD, err := repo.Dataset(context.TODO(), sst.IRI(ngIDA.URN()))
		if err != nil {
			panic(err)
		}

		st, err := dsD.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngA := st.NamedGraph(sst.IRI(ngIDA.URN()))
		if ngA == nil {
			panic("got nil NamedGraph")
		}

		ngA.Dump()

		ngB := st.NamedGraph(sst.IRI(ngIDB.URN()))
		if ngB == nil {
			panic("got nil NamedGraph")
		}

		ngB.Dump()

		ngC := st.NamedGraph(sst.IRI(ngIDC.URN()))
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		ngC.Dump()

		traversalByAPI(ngA)
	})
}

func Test_LocalBasicRepository_DiamondCase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDA := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")
	ngIDD := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364")

	t.Run("write", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(sst.IRI(ngIDA.URN()))
		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))
		ngC := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))
		ngD := st.CreateNamedGraph(sst.IRI(ngIDD.URN()))

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB.AddImport(ngC)
		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD.AddImport(ngC)

		mainD := ngD.CreateIRINode("mainD")
		mainD.AddStatement(rep.MappingSource, mainC)

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		// NG-A imports NG-D
		// creates mainA node references mainD
		ngA.AddImport(ngD)

		mainA.AddStatement(rep.MappingSource, mainD)

		_, modifiedDS, _ := st.Commit(context.TODO(), "Diamond Case", sst.DefaultBranch)

		assert.Equal(t, 4, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, ngIDA, modifiedDS[0])
		assert.Equal(t, ngIDB, modifiedDS[1])
		assert.Equal(t, ngIDC, modifiedDS[2])
		assert.Equal(t, ngIDD, modifiedDS[3])
	})
	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		datasetIRIs, err := repo.Datasets(context.TODO())
		if err != nil {
			panic(err)
		}
		assert.Equal(t, len(datasetIRIs), 4)

		for key, val := range datasetIRIs {
			fmt.Println(key, val)
		}

		dsA, err := repo.Dataset(context.TODO(), sst.IRI(ngIDA.URN()))
		if err != nil {
			panic(err)
		}

		st, err := dsA.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngA := st.NamedGraph(sst.IRI(ngIDA.URN()))
		if ngA == nil {
			panic("got nil NamedGraph")
		}

		ngA.Dump()

		ngB := st.NamedGraph(sst.IRI(ngIDB.URN()))
		if ngB == nil {
			panic("got nil NamedGraph")
		}

		ngB.Dump()

		ngC := st.NamedGraph(sst.IRI(ngIDC.URN()))
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		ngC.Dump()

		traversalByAPI(ngA)
	})
}

func Test_LocalBasicRepository_DiamondCaseCreateAndOpen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIDA := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")
	ngIDD := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364")

	t.Run("create", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "", "", false)
		if err != nil {
			panic(err)
		}
		defer repo.Close()
	})

	t.Run("write", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "", "")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(sst.IRI(ngIDA.URN()))
		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))
		ngC := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))
		ngD := st.CreateNamedGraph(sst.IRI(ngIDD.URN()))

		// Create mainC node
		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		// NG-B imports NG-C
		// Create mainB node references mainC
		ngB.AddImport(ngC)
		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		// NG-D imports NG-C
		// Create mainD node references mainC
		ngD.AddImport(ngC)

		mainD := ngD.CreateIRINode("mainD")
		mainD.AddStatement(rep.MappingSource, mainC)

		// NG-A imports NG-B
		// creates mainA node references mainB
		ngA.AddImport(ngB)
		mainA := ngA.CreateIRINode("mainA")
		mainA.AddStatement(rep.MappingSource, mainB)

		// NG-A imports NG-D
		// creates mainA node references mainD
		ngA.AddImport(ngD)

		mainA.AddStatement(rep.MappingSource, mainD)

		_, modifiedDS, _ := st.Commit(context.TODO(), "Diamond Case", sst.DefaultBranch)

		assert.Equal(t, 4, len(modifiedDS))
		sort.Slice(modifiedDS, func(i, j int) bool {
			return bytes.Compare(modifiedDS[i][:], modifiedDS[j][:]) < 0
		})
		assert.Equal(t, ngIDA, modifiedDS[0])
		assert.Equal(t, ngIDB, modifiedDS[1])
		assert.Equal(t, ngIDC, modifiedDS[2])
		assert.Equal(t, ngIDD, modifiedDS[3])
	})

	t.Run("read", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "", "")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsA, err := repo.Dataset(context.TODO(), sst.IRI(ngIDA.URN()))
		if err != nil {
			panic(err)
		}

		st, err := dsA.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		ngA := st.NamedGraph(sst.IRI(ngIDA.URN()))
		if ngA == nil {
			panic("got nil NamedGraph")
		}

		ngA.Dump()

		ngB := st.NamedGraph(sst.IRI(ngIDB.URN()))
		if ngB == nil {
			panic("got nil NamedGraph")
		}

		ngB.Dump()

		ngC := st.NamedGraph(sst.IRI(ngIDC.URN()))
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		ngC.Dump()

		traversalByAPI(ngA)
	})
}
