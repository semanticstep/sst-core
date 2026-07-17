// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"context"

	"github.com/semanticstep/sst-core/bboltproto"
	cache "github.com/go-pkgz/expirable-cache/v2"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type cacheStream interface {
	Context() context.Context
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

type dsIDBranchNameKey struct {
	dsID   uuid.UUID
	brName string
}

type dsIDCommitIDKey struct {
	dsID     uuid.UUID
	commitID Hash
}

type remoteRequestCache struct {
	getDataset                      cache.Cache[uuid.UUID, *grpc.PreparedMsg]
	datasetInquiryNoParentsByBranch cache.Cache[dsIDBranchNameKey, *grpc.PreparedMsg]
	datasetInquiryNoParentsByCommit cache.Cache[dsIDCommitIDKey, *grpc.PreparedMsg]
	getRefByBranch                  cache.Cache[dsIDBranchNameKey, *grpc.PreparedMsg]
	getRefByCommit                  cache.Cache[dsIDCommitIDKey, *grpc.PreparedMsg]
	getCommit                       cache.Cache[Hash, *grpc.PreparedMsg]
	listCommitByBranch              cache.Cache[dsIDBranchNameKey, *grpc.PreparedMsg]
	listCommitByCommit              cache.Cache[dsIDCommitIDKey, *grpc.PreparedMsg]
}

func newRemoteRequestCache() *remoteRequestCache {
	return &remoteRequestCache{
		getDataset:                      cache.NewCache[uuid.UUID, *grpc.PreparedMsg]().WithMaxKeys(1024),
		datasetInquiryNoParentsByBranch: cache.NewCache[dsIDBranchNameKey, *grpc.PreparedMsg]().WithMaxKeys(1024),
		datasetInquiryNoParentsByCommit: cache.NewCache[dsIDCommitIDKey, *grpc.PreparedMsg]().WithMaxKeys(1024),
		getRefByBranch:                  cache.NewCache[dsIDBranchNameKey, *grpc.PreparedMsg]().WithMaxKeys(1024),
		getRefByCommit:                  cache.NewCache[dsIDCommitIDKey, *grpc.PreparedMsg]().WithMaxKeys(1024),
		getCommit:                       cache.NewCache[Hash, *grpc.PreparedMsg]().WithMaxKeys(1024),
		listCommitByBranch:              cache.NewCache[dsIDBranchNameKey, *grpc.PreparedMsg]().WithMaxKeys(1024),
		listCommitByCommit:              cache.NewCache[dsIDCommitIDKey, *grpc.PreparedMsg]().WithMaxKeys(1024),
	}
}

func (c *remoteRequestCache) loadGetDatasetMsg(dsID uuid.UUID) (*grpc.PreparedMsg, bool) {
	return c.getDataset.Get(dsID)
}

func (c *remoteRequestCache) storeGetDatasetMsg(
	dsID uuid.UUID, stream cacheStream, req *bboltproto.GetDatasetRequest,
) (*grpc.PreparedMsg, error) {
	return putPreparedMessage(c.getDataset, dsID, stream, req)
}

func (c *remoteRequestCache) loadDatasetInquiryNoParentsByBranchMsg(
	dsID uuid.UUID, brName string,
) (*grpc.PreparedMsg, bool) {
	return c.datasetInquiryNoParentsByBranch.Get(dsIDBranchNameKey{dsID: dsID, brName: brName})
}

func (c *remoteRequestCache) storeDatasetInquiryNoParentsByBranchMsg(
	dsID uuid.UUID, brName string, stream cacheStream, req *bboltproto.DatasetInquiry,
) (*grpc.PreparedMsg, error) {
	return putPreparedMessage(c.datasetInquiryNoParentsByBranch, dsIDBranchNameKey{dsID: dsID, brName: brName}, stream, req)
}

func (c *remoteRequestCache) loadDatasetInquiryNoParentsByCommitMsg(
	dsID uuid.UUID, commitID Hash,
) (*grpc.PreparedMsg, bool) {
	return c.datasetInquiryNoParentsByCommit.Get(dsIDCommitIDKey{dsID: dsID, commitID: commitID})
}

func (c *remoteRequestCache) storeDatasetInquiryNoParentsByCommitMsg(
	dsID uuid.UUID, commitID Hash, stream cacheStream, req *bboltproto.DatasetInquiry,
) (*grpc.PreparedMsg, error) {
	return putPreparedMessage(c.datasetInquiryNoParentsByCommit, dsIDCommitIDKey{dsID: dsID, commitID: commitID}, stream, req)
}

func (c *remoteRequestCache) loadRefByBranchMsg(dsID uuid.UUID, brName string) (*grpc.PreparedMsg, bool) {
	return c.getRefByBranch.Get(dsIDBranchNameKey{dsID: dsID, brName: brName})
}

func (c *remoteRequestCache) storeRefByBranchMsg(
	dsID uuid.UUID, brName string, stream cacheStream, req *bboltproto.GetRefRequest,
) (*grpc.PreparedMsg, error) {
	return putPreparedMessage(c.getRefByBranch, dsIDBranchNameKey{dsID: dsID, brName: brName}, stream, req)
}

func (c *remoteRequestCache) loadRefByCommitMsg(dsID uuid.UUID, commitID Hash) (*grpc.PreparedMsg, bool) {
	return c.getRefByCommit.Get(dsIDCommitIDKey{dsID: dsID, commitID: commitID})
}

func (c *remoteRequestCache) storeRefByCommitMsg(
	dsID uuid.UUID, commitID Hash, stream cacheStream, req *bboltproto.GetRefRequest,
) (*grpc.PreparedMsg, error) {
	return putPreparedMessage(c.getRefByCommit, dsIDCommitIDKey{dsID: dsID, commitID: commitID}, stream, req)
}

func (c *remoteRequestCache) loadGetCommitMsg(commitID Hash) (*grpc.PreparedMsg, bool) {
	return c.getCommit.Get(commitID)
}

func (c *remoteRequestCache) storeGetCommitMsg(
	commitID Hash, stream cacheStream, req *bboltproto.GetCommitRequest,
) (*grpc.PreparedMsg, error) {
	return putPreparedMessage(c.getCommit, commitID, stream, req)
}

func (c *remoteRequestCache) loadListCommitByBranchMsg(dsID uuid.UUID, brName string) (*grpc.PreparedMsg, bool) {
	return c.listCommitByBranch.Get(dsIDBranchNameKey{dsID: dsID, brName: brName})
}

func (c *remoteRequestCache) storeListCommitByBranchMsg(
	dsID uuid.UUID, brName string, stream cacheStream, req *bboltproto.ListCommitsRequest,
) (*grpc.PreparedMsg, error) {
	return putPreparedMessage(c.listCommitByBranch, dsIDBranchNameKey{dsID: dsID, brName: brName}, stream, req)
}

func (c *remoteRequestCache) loadListCommitByCommitMsg(dsID uuid.UUID, commitID Hash) (*grpc.PreparedMsg, bool) {
	return c.listCommitByCommit.Get(dsIDCommitIDKey{dsID: dsID, commitID: commitID})
}

func (c *remoteRequestCache) storeListCommitByCommitMsg(
	dsID uuid.UUID, commitID Hash, stream cacheStream, req *bboltproto.ListCommitsRequest,
) (*grpc.PreparedMsg, error) {
	return putPreparedMessage(c.listCommitByCommit, dsIDCommitIDKey{dsID: dsID, commitID: commitID}, stream, req)
}

func putPreparedMessage[K comparable](
	cache cache.Cache[K, *grpc.PreparedMsg], key K, stream cacheStream, plainMsg any,
) (*grpc.PreparedMsg, error) {
	msg := &grpc.PreparedMsg{}
	if err := msg.Encode(stream, plainMsg); err != nil {
		return nil, err
	}
	cache.Set(key, msg, 0)
	return msg, nil
}
