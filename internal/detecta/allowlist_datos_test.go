package detecta

import "testing"

// TestPorDefectoUsaLaAllowlistDeC26 fija el cableado de C26: el default del
// pipeline es el archivo de datos, no una lista fija que se quede atrás cuando
// el corpus crezca.
func TestPorDefectoUsaLaAllowlistDeC26(t *testing.T) {
	embebida, err := Embebida()
	if err != nil {
		t.Fatalf("la allowlist de C26 no carga: %v", err)
	}
	if len(embebida.Entradas) == 0 {
		t.Fatal("allowlist.json quedó sin entidades: el archivo no se embebió")
	}

	porDefecto := PorDefecto()
	if len(porDefecto.Entradas) != len(embebida.Entradas) {
		t.Fatalf("PorDefecto no usa la allowlist embebida: %d entradas contra %d",
			len(porDefecto.Entradas), len(embebida.Entradas))
	}
	for _, nombre := range []string{"Bancolombia", "Kipu Pay", "Kipi", "Superfinanciera"} {
		if motivo, ok := porDefecto.Conserva(nombre); !ok || motivo == "" {
			t.Errorf("%q debe conservarse con motivo; conservada=%v motivo=%q", nombre, ok, motivo)
		}
	}

	// CE-14: el apellido que también es lugar no se conserva como entidad, y con
	// nombre de pila delante es la persona, que sí se reemplaza.
	if _, ok := porDefecto.Conserva("Restrepo"); ok {
		t.Error("Restrepo no puede estar en las entidades: es apellido y lugar")
	}
	if porDefecto.ConservaApellidoLugar("Restrepo", "Luis") {
		t.Error("con nombre de pila delante, Restrepo es la persona y se reemplaza")
	}
	if !porDefecto.ConservaApellidoLugar("Restrepo", "vengo del") {
		t.Error("sin nombre de pila delante, Restrepo es el lugar y se conserva")
	}
}
