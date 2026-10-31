package captura_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/captura"
)

// La captura existe para reconstruir secuencias del jurado: ids, códigos y
// conteos. Nunca cuerpos: llevan PII.
func TestMiddlewareRegistraIdsCodigosYConteos(t *testing.T) {
	var destino bytes.Buffer
	interior := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := captura.Anota(r)
		m.Job = "job_7f3a"
		m.Fuente = "chats"
		m.Registros = 2
		m.Entidades = 3
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"text":"Manuela Castaño Gaviria"}`))
	})
	manejador := captura.Middleware(interior, captura.Opciones{Destinos: []io.Writer{&destino}})

	peticion := httptest.NewRequest(http.MethodPost, "/v1/anonymize?token=secreto", nil)
	manejador.ServeHTTP(httptest.NewRecorder(), peticion)

	lineas := strings.Split(strings.TrimSpace(destino.String()), "\n")
	if len(lineas) != 1 {
		t.Fatalf("líneas de captura = %d, se esperaba 1: %q", len(lineas), destino.String())
	}
	var linea captura.Linea
	if err := json.Unmarshal([]byte(lineas[0]), &linea); err != nil {
		t.Fatalf("captura no es JSON: %v (%s)", err, lineas[0])
	}
	if linea.N != 1 || linea.Metodo != http.MethodPost || linea.Ruta != "/v1/anonymize" {
		t.Fatalf("cabecera mal: %+v", linea)
	}
	if linea.Codigo != http.StatusAccepted {
		t.Fatalf("codigo = %d, se esperaba %d", linea.Codigo, http.StatusAccepted)
	}
	if linea.Job != "job_7f3a" || linea.Fuente != "chats" || linea.Registros != 2 || linea.Entidades != 3 {
		t.Fatalf("métrica perdida: %+v", linea)
	}
	for _, prohibido := range []string{"Manuela", "Castaño", "secreto", "token="} {
		if strings.Contains(lineas[0], prohibido) {
			t.Fatalf("la captura filtró %q: %s", prohibido, lineas[0])
		}
	}
}

// Anota fuera del middleware no rompe el handler: devuelve una métrica
// descartable.
func TestAnotaSinMiddleware(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	m := captura.Anota(r)
	if m == nil {
		t.Fatal("Anota devolvió nil")
	}
	m.Registros = 1
}

// Cada request recibe su número de llegada, aunque responda sin cuerpo.
func TestMiddlewareNumeraYMarcaErrores(t *testing.T) {
	var destino bytes.Buffer
	manejador := captura.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captura.Anota(r).Error = "sin_corrida"
		w.WriteHeader(http.StatusNotFound)
	}), captura.Opciones{Destinos: []io.Writer{&destino}})
	for i := 0; i < 2; i++ {
		manejador.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/output/chats.jsonl", nil))
	}
	lineas := strings.Split(strings.TrimSpace(destino.String()), "\n")
	if len(lineas) != 2 {
		t.Fatalf("líneas = %d, se esperaban 2", len(lineas))
	}
	var segunda captura.Linea
	if err := json.Unmarshal([]byte(lineas[1]), &segunda); err != nil {
		t.Fatalf("captura no es JSON: %v", err)
	}
	if segunda.N != 2 || segunda.Codigo != http.StatusNotFound || segunda.Error != "sin_corrida" {
		t.Fatalf("segunda línea mal: %+v", segunda)
	}
}
