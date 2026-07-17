// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

/*
HelloWorldRepoLocalFullWithBleve is an example SST application that demonstrates
creating a LocalFull repository with Bleve indexing. It shows registering a default
index handler, adding data, committing, writing a Turtle export, performing Bleve
searches, opening an existing Bleve index directly, and verifying dataset checkout.
*/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/blevesearch/bleve/v2"
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

	// Step 1: Create a LocalFullRepository, register index handler, add data, commit, and write TTL.
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		panic(err)
	}
	defer repo.Close()

	repo.RegisterIndexHandler(defaultderive.DeriveInfo())

	fmt.Println(repo.Info(context.TODO(), ""))

	stage := repo.OpenStage(sst.DefaultTriplexMode)
	graph := stage.CreateNamedGraph(sst.IRI(id.URN()))

	mainNode := graph.CreateIRINode("main")
	mainNode.AddStatement(rdf.Type, lci.Organization)
	mainNode.AddStatement(rdfs.Label, sst.String("Test Ltd."))

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

	// Step 2: Search the Bleve index.
	searchBleveIndex(repo.Bleve())

	// Step 3: Open the existing Bleve index directly and search.
	openAndSearchBleve(dir)

	// Step 4: Reopen the repository, checkout the dataset, and verify node count.
	checkoutAndVerify(dir, id)

	// Step 5: Run the same sequence in a fresh directory to demonstrate indexing behavior.
	dir2 := "./testsstrepo2"
	os.RemoveAll(dir2)
	defer os.RemoveAll(dir2)
	demonstrateIndexBehavior(dir2, id)
}

func searchBleveIndex(openedIndex bleve.Index) {
	query := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = 1000

	searchResults, err := openedIndex.Search(searchRequest)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(searchResults)

	for _, hit := range searchResults.Hits {
		_, err := openedIndex.Document(hit.ID)
		if err != nil {
			log.Printf("Error retrieving document %s: %v", hit.ID, err)
			continue
		}
	}
	fmt.Println("Data has been successfully exported!")
}

func openAndSearchBleve(dir string) {
	indexPath := dir + "/index.bleve"
	openedIndex, err := bleve.Open(indexPath)
	if err != nil {
		log.Fatal(err)
	}
	defer openedIndex.Close()

	query := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = 1000

	searchResults, err := openedIndex.Search(searchRequest)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(searchResults)
	fmt.Println("Data has been successfully exported!")
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
	if g.IRINodeCount() != 2 {
		panic(fmt.Sprintf("expected 2 IRINodes, got %d", g.IRINodeCount()))
	}
}

func demonstrateIndexBehavior(dir string, id uuid.UUID) {
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		panic(err)
	}
	defer repo.Close()

	repo.RegisterIndexHandler(defaultderive.DeriveInfo())

	stage := repo.OpenStage(sst.DefaultTriplexMode)
	graph := stage.CreateNamedGraph(sst.IRI(id.URN()))

	mainNode := graph.CreateIRINode("main")
	mainNode.AddStatement(rdf.Type, lci.Organization)
	mainNode.AddStatement(rdfs.Label, sst.String("Test Ltd."))
	stage.Commit(context.TODO(), "first commit: add main code", sst.DefaultBranch)

	searchBleveIndex(repo.Bleve())
	openAndSearchBleve(dir)
	checkoutAndVerify(dir, id)
}
