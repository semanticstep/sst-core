// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// Document storage tests (upload/download)

func TestLocalRepository_DocumentUploadAndDownload(t *testing.T) {
	path := "./testLocalFullRepo_upload_download"
	defer removeFolder(path)

	removeFolder(path)
	repo, err := sst.CreateLocalRepository(path, "default@semanticstep.net", "default", true)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()

	t.Run("uploadThenDownloadSuccessfully", func(t *testing.T) {
		originalContent := []byte("Hello, SST Document Vault!")
		reader := bufio.NewReader(bytes.NewReader(originalContent))

		hash, err := repo.DocumentSet(ctx, "text/plain", reader)
		require.NoError(t, err)

		var downloaded bytes.Buffer
		info, err := repo.Document(ctx, hash, &downloaded)
		require.NoError(t, err)

		assert.Equal(t, originalContent, downloaded.Bytes())
		assert.Equal(t, "text/plain", info.MIMEType)
		assert.Equal(t, "default@semanticstep.net", info.Author)
		assert.WithinDuration(t, time.Now(), info.Timestamp, 3*time.Second)
	})

	t.Run("duplicateUploadProducesSameHash", func(t *testing.T) {
		reader1 := bufio.NewReader(bytes.NewReader([]byte("duplicate")))
		hash1, err := repo.DocumentSet(ctx, "text/plain", reader1)
		require.NoError(t, err)

		reader2 := bufio.NewReader(bytes.NewReader([]byte("duplicate")))
		hash2, err := repo.DocumentSet(ctx, "text/plain", reader2)
		require.NoError(t, err)

		assert.Equal(t, hash1, hash2)
	})

	t.Run("downloadNonexistentHashFails", func(t *testing.T) {
		var fakeHash sst.Hash
		copy(fakeHash[:], bytes.Repeat([]byte{0xAB}, 32))

		var buf bytes.Buffer
		_, err := repo.Document(ctx, fakeHash, &buf)
		assert.Error(t, err)
	})

	t.Run("uploadEmptyFile", func(t *testing.T) {
		reader := bufio.NewReader(bytes.NewReader([]byte{}))
		hash, err := repo.DocumentSet(ctx, "text/plain", reader)
		require.NoError(t, err)

		var downloaded bytes.Buffer
		info, err := repo.Document(ctx, hash, &downloaded)
		require.NoError(t, err)
		assert.Empty(t, downloaded.Bytes())
		assert.Equal(t, "text/plain", info.MIMEType)
	})
}

type myTestProvider struct{}

func (myTestProvider) AuthProvider() {}

func (myTestProvider) Oauth2Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "test-token-1"}, nil
}

func (myTestProvider) Info() (email string, name string, err error) {
	return "test1@semanticstep.net", "Test User", nil
}

func TestRemoteRepository_DocumentUploadAndDownload(t *testing.T) {
	testName := t.Name() + "_RemoteVault"
	dir := filepath.Join("./testdata/" + testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	defer removeFolder(dir)
	t.Run("uploadThenDownloadSuccessfully", func(t *testing.T) {
		removeFolder(dir)

		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), myTestProvider{})
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		originalContent := []byte("Hello from remote repository vault!")
		reader := bufio.NewReader(bytes.NewReader(originalContent))

		hash, err := repo.DocumentSet(ctx, "text/plain", reader)
		require.NoError(t, err)

		var downloaded bytes.Buffer
		info, err := repo.Document(ctx, hash, &downloaded)
		require.NoError(t, err)

		assert.Equal(t, originalContent, downloaded.Bytes())
		assert.Equal(t, "text/plain", info.MIMEType)

		email, _, _ := myTestProvider{}.Info()
		assert.Equal(t, email, info.Author)

		assert.WithinDuration(t, time.Now(), info.Timestamp, 3*time.Second)
	})

	t.Run("duplicateUploadReturnsSameHash", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), myTestProvider{})
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		content := []byte("duplicate-content")
		reader1 := bufio.NewReader(bytes.NewReader(content))
		hash1, err := repo.DocumentSet(ctx, "text/plain", reader1)
		require.NoError(t, err)

		reader2 := bufio.NewReader(bytes.NewReader(content))
		hash2, err := repo.DocumentSet(ctx, "text/plain", reader2)
		require.NoError(t, err)

		assert.Equal(t, hash1, hash2)
	})

	t.Run("uploadEmptyFileAndDownload", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), myTestProvider{})
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		reader := bufio.NewReader(bytes.NewReader([]byte{}))
		hash, err := repo.DocumentSet(ctx, "text/plain", reader)
		require.NoError(t, err)

		var downloaded bytes.Buffer
		info, err := repo.Document(ctx, hash, &downloaded)
		require.NoError(t, err)

		assert.Empty(t, downloaded.Bytes())
		assert.Equal(t, "text/plain", info.MIMEType)
	})

	t.Run("downloadNonExistentHashFails", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), myTestProvider{})
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		var fakeHash sst.Hash
		for i := range fakeHash {
			fakeHash[i] = 0xFF
		}

		var buf bytes.Buffer
		_, err = repo.Document(ctx, fakeHash, &buf)
		assert.Error(t, err)
	})
}

