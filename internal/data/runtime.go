package data

import (
	"fmt"
	"strconv"
)

// NOTE: Runtime es un tipo personalizado basado en int32.
// Al implementar MarshalJSON(), satisface implícitamente la interface json.Marshaler.
// Esto significa que cuando el encoder JSON del runtime encuentre un campo de tipo Runtime,
// en lugar de serializar el entero crudo (ej: 94), llamará a nuestra implementación
// y producirá una cadena formateada (ej: "94 mins").
//
// El flujo interno del encoder es aproximadamente:
//   1. Encuentra el campo Runtime en Movie
//   2. Detecta que Runtime implementa json.Marshaler (tiene MarshalJSON)
//   3. Llama r.MarshalJSON() en lugar del comportamiento por defecto
//   4. Usa el []byte retornado como valor JSON del campo
//
// Receptor de valor (r Runtime) en lugar de puntero (*Runtime) para que
// la interface json.Marshaler se satisfaga tanto con valores como con punteros,
// maximizando la compatibilidad cuando Runtime se use en otros contextos.
// **esto es sobre escritura de un método de MarshalJSON() para el tipo Runtime, que formatea el valor como una cadena con el formato "X mins" en lugar de un número entero crudo.**

type Runtime int32

func (r Runtime) MarshalJSON() ([]byte, error) {
	jsonValue := fmt.Sprintf("%d mins", r)

	quotedJSONValue := strconv.Quote(jsonValue)

	// Convert the quoted string value to a byte slice and return it.
	return []byte(quotedJSONValue), nil
}
