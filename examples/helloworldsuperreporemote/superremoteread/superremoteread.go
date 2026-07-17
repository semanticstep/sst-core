// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

/*
HelloWorldSuperRepoRemoteRead is an example SST application that shows how to
connect to a remote gRPC super-repository server, retrieve a sub-repository by
name, check out a dataset, and dump its contents.
Note: This example requires running examples/helloworldsuperreporemote/main.go to start the super remote repository server before executing this code.
*/
package main

import (
	"context"
	"fmt"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
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

	ds, err := repo.Dataset(context.TODO(), sst.IRI(iri))
	if err != nil {
		panic(err)
	}

	st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
	if err != nil {
		panic(err)
	}

	st.Dump()
	fmt.Println("id:", iri)
}
