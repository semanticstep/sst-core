// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maxInitialOrder = 16 * 1024

var testingSenderInquiryBarrier chan struct{} // testing hook

type datasetObjectStream interface {
	Recv() (*bboltproto.DatasetInquiry, error)
	Send(*bboltproto.DatasetObject) error
}

type preSender func() (wantCount int, err error)

type datasetObjectSender struct {
	db               *bbolt.DB
	stream           datasetObjectStream
	inqChan          chan []*bboltproto.DatasetInquiry
	pendingInq       []*bboltproto.DatasetInquiry
	pendingInqOff    int
	recvResult       chan error
	seenRevisions    map[seenEntry]struct{}
	visitedPositions map[Hash]int
}

type datasetObjectMessageSender interface {
	Send(*bboltproto.DatasetObject) error
}

type seenEntryType byte

const (
	seenCommitWithParents = seenEntryType(iota)
	seenCommitWithNoParents
	seenDatasetRevision
	seenNamedGraphRevision
)

type seenEntry struct {
	hash      Hash
	entryType seenEntryType
}

func newDatasetObjectSender(r Repository, stream datasetObjectStream) (s *datasetObjectSender, err error) {
	if r == nil {
		err = ErrUnsupportedRepository
		return
	}
	db := r.(*localFullRepository).db
	s = &datasetObjectSender{
		db:               db,
		stream:           stream,
		pendingInq:       make([]*bboltproto.DatasetInquiry, 0, 2),
		pendingInqOff:    0,
		recvResult:       make(chan error, 1),
		seenRevisions:    map[seenEntry]struct{}{},
		visitedPositions: make(map[Hash]int, maxInitialOrder),
	}
	return
}

func (s *datasetObjectSender) sendObjects(ctx context.Context, dss *datasetServiceServer, preSender preSender) error {
	if s.inqChan == nil {
		s.inqChan = make(chan []*bboltproto.DatasetInquiry, 1)
		go handleWithRecovery(ctx, s.recvResult, func(context.Context) error {
			for {
				inq, err := s.stream.Recv()
				if err != nil {
					close(s.inqChan)
					if errors.Is(err, io.EOF) {
						return nil
					}
					return err
				}
				select {
				case s.inqChan <- []*bboltproto.DatasetInquiry{inq}:
				case inqList := <-s.inqChan:
					s.inqChan <- append(inqList, inq)
				default:
				}

				repo, err := datasetServiceServerToRepository(dss, ctx, inq.RepoName)
				if err != nil {
					return err
				}
				s.db = repo.(*localFullRepository).db
			}
		})
	}
	err := s.sendObjectsInternal(ctx, preSender)
	if err != nil {
		return err
	}
	select {
	case pending, ok := <-s.inqChan:
		if !ok {
			return <-s.recvResult
		}
		s.pendingInq = append(s.pendingInq, pending...)
	default:
	}
	return nil
}

func (s *datasetObjectSender) closeAndWait(closer func() error) error {
	err := closer()
	if err != nil {
		return err
	}
	if s.inqChan == nil {
		return nil
	}
	return <-s.recvResult
}

