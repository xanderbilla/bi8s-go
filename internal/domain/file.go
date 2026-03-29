// Package domain holds generalized abstract entities shared across database, service, and parsing boundaries globally.
// This prevents structural code replication violating DRY principles securely.
package domain

// FileUploadInput encapsulates file metadata and raw byte content.
// This is used across HTTP parsing, Business Services, and Storage layers to preserve DRY principles natively without layer-bleeding.
type FileUploadInput struct {
	FileName    string
	ContentType string
	Data        []byte
}
