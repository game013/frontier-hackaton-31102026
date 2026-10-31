package main

// Pruebas del monitor contra un API de fuentes simulado. Se ejecuta el binario
// de verdad (go build + exec): el contrato del comando es su salida y su código
// de salida, no sus funciones internas.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// tokenDePrueba es inventado y no debe aparecer nunca en la salida.
	tokenDePrueba = "token-inventado-de-prueba"
	// plazoDelComando acota cada ejecución: nada de pruebas colgadas.
	plazoDelComando = 30 * time.Second
)

// binario es el monitor compilado una sola vez para todas las pruebas.
var binario string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "monitor-bin")
	if err != nil {
		panic("prepara el temporal del binario: " + err.Error())
	}
	binario = filepath.Join(dir, "monitor")
	compila := exec.Command("go", "build", "-o", binario, ".")
	if salida, err := compila.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("compila cmd/monitor: " + err.Error() + "\n" + string(salida))
	}
	codigo := m.Run()
	os.RemoveAll(dir)
	os.Exit(codigo)
}

// pedido es lo que el monitor mandó al API simulado: así se comprueba el
// If-None-Match sin mirar dentro del comando.
type pedido struct {
	Ruta         string
	Autorizacion string
	SiNoCoincide string
}

// apiSimulada sirve /v1/record_index con el ETag y el índice vigentes y anota
// cada petición recibida.
type apiSimulada struct {
	mu      sync.Mutex
	etag    string
	chats   []string
	docs    []string
	tablas  []string
	codigo  int // distinto de cero: responde ese error HTTP
	pedidos []pedido
}

func (a *apiSimulada) sirve(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.pedidos = append(a.pedidos, pedido{
		Ruta:         r.URL.Path,
		Autorizacion: r.Header.Get("Authorization"),
		SiNoCoincide: r.Header.Get("If-None-Match"),
	})
	etag, codigo := a.etag, a.codigo
	chats, docs, tablas := a.chats, a.docs, a.tablas
	a.mu.Unlock()

	if r.URL.Path != "/v1/record_index" {
		http.NotFound(w, r)
		return
	}
	if codigo != 0 {
		http.Error(w, "falla simulada del API", codigo)
		return
	}
	if siNoCoincide := r.Header.Get("If-None-Match"); siNoCoincide != "" && siNoCoincide == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	cuerpo, err := json.Marshal(map[string][]string{"chats": chats, "docs": docs, "tables": tablas})
	if err != nil {
		http.Error(w, "índice ilegible", http.StatusInternalServerError)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	w.Write(cuerpo)
}

// crece añade un registro al índice y publica un ETag nuevo, como el API real
// cuando el corpus crece.
func (a *apiSimulada) crece(fuente, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch fuente {
	case "chats":
		a.chats = append(a.chats, id)
	case "docs":
		a.docs = append(a.docs, id)
	case "tables":
		a.tablas = append(a.tablas, id)
	}
	a.etag = etagDe(a.chats, a.docs, a.tablas)
}

func (a *apiSimulada) ultimoPedido(t *testing.T) pedido {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.pedidos) == 0 {
		t.Fatal("el monitor no llamó al API")
	}
	return a.pedidos[len(a.pedidos)-1]
}

// etagDe deriva un ETag de los conteos: es de mentira, pero cambia cuando
// cambia el índice.
func etagDe(chats, docs, tablas []string) string {
	return fmt.Sprintf("%q", fmt.Sprintf("v-%d-%d-%d", len(chats), len(docs), len(tablas)))
}

// apiDePrueba arma el servidor simulado con el índice inicial.
func apiDePrueba(t *testing.T) (*apiSimulada, *httptest.Server) {
	t.Helper()
	api := &apiSimulada{
		chats:  []string{"c1", "c2", "c3"},
		docs:   []string{"d1"},
		tablas: []string{"t1", "t2"},
	}
	api.etag = etagDe(api.chats, api.docs, api.tablas)
	servidor := httptest.NewServer(http.HandlerFunc(api.sirve))
	t.Cleanup(servidor.Close)
	return api, servidor
}

