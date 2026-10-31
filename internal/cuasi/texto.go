package cuasi

import (
	"crypto/sha256"
	"encoding/binary"
	"strings"
	"sync"
	"unicode"

	"github.com/game013/frontier-hackaton-31102026/internal/catalogo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
)

// disparadoresOcupacion son las palabras que convierten un oficio del catálogo
// en la ocupación de una persona: «soy fotógrafa», «es contadora» (la
// descripción de un agente), «trabaja como administradora», «nuestra gerente de
// riesgo». Un oficio sin disparador («la panadera», «en el portal») es un
// nombre común y se queda.
var disparadoresOcupacion = conjunto(
	"soy", "es", "eres", "era", "somos", "como", "siendo",
	"nuestro", "nuestra", "nuestros", "nuestras", "mi", "mis", "su", "sus", "tu", "tus",
)

// conectores pueden ir entre el disparador y la ocupación: «es nuestro primer
// analista de operaciones».
var conectores = conjunto(
	"nuestro", "nuestra", "nuestros", "nuestras", "primer", "primero", "primera",
	"el", "la", "los", "las", "un", "una", "de", "del", "nuevo", "nueva",
)

// articulos acompañan al barrio: «vivo en El Prado».
var articulos = conjunto("el", "la", "los", "las")

// verbosVivienda atan el barrio a la casa de una persona: «vive en Castilla».
var verbosVivienda = conjunto(
	"vivo", "vive", "vives", "vivimos", "viven", "vivia", "resido", "reside", "residia",
)

// verbosEntrada anclan el año de la relación con la empresa: «entré con ustedes
// en 2025». Se comparan con tildes para no confundirlos con la preposición
// «entre».
var verbosEntrada = conjunto("entré", "entro", "entró", "entraron", "entramos", "entraste")

// anclaAnio es la preposición del año de vinculación: «llevo con ustedes desde
// 2021», «es cliente desde 2020». Por sí sola no basta: detrás tiene que haber
// una relación de la persona con la empresa (anclasDeRelacion).
const anclaAnio = "desde"

// anclasDeRelacion son las palabras que atan el año a la relación de la persona
// con la empresa: «llevo con ustedes desde», «vine desde», «me vinculé desde»,
// «soy cliente desde», «vinculada desde». Un «desde» suelto es una fecha de
// negocio («Kipu Pay opera desde 2020») y se conserva (D1, DECISIONS §1.4).
var anclasDeRelacion = conjunto(
	"llevo", "lleva", "llevas", "llevamos", "llevan", "llevaba", "llevaban", "llevabamos",
	"estoy", "estas", "esta", "estamos", "estan", "estaba", "estaban",
	"vine", "vino", "vinimos", "vinieron",
	"trabajo", "trabajas", "trabaja", "trabajamos", "trabajan", "trabaje", "trabajaba",
	"vinculado", "vinculada", "vinculados", "vinculadas",
	"cliente", "clienta", "clientes", "usuario", "usuaria", "usuarios", "usuarias",
)

// interlocutores es la empresa como interlocutor: «con nosotros desde 2021» ata
// el año aunque el verbo no quepa en la ventana.
var interlocutores = conjunto("nosotros", "nosotras", "ustedes", "ustees", "usted")

// puentesRelacion pueden ir entre el ancla y «desde»: «llevo con ustedes desde»,
// «cliente nuestro desde».
var puentesRelacion = conjunto(
	"con", "nuestro", "nuestra", "nosotros", "nosotras", "ustedes", "ustees", "usted",
)

// topeAncla acota cuántas palabras atrás se busca el ancla.
const topeAncla = 4

// haceUnosAnios generaliza el año de la relación (D9).
const haceUnosAnios = "hace unos años"

// maxPalabras es el largo máximo de un valor del catálogo: «estudiante de
// ingeniería», «instrumentador quirúrgico», «real de minas».
const maxPalabras = 4

// tramo es el rango de palabras [desde, hasta) de un texto que se reemplaza.
type tramo struct {
	desde, hasta int
	nuevo        string
}

// palabra es un token del texto con sus offsets en bytes sobre el original. La
// puntuación separa: el reemplazo es literal y el resto queda byte a byte igual
// (D6).
type palabra struct {
	texto       string
	inicio, fin int
}

