// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Metadata represents metadata information of a stored document.
type DocumentInfo struct {
	Hash      Hash      `json:"id"`
	MIMEType  string    `json:"mime_type"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
	Size      int64     `json:"size"`
}

const documentInfoBucket = "document_info"

// writeDocumentInfo stores document info into a sub-bucket under the full hash key.
func writeDocumentInfo(db *bolt.DB, hash Hash, info *DocumentInfo) error {
	key := hash.String()

	return db.Update(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists([]byte(documentInfoBucket))
		if err != nil {
			return fmt.Errorf("failed to create document info root bucket: %w", err)
		}

		sub, err := root.CreateBucketIfNotExists([]byte(key))
		if err != nil {
			return fmt.Errorf("failed to create document info sub-bucket: %w", err)
		}

		if err := sub.Put([]byte("mime_type"), []byte(info.MIMEType)); err != nil {
			return err
		}
		if err := sub.Put([]byte("author"), []byte(info.Author)); err != nil {
			return err
		}
		if err := sub.Put([]byte("timestamp"), []byte(info.Timestamp.UTC().Format(time.RFC3339))); err != nil {
			return err
		}

		return nil
	})
}

// ReadDocumentInfo retrieves document info from the sub-bucket under the full hash key.
func readDocumentInfo(db *bolt.DB, hash Hash) (*DocumentInfo, error) {
	key := hash.String()
	var info DocumentInfo

	err := db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(documentInfoBucket))
		if root == nil {
			return fmt.Errorf("document info root bucket not found")
		}

		sub := root.Bucket([]byte(key))
		if sub == nil {
			return fmt.Errorf("no document info found for hash: %s", key)
		}

		info.Hash = hash
		info.MIMEType = string(sub.Get([]byte("mime_type")))
		info.Author = string(sub.Get([]byte("author")))

		timestampStr := string(sub.Get([]byte("timestamp")))
		t, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return fmt.Errorf("failed to parse document info timestamp: %w", err)
		}
		info.Timestamp = t.UTC()

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &info, nil
}
