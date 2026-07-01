// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package bboltproto

import (
	"sync"

	fieldmaskpb "google.golang.org/protobuf/types/known/fieldmaskpb"
)

// Install
// - protoc compiler
// - protoc and grpc plugins:
// go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
// go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
// go install github.com/yeqown/protoc-gen-fieldmask@v0.3.3

//go:generate -command protoc protoc --plugin=protoc-gen-go-grpc=$GOPATH/bin/protoc-gen-go-grpc --plugin=protoc-gen-go=$GOPATH/bin/protoc-gen-go --plugin=protoc-gen-fieldmask=$GOPATH/bin/protoc-gen-fieldmask

//go:generate protoc -I. -I$GOPATH/pkg/mod/github.com/yeqown/protoc-gen-fieldmask@v0.3.3/proto --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --fieldmask_out=. --fieldmask_opt=paths=source_relative,lang=go repository.proto

// need point the protoc-gen-fieldmask file out
// protoc -I . -I "C:\Users\50258\go\pkg\mod\github.com\yeqown\protoc-gen-fieldmask@v0.3.3\proto" --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative repository.proto
// protoc -I . -I "C:\Users\allen\go\pkg\mod\github.com\yeqown\protoc-gen-fieldmask@v0.3.3\proto" --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative repository.proto

var datasetsSkipIRI struct {
	fieldMask *fieldmaskpb.FieldMask
	once      sync.Once
}

func DatasetReadUUID() *fieldmaskpb.FieldMask {
	datasetsSkipIRI.once.Do(func() {
		req := ListDatasetsRequest{}
		datasetsSkipIRI.fieldMask = req.MaskOut_Uuid().ReadMask
	})
	return datasetsSkipIRI.fieldMask
}

type (
	BareRefName                                = isRefName_RefName
	Commit_MessageOrReason                     = isCommit_MessageOrReason                     //nolint:revive
	ListRefsResponse_NextPageToken_NextRefName = isListRefsResponse_NextPageToken_NextRefName //nolint:revive
)
