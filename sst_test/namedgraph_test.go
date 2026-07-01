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
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/owl"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/semanticstep/sst-core/vocabularies/sso"
	"github.com/stretchr/testify/assert"
)

func Test_Empty(t *testing.T) {
	st := sst.OpenStage(sst.DefaultTriplexMode)
	g := st.CreateNamedGraph("")
	g.CreateIRINode("", sso.Part)

	// NG Node and above created Node
	assert.Equal(t, g.IRINodeCount(), 2)
	g.Empty()
	assert.Equal(t, g.IRINodeCount(), 0)
}

func Test_EmptyBlankNodes(t *testing.T) {
	st := sst.OpenStage(sst.DefaultTriplexMode)
	g := st.CreateNamedGraph("")
	g.CreateBlankNode(sso.Part)

	assert.Equal(t, g.BlankNodeCount(), 1)
	g.Empty()
	assert.Equal(t, g.BlankNodeCount(), 0)
}

func Test_RdfRead(t *testing.T) {
	path := "./testdata/test"
	ttlFilePath := "./testdata/Test_RdfRead/33031.ttl"

	os.RemoveAll(path)
	defer os.RemoveAll(path)

	t.Run("Step1", func(t *testing.T) {
		file, err := os.Open(ttlFilePath)
		if err != nil {
			panic(err)
		}
		st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			panic(err)
		}
		st.Dump()

		for _, ng := range st.ReferencedGraphs() {
			ng.Dump()
		}
	})
}

// HashNil GKot5hBsd81kMupNCXHaqbhv3huEbxAFMLnpcX2hniwn
// Bucket: c
// commit SHA:[5LNuwHeMUTXJy4rMcCM1gdZUVT12EyvTJmv8HWa8dnwc] (subbucket)
//   committed _DS-UUID_: c1efcf543e8e4cc7a7d182a9f613a363
//     committed _DS-SHA_: 4YJMR9WionB375oYvf8vh4Kd5YtHzhqMaYRD5NY2CAf7
//     parent _Commit-SHA_: 7WeSifPq9WPx4geoYFq1ATu5LaWeR1VLCKGs6yaHNkZo
//   Key: author, Value: default@semanticstep.net 2025-10-21 07:49:06 +0000 UTC
//   Key: message, Value: Delete NG C
// commit SHA:[7WeSifPq9WPx4geoYFq1ATu5LaWeR1VLCKGs6yaHNkZo] (subbucket)
//   committed _DS-UUID_: c1efcf543e8e4cc7a7d182a9f613a363
//     committed _DS-SHA_: FyG1kC3SUaxcjqSx6Zss2SQYaLwJXoMJnxXwUVfUq2Cu
//     parent _Commit-SHA_:
//   Key: author, Value: default@semanticstep.net 2025-10-21 07:49:06 +0000 UTC
//   Key: message, Value: Create NG C
// Bucket: ds
// [c1efcf543e8e4cc7a7d182a9f613a363] (subbucket:ds)
//   branch: master latest _Commit_SHA: 5LNuwHeMUTXJy4rMcCM1gdZUVT12EyvTJmv8HWa8dnwc
// Bucket: dsr
// DS-SHA:[4YJMR9WionB375oYvf8vh4Kd5YtHzhqMaYRD5NY2CAf7] (subbucket)
//   default NG-SHA: GKot5hBsd81kMupNCXHaqbhv3huEbxAFMLnpcX2hniwn
// DS-SHA:[FyG1kC3SUaxcjqSx6Zss2SQYaLwJXoMJnxXwUVfUq2Cu] (subbucket)
//   default NG-SHA: 4wvS4GtqBxrUefUNdrh8wp8G5THuAaDhob1AJEH6QJz1
// Bucket: log
// [0000000000000000] (subbucket:log)
//   Key: author, Value: default <default@semanticstep.net>
//   Key: message, Value: repository created
//   Key: timestamp, Value: 2025-10-21T15:49:06+08:00
//   Key: type, Value: init
// [0000000000000001] (subbucket:log)
//   Key: branch, Value: master
//   Key: commit_id, Value: 7WeSifPq9WPx4geoYFq1ATu5LaWeR1VLCKGs6yaHNkZo
//   Key: type, Value: commit
// [0000000000000002] (subbucket:log)
//   Key: branch, Value: master
//   Key: commit_id, Value: 5LNuwHeMUTXJy4rMcCM1gdZUVT12EyvTJmv8HWa8dnwc
//   Key: type, Value: commit
// Bucket: ngr
// NG-SHA: 4wvS4GtqBxrUefUNdrh8wp8G5THuAaDhob1AJEH6QJz1
// Value: @prefix rep:	<http://ontology.semanticstep.net/rep#> .
// @prefix rdf:	<http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
// @prefix owl:	<http://www.w3.org/2002/07/owl#> .
// @prefix :	<urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a363#> .

