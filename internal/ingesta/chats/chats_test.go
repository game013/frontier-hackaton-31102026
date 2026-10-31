package chats

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// raiz es la raíz del repo desde este paquete.
const raiz = "../../.."

// muestra devuelve una ruta de sample/, que es un directorio ignorado por git: si
// no está, la prueba se salta en vez de fallar.
func muestra(t *testing.T, partes ...string) string {
	t.Helper()
	ruta := filepath.Join(append([]string{raiz, "sample"}, partes...)...)
	if _, err := os.Stat(ruta); err != nil {
		t.Skipf("sin muestra %s: %v", ruta, err)
	}
	return ruta
}

// salidaIdeal lee sample/expected_output/chats.jsonl, el formato que la ingesta
// tiene que igualar en id, thread_id y número de adjuntos.
func salidaIdeal(t *testing.T) []modelo.ChatRecord {
	t.Helper()
	f, err := os.Open(muestra(t, "expected_output", "chats.jsonl"))
	if err != nil {
		t.Fatalf("abre chats.jsonl: %v", err)
	}
	defer f.Close()
	var out []modelo.ChatRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var r modelo.ChatRecord
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("lee chats.jsonl: %v", err)
		}
		out = append(out, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("lee chats.jsonl: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("chats.jsonl vacío")
	}
	return out
}

func cargarMuestra(t *testing.T) *Corpus {
	t.Helper()
	c, err := Cargar(muestra(t, "input", "chats"))
	if err != nil {
		t.Fatalf("Cargar(sample/input/chats): %v", err)
	}
	return c
}

func TestCargarMuestra(t *testing.T) {
	quiero := salidaIdeal(t)
	corpus := cargarMuestra(t)

	if len(corpus.Registros) != len(quiero) {
		t.Fatalf("registros = %d, quiero %d", len(corpus.Registros), len(quiero))
	}
	for i, esperado := range quiero {
		tengo := corpus.Registros[i]
		if tengo.ID != esperado.ID {
			t.Errorf("registro %d: id = %q, quiero %q", i, tengo.ID, esperado.ID)
		}
		if tengo.ThreadID != esperado.ThreadID {
			t.Errorf("registro %s: thread_id = %q, quiero %q", tengo.ID, tengo.ThreadID, esperado.ThreadID)
		}
		if len(tengo.Attachments) != len(esperado.Attachments) {
			t.Errorf("registro %s: %d adjuntos, quiero %d", tengo.ID, len(tengo.Attachments), len(esperado.Attachments))
		}
		if tengo.TS != esperado.TS {
			t.Errorf("registro %s: ts = %q, quiero %q", tengo.ID, tengo.TS, esperado.TS)
		}
	}
}

// TestAutorNuncaEsUnUID cubre la regla del enunciado: author es un nombre, no el
// uid del mensaje en users.json.
func TestAutorNuncaEsUnUID(t *testing.T) {
	corpus := cargarMuestra(t)
	if len(corpus.Usuarios) == 0 {
		t.Fatal("índice de usuarios vacío")
	}
	for _, r := range corpus.Registros {
		if r.Author == "" {
			t.Fatalf("registro %s sin author", r.ID)
		}
		if _, esUID := corpus.Usuarios[r.Author]; esUID {
			t.Errorf("registro %s: author es un uid (%q)", r.ID, r.Author)
		}
	}
}

// TestNombreCortoEsPrefijoDelCompleto es la invariante que el resto del
// pipeline asume sobre el índice: author es una forma corta del nombre completo,
// nunca el uid ni otra cadena.
func TestNombreCortoEsPrefijoDelCompleto(t *testing.T) {
	corpus := cargarMuestra(t)
	for uid, u := range corpus.Usuarios {
		if u.NombreCompleto == "" {
			if u.Nombre != "" {
				t.Errorf("%s sin nombre completo pero con nombre %q", uid, u.Nombre)
			}
			continue
		}
		if !strings.HasPrefix(u.NombreCompleto, u.Nombre) {
			t.Errorf("%s: nombre %q no encabeza %q", uid, u.Nombre, u.NombreCompleto)
		}
	}
}

// TestCargarFuentes es la prueba de humo sobre el corpus real: los ids tienen que
// ser exactamente los de record_index.json.
func TestCargarFuentes(t *testing.T) {
	raizFuentes := filepath.Join(raiz, "fuentes")
	if _, err := os.Stat(raizFuentes); err != nil {
		t.Skipf("sin fuentes: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(raizFuentes, "record_index.json"))
	if err != nil {
		t.Fatalf("lee record_index.json: %v", err)
	}
	var indice struct {
		Chats []string `json:"chats"`
	}
	if err := json.Unmarshal(b, &indice); err != nil {
		t.Fatalf("parsea record_index.json: %v", err)
	}

	corpus, err := Cargar(filepath.Join(raizFuentes, "chats"))
	if err != nil {
		t.Fatalf("Cargar(fuentes/chats): %v", err)
	}

	esperados := make(map[string]bool, len(indice.Chats))
	for _, id := range indice.Chats {
		esperados[id] = true
	}
	if len(corpus.Registros) != len(esperados) {
		t.Fatalf("registros = %d, quiero %d", len(corpus.Registros), len(esperados))
	}
	for _, r := range corpus.Registros {
		if !esperados[r.ID] {
			t.Errorf("id %s fuera de record_index", r.ID)
		}
	}
	if len(corpus.Incidencias) != 0 {
		t.Errorf("el corpus real no tiene excepciones: %v", corpus.Incidencias)
	}
}