// palabras parte el texto en tokens de letras y dígitos.
func palabras(texto string) []palabra {
	var out []palabra
	inicio := -1
	for i, r := range texto {
		esPalabra := esLetraODigito(r)
		switch {
		case esPalabra && inicio < 0:
			inicio = i
		case !esPalabra && inicio >= 0:
			out = append(out, palabra{texto: texto[inicio:i], inicio: inicio, fin: i})
			inicio = -1
		}
	}
	if inicio >= 0 {
		out = append(out, palabra{texto: texto[inicio:], inicio: inicio, fin: len(texto)})
	}
	return out
}

// esLetraODigito separa palabras con el criterio de Unicode, no con ASCII más
// «todo lo alto»: la puntuación no ASCII («…», «—») no es parte de la palabra, y
// sin eso «desde 2024…» no dejaría ver el año.
func esLetraODigito(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// une junta las palabras [desde, hasta) con un espacio, que es como el catálogo
// guarda las ocupaciones de varias palabras.
func une(ps []palabra, desde, hasta int) string {
	partes := make([]string, 0, hasta-desde)
	for _, p := range ps[desde:hasta] {
		partes = append(partes, p.texto)
	}
	return strings.Join(partes, " ")
}

// ocupacionesEn encuentra las ocupaciones del catálogo que el texto liga a una
// persona (tramo) y su sector (D9).
func ocupacionesEn(ps []palabra) []tramo {
	var out []tramo
	for i := 0; i < len(ps); {
		n, sector := ocupacionEn(ps, i)
		if n == 0 {
			i++
			continue
		}
		if ligadaAPersona(ps, i) {
			out = append(out, tramo{desde: i, hasta: i + n, nuevo: sector})
		}
		i += n
	}
	return out
}

// ocupacionEn busca la ocupación más larga del catálogo que empieza en i: en
// «estudiante de derecho» manda la de dos palabras, no «estudiante».
func ocupacionEn(ps []palabra, i int) (int, string) {
	tope := len(ps) - i
	if tope > maxPalabras {
		tope = maxPalabras
	}
	for n := tope; n >= 1; n-- {
		if sector, ok := catalogo.Sector(une(ps, i, i+n)); ok {
			return n, sector
		}
	}
	return 0, ""
}

// ligadaAPersona dice si la palabra i es la ocupación de alguien: mira hasta
// tres palabras atrás saltando conectores.
func ligadaAPersona(ps []palabra, i int) bool {
	for j := i - 1; j >= 0 && j >= i-3; j-- {
		clave := norm.Norm(ps[j].texto)
		if disparadoresOcupacion[clave] {
			return true
		}
		if !conectores[clave] {
			return false
		}
	}
	return false
}

// barrio es un tramo de texto que es un barrio del catálogo.
type barrio struct {
	tramo
	// vivienda marca «vivo en», que basta por sí solo para generalizar; «en»
	// genérico solo generaliza dentro de la plantilla (D9).
	vivienda bool
}

// barriosEn encuentra los barrios del catálogo escritos con mayúscula inicial
// detrás de «en»: los lugares del corpus van capitalizados y los nombres
// comunes no («en el portal de talento»).
func barriosEn(ps []palabra, preferida string) []barrio {
	var out []barrio
	for i := 0; i < len(ps); i++ {
		if !mayusculaInicial(ps[i].texto) {
			continue
		}
		n, ciudad, ok := barrioEn(ps, i, preferida)
		if !ok {
			continue
		}
		j := i - 1
		if j >= 0 && articulos[norm.Norm(ps[j].texto)] {
			j--
		}
		if j < 0 || norm.Norm(ps[j].texto) != "en" {
			continue
		}
		out = append(out, barrio{
			tramo:    tramo{desde: i, hasta: i + n, nuevo: ciudad},
			vivienda: j > 0 && verbosVivienda[norm.Norm(ps[j-1].texto)],
		})
		i += n - 1
	}
	return out
}

// barrioEn busca el barrio más largo del catálogo que empieza en i y devuelve su
// largo y la ciudad a la que se generaliza.
func barrioEn(ps []palabra, i int, preferida string) (int, string, bool) {
	tope := len(ps) - i
	if tope > maxPalabras {
		tope = maxPalabras
	}
	for n := tope; n >= 1; n-- {
		if ciudad, ok := ciudadDeBarrio(une(ps, i, i+n), preferida); ok {
			return n, ciudad, true
		}
	}
	return 0, "", false
}

func mayusculaInicial(s string) bool {
	for _, r := range s {
		return (r >= 'A' && r <= 'Z') || (r >= 0xC0 && r <= 0xDE && r != 0xD7)
	}
	return false
}

// aniosEn encuentra los años de la relación con la empresa: los que van detrás
// de un «desde» con relación delante o de «entró … en». Un año de negocio
// («julio de 2026», «Kipu Pay opera desde 2020») y el de un id interno
// («PQR-2025-0548») se quedan.
func aniosEn(texto string, ps []palabra) []tramo {
	var out []tramo
	for i, p := range ps {
		if !esAnio(texto, p) || !anclado(ps, i) {
			continue
		}
		out = append(out, tramo{desde: i, hasta: i + 1, nuevo: haceUnosAnios})
	}
	return out
}

func esAnio(texto string, p palabra) bool {
	if len(p.texto) != 4 {
		return false
	}
	anio := 0
	for _, r := range p.texto {
		if r < '0' || r > '9' {
			return false
		}
		anio = anio*10 + int(r-'0')
	}
	if anio < 1900 || anio > 2099 {
		return false
	}
	// Los ids internos llevan el año pegado con guion y se conservan.
	if p.inicio > 0 && texto[p.inicio-1] == '-' {
		return false
	}
	return p.fin >= len(texto) || texto[p.fin] != '-'
}

// anclado dice si el año es el de la relación de la persona con la empresa.
func anclado(ps []palabra, i int) bool {
	if i == 0 {
		return false
	}
	switch norm.Norm(ps[i-1].texto) {
	case anclaAnio:
		return relacionAntes(ps, i-1)
	case "en":
		for j := i - 2; j >= 0 && j >= i-6; j-- {
			if verbosEntrada[strings.ToLower(ps[j].texto)] {
				return true
			}
		}
	}
	return false
}

// relacionAntes mira hacia atrás desde la posición de «desde»: salta los puentes
// del habla («llevo con ustedes», «cliente nuestro») y devuelve true solo si
// encuentra el ancla. Cualquier otra palabra corta la búsqueda: sin relación
// delante, el año es de negocio y se conserva.
func relacionAntes(ps []palabra, desde int) bool {
	for j := desde - 1; j >= 0 && desde-j <= topeAncla; j-- {
		clave := norm.Norm(ps[j].texto)
		if anclasDeRelacion[clave] {
			return true
		}
		if clave == "con" && interlocutores[norm.Norm(ps[j+1].texto)] {
			return true
		}
		if !puentesRelacion[clave] {
			return false
		}
	}
	return false
}

var (
	unaVez           sync.Once
	ciudadesDeBarrio map[string][]string
)

// indiceDeBarrios arma una vez el índice inverso barrio → ciudades, en el orden
// alfabético de catalogo.Ciudades: un barrio puede repetirse entre ciudades
// («Castilla» en Bogotá y en Medellín).
func indiceDeBarrios() map[string][]string {
	unaVez.Do(func() {
		ciudadesDeBarrio = map[string][]string{}
		for _, ciudad := range catalogo.Ciudades() {
			lista, _ := catalogo.Barrios(ciudad)
			for _, barrio := range lista {
				clave := norm.Norm(barrio)
				ciudadesDeBarrio[clave] = append(ciudadesDeBarrio[clave], ciudad)
			}
		}
	})
	return ciudadesDeBarrio
}

// ciudadDeBarrio generaliza un barrio a su ciudad (D9). Si el nombre existe en
// varias ciudades manda la ciudad de la persona, cuando se conoce, y si no la
// primera en orden alfabético: la respuesta es estable entre corridas.
func ciudadDeBarrio(barrio, preferida string) (string, bool) {
	lista, ok := indiceDeBarrios()[norm.Norm(barrio)]
	if !ok || len(lista) == 0 {
		return "", false
	}
	if preferida != "" {
		for _, ciudad := range lista {
			if norm.Norm(ciudad) == norm.Norm(preferida) {
				return ciudad, true
			}
		}
	}
	return lista[0], true
}

// indiceDeLlave deriva un índice estable del hash de una llave, sin azar ni
// orden de entrada.
func indiceDeLlave(llave string) int {
	suma := sha256.Sum256([]byte(llave))
	return int(binary.BigEndian.Uint32(suma[:4]) % 1000)
}

func conjunto(valores ...string) map[string]bool {
	out := make(map[string]bool, len(valores))
	for _, v := range valores {
		out[v] = true
	}
	return out
}
