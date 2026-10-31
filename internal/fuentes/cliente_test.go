package fuentes

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const tokenDePrueba = "token-de-prueba"

// nuevoClienteDePrueba levanta un servidor en loopback y devuelve un cliente
// apuntado a su /v1. Ninguna prueba de este paquete sale a la red.
func nuevoClienteDePrueba(t *testing.T, manejador http.Handler, opc Opciones) *Cliente {
	t.Helper()
	srv := httptest.NewServer(manejador)
	t.Cleanup(srv.Close)
	opc.Base = srv.URL + "/v1"
	if opc.Token == "" {
		opc.Token = tokenDePrueba
	}
	if opc.Timeout == 0 {
		opc.Timeout = 5 * time.Second
	}
	c, err := Nuevo(opc)
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	return c
}

// relojFalso mueve el tiempo en memoria: las pruebas del límite y del backoff
// terminan en milisegundos en vez de dormir un minuto de verdad.
type relojFalso struct {
	mu       sync.Mutex
	t        time.Time
	dormidas []time.Duration
}

func nuevoRelojFalso() *relojFalso {
	return &relojFalso{t: time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)}
}

func (r *relojFalso) ahora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.t
}

func (r *relojFalso) dormir(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dormidas = append(r.dormidas, d)
	if d > 0 {
		r.t = r.t.Add(d)
	}
	return nil
}

// esperas son las esperas pedidas al reloj, en orden.
func (r *relojFalso) esperas() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.dormidas...)
}

// inyecta el reloj falso en el cliente.
func (r *relojFalso) inyecta(c *Cliente) *relojFalso {
	c.reloj = reloj{ahora: r.ahora, dormir: r.dormir}
	return r
}

func TestObtenerMandaBearerYDevuelveBytesCrudos(t *testing.T) {
	latin1 := []byte("cliente_id;nombre_completo\nCL-1;Dora Mu\xf1oz\n")
	var autorizacion string
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		autorizacion = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/csv; charset=iso-8859-1")
		w.Header().Set("ETag", `"tabla-1"`)
		w.Write(latin1)
	}), Opciones{})

	resp, err := c.Tabla(context.Background(), "clientes.csv")
	if err != nil {
		t.Fatalf("Tabla: %v", err)
	}
	if autorizacion != "Bearer "+tokenDePrueba {
		t.Fatalf("Authorization = %q", autorizacion)
	}
	if !bytes.Equal(resp.Cuerpo, latin1) {
		t.Fatalf("el cuerpo cambió: %q", resp.Cuerpo)
	}
	if resp.ContentType != "text/csv; charset=iso-8859-1" {
		t.Fatalf("Content-Type = %q", resp.ContentType)
	}
	if resp.ETag != `"tabla-1"` {
		t.Fatalf("ETag = %q", resp.ETag)
	}
	if resp.SinCambios {
		t.Fatal("un 200 no es SinCambios")
	}
}

func TestObtenerMandaIfNoneMatchY304SinCambios(t *testing.T) {
	var vistos []string
	pedidos := 0
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos++
		vistos = append(vistos, r.Header.Get("If-None-Match"))
		if r.Header.Get("If-None-Match") == `"indice-1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"indice-1"`)
		io.WriteString(w, `{"chats":["c1"],"docs":["d1"],"tables":["t1"]}`)
	}), Opciones{})

	primera, err := c.IndiceRegistros(context.Background())
	if err != nil {
		t.Fatalf("primera: %v", err)
	}
	if primera.SinCambios || !strings.Contains(string(primera.Cuerpo), `"c1"`) {
		t.Fatalf("primera respuesta rara: %+v", primera)
	}
	if primera.ETag != `"indice-1"` {
		t.Fatalf("ETag de la primera = %q", primera.ETag)
	}

	segunda, err := c.IndiceRegistros(context.Background())
	if err != nil {
		t.Fatalf("segunda: %v", err)
	}
	if !segunda.SinCambios {
		t.Fatal("un 304 debe llegar como SinCambios")
	}
	if len(segunda.Cuerpo) != 0 {
		t.Fatalf("un 304 no trae cuerpo: %q", segunda.Cuerpo)
	}
	if segunda.ETag != `"indice-1"` {
		t.Fatalf("el 304 conserva el ETag conocido: %q", segunda.ETag)
	}
	if len(vistos) != 2 || vistos[0] != "" || vistos[1] != `"indice-1"` {
		t.Fatalf("If-None-Match = %q", vistos)
	}
	if pedidos != 2 {
		t.Fatalf("pedidos = %d", pedidos)
	}
}

