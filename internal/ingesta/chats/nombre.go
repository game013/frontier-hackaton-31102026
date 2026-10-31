package chats

import "strings"

// NombreCorto devuelve la forma corta de un nombre completo: nombre(s) más
// primer apellido, que es lo que pide output/ en author (nombre, nunca el uid).
//
// La regla es la misma de internal/formas: quitar el último apellido. El corpus
// trae real_name de 1 a 4 tokens (analisis/02_chats.md §2) y así los de 3 son
// nombre + dos apellidos y los de 4, nombre + segundo nombre + dos apellidos. La
// forma corta definitiva de una persona la fija el registro (C8, C12).
func NombreCorto(nombreCompleto string) string {
	campos := strings.Fields(nombreCompleto)
	switch {
	case len(campos) <= 2:
		return strings.Join(campos, " ")
	case len(campos) == 3:
		return strings.Join(campos[:2], " ")
	default:
		// Igual que internal/formas: la forma corta quita el último apellido.
		return strings.Join(campos[:len(campos)-1], " ")
	}
}