// <urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a363>	a	owl:Ontology .
// :mainC	a	rep:SchematicPort ;
// 	rdf:Bag	rep:angle .

func Test_LocalFullRepository_SingleNGDeleted(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

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

		_, _, err = stageC.Commit(context.TODO(), "Create NG C", sst.DefaultBranch)
		assert.NoError(t, err)

		info, err := repo.Info(context.TODO(), "")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(1), info.NumberOfDatasetsInBranch)

		err = ng.Delete()
		if err != nil {
			panic(err)
		}

		_, _, err = stageC.Commit(context.TODO(), "Delete NG C", sst.DefaultBranch)
		assert.NoError(t, err)

		info, err = repo.Info(context.TODO(), "")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), info.NumberOfDatasets)
		assert.Equal(t, int64(0), info.NumberOfDatasetsInBranch)

		// fmt.Println(repo.Info(context.TODO(), ""))
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

		lc, err := ds.LeafCommits(context.TODO())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(lc))

		bm, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, 0, len(bm))

		_, err = ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.ErrorIs(t, err, sst.ErrBranchNotFound)
	})

	t.Run("read commits", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}

		lc, err := dsC.LeafCommits(context.TODO())
		assert.Nil(t, err)

		commitDetail, err := dsC.CommitDetailsByHash(context.TODO(), lc[0])
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			if err != nil {
				if strings.Contains(err.Error(), sst.ErrDatasetHasBeenDeleted.Error()) {
					break
				}
			}
			ng := st.NamedGraph(sst.IRI(ngIDC.URN()))
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}
	})
}

func Test_LocalFullRepository_ErrNamedGraphImportcycle(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDA := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a361")
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")
	ngIDD := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a364")

	defer os.RemoveAll(dir)

	t.Run("2 namedgraphs", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
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

		assert.Equal(t, ngC.AddImport(ngB), sst.ErrNamedGraphImportCycle)
	})

	t.Run("3 namedgraphs", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))

		ngC := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))
		ngB.AddImport(ngC)

		ngD := st.CreateNamedGraph(sst.IRI(ngIDD.URN()))
		ngC.AddImport(ngD)

		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		assert.Equal(t, ngD.AddImport(ngB), sst.ErrNamedGraphImportCycle)
	})

	t.Run("4 namedgraphs", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngA := st.CreateNamedGraph(sst.IRI(ngIDA.URN()))
		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))
		ngA.AddImport(ngB)

		ngC := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))
		ngB.AddImport(ngC)

		ngD := st.CreateNamedGraph(sst.IRI(ngIDD.URN()))
		ngC.AddImport(ngD)

		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		assert.Equal(t, ngD.AddImport(ngA), sst.ErrNamedGraphImportCycle)
	})
}

func Test_LocalFullRepository_ErrNamedGraphIsImportedDeleted(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
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

		st.Commit(context.TODO(), "Commit NGB imports NGC", sst.DefaultBranch)

		err = ngC.Delete()
		assert.ErrorIs(t, err, sst.ErrNamedGraphIsImported)
	})
}

func Test_LocalFullRepository_RemoveImport(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
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

		st.Commit(context.TODO(), "Commit NGB imports NGC", sst.DefaultBranch)

		ngB.Dump()

		ngB.RemoveImport(ngC)

		ngB.Dump()
	})
}

func Test_LocalFullRepository_CreateNamedGraph(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		st.CreateNamedGraph(sst.IRI(ngIDB.URN()))

		assert.Panics(t, func() { st.CreateNamedGraph(sst.IRI(ngIDB.URN())) })
	})
}

func Test_LocalFullRepository_CreateNamedGraphByIRI(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		st.CreateNamedGraph("https://example.semanticstep.net/ontology1#")

		assert.Panics(t, func() { st.CreateNamedGraph("https://example.semanticstep.net/ontology1#") })
	})

	t.Run("write2", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		st.CreateNamedGraph("urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")

		assert.Panics(t, func() { st.CreateNamedGraph("urn:uuid:c1efcf54-3e8e-4cc7-a7d1-82a9f613a362") })
	})
}