// Cada ruta lleva su propio ETag: la caché es por ruta, no global.
func TestETagEsPorRuta(t *testing.T) {
	var vistos []string
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vistos = append(vistos, r.URL.Path+" "+r.Header.Get("If-None-Match"))
		w.Header().Set("ETag", `"`+r.URL.Path+`"`)
		io.WriteString(w, "[]")
	}), Opciones{})

	ctx := context.Background()
	if _, err := c.Documentos(ctx); err != nil {
		t.Fatalf("docs: %v", err)
	}
	if _, err := c.Tablas(ctx); err != nil {
		t.Fatalf("tables: %v", err)
	}
	if len(vistos) != 2 || vistos[1] != "/v1/tables " {
		t.Fatalf("una ruta nueva no debe mandar If-None-Match: %q", vistos)
	}
}

func TestPaginacionDeChatsPorCanalYFecha(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/chats/channels", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id":"CH1","name":"soporte-cobros","members":["U1"]}]`)
	})
	mux.HandleFunc("GET /v1/chats/users", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id":"U1","name":"agente"}]`)
	})
	mux.HandleFunc("GET /v1/chats/{canal}", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `["2026-09-14","2026-09-15"]`)
	})
	mux.HandleFunc("GET /v1/chats/{canal}/{fecha}", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"client_msg_id":"c1","user":"U1","text":"hola","ts":"1.1","type":"message"}]`)
	})
	c := nuevoClienteDePrueba(t, mux, Opciones{})
	ctx := context.Background()

	canales, err := c.Canales(ctx)
	if err != nil {
		t.Fatalf("Canales: %v", err)
	}
	if !strings.Contains(string(canales.Cuerpo), "soporte-cobros") {
		t.Fatalf("canales = %s", canales.Cuerpo)
	}
	usuarios, err := c.Usuarios(ctx)
	if err != nil {
		t.Fatalf("Usuarios: %v", err)
	}
	if !strings.Contains(string(usuarios.Cuerpo), `"U1"`) {
		t.Fatalf("usuarios = %s", usuarios.Cuerpo)
	}
	fechas, err := c.Fechas(ctx, "soporte-cobros")
	if err != nil {
		t.Fatalf("Fechas: %v", err)
	}
	if !strings.Contains(string(fechas.Cuerpo), "2026-09-15") {
		t.Fatalf("fechas = %s", fechas.Cuerpo)
	}
	mensajes, err := c.Mensajes(ctx, "soporte-cobros", "2026-09-14")
	if err != nil {
		t.Fatalf("Mensajes: %v", err)
	}
	if !strings.Contains(string(mensajes.Cuerpo), `"client_msg_id":"c1"`) {
		t.Fatalf("mensajes = %s", mensajes.Cuerpo)
	}
}

func TestRutasDelContrato(t *testing.T) {
	var pedidas []string
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidas = append(pedidas, r.RequestURI)
		io.WriteString(w, "[]")
	}), Opciones{})
	ctx := context.Background()

	casos := []struct {
		ruta string
		pide func() error
	}{
		{"/v1/chats/channels", func() error { _, err := c.Canales(ctx); return err }},
		{"/v1/chats/users", func() error { _, err := c.Usuarios(ctx); return err }},
		{"/v1/chats/soporte-cobros", func() error { _, err := c.Fechas(ctx, "soporte-cobros"); return err }},
		{"/v1/chats/soporte-cobros/2026-09-14", func() error { _, err := c.Mensajes(ctx, "soporte-cobros", "2026-09-14"); return err }},
		{"/v1/docs", func() error { _, err := c.Documentos(ctx); return err }},
		{"/v1/docs/doc_02", func() error { _, err := c.Documento(ctx, "doc_02"); return err }},
		{"/v1/tables", func() error { _, err := c.Tablas(ctx); return err }},
		{"/v1/tables/clientes.csv", func() error { _, err := c.Tabla(ctx, "clientes.csv"); return err }},
		{"/v1/record_index", func() error { _, err := c.IndiceRegistros(ctx); return err }},
		{"/v1/tests/public", func() error { _, err := c.TestsPublicos(ctx); return err }},
	}
	for _, caso := range casos {
		if err := caso.pide(); err != nil {
			t.Fatalf("%s: %v", caso.ruta, err)
		}
	}
	if len(pedidas) != len(casos) {
		t.Fatalf("pedidas = %q", pedidas)
	}
	for i, caso := range casos {
		if pedidas[i] != caso.ruta {
			t.Fatalf("petición %d = %q, se esperaba %q", i, pedidas[i], caso.ruta)
		}
	}
}

// Los segmentos que llegan del corpus se escapan: un canal con espacio no puede
// partir la ruta.
func TestRutasEscapanSegmentos(t *testing.T) {
	var pedidas []string
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidas = append(pedidas, r.RequestURI)
		io.WriteString(w, "[]")
	}), Opciones{})
	ctx := context.Background()

	if _, err := c.Fechas(ctx, "soporte cobros"); err != nil {
		t.Fatalf("Fechas: %v", err)
	}
	if _, err := c.Documento(ctx, "doc 02"); err != nil {
		t.Fatalf("Documento: %v", err)
	}
	quiere := []string{"/v1/chats/soporte%20cobros", "/v1/docs/doc%2002"}
	for i := range quiere {
		if pedidas[i] != quiere[i] {
			t.Fatalf("petición %d = %q, se esperaba %q", i, pedidas[i], quiere[i])
		}
	}
}

func TestReintenta429Y5xxConBackoff(t *testing.T) {
	pedidos := 0
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos++
		switch pedidos {
		case 1:
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			io.WriteString(w, `["2026-09-14"]`)
		}
	}), Opciones{EsperaBase: 10 * time.Millisecond})
	reloj := nuevoRelojFalso().inyecta(c)

	resp, err := c.Fechas(context.Background(), "soporte-cobros")
	if err != nil {
		t.Fatalf("Fechas: %v", err)
	}
	if !strings.Contains(string(resp.Cuerpo), "2026-09-14") {
		t.Fatalf("cuerpo = %s", resp.Cuerpo)
	}
	if pedidos != 3 {
		t.Fatalf("pedidos = %d, se esperaban 3 intentos", pedidos)
	}
	esperas := reloj.esperas()
	quiere := []time.Duration{2 * time.Second, 20 * time.Millisecond}
	if len(esperas) != len(quiere) {
		t.Fatalf("esperas = %v", esperas)
	}
	for i := range quiere {
		if esperas[i] != quiere[i] {
			t.Fatalf("espera %d = %v, se esperaba %v", i, esperas[i], quiere[i])
		}
	}
}

func TestNoReintenta404(t *testing.T) {
	pedidos := 0
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos++
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":"no existe"}`)
	}), Opciones{EsperaBase: time.Millisecond})

	_, err := c.Documento(context.Background(), "doc_99")
	var rechazo *ErrorHTTP
	if !errors.As(err, &rechazo) {
		t.Fatalf("err = %v, se esperaba *ErrorHTTP", err)
	}
	if rechazo.Codigo != http.StatusNotFound {
		t.Fatalf("código = %d", rechazo.Codigo)
	}
	if rechazo.Ruta != "/docs/doc_99" {
		t.Fatalf("ruta = %q", rechazo.Ruta)
	}
	if pedidos != 1 {
		t.Fatalf("pedidos = %d: un 404 no se reintenta", pedidos)
	}
}

