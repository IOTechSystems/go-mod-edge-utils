package common

import (
	"net/url"
	"strings"
)

// URLEncode encodes the input string with additional common character support
func URLEncode(s string) string {
	res := url.PathEscape(s)
	res = strings.Replace(res, "+", "%2B", -1) // MQTT topic reserved char
	res = strings.Replace(res, "-", "%2D", -1)
	res = strings.Replace(res, ".", "%2E", -1) // RegexCmd and Redis topic reserved char
	res = strings.Replace(res, "_", "%5F", -1)
	res = strings.Replace(res, "~", "%7E", -1)

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
