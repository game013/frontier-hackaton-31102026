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

// verboTrabajo ata el barrio al trabajo de la propia persona: «trabajo cerca, en
// Granada» (sample_08). Solo la primera persona del singular: «trabajaba» y
// «trabajé» se confunden con la tercera persona y el lugar se conserva (D1).
const verboTrabajo = "trabajo"

// antesDeTrabajo convierten «trabajo» en sustantivo: «el trabajo en Granada»,
// «su lugar de trabajo en El Prado»; detrás de ellas no ata barrio.
var antesDeTrabajo = conjunto(
	"el", "la", "los", "las", "un", "una", "mi", "mis", "su", "sus", "tu", "tus",
	"de", "del", "nuestro", "nuestra",
)

// locativosDeCosa atan el lugar a una cosa, no a una persona: la sede «queda»,
// la oficina «está», la empresa «opera» o «abrió» en un barrio. Ese barrio es el
// de la empresa y se conserva (D1, CE-41).
var locativosDeCosa = conjunto("queda", "quedan", "esta", "estan", "opera", "abrio", "encuentra")

// sujetosDeCosa nombran lo que no es una persona detrás de un locativo: «en la
// oficina en Buenos Aires» (CE-41).
var sujetosDeCosa = conjunto("sede", "oficina", "empresa", "compania")

// topeBarrio acota cuántas palabras atrás se busca el verbo que ata el barrio.
const topeBarrio = 5

// formasVinculado nombran la relación de la persona sin verbo propio («vinculada
// en 2023», «vinculado desde 2021»). Las usan los dos anclajes del año y también
// cuentan como sustantivo de persona, así que se declaran una sola vez.
var formasVinculado = conjunto("vinculado", "vinculada", "vinculados", "vinculadas")

// verbosEntrada anclan el año de la relación con la empresa: «entré con ustedes
// en 2025», «vinculada en 2023» (CE-40). Se comparan con tildes para no
// confundirlos con la preposición «entre».
var verbosEntrada = union(
	formasVinculado,
	conjunto("entré", "entro", "entró", "entraron", "entramos", "entraste"),
)

// anclasPersona presentan a una persona sin verbo delante: el acta de un comité
// habla de «la afectada» y a continuación le atribuye el oficio (CE-40).
var anclasPersona = conjunto("afectada", "afectado")

// anclaAnio es la preposición del año de vinculación: «llevo con ustedes desde
// 2021», «es cliente desde 2020». Por sí sola no basta: detrás tiene que haber
// una relación de la persona con la empresa (anclasPrimeraPersona,
// anclasTerceraPersona o sustantivosDePersona).
const anclaAnio = "desde"

// anclasPrimeraPersona atan el año a la relación de la persona sin más ayuda:
// «llevo con ustedes desde», «vine desde», «trabajo desde», «entré desde».
var anclasPrimeraPersona = union(
	conjunto(verboTrabajo, "llevo", "estoy", "vine"),
	conjunto("entré"),
)

// anclasTerceraPersona solo valen si en la ventana hay un sustantivo de persona
// o el puente «con nosotros/ustedes» (CE-43): «la clienta está desde» sí, «Kipu
// Pay trabaja desde 2019» no, que es una fecha de negocio (D1, DECISIONS §1.4).
var anclasTerceraPersona = conjunto(
	"lleva", "llevas", "llevamos", "llevan", "llevaba", "llevaban", "llevabamos",
	"estas", "esta", "estamos", "estan", "estaba", "estaban",
	"vino", "vinimos", "vinieron",
	"trabajas", "trabaja", "trabajamos", "trabajan", "trabaje", "trabajaba",
)

// sustantivosDePersona nombran a la persona en la ventana del año: «cliente
// desde», «usuaria desde», «vinculada desde». Atan el año por sí solos y
// habilitan las anclas en tercera persona.
var sustantivosDePersona = union(
	formasVinculado,
	conjunto("cliente", "clienta", "clientes", "usuario", "usuaria", "usuarios", "usuarias"),
)

// interlocutores es la empresa como interlocutor: «con nosotros desde 2021» ata
// el año aunque el verbo no quepa en la ventana. «ustees» no es una errata: es
// una forma del corpus tal cual («llevo con ustees desde 2023»), y por eso se
// queda en la lista.
var interlocutores = conjunto("nosotros", "nosotras", "ustedes", "ustees", "usted")

// puentesRelacion pueden ir entre el ancla y «desde»: «llevo con ustedes desde»,
// «cliente nuestro desde». Los interlocutores entran por referencia y no
// copiados: es el mismo concepto y se cambia en un solo sitio.
var puentesRelacion = union(interlocutores, conjunto("con", "nuestro", "nuestra"))

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
		if disparadoresOcupacion[clave] || anclasPersona[clave] {
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
	// deLaPersona marca el barrio donde vive o trabaja una persona; solo esos se
	// generalizan a su ciudad (D9, CE-41).
	deLaPersona bool
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
			tramo:       tramo{desde: i, hasta: i + n, nuevo: ciudad},
			deLaPersona: barrioDeLaPersona(ps, j),
		})
		i += n - 1
	}
	return out
}