func TestSeRindeTrasLosIntentos(t *testing.T) {
	pedidos := 0
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos++
		w.WriteHeader(http.StatusServiceUnavailable)
	}), Opciones{Intentos: 3, EsperaBase: time.Millisecond})

	_, err := c.IndiceRegistros(context.Background())
	var rechazo *ErrorHTTP
	if !errors.As(err, &rechazo) || rechazo.Codigo != http.StatusServiceUnavailable {
		t.Fatalf("err = %v", err)
	}
	if pedidos != 3 {
		t.Fatalf("pedidos = %d, se esperaban 3", pedidos)
	}
}

func TestTimeoutPorPeticion(t *testing.T) {
	suelta := make(chan struct{})
	t.Cleanup(func() { close(suelta) })
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-suelta:
		case <-r.Context().Done():
		}
	}), Opciones{Timeout: 50 * time.Millisecond})

	inicio := time.Now()
	_, err := c.Canales(context.Background())
	if err == nil {
		t.Fatal("se esperaba timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !os.IsTimeout(err) {
		t.Fatalf("err = %v, se esperaba un timeout", err)
	}
	if transcurrido := time.Since(inicio); transcurrido > 3*time.Second {
		t.Fatalf("tardó %v: el timeout por petición no se aplicó", transcurrido)
	}
}

