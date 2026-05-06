package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type fakeAttributeRepo struct {
	items []model.Attribute
}

func (f fakeAttributeRepo) GetAll(context.Context) ([]model.Attribute, error) {
	return f.items, nil
}

func (f fakeAttributeRepo) Get(context.Context, string) (*model.Attribute, error) {
	return nil, nil
}

func (f fakeAttributeRepo) GetByName(context.Context, string) (*model.Attribute, error) {
	return nil, nil
}

func (f fakeAttributeRepo) Create(context.Context, model.Attribute) error {
	return nil
}

func (f fakeAttributeRepo) Delete(context.Context, string) error {
	return nil
}

func TestGetConsumerAttributes_FilterGenresAlias(t *testing.T) {
	t.Parallel()

	h := NewAttributeHandler(service.NewAttributeService(fakeAttributeRepo{items: []model.Attribute{
		{ID: "g1", Name: "Action", AttributeType: []model.AttributeType{model.AttributeTypeGenre}, Active: true},
		{ID: "t1", Name: "Exciting", AttributeType: []model.AttributeType{model.AttributeTypeTag}, Active: true},
		{ID: "g2", Name: "Drama", AttributeType: []model.AttributeType{model.AttributeTypeGenre}, Active: false},
	}}))

	req := httptest.NewRequest(http.MethodGet, "/v1/c/attributes?type=genres", nil)
	rec := httptest.NewRecorder()
	h.GetConsumerAttributes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var env struct {
		Data []model.AttributePublicDetail `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(env.Data) != 1 {
		t.Fatalf("expected 1 active genre, got %d", len(env.Data))
	}
	if env.Data[0].ID != "g1" {
		t.Fatalf("expected g1, got %s", env.Data[0].ID)
	}
}

func TestGetConsumerAttributes_InvalidType(t *testing.T) {
	t.Parallel()

	h := NewAttributeHandler(service.NewAttributeService(fakeAttributeRepo{}))

	req := httptest.NewRequest(http.MethodGet, "/v1/c/attributes?type=whatever", nil)
	rec := httptest.NewRecorder()
	h.GetConsumerAttributes(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGetConsumerAttributes_SortAlphaAsc(t *testing.T) {
	t.Parallel()

	h := NewAttributeHandler(service.NewAttributeService(fakeAttributeRepo{items: []model.Attribute{
		{ID: "b", Name: "Drama", AttributeType: []model.AttributeType{model.AttributeTypeGenre}, Active: true},
		{ID: "a", Name: "Action", AttributeType: []model.AttributeType{model.AttributeTypeGenre}, Active: true},
	}}))

	req := httptest.NewRequest(http.MethodGet, "/v1/c/attributes?type=genres&sort=alpha_asc", nil)
	rec := httptest.NewRecorder()
	h.GetConsumerAttributes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var env struct {
		Data []model.AttributePublicDetail `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("expected 2 items, got %d", len(env.Data))
	}
	if env.Data[0].Name != "Action" || env.Data[1].Name != "Drama" {
		t.Fatalf("unexpected order: %+v", env.Data)
	}
}

func TestGetConsumerAttributes_SortAlphaDesc(t *testing.T) {
	t.Parallel()

	h := NewAttributeHandler(service.NewAttributeService(fakeAttributeRepo{items: []model.Attribute{
		{ID: "a", Name: "Action", AttributeType: []model.AttributeType{model.AttributeTypeGenre}, Active: true},
		{ID: "b", Name: "Drama", AttributeType: []model.AttributeType{model.AttributeTypeGenre}, Active: true},
	}}))

	req := httptest.NewRequest(http.MethodGet, "/v1/c/attributes?type=genres&sort=alpha_desc", nil)
	rec := httptest.NewRecorder()
	h.GetConsumerAttributes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var env struct {
		Data []model.AttributePublicDetail `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("expected 2 items, got %d", len(env.Data))
	}
	if env.Data[0].Name != "Drama" || env.Data[1].Name != "Action" {
		t.Fatalf("unexpected order: %+v", env.Data)
	}
}

func TestGetConsumerAttributes_InvalidSort(t *testing.T) {
	t.Parallel()

	h := NewAttributeHandler(service.NewAttributeService(fakeAttributeRepo{}))
	req := httptest.NewRequest(http.MethodGet, "/v1/c/attributes?sort=random", nil)
	rec := httptest.NewRecorder()
	h.GetConsumerAttributes(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
