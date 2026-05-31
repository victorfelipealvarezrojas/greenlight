package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
	"github.com/valvarez/greenlight/internal/validator"
)

type Movie struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"` // la directiva "-" indica que este campo no se incluirá en la representación JSON de la película
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"` // la directiva "omitempty" indica que este campo se omitirá de la representación JSON si su valor es 0 para su tipo
	Runtime   Runtime   `json:"runtime,omitempty"`
	Genres    []string  `json:"genres,omitempty"`
	Version   int32     `json:"version"`
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

// Defina un tipo de estructura MovieModel que englobe un grupo de conexiones sql.DB.
type MovieModel struct {
	BD *sql.DB
}

func (m MovieModel) Insert(movie *Movie) error {
	query := `
		INSERT INTO movies (title, year, runtime, genres)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, version`

	args := []any{movie.Title, movie.Year, movie.Runtime, pq.Array(movie.Genres)}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Utilice el método QueryRow() para ejecutar la consulta SQL en nuestro grupo de conexiones, pasando el slice de argumentos.
	// Scan escribe los valores generados por PostgreSQL (id, created_at, version)
	return m.BD.QueryRowContext(ctx, query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)

	// NOTE: Debido a que la firma del método Insert() toma un puntero *Movie como parámetro, cuando
	// a Scan() es llamado para leer los datos generados por el sistema; estamos actualizando los valores en la ubicación
	// el parámetro apunta a. Esencialmente, nuestro método Insert() muta la estructura Movie.
}

func (m MovieModel) Get(id int64) (*Movie, error) {

	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id, created_at, title, year, runtime, genres, version
		FROM movies
		WHERE id = $1`

	var movie Movie

	// Utilice la función context.WithTimeout() para crear un contexto.Context que lleve un
	// Fecha límite de tiempo de espera de 3 segundos.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	// llamar a la función cancel() para liberar los recursos asociados con el contexto una vez que se complete la consulta.
	defer cancel()

	err := m.BD.QueryRowContext(ctx, query, id).Scan(
		&movie.ID,
		&movie.CreatedAt,
		&movie.Title,
		&movie.Year,
		&movie.Runtime,
		pq.Array(&movie.Genres),
		&movie.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &movie, nil
}

func (m MovieModel) Update(movie *Movie) error {

	// Agregue la cláusula 'AND versión = $6' a la consulta SQL.
	query := `
		UPDATE movies
		SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING version`

	args := []any{
		movie.Title,
		movie.Year,
		movie.Runtime,
		pq.Array(movie.Genres),
		movie.ID,
		movie.Version, // Agrega la versión de la película esperada.
	}

	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Ejecutar la consulta SQL. Si no se pudo encontrar ninguna fila coincidente, conocemos la película.
	// la versión ha cambiado (o el registro ha sido eliminado) y volvemos a nuestra versión personalizada
	// Error ErrEditConflict.
	err := m.BD.QueryRowContext(ctx, query, args...).Scan(&movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (m MovieModel) Delete(id int64) error {

	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
		DELETE FROM movies
		WHERE id = $1`

	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.BD.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, error) {

	query := `
		SELECT id, created_at, title, year, runtime, genres, version
		FROM movies
		ORDER BY id`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.BD.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	movies := []*Movie{}

	for rows.Next() {

		var movie Movie

		err := rows.Scan(
			&movie.ID,
			&movie.CreatedAt,
			&movie.Title,
			&movie.Year,
			&movie.Runtime,
			pq.Array(&movie.Genres),
			&movie.Version,
		)
		if err != nil {
			return nil, err
		}
		movies = append(movies, &movie)
	}

	// Cuando el bucle rows.Next() haya finalizado, llame a rows.Err() para recuperar cualquier error
	// que se encontró durante la iteración.
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return movies, nil
}
