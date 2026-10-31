package chats

import "testing"

func TestNombreCorto(t *testing.T) {
	casos := []struct {
		nombre   string
		completo string
		quiero   string
	}{
		{"nombre y dos apellidos", "Nombre Apellido Uno", "Nombre Apellido"},
		{"nombre, segundo nombre y dos apellidos", "Nombre Segundo Apellido Uno", "Nombre Segundo Apellido"},
		{"nombre y un apellido", "Nombre Apellido", "Nombre Apellido"},
		{"solo un token", "Kipi", "Kipi"},
		{"vacío", "", ""},
		{"espacios de sobra", "  Nombre   Apellido Uno  ", "Nombre Apellido"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if tengo := NombreCorto(c.completo); tengo != c.quiero {
				t.Fatalf("NombreCorto(%q) = %q, quiero %q", c.completo, tengo, c.quiero)
			}
		})
	}
}
