// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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

package corpus

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

type dirInfo struct {
	Path      string `json:"path"`
	LatestMod string `json:"latestMod"`
}

type syncResponse struct {
	OK             bool     `json:"ok"`
	ReturnCode     int      `json:"returnCode"`
	Details        []string `json:"details"`
	SourceDir      dirInfo  `json:"srcDir"`
	DestinationDir dirInfo  `json:"dstDir"`
}

// synchronizeCorpusData automatically synchronizes data from CNC to KonText or vice versa
// based on which directory contains newer files. The function is based on calling rsync.
func synchronizeCorpusData(paths *CorporaDataPaths, corpname string) (syncResponse, error) {
	pathCNC := filepath.Clean(filepath.Join(paths.CNC, corpname))
	var ageCNC time.Time
	pathKontext := filepath.Clean(filepath.Join(paths.Kontext, corpname))
	var ageKontext time.Time
	var numCNC, numKontext int

	isDir, err := fs.IsDir(pathCNC)
	if err != nil {
		return syncResponse{}, err
	}
	if isDir {
		files1, err := fs.ListFilesInDir(pathCNC, true)
		if err != nil {
			return syncResponse{}, err
		}
		if numCNC = files1.Len(); numCNC > 0 {
			ageCNC = files1.First().ModTime()
		}
	}

	isDir, err = fs.IsDir(pathKontext)
	if err != nil {
		return syncResponse{}, err
	}
	if isDir {
		files2, err := fs.ListFilesInDir(pathKontext, true)
		if err != nil {
			return syncResponse{}, err
		}
		if numKontext = files2.Len(); numKontext > 0 {
			ageKontext = files2.First().ModTime()
		}
	}
	if ageCNC.IsZero() && ageKontext.IsZero() {
		return syncResponse{}, fmt.Errorf("Neither KonText (%s) nor CNC (%s) directory exists", pathKontext, pathCNC)
	}

	var srcPath, dstPath string
	if ageKontext.After(ageCNC) {
		srcPath = pathKontext
		dstPath = pathCNC

	} else if ageCNC.After(ageKontext) {
		srcPath = pathCNC
		dstPath = pathKontext

	} else if numCNC < numKontext {
		log.Warn().Msg("data sync anomaly - same file age but different num of files in src and dest")
		srcPath = pathKontext
		dstPath = pathCNC

	} else if numKontext < numCNC {
		log.Warn().Msg("data sync anomaly - same file age but different num of files in src and dest")
		srcPath = pathCNC
		dstPath = pathKontext

	} else {
		return syncResponse{},
			fmt.Errorf(
				"Nothing to synchronize - latest changes in both CNC and KonText data dirs have the same modification date %v",
				ageCNC.Format(time.RFC3339))
	}
	cmd := exec.Command("rsync", "-av", fmt.Sprintf("%s/", srcPath), dstPath)
	cmd.Env = os.Environ()
	var stdOut, errOut bytes.Buffer
	cmd.Stdout = &stdOut
	cmd.Stderr = &errOut
	err = cmd.Run()

	ans := syncResponse{
		OK: err == nil,
		SourceDir: dirInfo{
			Path:      srcPath,
			LatestMod: ageCNC.Format(time.RFC3339),
		},
		DestinationDir: dirInfo{
			Path:      dstPath,
			LatestMod: ageKontext.Format(time.RFC3339),
		},
	}

	exitErr, ok := err.(*exec.ExitError)
	if ok {
		ans.ReturnCode = exitErr.ExitCode()

	} else {
		ans.ReturnCode = -1
	}

	if err != nil {
		ans.Details = strings.Split(errOut.String(), "\n")
		return ans, err
	}
	ans.Details = strings.Split(stdOut.String(), "\n")
	return ans, nil
}
