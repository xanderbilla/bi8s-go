package service

import (
	"context"
	"fmt"
	"os"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/storage"
)

func cleanupUploadedKeys(ctx context.Context, uploader storage.FileUploader, keys []string) {
	if uploader == nil || len(keys) == 0 {
		return
	}
	for _, k := range keys {
		if err := uploader.Delete(ctx, k); err != nil {
			logger.WarnContext(ctx, "rollback delete failed", "key", k, "error", err.Error())
		}
	}
}

func uploadInputToStorage(
	ctx context.Context,
	uploader storage.FileUploader,
	prefix, resourceID, purpose string,
	input *model.FileUploadInput,
) (string, error) {
	if uploader == nil {
		return "", errs.ErrFileUploaderNotConfigured
	}

	if input.TempFilePath != "" {
		defer os.Remove(input.TempFilePath)
		f, err := os.Open(input.TempFilePath)
		if err != nil {
			return "", fmt.Errorf("open temp %s for %s %s: %w", purpose, prefix, resourceID, err)
		}
		defer f.Close()
		return uploader.UploadFileStream(
			ctx,
			prefix,
			resourceID,
			purpose,
			input.FileName,
			input.ContentType,
			f,
			input.Size,
		)
	}

	return uploader.UploadFile(
		ctx,
		prefix,
		resourceID,
		purpose,
		input.FileName,
		input.ContentType,
		input.Data,
	)
}
