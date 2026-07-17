// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_IRIStorageWithLocalFullRepository(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	testIRI := "http://example.com/test/dataset"

	defer os.RemoveAll(dir)

	t.Run("create_dataset_with_iri", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		// Create a stage and named graph with the IRI
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(testIRI))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Dataset"))

		// Commit the changes
		_, _, err = st.Commit(context.TODO(), "Create dataset with IRI", sst.DefaultBranch)
		assert.NoError(t, err)

		// Additional IRI assertions for the named graph
		assert.Equal(t, testIRI, ng.IRI().String(), "Named graph IRI should match original IRI")
		assert.NotEmpty(t, ng.IRI().String(), "Named graph IRI should not be empty")

		fmt.Printf("Original IRI: %s\n", testIRI)
	})

	t.Run("retrieve_iri_via_checkout", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		// Get the dataset back
		ds, err := repo.Dataset(context.TODO(), sst.IRI(testIRI))
		assert.NoError(t, err)

		// Verify the dataset IRI method returns the correct value
		assert.Equal(t, sst.IRI(testIRI), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Checkout the branch to get a stage
		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph from the stage
		ng := st.NamedGraph(sst.IRI(testIRI))
		assert.NotNil(t, ng)

		// Verify the named graph IRI matches the original
		assert.Equal(t, testIRI, ng.IRI().String())

		fmt.Printf("Dataset IRI: %s\n", ds.IRI())
		fmt.Printf("Retrieved named graph IRI: %s\n", ng.IRI())
	})

	t.Run("test_with_version_4_uuid", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		// Create a Version 4 UUID (random)
		version4UUID := uuid.New()

		// Create a stage and named graph with the Version 4 UUID
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(version4UUID.URN()))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Version 4 Dataset"))

		// Commit the changes
		_, _, err = st.Commit(context.TODO(), "Create dataset with Version 4 UUID", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset back
		ds, err := repo.Dataset(context.TODO(), sst.IRI(version4UUID.URN()))
		assert.NoError(t, err)
		assert.Equal(t, sst.IRI(version4UUID.URN()), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Checkout the branch to get a stage
		st2, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph from the stage
		ng2 := st2.NamedGraph(sst.IRI(version4UUID.URN()))
		assert.NotNil(t, ng2)

		// For Version 4 UUIDs, the IRI should be the URN-UUID
		assert.Equal(t, version4UUID.URN(), ng2.IRI().String())

		fmt.Printf("Version 4 UUID: %s\n", version4UUID)
		fmt.Printf("Named graph IRI for Version 4: %s\n", ng2.IRI())
	})
}

func Test_IRIStorageWithNamedGraph(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	testIRI := "http://example.com/test/namedgraph"

	defer os.RemoveAll(dir)

	t.Run("create_namedgraph_with_iri", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		// Create a stage and named graph with the IRI
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(testIRI))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Named Graph"))

		// Commit the changes
		_, _, err = st.Commit(context.TODO(), "Create named graph with IRI", sst.DefaultBranch)
		assert.NoError(t, err)

		fmt.Printf("Created named graph with IRI: %s\n", testIRI)
	})

	t.Run("retrieve_namedgraph_and_verify_iri", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		// Get the dataset for the named graph
		ds, err := repo.Dataset(context.TODO(), sst.IRI(testIRI))
		assert.NoError(t, err)
		assert.Equal(t, sst.IRI(testIRI), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Checkout the branch
		st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph
		ng := st.NamedGraph(sst.IRI(testIRI))
		assert.NotNil(t, ng)

		// Verify the IRI matches the original
		assert.Equal(t, testIRI, ng.IRI().String())

		fmt.Printf("Retrieved named graph with IRI: %s\n", ng.IRI())
	})
}

