//
// Copyright (C) 2026 IOTech Ltd
//

// Package bm25 scores already-tokenised documents against a query with Okapi
// BM25. Tokenising and what counts as a document are the caller's: the corpus
// is exactly the documents passed in, so N and IDF follow them.
package bm25

import "math"

const (
	K1 = 1.2
	B  = 0.75
)

// Score returns one score per document. Query tokens are de-duplicated, and
// IDF(q) = ln((N − n(q) + 0.5)/(n(q) + 0.5) + 1), which never goes negative. No
// documents give an empty slice.
func Score(docs [][]string, query []string) []float64 {
	n := float64(len(docs))
	scores := make([]float64, len(docs))
	if len(docs) == 0 {
		return scores
	}

	var total int
	freqs := make([]map[string]int, len(docs))
	for i, d := range docs {
		total += len(d)
		freqs[i] = map[string]int{}
		for _, tok := range d {
			freqs[i][tok]++
		}
	}
	avgdl := float64(total) / n

	seen := map[string]bool{}
	for _, q := range query {
		if seen[q] {
			continue
		}
		seen[q] = true

		var nq float64
		for _, f := range freqs {
			if f[q] > 0 {
				nq++
			}
		}
		idf := math.Log((n-nq+0.5)/(nq+0.5) + 1)
		for i, f := range freqs {
			tf := float64(f[q])
			if tf == 0 {
				continue
			}
			norm := 1 - B + B*float64(len(docs[i]))/avgdl
			scores[i] += idf * tf * (K1 + 1) / (tf + K1*norm)
		}
	}
	return scores
}
