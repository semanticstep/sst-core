// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"sort"

	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
)

type remoteRepoFS struct {
	ngRevisions map[Hash][]byte
	namedGraphs map[uuid.UUID]Hash
}

var (
	_ fs.FS        = (*remoteRepoFS)(nil)
	_ fs.ReadDirFS = (*remoteRepoFS)(nil)
	_ stageSstFS   = (*remoteRepoFS)(nil)
)

func remoteRepoFsOf(ngRevisions map[Hash][]byte, namedGraphs map[uuid.UUID]Hash) remoteRepoFS {
	return remoteRepoFS{
		ngRevisions: ngRevisions,
		namedGraphs: namedGraphs,
	}
}

func (f remoteRepoFS) Open(name string) (fs.File, error) {
	if name == "." {
		return namedDirFile{name: name}, nil
	}
	nameKey, err := uuid.Parse(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if h, ok := f.namedGraphs[nameKey]; ok {
		var content []byte
		if !h.IsNil() {
			content, ok = f.ngRevisions[h]
			if !ok {
				return nil, &fs.PathError{Op: "open", Path: name, Err: errNamedGraphNotFoundInDataset}
			}
		}
		return byteFileReader{keyedFile: keyedFile{nameKey}, byteReader: bytes.NewReader(content)}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: errNamedGraphNotFoundInDataset}
}

func (f remoteRepoFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." {
		return nil, &fs.PathError{Op: "read dir", Path: name, Err: errDatasetDirectoryNotFound}
	}
	ngIDs := make([]uuid.UUID, 0, len(f.namedGraphs))
	for id := range f.namedGraphs {
		ngIDs = append(ngIDs, id)
	}
	sort.Slice(ngIDs, func(i, j int) bool {
		return bytes.Compare(ngIDs[i][:], ngIDs[j][:]) < 0
	})
	dirEntries := make([]fs.DirEntry, 0, len(ngIDs))
	for _, ngID := range ngIDs {
		h, ok := f.namedGraphs[ngID]
		if !ok {
			return nil, &fs.PathError{Op: "read dir", Path: ngID.String(), Err: errNamedGraphNotFoundInDataset}
		}
		var content []byte
		if !h.IsNil() {
			content, ok = f.ngRevisions[h]
			if !ok {
				return nil, &fs.PathError{Op: "read dir", Path: ngID.String(), Err: errNamedGraphNotFoundInDataset}
			}
		}
		dirEntries = append(dirEntries, dirEntry{nameKey: ngID, size: len(content)})
	}
	return dirEntries, nil
}

func (f remoteRepoFS) OpenFile(name string, flag int, _ fs.FileMode) (fs.File, error) {
	nameKey, err := uuid.Parse(name)
	if err != nil {
		if flag&os.O_TRUNC == 0 {
			return nil, &fs.PathError{Op: "open file", Path: name, Err: err}
		}
		nameKey = uuid.Nil
	}
	var content []byte
	if flag&os.O_TRUNC == 0 {
		return nil, &fs.PathError{Op: "open file", Path: name, Err: errOpenFileMissedTruncFlagUnsupported}
	}
	buffer := bytes.NewBuffer(content)
	return memRevFileReadWriter{
		keyedFile:  keyedFile{nameKey},
		byteBuffer: buffer,
		fs:         f,
	}, nil
}

func (f remoteRepoFS) StageFileSystem() {}

type memRevFileReadWriter struct {
	keyedFile
	*byteBuffer
	fs remoteRepoFS
}

var (
	_ fs.File        = (*datasetFileReadWriter)(nil)
	_ io.ReadCloser  = (*datasetFileReadWriter)(nil)
	_ io.WriteCloser = (*datasetFileReadWriter)(nil)
	_ fs.FileInfo    = (*datasetFileReadWriter)(nil)
)

func (f memRevFileReadWriter) Stat() (fs.FileInfo, error) { return f, nil }
func (f memRevFileReadWriter) Size() int64                { return int64(f.Len()) }

func (f memRevFileReadWriter) Close() error {
	content := f.Bytes()
	hash := sha256.Sum256(content)
	f.fs.ngRevisions[hash] = content
	if f.fs.namedGraphs != nil {
		f.fs.namedGraphs[f.nameKey] = hash
	}
	return nil
}
