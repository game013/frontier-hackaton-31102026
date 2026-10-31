package juez

import "testing"

// Los pares salen de la forma en que docs/validar.mjs decide si una forma real
// sigue en la salida: pliegue de mayúsculas y tildes, confusables, espacios
// raros, separadores entre dígitos y límites de palabra.
func TestContiene(t *testing.T) {
	casos := []struct {
		nombre   string
		pajar    string
		aguja    string
		esperado bool
	}{
		{"palabra exacta", "hola Manuela Castaño, buen día", "Manuela Castaño", true},
		{"sin tildes en el pajar", "hola Manuela Castano", "Manuela Castaño", true},
		{"sin tildes en la aguja", "hola Manuela Castaño", "Manuela Castano", true},
		{"mayúsculas", "HOLA MANUELA CASTAÑO", "manuela castaño", true},
		{"no es prefijo de palabra", "las Manuelas del barrio", "Manuela", false},
		{"no es sufijo de palabra", "xmanuela", "manuela", false},
		{"puntuación como límite", "(Manuela), dijo", "Manuela", true},
		{"dos palabras con espacio de más", "Manuela    Castaño", "Manuela Castaño", true},
		{"dos palabras con salto de línea", "Manuela\nCastaño", "Manuela Castaño", true},
		{"palabras cortas no admiten separación", "Ana Lucía Marulanda", "Ana Lucía", true},
		{"orden distinto", "Castaño Manuela", "Manuela Castaño", false},
		{"correo literal", "escribe a mcastano@gmail.com ya", "mcastano@gmail.com", true},
		{"correo con mayúsculas", "escribe a MCastano@Gmail.com ya", "mcastano@gmail.com", true},
		{"correo dentro de otro texto", "xmcastano@gmail.comx", "mcastano@gmail.com", true},
		{"cédula con puntos en el pajar", "su cédula 1.107.360.039 vence", "1107360039", true},
		{"cédula con puntos en la aguja", "su cédula 1107360039 vence", "1.107.360.039", true},
		{"cédula con espacios", "su cédula 1 107 360 039 vence", "1107360039", true},
		{"cédula con guiones", "su cédula 1-107-360-039 vence", "1107360039", true},
		{"dígitos con letras alrededor", "ref1107360039abc", "1107360039", true},
		{"dígito de más", "su cédula 11073600390 vence", "1107360039", false},
		{"dígito de menos", "su cédula 110736003 vence", "1107360039", false},
		{"celular con prefijo 57", "marcar +57 314 872 9552", "3148729552", true},
		{"celular con prefijo 57 pegado", "marcar 573148729552", "3148729552", true},
		{"cédula de 6 dígitos", "doc 941842 y sigue", "941842", true},
		{"año corto se busca literal", "el año 2025 fue difícil", "2025", true},
		{"confusable cirílico", "Mаnuela Castaño", "Manuela Castaño", true},
		{"invisible de formato", "Manu\u200bela Castaño", "Manuela Castaño", true},
		{"espacio duro", "Manuela\u00a0Casta\u00f1o", "Manuela Castaño", true},
		{"slug en el nombre de un archivo", "extracto_manuela_castano.pdf", "manuela_castano", true},
		{"direccion con numeral", "vive en Diagonal 3A # 78-6 apto 1501", "Diagonal 3A # 78-6 apto 1501", true},
		{"fecha iso", "nació el 1997-07-23 en Cali", "1997-07-23", true},
		{"fecha iso como fecha", "nació el 1997/07/23 en Cali", "1997-07-23", true},
		{"aguja vacía nunca está", "cualquier cosa", "", false},
		{"pajar vacío", "", "Manuela", false},
		{"ß dentro de palabra", "Straße 4", "strasse", true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := Contiene(c.pajar, c.aguja); got != c.esperado {
				t.Fatalf("Contiene(%q, %q) = %v, se esperaba %v", c.pajar, c.aguja, got, c.esperado)
			}
		})
	}
}

func TestContieneExacto(t *testing.T) {
	if !ContieneExacto("dice Manuela Castaño aquí", "Manuela Castaño") {
		t.Fatal("el fragmento literal debería encontrarse")
	}
	if ContieneExacto("dice manuela castaño aquí", "Manuela Castaño") {
		t.Fatal("ContieneExacto no debe plegar mayúsculas")
	}
}

func TestNormaliza(t *testing.T) {
	casos := map[string]string{
		"Manuela  Castaño": "manuela castano",
		"ÁLVARO NÚÑEZ":     "alvaro nunez",
		"a\tb\nc":          "a b c",
		" ﬁn ":             " fin ",
	}
	for entrada, esperado := range casos {
		if got := Normaliza(entrada); got != esperado {
			t.Fatalf("Normaliza(%q) = %q, se esperaba %q", entrada, got, esperado)
		}
	}
}

func TestDigitos(t *testing.T) {
	if got := Digitos("1.107.360.039"); got != "1107360039" {
		t.Fatalf("Digitos con puntos = %q", got)
	}
	if got := Digitos("Manuela"); got != "" {
		t.Fatalf("Digitos sin dígitos = %q", got)
	}
}
