package common

import (
	"strings"
)

type PathBuilder struct {
	sb strings.Builder
}

func NewPathBuilder() *PathBuilder {
	return &PathBuilder{}
}

func (b *PathBuilder) SetPath(path string) *PathBuilder {
	b.sb.WriteString(path + "/")
	return b
}

func (b *PathBuilder) BuildPath() string {
	return strings.TrimSuffix(b.sb.String(), "/")
}
