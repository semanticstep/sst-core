// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

var (
	keyNamedGraphRevisions = []byte("ngr")
	keyDatasetRevisions    = []byte("dsr")
	keyCommits             = []byte("c")
	keyDatasets            = []byte("ds")
	keyDatasetLog          = []byte("dl")
	commitKeyAuthor        = []byte("author")
	commitKeyMessage       = []byte("message")
	dsLogKeyAuthorDate     = commitKeyAuthor
	dsLogKeyID             = []byte("id")
	dsLogKeyRel            = []byte("rel")
	dsLogKeyCompleted      = []byte("completed")
	errBucketNotExist      = errors.New("bucket not exist")
)

// This test can read a bbolt file and convert all data into a new SST repository.
// "br" parameter can be changed to specify which branch to read from the bbolt file.
// This test will lose all commit history, and only create one commit in the new repository.
func Test_bbolt_version_update(t *testing.T) {
	dir := filepath.Join("./testdata/" + t.Name())
	os.RemoveAll(dir)

	hashNil := sst.HashNil()
	fmt.Printf("HashNil %s\n", hashNil)
	db, err := bbolt.Open(filepath.Join("testdata/Test_LocalFullRepositoryMultipleCommits_NGBImportNGC_ModifyBRepo/bbolt.db"), 0600, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	if err != nil {
		panic(err)
	}
	defer repo.Close()
	defer os.RemoveAll(dir)

	br := sst.DefaultBranch

	st := repo.OpenStage(sst.DefaultTriplexMode)

	err = db.View(func(tx *bbolt.Tx) error {
		// From DS to CommitHash
		bucketDS := tx.Bucket(keyDatasets)
		if bucketDS == nil {
			sst.GlobalLogger.Error("commits bucket not found", zap.Error(errBucketNotExist))
			return errBucketNotExist
		}
		dsIDtoCommitHash := make(map[uuid.UUID]sst.Hash)
		err = bucketDS.ForEach(func(k, v []byte) error {

			sub_DS_bucket := bucketDS.Bucket(k)
			if sub_DS_bucket == nil {
				return fmt.Errorf("expected dataset bucket %s not found", k)
			}

			return sub_DS_bucket.ForEach(func(kk, vv []byte) error {
				if len(kk) > 0 && kk[0] == '\x00' && bytes.Equal(kk[1:], []byte(br)) {
					dsIDtoCommitHash[uuid.Must(uuid.FromBytes(k))] = sst.Hash(vv)
				}
				return nil
			})
		})
		if err != nil {
			panic(err)
		}

		// From CommitHash to DSRHash
		dsIDtoDSRHash := make(map[uuid.UUID]sst.Hash)
		bucketCommit := tx.Bucket(keyCommits)
		if bucketCommit == nil {
			sst.GlobalLogger.Error("commits bucket not found", zap.Error(errBucketNotExist))
			return errBucketNotExist
		}
		for dsID, commitHash := range dsIDtoCommitHash {
			commitB := bucketCommit.Bucket(commitHash[:])
			if commitB == nil {
				sst.GlobalLogger.Error("commit bucket not found", zap.String("commitHash", commitHash.String()))
				return errBucketNotExist
			}
			dsHashBytes := commitB.Get(append([]byte{0x00}, dsID[:]...))
			if dsHashBytes == nil {
				sst.GlobalLogger.Error("dataset hash not found in commit", zap.String("commitHash", commitHash.String()), zap.String("dsID", dsID.String()))
				return errBucketNotExist
			}
			dsIDtoDSRHash[dsID] = sst.BytesToHash(dsHashBytes)
		}

		// From DSRHash to NGRHash
		dsIDtoNGRHash := make(map[uuid.UUID]sst.Hash)
		bucketDSR := tx.Bucket(keyDatasetRevisions)
		if bucketDSR == nil {
			sst.GlobalLogger.Error("commits bucket not found", zap.Error(errBucketNotExist))
			return errBucketNotExist
		}
		for dsID, dsrHash := range dsIDtoDSRHash {
			dsrB := bucketDSR.Bucket(dsrHash[:])
			if dsrB == nil {
				sst.GlobalLogger.Error("dataset revision bucket not found", zap.String("dsrHash", dsrHash.String()))
				return errBucketNotExist
			}
			ngrHashBytes := dsrB.Get([]byte{0x00})
			if ngrHashBytes == nil {
				sst.GlobalLogger.Error("named graph revision hash not found in dataset revision", zap.String("dsrHash", dsrHash.String()), zap.String("dsID", dsID.String()))
				return errBucketNotExist
			}
			dsIDtoNGRHash[dsID] = sst.BytesToHash(ngrHashBytes)
		}

		// read NG to Stage
		bucketNGR := tx.Bucket(keyNamedGraphRevisions)
		if bucketNGR == nil {
			sst.GlobalLogger.Error("commits bucket not found", zap.Error(errBucketNotExist))
			return errBucketNotExist
		}

		for _, ngrHash := range dsIDtoNGRHash {
			ngrB := bucketNGR.Get(ngrHash[:])
			if ngrB == nil {
				sst.GlobalLogger.Error("named graph revision bucket not found", zap.String("ngrHash", ngrHash.String()))
				return errBucketNotExist
			}
			bufferV := bytes.NewBuffer(ngrB)
			reader := bufio.NewReader(bufferV)
			graph, err := sst.SstRead(reader, sst.DefaultTriplexMode)
			if err != nil {
				sst.GlobalLogger.Error("failed to read named graph revision", zap.String("ngrHash", ngrHash.String()), zap.Error(err))
				return err
			}
			_, err = st.MoveAndMerge(context.TODO(), graph.Stage())
			if err != nil {
				sst.GlobalLogger.Error("failed to move and merge named graph revision to stage", zap.String("ngrHash", ngrHash.String()), zap.Error(err))
				return err
			}
		}
		return nil
	})

	influenceDataset, commitHash, err := st.Commit(context.TODO(), "Migrate from bbolt to SST", sst.DefaultBranch)
	if err != nil {
		panic(err)
	}
	fmt.Println("influenceDataset:", influenceDataset)
	fmt.Println("commitHash:", commitHash)

	if err != nil {
		log.Fatal(err)
	}
}
