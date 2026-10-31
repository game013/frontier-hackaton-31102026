package servidor_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/servidor"
)

// nuevo levanta el servicio en un puerto efímero con una carpeta de datos
// temporal. La muestra es un directorio ignorado por git.
func nuevo(t *testing.T, deps servidor.Deps) *httptest.Server {
	t.Helper()
	s, err := servidor.Nuevo(servidor.Config{
		Puerto: "0", DatosDir: t.TempDir(), Equipo: "kipu-frontier", Version: "prueba",
		Modelos: []string{},
	}, deps)
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	h := httptest.NewServer(s.Handler())
	t.Cleanup(func() {
		h.Close()
		if err := s.Cerrar(); err != nil {
			t.Errorf("Cerrar: %v", err)
		}
	})
	return h
}

func pide(t *testing.T, metodo, url, cuerpo string) (int, []byte) {
	t.Helper()
	var lector io.Reader
	if cuerpo != "" {
		lector = strings.NewReader(cuerpo)
	}
	peticion, err := http.NewRequest(metodo, url, lector)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if cuerpo != "" {
		peticion.Header.Set("Content-Type", "application/json")
	}
	cliente := &http.Client{Timeout: 10 * time.Second}
	respuesta, err := cliente.Do(peticion)
	if err != nil {
		t.Fatalf("%s %s: %v", metodo, url, err)
	}
	defer respuesta.Body.Close()
	datos, err := io.ReadAll(respuesta.Body)
	if err != nil {
		t.Fatalf("lee respuesta: %v", err)
	}
	return respuesta.StatusCode, datos
}

