package http

import "github.com/xanderbilla/bi8s-go/internal/model"

func toMoviePublicDetail(m *model.Movie) model.MoviePublicDetail {
	return model.MoviePublicDetail{
		ID:               m.ID,
		Title:            m.Title,
		Overview:         m.Overview,
		BackdropPath:     m.BackdropPath,
		PosterPath:       m.PosterPath,
		ReleaseDate:      m.ReleaseDate,
		FirstAirDate:     m.FirstAirDate,
		Adult:            m.Adult,
		ContentRating:    m.ContentRating,
		OriginalLanguage: m.OriginalLanguage,
		Genres:           m.Genres,
		Casts:            m.Casts,
		Tags:             m.Tags,
		ContentType:      m.ContentType,
		OriginCountry:    m.OriginCountry,
		MoodTags:         m.MoodTags,
		Runtime:          m.Runtime,
		Status:           m.Status,
		Tagline:          m.Tagline,
		Studios:          m.Studios,
		Assets:           m.Assets,
	}
}

func toPersonPublicDetail(p *model.Person) model.PersonPublicDetail {
	return model.PersonPublicDetail{
		ID:           p.ID,
		ContentType:  p.ContentType,
		Name:         p.Name,
		Roles:        p.Roles,
		StageName:    p.StageName,
		Bio:          p.Bio,
		BirthDate:    p.BirthDate,
		BirthPlace:   p.BirthPlace,
		Nationality:  p.Nationality,
		Gender:       p.Gender,
		Height:       p.Height,
		Verified:     p.Verified,
		Active:       p.Active,
		DebutYear:    p.DebutYear,
		CareerStatus: p.CareerStatus,
		ProfilePath:  p.ProfilePath,
		BackdropPath: p.BackdropPath,
		Measurements: p.Measurements,
		Tags:         p.Tags,
		Categories:   p.Categories,
		Specialties:  p.Specialties,
	}
}

func toAttributePublicDetail(a *model.Attribute) model.AttributePublicDetail {
	return model.AttributePublicDetail{
		ID:            a.ID,
		Name:          a.Name,
		AttributeType: a.AttributeType,
		ContentType:   a.ContentType,
		Active:        a.Active,
	}
}

func convertToPublicList(movies []model.Movie) []model.MoviePublicList {
	out := make([]model.MoviePublicList, len(movies))
	for i, m := range movies {
		out[i] = model.MoviePublicList{
			ID:            m.ID,
			Title:         m.Title,
			BackdropPath:  m.BackdropPath,
			PosterPath:    m.PosterPath,
			ReleaseDate:   m.ReleaseDate,
			Tags:          m.Tags,
			ContentRating: m.ContentRating,
			ContentType:   m.ContentType,
			Assets:        m.Assets,
		}
	}
	return out
}

func convertToMinimalList(movies []model.Movie) []model.MoviesByPersonList {
	out := make([]model.MoviesByPersonList, len(movies))
	for i, m := range movies {
		out[i] = model.MoviesByPersonList{
			ID:           m.ID,
			Title:        m.Title,
			BackdropPath: m.BackdropPath,
		}
	}
	return out
}
