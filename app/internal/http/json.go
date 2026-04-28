package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

// Default upload size limits. Override at startup via ConfigureLimits; no
// init-time env reads keep this package side-effect free.
var (
	maxMultipartBodySize = int64(12_582_912)
	maxMultipartFileSize = int64(10_485_760)
	maxVideoBodySize     = int64(10_737_418_240)
	maxVideoFileSize     = int64(10_737_418_240)
)

// Limits groups the upload size caps consumed by Parse* helpers.
type Limits struct {
	MultipartBodyBytes int64
	MultipartFileBytes int64
	VideoBodyBytes     int64
	VideoFileBytes     int64
}

// ConfigureLimits overrides package defaults. Non-positive values keep the
// corresponding default.
func ConfigureLimits(l Limits) {
	if l.MultipartBodyBytes > 0 {
		maxMultipartBodySize = l.MultipartBodyBytes
	}
	if l.MultipartFileBytes > 0 {
		maxMultipartFileSize = l.MultipartFileBytes
	}
	if l.VideoBodyBytes > 0 {
		maxVideoBodySize = l.VideoBodyBytes
	}
	if l.VideoFileBytes > 0 {
		maxVideoFileSize = l.VideoFileBytes
	}
}

// Decode reads a single JSON object from the request body. The maximum body
// size for JSON requests is enforced globally by the MaxBytesJSON middleware
// installed in the router.
func Decode(w http.ResponseWriter, r *http.Request, payload any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(payload); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("body must contain only one JSON object")
	}
	return nil
}

func ParseMultipartForm(r *http.Request, w http.ResponseWriter) (url.Values, error) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		return nil, errors.New("body must be multipart/form-data and under 12 MB")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBodySize)

	if err := r.ParseMultipartForm(maxMultipartBodySize); err != nil {
		return nil, errors.New("body must be multipart/form-data and under 12 MB")
	}
	return r.Form, nil
}

func ExtractFile(r *http.Request, fieldName string) (*model.FileUploadInput, error) {
	return validation.ExtractFileToTemp(r, fieldName, maxMultipartFileSize)
}

func ParseVideoMultipartForm(r *http.Request, w http.ResponseWriter) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		return errors.New("body must be multipart/form-data")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxVideoBodySize)
	return r.ParseMultipartForm(32 << 20)
}
