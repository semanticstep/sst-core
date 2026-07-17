// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_localFlatRepository_ForDatasets(t *testing.T) {
	tests := []struct {
		name      string
		setupRepo func(t *testing.T) sst.Repository
		wantCount int
		wantIRIs  []sst.IRI
		assertion assert.ErrorAssertionFunc
	}{
		{
			name: "empty_repository",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalFlatRepository(d)
				require.NoError(t, err)
				return repo
			},
			wantCount: 0,
			wantIRIs:  nil,
			assertion: assert.NoError,
		},
		{
			name: "single_dataset",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalFlatRepository(d)
				require.NoError(t, err)
				st := repo.OpenStage(sst.DefaultTriplexMode)
				id := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
				ng := st.CreateNamedGraph(sst.IRI(id.URN()))
				main := ng.CreateIRINode("main")
				main.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st.Commit(context.TODO(), "test", sst.DefaultBranch)
				require.NoError(t, err)
				return repo
			},
			wantCount: 1,
			wantIRIs:  []sst.IRI{"urn:uuid:2be0be85-0e7d-49b2-9b2c-6751ae6883e2"},
			assertion: assert.NoError,
		},
		{
			name: "multiple_datasets",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalFlatRepository(d)
				require.NoError(t, err)
				// Create first dataset
				st1 := repo.OpenStage(sst.DefaultTriplexMode)
				id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
				ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
				main1 := ng1.CreateIRINode("main")
				main1.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st1.Commit(context.TODO(), "test1", sst.DefaultBranch)
				require.NoError(t, err)
				// Create second dataset
				st2 := repo.OpenStage(sst.DefaultTriplexMode)
				id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
				ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
				main2 := ng2.CreateIRINode("main")
				main2.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st2.Commit(context.TODO(), "test2", sst.DefaultBranch)
				require.NoError(t, err)
				return repo
			},
			wantCount: 2,
			assertion: assert.NoError,
		},
		{
			name: "callback_error_stops_iteration",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalFlatRepository(d)
				require.NoError(t, err)
				// Create first dataset
				st1 := repo.OpenStage(sst.DefaultTriplexMode)
				id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
				ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
				main1 := ng1.CreateIRINode("main")
				main1.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st1.Commit(context.TODO(), "test1", sst.DefaultBranch)
				require.NoError(t, err)
				// Create second dataset
				st2 := repo.OpenStage(sst.DefaultTriplexMode)
				id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
				ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
				main2 := ng2.CreateIRINode("main")
				main2.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st2.Commit(context.TODO(), "test2", sst.DefaultBranch)
				require.NoError(t, err)
				return repo
			},
			wantCount: 0,
			assertion: func(t assert.TestingT, err error, _ ...interface{}) bool {
				return assert.Error(t, err) && assert.Equal(t, "stop iteration", err.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo(t)
			defer repo.Close()

			var datasets []sst.Dataset
			err := repo.ForDatasets(context.TODO(), func(ds sst.Dataset) error {
				if tt.name == "callback_error_stops_iteration" && len(datasets) == 0 {
					datasets = append(datasets, ds)
					return errors.New("stop iteration")
				}
				datasets = append(datasets, ds)
				return nil
			})

			tt.assertion(t, err)
			if tt.wantIRIs != nil {
				assert.Equal(t, len(tt.wantIRIs), len(datasets))
				for i, ds := range datasets {
					assert.Equal(t, tt.wantIRIs[i], ds.IRI())
				}
			} else if tt.name != "callback_error_stops_iteration" {
				assert.Len(t, datasets, tt.wantCount)
			}
		})
	}
}

