package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	tmtypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type FileUploader interface {
	UploadFile(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, data []byte) (string, error)
	UploadFileStream(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, body io.Reader, size int64) (string, error)

	Delete(ctx context.Context, key string) error
}

type S3FileUploader struct {
	client *transfermanager.Client
	rawS3  *s3.Client
	bucket string
}

func NewS3FileUploader(client *s3.Client, bucket string) *S3FileUploader {
	return &S3FileUploader{
		client: transfermanager.New(client),
		rawS3:  client,
		bucket: strings.TrimSpace(bucket),
	}
}

func (u *S3FileUploader) Delete(ctx context.Context, key string) error {
	if u.bucket == "" {
		return errors.New("s3 bucket is not configured")
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("key is required")
	}
	_, err := u.rawS3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (u *S3FileUploader) UploadFile(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, data []byte) (string, error) {
	return u.UploadFileStream(ctx, prefix, resourceID, purpose, fileName, contentType, bytes.NewReader(data), int64(len(data)))
}

func (u *S3FileUploader) UploadFileStream(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, body io.Reader, size int64) (string, error) {
	if u.bucket == "" {
		return "", errors.New("s3 bucket is not configured")
	}

	if size == 0 {
		return "", errors.New("file is empty")
	}

	var key string
	longCache := false
	if strings.HasPrefix(purpose, "/") || strings.HasPrefix(purpose, "movies/") || strings.HasPrefix(purpose, "tv/") {
		candidate := strings.TrimPrefix(purpose, "/")
		if !isSafeKey(candidate) {
			return "", errors.New("invalid s3 key")
		}
		key = candidate

		longCache = true
	} else if resourceID == "" {
		return "", errors.New("resource id is required for file upload")
	} else {
		normalizedType, ext, ok := normalizeStreamContentType(contentType, fileName)
		if !ok {
			return "", errors.New("unsupported file type; allowed: jpeg, png, webp, gif, avif, mp4, mov, avi, mkv, webm")
		}

		keyPrefix := sanitizeSegment(strings.Trim(strings.TrimSpace(prefix), "/"))
		cleanResource := sanitizeSegment(resourceID)
		cleanPurpose := sanitizeSegment(purpose)
		if keyPrefix == "" || cleanResource == "" || cleanPurpose == "" {
			return "", errors.New("invalid s3 key segment")
		}
		key = fmt.Sprintf("%s/%s/%s-%s.%s", keyPrefix, cleanResource, cleanPurpose, uuid.NewString()[:8], ext)
		contentType = normalizedType
	}

	cacheControl := "private, max-age=3600"
	if longCache {
		cacheControl = "public, max-age=31536000, immutable"
	}

	_, err := u.client.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:       aws.String(u.bucket),
		Key:          aws.String(key),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String(cacheControl),

		ServerSideEncryption: tmtypes.ServerSideEncryptionAes256,
		Metadata: map[string]string{
			"resource-id": asciiSanitizeMetadata(resourceID),
			"purpose":     asciiSanitizeMetadata(purpose),
		},
	})
	if err != nil {
		return "", fmt.Errorf("upload file to s3: %w", err)
	}

	return key, nil
}

func sanitizeSegment(in string) string {
	s := strings.TrimSpace(in)
	if s == "" || s == "." || s == ".." {
		return ""
	}
	if strings.ContainsAny(s, "/\\\x00") {
		return ""
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return s
}

// asciiSanitizeMetadata replaces any byte outside of printable ASCII
// (0x20-0x7E) with '_' so the value is always a valid HTTP header value
// (S3 user-defined object metadata is sent as HTTP headers).
func asciiSanitizeMetadata(s string) string {
	if s == "" {
		return s
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c > 0x7E {
			b = append(b, '_')
			continue
		}
		b = append(b, c)
	}
	return string(b)
}

func isSafeKey(k string) bool {
	if k == "" || strings.HasPrefix(k, "/") || strings.Contains(k, "\\") {
		return false
	}
	if path.Clean(k) != k {
		return false
	}
	for _, seg := range strings.Split(k, "/") {
		if sanitizeSegment(seg) == "" {
			return false
		}
	}
	return true
}

func normalizeStreamContentType(contentType string, fileName string) (string, string, bool) {
	provided := strings.ToLower(strings.TrimSpace(contentType))
	if provided != "" {
		if idx := strings.Index(provided, ";"); idx > 0 {
			provided = strings.TrimSpace(provided[:idx])
		}
	}

	if isAllowedImageType(provided) || isAllowedVideoType(provided) {
		ext, ok := extensionByContentType(provided)
		return provided, ext, ok
	}

	if fileName == "" || !strings.Contains(fileName, ".") {
		return "", "", false
	}

	ext := strings.ToLower(strings.TrimPrefix(filepathExt(fileName), "."))
	if mediaType, ok := videoExtensionToContentType(ext); ok {
		return mediaType, ext, true
	}
	if mediaType, ok := imageExtensionToContentType(ext); ok {
		return mediaType, ext, true
	}
	return "", "", false
}

func filepathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}

func imageExtensionToContentType(ext string) (string, bool) {
	switch ext {
	case "jpg", "jpeg":
		return "image/jpeg", true
	case "png":
		return "image/png", true
	case "webp":
		return "image/webp", true
	case "gif":
		return "image/gif", true
	case "avif":
		return "image/avif", true
	default:
		return "", false
	}
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
