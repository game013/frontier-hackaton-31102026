package servidor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/campos"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
	"github.com/game013/frontier-hackaton-31102026/internal/llm"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
	"github.com/game013/frontier-hackaton-31102026/internal/nucleo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

const (
	tokenFuentes = "token-de-prueba"
	// secretoPruebaEjecutor es de pruebas: en el servicio el secreto sale del
	// entorno o del volumen (D4).
	secretoPruebaEjecutor = "secreto-de-prueba"
)

// muestraDeCorpus es la carpeta con chats/, docs/ y tables/ que sirve el API
// falso. Sin muestra descargada la prueba se salta (está ignorada por git).
func muestraDeCorpus(t *testing.T) string {
	t.Helper()
	raiz := filepath.Join("..", "..", "sample")
	if _, err := os.Stat(filepath.Join(raiz, "input")); err != nil {
		t.Skipf("sin muestra: %v", err)
	}
	return raiz
}

// apiFalsa sirve una copia de sample/ con las rutas del API de fuentes. Exige
// el token en Authorization, así que una corrida sin token falla aquí igual que
// contra el API real.
func apiFalsa(t *testing.T, muestra string) http.Handler {
	t.Helper()
	entrada := filepath.Join(muestra, "input")
	mux := http.NewServeMux()
	archivo := func(ruta string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			datos, err := os.ReadFile(ruta)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Write(datos)
		}
	}
	mux.HandleFunc("GET /v1/chats/users", archivo(filepath.Join(entrada, "chats", "users.json")))
	mux.HandleFunc("GET /v1/chats/channels", archivo(filepath.Join(entrada, "chats", "channels.json")))
	mux.HandleFunc("GET /v1/chats/{canal}", func(w http.ResponseWriter, r *http.Request) {
		dias, err := os.ReadDir(filepath.Join(entrada, "chats", r.PathValue("canal")))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		fechas := []string{}
		for _, dia := range dias {
			if nombre, ok := strings.CutSuffix(dia.Name(), ".json"); ok && !dia.IsDir() {
				fechas = append(fechas, nombre)
			}
		}
		sort.Strings(fechas)
		json.NewEncoder(w).Encode(fechas)
	})
	mux.HandleFunc("GET /v1/chats/{canal}/{fecha}", func(w http.ResponseWriter, r *http.Request) {
		archivo(filepath.Join(entrada, "chats", r.PathValue("canal"), r.PathValue("fecha")+".json"))(w, r)
	})
	mux.HandleFunc("GET /v1/docs", archivo(filepath.Join(entrada, "docs", "index.json")))
	mux.HandleFunc("GET /v1/docs/{id}", func(w http.ResponseWriter, r *http.Request) {
		nombre := nombreDeDocumento(t, entrada, r.PathValue("id"))
		if nombre == "" {
			http.NotFound(w, r)
			return
		}
		archivo(filepath.Join(entrada, "docs", nombre))(w, r)
	})
	mux.HandleFunc("GET /v1/tables", func(w http.ResponseWriter, r *http.Request) {
		// La muestra no trae index.json de tablas: se arma con los archivos que
		// hay, con la forma que documenta el OpenAPI de fuentes.
		entradas, err := os.ReadDir(filepath.Join(entrada, "tables"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		indice := []map[string]any{}
		for _, e := range entradas {
			if e.IsDir() || e.Name() == "README.txt" {
				continue
			}
			indice = append(indice, map[string]any{"name": e.Name(), "format": "csv"})
		}
		json.NewEncoder(w).Encode(indice)
	})
	mux.HandleFunc("GET /v1/tables/{nombre}", func(w http.ResponseWriter, r *http.Request) {
		archivo(filepath.Join(entrada, "tables", r.PathValue("nombre")))(w, r)
	})
	mux.HandleFunc("GET /v1/record_index", archivo(filepath.Join(muestra, "record_index.json")))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+tokenFuentes {
			http.Error(w, "sin token", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// nombreDeDocumento traduce el id del índice de la muestra a su archivo.
func nombreDeDocumento(t *testing.T, entrada, id string) string {
	t.Helper()
	crudo, err := os.ReadFile(filepath.Join(entrada, "docs", "index.json"))
	if err != nil {
		t.Fatalf("index.json: %v", err)
	}
	var entradas []struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(crudo, &entradas); err != nil {
		t.Fatalf("index.json: %v", err)
	}
	for _, e := range entradas {
		if e.ID == id {
			return e.Filename
		}
	}
	return ""
}

// ejecutorDePrueba arma el ejecutor con un núcleo sin bóveda: la corrida es
// determinista y no toca /data.
func ejecutorDePrueba(t *testing.T) *EjecutorFuentes {
	t.Helper()
	return ejecutorConContabilidad(t, nil)
}

// ejecutorConContabilidad es igual, pero con la contabilidad del modelo que el
// servicio le pasa desde el cliente de Jev.
func ejecutorConContabilidad(t *testing.T, uso func() llm.Uso) *EjecutorFuentes {
	t.Helper()
	n, err := nucleo.Nuevo(nucleo.Opciones{
		Secreto: []byte(secretoPruebaEjecutor),
		Plazo:   2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("nucleo: %v", err)
	}
	ejecutor, err := NuevoEjecutor(n, docs.NuevoLector(), uso)
	if err != nil {
		t.Fatalf("NuevoEjecutor: %v", err)
	}
	return ejecutor
}

// /run completo contra un API falso: descarga, materializa el árbol, ingesta,
// anonimiza y deja los cuatro archivos. Los conteos son los ids que declara
// record_index (10 chats, 1 doc, 4 filas de tablas).
func TestEjecutaDescargaYAnonimizaLaMuestra(t *testing.T) {
	muestra := muestraDeCorpus(t)
	srv := httptest.NewServer(apiFalsa(t, muestra))
	defer srv.Close()

	dir := t.TempDir()
	ejecutor := ejecutorDePrueba(t)
	conteos, err := ejecutor.Ejecuta(context.Background(), srv.URL+"/v1", tokenFuentes, dir)
	if err != nil {
		t.Fatalf("Ejecuta: %v", err)
	}
	if quiero := (Conteos{Chats: 10, Docs: 1, Tablas: 4}); conteos != quiero {
		t.Fatalf("conteos = %+v, quiero %+v", conteos, quiero)
	}

	// El corpus materializado es lo que leen los lectores de ingesta.
	for _, ruta := range []string{
		filepath.Join(dir, carpetaFuentes, "record_index.json"),
		filepath.Join(dir, carpetaFuentes, "chats", "users.json"),
		filepath.Join(dir, carpetaFuentes, "chats", "channels.json"),
		filepath.Join(dir, carpetaFuentes, "docs", "index.json"),
		filepath.Join(dir, carpetaFuentes, "tables", "index.json"),
	} {
		if _, err := os.Stat(ruta); err != nil {
			t.Errorf("falta %s: %v", ruta, err)
		}
	}

	// La salida está en dirSalida, que es lo que sirve /output/{archivo}.
	limpios := leeChatsDeSalida(t, dir)
	if len(limpios) != 10 {
		t.Fatalf("chats.jsonl trae %d registros, quiero 10", len(limpios))
	}
	for _, nombre := range []string{"docs.jsonl", "tables.jsonl", "entities.json"} {
		if _, err := os.Stat(filepath.Join(dir, nombre)); err != nil {
			t.Errorf("falta %s en la salida: %v", nombre, err)
		}
	}

	// Y es una salida anonimizada: ninguna forma real del corpus sobrevive, con
	// la misma vara del jurado (J-GLO).
	entrada, err := nucleo.CargaCorpus(context.Background(), filepath.Join(dir, carpetaFuentes), docs.NuevoLector())
	if err != nil {
		t.Fatalf("CargaCorpus: %v", err)
	}
	for _, forma := range formasDelCorpus(t, entrada) {
		for _, r := range limpios {
			for _, campo := range campos.Lista(r) {
				if norm.Contiene(campo.Valor, forma) {
					t.Errorf("%s %s conserva una forma real", r.IDRegistro(), campo.Ruta)
				}
			}
		}
	}
}

// contabilidad es la del cliente de Jev tal como la ve el ejecutor: acumulada
// desde que arrancó el proceso y creciente entre corridas. El mismo cliente
// sirve a /run y a /anonymize, que es de donde salió el bug de sumar a cada job
// el costo de los anteriores.
type contabilidad struct {
	mu sync.Mutex
	v  llm.Uso
}

func (c *contabilidad) Uso() llm.Uso {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.v
}

// gasta suma el consumo de una corrida al acumulado, como lo hace anotaUso.
func (c *contabilidad) gasta(u llm.Uso) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.v.Llamadas += u.Llamadas
	c.v.Aciertos += u.Aciertos
	c.v.Fallos += u.Fallos
	c.v.TokensEntrada += u.TokensEntrada
	c.v.TokensSalida += u.TokensSalida
	c.v.CostoUSD += u.CostoUSD
}

// apiFalsaQueGasta es el API de fuentes con el gasto de la corrida: suma gasta
// al acumulado cuando sirve el índice, que es lo primero que pide el ejecutor
// después de fotografiar la contabilidad. Es el modelo interviniendo a mitad de
// la corrida, que es lo que la cifra tiene que separar.
func apiFalsaQueGasta(t *testing.T, muestra string, c *contabilidad, gasta llm.Uso) http.Handler {
	t.Helper()
	manejador := apiFalsa(t, muestra)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/record_index" {
			c.gasta(gasta)
		}
		manejador.ServeHTTP(w, r)
	})
}

// /run/{id} publica el consumo DE ESA CORRIDA en cost.tokens_in y
// cost.tokens_out (C23, D13): la diferencia entre la contabilidad al terminar y
// la que había al empezar, no el acumulado del proceso. Las cifras son de la
// prueba: el cliente llega a 1000/100 y luego a 1500/180, así que la segunda
// corrida reporta 500/80 y la que no gastó nada, cero.
func TestEjecutaReportaElConsumoTokensDeCadaCorrida(t *testing.T) {
	muestra := muestraDeCorpus(t)
	acumulada := &contabilidad{}
	ejecutor := ejecutorConContabilidad(t, acumulada.Uso)

	corridas := []struct {
		nombre string
		gasta  llm.Uso
		quiero llm.Uso
	}{
		{
			"primera corrida",
			llm.Uso{Llamadas: 12, TokensEntrada: 1000, TokensSalida: 100, CostoUSD: 0.5},
			llm.Uso{Llamadas: 12, TokensEntrada: 1000, TokensSalida: 100, CostoUSD: 0.5},
		},
		{
			"segunda corrida",
			llm.Uso{Llamadas: 6, Aciertos: 4, TokensEntrada: 500, TokensSalida: 80, CostoUSD: 0.25},
			llm.Uso{Llamadas: 6, Aciertos: 4, TokensEntrada: 500, TokensSalida: 80, CostoUSD: 0.25},
		},
		{
			"corrida que sirvió todo de caché",
			llm.Uso{},
			llm.Uso{},
		},
	}
	for _, corrida := range corridas {
		t.Run(corrida.nombre, func(t *testing.T) {
			srv := httptest.NewServer(apiFalsaQueGasta(t, muestra, acumulada, corrida.gasta))
			defer srv.Close()
			conteos, err := ejecutor.Ejecuta(context.Background(), srv.URL+"/v1", tokenFuentes, t.TempDir())
			if err != nil {
				t.Fatalf("Ejecuta: %v", err)
			}
			if conteos.Modelo != corrida.quiero {
				t.Fatalf("consumo = %+v, quiero %+v (el acumulado del cliente es %+v)",
					conteos.Modelo, corrida.quiero, acumulada.Uso())
			}
		})
	}
	if acumulado := acumulada.Uso(); acumulado.TokensEntrada != 1500 || acumulado.TokensSalida != 180 {
		t.Fatalf("acumulado = %+v, quiero 1500/180: dos corridas de 1000/100 y 500/80", acumulado)
	}
}

// Un índice que declara ids que la salida no trae falla la corrida: una fuente
// ingerida a medias no puede pasar por corrida buena (D11).
func TestEjecutaFallaSiLaSalidaNoCubreElIndice(t *testing.T) {
	muestra := muestraDeCorpus(t)
	manejador := apiFalsa(t, muestra)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/record_index" {
			manejador.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+tokenFuentes {
			http.Error(w, "sin token", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string][]string{
			"chats":  {"0d1530e9-no-existe", "201e6a47-tampoco"},
			"docs":   {},
			"tables": {},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	_, err := ejecutorDePrueba(t).Ejecuta(context.Background(), srv.URL+"/v1", tokenFuentes, dir)
	if err == nil {
		t.Fatal("la corrida pasó con un índice que la salida no cubre")
	}
	if !strings.Contains(err.Error(), "record_index") {
		t.Fatalf("error = %v, quiero que nombre record_index", err)
	}
}

// Sin token el API falso responde 401 y la corrida falla antes de ingerir nada.
func TestEjecutaFallaSinToken(t *testing.T) {
	srv := httptest.NewServer(apiFalsa(t, muestraDeCorpus(t)))
	defer srv.Close()

	dir := t.TempDir()
	_, err := ejecutorDePrueba(t).Ejecuta(context.Background(), srv.URL+"/v1", "otro-token", dir)
	if err == nil {
		t.Fatal("la corrida pasó sin token válido")
	}
	if _, err := os.Stat(filepath.Join(dir, "chats.jsonl")); err == nil {
		t.Fatal("escribió la salida de una corrida que falló al descargar")
	}
}

// leeChatsDeSalida decodifica chats.jsonl como lo hace el jurado.
func leeChatsDeSalida(t *testing.T, dir string) []modelo.ChatRecord {
	t.Helper()
	archivo, err := os.Open(filepath.Join(dir, "chats.jsonl"))
	if err != nil {
		t.Fatalf("chats.jsonl: %v", err)
	}
	defer archivo.Close()
	var registros []modelo.ChatRecord
	decodificador := json.NewDecoder(archivo)
	for decodificador.More() {
		var r modelo.ChatRecord
		if err := decodificador.Decode(&r); err != nil {
			t.Fatalf("chats.jsonl: %v", err)
		}
		registros = append(registros, r)
	}
	return registros
}

// formasDelCorpus junta las formas reales del registro de personas.
func formasDelCorpus(t *testing.T, entrada nucleo.Entrada) []string {
	t.Helper()
	indice, err := registro.Construye(entrada.Registros, registro.Opciones{
		Usuarios: entrada.Usuarios, Autores: entrada.Autores, Canales: entrada.Canales,
	})
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	formas := []string{}
	for _, f := range indice.Formas() {
		formas = append(formas, f.Valor)
	}
	return formas
}
