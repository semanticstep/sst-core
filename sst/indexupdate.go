// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/document"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/blevesearch/bleve/v2/size"
	index "github.com/blevesearch/bleve_index_api"
	"github.com/google/uuid"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	deleteSearchSize           = 1024
	maxBatchDocSize            = 16 * 1024 * 1024
	gitIgnoreFileName          = ".gitignore"
	sstIndexVersionFieldPrefix = "version.sst."
	sstIndexVersion            = "00f6ebdc40e6"
	ssoIndexVersionFieldPrefix = "version.sso."
	ssoIndexVersion            = "691e05692807"
)

var errUnsatisfiedConstraint = errors.New("unsatisfied constraint")

// bleveIndexUpdaterI decouples Bleve index maintenance from BBolt storage operations.
// The default implementation is asynchronous: it captures the in-memory state it
// needs on the calling goroutine and then hands the actual Bleve I/O off to an
// internal worker. Callers must invoke Close when tearing down the repository
// so the worker can drain and shut down cleanly.
type bleveIndexUpdaterI interface {
	// updateAfterCommit is invoked after a successful BBolt commit.
	// pcNotifier is the value returned by the pre-commit condition; for
	// LocalBasicRepository it copies the staged Bleve documents into the
	// repository index and must run before the rest of the indexing logic.
	updateAfterCommit(
		ctx context.Context,
		repo Repository,
		stage Stage,
		pcNotifier postCommitNotifier,
		commitInfo commitInfo,
		modifiedDatasets []uuid.UUID,
		branchName string,
	)

	// updateAfterSetBranch is invoked after a branch reference has been moved.
	updateAfterSetBranch(ctx context.Context, repo Repository, dsID uuid.UUID, branch string)

	// updateAfterRemoveBranch is invoked after a branch reference has been removed.
	updateAfterRemoveBranch(ctx context.Context, repo Repository, dsID uuid.UUID, branch string)

	// updateAfterSync is invoked after a dataset has been synchronized from
	// another repository. It queries local commit history for the metadata
	// but overrides CommitAuthor to "sync" to distinguish sync from normal commits.
	updateAfterSync(ctx context.Context, repo Repository, dsID uuid.UUID, branch string)

	// flush blocks until all currently queued index work has been processed.
	flush()

	// close drains pending index work and stops the background worker.
	close()
}

// defaultBleveIndexUpdater is the default asynchronous Bleve index updater.
// It queues Bleve operations on a dedicated background goroutine so that
// BBolt writes never block on index I/O.
type defaultBleveIndexUpdater struct {
	queue chan func()
	stop  chan struct{}
	once  sync.Once
	wg    sync.WaitGroup
	idx   *healthAwareIndex
}

// newDefaultBleveIndexUpdater creates and starts a new asynchronous updater.
func newDefaultBleveIndexUpdater(idx *healthAwareIndex) *defaultBleveIndexUpdater {
	d := &defaultBleveIndexUpdater{
		queue: make(chan func(), 4096),
		stop:  make(chan struct{}),
		idx:   idx,
	}
	d.wg.Add(1)
	go d.run()
	return d
}

func (d *defaultBleveIndexUpdater) run() {
	defer d.wg.Done()
	for {
		select {
		case fn, ok := <-d.queue:
			if !ok {
				return
			}
			fn()
		case <-d.stop:
			for {
				select {
				case fn := <-d.queue:
					fn()
				default:
					return
				}
			}
		}
	}
}

// flush blocks until all work that has been submitted up to this point has
// been processed by the background worker.
func (d *defaultBleveIndexUpdater) flush() {
	done := make(chan struct{})
	select {
	case <-d.stop:
		return
	case d.queue <- func() { close(done) }:
	}
	<-done
}

// close drains pending work and stops the background worker.
func (d *defaultBleveIndexUpdater) close() {
	d.once.Do(func() { close(d.stop) })
	d.wg.Wait()
}

func (d *defaultBleveIndexUpdater) submit(fn func()) {
	select {
	case <-d.stop:
		return
	case d.queue <- fn:
	}
}

