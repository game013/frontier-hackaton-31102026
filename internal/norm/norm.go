// Package norm es el port en Go de la normalización del jurado
// (docs/validar.mjs, juez/texto.ts): limpia, quita tildes, pliega mayúsculas y
// busca con la misma vara con que el jurado mide una fuga (D5). Los nombres
// repiten los del jurado para poder comparar función por función, y
// testdata/casos.json fija la paridad caso por caso.
//
// La búsqueda corre sobre el espacio normalizado (Norm) y, como la salida exige
// reemplazo literal (D6), Normaliza además recuerda de dónde salió cada byte
// normalizado para volver a offsets del original.
package norm

import (
	"strings"
	"unicode/utf8"
)

// confusables son los homóglifos cirílicos y griegos que el jurado pasa a ASCII
// (CONFUSABLES en docs/validar.mjs). Ojo: el griego solo está en minúscula, así
// que un needle en minúsculas se latiniza y uno en mayúsculas no.
var confusables = map[rune]rune{
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'х': 'x', 'у': 'y',
	'і': 'i', 'ј': 'j', 'ѕ': 's',
	'А': 'A', 'Е': 'E', 'О': 'O', 'Р': 'P', 'С': 'C', 'Х': 'X', 'У': 'Y',
	'І': 'I', 'Ј': 'J', 'Ѕ': 'S', 'В': 'B', 'Н': 'H', 'К': 'K', 'М': 'M',
	'Т': 'T',
	'ο': 'o', 'α': 'a', 'ρ': 'p', 'ν': 'v', 'ι': 'i',
	'ℓ': 'l', 'ı': 'i', 'ɑ': 'a',
}

// Clean es clean() del jurado: NFKC, confusables a ASCII, sin caracteres de
// formato, espacios raros a espacio y espacios repetidos a uno.
func Clean(s string) string { return texto(limpia(piezasDe(s))) }

// QuitarTildes es quitarTildes() del jurado: NFD sin marcas no espaciadas.
func QuitarTildes(s string) string { return texto(sinTildes(piezasDe(s))) }

// Plegar es plegar() del jurado: minúsculas, ß→ss, ς→σ, ſ→s.
func Plegar(s string) string { return texto(pliega(piezasDe(s))) }

// Norm es norm() del jurado: plegar(quitarTildes(clean(s))). Es el espacio en
// el que se busca toda forma textual.
func Norm(s string) string { return texto(normaliza(piezasDe(s))) }

// DigitosEn es digitosEn() del jurado: busca la secuencia de dígitos en el
// haystack sin separadores entre dígitos, exigiendo que no tenga otro dígito
// pegado. Con doce dígitos que empiezan por 57 acepta también los diez últimos.
func DigitosEn(haystack, digitos string) bool {
	compacto := []rune(compactaDigitos(haystack))
	candidatos := [][]rune{[]rune(digitos)}
	if largo(digitos) == 12 && strings.HasPrefix(digitos, "57") {
		candidatos = append(candidatos, []rune(digitos)[2:])
	}
	for _, d := range candidatos {
		if buscaDigitos(compacto, d) {
			return true
		}
	}
	return false
}

// Contiene es contiene() del jurado con las opciones por defecto: la
// comprobación de que una forma real no quedó en la salida.
func Contiene(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	d := soloDigitos(needle)
	if d != "" && largo(d) >= 6 && float64(largo(d)) >= 0.6*float64(largo(needle)) && !letraASCII(needle) {
		return DigitosEn(haystack, d)
	}
	h := []rune(Norm(haystack))
	n := []rune(Norm(needle))
	if esTextoSolo(n) && minusculaASCII(n) {
		if contienePalabra(h, n) {
			return true
		}
		palabras := partir(n)
		if len(palabras) >= 2 && todasLargas(palabras, 3) {
			return contienePalabras(h, palabras)
		}
		return false
	}
	return strings.Contains(string(h), string(n))
}

