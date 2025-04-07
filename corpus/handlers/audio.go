package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

var (
	rangeRegexp = regexp.MustCompile(`^[bB]ytes=(\d+)-(\d*)$`)
)

func (a *Actions) getRange(hdr http.Header) (lft, rgt int, err error) {
	lft, rgt = -1, -1
	rng := hdr.Get("range")
	if rng == "" {
		lft = 0
		return
	}
	srch := rangeRegexp.FindStringSubmatch(rng)
	if len(srch) > 0 {
		lft, err = strconv.Atoi(srch[1])
		if err != nil {
			err = fmt.Errorf("failed to process audio range: %w", err)
			return
		}
		if srch[2] != "" {
			rgt, err = strconv.Atoi(srch[2])
			if err != nil {
				err = fmt.Errorf("failed to process audio range: %w", err)
				return
			}
		}
		return
	}
	err = fmt.Errorf("range expression not recognized")
	return
}

func (a *Actions) Audio(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	corpConf := a.conf.Resources.Get(corpusID)
	if !corpConf.HasPublicAudio {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("no audio data for %s", corpusID),
			http.StatusInternalServerError,
		)
		return
	}

	chunk := ctx.Request.URL.Query().Get("chunk")
	fPath := filepath.Join(
		a.conf.AudioFilesDir,
		corpusID,
		chunk[:2],
		chunk,
	)
	fPath = filepath.Clean(fPath)
	fPath, err := filepath.EvalSymlinks(fPath)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("failed to resolve chunk ID"),
			http.StatusNotFound,
		)
		return
	}
	if !strings.HasPrefix(fPath, a.conf.AudioFilesDir) {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("unknown chunk ID"),
			http.StatusNotFound,
		)
		return
	}
	fileSize, err := fs.FileSize(fPath)
	fmt.Println("FILE: ", fPath, ", size: ", fileSize)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("failed to determine chunk size"),
			http.StatusInternalServerError,
		)
		return
	}
	ctx.Writer.Header().Set("Content-Type", "audio/mpeg")
	ctx.Writer.Header().Set("Content-Length", strconv.Itoa(int(fileSize)))
	ctx.Writer.Header().Set("Accept-Ranges", "bytes")

	lft, rgt, err := a.getRange(ctx.Request.Header)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("failed to process data range"),
			http.StatusUnprocessableEntity,
		)
		return
	}

	if rgt < lft && rgt > -1 {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("invalid range %d .. %d", rgt, lft),
			http.StatusBadRequest,
		)
		return
	}

	if rgt == -1 {
		rgt = int(fileSize)
	}
	f, err := os.Open(fPath)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("failed to open audio file: %w", err),
			http.StatusInternalServerError,
		)
		return
	}

	if lft > 0 {
		if _, err := f.Seek(int64(lft), io.SeekStart); err != nil {
			uniresp.RespondWithErrorJSON(
				ctx,
				fmt.Errorf("failed to seek audio file position: %w", err),
				http.StatusInternalServerError,
			)
			return
		}
	}

	if ctx.Request.Header.Get("range") != "" {
		ctx.Writer.Header().Set(
			"Content-Range",
			fmt.Sprintf("bytes 0-%d/%d", fileSize-1, fileSize),
		)
	}

	limitedReader := io.LimitReader(f, int64(rgt-lft))
	buffer := make([]byte, int64(rgt-lft))
	_, err = limitedReader.Read(buffer)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("failed to read audio file: %w", err),
			http.StatusInternalServerError,
		)
		return
	}
	_, err = ctx.Writer.Write(buffer)
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			fmt.Errorf("failed to return audio file: %w", err),
			http.StatusInternalServerError,
		)
		return
	}
	ctx.Status(http.StatusPartialContent)
}
