// Package fuentes es el cliente del API de fuentes del reto.
//
// Una sola raíz (/v1 en Opciones.Base), Authorization: Bearer, 600 peticiones
// por minuto por token, timeout y reintentos con backoff ante 429 y 5xx, y ETag
// con If-None-Match: un 304 llega como Respuesta.SinCambios, o sea que la copia
// anterior sigue vigente.
//
// Todo se descarga completo en memoria y en bytes crudos. El charset declarado
// (Latin-1 en clientes.csv, UTF-8 en el resto) lo interpreta la ingesta: este
// cliente no lo toca, porque recodificar rompería la salida byte a byte.
package fuentes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// limitePorMinuto es el cupo que publica el API: 600 peticiones por minuto
	// por token.
	limitePorMinuto = 600
	// ventanaLimite es la ventana deslizante del cupo.
	ventanaLimite = time.Minute

	timeoutPorDefecto    = 30 * time.Second
	intentosPorDefecto   = 4
	esperaBasePorDefecto = 500 * time.Millisecond
	// esperaMaxima acota el backoff; Retry-After del servidor puede superarlo.
	esperaMaxima = 8 * time.Second
	// cuerpoEnError recorta lo que se guarda de una respuesta fallida para no
	// arrastrar cuerpos grandes ni datos reales a los logs.
	cuerpoEnError = 256
)

// Opciones configura el cliente. Los valores en cero toman el del API.
type Opciones struct {
	// Base es la raíz del API con su /v1: https://host/v1.
	Base string
	// Token viaja en Authorization: Bearer.
	Token string
	// Timeout es el tope de cada petición, no del total con reintentos.
	Timeout time.Duration
	// Intentos es el total de intentos ante 429 o 5xx; 1 desactiva los
	// reintentos.
	Intentos int
	// EsperaBase es el primer backoff; cada reintento lo duplica.
	EsperaBase time.Duration
	// PeticionesPorMinuto es el cupo de la ventana deslizante.
	PeticionesPorMinuto int
}

// Respuesta es una descarga completa del API.
type Respuesta struct {
	// Cuerpo son los bytes tal como llegaron, sin recodificar.
	Cuerpo []byte
	// ETag identifica la versión; se reenvía como If-None-Match.
	ETag string
	// ContentType es el Content-Type literal de la respuesta.
	ContentType string
	// SinCambios es un 304: no hay cuerpo nuevo y la copia anterior sigue
	// vigente.
	SinCambios bool
}

// ErrorHTTP es una respuesta que no fue 2xx ni 304 después de los reintentos.
type ErrorHTTP struct {
	Metodo string
	Ruta   string
	Codigo int
	// Cuerpo es el inicio de la respuesta de error, recortado.
	Cuerpo []byte
}

func (e *ErrorHTTP) Error() string {
	detalle := strings.TrimSpace(string(e.Cuerpo))
	if detalle == "" {
		return fmt.Sprintf("el API de fuentes respondió %d a %s %s", e.Codigo, e.Metodo, e.Ruta)
	}
	return fmt.Sprintf("el API de fuentes respondió %d a %s %s: %s", e.Codigo, e.Metodo, e.Ruta, detalle)
}

// Cliente del API de fuentes. Es seguro para usarse desde varias goroutines.
type Cliente struct {
	base       string
	token      string
	http       *http.Client
	intentos   int
	esperaBase time.Duration
	cupo       int
	reloj      reloj

	mu     sync.Mutex
	etags  map[string]string
	marcas []time.Time
}

// reloj inyecta el paso del tiempo: las pruebas no duermen de verdad.
type reloj struct {
	ahora  func() time.Time
	dormir func(context.Context, time.Duration) error
}

