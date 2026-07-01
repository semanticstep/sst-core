// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"encoding/base64"
	"errors"

	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
)

var ErrDirectoryExpectedAsBasePath = errors.New("directory expected as base path")

// writeToSstFilesWithID writes this Stage data to the specified directory using the graph UUID as filename.
// It is an internal implementation used by repository code paths that require UUID-based file storage.
//
// This private writer is currently used by local repository implementations that store
// stage content keyed by NamedGraph UUID, such as localBasicRepository, localFullRepository.
func (to *stage) writeToSstFilesWithID(stageDir fs.FS) (err error) {
	return to.writeStageToSstFilesWithIDMatched(stageDir, func(uuid.UUID) bool { return true })
}

// encodeBaseURL encodes a base URL to a safe filename using base64 URL encoding.
// Uses RawURLEncoding to avoid padding '=' characters in the filename.
func encodeBaseURL(baseURL string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(baseURL))
}

// WriteSstFilesDirectory persists the NamedGraphs of this Stage into the
// directory represented by stageDir.
//
// Each modified graph is written to its own file. The file name is the
// base64 URL-safe encoding (without padding) of the graph's IRI base
// portion, i.e. the part before the last '#'. If the IRI contains no '#',
// the whole IRI is encoded. This encoding keeps file names filesystem-safe
// and allows the original base IRI to be recovered.
//
// Parameters:
//   - stageDir: An fs.FS referring to an existing directory where the graph
//     files will be created.
//
// Returns:
//   - err: nil on success, or ErrDirectoryExpectedAsBasePath if stageDir
//     does not denote a directory. I/O failures during graph writing are
//     reported via panic.
func (to *stage) WriteSstFilesDirectory(stageFS fs.FS) (err error) {
	var sfs stageSstFS
	if temp, ok := stageFS.(stageSstFS); ok {
		sfs = temp
	} else {
		sfs = stageDirFS{stageFS}
	}

	stagePathInfo, err := fs.Stat(stageFS, ".")
	if err != nil {
		panic(err)
	}
	if !stagePathInfo.IsDir() {
		return ErrDirectoryExpectedAsBasePath
	}

	graphIRIs := make(map[IRI]struct{})
	for _, ng := range to.NamedGraphs() {
		graphIRIs[ng.IRI()] = struct{}{}
	}

	for graphIRI := range graphIRIs {
		graph := to.NamedGraph(graphIRI)

		if _, ok := to.repo.(*remoteRepository); !ok {
			// not modified, skip
			if !graph.(*namedGraph).flags.modified {
				continue
			}
		}

		// Get base URL from graph IRI and encode it as filename
		baseURL, _ := graph.IRI().Split()
		filename := encodeBaseURL(baseURL)

		err = func() (err error) {
			var graphW fs.WriteFile
			graphW, err = fs.Create(sfs, filename)
			if err != nil {
				panic(err)
			}
			bufW := bufio.NewWriter(graphW)
			defer func() {
				e := bufW.Flush()
				if e != nil {
					if err == nil {
						err = e
					}
					return
				}
				e = graphW.Close()
				if e != nil {
					if err == nil {
						err = e
					}
					return
				}
			}()
			return graph.SstWrite(bufW)
		}()
		if err != nil {
			panic(err)
		}
	}
	return nil
}

func (to *stage) writeStageToSstFilesWithIDMatched(stageFS fs.FS, match func(ngID uuid.UUID) bool) (err error) {
	var sfs stageSstFS
	if temp, ok := stageFS.(stageSstFS); ok {
		sfs = temp
	} else {
		sfs = stageDirFS{stageFS}
	}

	stagePathInfo, err := fs.Stat(stageFS, ".")
	if err != nil {
		panic(err)
	}
	if !stagePathInfo.IsDir() {
		return ErrDirectoryExpectedAsBasePath
	}

	graphIDs := map[uuid.UUID]IRI{}
	for _, ng := range to.NamedGraphs() {
		graphIDs[ng.ID()] = ng.IRI()
	}

	for graphID := range graphIDs {
		if !match(graphID) {
			continue
		}

		graph := to.namedGraphByUUID(graphID)

		if _, ok := to.repo.(*remoteRepository); !ok {
			// not modified, skip
			if !graph.(*namedGraph).flags.modified {
				continue
			}
		}

		err = func() (err error) {
			var graphW fs.WriteFile
			graphW, err = fs.Create(sfs, graphID.String())
			if err != nil {
				panic(err)
			}
			bufW := bufio.NewWriter(graphW)
			defer func() {
				e := bufW.Flush()
				if e != nil {
					if err == nil {
						err = e
					}
					return
				}
				e = graphW.Close()
				if e != nil {
					if err == nil {
						err = e
					}
					return
				}
			}()
			return graph.SstWrite(bufW)
		}()
		if err != nil {
			panic(err)
		}
	}
	return nil
}

// func sstAddNamedGraphImportsRecursively(gi ImportDetails, graphIDs map[uuid.UUID]struct{}) {
// 	graphIDs[gi.ID()] = struct{}{}
// 	for _, di := range gi.Direct() {
// 		sstAddNamedGraphImportsRecursively(di, graphIDs)
// 	}
// }
