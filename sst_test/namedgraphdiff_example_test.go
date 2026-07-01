// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"text/tabwriter"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
)

func printDiffTriples(diffTriples []sst.DiffTriple) {
	fmt.Println("=== Diff Result ===")
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for i := range diffTriples {
		flagStr := "="
		if diffTriples[i].Flag < 0 {
			flagStr = "-"
		}
		if diffTriples[i].Flag > 0 {
			flagStr = "+"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", flagStr, diffTriples[i].Sub, diffTriples[i].Pred, diffTriples[i].Obj)
	}
	tw.Flush()
}

func TestNamedGraphDiffWithCommits(t *testing.T) {
	ctx := context.Background()

	// 1. Create local repository (directory must not exist)
	dir := t.TempDir() + "/repo"
	repo, err := sst.CreateLocalRepository(dir, "test@example.com", "Test", true)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	// ====== V1: create schema + product(import schema) + PartA ======
	stage1 := repo.OpenStage(sst.DefaultTriplexMode)
	schema := stage1.CreateNamedGraph(sst.IRI("http://example.com/schema"))
	product := stage1.CreateNamedGraph(sst.IRI("http://example.com/products"))
	product.AddImport(schema)

	// schema content
	typeDef := schema.CreateIRINode("PartType")
	typeDef.AddStatement(rdf.Type, lci.Class)

	// product content
	partA := product.CreateIRINode("PartA")
	partA.AddStatement(rdf.Type, lci.Class)
	partA.AddStatement(rdfs.Label, sst.String("Gear Housing"))

	// Commit v1 (dataset auto-created)
	hash1, _, _ := stage1.Commit(ctx, "v1: schema + PartA", "master")
	fmt.Printf("V1 commit: %s\n", hash1)

	// get dataset
	ds, err := repo.Dataset(ctx, sst.IRI("http://example.com/products"))
	if err != nil {
		t.Fatalf("failed to get dataset: %v", err)
	}

	// ====== V2: checkout -> modify schema content + add PartB ======
	stage2, err := ds.CheckoutCommit(ctx, hash1, sst.DefaultTriplexMode)
	if err != nil {
		t.Fatalf("checkout v1 failed: %v", err)
	}
	schema2 := stage2.NamedGraph(sst.IRI("http://example.com/schema"))
	product2 := stage2.NamedGraph(sst.IRI("http://example.com/products"))

	// modify schema: add a new type definition
	typeDef2 := schema2.CreateIRINode("BearingType")
	typeDef2.AddStatement(rdf.Type, lci.Class)

	// product add new content
	partB := product2.CreateIRINode("PartB")
	partB.AddStatement(rdf.Type, lci.Class)
	partB.AddStatement(rdfs.Label, sst.String("Bearing"))

	// Commit v2
	hash2, _, _ := stage2.Commit(ctx, "v2: updated schema + PartB", "master")
	fmt.Printf("V2 commit: %s\n\n", hash2)

	// ====== diff two commits' product NamedGraph ======
	stageFrom, _ := ds.CheckoutCommit(ctx, hash1, sst.DefaultTriplexMode)
	ngFrom := stageFrom.NamedGraph(sst.IRI("http://example.com/products"))

	stageTo, _ := ds.CheckoutCommit(ctx, hash2, sst.DefaultTriplexMode)
	ngTo := stageTo.NamedGraph(sst.IRI("http://example.com/products"))

	diffTriples, err := ngFrom.Diff(ngTo)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}

	printDiffTriples(diffTriples)
}

