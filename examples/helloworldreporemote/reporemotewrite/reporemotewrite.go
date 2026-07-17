// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

/*
HelloWorldRepoRemoteWrite is an example SST application that shows how to connect
to a remote gRPC repository server, create a NamedGraph, add RDF data, commit it,
and manage branches on the remote repository.
Note: This example requires running examples/helloworldreporemote/main.go to start the remote repository server before executing this code.
*/
package main

import (
	"context"
	"fmt"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	iri := uuid.MustParse("77239c1d-228f-44cc-b225-52abc635d7b7").URN()

	transportCreds := grpc.WithTransportCredentials(insecure.NewCredentials())

	// open the remote repository
	repo, err := sst.OpenRemoteRepository(context.TODO(), "localhost:5581", transportCreds)
	if err != nil {
		panic("OpenRepository failed")
	}
	defer repo.Close()

	stage := repo.OpenStage(sst.DefaultTriplexMode)

	graph := stage.CreateNamedGraph(sst.IRI(iri))

	main := graph.CreateIRINode("main")

	main.AddStatement(rdf.Type, lci.Organization)
	main.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
	commitid, _, _ := stage.Commit(context.TODO(), "first commit: add main code", sst.DefaultBranch)

	ds, err := repo.Dataset(context.TODO(), sst.IRI(iri))
	if err != nil {
		panic(err)
	}

	err = ds.SetBranchCommit(context.TODO(), commitid, "testbranch")
	if err != nil {
		panic(err)
	}
	fmt.Println("id:", iri)
}
