package storage

import (
	"errors"
	"net/http"
	"strings"
)

const SniffSize = 512

func VerifyMagicBytes(declared string, head []byte) error {
	declared = strings.ToLower(strings.TrimSpace(declared))
	if declared == "" {
		return errors.New("missing content type")
	}
	if len(head) == 0 {
		return errors.New("empty payload")
	}
	if len(head) > SniffSize {
		head = head[:SniffSize]
	}
	detected := http.DetectContentType(head)
	if idx := strings.Index(detected, ";"); idx > 0 {
		detected = detected[:idx]
	}
	detected = strings.ToLower(strings.TrimSpace(detected))

	if detected == "application/octet-stream" {
		return nil
	}

	declaredFamily := family(declared)
	detectedFamily := family(detected)
	if declaredFamily == "" || detectedFamily == "" {
		return errors.New("content type not allowed")
	}
	if declaredFamily != detectedFamily {
		return errors.New("declared content type does not match payload")
	}
	return nil
}

func family(ct string) string {
	if i := strings.Index(ct, "/"); i > 0 {
		return ct[:i]
	}
	return ""
}