// In this case, both NG-B and NG-C are loaded into the Stage,
// NG-B will reference NG-C without importing NG-C
func Test_LocalFullRepository_NGBReferencesNGC(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))

		ngC := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		mainC.AddStatement(rdf.Bag, rep.Angle)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		st.Commit(context.TODO(), "Commit NGB and NGC", sst.DefaultBranch)
	})
	t.Run("read Dataset B", func(t *testing.T) {
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
		// NGB does not import NGC, so open st of DatasetB will not contain NGC
		// so, ngC will be Nil
		assert.Nil(t, ngC)
	})
	t.Run("read Dataset C", func(t *testing.T) {
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

		ngC := st.NamedGraph(sst.IRI(ngIDC.URN()))
		if ngC == nil {
			panic("got nil NamedGraph")
		}

		ngC.Dump()
	})
}

func Test_LocalFullRepository_NGBReferencesNGC_DeleteNGC(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	ngIDB := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a362")
	ngIDC := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a363")

	defer os.RemoveAll(dir)
	t.Run("write", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)

		ngB := st.CreateNamedGraph(sst.IRI(ngIDB.URN()))

		ngC := st.CreateNamedGraph(sst.IRI(ngIDC.URN()))

		mainC := ngC.CreateIRINode("mainC")
		mainC.AddStatement(rdf.Type, rep.SchematicPort)

		mainC.AddStatement(rdf.Bag, rep.Angle)

		mainB := ngB.CreateIRINode("mainB")
		mainB.AddStatement(rep.MappingSource, mainC)

		st.Commit(context.TODO(), "Commit NGB and NGC", sst.DefaultBranch)

		err = ngC.Delete()
		if err != nil {
			panic(err)
		}

		ngC.Dump()

		ngB.Dump()

		st.Commit(context.TODO(), "Commit NGC Deleted", sst.DefaultBranch)
	})
	t.Run("read Dataset B", func(t *testing.T) {
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
		// NGB does not import NGC, so open st of DatasetB will not contain NGC
		// so, ngC will be Nil
		assert.Nil(t, ngC)
	})
	t.Run("read Dataset C", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}

		_, err = ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.ErrorIs(t, err, sst.ErrBranchNotFound)
	})

	t.Run("read commits", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		dsC, err := repo.Dataset(context.TODO(), sst.IRI(ngIDC.URN()))
		if err != nil {
			panic(err)
		}
		lc, err := dsC.LeafCommits(context.TODO())
		assert.Nil(t, err)

		commitDetail, err := dsC.CommitDetailsByHash(context.TODO(), lc[0])
		assert.NoError(t, err)
		for {
			commitDetail.Dump()
			st, err := dsC.CheckoutCommit(context.TODO(), commitDetail.Commit, sst.DefaultTriplexMode)
			if err != nil {
				if strings.Contains(err.Error(), sst.ErrDatasetHasBeenDeleted.Error()) {
					break
				}
			}
			ng := st.NamedGraph(sst.IRI(ngIDC.URN()))
			ng.Dump()
			if len(commitDetail.ParentCommits[dsC.IRI()]) != 0 {
				// Suppose there is only one parent
				commitDetail, err = dsC.CommitDetailsByHash(context.TODO(), commitDetail.ParentCommits[dsC.IRI()][0])
				assert.NoError(t, err)
			} else {
				break
			}
		}
	})
}