func (d *defaultBleveIndexUpdater) recordError(err error) {
	if d.idx != nil {
		d.idx.recordError(err)
	}
}

func (d *defaultBleveIndexUpdater) recordSuccess() {
	if d.idx != nil {
		d.idx.recordSuccess()
	}
}

// updateAfterCommit implements BleveIndexUpdater.
// pcNotifier.postCommitNotify() runs synchronously (LocalBasic needs this).
// The in-memory stage is read synchronously to build a Bleve batch; the batch
// commit and pre-condition checks run on the background worker.
func (d *defaultBleveIndexUpdater) updateAfterCommit(
	_ context.Context,
	repo Repository,
	stage Stage,
	pcNotifier postCommitNotifier,
	commitInfo commitInfo,
	modifiedDatasets []uuid.UUID,
	branchName string,
) {
	// LocalBasicRepository uses pcNotifier to flush the staged Bleve index into
	// the repository index. This is independent of any branch semantics and
	// must run whenever a pre-commit notifier was produced.
	if pcNotifier != nil {
		pcNotifier.postCommitNotify()
	}

	if branchName != DefaultBranch {
		return
	}

	bleveIdx := repo.Bleve()
	if bleveIdx == nil {
		return
	}

	var graphIDs []uuid.UUID
	for _, ng := range stage.NamedGraphs() {
		for _, id := range modifiedDatasets {
			if ng.ID() == id {
				graphIDs = append(graphIDs, ng.ID())
				break
			}
		}
	}

	if len(graphIDs) == 0 {
		return
	}

	batch := &indexBatch{index: bleveIdx, repo: repo, batch: bleveIdx.NewBatch()}
	for _, ngID := range graphIDs {
		if ngID == uuid.Nil {
			continue
		}
		graph := stage.NamedGraph(IRI(ngID.URN()))
		if graph == nil {
			GlobalLogger.Warn("named graph not found in stage during index update", zap.String("graph", ngID.String()))
			continue
		}
		if err := updateIndexForGraph(graph, batch, true, commitInfo); err != nil {
			GlobalLogger.Error("failed to update index for graph", zap.String("graph", ngID.String()), zap.Error(err))
			d.recordError(err)
			return
		}
		if err := indexBatchIfOverThreshold(batch); err != nil {
			GlobalLogger.Error("failed to flush intermediate index batch", zap.Error(err))
			d.recordError(err)
			return
		}
	}

	// Capture everything the worker needs; stage must not be accessed later.
	capturedRepo := repo
	capturedIdx := bleveIdx
	capturedBatch := batch.batch
	capturedPreConditions := append([]batchPreCondition(nil), batch.preConditions...)

	d.submit(func() {
		if err := capturedIdx.Batch(capturedBatch); err != nil {
			GlobalLogger.Warn("failed to commit bleve batch after commit", zap.Error(err))
			d.recordError(err)
			return
		}
		d.recordSuccess()
		if capturedRepo.Bleve() != nil {
			for _, c := range capturedPreConditions {
				// Pass nil as overrideIndex because the batch has already been
				// committed to the live index; we only need to search the live index.
				if err := checkPreCondition(capturedRepo, nil, c.id, c.query); err != nil {
					GlobalLogger.Warn("pre-condition check failed after commit", zap.Error(err))
					return
				}
			}
		}
	})
}

// updateAfterSetBranch implements BleveIndexUpdater asynchronously.
func (d *defaultBleveIndexUpdater) updateAfterSetBranch(
	_ context.Context,
	repo Repository,
	dsID uuid.UUID,
	branch string,
) {
	if branch != DefaultBranch {
		return
	}
	capturedRepo := repo
	capturedID := dsID
	d.submit(func() {
		// Fetch commit info so the re-indexed document has up-to-date metadata.
		var ci commitInfo
		if ds, err := capturedRepo.Dataset(context.Background(), IRI(capturedID.URN())); err == nil {
			if details, err := ds.CommitDetailsByBranch(context.Background(), branch); err == nil {
				ci = commitInfo{
					CommitHash:    details.Commit.String(),
					CommitAuthor:  details.Author,
					CommitTime:    details.AuthorDate.UTC().Format(time.RFC3339),
					CommitMessage: details.Message,
				}
			}
		}
		if err := updateBleveIndexForDatasetBranch(context.Background(), capturedRepo, capturedID, branch, ci); err != nil {
			GlobalLogger.Warn("failed to update bleve index after SetBranch", zap.Error(err))
			d.recordError(err)
		} else {
			d.recordSuccess()
		}
	})
}

