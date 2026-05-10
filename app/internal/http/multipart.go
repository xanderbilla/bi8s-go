package http

import (
	"net/http"
	"net/url"

	"github.com/xanderbilla/bi8s-go/internal/model"
)

func ParseFormAndFiles(
	w http.ResponseWriter,
	r *http.Request,
	fileFields []string,
) (url.Values, map[string]*model.FileUploadInput, error) {
	formValues, err := ParseMultipartForm(r, w)
	if err != nil {
		return nil, nil, err
	}

	files := make(map[string]*model.FileUploadInput, len(fileFields))
	for _, field := range fileFields {
		input, err := ExtractFile(r, field)
		if err != nil {
			return nil, nil, err
		}
		files[field] = input
	}
	return formValues, files, nil
}
