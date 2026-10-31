package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// SnapshotJev es el snapshot fijo de Jev 1.14 (D32). Va en cada llamada y en la
// llave de caché; /v1/health lo reporta con Nombre.
const SnapshotJev = "typesafe/jev-1.14-20261014"

// TopeConcurrencia es el máximo de llamadas simultáneas al modelo (D28).
const TopeConcurrencia = 16

// Umbrales de D31: cada decisión tiene un costo de error distinto. Son
// exclusivos, se acepta con probabilidad estrictamente mayor, así el caso límite
// cae del lado seguro: la cola de dudosos (D19).
const (
	// UmbralPersona acepta que un fragmento es persona (P > 0,5).
	UmbralPersona = 0.5
	// UmbralDesempate acepta un desempate entre candidatos (0,7; el empate
	// exacto va a dudoso).
	UmbralDesempate = 0.7
	// UmbralGenero acepta un género; por debajo la persona queda unisex (0,6).
	UmbralGenero = 0.6
)

// Errores tipados del cliente. Todo paso de red puede fallar: quien llama
// distingue el caso y cae al respaldo determinista, sin leer textos de error.
var (
	// ErrSinClave falta OPENROUTER_API_KEY; sin cliente, respaldo sin LLM.
	ErrSinClave = errors.New("llm: falta la clave de OpenRouter")
	// ErrPreguntaInvalida la pregunta no cumple el contrato; no gasta llamada.
	ErrPreguntaInvalida = errors.New("llm: pregunta fuera del contrato")
	// ErrTiempoLimite se agotó el plazo del intento o del contexto.
	ErrTiempoLimite = errors.New("llm: se agotó el tiempo límite")
	// ErrRespuestaInvalida el cuerpo no es el JSON esperado.
	ErrRespuestaInvalida = errors.New("llm: respuesta inválida")
	// ErrHTTP el proveedor respondió con un estado de error.
	ErrHTTP = errors.New("llm: el proveedor respondió con error")
	// ErrBajaConfianza la probabilidad de la opción elegida no supera el
	// umbral de D31; el pipeline manda la mención a dudosos (D19).
	ErrBajaConfianza = errors.New("llm: confianza por debajo del umbral")
)

// ErrorHTTP lleva el estado y el mensaje del proveedor, nunca el state enviado.
type ErrorHTTP struct {
	Estado  int
	Mensaje string
}

func (e *ErrorHTTP) Error() string {
	if e.Mensaje == "" {
		return fmt.Sprintf("llm: el proveedor respondió %d", e.Estado)
	}
	return fmt.Sprintf("llm: el proveedor respondió %d: %s", e.Estado, e.Mensaje)
}

// Unwrap permite errors.Is(err, ErrHTTP).
func (e *ErrorHTTP) Unwrap() error { return ErrHTTP }

// Configuracion arma el cliente. Solo ClaveAPI es obligatoria.
type Configuracion struct {
	// ClaveAPI es OPENROUTER_API_KEY.
	ClaveAPI string
	// Modelo es el snapshot; por defecto SnapshotJev. Sale de LLM_MODEL.
	Modelo string
	// URLBase es el origen del API; por defecto https://openrouter.ai.
	URLBase string
	// TiempoLimite acota cada intento; por defecto 8 s.
	TiempoLimite time.Duration
	// Intentos es el total de intentos por pregunta, reintentos incluidos; por
	// defecto 2. Solo se reintenta ante 408, 429, 5xx o fallo de red.
	Intentos int
	// Espera es la base del backoff entre intentos; por defecto 200 ms.
	Espera time.Duration
	// Concurrentes acota las llamadas simultáneas; por defecto
	// TopeConcurrencia y nunca más que eso.
	Concurrentes int
	// Cache guarda respuestas por hash de entrada; nil deja el cliente sin
	// caché. Producción pasa la tabla cache_modelo (CacheSQLite).
	Cache Cache
	// Mascara sustituye formas reales conocidas antes de enviar (D15); nil
	// envía el state tal cual, que solo es aceptable si ya viene enmascarado.
	Mascara Enmascarador
	// HTTP permite inyectar el cliente HTTP; nil usa uno con el timeout de
	// TiempoLimite.
	HTTP *http.Client
}