// Nuevo valida las opciones y arma el cliente. No hace ninguna petición.
func Nuevo(o Opciones) (*Cliente, error) {
	base := strings.TrimRight(o.Base, "/")
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("base del API de fuentes inválida %q: %w", o.Base, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("base del API de fuentes inválida %q: hace falta http(s)://host/v1", o.Base)
	}
	if o.Token == "" {
		return nil, errors.New("falta el token del API de fuentes")
	}
	if o.PeticionesPorMinuto < 0 {
		return nil, errors.New("el cupo de peticiones por minuto no puede ser negativo")
	}
	if o.Intentos < 0 {
		return nil, errors.New("el número de intentos no puede ser negativo")
	}

	timeout := o.Timeout
	if timeout <= 0 {
		timeout = timeoutPorDefecto
	}
	intentos := o.Intentos
	if intentos == 0 {
		intentos = intentosPorDefecto
	}
	espera := o.EsperaBase
	if espera <= 0 {
		espera = esperaBasePorDefecto
	}
	cupo := o.PeticionesPorMinuto
	if cupo == 0 {
		cupo = limitePorMinuto
	}

	return &Cliente{
		base:       base,
		token:      o.Token,
		http:       &http.Client{Timeout: timeout},
		intentos:   intentos,
		esperaBase: espera,
		cupo:       cupo,
		reloj:      reloj{ahora: time.Now, dormir: dormirContexto},
		etags:      map[string]string{},
	}, nil
}

// Obtener pide una ruta del API y devuelve los bytes crudos. Si ya se conoce el
// ETag de esa ruta lo manda como If-None-Match y un 304 llega como SinCambios.
func (c *Cliente) Obtener(ctx context.Context, ruta string) (Respuesta, error) {
	if !strings.HasPrefix(ruta, "/") || strings.HasPrefix(ruta, "//") {
		return Respuesta{}, fmt.Errorf("ruta del API de fuentes inválida %q: debe empezar con /", ruta)
	}
	etag := c.etagDe(ruta)
	for intento := 1; ; intento++ {
		if err := c.esperaTurno(ctx); err != nil {
			return Respuesta{}, err
		}
		resp, esperaServidor, err := c.pide(ctx, ruta, etag)
		if err == nil {
			return resp, nil
		}
		var rechazo *ErrorHTTP
		if !errors.As(err, &rechazo) || !reintentable(rechazo.Codigo) || intento >= c.intentos {
			return Respuesta{}, err
		}
		if err := c.reloj.dormir(ctx, c.esperaDe(intento, esperaServidor)); err != nil {
			return Respuesta{}, err
		}
	}
}

// Canales trae /chats/channels.
func (c *Cliente) Canales(ctx context.Context) (Respuesta, error) {
	return c.Obtener(ctx, "/chats/channels")
}

// Usuarios trae /chats/users.
func (c *Cliente) Usuarios(ctx context.Context) (Respuesta, error) {
	return c.Obtener(ctx, "/chats/users")
}

// Fechas trae las fechas con mensajes de un canal.
func (c *Cliente) Fechas(ctx context.Context, canal string) (Respuesta, error) {
	return c.Obtener(ctx, "/chats/"+url.PathEscape(canal))
}

// Mensajes trae los mensajes de un canal en una fecha.
func (c *Cliente) Mensajes(ctx context.Context, canal, fecha string) (Respuesta, error) {
	return c.Obtener(ctx, "/chats/"+url.PathEscape(canal)+"/"+url.PathEscape(fecha))
}

// Documentos trae el índice de documentos (/docs).
func (c *Cliente) Documentos(ctx context.Context) (Respuesta, error) {
	return c.Obtener(ctx, "/docs")
}

// Documento trae el archivo original de un documento, PDF incluido.
func (c *Cliente) Documento(ctx context.Context, id string) (Respuesta, error) {
	return c.Obtener(ctx, "/docs/"+url.PathEscape(id))
}

// Tablas trae la lista de tablas con su codificación.
func (c *Cliente) Tablas(ctx context.Context) (Respuesta, error) {
	return c.Obtener(ctx, "/tables")
}

