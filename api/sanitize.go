package api

import (
	"strings"
)

func safeHTML(in string) string {
	return strings.ReplaceAll(in, "<", "&lt;")
}

func safeUrl(in string) string {
	return strings.ReplaceAll(in, "\"", "")
}
