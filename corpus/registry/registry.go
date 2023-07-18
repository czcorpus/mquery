// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of CNC-MASM.
//
//  CNC-MASM is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  CNC-MASM is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with CNC-MASM.  If not, see <https://www.gnu.org/licenses/>.

package registry

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

const (
	tmpFileNameTemplate = "masm_registry_*"
)

var (
	dataPathPatt     = regexp.MustCompile(`^PATH\s+("([^"]+)"|(.+))$`)
	RegistryNotFound = errors.New("failed to find registry file")
)

// CorpusConfProvider represents a small subset of corpus configuration
// required by some functions in the 'registry' package.
type CorpusConfProvider interface {
	GetFirstValidRegistry(corpusID string) string
	GetCorpusCNCDataPath() string
}

// EnsureValidDataRegistry tests whether a provided corpusID registry file
// specifies existing PATH.
// a) In case the data referred by PATH exists, the original registry path
// is returned.
// b) If not then it assumes that the data have not
// been yet synchronized and it creates a temporary patched registry
// with the PATH pointing to CorpusDataPath.CNC (in the real world this is
// probably /cnk/run/manatee/data).
// The file is stored to /tmp so we don't have to care about removing it.
// It is considered OK to create a new registry file each time the function
// is called and the synchronization is not yet ready - as we expect this
// to happen only rarely.
// In case there is no registry found to read from, RegistryNotFound error
// is returned. Other possible returned errors are rather low level and should
// translate in the API as "Internal Server Error".
func EnsureValidDataRegistry(conf CorpusConfProvider, corpusID string) (string, error) {
	regPath := conf.GetFirstValidRegistry(corpusID)
	if regPath == "" {
		return "", fmt.Errorf("failed to find valid registry path for %s", corpusID)
	}
	regIsFile, err := fs.IsFile(regPath)
	if err != nil {
		if !regIsFile || os.IsNotExist(err) {
			return "", RegistryNotFound

		} else {
			return "", err
		}
	}
	file, err := os.Open(regPath)
	if err != nil {
		return "", fmt.Errorf("failed to open registry file for %s: %w", corpusID, err)
	}
	defer file.Close()

	fsc1 := bufio.NewScanner(file)
	fwr2 := bytes.Buffer{}
	isPatched := false
	for fsc1.Scan() {
		line := fsc1.Text()
		srch := dataPathPatt.FindStringSubmatch(line)
		if len(srch) > 0 {
			var realDataPath string
			if srch[2] != "" {
				realDataPath = srch[2]

			} else {
				realDataPath = srch[3]
			}
			rdpExists, err := fs.IsDir(realDataPath)
			if err != nil || !rdpExists {
				isPatched = true
				fwr2.WriteString(
					fmt.Sprintf("PATH \"%s\"\n",
						filepath.Join(conf.GetCorpusCNCDataPath(), corpusID)))
				log.Info().
					Str("corpusId", corpusID).
					Str("dataPath", conf.GetCorpusCNCDataPath()).
					Msg("patching registry file with CNC data path")

			} else {
				fwr2.WriteString(line + "\n")
			}

		} else {
			fwr2.WriteString(line + "\n")
		}
	}
	if isPatched {
		file2, err := ioutil.TempFile(os.TempDir(), tmpFileNameTemplate)
		if err != nil {
			return "", fmt.Errorf("failed to create tmp file for writing registry: %w", err)
		}
		defer file2.Close()
		io.Copy(file2, &fwr2)
	}
	if err != nil {
		return "", fmt.Errorf("failed to create patched registry: %w", err)
	}
	return regPath, nil
}
