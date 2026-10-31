// Comando monitor: una pasada sobre /record_index del API de fuentes para
// avisar cuando el corpus crece. Es para cron o un servicio externo, no un
// bucle: cada ejecución deja el ETag y el último índice en --datos y sale.
//
//	go run ./cmd/monitor --datos tmp/monitor
//
// El cliente del API está en internal/fuentes, pero guarda los ETag en memoria
// del proceso; aquí hace falta el ETag de la pasada anterior, así que el monitor
// hace su única petición: GET /record_index con If-None-Match y timeout.
//
// Códigos de salida:
//
//	0  sin cambios: el API respondió 304, o es la primera lectura (línea base)
//	1  el corpus cambió: hay índice nuevo y se reportó el delta por fuente
//	2  error: red, HTTP, estado local ilegible o faltan la URL y el token
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// codigoSinCambios es 304 o primera lectura: nada creció respecto de la
	// copia guardada.
	codigoSinCambios = 0
	// codigoCambio avisa al cron que hay registros nuevos que procesar.
	codigoCambio = 1
	// codigoError cubre red, HTTP y estado local.
	codigoError = 2

	apiPorDefecto     = "https://reto-pii-fuentes-production.up.railway.app/v1"
	timeoutPorDefecto = 30 * time.Second

	// archivoIndice guarda el último /record_index tal como llegó, para que el
	// delta de la próxima pasada sea contra la copia buena.
	archivoIndice = "record_index.json"
	// archivoETag guarda su ETag, que viaja como If-None-Match.
	archivoETag = "record_index.etag"

	// cuerpoEnError recorta la respuesta fallida: ni cuerpos grandes ni datos
	// ajenos en los logs.
	cuerpoEnError = 200
)

func main() {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	api := fs.String("api", "", "raíz del API de fuentes con su /v1 (vacío: SOURCES_API_URL o "+apiPorDefecto+")")
	token := fs.String("token", "", "token del API de fuentes (vacío: TOKEN). Nunca se imprime")
	datos := fs.String("datos", "", "carpeta del ETag y del último índice (vacío: /data si existe, si no tmp/monitor)")
	timeout := fs.Duration("timeout", timeoutPorDefecto, "tope de la petición al API")
	fs.Usage = func() {
		salida := fs.Output()
		fmt.Fprint(salida, "monitor: una pasada sobre record_index; avisa si el corpus creció.\n\n"+
			"Uso:\n  monitor --datos tmp/monitor\n\n"+
			"Códigos de salida:\n"+
			"  0  sin cambios (304) o primera lectura que guarda la línea base\n"+
			"  1  el corpus cambió: índice nuevo guardado y delta por fuente\n"+
			"  2  error de red, HTTP o estado local (o faltan la URL y el token)\n\n"+
			"Opciones:\n")
		fs.PrintDefaults()
	}
	fs.Parse(os.Args[1:])

	os.Exit(corre(opciones{api: *api, token: *token, datos: *datos, timeout: *timeout}, os.Stdout, os.Stderr))
}

// opciones es lo que llega por flags; lo que falte se toma del entorno.
type opciones struct {
	api     string
	token   string
	datos   string
	timeout time.Duration
}