func TestNamedGraphDiffAddImport(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir() + "/repo_add"
	repo, err := sst.CreateLocalRepository(dir, "test@example.com", "Test", true)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	// V1: product without any imports
	stage1 := repo.OpenStage(sst.DefaultTriplexMode)
	product := stage1.CreateNamedGraph(sst.IRI("http://example.com/products"))
	partA := product.CreateIRINode("PartA")
	partA.AddStatement(rdf.Type, lci.Class)
	hash1, _, _ := stage1.Commit(ctx, "v1: product without imports", "master")
	fmt.Printf("V1 commit: %s\n", hash1)

	ds, err := repo.Dataset(ctx, sst.IRI("http://example.com/products"))
	if err != nil {
		t.Fatalf("failed to get dataset: %v", err)
	}

	// V2: add schema import to product
	stage2, _ := ds.CheckoutCommit(ctx, hash1, sst.DefaultTriplexMode)
	schema := stage2.CreateNamedGraph(sst.IRI("http://example.com/schema"))
	product2 := stage2.NamedGraph(sst.IRI("http://example.com/products"))
	product2.AddImport(schema)
	hash2, _, _ := stage2.Commit(ctx, "v2: product with schema import", "master")
	fmt.Printf("V2 commit: %s\n\n", hash2)

	// diff
	stageFrom, err := ds.CheckoutCommit(ctx, hash1, sst.DefaultTriplexMode)
	if err != nil {
		t.Fatalf("checkout v1 for diff failed: %v", err)
	}
	ngFrom := stageFrom.NamedGraph(sst.IRI("http://example.com/products"))
	stageTo, err := ds.CheckoutCommit(ctx, hash2, sst.DefaultTriplexMode)
	if err != nil {
		t.Fatalf("checkout v2 for diff failed: %v", err)
	}
	ngTo := stageTo.NamedGraph(sst.IRI("http://example.com/products"))

	diffTriples, err := ngFrom.Diff(ngTo)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}

	printDiffTriples(diffTriples)
}

func TestNamedGraphDiffRemoveImport(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir() + "/repo_remove"
	repo, err := sst.CreateLocalRepository(dir, "test@example.com", "Test", true)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	defer repo.Close()

	// V1: product with schema import
	stage1 := repo.OpenStage(sst.DefaultTriplexMode)
	schema := stage1.CreateNamedGraph(sst.IRI("http://example.com/schema"))
	product := stage1.CreateNamedGraph(sst.IRI("http://example.com/products"))
	product.AddImport(schema)
	partA := product.CreateIRINode("PartA")
	partA.AddStatement(rdf.Type, lci.Class)
	hash1, _, err := stage1.Commit(ctx, "v1: product with schema import", "master")
	if err != nil {
		t.Fatalf("commit v1 failed: %v", err)
	}
	fmt.Printf("V1 commit: %s\n", hash1)

	ds, err := repo.Dataset(ctx, sst.IRI("http://example.com/products"))
	if err != nil {
		t.Fatalf("failed to get dataset: %v", err)
	}

	// V2: remove schema import from product
	stage2, _ := ds.CheckoutCommit(ctx, hash1, sst.DefaultTriplexMode)
	product2 := stage2.NamedGraph(sst.IRI("http://example.com/products"))
	schemaOld := stage2.NamedGraph(sst.IRI("http://example.com/schema"))
	if schemaOld == nil {
		t.Fatalf("schemaOld is nil")
	}
	if err := product2.RemoveImport(schemaOld); err != nil {
		t.Fatalf("RemoveImport failed: %v", err)
	}
	hash2, _, err := stage2.Commit(ctx, "v2: product without schema import", "master")
	if err != nil {
		t.Fatalf("commit v2 failed: %v", err)
	}
	fmt.Printf("V2 commit: %s\n\n", hash2)

	// diff
	stageFrom, err := ds.CheckoutCommit(ctx, hash1, sst.DefaultTriplexMode)
	if err != nil {
		t.Fatalf("checkout v1 for diff failed: %v", err)
	}
	ngFrom := stageFrom.NamedGraph(sst.IRI("http://example.com/products"))
	stageTo, err := ds.CheckoutCommit(ctx, hash2, sst.DefaultTriplexMode)
	if err != nil {
		t.Fatalf("checkout v2 for diff failed: %v", err)
	}
	ngTo := stageTo.NamedGraph(sst.IRI("http://example.com/products"))

	diffTriples, err := ngFrom.Diff(ngTo)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}

	printDiffTriples(diffTriples)
}
