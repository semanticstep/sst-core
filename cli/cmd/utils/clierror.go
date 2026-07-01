// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/semanticstep/sst-core/sst"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var wrapErrorLinePrefix = regexp.MustCompile(`^[a-zA-Z0-9_]+\.go:\d+:\s*`)

// FormatCLIProblem builds a standard user-facing failure line:
// "Cannot {operation}: {reason}."
func FormatCLIProblem(operation, reason string) string {
	operation = strings.TrimSpace(operation)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "operation failed"
	}
	reason = strings.TrimSuffix(reason, ".")
	return fmt.Sprintf("Cannot %s: %s.", operation, reason)
}

// ExplainCLIError maps a backend error to the standard CLI problem format.
func ExplainCLIError(operation string, err error) string {
	if err == nil {
		return FormatCLIProblem(operation, "operation failed")
	}
	return FormatCLIProblem(operation, reasonFromError(err))
}

// PrintCLIProblem prints ExplainCLIError as a single line.
func PrintCLIProblem(operation string, err error) {
	if err == nil {
		return
	}
	fmt.Println(ExplainCLIError(operation, err))
}

func reasonFromError(err error) string {
	if err == nil {
		return "operation failed"
	}

	if reason := reasonFromSentinel(err); reason != "" {
		return reason
	}

	if reason := reasonFromGRPCChain(err); reason != "" {
		return reason
	}

	if reason := reasonFromKnownMessages(err); reason != "" {
		return reason
	}

	if reason := reasonFromNetwork(strings.ToLower(cleanErrorMessage(err.Error()))); reason != "" {
		return reason
	}

	msg := cleanErrorMessage(err.Error())
	if strings.EqualFold(msg, "forbidden") {
		return "write access required"
	}
	if msg == "" {
		return "operation failed"
	}
	return humanizeMessage(msg)
}

func reasonFromSentinel(err error) string {
	switch {
	case errors.Is(err, sst.ErrBranchNotFound):
		return "branch not found"
	case errors.Is(err, sst.ErrDatasetNotFound):
		return "dataset not found"
	case errors.Is(err, sst.ErrRepositoryNotFound), errors.Is(err, sst.ErrRepositoryDoesNotExist):
		return "repository not found"
	case errors.Is(err, sst.ErrDatasetAlreadyExists):
		return "dataset already exists"
	case errors.Is(err, sst.ErrNothingToCommit):
		return "nothing to commit"
	case errors.Is(err, sst.ErrDatasetHasBeenDeleted):
		return "dataset has been deleted"
	case errors.Is(err, sst.ErrConcurrentModification):
		return "concurrent modification"
	case errors.Is(err, sst.ErrIllegalHashLength):
		return "invalid hash"
	case errors.Is(err, sst.ErrEmptyCommitMessage):
		return "commit message cannot be empty"
	case errors.Is(err, sst.ErrRepoNotFound):
		return "repository not found"
	case errors.Is(err, sst.ErrRepoClosed):
		return "repository is closed"
	case errors.Is(err, sst.ErrStageExpired):
		return "stage lease expired"
	}
	return ""
}

// reasonFromKnownMessages matches sentinel error text embedded in wrapped or gRPC errors
// when errors.Is cannot find the sentinel (e.g. remote Unknown status with wrapError prefix).
func reasonFromKnownMessages(err error) string {
	lower := strings.ToLower(cleanErrorMessage(err.Error()))
	switch {
	case strings.Contains(lower, "branch not found"):
		return "branch not found"
	case strings.Contains(lower, "dataset not found"):
		return "dataset not found"
	case strings.Contains(lower, "repository not found"):
		return "repository not found"
	case strings.Contains(lower, "document not found"):
		return "document not found"
	case strings.Contains(lower, "dataset has been deleted"):
		return "dataset has been deleted"
	case strings.Contains(lower, "nothing to commit"):
		return "nothing to commit"
	}
	return ""
}

func reasonFromGRPCChain(err error) string {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if reason := reasonFromGRPC(e); reason != "" {
			return reason
		}
	}
	return ""
}

func reasonFromGRPC(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return ""
	}

	switch st.Code() {
	case codes.PermissionDenied:
		return "write access required"
	case codes.Unauthenticated:
		return "authentication failed"
	case codes.NotFound:
		msg := strings.ToLower(strings.TrimSpace(st.Message()))
		switch {
		case strings.Contains(msg, "branch"):
			return "branch not found"
		case strings.Contains(msg, "dataset"):
			return "dataset not found"
		case strings.Contains(msg, "document"):
			return "document not found"
		case strings.Contains(msg, "commit"):
			return "commit not found"
		case strings.Contains(msg, "repository"):
			return "repository not found"
		default:
			return "resource not found"
		}
	case codes.Unavailable:
		return "service unavailable"
	case codes.InvalidArgument:
		return humanizeMessage(st.Message())
	case codes.AlreadyExists:
		return "already exists"
	default:
		msg := strings.TrimSpace(st.Message())
		if msg != "" && !looksLikeInternalError(msg) {
			return humanizeMessage(msg)
		}
	}
	return ""
}

func reasonFromNetwork(lower string) string {
	switch {
	case strings.Contains(lower, "produced zero addresses"),
		strings.Contains(lower, "name resolver error"),
		strings.Contains(lower, "no such host"):
		return "host not found"
	case strings.Contains(lower, "connection refused"):
		return "connection refused"
	case strings.Contains(lower, "deadline exceeded"),
		strings.Contains(lower, "context deadline exceeded"),
		strings.Contains(lower, "timeout"):
		return "connection timed out"
	case strings.Contains(lower, "certificate"),
		strings.Contains(lower, "tls "),
		strings.Contains(lower, "x509"):
		return "TLS handshake failed"
	}
	return ""
}

func cleanErrorMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	for wrapErrorLinePrefix.MatchString(msg) {
		msg = wrapErrorLinePrefix.ReplaceAllString(msg, "")
		msg = strings.TrimSpace(msg)
	}
	return msg
}

func humanizeMessage(msg string) string {
	msg = cleanErrorMessage(msg)
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "operation failed"
	}
	if strings.EqualFold(msg, "forbidden") {
		return "write access required"
	}
	if looksLikeInternalError(msg) {
		return "operation failed"
	}
	return strings.ToLower(msg)
}

func looksLikeInternalError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, ".go:") ||
		strings.Contains(lower, "rpc error") ||
		strings.Contains(lower, "failed to resolve grpc")
}
