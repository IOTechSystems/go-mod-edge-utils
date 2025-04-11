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

type pathBuilder struct {
	sb                    strings.Builder
	enableNameFieldEscape bool
}

func NewPathBuilder() *pathBuilder {
	return &pathBuilder{}
}

func (b *pathBuilder) EnableNameFieldEscape(enableNameFieldEscape bool) *pathBuilder {
	b.enableNameFieldEscape = enableNameFieldEscape
	return b
}

func (b *pathBuilder) SetPath(path string) *pathBuilder {
	b.sb.WriteString(path + "/")
	return b
}

// SetNameFieldPath set name path, such as device name, profile name, interval name
func (b *pathBuilder) SetNameFieldPath(namePath string) *pathBuilder {
	if b.enableNameFieldEscape {
		namePath = URLEncode(namePath)
	}
	b.sb.WriteString(namePath + "/")
	return b
}

func (b *pathBuilder) BuildPath() string {
	return strings.TrimSuffix(b.sb.String(), "/")
}