// Cliente implementa modelo.Modelo contra el endpoint de decisiones. Es seguro
// para uso concurrente: el semáforo y la contabilidad se comparten.
type Cliente struct {
	conf Configuracion
	http *http.Client
	sem  chan struct{}
	uso  usoAcumulado
}

// usoAcumulado protege la contabilidad con un mutex.
type usoAcumulado struct {
	mu sync.Mutex
	v  Uso
}

// Uso es la contabilidad acumulada del cliente: lo que costó y lo que se ahorró
// (D13). /run/{id} la reporta.
type Uso struct {
	// Llamadas son las peticiones HTTP enviadas al proveedor.
	Llamadas int
	// Aciertos son las respuestas servidas desde la caché, sin red.
	Aciertos int
	// Fallos son los intentos que terminaron en error.
	Fallos int
	// TokensEntrada y TokensSalida son los que reportó el proveedor.
	TokensEntrada int
	TokensSalida  int
	// CostoUSD es el costo que reportó el proveedor, en dólares.
	CostoUSD float64
}

// Menos devuelve el consumo que va de antes a u: la contabilidad del cliente es
// acumulada, así que la de una corrida es la diferencia entre la fotografía del
// principio y la del final (C23).
func (u Uso) Menos(antes Uso) Uso {
	return Uso{
		Llamadas:      u.Llamadas - antes.Llamadas,
		Aciertos:      u.Aciertos - antes.Aciertos,
		Fallos:        u.Fallos - antes.Fallos,
		TokensEntrada: u.TokensEntrada - antes.TokensEntrada,
		TokensSalida:  u.TokensSalida - antes.TokensSalida,
		CostoUSD:      u.CostoUSD - antes.CostoUSD,
	}
}

// Nueva valida la configuración y arma el cliente.
func Nueva(conf Configuracion) (*Cliente, error) {
	if strings.TrimSpace(conf.ClaveAPI) == "" {
		return nil, ErrSinClave
	}
	if conf.Modelo == "" {
		conf.Modelo = SnapshotJev
	}
	if conf.URLBase == "" {
		conf.URLBase = "https://openrouter.ai"
	}
	conf.URLBase = strings.TrimRight(conf.URLBase, "/")
	if conf.TiempoLimite <= 0 {
		conf.TiempoLimite = 8 * time.Second
	}
	if conf.Intentos <= 0 {
		conf.Intentos = 2
	}
	if conf.Espera <= 0 {
		conf.Espera = 200 * time.Millisecond
	}
	if conf.Concurrentes <= 0 || conf.Concurrentes > TopeConcurrencia {
		conf.Concurrentes = TopeConcurrencia
	}
	httpCliente := conf.HTTP
	if httpCliente == nil {
		httpCliente = &http.Client{Timeout: conf.TiempoLimite}
	}
	return &Cliente{
		conf: conf,
		http: httpCliente,
		sem:  make(chan struct{}, conf.Concurrentes),
	}, nil
}

// NuevaDesdeEntorno lee OPENROUTER_API_KEY, LLM_MODEL y, si está, LLM_URL_BASE.
func NuevaDesdeEntorno(getenv func(string) string) (*Cliente, error) {
	return Nueva(Configuracion{
		ClaveAPI: getenv("OPENROUTER_API_KEY"),
		Modelo:   getenv("LLM_MODEL"),
		URLBase:  getenv("LLM_URL_BASE"),
	})
}

// Nombre es el snapshot en uso; /v1/health lo reporta (D32).
func (c *Cliente) Nombre() string { return c.conf.Modelo }

