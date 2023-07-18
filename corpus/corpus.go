// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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
	"errors"
	"fmt"
	"mquery/corpus/registry"
	"mquery/mango"
	"os"
	"path/filepath"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

var (
	CorpusNotFound = errors.New("corpus not found")
)

// FileMappedValue is an abstraction of a configured file-related
// value where 'Value' represents the value to be inserted into
// some configuration and may or may not be actual file path.
type FileMappedValue struct {
	Value        string  `json:"value"`
	Path         string  `json:"-"`
	FileExists   bool    `json:"exists"`
	LastModified *string `json:"lastModified"`
	Size         int64   `json:"size"`
}

// WordSketchConf wraps different word-sketches related data/configuration files
type WordSketchConf struct {
	WSDef  FileMappedValue `json:"wsdef"`
	WSBase FileMappedValue `json:"wsbase"`
	WSThes FileMappedValue `json:"wsthes"`
}

// RegistryConf wraps registry configuration related info
type RegistryConf struct {
	Paths        []FileMappedValue   `json:"paths"`
	Vertical     FileMappedValue     `json:"vertical"`
	WordSketches WordSketchConf      `json:"wordSketch"`
	Encoding     string              `json:"encoding"`
	SubcorpAttrs map[string][]string `json:"subcorpAttrs"`
}

// Data wraps information about indexed corpus data/files
type Data struct {
	Size         int64           `json:"size"`
	Path         FileMappedValue `json:"path"`
	ManateeError *string         `json:"manateeError"`
}

// Info wraps information about a corpus installation
type Info struct {
	ID             string       `json:"id"`
	IndexedData    Data         `json:"indexedData"`
	IndexedStructs []string     `json:"indexedStructs"`
	RegistryConf   RegistryConf `json:"registry"`
}

// InfoError is a general corpus data information error.
type InfoError struct {
	error
}

type CorpusError struct {
	error
}

// bindValueToPath creates a new FileMappedValue instance
// using 'value' argument. Then it tests whether the
// 'path' exists and if so then it sets related properties
// (FileExists, LastModified, Size) to proper values
func bindValueToPath(value, path string) (FileMappedValue, error) {
	ans := FileMappedValue{Value: value}
	isFile, err := fs.IsFile(path)
	if err != nil {
		return ans, err
	}
	if isFile {
		mTime, err := fs.GetFileMtime(path)
		if err != nil {
			return ans, err
		}
		mTimeString := mTime.Format("2006-01-02T15:04:05-0700")
		size, err := fs.FileSize(path)
		if err != nil {
			return ans, err
		}
		ans.FileExists = true
		ans.LastModified = &mTimeString
		ans.Size = size
	}
	return ans, nil
}

func findVerticalFile(basePath, corpusID string) (FileMappedValue, error) {
	suffixes := []string{".tar.gz", ".tar.bz2", ".tgz", ".tbz2", ".7z", ".gz", ".zip", ".tar", ".rar", ""}
	var verticalPath string
	if IsIntercorpFilename(corpusID) {
		verticalPath = filepath.Join(basePath, GenCorpusGroupName(corpusID), corpusID)

	} else {
		verticalPath = filepath.Join(basePath, corpusID, "vertikala")
	}

	ans := FileMappedValue{Value: verticalPath}
	for _, suff := range suffixes {
		fullPath := verticalPath + suff
		if fs.PathExists(fullPath) { // on some systems fsops.IsFile returned False?!
			mTime, err := fs.GetFileMtime(fullPath)
			if err != nil {
				return ans, err
			}
			mTimeString := mTime.Format("2006-01-02T15:04:05-0700")
			size, err := fs.FileSize(fullPath)
			if err != nil {
				return ans, err
			}
			ans.LastModified = &mTimeString
			ans.Value = fullPath
			ans.Path = fullPath
			ans.FileExists = true
			ans.Size = size
			return ans, nil
		}
	}
	return ans, nil
}

