//
// Copyright (C) 2026 IOTech Ltd
//

package bm25

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

var threeDocs = [][]string{{"a", "b"}, {"a"}, {"c"}}

func TestBM25_Score(t *testing.T) {
	scores := Score(threeDocs, []string{"a", "b"})

	n, avgdl := 3.0, 4.0/3.0
	idf := func(nq float64) float64 { return math.Log((n-nq+0.5)/(nq+0.5) + 1) }
	// B4's k1 and b written out, not K1 and B: a changed constant must fail here.
	k1, b := 1.2, 0.75
	term := func(tf, dl float64) float64 {
		return tf * (k1 + 1) / (tf + k1*(1-b+b*dl/avgdl))
	}
	want := []float64{
		idf(2)*term(1, 2) + idf(1)*term(1, 2),
		idf(2) * term(1, 1),
		0,
	}
	for i := range want {
		assert.InDeltaf(t, want[i], scores[i], 1e-9, "doc %d", i)
	}
	assert.Greater(t, scores[0], scores[1])
	assert.Greater(t, scores[1], 0.0)
	assert.Zero(t, scores[2])
}

func TestScore_EmptyCorpus(t *testing.T) {
	scores := Score(nil, []string{"a"})

	assert.NotNil(t, scores)
	assert.Empty(t, scores)
}

func TestScore_DuplicateQueryTokens(t *testing.T) {
	assert.Equal(t, Score(threeDocs, []string{"a"}), Score(threeDocs, []string{"a", "a"}))
}