// Tope es el máximo de llamadas simultáneas configurado (D28).
func (c *Cliente) Tope() int { return cap(c.sem) }

// Uso devuelve la contabilidad acumulada.
func (c *Cliente) Uso() Uso {
	c.uso.mu.Lock()
	defer c.uso.mu.Unlock()
	return c.uso.v
}

// Decide hace una llamada con el state dado y la aplica el umbral de persona
// (D31). Ver DecideUmbral.
func (c *Cliente) Decide(ctx context.Context, state string, pregunta modelo.Pregunta) (modelo.Respuesta, error) {
	return c.DecideUmbral(ctx, state, pregunta, UmbralPersona)
}

// DecideUmbral hace una llamada por pregunta, con el state de esa pregunta y
// sin agregar contexto de otros registros (D28). El state debería venir ya
// enmascarado; si hay Mascara configurada, se enmascara aquí antes de enviarlo
// (D15). La respuesta vuelve aunque no supere el umbral, junto con
// ErrBajaConfianza, para que la mención quede en la cola de dudosos (D19).
func (c *Cliente) DecideUmbral(ctx context.Context, state string, pregunta modelo.Pregunta, umbral float64) (modelo.Respuesta, error) {
	preguntaEnviada, err := preguntaDecisionDe(pregunta)
	if err != nil {
		return modelo.Respuesta{}, err
	}
	estado := state
	if c.conf.Mascara != nil {
		estado = c.conf.Mascara.Enmascara(state)
	}

	cuerpo := cuerpoDecision{
		Modelo:    c.conf.Modelo,
		Proveedor: proveedorPreferencias{RecoleccionDatos: "deny"},
		Estado:    estado,
		Preguntas: map[string]preguntaDecision{pregunta.ID: preguntaEnviada},
	}
	crudo, err := json.Marshal(cuerpo)
	if err != nil {
		return modelo.Respuesta{}, fmt.Errorf("%w: %v", ErrPreguntaInvalida, err)
	}

	llave, err := llaveDeCache(c.conf.Modelo, estado, cuerpo.Preguntas)
	if err != nil {
		return modelo.Respuesta{}, fmt.Errorf("%w: %v", ErrPreguntaInvalida, err)
	}
	if cacheado, hay := c.leeCache(ctx, llave); hay {
		return c.evalua(cacheado, umbral)
	}

	item, err := c.llama(ctx, crudo, pregunta.ID)
	if err != nil {
		return modelo.Respuesta{}, err
	}
	respuesta, confianza, err := aRespuesta(pregunta, item)
	if err != nil {
		return modelo.Respuesta{}, err
	}
	c.guardaCache(ctx, llave, respuesta)
	return c.evaluaConConfianza(respuesta, confianza, umbral)
}

// evalua aplica el umbral a una respuesta que viene de la caché.
func (c *Cliente) evalua(respuesta modelo.Respuesta, umbral float64) (modelo.Respuesta, error) {
	return c.evaluaConConfianza(respuesta, 0, umbral)
}

func (c *Cliente) evaluaConConfianza(respuesta modelo.Respuesta, confianza, umbral float64) (modelo.Respuesta, error) {
	prob := probabilidadDeLaOpcion(respuesta, confianza)
	if umbral > 0 && prob <= umbral {
		return respuesta, fmt.Errorf("%w: P(%s)=%.3f ≤ %.2f", ErrBajaConfianza, respuesta.Opcion, prob, umbral)
	}
	return respuesta, nil
}