func TestWriteCheckIdentifier(t *testing.T) {
	outTTLFileName := t.Name() + ".ttl"
	outSSTFileName := t.Name() + ".sst"

	defer os.Remove(outTTLFileName)
	defer os.Remove(outSSTFileName)
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph("")
		ib1 := graph.CreateIRINode("", owl.Class)
		ib2 := graph.CreateIRINode("", owl.Class)
		blank := graph.CreateBlankNode(owl.Class)
		assert.Panics(t, func() {
			fmt.Println("0 ", blank.IRI())
		})
		fmt.Println("1 ", blank.ID())
		assert.Panics(t, func() {
			fmt.Println("2 ", ib1.ID())
		})
		fmt.Println("3 ", ib1.IRI())
		fmt.Println("4 ", ib1.PrefixedFragment())
		fmt.Println("5 ", owl.Class.IRI())
		fmt.Println("6 ", ib1.PrefixedFragment())
		ib1.ForAll(func(_ int, st, p sst.IBNode, o sst.Term) error {
			if st != ib1 { // skip inverses
				return nil
			}
			switch o.TermKind() {
			case sst.TermKindIBNode, sst.TermKindTermCollection:
				// fmt.Printf("    %st %st\n", p.IRI(), o.(sst.IBNode).IRI())
				var ib sst.IBNode
				ib = o.(sst.IBNode)
				if ib.IsBlankNode() {
					fmt.Printf("    %st _:%st\n", p.IRI(), ib.ID())
				} else {
					fmt.Printf("    %st %st\n", p.IRI(), ib.IRI())
					fmt.Printf("    %st %st\n", p.IRI(), ib.PrefixedFragment())
				}
			case sst.TermKindLiteral:
				fmt.Printf("    %st %v^^%st\n", p.IRI(), o.(sst.Literal), o.(sst.Literal).DataType().IRI())
			default:
				fmt.Printf("    %st %st\n", p.IRI(), o)
			}
			return nil
		})

		ib1.AddStatement(rdf.Type, blank)
		ib2.AddStatement(rdf.Type, blank)

		graph2 := st.CreateNamedGraph("")
		graph2.CreateIRINode(uuid.New().String(), owl.Class)

		graph3 := st.CreateNamedGraph("")
		graph4 := st.CreateNamedGraph("")
		graph.AddImport(graph2)
		graph.AddImport(graph3)
		graph.AddImport(graph4)
		outTTLFile, err := os.Create(outTTLFileName)
		if err != nil {
			panic(err)
		}
		defer outTTLFile.Close()

		err = graph.RdfWrite(outTTLFile, sst.RdfFormatTurtle)
		if err != nil {
			log.Panic(err)
		}

		outSSTFile, err := os.Create(outSSTFileName)
		if err != nil {
			log.Panic(err)
		}
		defer func() {
			_ = outSSTFile.Close()
		}()

		err = graph.SstWrite(outSSTFile)
		if err != nil {
			log.Panic(err)
		}
	})

	t.Run("readTTL", func(t *testing.T) {
		file, err := os.Open(outTTLFileName)
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()
		if err != nil {
			panic("open file failed!")
		}

		st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		_ = st
	})

	t.Run("readSST", func(t *testing.T) {
		file, err := os.Open(outSSTFileName)
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()
		if err != nil {
			panic("open file failed!")
		}

		st, err := sst.SstRead(bufio.NewReader(file), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}

		_ = st
	})
}

func TestWriteOnlyTermCollection(t *testing.T) {
	outTTLFileName := t.Name() + ".ttl"
	outSSTFileName := t.Name() + ".sst"

	defer os.Remove(outTTLFileName)
	defer os.Remove(outSSTFileName)
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(uuid.MustParse("874396c7-d43b-4251-b925-318383fe035e").URN()))
		ib1 := ng.CreateIRINode("ib1", owl.Class)
		ib2 := ng.CreateIRINode("ib2", owl.Class)

		n1 := ng.CreateIRINode("n1", rdfs.Comment)
		n2 := ng.CreateIRINode("n2", rdfs.Comment)

		col := ng.CreateCollection(n1, n2)
		ib1.AddStatement(ib2, col)

		outTTLFile, err := os.Create(outTTLFileName)
		if err != nil {
			panic(err)
		}
		defer outTTLFile.Close()

		err = ng.RdfWrite(outTTLFile, sst.RdfFormatTurtle)
		if err != nil {
			log.Panic(err)
		}

		outSSTFile, err := os.Create(outSSTFileName)
		if err != nil {
			log.Panic(err)
		}
		defer outSSTFile.Close()

		err = ng.SstWrite(outSSTFile)
		if err != nil {
			log.Panic(err)
		}
	})

	t.Run("readTTL", func(t *testing.T) {
		file, err := os.Open(outTTLFileName)
		assert.NoError(t, err)
		defer file.Close()

		if err != nil {
			panic("open file failed!")
		}

		st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		st.NamedGraphs()[0].PrintTriples()
		_ = st
	})

	t.Run("readSST", func(t *testing.T) {
		file, err := os.Open(outSSTFileName)
		assert.NoError(t, err)
		defer file.Close()

		if err != nil {
			panic("open file failed!")
		}

		ng, err := sst.SstRead(bufio.NewReader(file), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		ng.PrintTriples()

		_ = ng
	})
}

