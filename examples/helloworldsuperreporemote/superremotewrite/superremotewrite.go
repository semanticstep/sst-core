// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

/*
HelloWorldSuperRepoRemoteWrite is an example SST application that shows how to
connect to a remote gRPC super-repository server with OAuth2 authentication,
retrieve a sub-repository, create a NamedGraph, add RDF data, commit changes,
and manage branches on the remote repository.
Note: This example requires running examples/helloworldsuperreporemote/main.go to start the super remote repository server before executing this code.
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
	iri := uuid.MustParse("512fb4b9-cd58-491b-afbc-a4cf85354e4d").URN()

	transportCreds := grpc.WithTransportCredentials(insecure.NewCredentials())
	// open the remote repository
	superRepo, err := sst.OpenRemoteSuperRepository(context.TODO(), "localhost:5581", transportCreds)
	if err != nil {
		panic("OpenRemoteSuperRepository failed")
	}

	repo, err := superRepo.Get(context.TODO(), "default")
	if err != nil {
		panic("OpenRepository failed")
	}
	defer repo.Close()

	info, err := repo.Info(context.TODO(), sst.DefaultBranch)
	if err != nil {
		panic(err)
	}
	fmt.Println("Repository Info:", info)

	stage := repo.OpenStage(sst.DefaultTriplexMode)

	graph := stage.CreateNamedGraph(sst.IRI(iri))

	main := graph.CreateIRINode("main")

	main.AddStatement(rdf.Type, lci.Organization)
	main.AddStatement(rdfs.Label, sst.String("ABC Ltd."))
	commitid, _, err := stage.Commit(context.TODO(), "first commit: add main code", sst.DefaultBranch)
	if err != nil {
		panic(err)
	}
	_ = commitid

	ds, err := repo.Dataset(context.TODO(), sst.IRI(iri))
	if err != nil {
		panic(err)
	}

	err = ds.SetBranchCommit(context.TODO(), commitid, "testbranch")
	if err != nil {
		panic(err)
	}
	fmt.Println("SetBranch succeed ", commitid.String(), "is set to", "testbranch")

	br, err := ds.Branches(context.TODO())
	if err != nil {
		panic(err)
	}
	fmt.Println(br)

}