// llama hace los intentos, con el semáforo tomado y el contexto respetado.
func (c *Cliente) llama(ctx context.Context, crudo []byte, idPregunta string) (respuestaItem, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return respuestaItem{}, c.errorDeContexto(ctx, nil)
	}

	var ultimo error
	for intento := 1; intento <= c.conf.Intentos; intento++ {
		if intento > 1 {
			if err := c.espera(ctx, intento); err != nil {
				return respuestaItem{}, err
			}
		}
		item, err := c.intento(ctx, crudo, idPregunta)
		if err == nil {
			return item, nil
		}
		c.anotaFallo()
		ultimo = err
		if !reintentable(err) {
			return respuestaItem{}, err
		}
	}
	return respuestaItem{}, ultimo
}

// intento es una petición HTTP completa.
func (c *Cliente) intento(ctx context.Context, crudo []byte, idPregunta string) (respuestaItem, error) {
	ctxIntento, cancelar := context.WithTimeout(ctx, c.conf.TiempoLimite)
	defer cancelar()

	req, err := http.NewRequestWithContext(ctxIntento, http.MethodPost, c.conf.URLBase+"/api/alpha/decisions", bytes.NewReader(crudo))
	if err != nil {
		return respuestaItem{}, fmt.Errorf("%w: %v", ErrRespuestaInvalida, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.conf.ClaveAPI)

	c.anotaLLamada()
	resp, err := c.http.Do(req)
	if err != nil {
		return respuestaItem{}, c.errorDeContexto(ctx, err)
	}
	defer resp.Body.Close()

	datos, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return respuestaItem{}, c.errorDeContexto(ctx, err)
	}
	if resp.StatusCode != http.StatusOK {
		return respuestaItem{}, &ErrorHTTP{Estado: resp.StatusCode, Mensaje: mensajeDeError(datos)}
	}

	var salida respuestaDecision
	if err := json.Unmarshal(datos, &salida); err != nil {
		return respuestaItem{}, fmt.Errorf("%w: JSON ilegible: %v", ErrRespuestaInvalida, err)
	}
	c.anotaUso(salida.Uso)

	if salida.Modelo != "" && salida.Modelo != c.conf.Modelo {
		return respuestaItem{}, fmt.Errorf("%w: el proveedor sirvió %q, se pidió %q", ErrRespuestaInvalida, salida.Modelo, c.conf.Modelo)
	}
	item, hay := salida.Respuestas[idPregunta]
	if !hay {
		return respuestaItem{}, fmt.Errorf("%w: falta la respuesta de la pregunta %q", ErrRespuestaInvalida, idPregunta)
	}
	return item, nil
}

// errorDeContexto distingue el plazo agotado de un fallo de red.
func (c *Cliente) errorDeContexto(ctx context.Context, causa error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %v", ErrTiempoLimite, ctx.Err())
	}
	if errors.Is(causa, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrTiempoLimite, causa)
	}
	if causa == nil {
		return ErrTiempoLimite
	}
	return fmt.Errorf("%w: fallo de red: %v", ErrHTTP, causa)
}

// espera duerme el backoff sin ignorar la cancelación.
func (c *Cliente) espera(ctx context.Context, intento int) error {
	espera := c.conf.Espera << (intento - 2)
	temporizador := time.NewTimer(espera)
	defer temporizador.Stop()
	select {
	case <-temporizador.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrTiempoLimite, ctx.Err())
	}
}

// reintentable dice si el error merece otro intento.
func reintentable(err error) bool {
	if errors.Is(err, ErrTiempoLimite) || errors.Is(err, ErrHTTP) {
		var httpErr *ErrorHTTP
		if errors.As(err, &httpErr) {
			switch httpErr.Estado {
			case http.StatusRequestTimeout, http.StatusTooManyRequests,
				http.StatusInternalServerError, http.StatusBadGateway,
				http.StatusServiceUnavailable, http.StatusGatewayTimeout:
				return true
			}
			return false
		}
		// Fallo de red o timeout: vale otro intento.
		return true
	}
	return false
}