// chatsDeMuestra devuelve las primeras n líneas de la salida ideal de chats.
func chatsDeMuestra(t *testing.T, n int) []string {
	t.Helper()
	ruta := filepath.Join("..", "..", "sample", "expected_output", "chats.jsonl")
	f, err := os.Open(ruta)
	if err != nil {
		t.Skipf("sin muestra %s: %v", ruta, err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for sc.Scan() && len(out) < n {
		if len(sc.Bytes()) > 0 {
			out = append(out, sc.Text())
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("lee muestra: %v", err)
	}
	if len(out) < n {
		t.Fatalf("la muestra trae %d mensajes, se esperaban %d", len(out), n)
	}
	return out
}

type respuestaAnonimiza struct {
	Records  []json.RawMessage `json:"records"`
	Entities []json.RawMessage `json:"entities"`
}

func TestSaludRespondeStatusEquipoYModelos(t *testing.T) {
	h := nuevo(t, servidor.Deps{})
	codigo, cuerpo := pide(t, http.MethodGet, h.URL+"/v1/health", "")
	if codigo != http.StatusOK {
		t.Fatalf("health = %d: %s", codigo, cuerpo)
	}
	var salud struct {
		Status  string   `json:"status"`
		Team    string   `json:"team"`
		Version string   `json:"version"`
		Models  []string `json:"models"`
	}
	if err := json.Unmarshal(cuerpo, &salud); err != nil {
		t.Fatalf("health no es JSON: %v", err)
	}
	if salud.Status != "ok" || salud.Team == "" || salud.Version == "" {
		t.Fatalf("health incompleto: %s", cuerpo)
	}
	if salud.Models == nil {
		t.Fatalf("models debe viajar como lista: %s", cuerpo)
	}
}

// /anonymize es la prueba en vivo del jurado: el eco de C1 tiene que devolver
// los mismos ids con el esquema del contrato y una lista de entidades.
func TestAnonimizaDevuelveEcoValido(t *testing.T) {
	chats := chatsDeMuestra(t, 2)
	cuerpo := fmt.Sprintf(`{"source":"chats","records":[%s,%s]}`, chats[0], chats[1])
	h := nuevo(t, servidor.Deps{})

	codigo, datos := pide(t, http.MethodPost, h.URL+"/v1/anonymize", cuerpo)
	if codigo != http.StatusOK {
		t.Fatalf("anonymize = %d: %s", codigo, datos)
	}
	var respuesta respuestaAnonimiza
	if err := json.Unmarshal(datos, &respuesta); err != nil {
		t.Fatalf("anonymize no es JSON: %v", err)
	}
	if len(respuesta.Records) != 2 {
		t.Fatalf("records = %d, se esperaban 2: %s", len(respuesta.Records), datos)
	}
	if respuesta.Entities == nil || len(respuesta.Entities) != 0 {
		t.Fatalf("entities debe ser una lista vacía, no null: %s", datos)
	}
	for i, esperado := range chats {
		var quiero, tengo map[string]any
		if err := json.Unmarshal([]byte(esperado), &quiero); err != nil {
			t.Fatalf("muestra ilegible: %v", err)
		}
		if err := json.Unmarshal(respuesta.Records[i], &tengo); err != nil {
			t.Fatalf("registro %d ilegible: %v", i, err)
		}
		if quiero["id"] != tengo["id"] {
			t.Fatalf("registro %d cambió de id: %v != %v", i, quiero["id"], tengo["id"])
		}
		if !reflect.DeepEqual(quiero, tengo) {
			t.Fatalf("registro %d no round-trip:\nesperado: %s\nobtenido: %s", i, esperado, respuesta.Records[i])
		}
	}
}

func TestAnonimizaRechazaRegistroFueraDeEsquema(t *testing.T) {
	casos := []struct {
		nombre string
		cuerpo string
	}{
		{"sin records", `{"source":"chats"}`},
		{"fuente desconocida", `{"source":"pdfs","records":[]}`},
		{"campo de más", `{"source":"chats","records":[{"id":"a","thread_id":"t","author":"x","ts":"1","text":"y","attachments":[],"extra":1}]}`},
		{"falta text", `{"source":"chats","records":[{"id":"a","thread_id":"t","author":"x","ts":"1","attachments":[]}]}`},
		{"id vacío", `{"source":"tables","records":[{"id":"","table":"clientes","columns":{}}]}`},
		{"tabla desconocida", `{"source":"tables","records":[{"id":"t:1","table":"otros","columns":{}}]}`},
		{"formato de doc desconocido", `{"source":"docs","records":[{"id":"d1","format":"txt","filename":"a.txt","title":"","body":"","metadata":{}}]}`},
	}
	h := nuevo(t, servidor.Deps{})
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			codigo, datos := pide(t, http.MethodPost, h.URL+"/v1/anonymize", caso.cuerpo)
			if codigo != http.StatusUnprocessableEntity {
				t.Fatalf("codigo = %d, se esperaba %d: %s", codigo, http.StatusUnprocessableEntity, datos)
			}
			if !strings.Contains(string(datos), "error") {
				t.Fatalf("falta el error en la respuesta: %s", datos)
			}
		})
	}
}

func TestAnonimizaRechazaLoteMayorA50(t *testing.T) {
	registros := make([]string, 51)
	for i := range registros {
		registros[i] = fmt.Sprintf(`{"id":"t:%d","table":"clientes","columns":{}}`, i)
	}
	cuerpo := fmt.Sprintf(`{"source":"tables","records":[%s]}`, strings.Join(registros, ","))
	h := nuevo(t, servidor.Deps{})
	if codigo, datos := pide(t, http.MethodPost, h.URL+"/v1/anonymize", cuerpo); codigo != http.StatusUnprocessableEntity {
		t.Fatalf("codigo = %d, se esperaba 422: %s", codigo, datos)
	}
}

func TestAnonimizaRechazaCuerpoIlegible(t *testing.T) {
	h := nuevo(t, servidor.Deps{})
	if codigo, datos := pide(t, http.MethodPost, h.URL+"/v1/anonymize", `{`); codigo != http.StatusBadRequest {
		t.Fatalf("codigo = %d, se esperaba 400: %s", codigo, datos)
	}
}

// ejecutorDePrueba escribe los cuatro archivos y espera la señal de liberación,
// para observar el job mientras corre.
type ejecutorDePrueba struct {
	liberar chan struct{}
	veces   int
	mu      sync.Mutex
}