func TestWriteOnlyLiteral(t *testing.T) {
	outTTLFileName := t.Name() + ".ttl"
	outSSTFileName := t.Name() + ".sst"

	defer os.Remove(outTTLFileName)
	defer os.Remove(outSSTFileName)
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		ib.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
		ib.AddStatement(rdfs.Label, sst.LangString{Val: "Hello", LangTag: "en"})
		ib.AddStatement(rdfs.Label, sst.Double(3.14))
		ib.AddStatement(rdfs.Label, sst.Integer(42))
		ib.AddStatement(rdfs.Label, sst.Boolean(true))

		outTTLFile, err := os.Create(outTTLFileName)
		if err != nil {
			panic(err)
		}
		defer outTTLFile.Close()

		err = ng.RdfWrite(outTTLFile, sst.RdfFormatTurtle)
		if err != nil {
			log.Panic(err)
		}

		outSSTFile, err := os.Create(outSSTFileName)
		if err != nil {
			log.Panic(err)
		}
		defer func() {
			_ = outSSTFile.Close()
		}()

		err = ng.SstWrite(outSSTFile)
		if err != nil {
			log.Panic(err)
		}
	})

	t.Run("readTTL", func(t *testing.T) {
		file, err := os.Open(outTTLFileName)
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()
		if err != nil {
			panic("open file failed!")
		}

		st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		st.NamedGraphs()[0].PrintTriples()
		_ = st
	})

	t.Run("readSST", func(t *testing.T) {
		file, err := os.Open(outSSTFileName)
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()
		if err != nil {
			panic("open file failed!")
		}

		ng, err := sst.SstRead(bufio.NewReader(file), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		ng.PrintTriples()

		_ = ng
	})
}

func TestWriteOnlyLiteralCollection(t *testing.T) {
	outTTLFileName := t.Name() + ".ttl"
	outSSTFileName := t.Name() + ".sst"

	defer os.Remove(outTTLFileName)
	defer os.Remove(outSSTFileName)
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph("")
		ib := ng.CreateIRINode("main")
		lc1 := sst.NewLiteralCollection(sst.Double(1.0), sst.Double(1.5), sst.Double(2))
		lc2 := sst.NewLiteralCollection(sst.Double(3), sst.Double(2.5), sst.Double(4))
		ib.AddStatement(rdfs.Label, lc1)
		ib.AddStatement(rdfs.Label, lc2)

		outTTLFile, err := os.Create(outTTLFileName)
		if err != nil {
			panic(err)
		}
		defer outTTLFile.Close()

		err = ng.RdfWrite(outTTLFile, sst.RdfFormatTurtle)
		if err != nil {
			log.Panic(err)
		}

		outSSTFile, err := os.Create(outSSTFileName)
		if err != nil {
			log.Panic(err)
		}
		defer func() {
			_ = outSSTFile.Close()
		}()

		err = ng.SstWrite(outSSTFile)
		if err != nil {
			log.Panic(err)
		}
	})

	t.Run("readTTL", func(t *testing.T) {
		file, err := os.Open(outTTLFileName)
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()
		if err != nil {
			panic("open file failed!")
		}

		st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		st.NamedGraphs()[0].PrintTriples()
		_ = st
	})

	t.Run("readSST", func(t *testing.T) {
		file, err := os.Open(outSSTFileName)
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()
		if err != nil {
			panic("open file failed!")
		}

		ng, err := sst.SstRead(bufio.NewReader(file), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		ng.PrintTriples()

		_ = ng
	})
}

