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
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/fs"
)

// CorporaDataPaths describes three
// different ways how paths to corpora
// data are specified:
// 1) CNC - a global storage path (typically slow but reliable)
// 2) Kontext - a special fast storage for KonText
// 3) abstract - a path for data consumers; points to either
// (1) or (2)
type CorporaDataPaths struct {
	Abstract string `json:"abstract"`
	CNC      string `json:"cnc"`
	Kontext  string `json:"kontext"`
}

// CorporaSetup defines mquery application configuration related
// to a corpus
type CorporaSetup struct {
	RegistryDirPaths     []string          `json:"registryDirPaths"`
	RegistryTmpDir       string            `json:"registryTmpDir"`
	CorpusDataPath       CorporaDataPaths  `json:"corpusDataPath"`
	AltAccessMapping     map[string]string `json:"altAccessMapping"` // registry => data mapping
	WordSketchDefDirPath string            `json:"wordSketchDefDirPath"`
	SyncAllowedCorpora   []string          `json:"syncAllowedCorpora"`
	VerticalFilesDirPath string            `json:"verticalFilesDirPath"`
	ManateeDynlibPath    string            `json:"manateeDynlibPath"`
}

func (cs *CorporaSetup) GetFirstValidRegistry(corpusID string) string {
	for _, dir := range cs.RegistryDirPaths {
		d := filepath.Join(dir, corpusID)
		pe := fs.PathExists(d)
		isf, _ := fs.IsFile(d)
		if pe && isf {
			return d
		}
	}
	return ""
}

func (cs *CorporaSetup) GetCorpusCNCDataPath() string {
	return cs.CorpusDataPath.CNC
}

func (cs *CorporaSetup) AllowsSyncForCorpus(name string) bool {
	for _, v := range cs.SyncAllowedCorpora {
		if v == name {
			return true
		}
	}
	return false
}

func (cs *CorporaSetup) SubdirIsInAltAccessMapping(subdir string) bool {
	_, ok := cs.AltAccessMapping[subdir]
	return ok
}

type DatabaseSetup struct {
	Host                     string `json:"host"`
	User                     string `json:"user"`
	Passwd                   string `json:"passwd"`
	Name                     string `json:"db"`
	OverrideCorporaTableName string `json:"overrideCorporaTableName"`
	OverridePCTableName      string `json:"overridePcTableName"`
}