// correMonitor ejecuta el binario contra el API y devuelve su salida combinada
// y su código de salida.
func correMonitor(t *testing.T, base, datos string, extra ...string) (string, int) {
	t.Helper()
	args := append([]string{
		"--api", base + "/v1",
		"--token", tokenDePrueba,
		"--datos", datos,
		"--timeout", "5s",
	}, extra...)
	ctx, cancelar := context.WithTimeout(context.Background(), plazoDelComando)
	defer cancelar()
	cmd := exec.CommandContext(ctx, binario, args...)
	var salida bytes.Buffer
	cmd.Stdout = &salida
	cmd.Stderr = &salida
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("el monitor no terminó en %s: %v", plazoDelComando, ctx.Err())
	}
	codigo := 0
	if err != nil {
		var salidaErr *exec.ExitError
		if !errors.As(err, &salidaErr) {
			t.Fatalf("ejecuta el monitor: %v", err)
		}
		codigo = salidaErr.ExitCode()
	}
	t.Logf("monitor salió con %d:\n%s", codigo, salida.String())
	return salida.String(), codigo
}

// TestMonitorDetectaCambioSimulado es el criterio de C21: el API devuelve 200
// con ETag, luego 304, luego 200 con ETag nuevo y un chat más; el monitor
// reporta la línea base, el sin-cambios y el delta, en ese orden.
func TestMonitorDetectaCambioSimulado(t *testing.T) {
	api, servidor := apiDePrueba(t)
	datos := t.TempDir()

	t.Run("paso 1: guarda la línea base", func(t *testing.T) {
		salida, codigo := correMonitor(t, servidor.URL, datos)
		if codigo != 0 {
			t.Fatalf("código = %d, quiero 0 (primera lectura, sin referencia previa)", codigo)
		}
		if !strings.Contains(salida, "línea base") {
			t.Fatalf("salida = %q, quiero que anuncie la línea base", salida)
		}
		ultimo := api.ultimoPedido(t)
		if ultimo.Ruta != "/v1/record_index" {
			t.Fatalf("ruta = %q, quiero /v1/record_index", ultimo.Ruta)
		}
		if ultimo.SiNoCoincide != "" {
			t.Fatalf("If-None-Match = %q sin ETag guardado, quiero vacío", ultimo.SiNoCoincide)
		}
		if ultimo.Autorizacion != "Bearer "+tokenDePrueba {
			t.Fatalf("Authorization = %q, quiero el Bearer del token", ultimo.Autorizacion)
		}
		if guardado := leeArchivo(t, filepath.Join(datos, "record_index.etag")); guardado != api.etag {
			t.Fatalf("ETag guardado = %q, quiero %q", guardado, api.etag)
		}
		if indice := leeArchivo(t, filepath.Join(datos, "record_index.json")); !strings.Contains(indice, `"c3"`) {
			t.Fatalf("índice guardado = %q, quiero el del API", indice)
		}
	})

	t.Run("paso 2: 304 es sin cambios", func(t *testing.T) {
		salida, codigo := correMonitor(t, servidor.URL, datos)
		if codigo != 0 {
			t.Fatalf("código = %d, quiero 0 (sin cambios)", codigo)
		}
		if !strings.Contains(salida, "sin cambios") {
			t.Fatalf("salida = %q, quiero que diga sin cambios", salida)
		}
		if lineas := cuentaLineas(salida); lineas != 1 {
			t.Fatalf("salida = %q en %d líneas, quiero una sola", salida, lineas)
		}
		if ultimo := api.ultimoPedido(t); ultimo.SiNoCoincide != api.etag {
			t.Fatalf("If-None-Match = %q, quiero el ETag guardado %q", ultimo.SiNoCoincide, api.etag)
		}
	})

	t.Run("paso 3: el índice nuevo reporta el delta", func(t *testing.T) {
		api.crece("chats", "c4")
		salida, codigo := correMonitor(t, servidor.URL, datos)
		if codigo != 1 {
			t.Fatalf("código = %d, quiero 1 (el corpus cambió)", codigo)
		}
		for _, quiero := range []string{"el corpus cambió", "chats: 3 → 4 (+1)", "docs: 1 → 1 (igual)", "tables: 2 → 2 (igual)"} {
			if !strings.Contains(salida, quiero) {
				t.Fatalf("salida = %q, quiero que contenga %q", salida, quiero)
			}
		}
		if guardado := leeArchivo(t, filepath.Join(datos, "record_index.etag")); guardado != api.etag {
			t.Fatalf("ETag guardado = %q, quiero el nuevo %q", guardado, api.etag)
		}
		if indice := leeArchivo(t, filepath.Join(datos, "record_index.json")); !strings.Contains(indice, `"c4"`) {
			t.Fatalf("índice guardado = %q, quiero el nuevo", indice)
		}
	})

	t.Run("paso 4: el ETag nuevo ya da 304", func(t *testing.T) {
		salida, codigo := correMonitor(t, servidor.URL, datos)
		if codigo != 0 || !strings.Contains(salida, "sin cambios") {
			t.Fatalf("código = %d, salida = %q; quiero 0 y sin cambios", codigo, salida)
		}
	})
}

