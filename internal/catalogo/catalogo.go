// Package catalogo reúne los datos embebidos con que la fábrica de seudónimos
// inventa la identidad falsa de una persona (D18): nombres de pila colombianos
// por género, unisex y extranjeros; apellidos con tildes; dominios públicos de
// correo; barrios por ciudad; y el mapa ocupación → uno de los 13 sectores
// (D9). Los datos viajan compilados con go:embed: el binario no lee archivos.
//
// Ningún valor coincide con una forma real del corpus. Los nombres y apellidos
// no comparten ni un token con los nombres reales, así que ninguna combinación
// de nombre y apellido del catálogo puede reproducir un nombre completo del
// corpus; la prueba de colisión lo comprueba contra `fuentes/`. La fábrica
// (C13) sigue siendo la que descarta las formas reales de cada persona.
//
// Todo el catálogo es de solo lectura y determinista: los índices que reciben
// las funciones salen de la semilla HMAC de la persona (D4, D22); el catálogo
// no conoce llaves ni ordena por azar.
package catalogo

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

//go:embed datos/*.json
var archivos embed.FS

// Origen del nombre de pila. Un perfil extranjero conserva el origen de la
// persona real, para no darle a un inversionista un nombre colombiano.
type Origen string

const (
	OrigenColombiano Origen = "colombiano"
	OrigenExtranjero Origen = "extranjero"
)

// catalogo es el contenido de datos/*.json ya indexado para buscar sin tildes.
type catalogo struct {
	// nombres va de origen normalizado a grupo normalizado a nombres de pila.
	nombres map[string]map[string][]string
	// apellidos y dominios se recorren por índice.
	apellidos []string
	dominios  []string
	// ciudades va en orden alfabético; barrios indexa por ciudad normalizada.
	ciudades []string
	barrios  map[string][]string
	// sectores son los 13 sectores de D9; ocupaciones va de ocupación
	// normalizada a sector.
	sectores    []string
	ocupaciones map[string]string
}

// grupo traduce el género a la lista de nombres de pila que le corresponde.
// Indeterminado comparte el catálogo unisex (D18, D27).
func grupo(g modelo.Genero) string {
	switch g {
	case modelo.GeneroM:
		return "M"
	case modelo.GeneroF:
		return "F"
	default:
		return "unisex"
	}
}

var (
	unaVez sync.Once
	datos  *catalogo
)

// carga lee los datos una sola vez. Un error de datos embebidos es un error de
// programación del binario, no una condición de ejecución: no hay a quién
// devolverle el error.
func carga() *catalogo {
	unaVez.Do(func() { datos = leer() })
	return datos
}

func leer() *catalogo {
	c := &catalogo{
		nombres:     map[string]map[string][]string{},
		barrios:     map[string][]string{},
		ocupaciones: map[string]string{},
	}
	decodifica("datos/nombres.json", &c.nombres)
	decodifica("datos/apellidos.json", &c.apellidos)
	decodifica("datos/dominios.json", &c.dominios)
	var sectores struct {
		Sectores    []string          `json:"sectores"`
		Ocupaciones map[string]string `json:"ocupaciones"`
	}
	decodifica("datos/sectores.json", &sectores)
	c.sectores = sectores.Sectores
	for ocupacion, sector := range sectores.Ocupaciones {
		clave := normaliza(ocupacion)
		if _, repetida := c.ocupaciones[clave]; repetida {
			panic(fmt.Errorf("catalogo: datos/sectores.json: la ocupación %q se repite al normalizar", ocupacion))
		}
		c.ocupaciones[clave] = sector
	}
	var barrios map[string][]string
	decodifica("datos/barrios.json", &barrios)
	for ciudad, lista := range barrios {
		c.ciudades = append(c.ciudades, ciudad)
		c.barrios[normaliza(ciudad)] = lista
	}
	sort.Strings(c.ciudades)
	return c
}

// decodifica lee un JSON embebido y rechaza claves que el tipo no espera: un
// nombre mal escrito en los datos tiene que romper en las pruebas, no perderse.
func decodifica(nombre string, destino any) {
	crudo, err := archivos.ReadFile(nombre)
	if err != nil {
		panic(fmt.Errorf("catalogo: %s: %w", nombre, err))
	}
	dec := json.NewDecoder(bytes.NewReader(crudo))
	dec.DisallowUnknownFields()
	if err := dec.Decode(destino); err != nil {
		panic(fmt.Errorf("catalogo: %s: %w", nombre, err))
	}
}