func Test_IRIStorageEdgeCases(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)

	defer os.RemoveAll(dir)

	t.Run("test_with_nil_uuid", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		// Create a nil UUID (Version 0)
		nilUUID := uuid.Nil

		// Create a stage and named graph with the nil UUID
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(nilUUID.URN()))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Nil UUID Dataset"))

		// Commit the changes
		_, _, err = st.Commit(context.TODO(), "Create dataset with nil UUID", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset back
		ds, err := repo.Dataset(context.TODO(), sst.IRI(nilUUID.URN()))
		assert.NoError(t, err)
		assert.Equal(t, sst.IRI(nilUUID.URN()), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Checkout the branch to get a stage
		st2, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph from the stage
		ng2 := st2.NamedGraph(sst.IRI(nilUUID.URN()))
		assert.NotNil(t, ng2)

		// For nil UUID, the IRI should be the URN
		assert.Equal(t, nilUUID.URN(), ng2.IRI().String())

		fmt.Printf("Nil UUID: %s\n", nilUUID)
		fmt.Printf("Named graph IRI for nil UUID: %s\n", ng2.IRI())
	})

	t.Run("test_with_version_5_uuid_empty_iri", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		if err != nil {
			panic(err)
		}
		defer repo.Close()

		// Create a Version 5 UUID with empty IRI
		emptyIRI := ""
		version5UUID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(emptyIRI))

		// Create a stage and named graph with the Version 5 UUID
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(version5UUID.URN()))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Empty IRI Dataset"))

		// Commit the changes
		_, _, err = st.Commit(context.TODO(), "Create dataset with empty IRI", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset back
		ds, err := repo.Dataset(context.TODO(), sst.IRI(version5UUID.URN()))
		assert.NoError(t, err)
		assert.Equal(t, sst.IRI(version5UUID.URN()), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Checkout the branch to get a stage
		st2, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph from the stage
		ng2 := st2.NamedGraph(sst.IRI(version5UUID.URN()))
		assert.NotNil(t, ng2)

		// For Version 5 UUID with empty IRI, the IRI should be the URN
		assert.Equal(t, version5UUID.URN(), ng2.IRI().String())

		fmt.Printf("Version 5 UUID with empty IRI: %s\n", version5UUID)
		fmt.Printf("Named graph IRI for empty IRI: %s\n", ng2.IRI())
	})
}

// ============================================
// RemoteRepository IRI Storage Tests
// ============================================

func Test_IRIStorageWithRemoteRepository(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	testIRI := "http://example.com/test/dataset"
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	t.Run("create_dataset_with_iri_remote", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Create a stage and named graph with the IRI
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(testIRI))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Dataset"))

		// Commit the changes
		_, _, err = st.Commit(constructCtx, "Create dataset with IRI", sst.DefaultBranch)
		assert.NoError(t, err)

		// Additional IRI assertions for the named graph
		assert.Equal(t, testIRI, ng.IRI().String(), "Named graph IRI should match original IRI")
		assert.NotEmpty(t, ng.IRI().String(), "Named graph IRI should not be empty")

		fmt.Printf("Original IRI: %s\n", testIRI)
	})

	t.Run("retrieve_iri_via_checkout_remote", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Get the dataset back
		ds, err := repo.Dataset(constructCtx, sst.IRI(testIRI))
		assert.NoError(t, err)
		assert.Equal(t, sst.IRI(testIRI), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Verify the dataset IRI method returns the correct value
		assert.Equal(t, testIRI, ds.IRI().String())

		// Checkout the branch to get a stage
		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph from the stage
		ng := st.NamedGraph(sst.IRI(testIRI))
		assert.NotNil(t, ng)

		// Verify the named graph IRI matches the original
		assert.Equal(t, testIRI, ng.IRI().String())

		fmt.Printf("Dataset IRI: %s\n", ds.IRI())
		fmt.Printf("Retrieved named graph IRI: %s\n", ng.IRI())
	})

	t.Run("test_with_version_4_uuid_remote", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Create a Version 4 UUID (random)
		version4UUID := uuid.New()

		// Create a stage and named graph with the Version 4 UUID
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(version4UUID.URN()))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Version 4 Dataset"))

		// Commit the changes
		_, _, err = st.Commit(constructCtx, "Create dataset with Version 4 UUID", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset back
		ds, err := repo.Dataset(constructCtx, sst.IRI(version4UUID.URN()))
		assert.NoError(t, err)
		assert.Equal(t, sst.IRI(version4UUID.URN()), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Checkout the branch to get a stage
		st2, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph from the stage
		ng2 := st2.NamedGraph(sst.IRI(version4UUID.URN()))
		assert.NotNil(t, ng2)

		// For Version 4 UUIDs, the IRI should be the URN-UUID
		assert.Equal(t, version4UUID.URN(), ng2.IRI().String())

		fmt.Printf("Version 4 UUID: %s\n", version4UUID)
		fmt.Printf("Named graph IRI for Version 4: %s\n", ng2.IRI())
	})
}

