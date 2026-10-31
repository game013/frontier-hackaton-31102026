package formas

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// claveForma traduce las claves de `forms` de sample/tests_sample.json (las que
// usa el jurado para buscar la entidad) al tipo de forma que las produce.
var claveForma = map[string]modelo.TipoForma{
	"full":           modelo.FormaNombreCorto,
	"full2":          modelo.FormaNombreCompleto,
	"first":          modelo.FormaNombrePila,
	"cedula":         modelo.FormaCedula,
	"cedula_dots":    modelo.FormaCedulaPuntos,
	"email":          modelo.FormaCorreo,
	"phone":          modelo.FormaCelular,
	"address":        modelo.FormaDireccion,
	"birth_iso":      modelo.FormaFechaISO,
	"filename_token": modelo.FormaSlug,
}

// perfilDeForms arma el perfil canónico con las formas que declara el test.
func perfilDeForms(forms map[string]string) modelo.Perfil {
	nombre := forms["full2"]
	if nombre == "" {
		nombre = forms["full"]
	}
	return modelo.Perfil{
		Nombre:     nombre,
		Correo:     forms["email"],
		Celular:    forms["phone"],
		Cedula:     forms["cedula"],
		Direccion:  forms["address"],
		Nacimiento: forms["birth_iso"],
	}
}

// TestFormasDeLaMuestra comprueba cada clave de `forms` de los tests de
// consistencia contra el renderizador del tipo equivalente.
func TestFormasDeLaMuestra(t *testing.T) {
	ruta := rutaMuestra(t, "tests_sample.json")
	var tests []struct {
		ID    string            `json:"id"`
		Forms map[string]string `json:"forms"`
	}
	leerJSON(t, ruta, &tests)

	vistos := 0
	for _, tc := range tests {
		if len(tc.Forms) == 0 {
			continue
		}
		vistos++
		perfil := perfilDeForms(tc.Forms)
		for clave, quiero := range tc.Forms {
			tipo, ok := claveForma[clave]
			if !ok {
				t.Fatalf("%s: clave de forma sin renderizador: %q", tc.ID, clave)
			}
			f, err := Render(perfil, tipo)
			if err != nil {
				t.Fatalf("%s/%s: Render(%s): %v", tc.ID, clave, tipo, err)
			}
			if f.Valor != quiero {
				t.Fatalf("%s/%s: Valor = %q, quiero %q", tc.ID, clave, f.Valor, quiero)
			}
		}
	}
	if vistos == 0 {
		t.Fatal("sample/tests_sample.json no trajo ningún test con forms")
	}
}

// TestOriginalsDeLaMuestra exige que toda forma real de entities.json
// (originals) salga de algún renderizador con el perfil de esa persona.
func TestOriginalsDeLaMuestra(t *testing.T) {
	var doc struct {
		Entities []struct {
			EntityID  string   `json:"entity_id"`
			Type      string   `json:"type"`
			Originals []string `json:"originals"`
		} `json:"entities"`
	}
	leerJSON(t, rutaMuestra(t, filepath.Join("expected_output", "entities.json")), &doc)

	personas := 0
	for _, e := range doc.Entities {
		if e.Type != "person" {
			continue
		}
		personas++
		formas, err := Todas(perfilDeOriginals(e.Originals))
		if err != nil {
			t.Fatalf("%s: Todas: %v", e.EntityID, err)
		}
		valores := map[string]bool{}
		for _, f := range formas {
			valores[f.Valor] = true
		}
		for _, original := range e.Originals {
			if !valores[original] {
				t.Fatalf("%s: la forma real %q no la produce ningún renderizador", e.EntityID, original)
			}
		}
	}
	if personas == 0 {
		t.Fatal("entities.json de la muestra no trajo personas")
	}
}

// perfilDeOriginals arma el perfil canónico de una persona con sus formas
// reales: el nombre más largo es el completo y los demás datos se reconocen por
// su forma. El registro construye el perfil igual, desde el maestro.
func perfilDeOriginals(originals []string) modelo.Perfil {
	p := modelo.Perfil{Nombre: nombreLargo(originals)}
	for _, o := range originals {
		switch {
		case strings.Contains(o, "@"):
			p.Correo = o
		case strings.Contains(o, "#"):
			p.Direccion = o
		case esFechaISO(o):
			p.Nacimiento = o
		case soloDigitos(o) && len(o) == 10 && o[0] == '3':
			p.Celular = o
		case soloDigitos(o) && (len(o) == 8 || len(o) == 10):
			p.Cedula = o
		}
	}
	return p
}

func esFechaISO(s string) bool {
	if len(s) != len("2006-01-02") || s[4] != '-' || s[7] != '-' {
		return false
	}
	return soloDigitos(s[:4]) && soloDigitos(s[5:7]) && soloDigitos(s[8:])
}

func soloDigitos(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// nombreLargo devuelve el original con más palabras: los apellidos son las dos
// últimas y el maestro trae el nombre completo con dos.
func nombreLargo(originals []string) string {
	largo := ""
	for _, o := range originals {
		if !soloLetras(o) {
			continue
		}
		if len(strings.Fields(o)) > len(strings.Fields(largo)) {
			largo = o
		}
	}
	return largo
}

func soloLetras(s string) bool {
	for _, r := range s {
		if !(r == ' ' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r >= 0x80) {
			return false
		}
	}
	return s != ""
}

// rutaMuestra apunta a un archivo del dataset de muestra, que no viaja en git:
// si no está, la prueba se salta en vez de fallar.
func rutaMuestra(t *testing.T, nombre string) string {
	t.Helper()
	ruta := filepath.Join("..", "..", "sample", nombre)
	if _, err := os.Stat(ruta); err != nil {
		t.Skipf("sin muestra %s: %v", ruta, err)
	}
	return ruta
}

func leerJSON(t *testing.T, ruta string, destino any) {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("lee %s: %v", ruta, err)
	}
	if err := json.Unmarshal(b, destino); err != nil {
		t.Fatalf("decodifica %s: %v", ruta, err)
	}
}
