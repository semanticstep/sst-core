// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	fs "github.com/relab/wrfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func documentInfoByHash(docs []sst.DocumentInfo) map[sst.Hash]sst.DocumentInfo {
	m := make(map[sst.Hash]sst.DocumentInfo, len(docs))
	for _, d := range docs {
		m[d.Hash] = d
	}
	return m
}

func Test_localFullRepository_ForDocuments(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name      string
		setupRepo func(t *testing.T) sst.Repository
		run       func(t *testing.T, repo sst.Repository)
	}{
		{
			name: "empty_repository",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", true)
				require.NoError(t, err)
				return repo
			},
			run: func(t *testing.T, repo sst.Repository) {
				var got []sst.DocumentInfo
				err := repo.ForDocuments(ctx, func(doc sst.DocumentInfo) error {
					got = append(got, doc)
					return nil
				})
				require.NoError(t, err)
				assert.Len(t, got, 0)
			},
		},
		{
			name: "multiple_documents_all_seen",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", true)
				require.NoError(t, err)
				for _, body := range [][]byte{[]byte("doc a"), []byte("doc b"), []byte("doc c")} {
					_, err := repo.DocumentSet(ctx, "text/plain", bufio.NewReader(bytes.NewReader(body)))
					require.NoError(t, err)
				}
				return repo
			},
			run: func(t *testing.T, repo sst.Repository) {
				docs, err := repo.Documents(ctx)
				require.NoError(t, err)
				byHash := documentInfoByHash(docs)

				var got []sst.DocumentInfo
				err = repo.ForDocuments(ctx, func(doc sst.DocumentInfo) error {
					got = append(got, doc)
					return nil
				})
				require.NoError(t, err)
				require.Len(t, got, len(byHash))
				for _, doc := range got {
					exp, ok := byHash[doc.Hash]
					require.True(t, ok, "unexpected hash in ForDocuments")
					assert.Equal(t, exp, doc)
				}
			},
		},
		{
			name: "callback_error_stops_iteration",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", true)
				require.NoError(t, err)
				for _, body := range [][]byte{[]byte("first"), []byte("second")} {
					_, err := repo.DocumentSet(ctx, "text/plain", bufio.NewReader(bytes.NewReader(body)))
					require.NoError(t, err)
				}
				return repo
			},
			run: func(t *testing.T, repo sst.Repository) {
				var got []sst.DocumentInfo
				err := repo.ForDocuments(ctx, func(doc sst.DocumentInfo) error {
					if len(got) == 0 {
						got = append(got, doc)
						return errors.New("stop iteration")
					}
					got = append(got, doc)
					return nil
				})
				require.Error(t, err)
				assert.Equal(t, "stop iteration", err.Error())
				assert.Len(t, got, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo(t)
			defer repo.Close()
			tt.run(t, repo)
		})
	}
}

func Test_ForDocuments_Consistency_With_Documents(t *testing.T) {
	ctx := context.TODO()
	d := filepath.Join(t.TempDir(), "repo")
	repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", true)
	require.NoError(t, err)
	defer repo.Close()

	for _, body := range [][]byte{[]byte("one"), []byte("two")} {
		_, err := repo.DocumentSet(ctx, "application/octet-stream", bufio.NewReader(bytes.NewReader(body)))
		require.NoError(t, err)
	}

	fromList, err := repo.Documents(ctx)
	require.NoError(t, err)

	var fromFor []sst.DocumentInfo
	err = repo.ForDocuments(ctx, func(doc sst.DocumentInfo) error {
		fromFor = append(fromFor, doc)
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, documentInfoByHash(fromList), documentInfoByHash(fromFor))
}

func Test_remoteRepository_ForDocuments(t *testing.T) {
	dir := filepath.Join("testdata", strings.ReplaceAll(t.Name(), "/", "_"))
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	removeFolder(dir)
	t.Cleanup(func() { removeFolder(dir) })

	url := testutil.ServerServe(t, dir)
	ctx := sstauth.ContextWithAuthProvider(context.TODO(), myTestProvider{})
	repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
	require.NoError(t, err)
	defer repo.Close()

	for _, body := range [][]byte{[]byte("r1"), []byte("r2")} {
		_, err := repo.DocumentSet(ctx, "text/plain", bufio.NewReader(bytes.NewReader(body)))
		require.NoError(t, err)
	}

	fromList, err := repo.Documents(ctx)
	require.NoError(t, err)

	var fromFor []sst.DocumentInfo
	err = repo.ForDocuments(ctx, func(doc sst.DocumentInfo) error {
		fromFor = append(fromFor, doc)
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, documentInfoByHash(fromList), documentInfoByHash(fromFor))
}

func Test_repository_ForDocuments_notSupported(t *testing.T) {
	ctx := context.TODO()

	t.Run("localFlat", func(t *testing.T) {
		d := filepath.Join(t.TempDir(), "flat")
		repo, err := sst.CreateLocalFlatRepository(d)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.ForDocuments(ctx, func(_ sst.DocumentInfo) error { return nil })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ForDocuments")
	})

	t.Run("localBasic", func(t *testing.T) {
		d := filepath.Join(t.TempDir(), "basic")
		repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", false)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.ForDocuments(ctx, func(_ sst.DocumentInfo) error { return nil })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ForDocuments")
	})

	t.Run("localFlatFileSystem", func(t *testing.T) {
		repo, err := sst.OpenLocalFlatFileSystemRepository(fs.DirFS("../vocabularies/dict"))
		require.NoError(t, err)
		defer repo.Close()

		err = repo.ForDocuments(ctx, func(_ sst.DocumentInfo) error { return nil })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ForDocuments")
	})
}
