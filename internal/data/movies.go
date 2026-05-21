package data

import "time"

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
