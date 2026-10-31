package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// respondeChoice escribe la respuesta de Jev con el snapshot que pidió el
// cliente y con la clave de la pregunta que llegó: el router devuelve el modelo
// servido y las respuestas vienen por la clave de questions.
func respondeChoice(w http.ResponseWriter, cuerpo peticion) {
	modelo, _ := cuerpo["model"].(string)
	idPregunta := "q1"
	if preguntas, ok := cuerpo["questions"].(map[string]any); ok {
		for clave := range preguntas {
			idPregunta = clave
		}
	}
	fmt.Fprintf(w, `{
  "id": "gen-dec-1",
  "model": %q,
  "provider": "TypeSafe",
  "answers": {
    %q: {
      "type": "choice",
      "choice": "person",
      "confidence": 0.92,
      "probabilities": {"person": 0.92, "organization_or_place": 0.05, "other": 0.03}
    }
  },
  "usage": {"cost": 0.00002, "input_tokens": 120, "output_tokens": 12}
}`, modelo, idPregunta)
}

// peticion es un cuerpo recibido por el servidor falso, ya decodificado.
type peticion map[string]any

// servidorFalso levanta un OpenRouter falso. Responde con el estado y el cuerpo
// que devuelve responde, y deja ver los cuerpos recibidos.
type falso struct {
	servidor *httptest.Server
	mu       sync.Mutex
	cuerpos  []peticion
}

func (f *falso) veces() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.cuerpos)
}

func (f *falso) cuerpo(i int) peticion {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cuerpos[i]
}

func nuevoFalso(t *testing.T, responde func(n int, w http.ResponseWriter, cuerpo peticion)) *falso {
	t.Helper()
	f := &falso{}
	f.servidor = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var cuerpo peticion
		if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
			t.Errorf("cuerpo ilegible: %v", err)
		}
		f.mu.Lock()
		f.cuerpos = append(f.cuerpos, cuerpo)
		n := len(f.cuerpos)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		responde(n, w, cuerpo)
	}))
	t.Cleanup(f.servidor.Close)
	return f
}

func clienteDePrueba(t *testing.T, f *falso, ajusta func(*Configuracion)) *Cliente {
	t.Helper()
	conf := Configuracion{
		ClaveAPI:     "clave-de-prueba",
		Modelo:       SnapshotJev,
		URLBase:      f.servidor.URL,
		TiempoLimite: 2 * time.Second,
		Intentos:     1,
		Cache:        NuevaCacheMemoria(),
	}
	if ajusta != nil {
		ajusta(&conf)
	}
	c, err := Nueva(conf)
	if err != nil {
		t.Fatalf("Nueva: %v", err)
	}
	return c
}

func preguntaChoice() modelo.Pregunta {
	return modelo.Pregunta{
		ID:       "q1",
		Tipo:     "choice",
		Texto:    "¿A qué tipo de entidad corresponde el fragmento?",
		Opciones: []string{"person", "organization_or_place", "brand", "public_figure", "other"},
	}
}

