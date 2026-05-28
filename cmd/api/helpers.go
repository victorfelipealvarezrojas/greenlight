package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	// Use http.MaxBytesReader() to limit the size of the request body to 1MB.
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // configura comportamiento, no lee nada

	// decoder por medio de reflexión escribe los datos decodificados en la variable (dst) que se le pasa como puntero. y valida el formato del JSON,
	// si es incorrecto devuelve un error que se puede analizar para dar un mensaje de error más específico al cliente.
	// ese error puede ser un error de sintaxis, un error de tipo, un error de tamaño, un error de campo desconocido, etc.
	// pero es decoder quien hace todo ese trabajo.
	err := dec.Decode(dst)
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
		var maxBytesError *http.MaxBytesError

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

		// If the JSON contains a field which cannot be mapped to the target destination
		// then Decode() will now return an error message in the format "json: unknown
		// field "<name>"". We check for this, extract the field name from the error,
		// and interpolate it into our custom error message. Note that there's an open
		// issue at https://github.com/golang/go/issues/29035 regarding turning this
		// into a distinct error type in the future.
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		// Use the errors.As() function to check whether the error has the type
		// *http.MaxBytesError. If it does, then it means the request body exceeded our
		// size limit of 1MB and we return a clear error message.
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}

	}

	// Call Decode() again, using a pointer to an empty anonymous struct as the
	// destination. If the request body only contained a single JSON value this will
	// return an io.EOF error. So if we get anything else, we know that there is
	// additional data in the request body and we return our own custom error message.
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) { // si el error NO es EOF → problema ( io.EOF es un error de lectura — específicamente indica fin del stream.)
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}