// compactaDigitos es digitosCompactos() del jurado: clean() y fuera los
// separadores que están entre dos dígitos.
func compactaDigitos(s string) string {
	ps := limpia(piezasDe(s))
	var b strings.Builder
	b.Grow(len(ps))
	for i, p := range ps {
		if i > 0 && i+1 < len(ps) && separadorDigitos(p.r) && digito(ps[i-1].r) && digito(ps[i+1].r) {
			continue
		}
		b.WriteRune(p.r)
	}
	return b.String()
}

// soloDigitos es soloDigitos(): deja solo los dígitos decimales.
func soloDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if digito(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// buscaDigitos replica (?<!\p{Nd})(?:57)?D(?!\p{Nd}) sin lookarounds: Go no los
// tiene.
func buscaDigitos(h, d []rune) bool {
	if len(d) == 0 {
		return false
	}
	for i := 0; i+len(d) <= len(h); i++ {
		if !iguales(h[i:i+len(d)], d) {
			continue
		}
		fin := i + len(d)
		if fin < len(h) && digito(h[fin]) {
			continue
		}
		if i == 0 || !digito(h[i-1]) {
			return true
		}
		if i >= 2 && h[i-2] == '5' && h[i-1] == '7' && (i == 2 || !digito(h[i-3])) {
			return true
		}
	}
	return false
}

// contienePalabra replica (?<![a-z0-9])n(?![a-z0-9]) en el texto normalizado.
func contienePalabra(h, n []rune) bool {
	if len(n) == 0 {
		return false
	}
	for i := 0; i+len(n) <= len(h); i++ {
		if !iguales(h[i:i+len(n)], n) {
			continue
		}
		if i > 0 && letraODigitoASCII(h[i-1]) {
			continue
		}
		if fin := i + len(n); fin < len(h) && letraODigitoASCII(h[fin]) {
			continue
		}
		return true
	}
	return false
}

// contienePalabras replica (?<![a-z0-9])w1[ESPACIO]*w2...[ESPACIO]*wn(?![a-z0-9]):
// el jurado acepta las palabras pegadas ("anamaria") o separadas por espacios.
func contienePalabras(h []rune, palabras [][]rune) bool {
	primera := palabras[0]
	for i := 0; i+len(primera) <= len(h); i++ {
		if !iguales(h[i:i+len(primera)], primera) {
			continue
		}
		if i > 0 && letraODigitoASCII(h[i-1]) {
			continue
		}
		j := i + len(primera)
		completo := true
		for _, w := range palabras[1:] {
			for j < len(h) && espacioJS(h[j]) {
				j++
			}
			if j+len(w) > len(h) || !iguales(h[j:j+len(w)], w) {
				completo = false
				break
			}
			j += len(w)
		}
		if !completo {
			continue
		}
		if j < len(h) && letraODigitoASCII(h[j]) {
			continue
		}
		return true
	}
	return false
}

// esTextoSolo replica RX_TEXTO: ^[\p{L}\p{N}_ .'_-]+$.
func esTextoSolo(rs []rune) bool {
	if len(rs) == 0 {
		return false
	}
	for _, r := range rs {
		switch r {
		case ' ', '.', '\'', '-', '_':
			continue
		}
		if !palabra(r) {
			return false
		}
	}
	return true
}

// partir replica partir(): trozos separados por la clase ESPACIO.
func partir(rs []rune) [][]rune {
	var out [][]rune
	ini := -1
	for i, r := range rs {
		if espacioJS(r) {
			if ini >= 0 {
				out = append(out, rs[ini:i])
				ini = -1
			}
			continue
		}
		if ini < 0 {
			ini = i
		}
	}
	if ini >= 0 {
		out = append(out, rs[ini:])
	}
	return out
}

func todasLargas(palabras [][]rune, minimo int) bool {
	for _, p := range palabras {
		if len(p) < minimo {
			return false
		}
	}
	return true
}

func iguales(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func letraODigitoASCII(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// letraASCII replica /[a-zA-Z]/.
func letraASCII(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// minusculaASCII replica /[a-z]/.
func minusculaASCII(rs []rune) bool {
	for _, r := range rs {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}

// largo cuenta runas, como largo() del jurado (que cuenta puntos de código).
func largo(s string) int { return utf8.RuneCountInString(s) }