// Document delete tests

func TestLocalRepository_DocumentDelete(t *testing.T) {
	path := "./testLocalFullRepo_document_delete"
	removeFolder(path)
	defer removeFolder(path)

	repo, err := sst.CreateLocalRepository(path, "default@semanticstep.net", "default", true)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()

	// 1. Upload a document
	content := []byte("document to be deleted")
	reader := bufio.NewReader(bytes.NewReader(content))

	hash, err := repo.DocumentSet(ctx, "text/plain", reader)
	require.NoError(t, err)

	// 2. Confirm the document exists
	docs, err := repo.Documents(ctx)
	require.NoError(t, err)

	var found bool
	for _, doc := range docs {
		if doc.Hash == hash {
			found = true
			break
		}
	}
	assert.True(t, found, "document should be found before deletion")

	// 3. Delete the document
	err = repo.DocumentDelete(ctx, hash)
	require.NoError(t, err)

	// 4. Confirm the document is no longer exists
	docsAfter, err := repo.Documents(ctx)
	require.NoError(t, err)
	for _, doc := range docsAfter {
		assert.NotEqual(t, hash, doc.Hash, "deleted document still found in list")
	}

	// 5. Download the deleted document should fail
	var buf bytes.Buffer
	_, err = repo.Document(ctx, hash, &buf)
	assert.Error(t, err)

	t.Run("deleteNonExistentDocument", func(t *testing.T) {
		var fakeHash sst.Hash
		for i := range fakeHash {
			fakeHash[i] = 0xAB
		}
		err := repo.DocumentDelete(ctx, fakeHash)
		assert.Error(t, err)
	})

	t.Run("deleteTwice", func(t *testing.T) {
		content := []byte("to delete twice")
		reader := bufio.NewReader(bytes.NewReader(content))
		hash, err := repo.DocumentSet(ctx, "text/plain", reader)
		require.NoError(t, err)

		err = repo.DocumentDelete(ctx, hash)
		require.NoError(t, err)

		err = repo.DocumentDelete(ctx, hash) // Delete again
		assert.Error(t, err)
	})

	t.Run("deleteWithZeroHash", func(t *testing.T) {
		var zeroHash sst.Hash // all zero hash
		err := repo.DocumentDelete(ctx, zeroHash)
		assert.Error(t, err)
	})
}

func TestRemoteRepository_DocumentDelete(t *testing.T) {
	testName := t.Name() + "_RemoteDelete"
	dir := filepath.Join("./testdata/" + testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	defer removeFolder(dir)

	t.Run("deleteUploadedDocumentSuccessfully", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), myTestProvider{})
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		content := []byte("doc to delete")
		reader := bufio.NewReader(bytes.NewReader(content))
		hash, err := repo.DocumentSet(ctx, "text/plain", reader)
		require.NoError(t, err)

		docs, err := repo.Documents(ctx)
		require.NoError(t, err)

		var found bool
		for _, doc := range docs {
			if doc.Hash == hash {
				found = true
				break
			}
		}
		assert.True(t, found, "uploaded document not found before deletion")

		err = repo.DocumentDelete(ctx, hash)
		require.NoError(t, err)

		docsAfter, err := repo.Documents(ctx)
		require.NoError(t, err)

		for _, doc := range docsAfter {
			assert.NotEqual(t, hash, doc.Hash, "deleted document still present in listing")
		}

		var buf bytes.Buffer
		_, err = repo.Document(ctx, hash, &buf)
		assert.Error(t, err)
	})
}

