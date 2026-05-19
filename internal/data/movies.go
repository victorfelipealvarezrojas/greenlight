package data

import "time"

type Movie struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"` // la directiva "-" indica que este campo no se incluirá en la representación JSON de la película
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"` // la directiva "omitempty" indica que este campo se omitirá de la representación JSON si su valor es 0 para su tipo
	Runtime   int32     `json:"runtime,omitempty,string"`
	Genres    []string  `json:"genres,omitempty"`
	Version   int32     `json:"version"`
}