// barrioDeLaPersona dice si el lugar que sigue al «en» de la posición j es el
// barrio de una persona y no el de una cosa (D1, CE-41). Lo es detrás de un
// verbo de vivienda («vivo en Castilla») o de un verbo de trabajo en primera
// persona dentro de la misma oración («trabajo cerca, en Granada»); si el verbo
// más cercano es locativo de una cosa («la sede queda en Envigado») o el sujeto
// es la sede, la oficina o la empresa, el lugar se conserva.
func barrioDeLaPersona(ps []palabra, j int) bool {
	for k := j - 1; k >= 0 && j-k <= topeBarrio; k-- {
		clave := norm.Norm(ps[k].texto)
		switch {
		case sujetosDeCosa[clave], locativosDeCosa[clave]:
			return false
		case verbosVivienda[clave]:
			return true
		case clave == verboTrabajo:
			return k == 0 || !antesDeTrabajo[norm.Norm(ps[k-1].texto)]
		}
	}
	return false
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

// articulosDelAnio pueden ir entre la preposición y el año: «cliente desde el
// 2021», «desde el año 2021», «desde del año 2021» (CE-44). Las claves van
// normalizadas: «año» llega como «ano».
var articulosDelAnio = conjunto("el", "del", "ano")

// anclado dice si el año es el de la relación de la persona con la empresa.
func anclado(ps []palabra, i int) bool {
	j := i - 1
	for n := 0; n < 2 && j >= 0 && articulosDelAnio[norm.Norm(ps[j].texto)]; n++ {
		j--
	}
	if j < 0 {
		return false
	}
	switch norm.Norm(ps[j].texto) {
	case anclaAnio:
		return relacionAntes(ps, j) || relacionDespues(ps, i)
	case "en":
		return entradaAntes(ps, j)
	}
	return false
}

// relacionDespues mira hasta cuatro palabras después del año: el habla invierte
// el orden y la relación de la persona va detrás («Desde 2021 soy cliente»,
// CE-44).
func relacionDespues(ps []palabra, i int) bool {
	for j := i + 1; j < len(ps) && j <= i+topeAncla; j++ {
		if sustantivosDePersona[norm.Norm(ps[j].texto)] {
			return true
		}
	}
	return false
}

// topeEntrada acota cuántas palabras atrás se busca el verbo de entrada: el
// mismo alcance que tenía el año cuando la búsqueda era un bucle en anclado.
const topeEntrada = 5

// entradaAntes busca el verbo de entrada detrás del «en» que precede al año
// (posición en). Si entre el verbo y el año hay otro «en» o un «de» que une otra
// frase, el año pertenece a una fecha de negocio y se conserva: «la resolución
// entró en vigencia en 2024» (CE-42). El precio es que «entró en la empresa en
// 2021» también se conserva: sin analizar la oración, la duda se resuelve a
// favor del dato (D1).
func entradaAntes(ps []palabra, en int) bool {
	for j := en - 1; j >= 0 && en-j <= topeEntrada; j-- {
		clave := norm.Norm(ps[j].texto)
		if clave == "en" || clave == "de" {
			return false
		}
		if verbosEntrada[strings.ToLower(ps[j].texto)] {
			return true
		}
	}
	return false
}

// relacionAntes mira hacia atrás desde la posición de «desde»: salta los puentes
// del habla («llevo con ustedes», «cliente nuestro») y devuelve true solo si
// encuentra el ancla. Cualquier otra palabra corta la búsqueda: sin relación
// delante, el año es de negocio y se conserva. Un ancla en tercera persona
// necesita además un sustantivo de persona en la ventana (CE-43).
func relacionAntes(ps []palabra, desde int) bool {
	for j := desde - 1; j >= 0 && desde-j <= topeAncla; j-- {
		clave := norm.Norm(ps[j].texto)
		switch {
		case anclasPrimeraPersona[clave], sustantivosDePersona[clave]:
			return true
		case anclasTerceraPersona[clave]:
			return personaEnLaVentana(ps, desde)
		case clave == "con" && interlocutores[norm.Norm(ps[j+1].texto)]:
			return true
		case !puentesRelacion[clave]:
			return false
		}
	}
	return false
}

// personaEnLaVentana dice si en la ventana del «desde» hay un sustantivo de
// persona: «la clienta de Cali está desde», «el arquitecto trabaja desde». Sin
// él, el verbo en tercera persona habla de la empresa o de una cosa y el año es
// de negocio (CE-43).
func personaEnLaVentana(ps []palabra, desde int) bool {
	for j := desde - 1; j >= 0 && desde-j <= topeAncla; j-- {
		if sustantivosDePersona[norm.Norm(ps[j].texto)] {
			return true
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

// union junta conjuntos ya definidos en uno nuevo, para que una palabra se
// declare una sola vez y los demás conjuntos la referencien en vez de copiarla.
func union(bases ...map[string]bool) map[string]bool {
	total := 0
	for _, base := range bases {
		total += len(base)
	}
	out := make(map[string]bool, total)
	for _, base := range bases {
		for k := range base {
			out[k] = true
		}
	}
	return out
}