func Test_IRIStorageWithRemoteNamedGraph(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	testIRI := "http://example.com/test/namedgraph"
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	t.Run("create_namedgraph_with_iri_remote", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Create a stage and named graph with the IRI
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(testIRI))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Named Graph"))

		// Commit the changes
		_, _, err = st.Commit(constructCtx, "Create named graph with IRI", sst.DefaultBranch)
		assert.NoError(t, err)

		fmt.Printf("Created named graph with IRI: %s\n", testIRI)
	})

	t.Run("retrieve_namedgraph_and_verify_iri_remote", func(t *testing.T) {
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Get the dataset for the named graph
		ds, err := repo.Dataset(constructCtx, sst.IRI(testIRI))
		assert.NoError(t, err)
		assert.Equal(t, sst.IRI(testIRI), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Checkout the branch
		st, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph
		ng := st.NamedGraph(sst.IRI(testIRI))
		assert.NotNil(t, ng)

		// Verify the IRI matches the original
		assert.Equal(t, testIRI, ng.IRI().String())

		fmt.Printf("Retrieved named graph with IRI: %s\n", ng.IRI())
	})
}

func Test_IRIStorageEdgeCasesRemote(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	t.Run("test_with_nil_uuid_remote", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Create a nil UUID (Version 0)
		nilUUID := uuid.Nil

		// Create a stage and named graph with the nil UUID
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(nilUUID.URN()))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Nil UUID Dataset"))

		// Commit the changes
		_, _, err = st.Commit(constructCtx, "Create dataset with nil UUID", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset back
		ds, err := repo.Dataset(constructCtx, sst.IRI(nilUUID.URN()))
		assert.NoError(t, err)
		assert.Equal(t, sst.IRI(nilUUID.URN()), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Checkout the branch to get a stage
		st2, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph from the stage
		ng2 := st2.NamedGraph(sst.IRI(nilUUID.URN()))
		assert.NotNil(t, ng2)

		// For nil UUID, the IRI should be the URN
		assert.Equal(t, nilUUID.URN(), ng2.IRI().String())

		fmt.Printf("Nil UUID: %s\n", nilUUID)
		fmt.Printf("Named graph IRI for nil UUID: %s\n", ng2.IRI())
	})

	t.Run("test_with_version_5_uuid_empty_iri_remote", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Create a Version 5 UUID with empty IRI
		emptyIRI := ""
		version5UUID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(emptyIRI))

		// Create a stage and named graph with the Version 5 UUID
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(version5UUID.URN()))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Empty IRI Dataset"))

		// Commit the changes
		_, _, err = st.Commit(constructCtx, "Create dataset with empty IRI", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset back
		ds, err := repo.Dataset(constructCtx, sst.IRI(version5UUID.URN()))
		assert.NoError(t, err)
		assert.Equal(t, sst.IRI(version5UUID.URN()), ds.IRI(), "Dataset IRI should match the requested IRI")

		// Checkout the branch to get a stage
		st2, err := ds.CheckoutBranch(constructCtx, sst.DefaultBranch, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		// Get the named graph from the stage
		ng2 := st2.NamedGraph(sst.IRI(version5UUID.URN()))
		assert.NotNil(t, ng2)

		// For Version 5 UUID with empty IRI, the IRI should be the URN
		assert.Equal(t, version5UUID.URN(), ng2.IRI().String())

		fmt.Printf("Version 5 UUID with empty IRI: %s\n", version5UUID)
		fmt.Printf("Named graph IRI for empty IRI: %s\n", ng2.IRI())
	})
}

// ============================================
// IRI Storage Tests for Flat and Basic Repositories
// ============================================