func (e *ejecutorDePrueba) Ejecuta(_ context.Context, sourcesURL, token, dir string) (servidor.Conteos, error) {
	e.mu.Lock()
	e.veces++
	e.mu.Unlock()
	<-e.liberar
	if sourcesURL == "" || token == "" || dir == "" {
		return servidor.Conteos{}, fmt.Errorf("faltan datos de la corrida")
	}
	archivos := map[string]string{
		"chats.jsonl":   "{\"id\":\"c1\"}\n",
		"docs.jsonl":    "",
		"tables.jsonl":  "",
		"entities.json": `{"entities":[]}`,
	}
	for nombre, contenido := range archivos {
		if err := os.WriteFile(filepath.Join(dir, nombre), []byte(contenido), 0o600); err != nil {
			return servidor.Conteos{}, err
		}
	}
	return servidor.Conteos{Chats: 1}, nil
}

func TestCorridaEncolaReportaYPublicaSalida(t *testing.T) {
	ejecutor := &ejecutorDePrueba{liberar: make(chan struct{})}
	h := nuevo(t, servidor.Deps{Ejecutor: ejecutor})
	peticion := fmt.Sprintf(`{"sources_url":"%s","token":"tok_de_prueba"}`, h.URL)

	codigo, datos := pide(t, http.MethodPost, h.URL+"/v1/run", peticion)
	if codigo != http.StatusAccepted {
		t.Fatalf("run = %d: %s", codigo, datos)
	}
	var aceptado struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(datos, &aceptado); err != nil || aceptado.JobID == "" {
		t.Fatalf("falta job_id: %s (%v)", datos, err)
	}

	// Idempotente: mientras corre, /run devuelve el mismo job.
	if _, datos := pide(t, http.MethodPost, h.URL+"/v1/run", peticion); !strings.Contains(string(datos), aceptado.JobID) {
		t.Fatalf("segundo /run no reusó el job %s: %s", aceptado.JobID, datos)
	}

	estado := esperaEstado(t, h.URL+"/v1/run/"+aceptado.JobID, "running")
	if estado.StartedAt == "" {
		t.Fatalf("running sin started_at: %+v", estado)
	}

	// Sin corrida terminada, /output no sirve nada.
	if codigo, _ := pide(t, http.MethodGet, h.URL+"/v1/output/chats.jsonl", ""); codigo != http.StatusNotFound {
		t.Fatalf("/output antes de terminar = %d, se esperaba 404", codigo)
	}

	close(ejecutor.liberar)
	estado = esperaEstado(t, h.URL+"/v1/run/"+aceptado.JobID, "done")
	if estado.FinishedAt == "" || estado.Records.Chats != 1 {
		t.Fatalf("corrida terminada mal: %+v", estado)
	}

	codigo, chats := pide(t, http.MethodGet, h.URL+"/v1/output/chats.jsonl", "")
	if codigo != http.StatusOK || !strings.Contains(string(chats), `"c1"`) {
		t.Fatalf("/output/chats.jsonl = %d: %s", codigo, chats)
	}
	codigo, entidades := pide(t, http.MethodGet, h.URL+"/v1/entities", "")
	if codigo != http.StatusOK || !strings.Contains(string(entidades), `"entities"`) {
		t.Fatalf("/v1/entities = %d: %s", codigo, entidades)
	}
	if codigo, _ := pide(t, http.MethodGet, h.URL+"/v1/output/otro.json", ""); codigo != http.StatusNotFound {
		t.Fatalf("/output/otro.json = %d, se esperaba 404", codigo)
	}
	if ejecutor.veces != 1 {
		t.Fatalf("el ejecutor corrió %d veces, se esperaba 1", ejecutor.veces)
	}
}

type estadoCorrida struct {
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Records    struct {
		Chats  int `json:"chats"`
		Docs   int `json:"docs"`
		Tables int `json:"tables"`
	} `json:"records"`
	Error string `json:"error"`
}