func Test_localFlatFileSystemRepository_ForDatasets(t *testing.T) {
	tests := []struct {
		name      string
		setupRepo func(t *testing.T) sst.Repository
		wantCount int
		assertion assert.ErrorAssertionFunc
	}{
		{
			name: "dict_repository",
			setupRepo: func(t *testing.T) sst.Repository {
				repo, err := sst.OpenLocalFlatFileSystemRepository(fs.DirFS("../vocabularies/dict"))
				require.NoError(t, err)
				return repo
			},
			wantCount: 15,
			assertion: assert.NoError,
		},
		{
			name: "callback_error_stops_iteration",
			setupRepo: func(t *testing.T) sst.Repository {
				repo, err := sst.OpenLocalFlatFileSystemRepository(fs.DirFS("../vocabularies/dict"))
				require.NoError(t, err)
				return repo
			},
			wantCount: 0,
			assertion: func(t assert.TestingT, err error, _ ...interface{}) bool {
				return assert.Error(t, err) && assert.Equal(t, "stop iteration", err.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo(t)
			defer repo.Close()

			var count int
			err := repo.ForDatasets(context.TODO(), func(ds sst.Dataset) error {
				if tt.name == "callback_error_stops_iteration" {
					count++
					return errors.New("stop iteration")
				}
				count++
				return nil
			})

			tt.assertion(t, err)
			if tt.name == "dict_repository" {
				assert.Equal(t, tt.wantCount, count)
			}
		})
	}
}

func Test_localBasicRepository_ForDatasets(t *testing.T) {
	tests := []struct {
		name      string
		setupRepo func(t *testing.T) sst.Repository
		wantCount int
		wantIRIs  []sst.IRI
		assertion assert.ErrorAssertionFunc
	}{
		{
			name: "empty_repository",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", false)
				require.NoError(t, err)
				return repo
			},
			wantCount: 0,
			wantIRIs:  nil,
			assertion: assert.NoError,
		},
		{
			name: "single_dataset",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", false)
				require.NoError(t, err)
				id := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
				st := repo.OpenStage(sst.DefaultTriplexMode)
				ng := st.CreateNamedGraph(sst.IRI(id.URN()))
				main := ng.CreateIRINode("main")
				main.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st.Commit(context.TODO(), "test", sst.DefaultBranch)
				require.NoError(t, err)
				return repo
			},
			wantCount: 1,
			wantIRIs:  []sst.IRI{"urn:uuid:2be0be85-0e7d-49b2-9b2c-6751ae6883e2"},
			assertion: assert.NoError,
		},
		{
			name: "multiple_datasets",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", false)
				require.NoError(t, err)
				// Create first dataset
				st1 := repo.OpenStage(sst.DefaultTriplexMode)
				id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
				ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
				main1 := ng1.CreateIRINode("main")
				main1.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st1.Commit(context.TODO(), "test1", sst.DefaultBranch)
				require.NoError(t, err)
				// Create second dataset
				st2 := repo.OpenStage(sst.DefaultTriplexMode)
				id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
				ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
				main2 := ng2.CreateIRINode("main")
				main2.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st2.Commit(context.TODO(), "test2", sst.DefaultBranch)
				require.NoError(t, err)
				return repo
			},
			wantCount: 2,
			assertion: assert.NoError,
		},
		{
			name: "callback_error_stops_iteration",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", false)
				require.NoError(t, err)
				// Create first dataset
				st1 := repo.OpenStage(sst.DefaultTriplexMode)
				id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
				ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
				main1 := ng1.CreateIRINode("main")
				main1.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st1.Commit(context.TODO(), "test1", sst.DefaultBranch)
				require.NoError(t, err)
				// Create second dataset
				st2 := repo.OpenStage(sst.DefaultTriplexMode)
				id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
				ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
				main2 := ng2.CreateIRINode("main")
				main2.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st2.Commit(context.TODO(), "test2", sst.DefaultBranch)
				require.NoError(t, err)
				return repo
			},
			wantCount: 0,
			assertion: func(t assert.TestingT, err error, _ ...interface{}) bool {
				return assert.Error(t, err) && assert.Equal(t, "stop iteration", err.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo(t)
			defer repo.Close()

			var datasets []sst.Dataset
			err := repo.ForDatasets(context.TODO(), func(ds sst.Dataset) error {
				if tt.name == "callback_error_stops_iteration" && len(datasets) == 0 {
					datasets = append(datasets, ds)
					return errors.New("stop iteration")
				}
				datasets = append(datasets, ds)
				return nil
			})

			tt.assertion(t, err)
			if tt.wantIRIs != nil {
				assert.Equal(t, len(tt.wantIRIs), len(datasets))
				for i, ds := range datasets {
					assert.Equal(t, tt.wantIRIs[i], ds.IRI())
				}
			} else if tt.name != "callback_error_stops_iteration" {
				assert.Len(t, datasets, tt.wantCount)
			}
		})
	}
}

