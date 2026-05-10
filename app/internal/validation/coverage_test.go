package validation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

var pngBytes = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x62, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

func TestValidateStruct_DateRange(t *testing.T) {
	type s struct {
		D string `validate:"daterange"`
	}
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"empty ok", "", false},
		{"valid past", "2020-01-01", false},
		{"too old", "1900-01-01", true},
		{"future", time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02"), true},
		{"garbage", "not-a-date", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStruct(s{D: tc.in})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateStruct_Age18Plus(t *testing.T) {
	type s struct {
		B string `validate:"age18plus"`
	}
	now := time.Now().UTC()
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"empty ok", "", false},
		{"adult", now.AddDate(-25, 0, 0).Format("2006-01-02"), false},
		{"minor", now.AddDate(-10, 0, 0).Format("2006-01-02"), true},
		{"garbage", "x", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStruct(s{B: tc.in})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateStruct_CustomDate(t *testing.T) {
	type s struct {
		D model.Date `validate:"customdate"`
	}
	if err := ValidateStruct(s{D: model.Date{}}); err == nil {
		t.Fatal("zero date should fail customdate")
	}
	if err := ValidateStruct(s{D: model.Date{Time: time.Now()}}); err != nil {
		t.Fatalf("non-zero date should pass: %v", err)
	}
}

func multipartReq(t *testing.T, field, filename, contentType string, body []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(map[string][]string)
	hdr["Content-Disposition"] = []string{`form-data; name="` + field + `"; filename="` + filename + `"`}
	if contentType != "" {
		hdr["Content-Type"] = []string{contentType}
	}
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestExtractFile(t *testing.T) {
	t.Run("missing returns nil,nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")

		got, err := ExtractFile(req, "missing", 1024)
		if err == nil && got != nil {
			t.Fatalf("unexpected got=%v err=%v", got, err)
		}
	})
	t.Run("valid png", func(t *testing.T) {
		req := multipartReq(t, "f", "tiny.png", "image/png", pngBytes)
		w := httptest.NewRecorder()
		if err := req.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		_ = w
		got, err := ExtractFile(req, "f", 1<<20)
		if err != nil || got == nil {
			t.Fatalf("err=%v got=%v", err, got)
		}
		if got.ContentType != "image/png" {
			t.Errorf("ContentType=%s", got.ContentType)
		}
	})
	t.Run("size cap", func(t *testing.T) {
		req := multipartReq(t, "f", "tiny.png", "image/png", pngBytes)
		if err := req.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		_, err := ExtractFile(req, "f", 10)
		if err == nil {
			t.Fatal("expected size error")
		}
	})
	t.Run("bad extension", func(t *testing.T) {
		req := multipartReq(t, "f", "evil.exe", "image/png", pngBytes)
		if err := req.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		_, err := ExtractFile(req, "f", 1<<20)
		if err == nil {
			t.Fatal("expected extension error")
		}
	})
	t.Run("empty body", func(t *testing.T) {
		req := multipartReq(t, "f", "tiny.png", "image/png", []byte{})
		if err := req.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		_, err := ExtractFile(req, "f", 1<<20)
		if err == nil {
			t.Fatal("expected empty error")
		}
	})
}

func TestExtractFileToTemp(t *testing.T) {
	req := multipartReq(t, "f", "tiny.png", "image/png", pngBytes)
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	got, err := ExtractFileToTemp(req, "f", 1<<20)
	if err != nil || got == nil {
		t.Fatalf("err=%v got=%v", err, got)
	}
	defer func() {
		if err := os.Remove(got.TempFilePath); err != nil && !os.IsNotExist(err) {
			t.Logf("cleanup remove temp: %v", err)
		}
	}()
	if got.Size != int64(len(pngBytes)) {
		t.Errorf("size=%d want=%d", got.Size, len(pngBytes))
	}
	if _, err := os.Stat(got.TempFilePath); err != nil {
		t.Errorf("temp file missing: %v", err)
	}
}

func TestValidateStruct_DateRange_ModelDate(t *testing.T) {
	type s struct {
		D model.Date `validate:"daterange"`
	}
	if err := ValidateStruct(s{D: model.Date{}}); err != nil {
		t.Errorf("zero date should pass: %v", err)
	}
	if err := ValidateStruct(s{D: model.Date{Time: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)}}); err == nil {
		t.Error("too old should fail")
	}
	if err := ValidateStruct(s{D: model.Date{Time: time.Now().UTC().AddDate(1, 0, 0)}}); err == nil {
		t.Error("future should fail")
	}
}

type fakeAttributeRepo struct {
	byID map[string]*model.Attribute
	err  error
}

func (f *fakeAttributeRepo) Get(_ context.Context, id string) (*model.Attribute, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byID[id], nil
}

func (f *fakeAttributeRepo) GetAll(context.Context) ([]model.Attribute, error) { return nil, nil }
func (f *fakeAttributeRepo) GetByName(context.Context, string) (*model.Attribute, error) {
	return nil, nil
}
func (f *fakeAttributeRepo) Create(context.Context, model.Attribute) error { return nil }
func (f *fakeAttributeRepo) Delete(context.Context, string) error          { return nil }

func TestValidateAndPopulateAttributes(t *testing.T) {
	ctx := context.Background()
	repo := &fakeAttributeRepo{byID: map[string]*model.Attribute{
		"g1": {ID: "g1", Name: "Action", AttributeType: []model.AttributeType{model.AttributeTypeGenre}},
	}}
	t.Run("empty passes through", func(t *testing.T) {
		out, err := ValidateAndPopulateAttributes(ctx, nil, model.AttributeTypeGenre, repo)
		if err != nil || out != nil {
			t.Fatalf("out=%v err=%v", out, err)
		}
	})
	t.Run("happy path populates Name", func(t *testing.T) {
		out, err := ValidateAndPopulateAttributes(ctx,
			[]model.EntityRef{{ID: "g1"}}, model.AttributeTypeGenre, repo)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 || out[0].Name != "Action" {
			t.Fatalf("out=%+v", out)
		}
	})
	t.Run("missing attribute returns NotFound", func(t *testing.T) {
		_, err := ValidateAndPopulateAttributes(ctx,
			[]model.EntityRef{{ID: "missing"}}, model.AttributeTypeGenre, repo)
		var nf *errs.AttributeNotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("want AttributeNotFoundError, got %T %v", err, err)
		}
	})
	t.Run("repo error bubbles", func(t *testing.T) {
		bad := &fakeAttributeRepo{err: io.EOF}
		_, err := ValidateAndPopulateAttributes(ctx,
			[]model.EntityRef{{ID: "x"}}, model.AttributeTypeGenre, bad)
		if !errors.Is(err, io.EOF) {
			t.Fatalf("err=%v", err)
		}
	})
}

