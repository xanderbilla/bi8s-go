package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xanderbilla/bi8s-go/internal/model"
)

func TestMovieService_UploadAssets_PartialSuccess(t *testing.T) {

	movieRepo := newMockMovieRepository()
	personRepo := newMockPersonRepository()
	attributeRepo := newMockAttributeRepository()
	encoderRepo := newMockEncoderRepository()
	fileUploader := &failingMockFileUploader{failAfter: 2}

	service := NewMovieService(movieRepo, personRepo, attributeRepo, encoderRepo, fileUploader)

	movie := model.Movie{
		ID:          "movie123",
		Title:       "Test Movie",
		ContentType: model.ContentTypeMovie,
		Assets:      []model.Asset{},
	}
	movieRepo.movies["movie123"] = movie

	files := buildMultipartFileHeaders(t, "videos", []string{"trailer1.mp4", "trailer2.mp4", "trailer3.mp4", "trailer4.mp4"})

	ctx := context.Background()
	uploadedPaths, err := service.UploadAssets(ctx, "movie123", model.AssetTypeTrailer, files)

	if err == nil {
		t.Error("Expected error from partial upload failure")
	}

	if len(uploadedPaths) != 2 {
		t.Errorf("Expected 2 successful uploads, got %d", len(uploadedPaths))
	}

	updatedMovie, exists := movieRepo.movies["movie123"]
	if !exists {
		t.Fatal("Movie should exist in repository")
	}

	if len(updatedMovie.Assets) != 1 {
		t.Errorf("Expected 1 asset type, got %d", len(updatedMovie.Assets))
	}

	if len(updatedMovie.Assets) > 0 {
		if updatedMovie.Assets[0].Type != model.AssetTypeTrailer {
			t.Errorf("Expected asset type TRAILER, got %v", updatedMovie.Assets[0].Type)
		}
		if len(updatedMovie.Assets[0].Keys) != 2 {
			t.Errorf("Expected 2 asset keys, got %d", len(updatedMovie.Assets[0].Keys))
		}
	}
}

func TestMovieService_UploadAssets_AllSuccess(t *testing.T) {
	movieRepo := newMockMovieRepository()
	personRepo := newMockPersonRepository()
	attributeRepo := newMockAttributeRepository()
	encoderRepo := newMockEncoderRepository()
	fileUploader := newMockFileUploader()

	service := NewMovieService(movieRepo, personRepo, attributeRepo, encoderRepo, fileUploader)

	movie := model.Movie{
		ID:          "movie123",
		Title:       "Test Movie",
		ContentType: model.ContentTypeMovie,
		Assets:      []model.Asset{},
	}
	movieRepo.movies["movie123"] = movie

	files := buildMultipartFileHeaders(t, "videos", []string{"trailer1.mp4", "trailer2.mp4", "trailer3.mp4"})

	ctx := context.Background()
	uploadedPaths, err := service.UploadAssets(ctx, "movie123", model.AssetTypeTrailer, files)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(uploadedPaths) != 3 {
		t.Errorf("Expected 3 successful uploads, got %d", len(uploadedPaths))
	}

	updatedMovie, exists := movieRepo.movies["movie123"]
	if !exists {
		t.Fatal("Movie should exist in repository")
	}

	if len(updatedMovie.Assets) != 1 {
		t.Errorf("Expected 1 asset type, got %d", len(updatedMovie.Assets))
	}

	if len(updatedMovie.Assets) > 0 {
		if len(updatedMovie.Assets[0].Keys) != 3 {
			t.Errorf("Expected 3 asset keys, got %d", len(updatedMovie.Assets[0].Keys))
		}
	}
}

