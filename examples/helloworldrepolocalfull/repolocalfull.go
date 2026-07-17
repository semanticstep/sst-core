// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

/*
HelloWorldRepoLocalFull is an example SST application that demonstrates how to
create, open, and interact with a LocalFull repository without Bleve indexing.
It shows creating a repository, adding data, committing changes, writing a Turtle
export, and checking out datasets to verify their contents.
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
	dir := "./testsstrepo"
	id := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a369")

	// Clean up old folder and files.
	if _, err := os.Stat(dir); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Printf("Failed to delete %s: %s\n", dir, err)
		} else {
			fmt.Printf("%s has been deleted successfully\n", dir)
		}
	} else if os.IsNotExist(err) {
		fmt.Println(dir, " - This file or directory does not exist.")
	} else {
		fmt.Printf("Error checking if file exists: %s\n", err)
	}

	// Step 1: Create a LocalFullRepository, add data, commit, and write TTL.
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		panic("OpenRepository failed")
	}
	defer repo.Close()

	stage := repo.OpenStage(sst.DefaultTriplexMode)
	graph := stage.CreateNamedGraph(sst.IRI(id.URN()))

	mainNode := graph.CreateIRINode("main")
	mainNode.AddStatement(rdf.Type, lci.Organization)
	mainNode.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
	stage.Commit(context.TODO(), "first commit: add main code", sst.DefaultBranch)

	secondary := graph.CreateIRINode("secondary")
	secondary.AddStatement(rdf.Type, lci.Person)
	secondary.AddStatement(rdfs.Label, sst.String("Adam"))
	stage.Commit(context.TODO(), "second commit: add secondary code", sst.DefaultBranch)

	f, err := os.Create("repolocalfull.ttl")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	graph.RdfWrite(f, sst.RdfFormatTurtle)

	// Step 2: Reopen the repository, checkout the dataset, and verify the IRINode count.
	checkoutAndVerify(dir, id)
}

func checkoutAndVerify(dir string, id uuid.UUID) {
	repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
	if err != nil {
		panic(err)
	}
	defer repo.Close()

	dataset, err := repo.Dataset(context.TODO(), sst.IRI(id.URN()))
	if err != nil {
		panic(err)
	}

	stage, err := dataset.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
	if err != nil {
		panic(err)
	}

	g := stage.NamedGraph(sst.IRI(id.URN()))
	if g.IRINodeCount() != 3 {
		panic(fmt.Sprintf("expected 3 IRINodes, got %d", g.IRINodeCount()))
	}
}
