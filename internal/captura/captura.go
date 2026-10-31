// Package captura registra cada request HTTP como una línea JSON de ids,
// códigos y conteos. Nunca guarda cuerpos: los cuerpos del jurado traen PII.
// La captura va al volumen (secuencias que se pueden repetir en local) y a
// stdout solo el resumen, sin nombres ni textos.
package captura

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Linea es un request registrado. Ruta es la ruta ya resuelta por el mux (con
// comodines), nunca el cuerpo; el token que llegue en la URL se recorta.
type Linea struct {
	N          int     `json:"n"`
	TS         string  `json:"ts"`
	Metodo     string  `json:"metodo"`
	Ruta       string  `json:"ruta"`
	Codigo     int     `json:"codigo"`
	DuracionMS float64 `json:"duracion_ms"`
	Bytes      int     `json:"bytes"`
	Job        string  `json:"job,omitempty"`
	Fuente     string  `json:"fuente,omitempty"`
	Registros  int     `json:"registros,omitempty"`
	Entidades  int     `json:"entidades,omitempty"`
	Error      string  `json:"error,omitempty"`
}

// Metrica es lo que el handler aporta a la línea de captura. Solo ids y
// conteos: nada de valores de registro.
type Metrica struct {
	Job       string
	Fuente    string
	Registros int
	Entidades int
	Error     string
}

type claveMetrica struct{}

// Anota devuelve la métrica del request para que el handler la complete. Fuera
// del middleware devuelve una métrica descartable; nunca nil.
func Anota(r *http.Request) *Metrica {
	if m, ok := r.Context().Value(claveMetrica{}).(*Metrica); ok {
		return m
	}
	return &Metrica{}
}

// Opciones dice a dónde va cada línea. Archivo (con fsync por línea) es lo que
// permite reproducir después un lote del jurado; la terminal, el resumen.
type Opciones struct {
	Destinos []io.Writer
}

// Middleware envuelve siguiente y publica una línea por request. No altera la
// respuesta: solo cuenta bytes y código.
func Middleware(siguiente http.Handler, o Opciones) http.Handler {
	var mu sync.Mutex
	n := 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		mu.Lock()
		n++
		numero := n
		mu.Unlock()

		metrica := &Metrica{}
		contador := &respuesta{w: w}
		siguiente.ServeHTTP(contador, r.WithContext(context.WithValue(r.Context(), claveMetrica{}, metrica)))
		if contador.codigo == 0 {
			contador.codigo = http.StatusOK
		}

		linea := Linea{
			N:          numero,
			TS:         inicio.UTC().Format(time.RFC3339Nano),
			Metodo:     r.Method,
			Ruta:       ruta(r),
			Codigo:     contador.codigo,
			DuracionMS: float64(time.Since(inicio).Microseconds()) / 1000,
			Bytes:      contador.bytes,
			Job:        metrica.Job,
			Fuente:     metrica.Fuente,
			Registros:  metrica.Registros,
			Entidades:  metrica.Entidades,
			Error:      metrica.Error,
		}
		mu.Lock()
		defer mu.Unlock()
		publica(o.Destinos, linea)
	})
}

// respuesta cuenta el código y los bytes escritos.
type respuesta struct {
	w      http.ResponseWriter
	codigo int
	bytes  int
}

func (r *respuesta) Header() http.Header { return r.w.Header() }

func (r *respuesta) WriteHeader(codigo int) {
	if r.codigo == 0 {
		r.codigo = codigo
	}
	r.w.WriteHeader(codigo)
}

func (r *respuesta) Write(b []byte) (int, error) {
	if r.codigo == 0 {
		r.codigo = http.StatusOK
	}
	n, err := r.w.Write(b)
	r.bytes += n
	return n, err
}

// Flush conserva el soporte de streaming de quien lo use.
func (r *respuesta) Flush() {
	if f, ok := r.w.(http.Flusher); ok {
		f.Flush()
	}
}

// ruta usa el patrón del mux cuando el mux ya lo fijó (`GET /v1/run/{id}`) y,
// si no, la ruta de la URL sin query. Se recorta para que un path absurdo no
// infle la captura.
func ruta(r *http.Request) string {
	p := r.Pattern
	if p == "" {
		p = r.URL.Path
	}
	const maximo = 200
	if len(p) > maximo {
		return p[:maximo]
	}
	return p
}

func publica(destinos []io.Writer, linea Linea) {
	b, err := json.Marshal(linea)
	if err != nil {
		fmt.Fprintf(os.Stderr, "captura: %v\n", err)
		return
	}
	b = append(b, '\n')
	for _, d := range destinos {
		if _, err := d.Write(b); err != nil {
			fmt.Fprintf(os.Stderr, "captura: %v\n", err)
		}
	}
}

// Archivo abre (o crea) el JSONL de captura y sincroniza cada línea a disco,
// para que un reinicio no pierda lo que mandó el jurado.
func Archivo(nombre string) (io.WriteCloser, error) {
	f, err := os.OpenFile(nombre, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return sincronizado{f}, nil
}

type sincronizado struct{ *os.File }

func (s sincronizado) Write(b []byte) (int, error) {
	n, err := s.File.Write(b)
	if err == nil {
		err = s.File.Sync()
	}
	return n, err
}