// TestMonitorNoFiltraElTokenNiPisaElEstadoSiElAPIFalla cubre el error HTTP: un
// 5xx sale con código 2, nombra el código, no imprime el token y no pisa el
// estado bueno que ya había.
func TestMonitorNoFiltraElTokenNiPisaElEstadoSiElAPIFalla(t *testing.T) {
	api, servidor := apiDePrueba(t)
	datos := t.TempDir()
	if _, codigo := correMonitor(t, servidor.URL, datos); codigo != 0 {
		t.Fatalf("prepara la línea base: código = %d, quiero 0", codigo)
	}
	etagAntes := leeArchivo(t, filepath.Join(datos, "record_index.etag"))

	api.mu.Lock()
	api.codigo = http.StatusInternalServerError
	api.mu.Unlock()

	salida, codigo := correMonitor(t, servidor.URL, datos)
	if codigo != 2 {
		t.Fatalf("código = %d, quiero 2 (error del API)", codigo)
	}
	if !strings.Contains(salida, "500") {
		t.Fatalf("salida = %q, quiero que nombre el 500", salida)
	}
	if strings.Contains(salida, tokenDePrueba) {
		t.Fatalf("salida = %q: el token no puede aparecer", salida)
	}
	if guardado := leeArchivo(t, filepath.Join(datos, "record_index.etag")); guardado != etagAntes {
		t.Fatalf("ETag guardado = %q, quiero el de antes %q", guardado, etagAntes)
	}
}

// TestMonitorSaleConErrorSiElAPINoResponde cubre la red caída: el servidor de
// prueba se cierra y el monitor sale con 2 en vez de colgarse.
func TestMonitorSaleConErrorSiElAPINoResponde(t *testing.T) {
	caido := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := caido.URL
	caido.Close()

	salida, codigo := correMonitor(t, base, t.TempDir())
	if codigo != 2 {
		t.Fatalf("código = %d, quiero 2 (red caída)", codigo)
	}
	if !strings.Contains(salida, "monitor:") {
		t.Fatalf("salida = %q, quiero un mensaje claro con el prefijo monitor:", salida)
	}
	if strings.Contains(salida, tokenDePrueba) {
		t.Fatalf("salida = %q: el token no puede aparecer", salida)
	}
}

// TestMonitorNoSaleSinToken comprueba que sin token el comando se detiene antes
// de salir a la red.
func TestMonitorNoSaleSinToken(t *testing.T) {
	ctx, cancelar := context.WithTimeout(context.Background(), plazoDelComando)
	defer cancelar()
	cmd := exec.CommandContext(ctx, binario, "--api", "http://127.0.0.1:1/v1", "--datos", t.TempDir())
	cmd.Env = append(os.Environ(), "TOKEN=", "SOURCES_API_URL=")
	var salida bytes.Buffer
	cmd.Stdout = &salida
	cmd.Stderr = &salida
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("el monitor no terminó en %s: %v", plazoDelComando, ctx.Err())
	}
	if err == nil {
		t.Fatalf("salió con 0 sin token: %q", salida.String())
	}
	if !strings.Contains(strings.ToLower(salida.String()), "token") {
		t.Fatalf("salida = %q, quiero que pida el token", salida.String())
	}
}

// leeArchivo devuelve el contenido de una ruta sin espacios sobrantes.
func leeArchivo(t *testing.T, ruta string) string {
	t.Helper()
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("lee %s: %v", ruta, err)
	}
	return strings.TrimSpace(string(crudo))
}

// cuentaLineas cuenta las líneas no vacías de un texto.
func cuentaLineas(texto string) int {
	n := 0
	for _, linea := range strings.Split(texto, "\n") {
		if strings.TrimSpace(linea) != "" {
			n++
		}
	}
	return n
}