// corre hace la pasada completa y devuelve el código de salida.
func corre(o opciones, salida, errores io.Writer) int {
	// Mismo .env que cmd/pipeline: local se define TOKEN ahí, y lo que ya está
	// en el entorno no se pisa.
	cargaEntorno(".env")

	base := o.api
	if base == "" {
		base = os.Getenv("SOURCES_API_URL")
	}
	if base == "" {
		base = apiPorDefecto
	}
	token := o.token
	if token == "" {
		token = os.Getenv("TOKEN")
	}
	if token == "" {
		fmt.Fprintln(errores, "monitor: falta el token del API de fuentes: usa --token o la variable TOKEN")
		return codigoError
	}
	if o.timeout <= 0 {
		fmt.Fprintf(errores, "monitor: el timeout tiene que ser positivo, llegó %s\n", o.timeout)
		return codigoError
	}
	base, err := normalizaBase(base)
	if err != nil {
		fmt.Fprintf(errores, "monitor: %v\n", err)
		return codigoError
	}
	dir := o.datos
	if dir == "" {
		dir = datosPorDefecto()
	}

	antes, err := leeEstado(dir)
	if err != nil {
		fmt.Fprintf(errores, "monitor: %v\n", err)
		return codigoError
	}

	ctx, cancelar := context.WithTimeout(context.Background(), o.timeout)
	defer cancelar()
	lectura, err := pideIndice(ctx, base, token, antes.etag, o.timeout)
	if err != nil {
		fmt.Fprintf(errores, "monitor: %v\n", err)
		return codigoError
	}

	if lectura.sinCambios {
		fmt.Fprintf(salida, "monitor: sin cambios (ETag %s)\n", antes.etag)
		return codigoSinCambios
	}
	ahora, err := parseaIndice(lectura.crudo)
	if err != nil {
		fmt.Fprintf(errores, "monitor: %v\n", err)
		return codigoError
	}
	if lectura.etag == "" {
		fmt.Fprintln(errores, "monitor: el API no mandó ETag: la próxima pasada no podrá preguntar If-None-Match")
	}

	// Primera pasada: no hay contra qué comparar, así que solo se guarda.
	if !antes.hay {
		if err := guardaEstado(dir, lectura.crudo, lectura.etag); err != nil {
			fmt.Fprintf(errores, "monitor: %v\n", err)
			return codigoError
		}
		fmt.Fprintf(salida, "monitor: línea base guardada (ETag %s)\n", sinVacio(lectura.etag))
		fmt.Fprintf(salida, "chats: %d\ndocs: %d\ntables: %d\n", len(ahora.Chats), len(ahora.Docs), len(ahora.Tablas))
		return codigoSinCambios
	}

	// Un 200 con el índice idéntico es un servidor que no respetó
	// If-None-Match: el corpus no creció.
	if bytes.Equal(lectura.crudo, antes.crudo) {
		fmt.Fprintln(salida, "monitor: sin cambios (el índice es idéntico al guardado)")
		return codigoSinCambios
	}

	viejo, err := parseaIndice(antes.crudo)
	if err != nil {
		fmt.Fprintf(errores, "monitor: el índice guardado en %s no sirve: %v\n", dir, err)
		return codigoError
	}
	if err := guardaEstado(dir, lectura.crudo, lectura.etag); err != nil {
		fmt.Fprintf(errores, "monitor: %v\n", err)
		return codigoError
	}
	fmt.Fprintf(salida, "monitor: el corpus cambió (ETag %s)\n", sinVacio(lectura.etag))
	fmt.Fprintln(salida, lineaDelta("chats", len(viejo.Chats), len(ahora.Chats)))
	fmt.Fprintln(salida, lineaDelta("docs", len(viejo.Docs), len(ahora.Docs)))
	fmt.Fprintln(salida, lineaDelta("tables", len(viejo.Tablas), len(ahora.Tablas)))
	return codigoCambio
}

// lectura es una respuesta de /record_index.
type lectura struct {
	crudo      []byte
	etag       string
	sinCambios bool
}

// pideIndice consulta /record_index con el ETag de la pasada anterior. Un 304
// llega como sinCambios, o sea que la copia guardada sigue vigente.
func pideIndice(ctx context.Context, base, token, etag string, timeout time.Duration) (lectura, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/record_index", nil)
	if err != nil {
		return lectura{}, fmt.Errorf("arma la petición a /record_index: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	cliente := &http.Client{Timeout: timeout}
	resp, err := cliente.Do(req)
	if err != nil {
		return lectura{}, fmt.Errorf("consulta /record_index: %w", err)
	}
	defer resp.Body.Close()
	crudo, err := io.ReadAll(resp.Body)
	if err != nil {
		return lectura{}, fmt.Errorf("descarga /record_index: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusNotModified:
		return lectura{etag: etag, sinCambios: true}, nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return lectura{crudo: crudo, etag: resp.Header.Get("ETag")}, nil
	default:
		return lectura{}, fmt.Errorf("el API respondió %d a GET /record_index: %s", resp.StatusCode, recorta(crudo))
	}
}

// recorta limita el detalle de una respuesta fallida.
func recorta(cuerpo []byte) string {
	if len(cuerpo) > cuerpoEnError {
		cuerpo = cuerpo[:cuerpoEnError]
	}
	return strings.TrimSpace(string(cuerpo))
}

// normalizaBase valida la raíz del API y le quita la barra final.
func normalizaBase(base string) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("raíz del API inválida %q: %w", base, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("raíz del API inválida %q: hace falta http(s)://host/v1", base)
	}
	return base, nil
}

// indice es /record_index: los ids que la salida debe contener, por fuente.
// Aquí solo se cuentan.
type indice struct {
	Chats  []string `json:"chats"`
	Docs   []string `json:"docs"`
	Tablas []string `json:"tables"`
}

func parseaIndice(crudo []byte) (indice, error) {
	var i indice
	if err := json.Unmarshal(crudo, &i); err != nil {
		return indice{}, fmt.Errorf("/record_index no es el JSON esperado: %w", err)
	}
	return i, nil
}

// estado es lo que dejó la pasada anterior.
type estado struct {
	// hay distingue la primera lectura de un corpus sin cambios.
	hay   bool
	crudo []byte
	etag  string
}

// leeEstado carga el índice y el ETag guardados. Sin archivos todavía es una
// primera lectura, no un error.
func leeEstado(dir string) (estado, error) {
	crudo, err := os.ReadFile(filepath.Join(dir, archivoIndice))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return estado{}, nil
	case err != nil:
		return estado{}, fmt.Errorf("lee el índice guardado en %s: %w", dir, err)
	}
	listo := estado{hay: true, crudo: crudo}
	etag, err := os.ReadFile(filepath.Join(dir, archivoETag))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return estado{}, fmt.Errorf("lee el ETag guardado en %s: %w", dir, err)
	default:
		listo.etag = strings.TrimSpace(string(etag))
	}
	return listo, nil
}

