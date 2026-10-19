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

package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/czcorpus/mquery-common/corp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestRegistry(t *testing.T, registryDir, dataDir, corpusID string) {
	reg := fmt.Sprintf(
		"NAME \"%s\"\nPATH \"%s\"\n"+
			"ATTRIBUTE word {\n\tLABEL \"word form\"\n}\n"+
			"ATTRIBUTE lemma {\n\tLABEL \"lemma (registry)\"\n}\n"+
			"ATTRIBUTE tag\n"+
			"STRUCTURE doc {\n\tATTRIBUTE title\n\tATTRIBUTE year\n}\n",
		corpusID, dataDir,
	)
	require.NoError(t, os.WriteFile(filepath.Join(registryDir, corpusID), []byte(reg), 0644))
}

func createTestCorporaSetup(t *testing.T) *CorporaSetup {
	root := t.TempDir()
	registryDir := filepath.Join(root, "registry")
	dataDir := filepath.Join(root, "data")
	splitDir := filepath.Join(root, "split")
	for _, d := range []string{registryDir, dataDir, splitDir} {
		require.NoError(t, os.Mkdir(d, 0755))
	}
	mktokencov := filepath.Join(root, "mktokencov")
	require.NoError(t, os.WriteFile(mktokencov, []byte{}, 0755))
	for _, c := range []string{"plain", "par_cs", "par_en"} {
		writeTestRegistry(t, registryDir, dataDir, c)
	}

	posAttrs := corp.PosAttrList{
		{Name: "word", Description: map[string]string{"en": "word"}},
		{Name: "lemma", Description: map[string]string{"en": "lemma (json)"}},
	}
	return &CorporaSetup{
		RegistryDir:     registryDir,
		SplitCorporaDir: splitDir,
		MktokencovPath:  mktokencov,
		Resources: Resources{
			{
				CorpusSetup: corp.CorpusSetup{
					ID:       "plain",
					FullName: map[string]string{"en": "Plain corpus"},
					PosAttrs: posAttrs,
				},
			},
			{
				CorpusSetup: corp.CorpusSetup{
					ID:                "par_*",
					PosAttrs:          posAttrs,
					SyntaxConcordance: corp.SyntaxConcordance{ParentAttr: "parent"},
					Variants: map[string]corp.CorpusVariant{
						"par_cs": {ID: "par_cs", FullName: map[string]string{"en": "Parallel CS"}},
						"par_en": {ID: "par_en", FullName: map[string]string{"en": "Parallel EN"}},
					},
				},
				InternalNetworkAccessOnly: true,
			},
		},
	}
}

func TestValidateAndDefaultsExpandsVariants(t *testing.T) {
	cs := createTestCorporaSetup(t)
	require.NoError(t, cs.ValidateAndDefaults("corpora"))
	require.NoError(t, cs.ValidateAndDefaults("corpora")) // must be idempotent

	all := cs.GetAllCorpora("")
	ids := make([]string, len(all))
	for i, c := range all {
		ids[i] = c.ID
	}
	assert.ElementsMatch(t, []string{"plain", "par_cs", "par_en"}, ids)
	assert.Nil(t, cs.GetCorp("par_*"))

	cs.ZeroConfCorpora = false
	parCS := cs.GetCorp("par_cs")
	require.NotNil(t, parCS)
	assert.Equal(t, "Parallel CS", parCS.FullName["en"])
	assert.True(t, parCS.InternalNetworkAccessOnly)
	assert.Equal(t, "parent", parCS.SyntaxConcordance.ParentAttr)
	assert.Equal(t, DfltMaximumRecords, parCS.MaximumRecords)
	assert.Nil(t, parCS.Variants)
	assert.ElementsMatch(t, []string{"doc.title", "doc.year"}, parCS.FullConcTextPropsAttrs())
}

func TestSyncWithRegistryPosAttrs(t *testing.T) {
	cs := createTestCorporaSetup(t)
	require.NoError(t, cs.ValidateAndDefaults("corpora"))

	for _, corpusID := range []string{"plain", "par_en"} {
		c := cs.GetCorp(corpusID)
		require.NotNil(t, c)
		// configured attributes stay intact
		assert.Equal(t, []string{"word", "lemma"}, c.PosAttrs.GetIDs())
		// all registry attributes, in registry order
		assert.Equal(t, []string{"word", "lemma", "tag"}, c.AllPosAttrs().GetIDs())
		// JSON descriptions take precedence over registry labels
		assert.Equal(t, "lemma (json)", c.AllPosAttrs()[1].Description["en"])
		assert.Equal(t, "", c.AllPosAttrs()[2].Description["en"])
		// GetPosAttr searches all registry attributes
		assert.Equal(t, "tag", c.GetPosAttr("tag").Name)
		assert.Equal(t, "lemma (json)", c.GetPosAttr("lemma").Description["en"])
		assert.True(t, c.GetPosAttr("foo").IsZero())
	}
}

func TestGetPosAttrNotSynced(t *testing.T) {
	c := &MQCorpusSetup{
		CorpusSetup: corp.CorpusSetup{
			ID:       "plain",
			PosAttrs: corp.PosAttrList{{Name: "word"}},
		},
	}
	assert.Equal(t, "word", c.GetPosAttr("word").Name)
	assert.True(t, c.GetPosAttr("tag").IsZero())
}

func TestGetCorpConcurrent(t *testing.T) {
	cs := createTestCorporaSetup(t)
	require.NoError(t, cs.ValidateAndDefaults("corpora"))
	cs.ZeroConfCorpora = true

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NotNil(t, cs.GetCorp("plain"))
			assert.NotNil(t, cs.GetCorp("par_cs"))
			cs.GetCorp("unknown") // zero-conf path writing to cache
		}()
	}
	wg.Wait()
}
