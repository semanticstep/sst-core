// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

/*
HelloWorldRepoRemoteRead is an example SST application that shows how to connect
to a remote gRPC repository server, check out an existing dataset, and perform
Bleve full-text search queries over the remote repository's search index.
Note: This example requires running examples/helloworldreporemote/main.go to start the remote repository server before executing this code.
*/
package main

import (
	"context"
	"fmt"

	"github.com/semanticstep/sst-core/sst"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/keyword" // this package is needed for bleve searching
)

// This example will open a remote repository(including bbolt.db and index.bleve) and
// perform some query operations by using the index of repository.
func main() {
	iri := uuid.MustParse("77239c1d-228f-44cc-b225-52abc635d7b7").URN()

	// open the remote repository
	transportCreds := grpc.WithTransportCredentials(insecure.NewCredentials())

	// open the remote repository
	repo, err := sst.OpenRemoteRepository(context.TODO(), "localhost:5581", transportCreds)
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

	ng := st.NamedGraph(sst.IRI(iri))

	ng.Dump()

	var q query.Query
	searchRequestSize := 1024
	filterQuery := ""
	if filterQuery != "" {
		q = bleve.NewQueryStringQuery(filterQuery)
	} else {
		q = bleve.NewMatchAllQuery()
	}
	req := bleve.NewSearchRequestOptions(q, searchRequestSize, 0, false)
	req.Fields = []string{"graphID", "graphURI", "mainType", "nodeCount", "directImport", "partCategory"}

	repoBleve := repo.Bleve()
	sr, err := repoBleve.SearchInContext(context.TODO(), req)

	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v", sr)
	fmt.Println("Data has been successfully exported!")
}
