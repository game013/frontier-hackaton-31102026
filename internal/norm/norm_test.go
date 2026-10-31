package norm_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/game013/frontier-hackaton-31102026/internal/norm"
)

// fixture es la tabla de paridad con el jurado. La genera Node desde
// docs/validar.mjs: cada resultado esperado es el que da el propio jurado.
type fixture struct {
	Contiene      []casoContiene      `json:"contiene"`
	Normalizacion []casoNormalizacion `json:"normalizacion"`
	DigitosEn     []casoDigitosEn     `json:"digitosEn"`
}

type casoContiene struct {
	Haystack  string `json:"haystack"`
	Needle    string `json:"needle"`
	Resultado bool   `json:"resultado"`
	Nota      string `json:"nota"`
}

type casoNormalizacion struct {
	Entrada      string `json:"entrada"`
	Clean        string `json:"clean"`
	QuitarTildes string `json:"quitarTildes"`
	Plegar       string `json:"plegar"`
	Norm         string `json:"norm"`
	Nota         string `json:"nota"`
}

type casoDigitosEn struct {
	Haystack  string `json:"haystack"`
	Digitos   string `json:"digitos"`
	Resultado bool   `json:"resultado"`
	Nota      string `json:"nota"`
}

func cargaFixture(t *testing.T) fixture {
	t.Helper()
	crudo, err := os.ReadFile("testdata/casos.json")
	if err != nil {
		t.Fatalf("no pude leer el fixture: %v", err)
	}
	var f fixture
	if err := json.Unmarshal(crudo, &f); err != nil {
		t.Fatalf("el fixture no es JSON válido: %v", err)
	}
	if len(f.Contiene) < 40 {
		t.Fatalf("el fixture trae %d casos de contiene(): el port exige al menos 40", len(f.Contiene))
	}
	return f
}

func TestContieneIgualQueElJurado(t *testing.T) {
	f := cargaFixture(t)
	for i, caso := range f.Contiene {
		t.Run(fmt.Sprintf("%02d %s", i, caso.Nota), func(t *testing.T) {
			obtenido := norm.Contiene(caso.Haystack, caso.Needle)
			if obtenido != caso.Resultado {
				t.Fatalf("Contiene(%q, %q) = %v; el jurado dice %v",
					caso.Haystack, caso.Needle, obtenido, caso.Resultado)
			}
		})
	}
}

func TestNormalizacionIgualQueElJurado(t *testing.T) {
	f := cargaFixture(t)
	for i, caso := range f.Normalizacion {
		t.Run(fmt.Sprintf("%02d %s", i, caso.Nota), func(t *testing.T) {
			if obtenido := norm.Clean(caso.Entrada); obtenido != caso.Clean {
				t.Errorf("Clean(%q) = %q; el jurado dice %q", caso.Entrada, obtenido, caso.Clean)
			}
			if obtenido := norm.QuitarTildes(caso.Entrada); obtenido != caso.QuitarTildes {
				t.Errorf("QuitarTildes(%q) = %q; el jurado dice %q", caso.Entrada, obtenido, caso.QuitarTildes)
			}
			if obtenido := norm.Plegar(caso.Entrada); obtenido != caso.Plegar {
				t.Errorf("Plegar(%q) = %q; el jurado dice %q", caso.Entrada, obtenido, caso.Plegar)
			}
			if obtenido := norm.Norm(caso.Entrada); obtenido != caso.Norm {
				t.Errorf("Norm(%q) = %q; el jurado dice %q", caso.Entrada, obtenido, caso.Norm)
			}
		})
	}
}

func TestDigitosEnIgualQueElJurado(t *testing.T) {
	f := cargaFixture(t)
	for i, caso := range f.DigitosEn {
		t.Run(fmt.Sprintf("%02d %s", i, caso.Nota), func(t *testing.T) {
			obtenido := norm.DigitosEn(caso.Haystack, caso.Digitos)
			if obtenido != caso.Resultado {
				t.Fatalf("DigitosEn(%q, %q) = %v; el jurado dice %v",
					caso.Haystack, caso.Digitos, obtenido, caso.Resultado)
			}
		})
	}
}

