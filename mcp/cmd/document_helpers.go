// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package mcp

import (
	"bufio"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/semanticstep/sst-core/sst"
)

func formatDocumentInfo(doc sst.DocumentInfo) map[string]any {
	return map[string]any{
		"hash":      doc.Hash.String(),
		"mime_type": doc.MIMEType,
		"author":    doc.Author,
		"timestamp": doc.Timestamp.UTC().Format(time.RFC3339),
		"size":      doc.Size,
	}
}

func guessMimeType(filename string, reader *bufio.Reader) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".csv":
		return "text/csv"
	}

	peekBytes, err := reader.Peek(512)
	if err == nil {
		return http.DetectContentType(peekBytes)
	}

	return "application/octet-stream"
}

func extFromMime(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	case "application/json":
		return ".json"
	case "text/html":
		return ".html"
	case "text/csv":
		return ".csv"
	default:
		return ""
	}
}