// mensajeDeError extrae solo el mensaje del proveedor: el cuerpo de error puede
// repetir el state y el state lleva datos reales (D15).
func mensajeDeError(crudo []byte) string {
	var envoltura struct {
		Error struct {
			Mensaje string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(crudo, &envoltura); err == nil && envoltura.Error.Mensaje != "" {
		return recorta(envoltura.Error.Mensaje, 200)
	}
	return recorta(strings.TrimSpace(string(crudo)), 80)
}

func recorta(texto string, max int) string {
	if len(texto) <= max {
		return texto
	}
	return texto[:max] + "…"
}

// llaveDeCache fija snapshot, state enmascarado y pregunta en un hash (D32).
func llaveDeCache(snapshot, estado string, preguntas map[string]preguntaDecision) (string, error) {
	canonico, err := json.Marshal(preguntas)
	if err != nil {
		return "", err
	}
	suma := sha256.New()
	for _, parte := range []string{snapshot, "\x00", estado, "\x00", string(canonico)} {
		suma.Write([]byte(parte))
	}
	return hex.EncodeToString(suma.Sum(nil)), nil
}

// leeCache devuelve la respuesta guardada para la llave.
func (c *Cliente) leeCache(ctx context.Context, llave string) (modelo.Respuesta, bool) {
	if c.conf.Cache == nil {
		return modelo.Respuesta{}, false
	}
	crudo, hay, err := c.conf.Cache.Lee(ctx, llave)
	if err != nil || !hay {
		return modelo.Respuesta{}, false
	}
	var respuesta modelo.Respuesta
	if err := json.Unmarshal([]byte(crudo), &respuesta); err != nil {
		return modelo.Respuesta{}, false
	}
	c.uso.mu.Lock()
	c.uso.v.Aciertos++
	c.uso.mu.Unlock()
	return respuesta, true
}

// guardaCache guarda la respuesta ya traducida; un fallo de caché no tumba la
// decisión.
func (c *Cliente) guardaCache(ctx context.Context, llave string, respuesta modelo.Respuesta) {
	if c.conf.Cache == nil {
		return
	}
	crudo, err := json.Marshal(respuesta)
	if err != nil {
		return
	}
	_ = c.conf.Cache.Guarda(ctx, llave, string(crudo))
}

func (c *Cliente) anotaLLamada() {
	c.uso.mu.Lock()
	c.uso.v.Llamadas++
	c.uso.mu.Unlock()
}

func (c *Cliente) anotaFallo() {
	c.uso.mu.Lock()
	c.uso.v.Fallos++
	c.uso.mu.Unlock()
}

func (c *Cliente) anotaUso(u usoDecision) {
	c.uso.mu.Lock()
	c.uso.v.TokensEntrada += u.Entrada
	c.uso.v.TokensSalida += u.Salida
	c.uso.v.CostoUSD += u.Costo
	c.uso.mu.Unlock()
}

// validaPregunta exige lo que el endpoint necesita y lo que el pipeline promete:
// un id para la respuesta, un texto y las opciones de las preguntas con opciones.
func validaPregunta(p modelo.Pregunta) error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("%w: falta el id de la pregunta", ErrPreguntaInvalida)
	}
	if strings.TrimSpace(p.Texto) == "" {
		return fmt.Errorf("%w: la pregunta %q no tiene texto", ErrPreguntaInvalida, p.ID)
	}
	switch p.Tipo {
	case tipoChoice, tipoScore:
		if len(p.Opciones) == 0 {
			return fmt.Errorf("%w: la pregunta %q de tipo %s no tiene opciones", ErrPreguntaInvalida, p.ID, p.Tipo)
		}
		for _, opcion := range p.Opciones {
			if strings.TrimSpace(opcion) == "" {
				return fmt.Errorf("%w: la pregunta %q tiene una opción vacía", ErrPreguntaInvalida, p.ID)
			}
		}
	case tipoNoul:
	default:
		return fmt.Errorf("%w: tipo %q desconocido; son choice, noul o score", ErrPreguntaInvalida, p.Tipo)
	}
	return nil
}