func attachWordSketchConfInfo(corpusID string, wsattr string, conf *CorporaSetup, result *Info) error {
	tmp := GenWSDefFilename(conf.WordSketchDefDirPath, corpusID)
	value, err := bindValueToPath(tmp, tmp)
	if err != nil {
		return err
	}
	result.RegistryConf.WordSketches = WordSketchConf{
		WSDef: value,
	}

	wsBaseFile, wsBaseVal := GenWSBaseFilename(conf.CorpusDataPath.Abstract, corpusID, wsattr)
	value, err = bindValueToPath(wsBaseVal, wsBaseFile)
	if err != nil {
		return err
	}
	result.RegistryConf.WordSketches.WSBase = value

	wsThesFile, wsThesVal := GenWSThesFilename(conf.CorpusDataPath.Abstract, corpusID, wsattr)
	value, err = bindValueToPath(wsThesVal, wsThesFile)
	if err != nil {
		return err
	}
	result.RegistryConf.WordSketches.WSThes = value
	return nil
}

// GetCorpusInfo provides miscellaneous corpus installation information mostly
// related to different data files.
// It should return an error only in case Manatee or filesystem produces some
// error (i.e. not in case something is just not found).
func GetCorpusInfo(corpusID string, wsattr string, setup *CorporaSetup) (*Info, error) {
	ans := &Info{ID: corpusID}
	ans.IndexedData = Data{}
	ans.RegistryConf = RegistryConf{Paths: make([]FileMappedValue, 0, 10)}
	vertical, err := findVerticalFile(setup.VerticalFilesDirPath, corpusID)
	if err != nil {
		return nil, err
	}
	ans.RegistryConf.Vertical = vertical
	ans.RegistryConf.SubcorpAttrs = make(map[string][]string)

	corpReg, err := registry.EnsureValidDataRegistry(setup, corpusID)
	isFile := true
	if err != nil {
		if errors.Is(err, registry.RegistryNotFound) {
			isFile = false

		} else {
			return nil, InfoError{err}
		}
	}

	if isFile {
		value, err := bindValueToPath(corpReg, corpReg)
		if err != nil {
			return nil, InfoError{err}
		}
		ans.RegistryConf.Paths = append(ans.RegistryConf.Paths, value)
		corp, err := mango.OpenCorpus(corpReg)
		if err != nil {
			// call of registry.EnsureValidDataRegistry should make this
			// almost impossible. In case this happens, we consider the
			// error "internal" (i.e. no "not found" here)
			if strings.Contains(err.Error(), "CorpInfoNotFound") {
				return nil, InfoError{fmt.Errorf("Manatee cannot open/find corpus %s", corpusID)}

			}
			return nil, InfoError{err}
		}

		defer mango.CloseCorpus(corp)
		ans.IndexedData.Size, err = mango.GetCorpusSize(corp)
		if err != nil {
			if !strings.Contains(err.Error(), "FileAccessError") {
				return nil, InfoError{err}
			}
			errStr := err.Error()
			ans.IndexedData.ManateeError = &errStr
		}
		corpDataPath, err := mango.GetCorpusConf(corp, "PATH")
		if err != nil {
			return nil, InfoError{err}
		}
		dataDirPath := filepath.Clean(corpDataPath)
		dataDirMtime, err := fs.GetFileMtime(dataDirPath)
		if err != nil {
			return nil, InfoError{err}
		}
		dataDirMtimeR := dataDirMtime.Format("2006-01-02T15:04:05-0700")
		isDir, err := fs.IsDir(dataDirPath)
		if err != nil {
			return nil, InfoError{err}
		}
		size, err := fs.FileSize(dataDirPath)
		if err != nil {
			return nil, InfoError{err}
		}
		ans.IndexedData.Path = FileMappedValue{
			Value:        dataDirPath,
			LastModified: &dataDirMtimeR,
			FileExists:   isDir,
			Size:         size,
		}

		// get encoding
		ans.RegistryConf.Encoding, err = mango.GetCorpusConf(corp, "ENCODING")
		if err != nil {
			return nil, InfoError{err}
		}

		// parse SUBCORPATTRS
		subcorpAttrsString, err := mango.GetCorpusConf(corp, "SUBCORPATTRS")
		if err != nil {
			return nil, InfoError{err}
		}
		if subcorpAttrsString != "" {
			for _, attr1 := range strings.Split(subcorpAttrsString, "|") {
				for _, attr2 := range strings.Split(attr1, ",") {
					split := strings.Split(attr2, ".")
					ans.RegistryConf.SubcorpAttrs[split[0]] = append(ans.RegistryConf.SubcorpAttrs[split[0]], split[1])
				}
			}
		}

		unparsedStructs, err := mango.GetCorpusConf(corp, "STRUCTLIST")
		if err != nil {
			return nil, InfoError{err}
		}
		if unparsedStructs != "" {
			structs := strings.Split(unparsedStructs, ",")
			ans.IndexedStructs = make([]string, len(structs))
			for i, st := range structs {
				ans.IndexedStructs[i] = st
			}
		}

		// try registry's VERTICAL
		regVertical, err := mango.GetCorpusConf(corp, "VERTICAL")
		if err != nil {
			return nil, InfoError{err}
		}
		if regVertical != "" && ans.RegistryConf.Vertical.Path != regVertical {
			if ans.RegistryConf.Vertical.FileExists {
				log.Warn().Msgf(
					"Registry file likely provides an incorrect VERTICAL %s",
					regVertical,
				)
				log.Warn().Msgf(
					"MQUERY will keep using inferred file %s for %s",
					ans.RegistryConf.Vertical.Path,
					corpusID,
				)

			} else {
				ans.RegistryConf.Vertical.Value = regVertical
				ans.RegistryConf.Vertical.Path = regVertical
				ans.RegistryConf.Vertical.FileExists = false
				ans.RegistryConf.Vertical.LastModified = nil
				ans.RegistryConf.Vertical.Size = 0
			}
		}

	} else {
		dataDirPath := filepath.Clean(filepath.Join(setup.CorpusDataPath.Abstract, corpusID))
		isDir, err := fs.IsDir(dataDirPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, InfoError{err}
		}
		var dataDirMtimeR *string
		if isDir {
			dataDirMtime, err := fs.GetFileMtime(dataDirPath)
			if err != nil {
				return nil, InfoError{err}
			}
			tmp := dataDirMtime.Format("2006-01-02T15:04:05-0700")
			dataDirMtimeR = &tmp
		}
		ans.IndexedData.Size = 0
		ans.IndexedData.Path = FileMappedValue{
			Value:        dataDirPath,
			LastModified: dataDirMtimeR,
			FileExists:   isDir,
			Path:         dataDirPath,
		}
	}

	// -----
	err = attachWordSketchConfInfo(corpusID, wsattr, setup, ans)
	if err != nil {
		return nil, InfoError{err}
	}
	return ans, nil
}

func OpenCorpus(corpusID string, setup *CorporaSetup) (*mango.GoCorpus, error) {
	for _, regPathRoot := range setup.RegistryDirPaths {
		regPath := filepath.Join(regPathRoot, corpusID)
		isFile, err := fs.IsFile(regPath)
		if err != nil {
			return nil, InfoError{err}
		}
		if isFile {
			corp, err := mango.OpenCorpus(regPath)
			if err != nil {
				if strings.Contains(err.Error(), "CorpInfoNotFound") {
					return nil, CorpusNotFound

				}
				return nil, CorpusError{err}
			}
			return corp, nil
		}
	}
	return nil, CorpusNotFound
}

func GetCorpusAttrs(corpusID string, setup *CorporaSetup) ([]string, error) {

	corp, err := OpenCorpus(corpusID, setup)
	if err != nil {
		return []string{}, err
	}

	unparsedStructs, err := mango.GetCorpusConf(corp, "ATTRLIST")
	if err != nil {
		return nil, InfoError{err}
	}
	if unparsedStructs != "" {
		return strings.Split(unparsedStructs, ","), nil
	}
	return nil, nil
}