func TestWriteOnlyLiteralCollectionTermCollection(t *testing.T) {
	outTTLFileName := t.Name() + ".ttl"
	outSSTFileName := t.Name() + ".sst"

	defer os.Remove(outTTLFileName)
	defer os.Remove(outSSTFileName)
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph("")
		ib1 := ng.CreateIRINode("ib1", owl.Class)
		ib2 := ng.CreateIRINode("ib2", owl.Class)

		lc1 := sst.NewLiteralCollection(sst.Double(1.0), sst.Double(1.5), sst.Double(2))
		col := ng.CreateCollection(sst.Double(3), sst.Double(2.5))
		ib1.AddStatement(ib2, lc1)
		ib1.AddStatement(ib2, col)

		outTTLFile, err := os.Create(outTTLFileName)
		if err != nil {
			panic(err)
		}
		defer outTTLFile.Close()

		err = ng.RdfWrite(outTTLFile, sst.RdfFormatTurtle)
		if err != nil {
			log.Panic(err)
		}

		outSSTFile, err := os.Create(outSSTFileName)
		if err != nil {
			log.Panic(err)
		}
		defer func() {
			_ = outSSTFile.Close()
		}()

		err = ng.SstWrite(outSSTFile)
		if err != nil {
			log.Panic(err)
		}
	})

	t.Run("readTTL", func(t *testing.T) {
		file, err := os.Open(outTTLFileName)
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()
		if err != nil {
			panic("open file failed!")
		}

		st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		st.NamedGraphs()[0].PrintTriples()
		_ = st
	})

	t.Run("readSST", func(t *testing.T) {
		file, err := os.Open(outSSTFileName)
		defer func() {
			e := file.Close()
			if err == nil {
				err = e
			}
		}()
		if err != nil {
			panic("open file failed!")
		}

		ng, err := sst.SstRead(bufio.NewReader(file), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		ng.PrintTriples()

		_ = ng
	})
}

func TestWrite(t *testing.T) {
	outTTLFileName := t.Name() + ".ttl"
	outSSTFileName := t.Name() + ".sst"

	defer os.Remove(outTTLFileName)
	defer os.Remove(outSSTFileName)
	t.Run("write", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph("")
		ib1 := ng.CreateIRINode(uuid.New().String(), owl.Class)
		ib2 := ng.CreateIRINode(uuid.New().String(), owl.Class)
		blank := ng.CreateBlankNode(owl.Class)

		lc1 := sst.NewLiteralCollection(sst.Double(1.0), sst.Double(1.5), sst.Double(2))
		col := ng.CreateCollection(sst.Double(3), sst.Double(2.5))
		ib1.AddStatement(ib2, lc1)
		ib1.AddStatement(ib2, col)

		n1 := ng.CreateIRINode("node1", owl.Class)
		n2 := ng.CreateIRINode("node2", owl.Class)
		col2 := ng.CreateCollection(n1, n2)
		ib2.AddStatement(ib1, col2)

		ib := ng.CreateIRINode("main")
		ib.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
		ib.AddStatement(rdfs.Label, sst.LangString{Val: "Hello", LangTag: "en"})
		ib.AddStatement(rdfs.Label, sst.Double(3.14))
		ib.AddStatement(rdfs.Label, sst.Integer(42))
		ib.AddStatement(rdfs.Label, sst.Boolean(true))

		ib1.AddStatement(rdf.Type, blank)
		ib2.AddStatement(rdf.Type, blank)

		graph2 := st.CreateNamedGraph("")
		graph2.CreateIRINode(uuid.New().String(), owl.Class)

		graph3 := st.CreateNamedGraph("")
		graph4 := st.CreateNamedGraph("")
		ng.AddImport(graph2)
		ng.AddImport(graph3)
		ng.AddImport(graph4)
		outTTLFile, err := os.Create(outTTLFileName)
		if err != nil {
			panic(err)
		}
		defer outTTLFile.Close()

		err = ng.RdfWrite(outTTLFile, sst.RdfFormatTurtle)
		if err != nil {
			log.Panic(err)
		}

		outSSTFile, err := os.Create(outSSTFileName)
		if err != nil {
			log.Panic(err)
		}
		defer outSSTFile.Close()

		err = ng.SstWrite(outSSTFile)
		if err != nil {
			log.Panic(err)
		}
	})

	t.Run("readTTL", func(t *testing.T) {
		file, err := os.Open(outTTLFileName)
		assert.NoError(t, err)
		defer file.Close()

		if err != nil {
			panic("open file failed!")
		}

		st, err := sst.RdfRead(bufio.NewReader(file), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		st.NamedGraphs()[0].PrintTriples()
		_ = st
	})

	t.Run("readSST", func(t *testing.T) {
		file, err := os.Open(outSSTFileName)
		assert.NoError(t, err)
		defer file.Close()

		st, err := sst.SstRead(bufio.NewReader(file), sst.DefaultTriplexMode)
		if err != nil {
			log.Panic(err)
		}
		st.PrintTriples()
		_ = st
	})
}
