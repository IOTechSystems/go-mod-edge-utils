package common

import (
	"net/url"
	"strings"
)

// URLEncode encodes the input string with additional common character support
func URLEncode(s string) string {
	res := url.PathEscape(s)
	res = strings.ReplaceAll(res, "+", "%2B") // MQTT topic reserved char
	res = strings.ReplaceAll(res, "-", "%2D")
	res = strings.ReplaceAll(res, ".", "%2E") // RegexCmd and Redis topic reserved char
	res = strings.ReplaceAll(res, "_", "%5F")
	res = strings.ReplaceAll(res, "~", "%7E")

	return res
}

type PathBuilder struct {
	sb                    strings.Builder
	enableNameFieldEscape bool
}

func NewPathBuilder() *PathBuilder {
	return &PathBuilder{}
}

func (b *PathBuilder) EnableNameFieldEscape(enableNameFieldEscape bool) *PathBuilder {
	b.enableNameFieldEscape = enableNameFieldEscape
	return b
}

func (b *PathBuilder) SetPath(path string) *PathBuilder {
	b.sb.WriteString(path + "/")
	return b
}

// SetNameFieldPath set name path, such as device name, profile name, interval name
func (b *PathBuilder) SetNameFieldPath(namePath string) *PathBuilder {
	if b.enableNameFieldEscape {
		namePath = URLEncode(namePath)
	}
	b.sb.WriteString(namePath + "/")
	return b
}

func (b *PathBuilder) BuildPath() string {
	return strings.TrimSuffix(b.sb.String(), "/")
}
