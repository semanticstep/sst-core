// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

/*
HelloWorldRepoLocalFlat is an example SST application that shows how to create
and use a LocalFlat repository. A LocalFlat repository stores data as SST files
in a directory without revision history or Bleve search indexing.
*/
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/google/uuid"
)

func main() {
	// check and delete old folder and files
	path := "./testsstrepo"
	if _, err := os.Stat(path); err == nil {
		err := os.RemoveAll(path)
		if err != nil {
			fmt.Printf("Failed to delete %s: %s\n", path, err)
		} else {
			fmt.Printf("%s has been deleted successfully\n", path)
		}
	} else if os.IsNotExist(err) {
		fmt.Println("The file or directory does not exist.")
	} else {
		fmt.Printf("Error checking if file exists: %s\n", err)
	}

	id := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a369")

	if err := os.MkdirAll(path, 0755); err != nil {
		panic(err)
	}

	// open a LocalFlatRepository
	repo, err := sst.OpenLocalFlatRepository(path)
	if err != nil {
		panic("OpenRepository failed")
	}
	defer repo.Close()

	stage := repo.OpenStage(sst.DefaultTriplexMode)

	graph := stage.CreateNamedGraph(sst.IRI(id.URN()))

	main := graph.CreateIRINode("main")

	main.AddStatement(rdf.Type, lci.Organization)
	main.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
	stage.Commit(context.TODO(), "first commit: add main code", sst.DefaultBranch)

	secondary := graph.CreateIRINode("secondary")

	secondary.AddStatement(rdf.Type, lci.Person)
	secondary.AddStatement(rdfs.Label, sst.String("Adam"))
	stage.Commit(context.TODO(), "second commit: add secondary code", sst.DefaultBranch)

	f, err := os.Create("helloworldlocalflatrepo.ttl")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	graph.RdfWrite(f, sst.RdfFormatTurtle)
}
