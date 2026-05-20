package main

import (
	"encoding/json"
	"errors"
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