// El cuerpo enviado es el del endpoint de decisiones: una sola pregunta, con
// su propio state (D28), sin retención de datos (D15) y con el snapshot fijo
// (D32).
func TestDecideEnviaUnaPreguntaCerrada(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, nil)

	r, err := c.Decide(context.Background(), "[PERSONA_1] pide una reposición", preguntaChoice())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if r.Opcion != "person" {
		t.Fatalf("opción %q, se esperaba person", r.Opcion)
	}
	if r.Pregunta != "q1" {
		t.Fatalf("pregunta %q, se esperaba q1", r.Pregunta)
	}
	if got := r.Probabilidades["organization_or_place"]; got != 0.05 {
		t.Fatalf("P(organization_or_place) = %v, se esperaba 0,05", got)
	}

	cuerpo := f.cuerpo(0)
	if cuerpo["model"] != SnapshotJev {
		t.Fatalf("model %v, se esperaba el snapshot %s", cuerpo["model"], SnapshotJev)
	}
	prov, ok := cuerpo["provider"].(map[string]any)
	if !ok || prov["data_collection"] != "deny" {
		t.Fatalf("provider %v, se esperaba data_collection deny", cuerpo["provider"])
	}
	if cuerpo["state"] != "[PERSONA_1] pide una reposición" {
		t.Fatalf("state %v", cuerpo["state"])
	}
	preguntas, ok := cuerpo["questions"].(map[string]any)
	if !ok || len(preguntas) != 1 {
		t.Fatalf("questions %v, se esperaba una sola", cuerpo["questions"])
	}
	q, ok := preguntas["q1"].(map[string]any)
	if !ok {
		t.Fatalf("questions sin la clave q1: %v", preguntas)
	}
	if q["type"] != "choice" || q["instructions"] != "¿A qué tipo de entidad corresponde el fragmento?" {
		t.Fatalf("pregunta mal formada: %v", q)
	}
	criterios, ok := q["criteria"].(map[string]any)
	if !ok || len(criterios) != 5 {
		t.Fatalf("criteria %v, se esperaban las cinco opciones", q["criteria"])
	}
}

// La segunda llamada con la misma entrada no sale a la red (D15, D32).
func TestCacheEvitaLaRed(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, nil)

	for i := 0; i < 3; i++ {
		if _, err := c.Decide(context.Background(), "mismo state", preguntaChoice()); err != nil {
			t.Fatalf("Decide %d: %v", i, err)
		}
	}
	if n := f.veces(); n != 1 {
		t.Fatalf("%d peticiones, se esperaba 1", n)
	}
	if uso := c.Uso(); uso.Aciertos != 2 {
		t.Fatalf("aciertos de caché %d, se esperaban 2", uso.Aciertos)
	}
}

// La caché distingue por snapshot: el alias movido no reusa decisiones viejas
// (D32).
func TestCacheSeparaPorSnapshot(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, nil)
	if _, err := c.Decide(context.Background(), "mismo state", preguntaChoice()); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	otro, err := Nueva(Configuracion{
		ClaveAPI:     "clave-de-prueba",
		Modelo:       "typesafe/jev-1.14-20261015",
		URLBase:      f.servidor.URL,
		TiempoLimite: time.Second,
		Intentos:     1,
		Cache:        c.conf.Cache,
	})
	if err != nil {
		t.Fatalf("Nueva: %v", err)
	}
	if _, err := otro.Decide(context.Background(), "mismo state", preguntaChoice()); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if n := f.veces(); n != 2 {
		t.Fatalf("%d peticiones, se esperaban 2 (snapshots distintos)", n)
	}
}

// Un proveedor colgado cae por timeout, con error tipado y sin colgar.
func TestTiempoLimiteCaeConErrorTipado(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		time.Sleep(2 * time.Second)
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, func(conf *Configuracion) {
		conf.TiempoLimite = 60 * time.Millisecond
	})

	inicio := time.Now()
	_, err := c.Decide(context.Background(), "state", preguntaChoice())
	tardanza := time.Since(inicio)

	if !errors.Is(err, ErrTiempoLimite) {
		t.Fatalf("error %v, se esperaba ErrTiempoLimite", err)
	}
	if tardanza > time.Second {
		t.Fatalf("tardó %v: el timeout no corta", tardanza)
	}
}

// El plazo del contexto manda sobre el timeout por intento.
func TestContextoCanceladoCorta(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		time.Sleep(2 * time.Second)
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, func(conf *Configuracion) {
		conf.TiempoLimite = 5 * time.Second
	})
	ctx, cancelar := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancelar()

	if _, err := c.Decide(ctx, "state", preguntaChoice()); err == nil {
		t.Fatal("se esperaba error al cortar el contexto")
	}
}