func esperaEstado(t *testing.T, url, quiero string) estadoCorrida {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for {
		codigo, datos := pide(t, http.MethodGet, url, "")
		if codigo != http.StatusOK {
			t.Fatalf("estado = %d: %s", codigo, datos)
		}
		var estado estadoCorrida
		if err := json.Unmarshal(datos, &estado); err != nil {
			t.Fatalf("estado no es JSON: %v", err)
		}
		if estado.Status == quiero {
			return estado
		}
		if time.Now().After(limite) {
			t.Fatalf("estado %q no llegó: %+v", quiero, estado)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCorridaSinCablearTerminaFallida(t *testing.T) {
	h := nuevo(t, servidor.Deps{})
	codigo, datos := pide(t, http.MethodPost, h.URL+"/v1/run",
		fmt.Sprintf(`{"sources_url":"%s","token":"tok"}`, h.URL))
	if codigo != http.StatusAccepted {
		t.Fatalf("run = %d: %s", codigo, datos)
	}
	var aceptado struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(datos, &aceptado); err != nil {
		t.Fatalf("run no es JSON: %v", err)
	}
	estado := esperaEstado(t, h.URL+"/v1/run/"+aceptado.JobID, "failed")
	if estado.Error == "" {
		t.Fatalf("failed sin error: %+v", estado)
	}
	if codigo, _ := pide(t, http.MethodGet, h.URL+"/v1/output/chats.jsonl", ""); codigo != http.StatusNotFound {
		t.Fatalf("/output tras corrida fallida = %d, se esperaba 404", codigo)
	}
}

func TestRunValidaCuerpoYJobDesconocido(t *testing.T) {
	h := nuevo(t, servidor.Deps{})
	casos := []string{
		`{"sources_url":"http://ejemplo/v1"}`,
		`{"token":"tok"}`,
		`{"sources_url":"no-es-url","token":"tok"}`,
		`{"sources_url":"ftp://ejemplo/v1","token":"tok"}`,
		`{`,
	}
	for _, cuerpo := range casos {
		if codigo, datos := pide(t, http.MethodPost, h.URL+"/v1/run", cuerpo); codigo != http.StatusBadRequest {
			t.Fatalf("run %s = %d, se esperaba 400: %s", cuerpo, codigo, datos)
		}
	}
	if codigo, _ := pide(t, http.MethodGet, h.URL+"/v1/run/job_inexistente", ""); codigo != http.StatusNotFound {
		t.Fatalf("job desconocido = %d, se esperaba 404", codigo)
	}
	if codigo, _ := pide(t, http.MethodGet, h.URL+"/v1/entities", ""); codigo != http.StatusNotFound {
		t.Fatalf("/v1/entities sin corrida = %d, se esperaba 404", codigo)
	}
}

// El núcleo es el único camino de anonimización (D8): /anonymize usa el que se
// inyecte, y lo hace una sola vez por lote.
type nucleoDePrueba struct {
	fuente       modelo.Fuente
	registros    int
	entidades    []modelo.Entity
	invocaciones int
}

func (n *nucleoDePrueba) Anonymize(_ context.Context, fuente modelo.Fuente, registros []modelo.Registro) (modelo.Resultado, error) {
	n.invocaciones++
	n.fuente = fuente
	n.registros = len(registros)
	return modelo.Resultado{Registros: registros, Entities: n.entidades}, nil
}

func TestAnonimizaUsaElNucleoInyectado(t *testing.T) {
	nucleo := &nucleoDePrueba{entidades: []modelo.Entity{{
		EntityID: "p_cl_1", Type: modelo.EntidadPersona, Originals: []string{"x"}, Pseudonyms: map[string]string{"name": "y"},
	}}}
	h := nuevo(t, servidor.Deps{Nucleo: nucleo})
	cuerpo := `{"source":"tables","records":[{"id":"empleados:E-1","table":"empleados","columns":{"nombre":"x"}}]}`
	codigo, datos := pide(t, http.MethodPost, h.URL+"/v1/anonymize", cuerpo)
	if codigo != http.StatusOK {
		t.Fatalf("anonymize = %d: %s", codigo, datos)
	}
	if nucleo.invocaciones != 1 || nucleo.fuente != modelo.FuenteTablas || nucleo.registros != 1 {
		t.Fatalf("el núcleo no recibió el lote: %+v", nucleo)
	}
	if !strings.Contains(string(datos), `"p_cl_1"`) {
		t.Fatalf("entities no viajó: %s", datos)
	}
}
