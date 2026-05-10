package model

type FileUploadInput struct {
	FileName     string
	ContentType  string
	Data         []byte
	TempFilePath string
	Size         int64
}