func TestContextoCanceladoNoPideNada(t *testing.T) {
	var pedidos atomic.Int32
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		io.WriteString(w, "[]")
	}), Opciones{})

	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	if _, err := c.Canales(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, se esperaba context.Canceled", err)
	}
	if pedidos.Load() != 0 {
		t.Fatalf("salió a la red con el contexto cancelado: %d pedidos", pedidos.Load())
	}
}

// El cupo es una ventana deslizante de un minuto: con cupo 2, la tercera
// petición espera a que la primera salga de la ventana.
func TestRespetaElLimiteSinRafagas(t *testing.T) {
	var pedidos atomic.Int32
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		io.WriteString(w, "[]")
	}), Opciones{PeticionesPorMinuto: 2})
	reloj := nuevoRelojFalso().inyecta(c)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := c.Canales(ctx); err != nil {
			t.Fatalf("Canales %d: %v", i, err)
		}
	}
	if pedidos.Load() != 5 {
		t.Fatalf("pedidos = %d", pedidos.Load())
	}
	esperas := reloj.esperas()
	if len(esperas) != 2 {
		t.Fatalf("esperas = %v, se esperaban dos ventanas", esperas)
	}
	for i, espera := range esperas {
		if espera != time.Minute {
			t.Fatalf("espera %d = %v, se esperaba un minuto", i, espera)
		}
	}
}

func TestValoresPorDefecto(t *testing.T) {
	c, err := Nuevo(Opciones{Base: "https://api.example/v1/", Token: "t"})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	if c.base != "https://api.example/v1" {
		t.Fatalf("base = %q", c.base)
	}
	if c.cupo != 600 {
		t.Fatalf("cupo = %d, el API permite 600 por minuto", c.cupo)
	}
	if c.intentos != 4 {
		t.Fatalf("intentos = %d", c.intentos)
	}
	if c.esperaBase != 500*time.Millisecond {
		t.Fatalf("esperaBase = %v", c.esperaBase)
	}
	if c.http.Timeout != 30*time.Second {
		t.Fatalf("timeout = %v", c.http.Timeout)
	}
}

func TestNuevoRechazaOpcionesInvalidas(t *testing.T) {
	casos := []Opciones{
		{Token: "t"},
		{Base: "ftp://api.example/v1", Token: "t"},
		{Base: "/v1", Token: "t"},
		{Base: "https://api.example/v1"},
		{Base: "https://api.example/v1", Token: "t", PeticionesPorMinuto: -1},
		{Base: "https://api.example/v1", Token: "t", Intentos: -1},
	}
	for _, caso := range casos {
		if _, err := Nuevo(caso); err == nil {
			t.Fatalf("Nuevo(%+v) no falló", caso)
		}
	}
}

func TestObtenerRechazaRutasRelativas(t *testing.T) {
	c, err := Nuevo(Opciones{Base: "https://api.example/v1", Token: "t"})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	for _, ruta := range []string{"chats/users", "", "//otro.example/x"} {
		if _, err := c.Obtener(context.Background(), ruta); err == nil {
			t.Fatalf("Obtener(%q) no falló", ruta)
		}
	}
}

// Varias goroutines comparten cliente y caché de ETag (-race lo vigila).
func TestPeticionesConcurrentes(t *testing.T) {
	var pedidos atomic.Int32
	c := nuevoClienteDePrueba(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		w.Header().Set("ETag", `"`+r.URL.Path+`"`)
		io.WriteString(w, "[]")
	}), Opciones{})

	var grupo sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		grupo.Add(1)
		go func(i int) {
			defer grupo.Done()
			_, err := c.Documento(context.Background(), strings.Repeat("doc", i+1))
			errs <- err
		}(i)
	}
	grupo.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("petición concurrente: %v", err)
		}
	}
	if pedidos.Load() != 8 {
		t.Fatalf("pedidos = %d", pedidos.Load())
	}
}