func TestMovieService_UploadAssets_FirstFileFails(t *testing.T) {
	movieRepo := newMockMovieRepository()
	personRepo := newMockPersonRepository()
	attributeRepo := newMockAttributeRepository()
	encoderRepo := newMockEncoderRepository()
	fileUploader := &failingMockFileUploader{failAfter: 0}

	service := NewMovieService(movieRepo, personRepo, attributeRepo, encoderRepo, fileUploader)

	movie := model.Movie{
		ID:          "movie123",
		Title:       "Test Movie",
		ContentType: model.ContentTypeMovie,
		Assets:      []model.Asset{},
	}
	movieRepo.movies["movie123"] = movie

	files := buildMultipartFileHeaders(t, "videos", []string{"trailer1.mp4", "trailer2.mp4"})

	ctx := context.Background()
	uploadedPaths, err := service.UploadAssets(ctx, "movie123", model.AssetTypeTrailer, files)

	if err == nil {
		t.Error("Expected error from upload failure")
	}

	if len(uploadedPaths) != 0 {
		t.Errorf("Expected 0 successful uploads, got %d", len(uploadedPaths))
	}

	updatedMovie, exists := movieRepo.movies["movie123"]
	if !exists {
		t.Fatal("Movie should exist in repository")
	}

	if len(updatedMovie.Assets) != 0 {
		t.Errorf("Expected 0 assets, got %d", len(updatedMovie.Assets))
	}
}

func TestMovieService_UploadAssets_AppendToExisting(t *testing.T) {
	movieRepo := newMockMovieRepository()
	personRepo := newMockPersonRepository()
	attributeRepo := newMockAttributeRepository()
	encoderRepo := newMockEncoderRepository()
	fileUploader := newMockFileUploader()

	service := NewMovieService(movieRepo, personRepo, attributeRepo, encoderRepo, fileUploader)

	movie := model.Movie{
		ID:          "movie123",
		Title:       "Test Movie",
		ContentType: model.ContentTypeMovie,
		Assets: []model.Asset{
			{
				Type: model.AssetTypeTrailer,
				Keys: []string{"/existing/trailer1.mp4", "/existing/trailer2.mp4"},
			},
		},
	}
	movieRepo.movies["movie123"] = movie

	files := buildMultipartFileHeaders(t, "videos", []string{"trailer3.mp4", "trailer4.mp4"})

	ctx := context.Background()
	uploadedPaths, err := service.UploadAssets(ctx, "movie123", model.AssetTypeTrailer, files)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(uploadedPaths) != 2 {
		t.Errorf("Expected 2 successful uploads, got %d", len(uploadedPaths))
	}

	updatedMovie, exists := movieRepo.movies["movie123"]
	if !exists {
		t.Fatal("Movie should exist in repository")
	}

	if len(updatedMovie.Assets) != 1 {
		t.Errorf("Expected 1 asset type, got %d", len(updatedMovie.Assets))
	}

	if len(updatedMovie.Assets) > 0 {
		if len(updatedMovie.Assets[0].Keys) != 4 {
			t.Errorf("Expected 4 total asset keys, got %d", len(updatedMovie.Assets[0].Keys))
		}
	}
}

type failingMockFileUploader struct {
	failAfter int
	count     int
}

func (m *failingMockFileUploader) UploadFile(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, data []byte) (string, error) {
	if m.count >= m.failAfter {
		return "", errors.New("simulated upload failure")
	}
	m.count++
	return prefix + "/" + resourceID + "/" + fileName, nil
}

func (m *failingMockFileUploader) UploadFileStream(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, body io.Reader, size int64) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	return m.UploadFile(ctx, prefix, resourceID, purpose, fileName, contentType, data)
}

func (m *failingMockFileUploader) Delete(ctx context.Context, key string) error {
	return nil
}

type mockMovieRepository struct {
	movies map[string]model.Movie
}

type mockPersonRepository struct {
	people map[string]model.Person
}

func newMockPersonRepository() *mockPersonRepository {
	return &mockPersonRepository{people: make(map[string]model.Person)}
}

