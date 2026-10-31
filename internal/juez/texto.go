package juez

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Port de juez/texto.ts de docs/validar.mjs: es la normalización con la que el
// servicio de evaluación decide si una forma real sigue en la salida (fuga).
// Se copia aquí porque docs/ es de solo lectura y el juez tiene que medir con
// el mismo criterio que el jurado.
//
// Diferencia conocida: no hay NFKC/NFD completo (solo biblioteca estándar). Se
// pliegan los acentos latinos con tabla propia y se quitan las marcas
// combinantes, que es lo que aparece en el corpus; queda pendiente reemplazar
// este archivo por internal/norm (C3), que hace el port exacto.

// confusables son los caracteres de otros alfabetos que el jurado lee como su
// equivalente latino.
var confusables = map[rune]rune{
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'х': 'x', 'у': 'y',
	'і': 'i', 'ј': 'j', 'ѕ': 's', 'А': 'A', 'Е': 'E', 'О': 'O', 'Р': 'P',
	'С': 'C', 'Х': 'X', 'У': 'Y', 'І': 'I', 'Ј': 'J', 'Ѕ': 'S', 'В': 'B',
	'Н': 'H', 'К': 'K', 'М': 'M', 'Т': 'T', 'ο': 'o', 'α': 'a', 'ρ': 'p',
	'ν': 'v', 'ι': 'i', 'ℓ': 'l', 'ı': 'i', 'ɑ': 'a',
}

// ligaduras es la parte de NFKC que sí aparece en el corpus: el jurado lee
// "ﬁn" como "fin".
var ligaduras = map[rune]string{
	'\uFB00': "ff", '\uFB01': "fi", '\uFB02': "fl", '\uFB03': "ffi",
	'\uFB04': "ffl", '\uFB05': "st", '\uFB06': "st",
}

// sinTilde descompone las letras latinas acentuadas del corpus.
var sinTilde = func() map[rune]string {
	familias := map[string]string{
		"a": "àáâãäåāăą",
		"c": "çćĉċč",
		"d": "ď",
		"e": "èéêëēĕėęě",
		"g": "ĝğġģ",
		"h": "ĥ",
		"i": "ìíîïĩīĭį",
		"j": "ĵ",
		"k": "ķ",
		"l": "ĺļľ",
		"n": "ñńņňŉ",
		"o": "òóôõöōŏő",
		"r": "ŕŗř",
		"s": "śŝşš",
		"t": "ţť",
		"u": "ùúûüũūŭůűų",
		"w": "ŵ",
		"y": "ýÿŷ",
		"z": "źżž",
	}
	m := make(map[rune]string, 128)
	for base, acentuadas := range familias {
		for _, r := range acentuadas {
			m[r] = base
			m[unicode.ToUpper(r)] = strings.ToUpper(base)
		}
	}
	return m
}()

