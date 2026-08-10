package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// NOTE: serializasion Runtime es un tipo personalizado basado en int32.
// Al implementar MarshalJSON(), satisface implícitamente la interface json.Marshaler.
// Esto significa que cuando el encoder JSON del runtime encuentre un campo de tipo Runtime,
// en lugar de serializar el entero crudo (ej: 94), llamará a nuestra implementación
// y producirá una cadena formateada (ej: "94 mins").

// NOTE: UnmarshalJSON es para el caso contrario, cuando el decoder JSON encuentre un campo de tipo Runtime, llamará a UnmarshalJSON para deserializar la cadena
//  formateada (ej: "94 mins") y convertirla de nuevo a un entero (ej: 94).

// Definimos un error que nuestro método UnmarshalJSON() puede devolver si no podemos analizar
// o convertir la cadena JSON correctamente.
var ErrInvalidRuntimeFormat = errors.New("invalid runtime format")

type Runtime int32

// solo lee el receptor, no lo modifica, por eso no es un receptor de puntero (va de salida, no de entrada)
func (r Runtime) MarshalJSON() ([]byte, error) {
	jsonValue := fmt.Sprintf("%d mins", r)

	// strconv.Quote() toma una cadena sin formato y devuelve una nueva cadena con las comillas dobles escapadas.
	quotedJSONValue := strconv.Quote(jsonValue)

	// Convert the quoted string value to a byte slice and return it.
	return []byte(quotedJSONValue), nil
}

// escribimos el receptor como un puntero porque vamos a modificar su valor
func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {
	// Esperamos que el valor JSON entrante sea una cadena en el formato
	// "<runtime> mins", y lo primero que debemos hacer es eliminar las comillas dobles de esa cadena.
	// Si no podemos eliminar las comillas, devolvemos el Error ErrInvalidRuntimeFormat.
	unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	parts := strings.Split(unquotedJSONValue, " ")

	if len(parts) != 2 || parts[1] != "mins" {
		return ErrInvalidRuntimeFormat
	}

	// analiza la cadena que contiene el número en un int32.
	i, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	*r = Runtime(i)

	return nil

}
