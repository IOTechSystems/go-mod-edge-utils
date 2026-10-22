//
// Copyright (C) 2026 IOTech Ltd
//

// Package toolsearch ranks an MCP tool catalogue against a free-text query, for
// a search_tools meta-tool. It owns what a tool is to the search — tokens, name
// boost, schema property names, order, summary; pkg/bm25 scores. The corpus is
// the caller's visible catalogue, so N and IDF are per caller.
package toolsearch

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bm25"
)

// Cap bounds what one search pushes into a small context.
const Cap = 5

const (
	// nameBoost is how many times a tool's name tokens are counted, so a query
	// word that names the tool outweighs the same word in a description.
	nameBoost = 3

	summaryFallbackLen = 160
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// tokenize lower-cases s, splits it on anything that is not a letter or digit,
// and drops one trailing "s" from tokens longer than 3 characters, so
// "commands" and "command" are one term.
func tokenize(s string) []string {
	var out []string
	for _, tok := range nonAlnum.Split(strings.ToLower(s), -1) {
		if tok == "" {
			continue
		}
		if len(tok) > 3 && strings.HasSuffix(tok, "s") {
			tok = tok[:len(tok)-1]
		}
		out = append(out, tok)
	}
	return out
}

// document is one tool as BM25 sees it.
type document struct {
	tool   *mcp.Tool
	tokens []string
}

func newDocument(t *mcp.Tool) document {
	var toks []string
	name := tokenize(t.Name)
	for range nameBoost {
		toks = append(toks, name...)
	}
	toks = append(toks, tokenize(t.Description)...)
	for _, p := range schemaPropertyNames(t.InputSchema) {
		toks = append(toks, tokenize(p)...)
	}
	return document{tool: t, tokens: toks}
}

// schemaPropertyNames returns the top-level property names of an input schema.
// On the server it is the *jsonschema.Schema the tool was registered with; the
// map form is what a schema becomes once decoded from JSON.
func schemaPropertyNames(schema any) []string {
	var names []string
	switch s := schema.(type) {
	case *jsonschema.Schema:
		if s == nil {
			return nil
		}
		for k := range s.Properties {
			names = append(names, k)
		}
	case map[string]any:
		props, _ := s["properties"].(map[string]any)
		for k := range props {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return names
}

// rank returns the tools matching query, best first. A tool's exact name, in any
// case, returns that tool alone; no query terms return every tool by name.
func rank(catalogue []*mcp.Tool, query string) []*mcp.Tool {
	q := strings.TrimSpace(query)
	for _, t := range catalogue {
		if strings.EqualFold(t.Name, q) {
			return []*mcp.Tool{t}
		}
	}

	terms := tokenize(q)
	if len(terms) == 0 {
		out := append([]*mcp.Tool(nil), catalogue...)
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out
	}

	docs := make([][]string, len(catalogue))
	for i, t := range catalogue {
		docs[i] = newDocument(t).tokens
	}
	scores := bm25.Score(docs, terms)

	type hit struct {
		tool  *mcp.Tool
		score float64
	}
	var hits []hit
	for i, s := range scores {
		if s > 0 {
			hits = append(hits, hit{catalogue[i], s})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].tool.Name < hits[j].tool.Name
	})
	out := make([]*mcp.Tool, len(hits))
	for i, h := range hits {
		out[i] = h.tool
	}
	return out
}

// summary is the description's first sentence, or its first 160 characters.
func summary(desc string) string {
	idx := -1
	for _, sep := range []string{". ", ".\n"} {
		if i := strings.Index(desc, sep); i >= 0 && (idx < 0 || i < idx) {
			idx = i
		}
	}
	if idx >= 0 {
		return desc[:idx+1]
	}
	if r := []rune(desc); len(r) > summaryFallbackLen {
		return string(r[:summaryFallbackLen])
	}
	return desc
}

// Entry is one search hit. Description and InputSchema are filled only when the
// search narrows to one tool.
type Entry struct {
	Name        string `json:"name"`
	Summary     string `json:"summary"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

// Result is what search_tools answers with: at most Cap entries, the number of
// matches before the cap, and a note when the cap cut some off.
type Result struct {
	Tools []Entry `json:"tools"`
	Total int     `json:"total"`
	Note  string  `json:"note,omitempty"`
}

// Search ranks catalogue for query and returns at most Cap entries; kind maps a
// tool name to "read" or "write".
func Search(catalogue []*mcp.Tool, query string, kind func(name string) string) Result {
	hits := rank(catalogue, query)
	res := Result{Tools: []Entry{}, Total: len(hits)}
	for i, t := range hits {
		if i == Cap {
			break
		}
		e := Entry{Name: t.Name, Summary: summary(t.Description), Kind: kind(t.Name)}
		if len(hits) == 1 {
			e.Description = t.Description
			e.InputSchema = t.InputSchema
		}
		res.Tools = append(res.Tools, e)
	}
	if res.Total > Cap {
		res.Note = fmt.Sprintf("Showing %d of %d. Narrow the query.", Cap, res.Total)
	}
	return res
}
