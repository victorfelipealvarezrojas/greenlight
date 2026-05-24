package data

import (
	"time"

	"github.com/valvarez/greenlight/internal/validator"
)

type Movie struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"` // la directiva "-" indica que este campo no se incluirá en la representación JSON de la película
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"` // la directiva "omitempty" indica que este campo se omitirá de la representación JSON si su valor es 0 para su tipo
	// Runtime usa tipo Runtime (no int32 directo) para que el encoder JSON
	// invoque automáticamente MarshalJSON() y formatee el valor como "X mins".
	// La directiva "string" en el tag es redundante aquí porque MarshalJSON
	// ya controla completamente la representación — Runtime se encarga solo.
	Runtime Runtime  `json:"runtime,omitempty"`
	Genres  []string `json:"genres,omitempty"`
	Version int32    `json:"version"`
}

func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")
	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888")
	v.Check(movie.Year <= int32(time.Now().Year()), "year", "must not be in the future")
	v.Check(movie.Runtime != 0, "runtime", "must be provided")
	v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")
	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}
