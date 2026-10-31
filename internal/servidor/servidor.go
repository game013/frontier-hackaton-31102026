// Package servidor expone el pipeline como servicio HTTP: las seis rutas del
// contrato bajo /v1, corridas en goroutine, salida servida desde el volumen y
// un único núcleo para /run y /anonymize (D8, D12, D33).
package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/captura"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Config es lo que se lee del entorno al arrancar. DatosDir es el volumen
// (SQLite, salidas y captura); en local vale var/.
type Config struct {
	Puerto      string
	DatosDir    string
	Equipo      string
	Version     string
	Modelos     []string
	PythonBin   string
	AyudantePDF string
}

// Carga valida la configuración una sola vez: un valor inválido impide
// arrancar en vez de elegir un comportamiento por defecto oculto.
func Carga(getenv func(string) string) (Config, error) {
	c := Config{
		Puerto:      or(getenv("PORT"), "9000"),
		DatosDir:    or(getenv("DATA_DIR"), "var"),
		Equipo:      or(getenv("TEAM"), "kipu-frontier"),
		Version:     or(getenv("VERSION"), "dev"),
		Modelos:     lista(or(getenv("MODELOS"), getenv("LLM_MODEL"))),
		PythonBin:   or(getenv("PYTHON_BIN"), "python3"),
		AyudantePDF: or(getenv("PDF_HELPER"), filepath.Join("tools", "pdf", "extraer.py")),
	}
	if _, err := strconv.Atoi(c.Puerto); err != nil {
		return c, fmt.Errorf("PORT no es un número: %q", c.Puerto)
	}
	if strings.TrimSpace(c.DatosDir) == "" {
		return c, errors.New("DATA_DIR vacío")
	}
	if c.Equipo == "" {
		return c, errors.New("TEAM vacío")
	}
	return c, nil
}

func or(valor, porDefecto string) string {
	if valor == "" {
		return porDefecto
	}
	return valor
}

// lista separa por comas y descarta vacíos; siempre devuelve lista no nula para
// que /v1/health viaje con [] y no con null.
func lista(valor string) []string {
	salida := []string{}
	for _, parte := range strings.Split(valor, ",") {
		if parte = strings.TrimSpace(parte); parte != "" {
			salida = append(salida, parte)
		}
	}
	return salida
}

// Deps son las piezas que se inyectan: el núcleo compartido (D8) y el ejecutor
// de /run. Sin ellas, C1 sirve el eco y /run queda pendiente.
type Deps struct {
	Nucleo   modelo.Nucleo
	Ejecutor Ejecutor
}

// Servidor es el servicio completo. Handler() se envuelve con la captura en
// cmd/servidor; Cerrar cancela las corridas en curso.
type Servidor struct {
	cfg      Config
	nucleo   modelo.Nucleo
	corridas *corridas
	mux      *http.ServeMux
	cancel   context.CancelFunc
}

