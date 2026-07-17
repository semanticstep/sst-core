// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bytes"
	"errors"
	"reflect"
	"unsafe"

	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

const (
	dsPrefix                              = byte(0x0)
	ngPrefix                              = byte(0x1)
	dsLogModDsPrefix                      = byte(0x00)
	dsLogSyncDsPrefix                     = byte(0x10)
	commitDsPrefix                        = byte(0x0)
	dsLogRefDeletedOffset                 = byte(0x08)
	dsLogFirstRefFlag                     = byte(0x00)
	dsLogNoParentRefFlag                  = byte(0x01)
	dsLogSyncDsPrefixStr                  = "\x10"
	dsLogSyncSucceededStatus              = "\x00"
	dsLogSyncPreCommitCondFailureStatus   = "\x01"
	dsLogSyncConcurrentModificationStatus = "\x02"
	logEntryNumSize                       = int(unsafe.Sizeof(uint64(0)))
	// dsIRIPrefix is used to store the original IRI for Version 5 UUIDs
	dsIRIPrefix = byte(0x03)
)

// this is entry in bbolt commit structure indicating a commit revision by branch, tag or Hash for other leaf commits.
type refPrefix byte

const (
	refBranchPrefix = refPrefix(iota)
	_               // RefTagPrefix, not implemented
	refLeafPrefix
)

var (
	keyNamedGraphRevisions = []byte("ngr")
	keyDatasetRevisions    = []byte("dsr")
	keyCommits             = []byte("c")
	keyDatasets            = []byte("ds")
	keyDatasetLog          = []byte("dl")
	dsGraphsSeek           = [len(uuid.UUID{}) + 1]byte{ngPrefix}
	commitKeyAuthor        = []byte("author")
	commitKeyMessage       = []byte("message")
)

var (
	ErrUnsupportedRepository      = errors.New("unsupported repository")
	ErrUnknownRefType             = errors.New("unknown ref type")
	ErrDatasetRevisionNotFound    = errors.New("dataset revision not found")
	ErrNamedGraphRevisionNotFound = errors.New("named graph revision not found")
	ErrConcurrentModification     = errors.New("concurrent modification")
)

type postCommitNotifier interface {
	postCommitNotify()
}

type postCommitNotifierFunc func()

func (n postCommitNotifierFunc) postCommitNotify() { n() }

type preCommitConditionCallback func(stage Stage) (postCommitNotifier, error)

type preCommitConditionGetter interface {
	preCommitCondition() preCommitConditionCallback
}

type postCommitNotificationCallback func(stage Stage, commitID Hash)

type postCommitNotificationGetter interface {
	postCommitNotification() postCommitNotificationCallback
}

// StorageDB provides the bbolt database with the following layout:
// Hash is defined to be Hash256 value.

// ### Bucket structure of Repository
// - bucket "NamedGraphRevisions"->*ngr* contains versioned NamedGraphs
// 	* key is the Hash key of the corresponding standalone binary NG-SST file
// 	* value is a binary NG-SST file

// - bucket "DatasetRevisions"->*dsr* contains versioned Datasets
// 	* key is the Hash key of the _Dataset-revision_
// 	* value is a sub-bucket with the following structure:
// 	1. Default NG Hash hash (cardinality 1)
// 		- key is "\x00"
// 		- value is _NamedGraph-Revision-Hash_
//      In case of a deleted NamedGraph, the Hash256 of an empty array is to be used.
// 	2. Directly imported Dataset UUIDs (one by one) (cardinality 0 to *):
// 		- key is byte '\x00' concatenated with imported _DS-UUID_
// 		- value is _Dataset-Revision-Hash_
// 		This establishes the tree structure for the imported NGs.
// 	3. all (both direct and transitive) DS NGs (one by one) (cardinality 1 to *):
// 		- key is byte '\x01' concatenated with _NG-UUID_
// 		- value is _NamedGraph-Revision-Hash_
//  4. commitHash that created this Dataset Revision
//      - key is "\x02"
//      - value is commitHash

// 		Note. (1) and (2) are the core information while (3) is derived flat list of
// 		_NG-Hashes_ for performance optimization

// - bucket "Datasets"->*ds*
// 	* key is _DS-UUID_
// 	* value is a sub-bucket holding the _DS_ references with the following structure:
// 	- key is one of the following (cardinality 1+):
// 		1. byte '\x00' concatenated with branch name
// 		2. byte '\x01' (reserved) tag name
// 		3. byte '\x02' concatenated with _Commit_Hash_ (i.e. the same as the value) for leaf commit
//      4. byte '\x03' IRI of the dataset for Version 5 UUIDs
// 	- 1, 2, 3 value is latest _Commit_Hash_.
//    4 value is the original IRI for Version 5 UUIDs, or not existed for non-Version 5 UUIDs.
//    Discussion:
//       change the value to DatasetRevisionHash together with either the commit Hash or otherwise
//       identification(How?) of SetBranch or RemoveBranch invocation.
//
//  Note. Key (3) type entries exist only for the leaf commits (the ones that are not used in any
// 		commit as parent commits) and the ones that do not exist with keys (1) or (2)