// Tabla trae los bytes originales de una tabla, con su charset declarado.
func (c *Cliente) Tabla(ctx context.Context, nombre string) (Respuesta, error) {
	return c.Obtener(ctx, "/tables/"+url.PathEscape(nombre))
}

// IndiceRegistros trae /record_index: todos los ids que la salida debe contener.
func (c *Cliente) IndiceRegistros(ctx context.Context) (Respuesta, error) {
	return c.Obtener(ctx, "/record_index")
}

// TestsPublicos trae los cinco tests públicos del reto.
func (c *Cliente) TestsPublicos(ctx context.Context) (Respuesta, error) {
	return c.Obtener(ctx, "/tests/public")
}

// pide hace un solo intento y devuelve también lo que pidió el servidor en
// Retry-After, si lo pidió.
func (c *Cliente) pide(ctx context.Context, ruta, etag string) (Respuesta, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+ruta, nil)
	if err != nil {
		return Respuesta{}, 0, fmt.Errorf("arma la petición a %s: %w", ruta, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Respuesta{}, 0, err
	}
	defer resp.Body.Close()
	cuerpo, err := io.ReadAll(resp.Body)
	if err != nil {
		return Respuesta{}, 0, fmt.Errorf("descarga %s: %w", ruta, err)
	}

	tipo := resp.Header.Get("Content-Type")
	switch {
	case resp.StatusCode == http.StatusNotModified:
		return Respuesta{ETag: etag, ContentType: tipo, SinCambios: true}, 0, nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		nuevo := resp.Header.Get("ETag")
		c.guardaETag(ruta, nuevo)
		return Respuesta{Cuerpo: cuerpo, ETag: nuevo, ContentType: tipo}, 0, nil
	default:
		if len(cuerpo) > cuerpoEnError {
			cuerpo = cuerpo[:cuerpoEnError]
		}
		rechazo := &ErrorHTTP{Metodo: http.MethodGet, Ruta: ruta, Codigo: resp.StatusCode, Cuerpo: cuerpo}
		return Respuesta{}, esperaDeCabecera(resp.Header.Get("Retry-After")), rechazo
	}
}

// etagDe devuelve el ETag conocido de una ruta, o "".
func (c *Cliente) etagDe(ruta string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.etags[ruta]
}

// guardaETag recuerda el ETag de la última copia buena de una ruta.
func (c *Cliente) guardaETag(ruta, etag string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if etag == "" {
		delete(c.etags, ruta)
		return
	}
	c.etags[ruta] = etag
}

// reintentable dice si vale la pena repetir la petición: el API pidió bajar el
// ritmo o se cayó del lado del servidor.
func reintentable(codigo int) bool {
	return codigo == http.StatusTooManyRequests || codigo >= 500
}

// esperaDe calcula el backoff del intento que falló: EsperaBase duplicada por
// intento, con techo, o lo que pidió el servidor si es más largo.
func (c *Cliente) esperaDe(intento int, delServidor time.Duration) time.Duration {
	espera := c.esperaBase
	for i := 1; i < intento && espera < esperaMaxima; i++ {
		espera *= 2
	}
	if espera > esperaMaxima {
		espera = esperaMaxima
	}
	if delServidor > espera {
		return delServidor
	}
	return espera
}

// esperaDeCabecera interpreta Retry-After, en segundos o como fecha HTTP.
func esperaDeCabecera(valor string) time.Duration {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return 0
	}
	if segundos, err := strconv.Atoi(valor); err == nil {
		if segundos <= 0 {
			return 0
		}
		return time.Duration(segundos) * time.Second
	}
	if fecha, err := http.ParseTime(valor); err == nil {
		if espera := time.Until(fecha); espera > 0 {
			return espera
		}
	}
	return 0
}

// dormirContexto espera lo pedido o hasta que el contexto se cancele.
func dormirContexto(ctx context.Context, espera time.Duration) error {
	if espera <= 0 {
		return ctx.Err()
	}
	temporizador := time.NewTimer(espera)
	defer temporizador.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-temporizador.C:
		return nil
	}
}