func Test_IRIStorageWithLocalFlatRepository(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	testIRI := "http://example.com/test/flatdataset"

	defer os.RemoveAll(dir)

	t.Run("create_and_verify_iri_flat", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalFlatRepository(dir)
		require.NoError(t, err)
		defer repo.Close()

		// Create a stage and named graph with the IRI
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(testIRI))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Flat Dataset"))

		// Commit the changes
		_, _, err = st.Commit(context.TODO(), "Create flat dataset with IRI", sst.DefaultBranch)
		assert.NoError(t, err)

		// Verify the named graph IRI
		assert.Equal(t, testIRI, ng.IRI().String(), "Named graph IRI should match original IRI")

		fmt.Printf("Created flat dataset with IRI: %s\n", testIRI)
	})

	t.Run("retrieve_iri_from_file_flat", func(t *testing.T) {
		repo, err := sst.OpenLocalFlatRepository(dir)
		require.NoError(t, err)
		defer repo.Close()

		// Get the dataset back using the IRI
		ds, err := repo.Dataset(context.TODO(), sst.IRI(testIRI))
		assert.NoError(t, err)

		// Verify the dataset IRI is correctly read from the SST file
		assert.Equal(t, sst.IRI(testIRI), ds.IRI(), "Dataset IRI should be read from SST file")

		fmt.Printf("Retrieved flat dataset IRI: %s\n", ds.IRI())
	})

	t.Run("test_with_version_4_uuid_flat", func(t *testing.T) {
		repo, err := sst.OpenLocalFlatRepository(dir)
		require.NoError(t, err)
		defer repo.Close()

		// Create a Version 4 UUID (random)
		version4UUID := uuid.New()
		version4IRI := version4UUID.URN()

		// Create a stage and named graph with the Version 4 UUID
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(version4IRI))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Version 4 Dataset"))

		// Commit the changes
		_, _, err = st.Commit(context.TODO(), "Create flat dataset with Version 4 UUID", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset back
		ds, err := repo.Dataset(context.TODO(), sst.IRI(version4IRI))
		assert.NoError(t, err)

		// For Version 4 UUIDs, the IRI should be the URN
		assert.Equal(t, sst.IRI(version4IRI), ds.IRI(), "Dataset IRI should match the URN for Version 4 UUID")

		fmt.Printf("Version 4 UUID: %s\n", version4UUID)
		fmt.Printf("Dataset IRI for Version 4: %s\n", ds.IRI())
	})
}

func Test_IRIStorageWithLocalBasicRepository(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	testIRI := "http://example.com/test/basicdataset"

	defer os.RemoveAll(dir)

	t.Run("create_and_verify_iri_basic", func(t *testing.T) {
		removeFolder(dir)

		// Create a basic repository (revisionHistory=false)
		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
		require.NoError(t, err)
		defer repo.Close()

		// Create a stage and named graph with the IRI
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(testIRI))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Basic Dataset"))

		// Commit the changes
		_, _, err = st.Commit(context.TODO(), "Create basic dataset with IRI", sst.DefaultBranch)
		assert.NoError(t, err)

		// Verify the named graph IRI
		assert.Equal(t, testIRI, ng.IRI().String(), "Named graph IRI should match original IRI")

		fmt.Printf("Created basic dataset with IRI: %s\n", testIRI)
	})

	t.Run("retrieve_iri_from_db_basic", func(t *testing.T) {
		// Open the basic repository
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		// Get the dataset back using the IRI
		ds, err := repo.Dataset(context.TODO(), sst.IRI(testIRI))
		assert.NoError(t, err)

		// Verify the dataset IRI is correctly read from the database/SST file
		assert.Equal(t, sst.IRI(testIRI), ds.IRI(), "Dataset IRI should be read from SST file in DB")

		fmt.Printf("Retrieved basic dataset IRI: %s\n", ds.IRI())
	})

	t.Run("test_with_version_4_uuid_basic", func(t *testing.T) {
		// Open the basic repository
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		// Create a Version 4 UUID (random)
		version4UUID := uuid.New()
		version4IRI := version4UUID.URN()

		// Create a stage and named graph with the Version 4 UUID
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(sst.IRI(version4IRI))

		// Add some content to the named graph
		mainNode := ng.CreateIRINode("main", sst.IRI("http://www.w3.org/2002/07/owl#Class"))
		mainNode.AddStatement(sst.IRI("http://www.w3.org/2000/01/rdf-schema#label"), sst.String("Test Version 4 Dataset"))

		// Commit the changes
		_, _, err = st.Commit(context.TODO(), "Create basic dataset with Version 4 UUID", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset back
		ds, err := repo.Dataset(context.TODO(), sst.IRI(version4IRI))
		assert.NoError(t, err)

		// For Version 4 UUIDs, the IRI should be the URN
		assert.Equal(t, sst.IRI(version4IRI), ds.IRI(), "Dataset IRI should match the URN for Version 4 UUID")

		fmt.Printf("Version 4 UUID: %s\n", version4UUID)
		fmt.Printf("Dataset IRI for Version 4: %s\n", ds.IRI())
	})
}
