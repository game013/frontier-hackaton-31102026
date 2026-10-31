package perfil

import (
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Dos perfiles con distinto nombre completo pero el mismo nombre corto chocan:
// el corto es el que se escribe y se declara (CE-51, "Sol Lozada" ×2).
func TestRepiteNombreCorto(t *testing.T) {
	b := nuevaBoveda()
	b.agrega(modelo.Perfil{Nombre: "Sol Lozada Pérez", Correo: "slozada@live.com", Cedula: "111", Celular: "3000000001"})
	if !b.repite(modelo.Perfil{Nombre: "Sol Lozada Gómez", Correo: "slozada@gmx.com", Cedula: "222", Celular: "3000000002"}) {
		t.Fatalf("dejé pasar el mismo nombre corto")
	}
	if b.repite(modelo.Perfil{Nombre: "Sol Rueda Gómez", Correo: "srueda@gmx.com", Cedula: "333", Celular: "3000000003"}) {
		t.Fatalf("choqué un nombre corto distinto")
	}
}