func (m *mockPersonRepository) GetAll(ctx context.Context) ([]model.Person, error) {
	result := make([]model.Person, 0, len(m.people))
	for _, p := range m.people {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockPersonRepository) Get(ctx context.Context, id string) (*model.Person, error) {
	p, ok := m.people[id]
	if !ok {
		return nil, nil
	}
	return &p, nil
}

func (m *mockPersonRepository) Create(ctx context.Context, person model.Person) error {
	m.people[person.ID] = person
	return nil
}

func (m *mockPersonRepository) Delete(ctx context.Context, id string) error {
	delete(m.people, id)
	return nil
}

type mockAttributeRepository struct {
	attributes map[string]model.Attribute
}

func newMockAttributeRepository() *mockAttributeRepository {
	return &mockAttributeRepository{attributes: make(map[string]model.Attribute)}
}

func (m *mockAttributeRepository) GetAll(ctx context.Context) ([]model.Attribute, error) {
	result := make([]model.Attribute, 0, len(m.attributes))
	for _, a := range m.attributes {
		result = append(result, a)
	}
	return result, nil
}

func (m *mockAttributeRepository) Get(ctx context.Context, id string) (*model.Attribute, error) {
	a, ok := m.attributes[id]
	if !ok {
		return nil, nil
	}
	return &a, nil
}

func (m *mockAttributeRepository) GetByName(ctx context.Context, name string) (*model.Attribute, error) {
	for _, a := range m.attributes {
		if a.Name == name {
			attr := a
			return &attr, nil
		}
	}
	return nil, nil
}

func (m *mockAttributeRepository) Create(ctx context.Context, attribute model.Attribute) error {
	m.attributes[attribute.ID] = attribute
	return nil
}

func (m *mockAttributeRepository) Delete(ctx context.Context, id string) error {
	delete(m.attributes, id)
	return nil
}

func newMockMovieRepository() *mockMovieRepository {
	return &mockMovieRepository{
		movies: make(map[string]model.Movie),
	}
}

func (m *mockMovieRepository) GetAllAdmin(ctx context.Context) ([]model.Movie, error) {
	var movies []model.Movie
	for _, movie := range m.movies {
		movies = append(movies, movie)
	}
	return movies, nil
}

func (m *mockMovieRepository) GetRecentContent(ctx context.Context, contentTypeFilter string) ([]model.Movie, error) {
	return m.GetAllAdmin(ctx)
}

func (m *mockMovieRepository) Get(ctx context.Context, id string) (*model.Movie, error) {
	movie, exists := m.movies[id]
	if !exists {
		return nil, nil
	}
	return &movie, nil
}

func (m *mockMovieRepository) GetAdmin(ctx context.Context, id string) (*model.Movie, error) {
	return m.Get(ctx, id)
}

func (m *mockMovieRepository) Create(ctx context.Context, movie model.Movie) error {
	m.movies[movie.ID] = movie
	return nil
}

func (m *mockMovieRepository) Update(ctx context.Context, movie model.Movie) error {
	m.movies[movie.ID] = movie
	return nil
}

func (m *mockMovieRepository) Delete(ctx context.Context, id string) error {
	delete(m.movies, id)
	return nil
}

func (m *mockMovieRepository) GetMoviesByPersonId(ctx context.Context, personId string) ([]model.Movie, error) {
	return []model.Movie{}, nil
}

func (m *mockMovieRepository) GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	return []model.Movie{}, nil
}

func (m *mockMovieRepository) GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	return []model.Movie{}, nil
}

func (m *mockMovieRepository) GetMoviesByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string) ([]model.Movie, error) {
	return []model.Movie{}, nil
}

func (m *mockMovieRepository) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	return nil, nil
}

func (m *mockMovieRepository) GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string) ([]model.Movie, error) {
	return []model.Movie{}, nil
}

func buildMultipartFileHeaders(t *testing.T, field string, names []string) []*multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, name := range names {
		part, err := writer.CreateFormFile(field, name)
		if err != nil {
			t.Fatalf("failed to create form file %s: %v", name, err)
		}
		if _, err := io.WriteString(part, "fake video bytes"); err != nil {
			t.Fatalf("failed to write form file %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("failed to parse multipart form: %v", err)
	}

	files := req.MultipartForm.File[field]
	if len(files) != len(names) {
		t.Fatalf("unexpected file count: got %d want %d", len(files), len(names))
	}
	return files
}
