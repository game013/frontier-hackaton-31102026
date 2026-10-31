package chats

import "strings"

// NombreCorto devuelve la forma corta de un nombre completo: nombre(s) más
// primer apellido, que es lo que pide output/ en author (nombre, nunca el uid).
//
// Es una regla posicional porque no hay otra señal: el corpus solo trae real_name
// de 1, 2, 3 y 4 tokens (analisis/02_chats.md §2) y en él los de 3 son nombre +
// dos apellidos ("Nombre Apellido Uno" → "Nombre Apellido") y los de 4 son nombre
// + segundo nombre + dos apellidos ("Nombre Segundo Apellido Uno" → "Nombre
// Segundo Apellido"). Con un catálogo de nombres de pila (C9) podría afinarse, y
// la forma corta definitiva de una persona la fija el registro (C8, C12).
func NombreCorto(nombreCompleto string) string {
	campos := strings.Fields(nombreCompleto)
	switch {
	case len(campos) <= 2:
		return strings.Join(campos, " ")
	case len(campos) == 3:
		return strings.Join(campos[:2], " ")
	default:
		return strings.Join(campos[:3], " ")
	}
}