type fakePersonRepo struct {
	byID map[string]*model.Person
	err  error
}

func (f *fakePersonRepo) Get(_ context.Context, id string) (*model.Person, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byID[id], nil
}

func TestValidateAndPopulateCasts(t *testing.T) {
	ctx := context.Background()
	repo := &fakePersonRepo{byID: map[string]*model.Person{
		"p1": {ID: "p1", Name: "Alice"},
	}}
	t.Run("empty", func(t *testing.T) {
		out, err := ValidateAndPopulateCasts(ctx, nil, repo)
		if err != nil || out != nil {
			t.Fatalf("out=%v err=%v", out, err)
		}
	})
	t.Run("happy path", func(t *testing.T) {
		out, err := ValidateAndPopulateCasts(ctx, []model.EntityRef{{ID: "p1"}}, repo)
		if err != nil || len(out) != 1 || out[0].Name != "Alice" {
			t.Fatalf("err=%v out=%+v", err, out)
		}
	})
	t.Run("missing person", func(t *testing.T) {
		_, err := ValidateAndPopulateCasts(ctx, []model.EntityRef{{ID: "x"}}, repo)
		var nf *errs.PerformerNotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("want PerformerNotFoundError, got %T %v", err, err)
		}
	})
	t.Run("repo error bubbles", func(t *testing.T) {
		bad := &fakePersonRepo{err: io.EOF}
		_, err := ValidateAndPopulateCasts(ctx, []model.EntityRef{{ID: "p"}}, bad)
		if !errors.Is(err, io.EOF) {
			t.Fatalf("err=%v", err)
		}
	})
}