// TestNormalizaEsNorm comprueba que el mapa busca en el mismo espacio que
// Norm: si no, la detección y la compuerta de fuga medirían distinto (D5).
func TestNormalizaEsNorm(t *testing.T) {
	f := cargaFixture(t)
	for _, caso := range f.Normalizacion {
		if obtenido := norm.Normaliza(caso.Entrada).Texto; obtenido != caso.Norm {
			t.Fatalf("Normaliza(%q).Texto = %q; Norm dice %q", caso.Entrada, obtenido, caso.Norm)
		}
	}
}

// TestMapaDeOffsets es la prueba del mapa: para cada coincidencia encontrada en
// el texto normalizado, el tramo del original que devuelve Original normaliza
// exactamente a lo que se buscó. Sin eso no se puede reemplazar literal.
func TestMapaDeOffsets(t *testing.T) {
	casos := []struct {
		texto  string
		needle string
		nota   string
	}{
		{"cliente: Ana María Gómez, cédula 1.020.304.050", "ana maria gomez", "tildes"},
		{"CL-88421 de JOSÉ  ÁNGEL  RÍOS", "jose angel rios", "espacios repetidos"},
		{"correo Ana.Perez@KipuPay.co y celular +57 300 123 4567", "ana.perez@kipupay.co", "correo"},
		{"el señor Muñoz escribió", "munoz", "eñe"},
		{"González y después Gómez", "gonzalez", "dos apariciones"},
		{"Ana\u200bMaría llamó", "anamaria", "carácter de formato dentro"},
		{"ＡＮＡ PÉREZ", "ana perez", "ancho completo"},
		{"Detalle: ﬁnal del día", "final del dia", "ligadura"},
		{"Vаsyl Dmytrovych", "vasyl", "confusable cirílico"},
		{"  doble   espacio  ", "doble espacio", "espacios en los bordes"},
		{"expediente 1.020.304.050 de Ana", "ana", "después de una cédula"},
		{"Ana\tMaría\nGómez", "maria gomez", "controles de espacio"},
	}
	for _, caso := range casos {
		t.Run(caso.nota, func(t *testing.T) {
			mapa := norm.Normaliza(caso.texto)
			if mapa.Texto != norm.Norm(caso.texto) {
				t.Fatalf("el mapa no usa el espacio de Norm: %q contra %q",
					mapa.Texto, norm.Norm(caso.texto))
			}
			needle := norm.Norm(caso.needle)
			inicio := strings.Index(mapa.Texto, needle)
			if inicio < 0 {
				t.Fatalf("%q no está en %q", needle, mapa.Texto)
			}
			fin := inicio + len(needle)
			de, hasta, ok := mapa.Original(inicio, fin)
			if !ok || de < 0 || hasta > len(caso.texto) || de >= hasta {
				t.Fatalf("Original(%d, %d) = (%d, %d, %v) fuera del original", inicio, fin, de, hasta, ok)
			}
			trozo := caso.texto[de:hasta]
			if !utf8.ValidString(trozo) {
				t.Fatalf("el tramo parte una runa: %q", trozo)
			}
			if obtenido := norm.Norm(trozo); obtenido != mapa.Texto[inicio:fin] {
				t.Fatalf("Norm(%q) = %q; la coincidencia fue %q",
					trozo, obtenido, mapa.Texto[inicio:fin])
			}
			if obtenido := norm.Norm(trozo); obtenido != needle {
				t.Fatalf("Norm(%q) = %q y se buscaba %q", trozo, obtenido, needle)
			}
		})
	}
}

// TestMapaOriginalFueraDeRango fija el contrato del tramo inválido.
func TestMapaOriginalFueraDeRango(t *testing.T) {
	mapa := norm.Normaliza("Ana")
	if _, _, ok := mapa.Original(0, 0); ok {
		t.Fatal("un tramo vacío no puede traducirse")
	}
	if _, _, ok := mapa.Original(0, len(mapa.Texto)+1); ok {
		t.Fatal("un tramo más largo que el texto no puede traducirse")
	}
	if _, _, ok := mapa.Original(-1, 1); ok {
		t.Fatal("un inicio negativo no puede traducirse")
	}
	if de, hasta, ok := mapa.Original(0, len(mapa.Texto)); !ok || de != 0 || hasta != len("Ana") {
		t.Fatalf("el tramo completo es (0, %d, %v), quería (0, 3, true)", hasta, ok)
	}
}