// - bucket "Commits"->*c* contains commit objects
// 	* key is the Hash key of the _commit_.
// 	* value is a sub-bucket with the following structure:
// 	1. List of committed _DS_ revisions (cardinality 1+)
// 		- key is byte '\x00' concatenated with _DS-UUID_
// 		- value is _DS-Revision-Hash_; see bucket "DatasetRevisions"
// 			concatenated with parent _Commit-Hash_ hash(es) of this Dataset revision (cardinality 0 to *)
// 	2. Commit author (cardinality 1)
// 		- key is "author"
// 		- value is author URI string followed by author date unix time seconds in UTC as big endian encoded 64 bit integer (8 bytes)
// 	3. (reserved, not used yet) Commit committer (cardinality 0 to 1)
// 		- key is "committer"
// 		- value is committer URI string followed by committer date unix time seconds in UTC as big endian encoded 64 bit integer (8 bytes)
// 	4. Commit message (cardinality 1)
// 		- key is "message"
// 		- value is commit message (non empty value)
//  5. (reserved, not used yet) Signature of the commit hash(cardinality 0 to many)
//      - key is the signing person
//      - value is the signature concatenating with date

// - bucket "log" stores the ordered sequence of repository-level operation logs.
//   Each entry corresponds to a RepositoryLogEntry and describes one high-level action
//   that modifies the repository (e.g., commit, branch update, document upload).
//
//   * Key: 8-byte big endian encoded uint64 (sequence number), starting from 1 and strictly increasing.
//           This sequence is unique within a repository and determines the chronological order of events.
//
//   * Value: a sub-bucket representing a single RepositoryLogEntry,
//            stored as a string-to-string mapping (map[string]string).
//            These fields are directly used to populate the `Fields` map in RepositoryLogEntry.
//
//     Required fields (always present):
//     - "type":       Type of event: one of:
//                      - "commit"
//                      - "set_branch"
//                      - "remove_branch"
//                      - "upload_document"
//						- "delete_document"
//                      - "sync_from"
//                      - "admin"

//
//     Optional fields depending on type:
//     - "author":     Email of the user performing the action(for upload-document/set branch/remove branch/admin)
//     - "dataset":    UUID string of the dataset being modified(for set branch/remove branch)
//     - "branch":     Branch name (for commit/set/remove)
//     - "commit_id":  Commit hash (for commit/set-branch)
//     - "message":    Message (for set/remove/upload)
//     - "hash":   SHA256 document hash in hex (for upload-document)
//     - "file_name":  Original uploaded file name (for upload-document)
//     - "mime_type":  MIME type of the document (for upload-document)
//     - "timestamp":  indicating operation time (for upload-document/set branch/remove branch/admin)

// - bucket "document_info" stores metadata for each uploaded document.
//   The metadata describes file identity, type, and authorship.
//
//   * Key: 32-byte SHA-256 hash of the document content, encoded as lowercase hex string (64 chars)
//
//   * Value: a sub-bucket containing the following fields:
//
//     - "file_name":   Original file name (string)
//     - "mime_type":   MIME type (string), e.g., "application/pdf"
//     - "author":      Uploader's email address (string)
//     - "timestamp":   indicating load time

// What this iDToPrefixedKey function does is combine a UUID with a prefix byte
// to produce a slice of bytes that starts with a specific prefix.
func iDToPrefixedKey(id uuid.UUID, prefix byte) []byte {
	key := make([]byte, len(uuid.UUID{})+1)
	key[0] = prefix
	copy(key[1:], id[:])
	return key
}

func bytesToRefKey(b []byte, prefix refPrefix) []byte {
	return bytesToKeyInternal(b, byte(prefix))
}

func bytesToKeyInternal(b []byte, prefix byte) []byte {
	return append([]byte{prefix}, b...)
}

// isMasterBranchRefKey reports whether b is the branch ref key for the
// default (master) branch.
func isMasterBranchRefKey(b []byte) bool {
	return bytes.Equal(b, bytesToRefKey(stringAsImmutableBytes(DefaultBranch), refBranchPrefix))
}

// create datasets sub-bucket with dsID, and if it is already existed, return that
func createDatasetBucketIfNotExists(datasets *bbolt.Bucket, dsID uuid.UUID) (*bbolt.Bucket, error) {
	dsRefs, err := datasets.CreateBucket(dsID[:])
	if err == bbolt.ErrBucketExists { //nolint:errorLint
		return datasets.Bucket(dsID[:]), nil
	} else if err != nil {
		return nil, err
	}
	return dsRefs, nil
}

// stringAsImmutableBytes converts string to an immutable byte slice.
//
// Based on https://groups.google.com/g/golang-nuts/c/Zsfk-VMd_fU/m/O1ru4fO-BgAJ
func stringAsImmutableBytes(s string) []byte {
	const max = 0x7fff0000
	return (*[max]byte)(unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&s)).Data))[:len(s):len(s)]
}

// putDatasetIRI stores the original IRI for a Version 5 UUID in the ds bucket
func putDatasetIRI(dsBucket *bbolt.Bucket, dsID uuid.UUID, iri string) error {
	if dsID.Version() == 5 && iri != "" {
		return dsBucket.Put([]byte{dsIRIPrefix}, stringAsImmutableBytes(iri))
	}
	return nil
}

// getDatasetIRI retrieves the original IRI for a Version 5 UUID from the ds bucket
// For Version 4 UUIDs, returns the URN-UUID
func getDatasetIRI(dsBucket *bbolt.Bucket, dsID uuid.UUID) string {
	if dsID.Version() != 5 {
		return dsID.URN()
	}
	iriBytes := dsBucket.Get([]byte{dsIRIPrefix})
	if iriBytes == nil {
		return ""
	}
	return string(iriBytes)
}