// updateAfterRemoveBranch implements BleveIndexUpdater asynchronously.
func (d *defaultBleveIndexUpdater) updateAfterRemoveBranch(
	_ context.Context,
	repo Repository,
	dsID uuid.UUID,
	branch string,
) {
	if branch != DefaultBranch {
		return
	}
	bleveIdx := repo.Bleve()
	if bleveIdx == nil {
		return
	}
	capturedIdx := bleveIdx
	capturedID := dsID.String()
	d.submit(func() {
		if err := capturedIdx.Delete(capturedID); err != nil {
			GlobalLogger.Warn("failed to delete bleve index after RemoveBranch", zap.Error(err))
			d.recordError(err)
			return
		}
		GlobalLogger.Info("bleve index deleted for dataset", zap.String("dataset", capturedID))
		d.recordSuccess()
	})
}

// updateAfterSync implements BleveIndexUpdater asynchronously.
// It queries local commit history and overrides CommitAuthor to "sync".
func (d *defaultBleveIndexUpdater) updateAfterSync(
	ctx context.Context,
	repo Repository,
	dsID uuid.UUID,
	branch string,
) {
	if branch != DefaultBranch {
		return
	}
	capturedRepo := repo
	capturedID := dsID
	d.submit(func() {
		var ci commitInfo
		if ds, err := capturedRepo.Dataset(context.Background(), IRI(capturedID.URN())); err == nil {
			if details, err := ds.CommitDetailsByBranch(context.Background(), branch); err == nil {
				ci = commitInfo{
					CommitHash:   details.Commit.String(),
					CommitAuthor: "sync",
					CommitTime:   details.AuthorDate.UTC().Format(time.RFC3339),
				}
			}
		}
		if err := updateBleveIndexForDatasetBranch(context.Background(), capturedRepo, capturedID, branch, ci); err != nil {
			GlobalLogger.Warn("failed to update bleve index after sync", zap.Error(err))
			d.recordError(err)
		} else {
			d.recordSuccess()
		}
	})
}

type batchPreCondition struct {
	id    string
	query query.Query
}

type indexBatch struct {
	index         bleve.Index
	repo          Repository
	batch         *bleve.Batch
	preConditions []batchPreCondition
	advancedSize  uint64
}

type commitInfo struct {
	CommitHash    string
	CommitAuthor  string
	CommitTime    string
	CommitMessage string
}

