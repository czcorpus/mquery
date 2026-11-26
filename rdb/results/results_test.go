// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of MQUERY.
//
//  MQUERY is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  MQUERY is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with MQUERY.  If not, see <https://www.gnu.org/licenses/>.

package results

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcordanceIPM(t *testing.T) {
	data, err := json.Marshal(Concordance{ConcSize: 50, CorpusSize: 100_000_000})
	require.NoError(t, err)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(data, &resp))
	assert.Equal(t, 0.5, resp["ipm"])
	assert.NotContains(t, resp, "subcSize")
}

func TestConcordanceIPMSubcorpus(t *testing.T) {
	data, err := json.Marshal(Concordance{ConcSize: 50, CorpusSize: 100_000_000, SubcSize: 10_000_000})
	require.NoError(t, err)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(data, &resp))
	assert.Equal(t, 5.0, resp["ipm"])
	assert.Equal(t, 100_000_000.0, resp["corpusSize"])
	assert.Equal(t, 10_000_000.0, resp["subcSize"])
}

func TestConcordanceIPMEmpty(t *testing.T) {
	data, err := json.Marshal(Concordance{})
	require.NoError(t, err)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(data, &resp))
	assert.Equal(t, 0.0, resp["ipm"])
}

func TestConcSizeIPM(t *testing.T) {
	data, err := json.Marshal(&ConcSize{Total: 50, CorpusSize: 100_000_000})
	require.NoError(t, err)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(data, &resp))
	assert.Equal(t, 0.5, resp["ipm"])
	assert.NotContains(t, resp, "subcSize")

	data, err = json.Marshal(&ConcSize{Total: 50, CorpusSize: 100_000_000, SubcSize: 10_000_000})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &resp))
	assert.Equal(t, 5.0, resp["ipm"])
	assert.Equal(t, 10_000_000.0, resp["subcSize"])
}
