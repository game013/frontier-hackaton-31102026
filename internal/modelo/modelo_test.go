package modelo

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// directorios devuelve los directorios con salida de referencia: la fixture
// versionada (siempre) y la muestra completa si está en disco.
func directorios(t *testing.T) []string {
	t.Helper()
	dirs := []string{"testdata"}
	if _, err := os.Stat(filepath.Join("..", "..", "sample", "expected_output")); err == nil {
		dirs = append(dirs, filepath.Join("..", "..", "sample", "expected_output"))
	}
	return dirs
}

// mismoJSON compara dos documentos por valor, sin importar el orden de claves.
func mismoJSON(t *testing.T, quiero, tengo string) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal([]byte(quiero), &a); err != nil {
		t.Fatalf("esperado no es JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(tengo), &b); err != nil {
		t.Fatalf("obtenido no es JSON: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("no round-trip.\nesperado: %s\nobtenido: %s", quiero, tengo)
	}
}

func lineas(t *testing.T, ruta string) []string {
	t.Helper()
	f, err := os.Open(ruta)
	if err != nil {
		t.Fatalf("abre %s: %v", ruta, err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for sc.Scan() {
		if len(sc.Bytes()) > 0 {
			out = append(out, sc.Text())
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("lee %s: %v", ruta, err)
	}
	if len(out) == 0 {
		t.Fatalf("%s vacío", ruta)
	}
	return out
}

// Cada registro sale igual que entró, en la fixture versionada y en la muestra.
func TestRegistrosRoundTrip(t *testing.T) {
	casos := []struct {
		archivo string
		nuevo   func() any
	}{
		{"chats.jsonl", func() any { return new(ChatRecord) }},
		{"docs.jsonl", func() any { return new(DocRecord) }},
		{"tables.jsonl", func() any { return new(TableRecord) }},
	}
	for _, c := range casos {
		for _, dir := range directorios(t) {
			for _, linea := range lineas(t, filepath.Join(dir, c.archivo)) {
				r := c.nuevo()
				if err := json.Unmarshal([]byte(linea), r); err != nil {
					t.Fatalf("%s: unmarshal %s: %v", dir, c.archivo, err)
				}
				b, err := json.Marshal(r)
				if err != nil {
					t.Fatalf("%s: marshal %s: %v", dir, c.archivo, err)
				}
				mismoJSON(t, linea, string(b))
			}
		}
	}
}

func TestEntityRoundTrip(t *testing.T) {
	for _, dir := range directorios(t) {
		crudo, err := os.ReadFile(filepath.Join(dir, "entities.json"))
		if err != nil {
			t.Fatalf("lee entities de %s: %v", dir, err)
		}
		var doc struct {
			Entities []Entity `json:"entities"`
		}
		if err := json.Unmarshal(crudo, &doc); err != nil {
			t.Fatalf("%s: unmarshal entities: %v", dir, err)
		}
		if len(doc.Entities) == 0 {
			t.Fatalf("%s: entities.json sin entidades", dir)
		}
		b, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("%s: marshal entities: %v", dir, err)
		}
		mismoJSON(t, string(crudo), string(b))
	}
}

// La forma la exige el validador: los contenedores vacíos viajan como [] o {}
// y nunca como null, aunque el registro llegue incompleto.
func TestContenedoresNuncaNulos(t *testing.T) {
	casos := []struct {
		nombre string
		valor  any
		campo  string
	}{
		{"chats", ChatRecord{}, "attachments"},
		{"docs", DocRecord{}, "metadata"},
		{"tables", TableRecord{}, "columns"},
	}
	for _, c := range casos {
		b, err := json.Marshal(c.valor)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if m[c.campo] == nil {
			t.Fatalf("%s.%s viaja como null: %s", c.nombre, c.campo, b)
		}
	}
}

func claves(t *testing.T, v any) map[string]bool {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// Cada tipo declara exactamente las claves del contrato de output/.
func TestClavesDeRegistro(t *testing.T) {
	casos := []struct {
		nombre string
		valor  any
		quiere []string
	}{
		{"chats", ChatRecord{}, []string{"id", "thread_id", "author", "ts", "text", "attachments"}},
		{"docs", DocRecord{}, []string{"id", "format", "filename", "title", "body", "metadata"}},
		{"tables", TableRecord{}, []string{"id", "table", "columns"}},
	}
	for _, c := range casos {
		got := claves(t, c.valor)
		if len(got) != len(c.quiere) {
			t.Fatalf("%s: claves %v, se esperaban %v", c.nombre, got, c.quiere)
		}
		for _, k := range c.quiere {
			if !got[k] {
				t.Fatalf("%s: falta la clave %q en %v", c.nombre, k, got)
			}
		}
	}
}

// Los tres registros implementan Registro: el núcleo los trata igual.
func TestRegistrosCumplenInterfaz(t *testing.T) {
	registros := []Registro{ChatRecord{ID: "c"}, DocRecord{ID: "d"}, TableRecord{ID: "t"}}
	fuentes := map[string]Fuente{}
	for _, r := range registros {
		if r.IDRegistro() == "" {
			t.Fatal("id vacío")
		}
		fuentes[r.IDRegistro()] = r.FuenteRegistro()
	}
	if fuentes["c"] != FuenteChats || fuentes["d"] != FuenteDocs || fuentes["t"] != FuenteTablas {
		t.Fatalf("fuentes mal: %v", fuentes)
	}
}