func Test_localFullRepository_ForDatasets(t *testing.T) {
	tests := []struct {
		name      string
		setupRepo func(t *testing.T) sst.Repository
		wantCount int
		wantIRIs  []sst.IRI
		assertion assert.ErrorAssertionFunc
	}{
		{
			name: "empty_repository",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", true)
				require.NoError(t, err)
				return repo
			},
			wantCount: 0,
			wantIRIs:  nil,
			assertion: assert.NoError,
		},
		{
			name: "single_dataset",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", true)
				require.NoError(t, err)
				id := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
				st := repo.OpenStage(sst.DefaultTriplexMode)
				ng := st.CreateNamedGraph(sst.IRI(id.URN()))
				main := ng.CreateIRINode("main")
				main.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st.Commit(context.TODO(), "test", sst.DefaultBranch)
				require.NoError(t, err)
				return repo
			},
			wantCount: 1,
			wantIRIs:  []sst.IRI{"urn:uuid:2be0be85-0e7d-49b2-9b2c-6751ae6883e2"},
			assertion: assert.NoError,
		},
		{
			name: "multiple_datasets",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", true)
				require.NoError(t, err)
				// Create first dataset
				st1 := repo.OpenStage(sst.DefaultTriplexMode)
				id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
				ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
				main1 := ng1.CreateIRINode("main")
				main1.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st1.Commit(context.TODO(), "test1", sst.DefaultBranch)
				require.NoError(t, err)
				// Create second dataset
				st2 := repo.OpenStage(sst.DefaultTriplexMode)
				id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
				ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
				main2 := ng2.CreateIRINode("main")
				main2.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st2.Commit(context.TODO(), "test2", sst.DefaultBranch)
				require.NoError(t, err)
				return repo
			},
			wantCount: 2,
			assertion: assert.NoError,
		},
		{
			name: "callback_error_stops_iteration",
			setupRepo: func(t *testing.T) sst.Repository {
				d := filepath.Join(t.TempDir(), "repo")
				repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", true)
				require.NoError(t, err)
				// Create first dataset
				st1 := repo.OpenStage(sst.DefaultTriplexMode)
				id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
				ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
				main1 := ng1.CreateIRINode("main")
				main1.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st1.Commit(context.TODO(), "test1", sst.DefaultBranch)
				require.NoError(t, err)
				// Create second dataset
				st2 := repo.OpenStage(sst.DefaultTriplexMode)
				id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
				ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
				main2 := ng2.CreateIRINode("main")
				main2.AddStatement(rdf.Type, rep.SchematicPort)
				_, _, err = st2.Commit(context.TODO(), "test2", sst.DefaultBranch)
				require.NoError(t, err)
				return repo
			},
			wantCount: 0,
			assertion: func(t assert.TestingT, err error, _ ...interface{}) bool {
				return assert.Error(t, err) && assert.Equal(t, "stop iteration", err.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setupRepo(t)
			defer repo.Close()

			var datasets []sst.Dataset
			err := repo.ForDatasets(context.TODO(), func(ds sst.Dataset) error {
				if tt.name == "callback_error_stops_iteration" && len(datasets) == 0 {
					datasets = append(datasets, ds)
					return errors.New("stop iteration")
				}
				datasets = append(datasets, ds)
				return nil
			})

			tt.assertion(t, err)
			if tt.wantIRIs != nil {
				assert.Equal(t, len(tt.wantIRIs), len(datasets))
				for i, ds := range datasets {
					assert.Equal(t, tt.wantIRIs[i], ds.IRI())
				}
			} else if tt.name != "callback_error_stops_iteration" {
				assert.Len(t, datasets, tt.wantCount)
			}
		})
	}
}

