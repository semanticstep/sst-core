// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"

	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"go.etcd.io/bbolt"
)

type localFullFs struct {
	bucketNgr           *bbolt.Bucket
	dsSortedGraphIDs    []uuid.UUID
	dsSortedGraphHashes []Hash
}

var (
	_ fs.FS         = (*localFullFs)(nil)
	_ fs.ReadDirFS  = (*localFullFs)(nil)
	_ fs.OpenFileFS = (*localFullFs)(nil)
	_ stageSstFS    = (*localFullFs)(nil)
)

func localFullFsOf(
	bucketNgr *bbolt.Bucket, dsSortedGraphIDs []uuid.UUID, dsSortedGraphHashes []Hash,
) localFullFs {
	return localFullFs{
		bucketNgr:           bucketNgr,
		dsSortedGraphIDs:    dsSortedGraphIDs,
		dsSortedGraphHashes: dsSortedGraphHashes,
	}
}

func (f localFullFs) Open(name string) (fs.File, error) {
	if name == "." {
		return namedDirFile{name: name}, nil
	}
	nameKey, err := uuid.Parse(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if idx, found := sortedIDIdx(f.dsSortedGraphIDs, nameKey); found {
		content, err := namedGraphByHash(f.bucketNgr, f.dsSortedGraphHashes[idx][:])
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		return byteFileReader{keyedFile: keyedFile{nameKey}, byteReader: bytes.NewReader(content)}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: errNamedGraphNotFoundInDataset}
}

func (f localFullFs) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." {
		return nil, &fs.PathError{Op: "read dir", Path: name, Err: errDatasetDirectoryNotFound}
	}
	var dirEntries []fs.DirEntry
	for i, nameKey := range f.dsSortedGraphIDs {
		h := f.dsSortedGraphHashes[i]
		content, err := namedGraphByHash(f.bucketNgr, h[:])
		if err != nil {
			return nil, &fs.PathError{Op: "read dir", Path: nameKey.String(), Err: err}
		}
		dirEntries = append(dirEntries, dirEntry{nameKey: nameKey, size: len(content)})
	}
	return dirEntries, nil
}

func (f localFullFs) OpenFile(name string, flag int, _ fs.FileMode) (fs.File, error) {
	nameKey, err := uuid.Parse(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open file", Path: name, Err: err}
	}
	var content []byte
	if flag&os.O_TRUNC == 0 {
		return nil, &fs.PathError{Op: "open file", Path: name, Err: errOpenFileMissedTruncFlagUnsupported}
	}
	buffer := bytes.NewBuffer(content)
	return datasetFileReadWriter{
		keyedFile:  keyedFile{nameKey},
		byteBuffer: buffer,
		fs:         f,
	}, nil
}

func (f localFullFs) StageFileSystem() {}

func namedGraphByHash(namedGraphs *bbolt.Bucket, h []byte) ([]byte, error) {
	actualHash, content := namedGraphs.Cursor().Seek(h)
	if !bytes.Equal(actualHash, h) {
		return nil, errNamedGraphRevisionNotFound
	}
	return content, nil
}

type datasetFileReadWriter struct {
	keyedFile
	*byteBuffer
	fs localFullFs
}

var (
	_ fs.File        = (*datasetFileReadWriter)(nil)
	_ io.ReadCloser  = (*datasetFileReadWriter)(nil)
	_ io.WriteCloser = (*datasetFileReadWriter)(nil)
	_ fs.FileInfo    = (*datasetFileReadWriter)(nil)
)

func (f datasetFileReadWriter) Stat() (fs.FileInfo, error) { return f, nil }
func (f datasetFileReadWriter) Size() int64                { return int64(f.Len()) }

func (f datasetFileReadWriter) Close() error {
	content := f.Bytes()
	h := sha256.Sum256(content)
	if idx, ok := sortedIDIdx(f.fs.dsSortedGraphIDs, f.nameKey); ok {
		f.fs.dsSortedGraphHashes[idx] = h
		actualHash, _ := f.fs.bucketNgr.Cursor().Seek(h[:])
		if !bytes.Equal(actualHash, h[:]) {
			return f.fs.bucketNgr.Put(h[:], content)
		}
		return nil
	}
	panic(ErrNamedGraphNotFound)
}