// guardaEstado deja el índice y su ETag para la próxima pasada. Sin ETag borra
// el viejo: reenviar uno que ya no corresponde daría 304 falsos.
func guardaEstado(dir string, crudo []byte, etag string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("prepara %s: %w", dir, err)
	}
	if err := escribeAtomico(filepath.Join(dir, archivoIndice), crudo); err != nil {
		return fmt.Errorf("guarda el índice en %s: %w", dir, err)
	}
	rutaETag := filepath.Join(dir, archivoETag)
	if etag == "" {
		if err := os.Remove(rutaETag); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("borra %s: %w", rutaETag, err)
		}
		return nil
	}
	if err := escribeAtomico(rutaETag, []byte(etag+"\n")); err != nil {
		return fmt.Errorf("guarda el ETag en %s: %w", dir, err)
	}
	return nil
}

// escribeAtomico deja el archivo completo o no lo toca: un corte a mitad no
// puede dejar un índice a medias como referencia de la próxima pasada.
func escribeAtomico(ruta string, contenido []byte) error {
	temporal, err := os.CreateTemp(filepath.Dir(ruta), ".monitor-*")
	if err != nil {
		return err
	}
	if _, err := temporal.Write(contenido); err != nil {
		temporal.Close()
		os.Remove(temporal.Name())
		return err
	}
	if err := temporal.Close(); err != nil {
		os.Remove(temporal.Name())
		return err
	}
	return os.Rename(temporal.Name(), ruta)
}

// datosPorDefecto usa el volumen de Railway si está montado y tmp/monitor en
// local: el estado nunca entra al repo.
func datosPorDefecto() string {
	if info, err := os.Stat("/data"); err == nil && info.IsDir() {
		return filepath.Join("/data", "monitor")
	}
	return filepath.Join("tmp", "monitor")
}

// lineaDelta dice cuántos registros había, cuántos hay y cuánto cambió.
func lineaDelta(fuente string, antes, ahora int) string {
	switch d := ahora - antes; {
	case d > 0:
		return fmt.Sprintf("%s: %d → %d (+%d)", fuente, antes, ahora, d)
	case d < 0:
		return fmt.Sprintf("%s: %d → %d (%d)", fuente, antes, ahora, d)
	default:
		return fmt.Sprintf("%s: %d → %d (igual)", fuente, antes, ahora)
	}
}

// sinVacio deja el texto legible cuando el API no mandó ETag.
func sinVacio(valor string) string {
	if valor == "" {
		return "sin ETag"
	}
	return valor
}

// cargaEntorno lee un .env local si existe. No pisa variables ya definidas y
// nunca imprime valores (regla 8 de PROGRESS.md). Es el mismo comportamiento
// que cmd/pipeline.
func cargaEntorno(ruta string) {
	archivo, err := os.Open(ruta)
	if err != nil {
		return
	}
	defer archivo.Close()
	sc := bufio.NewScanner(archivo)
	for sc.Scan() {
		linea := strings.TrimSpace(sc.Text())
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		clave, valor, ok := strings.Cut(linea, "=")
		if !ok {
			continue
		}
		clave = strings.TrimSpace(clave)
		valor = strings.Trim(strings.TrimSpace(valor), `"'`)
		if _, definida := os.LookupEnv(clave); definida || clave == "" {
			continue
		}
		os.Setenv(clave, valor)
	}
}