// Nuevo arma el servicio sin abrir puertos: el arranque y el apagado viven en
// cmd/servidor.
func Nuevo(cfg Config, deps Deps) (*Servidor, error) {
	if strings.TrimSpace(cfg.DatosDir) == "" {
		return nil, errors.New("DatosDir vacío")
	}
	if deps.Nucleo == nil {
		deps.Nucleo = Eco{}
	}
	if deps.Ejecutor == nil {
		deps.Ejecutor = EjecutorPendiente{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Servidor{
		cfg:      cfg,
		nucleo:   deps.Nucleo,
		corridas: nuevoCorridas(ctx, cfg.DatosDir, deps.Ejecutor),
		cancel:   cancel,
	}
	s.mux = s.rutas()
	return s, nil
}

// Handler devuelve las rutas del contrato, todas bajo /v1 (servers.url del
// OpenAPI ya incluye el prefijo).
func (s *Servidor) Handler() http.Handler { return s.mux }

// Cerrar cancela las corridas y libera sus directorios de trabajo. Se llama
// antes de http.Server.Shutdown para que un SIGTERM no deje un job colgado.
func (s *Servidor) Cerrar() error {
	s.cancel()
	s.corridas.espera()
	return nil
}

func (s *Servidor) rutas() *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("GET /v1/health", s.salud)
	m.HandleFunc("POST /v1/run", s.iniciaCorrida)
	m.HandleFunc("GET /v1/run/{id}", s.estadoCorrida)
	m.HandleFunc("GET /v1/output/{archivo}", s.archivoSalida)
	m.HandleFunc("POST /v1/anonymize", s.anonimiza)
	m.HandleFunc("GET /v1/entities", s.entidades)
	return m
}

type respuestaSalud struct {
	Status  string   `json:"status"`
	Team    string   `json:"team"`
	Version string   `json:"version"`
	Models  []string `json:"models"`
}

func (s *Servidor) salud(w http.ResponseWriter, r *http.Request) {
	modelos := s.cfg.Modelos
	if modelos == nil {
		modelos = []string{}
	}
	respondeJSON(w, http.StatusOK, respuestaSalud{
		Status: "ok", Team: s.cfg.Equipo, Version: s.cfg.Version, Models: modelos,
	})
}

type peticionRun struct {
	SourcesURL string `json:"sources_url"`
	Token      string `json:"token"`
}

// iniciaCorrida acepta {sources_url, token}. Es idempotente mientras hay una
// corrida activa: devuelve ese mismo job en vez de encolar una segunda.
func (s *Servidor) iniciaCorrida(w http.ResponseWriter, r *http.Request) {
	var peticion peticionRun
	decodificador := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decodificador.Decode(&peticion); err != nil {
		captura.Anota(r).Error = "cuerpo"
		respondeError(w, http.StatusBadRequest, "cuerpo inválido: se espera JSON con sources_url y token")
		return
	}
	if err := validaURLFuentes(peticion.SourcesURL); err != nil {
		captura.Anota(r).Error = "sources_url"
		respondeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if peticion.Token == "" {
		captura.Anota(r).Error = "token"
		respondeError(w, http.StatusBadRequest, "falta token del API de fuentes")
		return
	}
	cor := s.corridas.enCola(peticion.SourcesURL, peticion.Token)
	captura.Anota(r).Job = cor.id
	respondeJSON(w, http.StatusAccepted, map[string]string{"job_id": cor.id})
}

func validaURLFuentes(valor string) error {
	u, err := url.Parse(valor)
	if err != nil {
		return errors.New("sources_url inválida")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("sources_url debe ser http o https")
	}
	if u.Host == "" {
		return errors.New("sources_url sin host")
	}
	return nil
}

func (s *Servidor) estadoCorrida(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	vista, ok := s.corridas.vista(id)
	if !ok {
		captura.Anota(r).Error = "job_desconocido"
		respondeError(w, http.StatusNotFound, "job desconocido")
		return
	}
	captura.Anota(r).Job = id
	respondeJSON(w, http.StatusOK, vista)
}

// tiposSalida fija los cuatro archivos y su content type.
var tiposSalida = map[string]string{
	"chats.jsonl":   "application/x-ndjson",
	"docs.jsonl":    "application/x-ndjson",
	"tables.jsonl":  "application/x-ndjson",
	"entities.json": "application/json",
}

func (s *Servidor) archivoSalida(w http.ResponseWriter, r *http.Request) {
	nombre := r.PathValue("archivo")
	if _, ok := tiposSalida[nombre]; !ok {
		captura.Anota(r).Error = "archivo_desconocido"
		respondeError(w, http.StatusNotFound, "archivo desconocido")
		return
	}
	s.sirveSalida(w, r, nombre)
}

// entidades sirve el entities.json de la última corrida terminada, tal cual
// quedó en el volumen.
func (s *Servidor) entidades(w http.ResponseWriter, r *http.Request) {
	s.sirveSalida(w, r, "entities.json")
}

func (s *Servidor) sirveSalida(w http.ResponseWriter, r *http.Request, nombre string) {
	dir, ok := s.corridas.ultimaSalida()
	if !ok {
		captura.Anota(r).Error = "sin_corrida"
		respondeError(w, http.StatusNotFound, "sin corrida terminada")
		return
	}
	f, err := os.Open(filepath.Join(dir, nombre))
	if err != nil {
		captura.Anota(r).Error = "sin_archivo"
		respondeError(w, http.StatusNotFound, "la corrida no produjo ese archivo")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		captura.Anota(r).Error = "sin_archivo"
		respondeError(w, http.StatusInternalServerError, "no se pudo leer el archivo")
		return
	}
	w.Header().Set("Content-Type", tiposSalida[nombre])
	http.ServeContent(w, r, nombre, info.ModTime(), f)
}

func respondeJSON(w http.ResponseWriter, codigo int, valor any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	if err := json.NewEncoder(w).Encode(valor); err != nil {
		fmt.Fprintf(os.Stderr, "servidor: escribe respuesta: %v\n", err)
	}
}

func respondeError(w http.ResponseWriter, codigo int, mensaje string) {
	respondeJSON(w, codigo, map[string]string{"error": mensaje})
}
