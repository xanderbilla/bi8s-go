package http

import (
	"net/url"
	"testing"
)

func FuzzParseMovieFromForm(f *testing.F) {
	seeds := []struct {
		title, releaseDate, runtime, adult, contentType, genres string
	}{
		{"Foo", "2024-01-01", "120", "true", "MOVIE", "g1,g2"},
		{"", "", "", "", "", ""},
		{"x", "not-a-date", "abc", "notbool", "", ","},
		{"\u0000", "0000-00-00", "-1", "1", "TV", "a, ,b,,c"},
	}
	for _, s := range seeds {
		f.Add(s.title, s.releaseDate, s.runtime, s.adult, s.contentType, s.genres)
	}

	f.Fuzz(func(t *testing.T, title, releaseDate, runtime, adult, contentType, genres string) {
		v := url.Values{}
		v.Set("title", title)
		v.Set("release_date", releaseDate)
		v.Set("runtime", runtime)
		v.Set("adult", adult)
		v.Set("content_type", contentType)
		v.Set("genres", genres)
		_, _ = ParseMovieFromForm(v)
	})
}

func FuzzParsePersonFromForm(f *testing.F) {
	seeds := []struct {
		name, birthDate, height, debutYear, active, gender, roles string
	}{
		{"Alice", "1990-01-02", "170", "2010", "true", "F", "ACTOR"},
		{"", "", "", "", "", "", ""},
		{"Bob", "bad-date", "xx", "yy", "notbool", "Z", ","},
	}
	for _, s := range seeds {
		f.Add(s.name, s.birthDate, s.height, s.debutYear, s.active, s.gender, s.roles)
	}

	f.Fuzz(func(t *testing.T, name, birthDate, height, debutYear, active, gender, roles string) {
		v := url.Values{}
		v.Set("name", name)
		v.Set("birth_date", birthDate)
		v.Set("height", height)
		v.Set("debut_year", debutYear)
		v.Set("active", active)
		v.Set("gender", gender)
		v.Set("roles", roles)
		_, _ = ParsePersonFromForm(v)
	})
}

func FuzzParseAttributeFromForm(f *testing.F) {
	for _, s := range []struct{ name, attrType string }{
		{"Drama", "GENRE"},
		{"", ""},
		{" ", ", , ,"},
		{"x", "GENRE,LANGUAGE,COUNTRY"},
	} {
		f.Add(s.name, s.attrType)
	}

	f.Fuzz(func(t *testing.T, name, attrType string) {
		v := url.Values{}
		v.Set("name", name)
		v.Set("attribute_type", attrType)
		_, _ = ParseAttributeFromForm(v)
	})
}
