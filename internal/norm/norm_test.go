package norm_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

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