// JSON inválido o incompleto da error claro y no pánico.
func TestJSONInvalidoNoEsPanico(t *testing.T) {
	casos := map[string]string{
		"basura":          `<html>502 Bad Gateway</html>`,
		"sin answers":     `{"id":"x","model":"m","usage":{"input_tokens":1,"output_tokens":1}}`,
		"sin la pregunta": `{"answers":{"otra":{"type":"choice","choice":"person"}},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"sin opción":      `{"answers":{"q1":{"type":"choice","probabilities":{"person":0.9}}}}`,
		"vacío":           ``,
	}
	for nombre, cuerpo := range casos {
		t.Run(nombre, func(t *testing.T) {
			f := nuevoFalso(t, func(n int, w http.ResponseWriter, _ peticion) {
				fmt.Fprint(w, cuerpo)
			})
			c := clienteDePrueba(t, f, nil)
			_, err := c.Decide(context.Background(), "state", preguntaChoice())
			if !errors.Is(err, ErrRespuestaInvalida) {
				t.Fatalf("error %v, se esperaba ErrRespuestaInvalida", err)
			}
		})
	}
}

// Un 400 no se reintenta y viaja como error tipado.
func TestErrorHTTPNoSeReintenta(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":400,"message":"Invalid request parameters"}}`)
	})
	c := clienteDePrueba(t, f, func(conf *Configuracion) { conf.Intentos = 3 })

	_, err := c.Decide(context.Background(), "state", preguntaChoice())
	if !errors.Is(err, ErrHTTP) {
		t.Fatalf("error %v, se esperaba ErrHTTP", err)
	}
	if n := f.veces(); n != 1 {
		t.Fatalf("%d peticiones, un 400 no se reintenta", n)
	}
	var httpErr *ErrorHTTP
	if !errors.As(err, &httpErr) || httpErr.Estado != http.StatusBadRequest {
		t.Fatalf("error %v, se esperaba ErrorHTTP con estado 400", err)
	}
}