func (s *datasetObjectSender) sendObjectsInternal(_ context.Context, preSender preSender) error {
	var wantCount int
	if preSender != nil {
		var err error
		wantCount, err = preSender()
		if err != nil {
			return err
		}
		if wantCount == 0 {
			return nil
		}
	}
	for {
		var inq *bboltproto.DatasetInquiry
		if len(s.pendingInq) > s.pendingInqOff {
			inq = s.pendingInq[s.pendingInqOff]
			s.pendingInqOff++
			if s.pendingInqOff > 1024 {
				restPending := s.pendingInq[s.pendingInqOff:]
				s.pendingInq = s.pendingInq[0 : len(s.pendingInq)-s.pendingInqOff]
				copy(s.pendingInq, restPending)
				s.pendingInqOff = 0
			}
		} else if inqList, ok := <-s.inqChan; ok {
			if cap(s.pendingInq) > 0 {
				s.pendingInq = append(s.pendingInq, inqList...)
			} else {
				s.pendingInq = inqList
			}
			continue
		} else {
			return nil
		}
		var wanted bool
		switch msg := inq.Inq.(type) {
		case *bboltproto.DatasetInquiry_HasCommitIdWithParents:
			commitID := BytesToHash(msg.HasCommitIdWithParents)
			s.seenRevisions[seenEntry{hash: commitID, entryType: seenCommitWithParents}] = struct{}{}
		case *bboltproto.DatasetInquiry_HasCommitIdNoParents:
			var commitID Hash
			copy(commitID[:], msg.HasCommitIdNoParents)
			s.seenRevisions[seenEntry{hash: commitID, entryType: seenCommitWithNoParents}] = struct{}{}
		case *bboltproto.DatasetInquiry_WantDatasetWithCommitParents:
			dsReq := msg.WantDatasetWithCommitParents
			pending, err := s.sendDatasetCommitWithParents(dsReq)
			if err != nil {
				return err
			}
			if pending != nil {
				s.pendingInq = append(s.pendingInq, pending...)
			}
			wanted = true
		case *bboltproto.DatasetInquiry_WantDatasetNoCommitParents:
			dsReq := msg.WantDatasetNoCommitParents
			err := s.sendDatasetCommitNoParents(dsReq)
			if err != nil {
				return err
			}
			wanted = true
		case *bboltproto.DatasetInquiry_WantDatasetRevision:
			dsRevHash := BytesToHash(msg.WantDatasetRevision)
			err := s.sendDatasetRevisionNoCommit(dsRevHash)
			if err != nil {
				return err
			}
			wanted = true
		}
		if wanted {
			if wantCount > 0 {
				wantCount--
				if wantCount == 0 {
					return nil
				}
			}
		}
	}
}

