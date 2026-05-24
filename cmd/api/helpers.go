package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (app *application) readIDParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}
	return id, nil
}

type envelope map[string]any

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	// Encode the data to JSON, returning the error if there was one.
	//NOTE: podria usar json.Encoder [json.NewEncoder(w).Encode(data)] que escribe directamente en el ResponseWriter,
	// y evitaria erscribir el w.Write que es un ResponseWriter, pero no e sposible configurar encabezados http antes de escribir la respuesta,
	// por eso se opta por usar json.Marshal y luego escribir el w.Write
	js, err := json.Marshal(data)

	// otra opcion es js, err := json.MarshalIndent(data, "", "\t") que agrega saltos de linea y tabulaciones para hacer el JSON mas legible, pero ocupa mas espacio

	if err != nil {
		return err
	}

	// Append a newline to make it easier to view in terminal applications.
	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js) // esto genera la respuesta HTTP, por eso no se pueden escribir encabezados después de esta línea

	return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil {

		// errors.As busca en la cadena de errores un error del tipo apuntado
		// y si lo encuentra lo escribe en la variable via la referencia.
		// Por eso se declaran vacías antes — necesitan existir como variables
		// addressables para que As pueda escribir en ellas.
		//
		// errors.Is  → compara por identidad (errores centinela: io.EOF, io.ErrUnexpectedEOF)
		// errors.As  → compara por tipo y extrae el valor para acceder a sus campos (syntaxError.Offset)

		//LINK - syntaxError.Offset — ese campo no existe en la interface error. Solo existe en *json.SyntaxError.
		// Si no la almacenaras no podrías construir el mensaje con la posición exacta del error.

		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		switch {

		// Error centinela predefinido → Is
		// Error con tipo propio y campos → As

		// *json.SyntaxError es un tipo con campos — necesitas extraerlo para usarlos
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		// io.EOF es un valor centinela — comparas por identidad
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}

	}

	return nil
}
