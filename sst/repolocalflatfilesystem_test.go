// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"context"
	"fmt"
	"testing"

	fs "github.com/relab/wrfs"
	"github.com/stretchr/testify/assert"
)

func Test_localFlatSstFileRepo_CommitPanic(t *testing.T) {
	repo, err := OpenLocalFlatFileSystemRepository(fs.DirFS("../vocabularies/dict"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ds, err := repo.Dataset(context.TODO(), IRI("http://ontology.semanticstep.net/lci"))
	if err != nil {
		t.Fatal(err)
	}

	st, err := ds.CheckoutBranch(context.TODO(), DefaultBranch, DefaultTriplexMode)
	if err != nil {
		t.Fatal(err)
	}

	ng := st.NamedGraphs()[0]

	ng.CreateIRINode("", lciPerson)

	_, _, err = st.Commit(context.TODO(), "test commit", DefaultBranch)
	assert.Contains(t, err.Error(), "Commit not supported")
}

func Test_localFlatSstFileRepo(t *testing.T) {
	repo, err := OpenLocalFlatFileSystemRepository(fs.DirFS("../vocabularies/dict"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	datasetIRIs, err := repo.Datasets(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(datasetIRIs), 15)

	for key, val := range datasetIRIs {
		fmt.Println(key, val)
	}

	ds, err := repo.Dataset(context.TODO(), IRI("http://ontology.semanticstep.net/rep"))
	if err != nil {
		t.Fatal(err)
	}

	st, err := ds.CheckoutBranch(context.TODO(), DefaultBranch, DefaultTriplexMode)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("local NamedGraphs:")
	for key, val := range st.NamedGraphs() {
		fmt.Println(key, val.IRI())
	}
	assert.Equal(t, 3, len(st.NamedGraphs()))

	fmt.Println("referenced NamedGraphs :")
	for key, val := range st.ReferencedGraphs() {
		fmt.Println(key, val.IRI())
	}
	assert.Equal(t, 7, len(st.ReferencedGraphs()))
}