func Test_localFlatRepository_ForDatasets_testdata(t *testing.T) {
	// Test using the existing testdata
	d, err := filepath.Abs("../sst/testdata/localflatrepository")
	require.NoError(t, err)
	repo, err := sst.OpenLocalFlatRepository(d)
	require.NoError(t, err)
	defer repo.Close()

	var datasets []sst.Dataset
	err = repo.ForDatasets(context.TODO(), func(ds sst.Dataset) error {
		datasets = append(datasets, ds)
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, datasets, 1)
	assert.Equal(t, sst.IRI("http://www.w3.org/ns/shacl"), datasets[0].IRI())
}

// Helper to ensure ForDatasets matches Datasets
func Test_ForDatasets_Consistency_With_Datasets(t *testing.T) {
	t.Run("localFlatRepository", func(t *testing.T) {
		d := filepath.Join(t.TempDir(), "repo")
		repo, err := sst.CreateLocalFlatRepository(d)
		require.NoError(t, err)
		// Create multiple datasets
		st1 := repo.OpenStage(sst.DefaultTriplexMode)
		id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
		ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
		main1 := ng1.CreateIRINode("main")
		main1.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st1.Commit(context.TODO(), "test1", sst.DefaultBranch)
		require.NoError(t, err)
		st2 := repo.OpenStage(sst.DefaultTriplexMode)
		id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
		ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
		main2 := ng2.CreateIRINode("main")
		main2.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st2.Commit(context.TODO(), "test2", sst.DefaultBranch)
		require.NoError(t, err)

		repo.Close()

		repo, err = sst.OpenLocalFlatRepository(d)
		require.NoError(t, err)
		defer repo.Close()

		// Get datasets using Datasets()
		datasetsIRIs, err := repo.Datasets(context.TODO())
		require.NoError(t, err)

		// Get datasets using ForDatasets()
		var forDatasetsIRIs []sst.IRI
		err = repo.ForDatasets(context.TODO(), func(ds sst.Dataset) error {
			forDatasetsIRIs = append(forDatasetsIRIs, ds.IRI())
			return nil
		})
		require.NoError(t, err)

		// Both should return the same number
		assert.Equal(t, len(datasetsIRIs), len(forDatasetsIRIs))
	})

	t.Run("localBasicRepository", func(t *testing.T) {
		d := filepath.Join(t.TempDir(), "repo")
		repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", false)
		require.NoError(t, err)
		defer repo.Close()

		// Create multiple datasets
		st1 := repo.OpenStage(sst.DefaultTriplexMode)
		id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
		ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
		main1 := ng1.CreateIRINode("main")
		main1.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st1.Commit(context.TODO(), "test1", sst.DefaultBranch)
		require.NoError(t, err)

		st2 := repo.OpenStage(sst.DefaultTriplexMode)
		id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
		ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
		main2 := ng2.CreateIRINode("main")
		main2.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st2.Commit(context.TODO(), "test2", sst.DefaultBranch)
		require.NoError(t, err)

		// Get datasets using Datasets()
		datasetsIRIs, err := repo.Datasets(context.TODO())
		require.NoError(t, err)

		// Get datasets using ForDatasets()
		var forDatasetsIRIs []sst.IRI
		err = repo.ForDatasets(context.TODO(), func(ds sst.Dataset) error {
			forDatasetsIRIs = append(forDatasetsIRIs, ds.IRI())
			return nil
		})
		require.NoError(t, err)

		// Both should return the same number
		assert.Equal(t, len(datasetsIRIs), len(forDatasetsIRIs))
	})

	t.Run("localFullRepository", func(t *testing.T) {
		d := filepath.Join(t.TempDir(), "repo")
		repo, err := sst.CreateLocalRepository(d, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		// Create multiple datasets
		st1 := repo.OpenStage(sst.DefaultTriplexMode)
		id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
		ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
		main1 := ng1.CreateIRINode("main")
		main1.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st1.Commit(context.TODO(), "test1", sst.DefaultBranch)
		require.NoError(t, err)

		st2 := repo.OpenStage(sst.DefaultTriplexMode)
		id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
		ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
		main2 := ng2.CreateIRINode("main")
		main2.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st2.Commit(context.TODO(), "test2", sst.DefaultBranch)
		require.NoError(t, err)

		// Get datasets using Datasets()
		datasetsIRIs, err := repo.Datasets(context.TODO())
		require.NoError(t, err)

		// Get datasets using ForDatasets()
		var forDatasetsIRIs []sst.IRI
		err = repo.ForDatasets(context.TODO(), func(ds sst.Dataset) error {
			forDatasetsIRIs = append(forDatasetsIRIs, ds.IRI())
			return nil
		})
		require.NoError(t, err)

		// Both should return the same number
		assert.Equal(t, len(datasetsIRIs), len(forDatasetsIRIs))
	})
}

// ============================================
// Stage ForNamedGraphs and ForReferencedNamedGraphs Tests
// ============================================

func Test_Stage_ForNamedGraphs(t *testing.T) {
	tests := []struct {
		name       string
		setupStage func(t *testing.T) sst.Stage
		wantCount  int
		wantIRIs   []sst.IRI
		assertion  assert.ErrorAssertionFunc
	}{
		{
			name: "empty_stage",
			setupStage: func(t *testing.T) sst.Stage {
				return sst.OpenStage(sst.DefaultTriplexMode)
			},
			wantCount: 0,
			wantIRIs:  nil,
			assertion: assert.NoError,
		},
		{
			name: "single_named_graph",
			setupStage: func(t *testing.T) sst.Stage {
				st := sst.OpenStage(sst.DefaultTriplexMode)
				ng := st.CreateNamedGraph(sst.IRI("http://example.com/graph1"))
				node := ng.CreateIRINode("node1")
				_ = node
				return st
			},
			wantCount: 1,
			wantIRIs:  []sst.IRI{"http://example.com/graph1"},
			assertion: assert.NoError,
		},
		{
			name: "multiple_named_graphs",
			setupStage: func(t *testing.T) sst.Stage {
				st := sst.OpenStage(sst.DefaultTriplexMode)
				// Create first named graph
				ng1 := st.CreateNamedGraph(sst.IRI("http://example.com/graph1"))
				node1 := ng1.CreateIRINode("node1")
				_ = node1
				// Create second named graph
				ng2 := st.CreateNamedGraph(sst.IRI("http://example.com/graph2"))
				node2 := ng2.CreateIRINode("node2")
				_ = node2
				return st
			},
			wantCount: 2,
			wantIRIs:  []sst.IRI{"http://example.com/graph1", "http://example.com/graph2"},
			assertion: assert.NoError,
		},
		{
			name: "callback_error_stops_iteration",
			setupStage: func(t *testing.T) sst.Stage {
				st := sst.OpenStage(sst.DefaultTriplexMode)
				ng1 := st.CreateNamedGraph(sst.IRI("http://example.com/graph1"))
				node1 := ng1.CreateIRINode("node1")
				_ = node1
				ng2 := st.CreateNamedGraph(sst.IRI("http://example.com/graph2"))
				node2 := ng2.CreateIRINode("node2")
				_ = node2
				return st
			},
			wantCount: 0,
			assertion: func(t assert.TestingT, err error, _ ...interface{}) bool {
				return assert.Error(t, err) && assert.Equal(t, "stop iteration", err.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := tt.setupStage(t)
			// Note: ephemeral stages should not be explicitly closed

			var graphs []sst.NamedGraph
			err := st.ForNamedGraphs(func(ng sst.NamedGraph) error {
				if tt.name == "callback_error_stops_iteration" && len(graphs) == 0 {
					graphs = append(graphs, ng)
					return errors.New("stop iteration")
				}
				graphs = append(graphs, ng)
				return nil
			})

			tt.assertion(t, err)
			if tt.wantIRIs != nil {
				assert.Equal(t, len(tt.wantIRIs), len(graphs))
				// Check that all expected IRIs are present (order-independent)
				iriSet := make(map[sst.IRI]bool)
				for _, ng := range graphs {
					iriSet[ng.IRI()] = true
				}
				for _, expectedIRI := range tt.wantIRIs {
					assert.True(t, iriSet[expectedIRI], "Expected IRI %s not found", expectedIRI)
				}
			} else if tt.name != "callback_error_stops_iteration" {
				assert.Len(t, graphs, tt.wantCount)
			}
		})
	}
}

func Test_Stage_ForReferencedNamedGraphs(t *testing.T) {
	tests := []struct {
		name       string
		setupStage func(t *testing.T) sst.Stage
		wantCount  int
		wantIRIs   []sst.IRI
		assertion  assert.ErrorAssertionFunc
	}{
		{
			name: "empty_stage",
			setupStage: func(t *testing.T) sst.Stage {
				return sst.OpenStage(sst.DefaultTriplexMode)
			},
			wantCount: 0,
			wantIRIs:  nil,
			assertion: assert.NoError,
		},
		{
			name: "no_referenced_graphs_only_local",
			setupStage: func(t *testing.T) sst.Stage {
				st := sst.OpenStage(sst.DefaultTriplexMode)
				ng := st.CreateNamedGraph(sst.IRI("http://example.com/graph1"))
				_ = ng.CreateIRINode("node1")
				return st
			},
			wantCount: 0,
			wantIRIs:  nil,
			assertion: assert.NoError,
		},
		{
			name: "referenced_graph_created_via_rdf_type",
			setupStage: func(t *testing.T) sst.Stage {
				st := sst.OpenStage(sst.DefaultTriplexMode)

				// Create a graph with a node
				ng := st.CreateNamedGraph(sst.IRI("http://example.com/graph1"))
				node := ng.CreateIRINode("node1")

				// Add statement with rdf.Type - this references the RDF vocabulary graph
				// which automatically creates it as a referenced graph
				node.AddStatement(rdf.Type, rdf.Property)

				return st
			},
			wantCount: 1,
			wantIRIs:  []sst.IRI{"http://www.w3.org/1999/02/22-rdf-syntax-ns"},
			assertion: assert.NoError,
		},
		{
			name: "two_referenced_graphs_rdf_and_rdfs",
			setupStage: func(t *testing.T) sst.Stage {
				st := sst.OpenStage(sst.DefaultTriplexMode)

				// Create a local graph with nodes that reference both RDF and RDFS vocabularies
				ng := st.CreateNamedGraph(sst.IRI("http://example.com/graph1"))

				// node1 uses rdf.Type
				node1 := ng.CreateIRINode("node1")
				node1.AddStatement(rdf.Type, rdf.Property)

				// node2 uses rdfs.Label
				node2 := ng.CreateIRINode("node2")
				node2.AddStatement(rdfs.Label, sst.String("test label"))

				return st
			},
			wantCount: 2,
			wantIRIs:  []sst.IRI{"http://www.w3.org/1999/02/22-rdf-syntax-ns", "http://www.w3.org/2000/01/rdf-schema"},
			assertion: assert.NoError,
		},
		{
			name: "mixed_local_and_referenced_graphs",
			setupStage: func(t *testing.T) sst.Stage {
				st := sst.OpenStage(sst.DefaultTriplexMode)

				// Create a local graph
				ng1 := st.CreateNamedGraph(sst.IRI("http://example.com/local-graph"))
				node1 := ng1.CreateIRINode("node1")

				// Add statement with rdf.Type - this references the RDF vocabulary graph
				node1.AddStatement(rdf.Type, rdf.Property)

				// Add another statement with rdfs.Label to reference RDFS vocabulary
				node1.AddStatement(rdfs.Label, sst.String("test label"))

				return st
			},
			wantCount: 2,
			wantIRIs:  []sst.IRI{"http://www.w3.org/1999/02/22-rdf-syntax-ns", "http://www.w3.org/2000/01/rdf-schema"},
			assertion: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := tt.setupStage(t)
			// Note: ephemeral stages should not be explicitly closed

			var graphs []sst.NamedGraph
			err := st.ForReferencedNamedGraphs(func(ng sst.NamedGraph) error {
				if tt.name == "callback_error_stops_iteration" && len(graphs) == 0 {
					graphs = append(graphs, ng)
					return errors.New("stop iteration")
				}
				graphs = append(graphs, ng)
				return nil
			})

			tt.assertion(t, err)
			if tt.wantIRIs != nil {
				assert.Equal(t, len(tt.wantIRIs), len(graphs))
				// Check that all expected IRIs are present (order-independent)
				iriSet := make(map[sst.IRI]bool)
				for _, ng := range graphs {
					iriSet[ng.IRI()] = true
				}
				for _, expectedIRI := range tt.wantIRIs {
					assert.True(t, iriSet[expectedIRI], "Expected IRI %s not found", expectedIRI)
				}
			} else {
				assert.Len(t, graphs, tt.wantCount)
			}
		})
	}
}

// Helper test to verify ForNamedGraphs consistency with NamedGraphs()
func Test_Stage_ForNamedGraphs_Consistency(t *testing.T) {
	t.Run("ephemeral_stage", func(t *testing.T) {
		st := sst.OpenStage(sst.DefaultTriplexMode)
		// Note: ephemeral stages should not be explicitly closed

		// Create multiple named graphs
		ng1 := st.CreateNamedGraph(sst.IRI("http://example.com/graph1"))
		_ = ng1.CreateIRINode("node1")

		ng2 := st.CreateNamedGraph(sst.IRI("http://example.com/graph2"))
		_ = ng2.CreateIRINode("node2")

		// Get graphs using NamedGraphs()
		sliceGraphs := st.NamedGraphs()

		// Get graphs using ForNamedGraphs()
		var forGraphs []sst.NamedGraph
		err := st.ForNamedGraphs(func(ng sst.NamedGraph) error {
			forGraphs = append(forGraphs, ng)
			return nil
		})
		require.NoError(t, err)

		// Both should return the same number
		assert.Equal(t, len(sliceGraphs), len(forGraphs))
	})

}

// ============================================
// RemoteRepository ForDatasets Tests
// ============================================

func Test_remoteRepository_ForDatasets(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/" + testName)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	t.Run("empty_repository", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		var datasets []sst.Dataset
		err = repo.ForDatasets(constructCtx, func(ds sst.Dataset) error {
			datasets = append(datasets, ds)
			return nil
		})

		assert.NoError(t, err)
		assert.Len(t, datasets, 0)
	})

	t.Run("single_dataset", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Create a single dataset
		st := repo.OpenStage(sst.DefaultTriplexMode)
		id := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
		ng := st.CreateNamedGraph(sst.IRI(id.URN()))
		main := ng.CreateIRINode("main")
		main.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st.Commit(constructCtx, "test", sst.DefaultBranch)
		require.NoError(t, err)

		var datasets []sst.Dataset
		err = repo.ForDatasets(constructCtx, func(ds sst.Dataset) error {
			datasets = append(datasets, ds)
			return nil
		})

		assert.NoError(t, err)
		assert.Len(t, datasets, 1)
		assert.Equal(t, sst.IRI("urn:uuid:2be0be85-0e7d-49b2-9b2c-6751ae6883e2"), datasets[0].IRI())
	})

	t.Run("multiple_datasets", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Create first dataset
		st1 := repo.OpenStage(sst.DefaultTriplexMode)
		id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
		ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
		main1 := ng1.CreateIRINode("main")
		main1.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st1.Commit(constructCtx, "test1", sst.DefaultBranch)
		require.NoError(t, err)

		// Create second dataset
		st2 := repo.OpenStage(sst.DefaultTriplexMode)
		id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
		ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
		main2 := ng2.CreateIRINode("main")
		main2.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st2.Commit(constructCtx, "test2", sst.DefaultBranch)
		require.NoError(t, err)

		var datasets []sst.Dataset
		err = repo.ForDatasets(constructCtx, func(ds sst.Dataset) error {
			datasets = append(datasets, ds)
			return nil
		})

		assert.NoError(t, err)
		assert.Len(t, datasets, 2)
	})

	t.Run("callback_error_stops_iteration", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Create first dataset
		st1 := repo.OpenStage(sst.DefaultTriplexMode)
		id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
		ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
		main1 := ng1.CreateIRINode("main")
		main1.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st1.Commit(constructCtx, "test1", sst.DefaultBranch)
		require.NoError(t, err)

		// Create second dataset
		st2 := repo.OpenStage(sst.DefaultTriplexMode)
		id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
		ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
		main2 := ng2.CreateIRINode("main")
		main2.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st2.Commit(constructCtx, "test2", sst.DefaultBranch)
		require.NoError(t, err)

		var count int
		err = repo.ForDatasets(constructCtx, func(ds sst.Dataset) error {
			count++
			if count == 1 {
				return errors.New("stop iteration")
			}
			return nil
		})

		assert.Error(t, err)
		assert.Equal(t, "stop iteration", err.Error())
		assert.Equal(t, 1, count)
	})

	t.Run("consistency_with_datasets_method", func(t *testing.T) {
		removeFolder(dir)
		url := testutil.ServerServe(t, dir)
		constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		// Create multiple datasets
		st1 := repo.OpenStage(sst.DefaultTriplexMode)
		id1 := uuid.MustParse("2be0be85-0e7d-49b2-9b2c-6751ae6883e2")
		ng1 := st1.CreateNamedGraph(sst.IRI(id1.URN()))
		main1 := ng1.CreateIRINode("main")
		main1.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st1.Commit(constructCtx, "test1", sst.DefaultBranch)
		require.NoError(t, err)

		st2 := repo.OpenStage(sst.DefaultTriplexMode)
		id2 := uuid.MustParse("8799e0e1-2484-4097-9c10-1301461cf6d1")
		ng2 := st2.CreateNamedGraph(sst.IRI(id2.URN()))
		main2 := ng2.CreateIRINode("main")
		main2.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st2.Commit(constructCtx, "test2", sst.DefaultBranch)
		require.NoError(t, err)

		// Get datasets using Datasets()
		datasetsIRIs, err := repo.Datasets(constructCtx)
		require.NoError(t, err)

		// Get datasets using ForDatasets()
		var forDatasetsIRIs []sst.IRI
		err = repo.ForDatasets(constructCtx, func(ds sst.Dataset) error {
			forDatasetsIRIs = append(forDatasetsIRIs, ds.IRI())
			return nil
		})
		require.NoError(t, err)

		// Both should return the same number
		assert.Equal(t, len(datasetsIRIs), len(forDatasetsIRIs))
	})
}
