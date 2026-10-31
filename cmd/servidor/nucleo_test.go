package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/chats"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
	"github.com/game013/frontier-hackaton-31102026/internal/servidor"
)

// registrosDeMuestra toma los mensajes de la muestra; la prueba se salta si el
// directorio no está descargado.
func registrosDeMuestra(t *testing.T) []modelo.ChatRecord {
	t.Helper()
	raiz := filepath.Join("..", "..", "sample", "input", "chats")
	if _, err := os.Stat(raiz); err != nil {
		t.Skipf("sin muestra: %v", err)
	}
	corpus, err := chats.Cargar(raiz)
	if err != nil {
		t.Fatalf("chats: %v", err)
	}
	// El lote que manda el jurado: registros canónicos, sin metadata de ingesta.
	return corpus.Registros
}

func config(datosDir string) servidor.Config {
	return servidor.Config{Puerto: "0", DatosDir: datosDir, Equipo: "prueba", Version: "prueba"}
}

// La prueba en vivo del contrato ya anonimiza de verdad: sin eco, sin formas
// reales y con el mismo seudónimo en dos llamadas (D8, D24).
func TestAnonymizeEnVivoAnonimiza(t *testing.T) {
	dir := t.TempDir()
	permiso, err := abreNucleo(dir, func(string) string { return "" })
	if err != nil {
		t.Fatalf("abreNucleo: %v", err)
	}
	defer permiso.Cerrar()
	svc, err := servidor.Nuevo(config(dir), servidor.Deps{Nucleo: permiso.Nucleo})
	if err != nil {
		t.Fatalf("servidor: %v", err)
	}
	srv := httptest.NewServer(svc.Handler())
	defer srv.Close()

	registros := registrosDeMuestra(t)
	cuerpo, err := json.Marshal(map[string]any{"source": "chats", "records": registros})
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	pide := func() (int, []modelo.ChatRecord, []modelo.Entity) {
		respuesta, err := http.Post(srv.URL+"/v1/anonymize", "application/json", bytes.NewReader(cuerpo))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer respuesta.Body.Close()
		var sobre struct {
			Records  []modelo.ChatRecord `json:"records"`
			Entities []modelo.Entity     `json:"entities"`
		}
		if err := json.NewDecoder(respuesta.Body).Decode(&sobre); err != nil {
			t.Fatalf("decodifica: %v", err)
		}
		return respuesta.StatusCode, sobre.Records, sobre.Entities
	}

	estado, primera, entidades := pide()
	if estado != http.StatusOK {
		t.Fatalf("estado = %d, quiero 200", estado)
	}
	if len(primera) != len(registros) {
		t.Fatalf("registros = %d, quiero %d", len(primera), len(registros))
	}
	if len(entidades) == 0 {
		t.Fatal("sin entities.json en la respuesta")
	}
	for _, r := range primera {
		for _, valor := range []string{"Manuela Castaño", "mcastano@gmail.com", "1.107.360.039"} {
			if norm.Contiene(r.Author+r.Text, valor) {
				t.Fatalf("el registro %s conserva una forma real", r.ID)
			}
		}
	}

	// Segunda llamada idéntica: el perfil sale de la bóveda, no del azar.
	_, segunda, _ := pide()
	for i := range primera {
		if primera[i].Author != segunda[i].Author || primera[i].Text != segunda[i].Text {
			t.Fatalf("el registro %s cambió entre llamadas", primera[i].ID)
		}
	}
}

// El secreto generado queda en el volumen: reabrir no cambia los perfiles.
func TestSecretoPersisteEnElVolumen(t *testing.T) {
	dir := t.TempDir()
	primero, err := secretoDe(dir, "")
	if err != nil {
		t.Fatalf("secretoDe: %v", err)
	}
	segundo, err := secretoDe(dir, "")
	if err != nil {
		t.Fatalf("secretoDe: %v", err)
	}
	if !bytes.Equal(primero, segundo) {
		t.Fatal("el secreto cambió al reabrir")
	}
	otro, err := secretoDe(t.TempDir(), "")
	if err != nil {
		t.Fatalf("secretoDe: %v", err)
	}
	if bytes.Equal(primero, otro) {
		t.Fatal("dos volúmenes comparten secreto")
	}
	delEntorno, err := secretoDe(dir, "secreto-del-entorno")
	if err != nil {
		t.Fatalf("secretoDe: %v", err)
	}
	if string(delEntorno) != "secreto-del-entorno" {
		t.Fatal("la variable de entorno no manda")
	}
}

// Una allowlist ilegible no puede pasar en silencio.
func TestAllowlistIlegibleFalla(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "allowlist.json")
	if err := os.WriteFile(ruta, []byte("{no es json"), 0o600); err != nil {
		t.Fatalf("escribe: %v", err)
	}
	if _, err := allowlistDe(ruta); err == nil {
		t.Fatal("quería error con una allowlist rota")
	}
}
