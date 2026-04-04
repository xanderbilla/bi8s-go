// Package storage controls raw data persistence onto separate infrastructures exclusively.
// It sits distinctly away from application services preventing AWS API pollution entirely.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// FileUploader defines a generic storage contract for file uploads.
type FileUploader interface {
	UploadFile(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, data []byte) (string, error)
}

// S3FileUploader uploads generic files to S3 and returns the object key.
type S3FileUploader struct {
	client *transfermanager.Client
	bucket string
}

// NewS3FileUploader creates a file uploader backed by S3 multipart upload manager.
func NewS3FileUploader(client *s3.Client, bucket string) *S3FileUploader {
	return &S3FileUploader{
		client: transfermanager.New(client),
		bucket: strings.TrimSpace(bucket),
	}
}

// UploadFile uploads file bytes to a dynamic key path and returns the generated object key.
func (u *S3FileUploader) UploadFile(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, data []byte) (string, error) {
	if u.bucket == "" {
		return "", errors.New("s3 bucket is not configured")
	}

	if len(data) == 0 {
		return "", errors.New("file is empty")
	}

	// If purpose contains a full path (starts with /), use it directly as the key
	var key string
	if strings.HasPrefix(purpose, "/") || strings.HasPrefix(purpose, "movies/") || strings.HasPrefix(purpose, "tv/") {
		key = strings.TrimPrefix(purpose, "/")
	} else if resourceID == "" {
		return "", errors.New("resource id is required for file upload")
	} else {
		// Try to normalize as image first, then video
		normalizedType, ext, ok := normalizeFileContentType(contentType, data, fileName)
		if !ok {
			return "", errors.New("unsupported file type; allowed: jpeg, png, webp, gif, avif, mp4, mov, avi, mkv, webm")
		}

		// build a generic S3 key: prefix/resourceId/purpose.ext
		keyPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
		key = fmt.Sprintf("%s/%s/%s.%s", keyPrefix, resourceID, purpose, ext)
		contentType = normalizedType
	}

	_, err := u.client.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:       aws.String(u.bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return "", fmt.Errorf("upload file to s3: %w", err)
	}

	return key, nil
}

func normalizeFileContentType(contentType string, data []byte, fileName string) (string, string, bool) {
	detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
	provided := strings.ToLower(strings.TrimSpace(contentType))

	if provided != "" {
		if idx := strings.Index(provided, ";"); idx > 0 {
			provided = strings.TrimSpace(provided[:idx])
		}
	}

	// Try image types first
	if isAllowedImageType(detected) {
		ext, _ := extensionByContentType(detected)
		return detected, ext, true
	}

	if isAllowedImageType(provided) {
		ext, _ := extensionByContentType(provided)
		return provided, ext, true
	}

	// Try video types
	if isAllowedVideoType(detected) {
		ext, _ := extensionByContentType(detected)
		return detected, ext, true
	}

	if isAllowedVideoType(provided) {
		ext, _ := extensionByContentType(provided)
		return provided, ext, true
	}

	// Fallback: try to get extension from filename
	if fileName != "" {
		ext := strings.ToLower(strings.TrimPrefix(strings.ToLower(fileName[strings.LastIndex(fileName, "."):]), "."))
		if videoExt, ok := videoExtensionToContentType(ext); ok {
			return videoExt, ext, true
		}
	}

	return "", "", false
}

func normalizeImageContentType(contentType string, data []byte) (string, bool) {
	detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
	provided := strings.ToLower(strings.TrimSpace(contentType))

	if provided != "" {
		if idx := strings.Index(provided, ";"); idx > 0 {
			provided = strings.TrimSpace(provided[:idx])
		}
	}

	if isAllowedImageType(detected) {
		return detected, true
	}

	if isAllowedImageType(provided) {
		return provided, true
	}

	return "", false
}

func isAllowedVideoType(contentType string) bool {
	switch contentType {
	case "video/mp4", "video/quicktime", "video/x-msvideo", "video/x-matroska", "video/webm":
		return true
	default:
		return false
	}
}

func videoExtensionToContentType(ext string) (string, bool) {
	switch ext {
	case "mp4":
		return "video/mp4", true
	case "mov":
		return "video/quicktime", true
	case "avi":
		return "video/x-msvideo", true
	case "mkv":
		return "video/x-matroska", true
	case "webm":
		return "video/webm", true
	default:
		return "", false
	}
}

func isAllowedImageType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp", "image/gif", "image/avif":
		return true
	default:
		return false
	}
}

func extensionByContentType(contentType string) (string, bool) {
	switch contentType {
	case "image/jpeg":
		return "jpg", true
	case "image/png":
		return "png", true
	case "image/webp":
		return "webp", true
	case "image/gif":
		return "gif", true
	case "image/avif":
		return "avif", true
	case "video/mp4":
		return "mp4", true
	case "video/quicktime":
		return "mov", true
	case "video/x-msvideo":
		return "avi", true
	case "video/x-matroska":
		return "mkv", true
	case "video/webm":
		return "webm", true
	default:
		return "", false
	}
}