// Document list tests

func TestLocalRepository_Documents(t *testing.T) {
	ctx := context.Background()

	t.Run("emptyWhenNoUpload", func(t *testing.T) {
		path := "./testLocalFullRepo_documents_empty"
		removeFolder(path)
		defer removeFolder(path)

		repo, err := sst.CreateLocalRepository(path, "nobody@semanticstep.net", "none", true)
		require.NoError(t, err)
		defer repo.Close()

		docs, err := repo.Documents(ctx)
		require.NoError(t, err)
		assert.Len(t, docs, 0)
	})

	t.Run("listAfterUpload", func(t *testing.T) {
		path := "./testLocalFullRepo_documents_single"
		removeFolder(path)
		defer removeFolder(path)

		repo, err := sst.CreateLocalRepository(path, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		originalContent := []byte("Document for listing")
		reader := bufio.NewReader(bytes.NewReader(originalContent))

		hash, err := repo.DocumentSet(ctx, "text/plain", reader)
		require.NoError(t, err)

		docs, err := repo.Documents(ctx)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		doc := docs[0]
		assert.Equal(t, hash, doc.Hash)
		assert.Equal(t, "text/plain", doc.MIMEType)
		assert.Equal(t, "default@semanticstep.net", doc.Author)
		assert.WithinDuration(t, time.Now(), doc.Timestamp, 3*time.Second)
	})

	t.Run("multipleUploadsReturnsAllDocuments", func(t *testing.T) {
		path := "./testLocalFullRepo_documents_multiple"
		removeFolder(path)
		defer removeFolder(path)

		repo, err := sst.CreateLocalRepository(path, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		contents := [][]byte{
			[]byte("doc one"),
			[]byte("doc two"),
			[]byte("doc three"),
		}
		var hashes []sst.Hash
		for _, c := range contents {
			hash, err := repo.DocumentSet(ctx, "text/plain", bufio.NewReader(bytes.NewReader(c)))
			require.NoError(t, err)
			hashes = append(hashes, hash)
		}

		docs, err := repo.Documents(ctx)
		require.NoError(t, err)
		require.Len(t, docs, len(contents))

		for _, expected := range hashes {
			found := false
			for _, doc := range docs {
				if doc.Hash == expected {
					found = true
					break
				}
			}
			assert.True(t, found, "missing document hash %x", expected)
		}
	})
}

func TestRemoteRepository_Documents(t *testing.T) {
	testName := t.Name() + "_RemoteDocuments"
	dir := filepath.Join("./testdata/" + testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	defer removeFolder(dir)

	t.Run("emptyWhenNoUpload", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), myTestProvider{})
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		docs, err := repo.Documents(ctx)
		require.NoError(t, err)
		assert.Len(t, docs, 0)
	})

	t.Run("listAfterSingleUpload", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), myTestProvider{})
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		content := []byte("Remote doc")
		hash, err := repo.DocumentSet(ctx, "text/plain", bufio.NewReader(bytes.NewReader(content)))
		require.NoError(t, err)

		docs, err := repo.Documents(ctx)
		require.NoError(t, err)
		require.Len(t, docs, 1)

		doc := docs[0]
		assert.Equal(t, hash, doc.Hash)
		assert.Equal(t, "text/plain", doc.MIMEType)
		assert.Equal(t, "test1@semanticstep.net", doc.Author)
		assert.WithinDuration(t, time.Now(), doc.Timestamp, 3*time.Second)
	})

	t.Run("multipleUploadsReturnAllDocuments", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), myTestProvider{})
		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		contents := [][]byte{
			[]byte("remote doc one"),
			[]byte("remote doc two"),
			[]byte("remote doc three"),
		}
		var hashes []sst.Hash
		for _, c := range contents {
			hash, err := repo.DocumentSet(ctx, "text/plain", bufio.NewReader(bytes.NewReader(c)))
			require.NoError(t, err)
			hashes = append(hashes, hash)
		}

		docs, err := repo.Documents(ctx)
		require.NoError(t, err)
		require.Len(t, docs, len(contents))

		for _, expected := range hashes {
			found := false
			for _, doc := range docs {
				if doc.Hash == expected {
					found = true
					break
				}
			}
			assert.True(t, found, "missing document hash %x", expected)
		}
	})
}