func (s *datasetObjectSender) sendDatasetCommitWithParents(
	refName *bboltproto.RefName,
) (pending []*bboltproto.DatasetInquiry, err error) {
	var commitID Hash
	var pendingCommitIDs []Hash
	err = s.db.View(func(tx *bbolt.Tx) error {
		datasets := tx.Bucket(keyDatasets)
		if datasets == nil {
			return ErrDatasetNotFound
		}
		dsID, err := uuid.FromBytes(refName.GetDatasetUuid())
		if err != nil {
			return err
		}
		dsRefs := datasets.Bucket(dsID[:])
		if dsRefs == nil {
			return ErrDatasetNotFound
		}
		var ref []byte
		var isFirstDefaultBranch func() bool
		switch refName := refName.RefName.(type) {
		case *bboltproto.RefName_Branch:
			brName := refName.Branch
			ref = make([]byte, len(brName)+1)
			ref[0] = byte(refBranchPrefix)
			copy(ref[1:], stringAsImmutableBytes(brName))
			isFirstDefaultBranch = func() bool { return brName == DefaultBranch && !hasEntries(dsRefs) }
		case *bboltproto.RefName_Leaf:
			leaf := refName.Leaf
			ref = make([]byte, len(leaf)+1)
			ref[0] = byte(refLeafPrefix)
			copy(ref[1:], leaf)
			isFirstDefaultBranch = func() bool { return false }
		}
		if actualRef, commitIDBytes := dsRefs.Cursor().Seek(ref); bytes.Equal(actualRef, ref) {
			commits := tx.Bucket(keyCommits)
			if commits == nil {
				return ErrCommitNotFound
			}
			commitID = BytesToHash(commitIDBytes)
			p := pendingBfsInit(s.seenRevisions, s.inqChan, s.stream, commits, s.visitedPositions)
			aborted := bfs(&p.graph, commitID, p.stop, p.do, p.visited)
			if p.graph.graphErr != nil {
				return p.graph.graphErr
			}
			if aborted {
				return context.Canceled
			}
			pendingCommitIDs, pending = p.pendingCommitIDs, p.pending
			return nil
		} else if isFirstDefaultBranch() {
			commitID = HashNil()
			return nil
		}
		return ErrBranchNotFound
	})
	if err != nil {
		return
	}
	err = s.db.View(func(tx *bbolt.Tx) error {
		for i := len(pendingCommitIDs) - 1; i >= 0; i-- {
			commitID := pendingCommitIDs[i]
			if commitID.IsNil() {
				continue
			}
			commits := tx.Bucket(keyCommits)
			dsRevisions := tx.Bucket(keyDatasetRevisions)
			ngRevisions := tx.Bucket(keyNamedGraphRevisions)
			err := sendCommit(commits, dsRevisions, ngRevisions, s.stream, s.seenRevisions, commitID, nil)
			if err == ErrCommitNotFound { //nolint:errorlint // ErrCommitNotFound is never wrapped in sendCommit
				continue // Commit may not be available if repository was synced with not parents
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return
	}
	err = s.stream.Send(&bboltproto.DatasetObject{
		Obj: &bboltproto.DatasetObject_WantedDatasetWithCommitParents{
			WantedDatasetWithCommitParents: &bboltproto.Ref{Ref: refName, CommitId: commitID[:]},
		},
	})
	return
}

func (s *datasetObjectSender) sendDatasetCommitNoParents(
	refName *bboltproto.RefName,
) (err error) {
	var commitID Hash
	var dsID uuid.UUID
	err = s.db.View(func(tx *bbolt.Tx) error {
		datasets := tx.Bucket(keyDatasets)
		if datasets == nil {
			return ErrDatasetNotFound
		}
		uuid, err := uuid.FromBytes(refName.GetDatasetUuid())
		if err != nil {
			return err
		}
		dsID = uuid
		dsRefs := datasets.Bucket(dsID[:])
		if dsRefs == nil {
			return ErrDatasetNotFound
		}
		switch refName := refName.RefName.(type) {
		case *bboltproto.RefName_Branch:
			brName := refName.Branch
			ref := make([]byte, len(brName)+1)
			ref[0] = byte(refBranchPrefix)
			copy(ref[1:], stringAsImmutableBytes(brName))
			if actualRef, commitIDBytes := dsRefs.Cursor().Seek(ref); bytes.Equal(actualRef, ref) {
				commits := tx.Bucket(keyCommits)
				if commits == nil {
					return ErrCommitNotFound
				}
				commitID = BytesToHash(commitIDBytes)
				return s.stream.Send(&bboltproto.DatasetObject{
					Obj: &bboltproto.DatasetObject_PendingCommitId{PendingCommitId: commitIDBytes},
				})
			} else if brName == DefaultBranch && !hasEntries(dsRefs) {
				commitID = HashNil()
				return nil
			}
			return ErrBranchNotFound
		case *bboltproto.RefName_Leaf:
			commitIDBytes := refName.Leaf
			commitID = BytesToHash(commitIDBytes)
			return s.stream.Send(&bboltproto.DatasetObject{
				Obj: &bboltproto.DatasetObject_PendingCommitId{PendingCommitId: commitIDBytes},
			})
		default:
			panic(refName)
		}
	})
	if err != nil {
		return
	}
	if !commitID.IsNil() {
		if _, ok := s.seenRevisions[seenEntry{hash: commitID, entryType: seenCommitWithNoParents}]; !ok {
			err = s.db.View(func(tx *bbolt.Tx) error {
				commits := tx.Bucket(keyCommits)
				dsRevisions := tx.Bucket(keyDatasetRevisions)
				ngRevisions := tx.Bucket(keyNamedGraphRevisions)
				s.seenRevisions[seenEntry{hash: commitID, entryType: seenCommitWithNoParents}] = struct{}{}
				err := sendCommit(commits, dsRevisions, ngRevisions, s.stream, s.seenRevisions, commitID, []uuid.UUID{dsID})
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return
			}
		}
	}
	err = s.stream.Send(&bboltproto.DatasetObject{
		Obj: &bboltproto.DatasetObject_WantedDatasetNoCommitParents{
			WantedDatasetNoCommitParents: &bboltproto.Ref{Ref: refName, CommitId: commitID[:]},
		},
	})
	return
}

func (s *datasetObjectSender) sendDatasetRevisionNoCommit(dsHash Hash) (err error) {
	if dsHash.IsNil() {
		return nil
	}
	err = s.db.View(func(tx *bbolt.Tx) error {
		dsRevisions := tx.Bucket(keyDatasetRevisions)
		ngRevisions := tx.Bucket(keyNamedGraphRevisions)
		if dsRevisions == nil {
			return ErrDatasetRevisionNotFound
		}
		_, err := sendDatasetRevision(nil, dsRevisions, ngRevisions, s.stream, s.seenRevisions, HashNil(), dsHash, nil)
		return err
	})
	if err != nil {
		return err
	}
	err = s.stream.Send(&bboltproto.DatasetObject{
		Obj: &bboltproto.DatasetObject_WantedDatasetRevision{
			WantedDatasetRevision: dsHash[:],
		},
	})
	return err
}

func hasEntries(b *bbolt.Bucket) bool {
	c := b.Cursor()
	k, _ := c.First()
	return k != nil
}

type pendingBfs struct {
	seenRevisions    map[seenEntry]struct{}
	inqChan          chan []*bboltproto.DatasetInquiry
	sender           datasetObjectMessageSender
	graph            pendingCommitGraph
	visitedPositions map[Hash]int
	pending          []*bboltproto.DatasetInquiry
	pendingCommitIDs []Hash
}

func pendingBfsInit(
	seenRevisions map[seenEntry]struct{},
	inqChan chan []*bboltproto.DatasetInquiry,
	sender datasetObjectMessageSender,
	commits *bbolt.Bucket,
	visitedPositions map[Hash]int,
) pendingBfs {
	for pos := range visitedPositions {
		delete(visitedPositions, pos)
	}
	return pendingBfs{
		seenRevisions:    seenRevisions,
		inqChan:          inqChan,
		sender:           sender,
		graph:            pendingCommitGraph{commits: commits},
		visitedPositions: visitedPositions,
	}
}

func (p *pendingBfs) stop(c Hash) bool {
	if _, ok := p.seenRevisions[seenEntry{hash: c, entryType: seenCommitWithParents}]; ok {
		return ok
	}
	if pendingPos, ok := p.visitedPositions[c]; ok {
		return p.pendingCommitIDs[pendingPos].IsNil()
	}
	return false
}

func (p *pendingBfs) do(_, w Hash) (_ bool) {
	p.visitedPositions[w] = len(p.pendingCommitIDs)
	p.pendingCommitIDs = append(p.pendingCommitIDs, w)
	err := p.sender.Send(&bboltproto.DatasetObject{
		Obj: &bboltproto.DatasetObject_PendingCommitId{PendingCommitId: w[:]},
	})
	if err != nil {
		p.graph.graphErr = err
		return true
	}
	err = p.maybeReceiveRequests()
	if err != nil {
		p.graph.graphErr = err
		return true
	}
	return
}

func (p *pendingBfs) visited(w Hash) bool {
	_, ok := p.visitedPositions[w]
	return ok
}

func (p *pendingBfs) maybeReceiveRequests() (err error) {
	if p.pending != nil {
		return
	}
	for {
		if testingSenderInquiryBarrier != nil {
			<-testingSenderInquiryBarrier
		}
		select {
		case inqList, ok := <-p.inqChan:
			if !ok {
				return
			}
			for inqIdx, inq := range inqList {
				var commitID Hash
				switch msg := inq.Inq.(type) {
				case *bboltproto.DatasetInquiry_HasCommitIdWithParents:
					commitID = BytesToHash(msg.HasCommitIdWithParents)
					p.seenRevisions[seenEntry{hash: commitID, entryType: seenCommitWithParents}] = struct{}{}
				case *bboltproto.DatasetInquiry_HasCommitIdNoParents:
					copy(commitID[:], msg.HasCommitIdNoParents)
					p.seenRevisions[seenEntry{hash: commitID, entryType: seenCommitWithNoParents}] = struct{}{}
				default:
					p.pending = inqList[inqIdx:]
					return
				}
				hasVisited := make(map[Hash]struct{}, len(p.visitedPositions))
				g := pendingCommitGraph{commits: p.graph.commits}
				aborted := bfs(&g, commitID, func(c Hash) bool {
					_, ok := p.visitedPositions[c]
					return !ok
				}, func(v, w Hash) (_ bool) {
					hasVisited[w] = struct{}{}
					if pendingPos, ok := p.visitedPositions[w]; ok {
						p.pendingCommitIDs[pendingPos] = HashNil()
					}
					return
				}, func(w Hash) bool {
					_, ok := hasVisited[w]
					return ok
				})
				if g.graphErr != nil {
					err = g.graphErr
					return
				}
				if aborted {
					return context.Canceled
				}
			}
		default:
			return
		}
	}
}

type pendingCommitGraph struct {
	commits  *bbolt.Bucket
	graphErr error
}

func (g *pendingCommitGraph) visit(
	c Hash,
	stop func(c Hash) bool,
	do func(w Hash) (skip bool),
) (_ bool) {
	if stop(c) {
		return
	}
	commit := g.commits.Bucket(c[:])
	if commit == nil {
		return
	}
	commitC := commit.Cursor()
	for k, v := commitC.First(); k != nil && k[0] == '\x00'; k, v = commitC.Next() {
		for i := len(Hash{}); i < len(v); i += len(Hash{}) {
			w := BytesToHash(v[i : i+len(Hash{})])
			if do(w) {
				return true
			}
		}
	}
	return
}

type graphVisitor interface {
	// visit calls the do function for each neighbor w of vertex v,
	// with c equal to the cost of the edge from v to w.
	//
	// • If do returns true, visit returns immediately, skipping
	// any remaining neighbors, and returns true.
	//
	// • The calls to the do function may occur in any order,
	// and the order may vary.
	//
	// The provided function visited returns true if the node w has already been visited
	// using do function.
	visit(v Hash, stop func(c Hash) bool, do func(w Hash) (skip bool)) (aborted bool)
}

func bfs(
	g graphVisitor,
	v Hash,
	stop func(c Hash) bool,
	do func(v, w Hash) (aborted bool),
	visited func(w Hash) bool,
) (aborted bool) {
	if stop(v) {
		return
	}
	if aborted = do(HashNil(), v); aborted {
		return
	}
	for queue := []Hash{v}; len(queue) > 0; {
		v := queue[0]
		queue = queue[1:]
		aborted = g.visit(v, stop, func(w Hash) (_ bool) {
			if visited(w) {
				return
			}
			if stop(w) {
				return
			}
			if do(v, w) {
				return true
			}
			queue = append(queue, w)
			return
		})
		if aborted {
			return
		}
	}
	return
}

func sendCommit(
	commits, dsRevisions, ngRevisions *bbolt.Bucket,
	sender datasetObjectMessageSender,
	seenRevisions map[seenEntry]struct{},
	commitID Hash,
	dsIDPath []uuid.UUID,
) error {
	if commits == nil {
		return ErrCommitNotFound
	}
	commit := commits.Bucket(commitID[:])
	GlobalLogger.Debug("sendCommit", zap.String("commitID", commitID.String()))
	if commit == nil {
		return ErrCommitNotFound
	}
	commitC := commit.Cursor()
	datasets, err := fillCommitDatasetsByCommitID(commitC, func(dsID []byte, dsHash Hash) error {
		dsIDPath := dsIDPath
		if len(dsIDPath) > 0 && !bytes.Equal(dsID, dsIDPath[0][:]) {
			dsIDPath = nil
		}
		_, err := sendDatasetRevision(commits, dsRevisions, ngRevisions, sender, seenRevisions, commitID, dsHash, dsIDPath)
		return err
	})
	if err != nil {
		return err
	}
	var authorURI string
	var authorTimestamp *timestamppb.Timestamp
	if k, v := commitC.Seek(commitKeyAuthor); bytes.Equal(k, commitKeyAuthor) {
		authorURI = string(v[0 : len(v)-8])
		authorUnixTime := binary.BigEndian.Uint64(v[len(v)-8:])
		authorTimestamp = timestamppb.New(time.Unix(int64(authorUnixTime), 0))
	} else {
		return ErrCommitNotFound
	}
	var mr bboltproto.Commit_MessageOrReason
	if k, v := commitC.Seek(commitKeyMessage); bytes.Equal(k, commitKeyMessage) {
		mr = &bboltproto.Commit_Message{Message: string(v)}
	} else {
		return ErrCommitNotFound
	}
	return sender.Send(&bboltproto.DatasetObject{
		Obj: &bboltproto.DatasetObject_Commit{
			Commit: &bboltproto.Commit{
				CommitId:        commitID[:],
				Datasets:        datasets,
				AuthorUri:       authorURI,
				AuthorTimestamp: authorTimestamp,
				MessageOrReason: mr,
			},
		},
	})
}

func sendDatasetRevision(
	commits, dsRevisions, ngRevisions *bbolt.Bucket,
	sender datasetObjectMessageSender,
	seenRevisions map[seenEntry]struct{},
	commitID Hash,
	dsHash Hash,
	dsIDPath []uuid.UUID,
) ([]uuid.UUID, error) {
	e := seenEntry{hash: dsHash, entryType: seenDatasetRevision}
	if _, found := seenRevisions[e]; found {
		return dsIDPath, nil
	}
	if dsRevisions == nil {
		return dsIDPath, ErrDatasetRevisionNotFound
	}
	dsRev := dsRevisions.Bucket(dsHash[:])
	if dsRev == nil {
		return dsIDPath, ErrDatasetRevisionNotFound
	}
	dsRevC := dsRev.Cursor()
	var defaultNamedGraphHash []byte
	var importedDatasets []*bboltproto.DatasetRevision_Import
	for k, v := dsRevC.First(); k != nil; k, v = dsRevC.Next() {
		if k[0] == '\x00' {
			if len(k) == 1 {
				// send DSR default NamedGraphRevision and content
				defaultNamedGraphHash = v
				err := sendNamedGraphRevision(ngRevisions, sender, seenRevisions, BytesToHash(v))
				if err != nil {
					return dsIDPath, err
				}
				continue
			}
			importedDatasets = append(importedDatasets, &bboltproto.DatasetRevision_Import{
				DatasetUuid: k[1:],
				DatasetHash: v,
			})
			if len(dsIDPath) > 0 {
				dsID, err := uuid.FromBytes(k[1:])
				if err != nil {
					return dsIDPath, err
				}
				dsIDPath = append(dsIDPath, dsID)
			}
			var err error
			// recursively send DSR of the imported dataset
			dsIDPath, err = sendDatasetRevision(
				commits, dsRevisions, ngRevisions, sender, seenRevisions, commitID, BytesToHash(v), dsIDPath,
			)
			if len(dsIDPath) > 0 {
				dsIDPath = dsIDPath[:len(dsIDPath)-1]
			}
			if err != nil {
				return dsIDPath, err
			}
		} else if k[0] == '\x02' { // send map, key: DSR value: CommitHash
			// if len(k) > 0 {
			// 	err := sendNamedGraphRevision(ngRevisions, sender, seenRevisions, BytesToHash(v))
			// 	if err != nil {
			// 		return dsIDPath, err
			// 	}
			err := sender.Send(&bboltproto.DatasetObject{
				Obj: &bboltproto.DatasetObject_DatasetRevisionCommitHash{
					DatasetRevisionCommitHash: &bboltproto.DatasetRevisionToCommitHash{
						DatasetRevision: dsHash[:],
						CommitHash:      v[:],
					},
				},
			})
			if err != nil {
				return dsIDPath, err
			}
			continue
			// }
		}
	}
	seenRevisions[e] = struct{}{}
	if len(dsIDPath) > 1 {
		// dsID := dsIDPath[len(dsIDPath)-1]
		// bCommitID := findParentCommitInTransaction(commits, commitID, dsIDPath[:len(dsIDPath)-1], dsID)
		// err := sender.Send(&bboltproto.DatasetObject{
		// 	Obj: &bboltproto.DatasetObject_NamedGraphBaseCommit{
		// 		NamedGraphBaseCommit: &bboltproto.ParentCommit{
		// 			DatasetUuid: dsID[:],
		// 			CommitId:    bCommitID[:],
		// 		},
		// 	},
		// })
		// if err != nil {
		// 	return dsIDPath, err
		// }
	}
	return dsIDPath, sender.Send(&bboltproto.DatasetObject{
		Obj: &bboltproto.DatasetObject_DatasetRevision{
			DatasetRevision: &bboltproto.DatasetRevision{
				Hash:                  dsHash[:],
				DefaultNamedGraphHash: defaultNamedGraphHash,
				ImportedDatasets:      importedDatasets,
			},
		},
	})
}

func sendNamedGraphRevision(
	ngRevisions *bbolt.Bucket,
	sender datasetObjectMessageSender,
	seenRevisions map[seenEntry]struct{},
	ngHash Hash,
) error {
	e := seenEntry{hash: ngHash, entryType: seenNamedGraphRevision}
	if _, found := seenRevisions[e]; found {
		return nil
	}
	var ngContent []byte
	if ngRevisions == nil {
		return ErrNamedGraphRevisionNotFound
	}
	if k, v := ngRevisions.Cursor().Seek(ngHash[:]); bytes.Equal(k, ngHash[:]) {
		ngContent = v
	} else {
		return ErrNamedGraphRevisionNotFound
	}
	seenRevisions[e] = struct{}{}
	GlobalLogger.Debug("sendNamedGraphRevision", zap.String("ngHash", ngHash.String()))
	return sender.Send(&bboltproto.DatasetObject{
		Obj: &bboltproto.DatasetObject_NamedGraphRevision{
			NamedGraphRevision: &bboltproto.NamedGraphRevision{
				Hash:    ngHash[:],
				Content: ngContent,
			},
		},
	})
}

// fillCommitDatasets fills the datasets of the given commits(can be several commits).
func fillCommitDatasets(r Repository, commitIDs []Hash) ([][]*bboltproto.CommittedDatasetRevision, error) {
	if r == nil {
		return nil, ErrUnsupportedRepository
	}
	db := r.(*localFullRepository).db
	allDatasets := make([][]*bboltproto.CommittedDatasetRevision, 0, len(commitIDs))
	err := db.View(func(tx *bbolt.Tx) error {
		commits := tx.Bucket(keyCommits)
		if commits == nil {
			return ErrCommitNotFound
		}
		for _, commitID := range commitIDs {
			commit := commits.Bucket(commitID[:])
			if commit == nil {
				return ErrCommitNotFound
			}
			commitC := commit.Cursor()
			datasets, err := fillCommitDatasetsByCommitID(commitC, func([]byte, Hash) error { return nil })
			if err != nil {
				return err
			}
			allDatasets = append(allDatasets, datasets)
		}
		return nil
	})
	return allDatasets, err
}

func fillCommitDatasetsByCommitID(
	commitC *bbolt.Cursor, dsFunc func(dsID []byte, dsHash Hash) error,
) ([]*bboltproto.CommittedDatasetRevision, error) {
	var datasetRevisions []*bboltproto.CommittedDatasetRevision
	// loop all datasets in the commit
	for k, v := commitC.First(); k != nil && k[0] == '\x00'; k, v = commitC.Next() {
		dsHash := BytesToHash(v)
		err := dsFunc(k[1:], dsHash)
		if err != nil {
			return nil, err
		}
		parentCommitIds := make([][]byte, 0, len(v)/len(Hash{})-1)
		for i := len(Hash{}); i < len(v); i += len(Hash{}) {
			parentCommitIds = append(parentCommitIds, v[i:i+len(Hash{})])
		}
		datasetRevisions = append(datasetRevisions, &bboltproto.CommittedDatasetRevision{
			DatasetUuid:     k[1:],
			DatasetHash:     v[0:len(Hash{})],
			ParentCommitIds: parentCommitIds,
		})
	}
	return datasetRevisions, nil
}