func rebuildIndex(ctx context.Context, r Repository, idx bleve.Index, repFS fs.FS) error {
	batch := indexBatch{index: idx, repo: r, batch: idx.NewBatch()}
	walkErr := fs.WalkDir(repFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if d.IsDir() || strings.IndexByte(p, '.') >= 0 {
			return fs.SkipDir
		}
		id, err := uuid.Parse(d.Name())
		if err != nil {
			return nil
		}
		ds, err := r.Dataset(ctx, IRI(id.URN()))
		if err != nil {
			return err
		}
		st, err := ds.CheckoutBranch(ctx, DefaultBranch, DefaultTriplexMode)
		if err != nil {
			if strings.Contains(err.Error(), ErrBranchNotFound.Error()) {
				return nil
			}
			return err
		}
		graph := st.NamedGraph(IRI(id.URN()))
		if graph == nil {
			return nil
		}

		// get commit info from ds
		var ci commitInfo
		if commitDetails, err := ds.CommitDetailsByBranch(ctx, DefaultBranch); err == nil {
			ci = commitInfo{
				CommitHash:    commitDetails.Commit.String(),
				CommitAuthor:  commitDetails.Author,
				CommitTime:    commitDetails.AuthorDate.UTC().Format(time.RFC3339),
				CommitMessage: commitDetails.Message,
			}
		}

		if err := updateIndexForGraph(graph, &batch, false, ci); err != nil {
			return err
		}
		if err := indexBatchIfOverThreshold(&batch); err != nil {
			return err
		}

		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	return indexAndCheckPreConditions(r, &batch)
}

func indexBatchIfOverThreshold(batch *indexBatch) error {
	if batch.batch.TotalDocsSize()+batch.advancedSize > maxBatchDocSize {
		err := batch.index.Batch(batch.batch)
		if err != nil {
			return err
		}
		batch.batch.Reset()
		batch.advancedSize = 0
	}
	return nil
}

func updateIndexForGraph(graph NamedGraph, batch *indexBatch, withOriginal bool, commitInfo ...commitInfo) error {
	var id string
	var data map[string]interface{}
	var preConditionQueries []query.Query
	var err error

	switch v := batch.repo.(type) {
	case *localFullRepository:
		id, data, preConditionQueries, err = v.config.deriveInfo.DeriveDocuments(batch.repo, graph)
	case *localBasicRepository:
		id, data, preConditionQueries, err = v.config.deriveInfo.DeriveDocuments(batch.repo, graph)
	default:
		return fmt.Errorf("unknown repository type %T", batch.repo)
	}

	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return err
	}

	if commitInfo != nil {
		data["commitHash"] = commitInfo[0].CommitHash
		data["commitAuthor"] = commitInfo[0].CommitAuthor
		data["commitTime"] = commitInfo[0].CommitTime
		data["commitMessage"] = commitInfo[0].CommitMessage
	}

	doc := document.NewDocument(id)
	err = batch.index.Mapping().MapDocument(doc, data)
	if err != nil {
		return err
	}
	if withOriginal {
		docBytes, err := json.Marshal(data)
		if err != nil {
			return err
		}
		doc.AddField(document.NewTextFieldWithIndexingOptions("_original", nil, docBytes, index.StoreField))
	}
	err = batch.batch.IndexAdvanced(doc)
	if err != nil {
		return err
	}
	batch.advancedSize += uint64(doc.Size() + len(id) + size.SizeOfString)
	for _, pcq := range preConditionQueries {
		batch.preConditions = append(batch.preConditions, batchPreCondition{id: id, query: pcq})
	}
	return nil
}