// 429 y 5xx se reintentan con espera corta y acotada.
func TestReintentaAnte429(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"code":429,"message":"Rate limit"}}`)
			return
		}
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, func(conf *Configuracion) {
		conf.Intentos = 2
		conf.Espera = 5 * time.Millisecond
	})

	r, err := c.Decide(context.Background(), "state", preguntaChoice())
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if r.Opcion != "person" {
		t.Fatalf("opción %q", r.Opcion)
	}
	if n := f.veces(); n != 2 {
		t.Fatalf("%d peticiones, se esperaban 2", n)
	}
}

// Los reintentos son acotados: si todo falla, el error llega.
func TestReintentosAcotados(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, `{"error":{"code":502,"message":"upstream"}}`)
	})
	c := clienteDePrueba(t, f, func(conf *Configuracion) {
		conf.Intentos = 3
		conf.Espera = time.Millisecond
	})
	if _, err := c.Decide(context.Background(), "state", preguntaChoice()); !errors.Is(err, ErrHTTP) {
		t.Fatalf("error %v, se esperaba ErrHTTP", err)
	}
	if n := f.veces(); n != 3 {
		t.Fatalf("%d peticiones, se esperaban 3 intentos", n)
	}
}

// Antes de salir, toda forma real conocida se enmascara (D15).
func TestCuerpoSinFormasReales(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, func(conf *Configuracion) {
		conf.Mascara = NuevaMascara([]string{"Ramona Salas Quintero", "ramona.salas@ejemplo.test", "900000123"})
	})

	state := "Ramona Salas Quintero (ramona.salas@ejemplo.test, cédula 900000123) dice que no le llegó el pago."
	if _, err := c.Decide(context.Background(), state, preguntaChoice()); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	enviado, err := json.Marshal(f.cuerpo(0)["state"])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forma := range []string{"Ramona", "Salas", "Quintero", "900000123", "ramona.salas"} {
		if strings.Contains(strings.ToLower(string(enviado)), strings.ToLower(forma)) {
			t.Fatalf("el cuerpo enviado lleva la forma %q: %s", forma, enviado)
		}
	}
	if !strings.Contains(string(enviado), "[PERSONA_1]") {
		t.Fatalf("sin marcador de persona: %s", enviado)
	}
}

// El enmascarado no distingue mayúsculas ni tildes y respeta el resto del texto.
func TestMascaraPliegaTildesYMayusculas(t *testing.T) {
	m := NuevaMascara([]string{"María Peña"})
	got := m.Enmascara("Hola, MARIA PENA; saludos a MARÍA PEÑA.")
	quiere := "Hola, [PERSONA_1]; saludos a [PERSONA_1]."
	if got != quiere {
		t.Fatalf("enmascarado %q, se esperaba %q", got, quiere)
	}
}

// Por debajo del umbral de D31 la respuesta no se acepta, pero se devuelve para
// la cola de dudosos (D19).
func TestUmbralesD31(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		fmt.Fprint(w, `{"answers":{"q1":{"type":"choice","choice":"person","confidence":0.42,
			"probabilities":{"person":0.42,"other":0.58}}},"usage":{"input_tokens":1,"output_tokens":1}}`)
	})
	c := clienteDePrueba(t, f, nil)

	r, err := c.Decide(context.Background(), "state", preguntaChoice())
	if !errors.Is(err, ErrBajaConfianza) {
		t.Fatalf("error %v, se esperaba ErrBajaConfianza", err)
	}
	if r.Opcion != "person" || r.Probabilidades["person"] != 0.42 {
		t.Fatalf("la respuesta se pierde: %+v", r)
	}

	f2 := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		respondeChoice(w, cuerpo)
	})
	c2 := clienteDePrueba(t, f2, nil)
	if _, err := c2.DecideUmbral(context.Background(), "state", preguntaChoice(), UmbralDesempate); err != nil {
		t.Fatalf("P 0,92 debía superar el umbral de desempate: %v", err)
	}
}

// Menos da el consumo que hay entre dos fotografías de la contabilidad
// acumulada: es lo que una corrida gastó, sin el acumulado de las anteriores
// (C23).
func TestUsoMenos(t *testing.T) {
	antes := Uso{Llamadas: 20, Aciertos: 3, Fallos: 1, TokensEntrada: 5000, TokensSalida: 300, CostoUSD: 1.5}
	despues := Uso{Llamadas: 26, Aciertos: 7, Fallos: 2, TokensEntrada: 5500, TokensSalida: 380, CostoUSD: 1.75}
	quiero := Uso{Llamadas: 6, Aciertos: 4, Fallos: 1, TokensEntrada: 500, TokensSalida: 80, CostoUSD: 0.25}
	if got := despues.Menos(antes); got != quiero {
		t.Fatalf("Menos = %+v, quiero %+v", got, quiero)
	}
	if got := despues.Menos(despues); got != (Uso{}) {
		t.Fatalf("Menos de la misma fotografía = %+v, quiero cero", got)
	}
}

// Los tokens y el costo de cada respuesta se acumulan (D13, D32).
func TestContabilidadDeTokensYCosto(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, nil)
	for i := 0; i < 2; i++ {
		p := preguntaChoice()
		p.ID = fmt.Sprintf("q%d", i+1)
		if _, err := c.Decide(context.Background(), "state", p); err != nil {
			t.Fatalf("Decide: %v", err)
		}
	}
	uso := c.Uso()
	if uso.Llamadas != 2 || uso.TokensEntrada != 240 || uso.TokensSalida != 24 {
		t.Fatalf("uso %+v", uso)
	}
	if uso.CostoUSD < 0.000039 || uso.CostoUSD > 0.000041 {
		t.Fatalf("costo %v, se esperaban 0,00004", uso.CostoUSD)
	}
}

// El tope de concurrencia es 16 (D28): con 40 llamadas simultáneas nunca hay
// más de 16 en vuelo, y sí llega a 16.
func TestTopeDeConcurrencia16(t *testing.T) {
	var enVuelo, pico atomic.Int32
	puerta := make(chan struct{})
	var cerrar sync.Once

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var cuerpo peticion
		if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
			t.Errorf("cuerpo ilegible: %v", err)
		}
		n := enVuelo.Add(1)
		for {
			p := pico.Load()
			if n <= p || pico.CompareAndSwap(p, n) {
				break
			}
		}
		if n >= TopeConcurrencia {
			cerrar.Do(func() { close(puerta) })
		}
		<-puerta
		defer enVuelo.Add(-1)
		respondeChoice(w, cuerpo)
	}))
	defer servidor.Close()

	conf := Configuracion{
		ClaveAPI:     "clave-de-prueba",
		Modelo:       SnapshotJev,
		URLBase:      servidor.URL,
		TiempoLimite: 5 * time.Second,
		Intentos:     1,
	}
	c, err := Nueva(conf)
	if err != nil {
		t.Fatalf("Nueva: %v", err)
	}
	if c.Tope() != TopeConcurrencia {
		t.Fatalf("tope %d, se esperaba %d", c.Tope(), TopeConcurrencia)
	}

	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := preguntaChoice()
			p.ID = fmt.Sprintf("q%d", i)
			if _, err := c.Decide(context.Background(), fmt.Sprintf("state %d", i), p); err != nil {
				t.Errorf("Decide %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	if got := pico.Load(); got != TopeConcurrencia {
		t.Fatalf("pico de concurrencia %d, se esperaba %d", got, TopeConcurrencia)
	}
}

// Una pregunta fuera del contrato no gasta una llamada.
func TestPreguntaInvalida(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, nil)
	casos := []modelo.Pregunta{
		{Tipo: "choice", Texto: "x", Opciones: []string{"a"}},
		{ID: "q1", Texto: "x", Opciones: []string{"a"}},
		{ID: "q1", Tipo: "abierta", Texto: "x"},
		{ID: "q1", Tipo: "choice", Opciones: []string{"a"}},
		{ID: "q1", Tipo: "choice", Texto: "x"},
	}
	for _, p := range casos {
		if _, err := c.Decide(context.Background(), "state", p); !errors.Is(err, ErrPreguntaInvalida) {
			t.Fatalf("%+v: error %v, se esperaba ErrPreguntaInvalida", p, err)
		}
	}
	if n := f.veces(); n != 0 {
		t.Fatalf("%d peticiones, ninguna pregunta inválida debía salir", n)
	}
}

// noul responde una sola probabilidad de "verdadero": se derivan las dos.
func TestNoulDerivaProbabilidades(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		fmt.Fprint(w, `{"answers":{"q1":{"type":"noul","noul":0.96}},"usage":{"input_tokens":10,"output_tokens":2}}`)
	})
	c := clienteDePrueba(t, f, nil)
	p := modelo.Pregunta{ID: "q1", Tipo: "noul", Texto: "¿El fragmento nombra a una persona?"}

	r, err := c.Decide(context.Background(), "state", p)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if r.Opcion != "true" || r.Probabilidades["true"] != 0.96 || math.Abs(r.Probabilidades["false"]-0.04) > 1e-9 {
		t.Fatalf("respuesta noul mal derivada: %+v", r)
	}
	if q, ok := f.cuerpo(0)["questions"].(map[string]any)["q1"].(map[string]any); !ok || q["type"] != "noul" {
		t.Fatalf("pregunta noul mal enviada: %v", f.cuerpo(0)["questions"])
	}
}

// score trae un legend por índice: la opción respuesta es el criterio, no el
// número.
func TestScoreUsaLeyenda(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		fmt.Fprint(w, `{"answers":{"q1":{"type":"score","score":1.99,
			"legend":{"0":"M","1":"F","2":"indeterminado"},
			"probabilities":{"0":0.01,"1":0.98,"2":0.01}}},"usage":{"input_tokens":10,"output_tokens":2}}`)
	})
	c := clienteDePrueba(t, f, nil)
	p := modelo.Pregunta{ID: "q1", Tipo: "score", Texto: "¿Qué género corresponde?", Opciones: []string{"M", "F", "indeterminado"}}

	r, err := c.Decide(context.Background(), "state", p)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if r.Opcion != "F" || r.Probabilidades["F"] != 0.98 {
		t.Fatalf("score mal traducido: %+v", r)
	}
}

// LLM_MODEL manda; sin él se usa el snapshot fijo (D32).
func TestNuevaDesdeEntorno(t *testing.T) {
	entorno := map[string]string{
		"OPENROUTER_API_KEY": "clave-de-prueba",
		"LLM_MODEL":          "typesafe/jev-1.14-20261015",
	}
	c, err := NuevaDesdeEntorno(func(k string) string { return entorno[k] })
	if err != nil {
		t.Fatalf("NuevaDesdeEntorno: %v", err)
	}
	if c.Nombre() != "typesafe/jev-1.14-20261015" {
		t.Fatalf("modelo %q", c.Nombre())
	}

	c2, err := NuevaDesdeEntorno(func(k string) string { return map[string]string{"OPENROUTER_API_KEY": "k"}[k] })
	if err != nil {
		t.Fatalf("NuevaDesdeEntorno: %v", err)
	}
	if c2.Nombre() != SnapshotJev {
		t.Fatalf("sin LLM_MODEL el modelo es %q, se esperaba %s", c2.Nombre(), SnapshotJev)
	}

	if _, err := NuevaDesdeEntorno(func(string) string { return "" }); !errors.Is(err, ErrSinClave) {
		t.Fatalf("error %v, se esperaba ErrSinClave", err)
	}
}

// La ruta es la del endpoint de decisiones, no la de chat.
func TestRutaDelEndpoint(t *testing.T) {
	var ruta string
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ruta = r.URL.Path
		respondeChoice(w, peticion{"model": SnapshotJev})
	}))
	defer servidor.Close()
	c, err := Nueva(Configuracion{ClaveAPI: "k", URLBase: servidor.URL, TiempoLimite: time.Second, Intentos: 1})
	if err != nil {
		t.Fatalf("Nueva: %v", err)
	}
	if _, err := c.Decide(context.Background(), "state", preguntaChoice()); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if ruta != "/api/alpha/decisions" {
		t.Fatalf("ruta %q", ruta)
	}
}

// Sin OPENROUTER_API_KEY no se arma el cliente: quien compone decide el respaldo
// determinista.
func TestNuevaSinClave(t *testing.T) {
	if _, err := Nueva(Configuracion{}); !errors.Is(err, ErrSinClave) {
		t.Fatalf("error %v, se esperaba ErrSinClave", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// Las preguntas idénticas que llegan a la vez viajan una sola vez: con 16 en
// paralelo, el mismo state sale varias veces antes de que la primera respuesta
// llegue a la caché, y el 90 % de las preguntas de una corrida se repite.
func TestPreguntasIdenticasEnVueloViajanUnaVez(t *testing.T) {
	f := nuevoFalso(t, func(n int, w http.ResponseWriter, cuerpo peticion) {
		time.Sleep(50 * time.Millisecond)
		respondeChoice(w, cuerpo)
	})
	c := clienteDePrueba(t, f, nil)

	const n = 12
	var wg sync.WaitGroup
	errores := make(chan error, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Decide(context.Background(), "mismo state", preguntaChoice()); err != nil {
				errores <- err
			}
		}()
	}
	wg.Wait()
	close(errores)
	for err := range errores {
		t.Errorf("Decide: %v", err)
	}
	if veces := f.veces(); veces != 1 {
		t.Fatalf("%d peticiones para la misma pregunta en vuelo, se esperaba 1", veces)
	}
}
