// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExplainRemoteRepositoryOpenError returns a user-oriented problem line for a failed
// OpenRemoteRepository. includeDetails is always false; raw errors are not shown.
func ExplainRemoteRepositoryOpenError(target string, err error) (friendly string, includeDetails bool) {
	if err == nil {
		return "", false
	}

	reason := remoteOpenReason(target, err)
	return FormatCLIProblem("open repository", reason), false
}

func remoteOpenReason(target string, err error) string {
	if reason := reasonFromGRPCChain(err); reason != "" {
		switch reason {
		case "authentication failed", "permission denied", "write access required":
			return "authentication failed"
		case "resource not found", "repository not found":
			return "repository not found"
		case "service unavailable":
			return "service unavailable"
		default:
			return reason
		}
	}

	raw := strings.ToLower(cleanErrorMessage(err.Error()))
	switch {
	case strings.Contains(raw, "produced zero addresses"),
		strings.Contains(raw, "name resolver error"),
		strings.Contains(raw, "no such host"):
		return "host not found"
	case strings.Contains(raw, "connection refused"):
		return "connection refused"
	case strings.Contains(raw, "deadline exceeded"),
		strings.Contains(raw, "context deadline exceeded"),
		strings.Contains(raw, "timeout"):
		return "connection timed out"
	case strings.Contains(raw, "certificate"),
		strings.Contains(raw, "tls "),
		strings.Contains(raw, "x509"):
		return "TLS handshake failed"
	}

	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unauthenticated, codes.PermissionDenied:
			return "authentication failed"
		case codes.NotFound:
			return "repository not found"
		case codes.Unavailable:
			return "service unavailable"
		}
	}

	_ = target
	return "operation failed"
}