func indexAndCheckPreConditions(repo Repository, batch *indexBatch) error {
	err := batch.index.Batch(batch.batch)
	if err != nil {
		return err
	}
	if repo.Bleve() != nil {
		for _, c := range batch.preConditions {
			// Pass nil as overrideIndex because the batch has already been
			// committed to the live index; we only need to search the live index.
			err := checkPreCondition(repo, nil, c.id, c.query)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func checkPreCondition(repo Repository, overrideIndex bleve.Index, selfID string, q query.Query) (err error) {
	request := bleve.NewSearchRequest(q)
	result, err := repo.Bleve().Search(request)
	if err != nil {
		return err
	}
	var hits, overridden map[string]struct{}
	if overrideIndex != nil {
		result, err := overrideIndex.Search(request)
		if err != nil {
			return err
		}
		if len(result.Hits) > 0 {
			overridden = make(map[string]struct{}, len(result.Hits))
			for _, h := range result.Hits {
				overridden[h.ID] = struct{}{}
			}
		}
	}
	if len(result.Hits) > 0 {
		hits = make(map[string]struct{}, len(result.Hits)+len(overridden))
		var isOverridden func(id string) (bool, error)
		if overrideIndex != nil {
			oi, err2 := overrideIndex.Advanced()
			if err2 != nil {
				return err2
			}
			oir, err2 := oi.Reader()
			if err2 != nil {
				return err2
			}
			defer multierr.AppendFunc(&err, func() error { return oir.Close() })
			isOverridden = func(id string) (bool, error) {
				doc, err := oir.Document(id)
				if err != nil {
					return false, err
				}
				return doc != nil, nil
			}
		} else {
			isOverridden = func(id string) (bool, error) { return false, nil }
		}
		for _, h := range result.Hits {
			skip, err := isOverridden(h.ID)
			if err != nil {
				return err
			}
			if !skip {
				hits[h.ID] = struct{}{}
			}
		}
		for id, h := range overridden {
			hits[id] = h
		}
	} else {
		hits = overridden
	}
	delete(hits, selfID)
	if len(hits) > 0 {
		ids := make([]string, 0, len(hits))
		for id := range hits {
			ids = append(ids, id)
		}
		return fmt.Errorf(
			"unique id + idOwner + mainType constraint violated for root node of Dataset: cannot write: %s conflicts:%v query=%s: %w",
			selfID, ids, queryToPrettyString(q), errUnsatisfiedConstraint,
		)
	}
	return
}

func queryToPrettyString(q query.Query) string {
	if q == nil {
		return "<nil>"
	}
	b, err := json.MarshalIndent(q, "", "  ")
	if err == nil {
		return string(b)
	}
	return fmt.Sprintf("<%T>", q)
}

// updateBleveIndexForDatasetBranch rebuilds the Bleve document for the dataset
// on the requested branch. It tolerates a missing branch by returning nil.
func updateBleveIndexForDatasetBranch(
	ctx context.Context,
	repo Repository,
	dsID uuid.UUID,
	branch string,
	commitInfo ...commitInfo,
) error {
	bleveIdx := repo.Bleve()
	if bleveIdx == nil {
		return nil
	}

	ds, err := repo.Dataset(ctx, IRI(dsID.URN()))
	if err != nil {
		return err
	}

	st, err := ds.CheckoutBranch(ctx, branch, DefaultTriplexMode)
	if err != nil {
		if strings.Contains(err.Error(), ErrBranchNotFound.Error()) {
			return nil
		}
		return err
	}

	graph := st.NamedGraph(IRI(dsID.URN()))
	if graph == nil {
		return nil
	}

	batch := &indexBatch{
		index: bleveIdx,
		repo:  repo,
		batch: bleveIdx.NewBatch(),
	}

	if err := updateIndexForGraph(graph, batch, true, commitInfo...); err != nil {
		return err
	}
	if err := indexBatchIfOverThreshold(batch); err != nil {
		return err
	}
	return indexAndCheckPreConditions(repo, batch)
}

// FlushBleveIndex blocks until the repository's Bleve index updater has
// processed all work that was queued prior to the call. It is primarily
// intended for tests that need to observe asynchronous index updates.
func FlushBleveIndex(r Repository) {
	type flusher interface{ flush() }
	switch v := r.(type) {
	case *localFullRepository:
		if f, ok := v.config.bleveIndexUpdater.(flusher); ok {
			f.flush()
		}
	case *localBasicRepository:
		if f, ok := v.config.bleveIndexUpdater.(flusher); ok {
			f.flush()
		}
	}
}

// indexHealth describes the health state of a Repository's Bleve index.
type indexHealth struct {
	healthy     bool
	lastError   error
	errorCount  int64
	lastSuccess time.Time
	lastCheck   time.Time
}

// isHealthy reports whether the index is currently usable.
func (h indexHealth) isHealthy() bool { return h.healthy }

// healthAwareIndex is a bleve.Index proxy that injects health checks and search retries.
type healthAwareIndex struct {
	inner  bleve.Index
	health atomic.Value // *indexHealth
}

func newHealthAwareIndex(idx bleve.Index) *healthAwareIndex {
	h := &healthAwareIndex{inner: idx}
	h.health.Store(&indexHealth{healthy: true, lastSuccess: time.Now()})
	return h
}

func (h *healthAwareIndex) currentHealth() *indexHealth {
	v := h.health.Load()
	if v == nil {
		return &indexHealth{healthy: true}
	}
	return v.(*indexHealth)
}

func (h *healthAwareIndex) recordError(err error) {
	cur := h.currentHealth()
	h.health.Store(&indexHealth{
		healthy:     false,
		lastError:   err,
		errorCount:  cur.errorCount + 1,
		lastSuccess: cur.lastSuccess,
		lastCheck:   time.Now(),
	})
}

func (h *healthAwareIndex) recordSuccess() {
	h.health.Store(&indexHealth{
		healthy:     true,
		lastError:   nil,
		errorCount:  0,
		lastSuccess: time.Now(),
		lastCheck:   time.Now(),
	})
}

func (h *healthAwareIndex) Search(req *bleve.SearchRequest) (*bleve.SearchResult, error) {
	return h.SearchInContext(context.Background(), req)
}

func (h *healthAwareIndex) SearchInContext(ctx context.Context, req *bleve.SearchRequest) (*bleve.SearchResult, error) {
	health := h.currentHealth()

	res, err := h.inner.SearchInContext(ctx, req)
	if err != nil {
		h.recordError(err)
		if isRetryableBleveError(err) {
			time.Sleep(100 * time.Millisecond)
			res, err = h.inner.SearchInContext(ctx, req)
			if err == nil {
				h.recordSuccess()
			} else {
				h.recordError(err)
			}
		}
	} else {
		h.recordSuccess()
	}

	// If the index was already unhealthy before this search, return the
	// successful result together with a warning that carries the error count.
	if err == nil && !health.healthy {
		return res, fmt.Errorf(
			"bleve search succeeded but index is unhealthy (errorCount=%d lastError=%v)",
			health.errorCount, health.lastError,
		)
	}
	return res, err
}

// The remaining methods simply delegate to the underlying index.

func (h *healthAwareIndex) Index(id string, data interface{}) error {
	err := h.inner.Index(id, data)
	if err != nil {
		h.recordError(err)
	} else {
		h.recordSuccess()
	}
	return err
}

func (h *healthAwareIndex) Delete(id string) error {
	err := h.inner.Delete(id)
	if err != nil {
		h.recordError(err)
	} else {
		h.recordSuccess()
	}
	return err
}
func (h *healthAwareIndex) NewBatch() *bleve.Batch { return h.inner.NewBatch() }
func (h *healthAwareIndex) Batch(b *bleve.Batch) error {
	err := h.inner.Batch(b)
	if err != nil {
		h.recordError(err)
	} else {
		h.recordSuccess()
	}
	return err
}

func (h *healthAwareIndex) Document(id string) (index.Document, error) {
	return h.inner.Document(id)
}
func (h *healthAwareIndex) DocCount() (uint64, error) { return h.inner.DocCount() }
func (h *healthAwareIndex) Fields() ([]string, error) { return h.inner.Fields() }
func (h *healthAwareIndex) FieldDict(field string) (index.FieldDict, error) {
	return h.inner.FieldDict(field)
}

func (h *healthAwareIndex) FieldDictRange(field string, startTerm []byte, endTerm []byte) (index.FieldDict, error) {
	return h.inner.FieldDictRange(field, startTerm, endTerm)
}

func (h *healthAwareIndex) FieldDictPrefix(field string, termPrefix []byte) (index.FieldDict, error) {
	return h.inner.FieldDictPrefix(field, termPrefix)
}
func (h *healthAwareIndex) Close() error                           { return h.inner.Close() }
func (h *healthAwareIndex) Mapping() mapping.IndexMapping          { return h.inner.Mapping() }
func (h *healthAwareIndex) Stats() *bleve.IndexStat                { return h.inner.Stats() }
func (h *healthAwareIndex) StatsMap() map[string]interface{}       { return h.inner.StatsMap() }
func (h *healthAwareIndex) GetInternal(key []byte) ([]byte, error) { return h.inner.GetInternal(key) }
func (h *healthAwareIndex) SetInternal(key, val []byte) error      { return h.inner.SetInternal(key, val) }
func (h *healthAwareIndex) DeleteInternal(key []byte) error        { return h.inner.DeleteInternal(key) }
func (h *healthAwareIndex) Name() string                           { return h.inner.Name() }
func (h *healthAwareIndex) SetName(name string)                    { h.inner.SetName(name) }
func (h *healthAwareIndex) Advanced() (index.Index, error)         { return h.inner.Advanced() }

func isRetryableBleveError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "too many open files") ||
		strings.Contains(s, "resource temporarily unavailable") ||
		strings.Contains(s, "no such file or directory") ||
		strings.Contains(s, "timeout")
}