// Limpia replica clean(): confusables a su letra latina, fuera los caracteres
// de formato (invisibles), todo espacio raro pasa a ' ' y se colapsan los
// espacios repetidos.
func Limpia(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	espacioPrevio := false
	for _, r := range s {
		if c, ok := confusables[r]; ok {
			r = c
		}
		if unicode.Is(unicode.Cf, r) {
			continue
		}
		if l, ok := ligaduras[r]; ok {
			for _, lr := range l {
				b.WriteRune(lr)
			}
			espacioPrevio = false
			continue
		}
		if unicode.Is(unicode.Zs, r) || r == '\t' || r == '\r' || r == '\n' || r == '\v' || r == '\f' {
			r = ' '
		}
		if r == ' ' {
			if espacioPrevio {
				continue
			}
			espacioPrevio = true
		} else {
			espacioPrevio = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Normaliza replica norm(): limpia, quita tildes y pliega mayúsculas.
func Normaliza(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range Limpia(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if base, ok := sinTilde[r]; ok {
			b.WriteString(base)
			continue
		}
		b.WriteRune(r)
	}
	return pliega(b.String())
}

func pliega(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'ß':
			b.WriteString("ss")
		case 'ς':
			b.WriteRune('σ')
		case 'ſ':
			b.WriteRune('s')
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// Digitos deja solo los dígitos decimales (una cédula con puntos, sin puntos).
func Digitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Nd, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func esSeparadorDeDigitos(r rune) bool {
	if unicode.Is(unicode.Zs, r) || r == '\t' || r == '\r' || r == '\n' || r == '\v' || r == '\f' {
		return true
	}
	return strings.ContainsRune(".,'`´·/-–—_’", r)
}

// compactaDigitos borra los separadores que quedan entre dos dígitos, para que
// "1.107.360.039", "1 107 360 039" y "1-107-360-039" se lean igual.
func compactaDigitos(s string) string {
	runas := []rune(Limpia(s))
	out := make([]rune, 0, len(runas))
	for i, r := range runas {
		if esSeparadorDeDigitos(r) && i > 0 && i+1 < len(runas) &&
			unicode.Is(unicode.Nd, runas[i-1]) && unicode.Is(unicode.Nd, runas[i+1]) {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func esPalabraASCII(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func limiteAntes(s string, pos int) bool {
	if pos == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:pos])
	return !esPalabraASCII(r)
}

func limiteDespues(s string, pos int) bool {
	if pos >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[pos:])
	return !esPalabraASCII(r)
}

func limiteDigitoAntes(s string, pos int) bool {
	if pos == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:pos])
	return !unicode.Is(unicode.Nd, r)
}

func limiteDigitoDespues(s string, pos int) bool {
	if pos >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[pos:])
	return !unicode.Is(unicode.Nd, r)
}

// buscaConLimites prueba un patrón sobre s y exige los límites de palabra de
// validar.mjs ((?<![a-z0-9])…(?![a-z0-9])), que Go no soporta: se comprueban a
// mano y se avanza un carácter para no perder coincidencias solapadas.
func buscaConLimites(s string, re *regexp.Regexp, antes, despues func(string, int) bool) bool {
	for desde := 0; desde <= len(s); {
		m := re.FindStringIndex(s[desde:])
		if m == nil {
			return false
		}
		ini, fin := desde+m[0], desde+m[1]
		if antes(s, ini) && despues(s, fin) {
			return true
		}
		_, ancho := utf8.DecodeRuneInString(s[ini:])
		if ancho == 0 {
			return false
		}
		desde = ini + ancho
	}
	return false
}

func soloTexto(n string) bool {
	for _, r := range n {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == ' ' || r == '.' || r == '\'' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func tieneASCIIMinuscula(n string) bool {
	for _, r := range n {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}

func palabras(n string) []string {
	return strings.Split(n, " ")
}

// contieneDigitos replica digitosEn(): la aguja es sobre todo dígitos, así que
// se busca sobre la copia compacta y con límites de dígito, admitiendo el
// prefijo 57 de los celulares.
func contieneDigitos(pajar, digitos string) bool {
	compacto := compactaDigitos(pajar)
	candidatos := []string{digitos}
	if len(digitos) == 12 && strings.HasPrefix(digitos, "57") {
		candidatos = append(candidatos, digitos[2:])
	}
	for _, d := range candidatos {
		patron := "(?:57)?" + regexp.QuoteMeta(d)
		re, err := regexp.Compile(patron)
		if err != nil {
			continue
		}
		if buscaConLimites(compacto, re, limiteDigitoAntes, limiteDigitoDespues) {
			return true
		}
	}
	return false
}

// Contiene replica contiene() de docs/validar.mjs: la aguja está en el pajar
// bajo el criterio del jurado (pliegue, confusables, separadores de dígitos y
// límites de palabra).
func Contiene(pajar, aguja string) bool {
	if aguja == "" {
		return false
	}
	digitos := Digitos(aguja)
	if largoRuna(digitos) >= 6 && float64(largoRuna(digitos)) >= 0.6*float64(largoRuna(aguja)) && !tieneASCII(aguja) {
		return contieneDigitos(pajar, digitos)
	}
	h, n := Normaliza(pajar), Normaliza(aguja)
	if soloTexto(n) && tieneASCIIMinuscula(n) {
		if coincidenciaExacta(h, n) {
			return true
		}
		partes := palabras(n)
		if len(partes) >= 2 && todasLargas(partes) {
			return coincidenciaConEspacios(h, partes)
		}
		return false
	}
	return strings.Contains(h, n)
}

// ContieneExacto es la comparación literal, sin normalizar (opciones.exacto).
func ContieneExacto(pajar, aguja string) bool {
	return aguja != "" && strings.Contains(pajar, aguja)
}

func coincidenciaExacta(h, n string) bool {
	re, err := regexp.Compile(regexp.QuoteMeta(n))
	if err != nil {
		return false
	}
	return buscaConLimites(h, re, limiteAntes, limiteDespues)
}

func coincidenciaConEspacios(h string, partes []string) bool {
	escapadas := make([]string, len(partes))
	for i, p := range partes {
		escapadas[i] = regexp.QuoteMeta(p)
	}
	// El jurado admite cualquier espacio entre palabras: tras limpiar solo
	// quedan espacios ASCII.
	re, err := regexp.Compile(strings.Join(escapadas, " *"))
	if err != nil {
		return false
	}
	return buscaConLimites(h, re, limiteAntes, limiteDespues)
}

func todasLargas(partes []string) bool {
	for _, p := range partes {
		if largoRuna(p) < 3 {
			return false
		}
	}
	return true
}

func tieneASCII(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func largoRuna(s string) int {
	return utf8.RuneCountInString(s)
}
