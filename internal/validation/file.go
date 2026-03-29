package validation

import (
	"errors"
	"io"
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/domain"
)

// ExtractFile extracts a file from a multipart request by field name with size validation.
// This is a generic helper that can be reused for any file upload (poster, trailer, etc.).
// Use this after r.ParseMultipartForm() has been called.
func ExtractFile(r *http.Request, fieldName string, maxSize int64) (*domain.FileUploadInput, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil // Optional file was not provided
		}
		return nil, errors.New(fieldName + " file is invalid or missing")
	}
	defer file.Close()

	// Read file with size enforcement
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

	return &domain.FileUploadInput{
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Data:        fileData,
	}, nil
}
