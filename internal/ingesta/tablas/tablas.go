// Package tablas convierte las cuatro tablas del corpus (clientes.csv,
// transacciones.csv, pqrs.xml y empleados.xml) en registros canónicos de
// modelo.TableRecord: id `<tabla>:<llave>` y todas las columnas originales como
// texto, sin reinterpretar números ni fechas.
//
// Un archivo ausente se omite: la muestra solo trae clientes.csv y el API de
// fuentes puede servir menos tablas según el momento. Si no hay ninguna, es
// error, para que un directorio equivocado no pase por corpus vacío.
package tablas

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// codificacion es el charset con el que la fuente declara su archivo.
type codificacion int

const (
	codificacionUTF8 codificacion = iota
	codificacionLatin1
)

// tablaCSV describe un archivo separado por un carácter con una columna llave.
type tablaCSV struct {
	nombre       string
	archivo      string
	codificacion codificacion
	separador    rune
	llave        string
}

// tablasCSV va en el orden de fuentes/tables/index.json.
var tablasCSV = []tablaCSV{
	{nombre: "clientes", archivo: "clientes.csv", codificacion: codificacionLatin1, separador: ';', llave: "cliente_id"},
	{nombre: "transacciones", archivo: "transacciones.csv", codificacion: codificacionUTF8, separador: ',', llave: "tx_id"},
}

// tablaXML describe un archivo donde cada registro es un elemento plano con un
// atributo que hace de llave.
type tablaXML struct {
	nombre   string
	archivo  string
	elemento string
	llave    string
}

var tablasXML = []tablaXML{
	{nombre: "pqrs", archivo: "pqrs.xml", elemento: "pqr", llave: "radicado"},
	{nombre: "empleados", archivo: "empleados.xml", elemento: "empleado", llave: "id"},
}

// Leer devuelve los registros de las tablas presentes en el directorio, en el
// orden clientes, transacciones, pqrs, empleados.
func Leer(directorio string) ([]modelo.TableRecord, error) {
	var registros []modelo.TableRecord
	for _, tabla := range tablasCSV {
		ruta := filepath.Join(directorio, tabla.archivo)
		if !existe(ruta) {
			continue
		}
		leidos, err := leerTablaCSV(ruta, tabla)
		if err != nil {
			return nil, err
		}
		registros = append(registros, leidos...)
	}
	for _, tabla := range tablasXML {
		ruta := filepath.Join(directorio, tabla.archivo)
		if !existe(ruta) {
			continue
		}
		leidos, err := leerTablaXML(ruta, tabla)
		if err != nil {
			return nil, err
		}
		registros = append(registros, leidos...)
	}
	if len(registros) == 0 {
		return nil, fmt.Errorf("%s: no hay ninguna tabla conocida", directorio)
	}
	return registros, nil
}

func existe(ruta string) bool {
	_, err := os.Stat(ruta)
	return err == nil
}

// leerTablaCSV lee un CSV completo: la primera fila es la cabecera y cada fila
// siguiente un registro. Las filas cortas se completan con "" y las que traen
// campos de más son error, para no perder datos en silencio.
func leerTablaCSV(ruta string, tabla tablaCSV) ([]modelo.TableRecord, error) {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return nil, err
	}
	lector := csv.NewReader(strings.NewReader(decodificar(datos, tabla.codificacion)))
	lector.Comma = tabla.separador
	lector.FieldsPerRecord = -1

	filas, err := lector.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ruta, err)
	}
	if len(filas) == 0 {
		return nil, fmt.Errorf("%s: sin cabecera", ruta)
	}
	cabecera := filas[0]
	if !tieneColumna(cabecera, tabla.llave) {
		return nil, fmt.Errorf("%s: la cabecera no trae la columna llave %q", ruta, tabla.llave)
	}

	registros := make([]modelo.TableRecord, 0, len(filas)-1)
	for i, fila := range filas[1:] {
		if len(fila) > len(cabecera) {
			if sobraAlgo(fila[len(cabecera):]) {
				return nil, fmt.Errorf("%s: la fila %d trae %d campos y la cabecera %d", ruta, i+2, len(fila), len(cabecera))
			}
			fila = fila[:len(cabecera)]
		}
		columnas := make(map[string]string, len(cabecera))
		for j, columna := range cabecera {
			if j < len(fila) {
				columnas[columna] = fila[j]
				continue
			}
			columnas[columna] = ""
		}
		llave := columnas[tabla.llave]
		if llave == "" {
			return nil, fmt.Errorf("%s: la fila %d no trae %q", ruta, i+2, tabla.llave)
		}
		registros = append(registros, modelo.TableRecord{
			ID:      tabla.nombre + ":" + llave,
			Table:   tabla.nombre,
			Columns: columnas,
		})
	}
	return registros, nil
}

// decodificar pasa los bytes del archivo a UTF-8 y le quita la marca de orden
// de bytes si viene.
func decodificar(datos []byte, cod codificacion) string {
	datos = bytes.TrimPrefix(datos, marcaOrdenBytes)
	if cod == codificacionLatin1 {
		// ISO-8859-1 mapea cada byte a la runa del mismo valor.
		runas := make([]rune, len(datos))
		for i, b := range datos {
			runas[i] = rune(b)
		}
		return string(runas)
	}
	return string(datos)
}

var marcaOrdenBytes = []byte{0xEF, 0xBB, 0xBF}

func tieneColumna(cabecera []string, columna string) bool {
	for _, nombre := range cabecera {
		if nombre == columna {
			return true
		}
	}
	return false
}

func sobraAlgo(campos []string) bool {
	for _, campo := range campos {
		if campo != "" {
			return true
		}
	}
	return false
}
