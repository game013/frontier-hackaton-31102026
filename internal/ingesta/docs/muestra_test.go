package docs_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

func leeDocsJSONL(t *testing.T, ruta string) []modelo.DocRecord {
	t.Helper()
	archivo, err := os.Open(ruta)
	if err != nil {
		t.Fatalf("no pude abrir %s: %v", ruta, err)
	}
	defer archivo.Close()
	var registros []modelo.DocRecord
	escaner := bufio.NewScanner(archivo)
	escaner.Buffer(make([]byte, 0, 1<<20), 1<<22)
	for escaner.Scan() {
		if strings.TrimSpace(escaner.Text()) == "" {
			continue
		}
		var registro modelo.DocRecord
		if err := json.Unmarshal(escaner.Bytes(), &registro); err != nil {
			t.Fatalf("línea ilegible en %s: %v", ruta, err)
		}
		registros = append(registros, registro)
	}
	if err := escaner.Err(); err != nil {
		t.Fatalf("no pude leer %s: %v", ruta, err)
	}
	return registros
}

func clavesDe(metadata map[string]string) []string {
	claves := make([]string, 0, len(metadata))
	for clave := range metadata {
		claves = append(claves, clave)
	}
	sort.Strings(claves)
	return claves
}

// similitudDeTokens compara dos textos con el coeficiente de Dice sobre
// multiconjuntos de palabras: 2·|A∩B| / (|A|+|B|). No distingue mayúsculas y
// cuenta palabras y números; no necesita alinear el texto.
func similitudDeTokens(a, b string) float64 {
	tokens := func(texto string) map[string]int {
		cuenta := map[string]int{}
		var palabra strings.Builder
		for _, letra := range texto {
			if unicode.IsLetter(letra) || unicode.IsDigit(letra) {
				palabra.WriteRune(unicode.ToLower(letra))
				continue
			}
			if palabra.Len() > 0 {
				cuenta[palabra.String()]++
				palabra.Reset()
			}
		}
		if palabra.Len() > 0 {
			cuenta[palabra.String()]++
		}
		return cuenta
	}
	cuentaA, cuentaB := tokens(a), tokens(b)
	total := 0
	for _, veces := range cuentaA {
		total += veces
	}
	for _, veces := range cuentaB {
		total += veces
	}
	if total == 0 {
		return 1
	}
	compartidos := 0
	for token, vecesA := range cuentaA {
		if vecesB, hay := cuentaB[token]; hay {
			if vecesA < vecesB {
				compartidos += vecesA
			} else {
				compartidos += vecesB
			}
		}
	}
	return 2 * float64(compartidos) / float64(total)
}

// TestMuestraDocsCoincideConLaSalidaIdeal es el criterio de la tarea: sobre
// sample/input, las mismas claves de metadata y cuerpos con similitud >= 0,9
// frente a sample/expected_output/docs.jsonl. sample/ no viaja en el repo.
func TestMuestraDocsCoincideConLaSalidaIdeal(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "sample")
	if _, err := os.Stat(filepath.Join(raiz, "input", "docs")); err != nil {
		t.Skip("sin sample/ en disco: nada que comparar")
	}

	registros, err := docs.NuevoLector().Directorio(context.Background(), filepath.Join(raiz, "input", "docs"))
	if err != nil {
		t.Fatalf("Directorio: %v", err)
	}
	ideales := leeDocsJSONL(t, filepath.Join(raiz, "expected_output", "docs.jsonl"))
	if len(registros) != len(ideales) {
		t.Fatalf("ingirió %d documentos, la salida ideal tiene %d", len(registros), len(ideales))
	}

	for i, ideal := range ideales {
		obtenido := registros[i]
		if obtenido.ID != ideal.ID {
			t.Fatalf("registro %d: id %q, esperaba %q", i, obtenido.ID, ideal.ID)
		}
		if obtenido.Format != ideal.Format || obtenido.Filename != ideal.Filename {
			t.Fatalf("docs:%s salió con %q/%q, esperaba %q/%q",
				ideal.ID, obtenido.Format, obtenido.Filename, ideal.Format, ideal.Filename)
		}
		if obtenido.Title == "" {
			t.Fatalf("docs:%s quedó sin title", ideal.ID)
		}
		clavesObtenidas, clavesIdeales := clavesDe(obtenido.Metadata), clavesDe(ideal.Metadata)
		if strings.Join(clavesObtenidas, ",") != strings.Join(clavesIdeales, ",") {
			t.Fatalf("docs:%s metadata con claves %v, la ideal tiene %v",
				ideal.ID, clavesObtenidas, clavesIdeales)
		}
		// Los valores que no llevan PII tienen que salir idénticos; los que sí
		// los cambia el núcleo (author con correo, por ejemplo).
		for _, clave := range clavesIdeales {
			if clave == "author" {
				continue
			}
			if obtenido.Metadata[clave] != ideal.Metadata[clave] {
				t.Fatalf("docs:%s metadata[%s] = %q, la ideal dice %q",
					ideal.ID, clave, obtenido.Metadata[clave], ideal.Metadata[clave])
			}
		}
		similitud := similitudDeTokens(obtenido.Body, ideal.Body)
		if similitud < 0.9 {
			t.Fatalf("docs:%s body con similitud %.3f, se exige 0.9", ideal.ID, similitud)
		}
	}
}

// TestFuentesDocsIngestaElCorpusCompleto es la prueba de humo sobre el corpus
// real descargado: los 26 documentos del índice y cuerpo en los .md, .html y,
// cuando hay pdfplumber, en los .pdf. fuentes/ no viaja en el repo.
func TestFuentesDocsIngestaElCorpusCompleto(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de humo sobre el corpus real")
	}
	carpeta := filepath.Join("..", "..", "..", "fuentes", "docs")
	if _, err := os.Stat(filepath.Join(carpeta, "index.json")); err != nil {
		t.Skip("sin fuentes/ en disco: nada que recorrer")
	}

	lector := docs.NuevoLector()
	lector.Ayudante = rutaAyudante(t)
	conPython := pythonConLibrerias(t, lector.Python)

	registros, err := lector.Directorio(context.Background(), carpeta)
	if err != nil {
		t.Fatalf("Directorio: %v", err)
	}
	if len(registros) != 26 {
		t.Fatalf("ingirió %d documentos, el índice tiene 26", len(registros))
	}
	for _, registro := range registros {
		if strings.TrimSpace(registro.Title) == "" {
			t.Fatalf("docs:%s quedó sin title", registro.ID)
		}
		if registro.Format == "pdf" && !conPython {
			continue
		}
		if strings.TrimSpace(registro.Body) == "" {
			t.Fatalf("docs:%s (%s) quedó sin body", registro.ID, registro.Format)
		}
	}
}
