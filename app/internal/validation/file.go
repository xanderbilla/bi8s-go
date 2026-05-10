package validation

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/utils"
)

var allowedUploadMIME = map[string]string{
	"image/jpeg":       "image/jpeg",
	"image/png":        "image/png",
	"image/webp":       "image/webp",
	"image/gif":        "image/gif",
	"image/avif":       "image/avif",
	"video/mp4":        "video/mp4",
	"video/quicktime":  "video/quicktime",
	"video/x-msvideo":  "video/x-msvideo",
	"video/x-matroska": "video/x-matroska",
	"video/webm":       "video/webm",
}

var allowedUploadExt = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".webp": {}, ".gif": {}, ".avif": {},
	".mp4": {}, ".mov": {}, ".avi": {}, ".mkv": {}, ".webm": {},
}

func ExtractFile(r *http.Request, fieldName string, maxSize int64) (*model.FileUploadInput, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, errors.New(fieldName + " file is invalid or missing")
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.WarnContext(r.Context(), "failed to close uploaded file", "field", fieldName, "error", err.Error())
		}
	}()

	fileData, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		return nil, errors.New("failed to read " + fieldName + " file")
	}

	if len(fileData) == 0 {
		return nil, errors.New(fieldName + " file is empty")
	}

	if len(fileData) > int(maxSize) {
		return nil, errors.New(fieldName + " file exceeds max size limit")
	}

	ct, err := sniffAndValidate(fileData, header.Header.Get("Content-Type"), header.Filename)
	if err != nil {
		return nil, errors.New(fieldName + ": " + err.Error())
	}

	return &model.FileUploadInput{
		FileName:    header.Filename,
		ContentType: ct,
		Data:        fileData,
	}, nil
}

func ExtractFileToTemp(r *http.Request, fieldName string, maxSize int64) (*model.FileUploadInput, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, errors.New(fieldName + " file is invalid or missing")
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.WarnContext(r.Context(), "failed to close uploaded file", "field", fieldName, "error", err.Error())
		}
	}()

	tmp, err := os.CreateTemp(utils.TmpDir(), fieldName+"-*."+trimmedExt(header.Filename))
	if err != nil {
		return nil, errors.New("failed to create temp file for " + fieldName)
	}

	var cleanup bool
	defer func() {
		if closeErr := tmp.Close(); closeErr != nil {
			logger.WarnContext(r.Context(), "failed to close temp file",
				"field", fieldName, "temp_file", tmp.Name(), "error", closeErr.Error())
		}
		if cleanup {
			if rmErr := os.Remove(tmp.Name()); rmErr != nil && !os.IsNotExist(rmErr) {
				logger.WarnContext(r.Context(), "failed to remove temp file",
					"field", fieldName, "temp_file", tmp.Name(), "error", rmErr.Error())
			}
		}
	}()

	written, err := io.Copy(tmp, io.LimitReader(file, maxSize+1))
	if err != nil {
		cleanup = true
		return nil, errors.New("failed to stream " + fieldName + " file")
	}
	if written == 0 {
		cleanup = true
		return nil, errors.New(fieldName + " file is empty")
	}
	if written > maxSize {
		cleanup = true
		return nil, errors.New(fieldName + " file exceeds max size limit")
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		cleanup = true
		return nil, errors.New("failed to rewind temp file for " + fieldName)
	}

	sniffBuf := make([]byte, 512)
	n, _ := io.ReadFull(tmp, sniffBuf)
	if _, err := tmp.Seek(0, 0); err != nil {
		cleanup = true
		return nil, errors.New("failed to rewind temp file for " + fieldName)
	}
	ct, err := sniffAndValidate(sniffBuf[:n], header.Header.Get("Content-Type"), header.Filename)
	if err != nil {
		cleanup = true
		return nil, errors.New(fieldName + ": " + err.Error())
	}

	return &model.FileUploadInput{
		FileName:     header.Filename,
		ContentType:  ct,
		TempFilePath: tmp.Name(),
		Size:         written,
	}, nil
}

func sniffAndValidate(head []byte, declaredCT, fileName string) (string, error) {
	if len(head) == 0 {
		return "", errors.New("file is empty")
	}
	sniffed := http.DetectContentType(head)

	if i := strings.Index(sniffed, ";"); i >= 0 {
		sniffed = strings.TrimSpace(sniffed[:i])
	}
	canonical, ok := allowedUploadMIME[sniffed]
	if !ok {

		if c, ok2 := allowedUploadMIME[strings.ToLower(strings.TrimSpace(declaredCT))]; ok2 && sniffed == "application/octet-stream" {
			canonical = c
		} else {
			return "", errors.New("unsupported content type: " + sniffed)
		}
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if _, ok := allowedUploadExt[ext]; !ok {
		return "", errors.New("unsupported file extension: " + ext)
	}
	return canonical, nil
}

func trimmedExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return "bin"
	}
	return ext[1:]
}