// normaliza baja a minúsculas, quita tildes y colapsa espacios: la misma
// normalización con que el jurado compara (D5).
func normaliza(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'á', 'à', 'â', 'ä', 'ã':
			r = 'a'
		case 'é', 'è', 'ê', 'ë':
			r = 'e'
		case 'í', 'ì', 'î', 'ï':
			r = 'i'
		case 'ó', 'ò', 'ô', 'ö', 'õ':
			r = 'o'
		case 'ú', 'ù', 'û', 'ü':
			r = 'u'
		case 'ñ':
			r = 'n'
		case 'ç':
			r = 'c'
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// enIndice recorre una lista por índice, con vuelta y con índices negativos
// desde el final. Devuelve "" si la lista está vacía.
func enIndice(lista []string, indice int) string {
	if len(lista) == 0 {
		return ""
	}
	i := indice % len(lista)
	if i < 0 {
		i += len(lista)
	}
	return lista[i]
}

// NombresPila devuelve los nombres de pila del grupo. La lista es de solo
// lectura: no se debe modificar. Un origen vacío vale por colombiano; un grupo
// inexistente devuelve nil.
func NombresPila(g modelo.Genero, o Origen) []string {
	origen := string(o)
	if origen == "" {
		origen = string(OrigenColombiano)
	}
	grupos, ok := carga().nombres[origen]
	if !ok {
		return nil
	}
	return grupos[grupo(g)]
}

// NombrePila devuelve el nombre de pila que toca en el índice dentro del grupo
// (módulo el tamaño; el índice negativo cuenta desde el final). La estructura
// se conserva pidiendo un nombre por cada nombre de pila real. Devuelve "" si
// el grupo está vacío.
func NombrePila(g modelo.Genero, o Origen, indice int) string {
	return enIndice(NombresPila(g, o), indice)
}

// Apellidos devuelve la lista de apellidos, de solo lectura.
func Apellidos() []string { return carga().apellidos }

// Apellido devuelve el apellido del índice, con la misma vuelta que NombrePila.
func Apellido(indice int) string { return enIndice(carga().apellidos, indice) }

// Dominios devuelve los dominios públicos de correo, de solo lectura.
func Dominios() []string { return carga().dominios }

// Dominio devuelve el dominio público del índice.
func Dominio(indice int) string { return enIndice(carga().dominios, indice) }

// Ciudades devuelve, en orden alfabético, las ciudades con barrios en el
// catálogo: las del corpus y los municipios del área metropolitana que el
// corpus usa como barrio.
func Ciudades() []string { return carga().ciudades }

// Barrios devuelve los barrios de una ciudad. La búsqueda ignora mayúsculas,
// tildes y espacios de más; la segunda salida es false si la ciudad no está.
func Barrios(ciudad string) ([]string, bool) {
	lista, ok := carga().barrios[normaliza(ciudad)]
	return lista, ok
}

// Barrio devuelve el barrio del índice en esa ciudad.
func Barrio(ciudad string, indice int) (string, bool) {
	lista, ok := Barrios(ciudad)
	if !ok {
		return "", false
	}
	return enIndice(lista, indice), true
}

// BarrioAlterno devuelve otro barrio de la misma ciudad, nunca el actual: el
// barrio real se reemplaza por uno de la misma ciudad y el par original no
// sobrevive en la salida (D9). La segunda salida es false si la ciudad no está
// o no tiene otro barrio.
func BarrioAlterno(ciudad, actual string, indice int) (string, bool) {
	lista, ok := Barrios(ciudad)
	if !ok {
		return "", false
	}
	clave := normaliza(actual)
	otros := make([]string, 0, len(lista))
	for _, barrio := range lista {
		if normaliza(barrio) != clave {
			otros = append(otros, barrio)
		}
	}
	if len(otros) == 0 {
		return "", false
	}
	return enIndice(otros, indice), true
}

// Sectores devuelve los 13 sectores de D9, de solo lectura. Son los mismos
// valores neutros de la salida ideal ("profesional de la salud").
func Sectores() []string { return carga().sectores }

// Sector generaliza una ocupación a su sector (D9). Ignora mayúsculas, tildes y
// espacios de más; la segunda salida es false si la ocupación no está en el
// mapa, para que el pipeline no invente un sector.
func Sector(ocupacion string) (string, bool) {
	sector, ok := carga().ocupaciones[normaliza(ocupacion)]
	return sector, ok
}
