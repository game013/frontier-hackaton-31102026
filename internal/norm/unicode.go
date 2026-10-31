package norm

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Los datos Unicode de tablas.go salen de unicodedata 17 y las categorías de la
// unicode de Go; el jurado corre sobre ICU, que va por Unicode 18. La diferencia
// son los caracteres asignados después de la 17: no se descomponen, no se
// pliegan ni se reconocen como marca. Son 119 puntos de código y ninguno aparece
// en el corpus.

// pieza es una runa con el tramo de bytes del original que la produjo. El
// pipeline de normalización trabaja sobre piezas para poder volver a offsets en
// bytes del original (ver Mapa): cada runa normalizada recuerda de dónde salió.
type pieza struct {
	r        rune
	ini, fin int
}

func piezasDe(s string) []pieza {
	ps := make([]pieza, 0, len(s))
	for i := 0; i < len(s); {
		r, ancho := utf8.DecodeRuneInString(s[i:])
		ps = append(ps, pieza{r: r, ini: i, fin: i + ancho})
		i += ancho
	}
	return ps
}

func texto(ps []pieza) string {
	var b strings.Builder
	b.Grow(len(ps))
	for _, p := range ps {
		b.WriteRune(p.r)
	}
	return b.String()
}

// Sílabas Hangul: descomposición y composición algorítmicas (UAX#15 §3.12), por
// eso no están en tablas.go.
const (
	baseL   = 0x1100
	baseV   = 0x1161
	baseT   = 0x11a7
	baseS   = 0xac00
	cuentaL = 19
	cuentaV = 21
	cuentaT = 28
	cuentaN = cuentaV * cuentaT
	cuentaS = cuentaL * cuentaN
)

func descomponeHangul(r rune) ([]rune, bool) {
	i := r - baseS
	if i < 0 || i >= cuentaS {
		return nil, false
	}
	l := baseL + i/cuentaN
	v := baseV + (i%cuentaN)/cuentaT
	if t := i % cuentaT; t != 0 {
		return []rune{l, v, baseT + t}, true
	}
	return []rune{l, v}, true
}

// descompone expande cada pieza en su descomposición de un paso, recursiva.
// conCompat incluye las descomposiciones de compatibilidad (NFKC) y, si no,
// solo las canónicas (NFD).
func descompone(ps []pieza, conCompat bool) []pieza {
	out := make([]pieza, 0, len(ps))
	var expande func(p pieza)
	expande = func(p pieza) {
		if rs, ok := descomponeHangul(p.r); ok {
			for _, r := range rs {
				out = append(out, pieza{r: r, ini: p.ini, fin: p.fin})
			}
			return
		}
		// Ninguna runa menor que U+00A0 tiene descomposición: atajo para el
		// texto latino, que es casi todo el corpus.
		if p.r >= 0xa0 {
			if d, ok := canonicas[p.r]; ok {
				for _, r := range d {
					expande(pieza{r: r, ini: p.ini, fin: p.fin})
				}
				return
			}
			if conCompat {
				if d, ok := compatibilidades[p.r]; ok {
					for _, r := range d {
						expande(pieza{r: r, ini: p.ini, fin: p.fin})
					}
					return
				}
			}
		}
		out = append(out, p)
	}
	for _, p := range ps {
		expande(p)
	}
	return out
}

// claseCombinante es la clase de combinación canónica (ccc). Ninguna runa menor
// que U+0300 tiene clase distinta de cero.
func claseCombinante(r rune) uint8 {
	if r < 0x300 {
		return 0
	}
	return clases[r]
}

// ordenaCanonica ordena las marcas de cada tramo por clase de combinación
// (orden canónico de UAX#15).
func ordenaCanonica(ps []pieza) []pieza {
	out := make([]pieza, len(ps))
	copy(out, ps)
	for i := 1; i < len(out); i++ {
		c := claseCombinante(out[i].r)
		if c == 0 {
			continue
		}
		for j := i; j > 0 && claseCombinante(out[j-1].r) > c; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// compone aplica la composición canónica (NFC) sobre piezas ya descompuestas y
// ordenadas. Dos piezas que se componen dejan una sola, con el tramo del
// original que cubre a las dos.
func compone(ps []pieza) []pieza {
	if len(ps) == 0 {
		return ps
	}
	out := make([]pieza, 1, len(ps))
	out[0] = ps[0]
	base := 0
	ultimaClase := claseCombinante(ps[0].r)
	if ultimaClase != 0 {
		// Una marca inicial no recibe composición.
		ultimaClase = 0xff
	}
	for _, p := range ps[1:] {
		c := claseCombinante(p.r)
		if ultimaClase < c || ultimaClase == 0 {
			if r, ok := compuesto(out[base].r, p.r); ok {
				out[base].r = r
				out[base].fin = p.fin
				continue
			}
		}
		if c == 0 {
			base = len(out)
		}
		ultimaClase = c
		out = append(out, p)
	}
	return out
}

// compuesto devuelve la composición primaria de dos runas, si existe.
func compuesto(a, b rune) (rune, bool) {
	if l := a - baseL; l >= 0 && l < cuentaL {
		if v := b - baseV; v >= 0 && v < cuentaV {
			return baseS + (l*cuentaV+v)*cuentaT, true
		}
	}
	if s := a - baseS; s >= 0 && s < cuentaS && s%cuentaT == 0 {
		if t := b - baseT; t > 0 && t < cuentaT {
			return a + t, true
		}
	}
	if a < 0x80 && b < 0x80 {
		return 0, false
	}
	r, ok := compuestos[[2]rune{a, b}]
	return r, ok
}

// espaciaRaro replica [\p{Zs}\t\r\n\v\f] de clean().
func espacioRaro(r rune) bool {
	switch r {
	case '\t', '\r', '\n', '\v', '\f':
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

// espacioJS replica la clase ESPACIO de docs/validar.mjs, que no es la de Go:
// incluye los separadores de control U+001C–U+001F y deja fuera U+180E.
func espacioJS(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\u0085', '\u00a0', '\u1680',
		'\u2028', '\u2029', '\u202f', '\u205f', '\u3000':
		return true
	case '\u001c', '\u001d', '\u001e', '\u001f':
		return true
	}
	if r >= '\u2000' && r <= '\u200a' {
		return true
	}
	return false
}

// separadorDigitos es la clase que RX_SEPARADOR_DIGITOS borra entre dos dígitos.
func separadorDigitos(r rune) bool {
	switch r {
	case '.', ',', '\'', '\u2019', '`', '\u00b4', '\u00b7', '/', '-', '\u2013', '\u2014', '_':
		return true
	}
	return espacioJS(r)
}

func palabra(r rune) bool {
	return unicode.Is(unicode.L, r) || unicode.Is(unicode.N, r) || r == '_'
}

func digito(r rune) bool { return unicode.Is(unicode.Nd, r) }

func marca(r rune) bool { return unicode.Is(unicode.Mn, r) }

// limpia es clean(): NFKC, confusables a ASCII, fuera los caracteres de
// formato, los espacios raros a espacio y los espacios repetidos a uno.
func limpia(ps []pieza) []pieza {
	base := compone(ordenaCanonica(descompone(ps, true)))
	out := make([]pieza, 0, len(base))
	for _, p := range base {
		r := p.r
		if c, ok := confusables[r]; ok {
			r = c
		}
		if unicode.Is(unicode.Cf, r) {
			continue
		}
		if espacioRaro(r) {
			r = ' '
		}
		if n := len(out); r == ' ' && n > 0 && out[n-1].r == ' ' {
			out[n-1].fin = p.fin
			continue
		}
		out = append(out, pieza{r: r, ini: p.ini, fin: p.fin})
	}
	return out
}

// sinTildes es quitarTildes(): NFD sin marcas no espaciadas. El tramo de la
// marca se suma al de la runa anterior, para que el original siga completo.
func sinTildes(ps []pieza) []pieza {
	des := ordenaCanonica(descompone(ps, false))
	out := make([]pieza, 0, len(des))
	for _, p := range des {
		if marca(p.r) {
			if n := len(out); n > 0 && p.fin > out[n-1].fin {
				out[n-1].fin = p.fin
			}
			continue
		}
		out = append(out, p)
	}
	return out
}

// pliega es plegar(): minúsculas, ß→ss, ς→σ, ſ→s.
func pliega(ps []pieza) []pieza {
	out := make([]pieza, 0, len(ps))
	agrega := func(p pieza, rs ...rune) {
		for _, r := range rs {
			out = append(out, pieza{r: r, ini: p.ini, fin: p.fin})
		}
	}
	for _, p := range ps {
		// İ es el único carácter cuya minúscula completa es dos runas (i +
		// punto), como en toLowerCase() de JavaScript.
		if p.r == '\u0130' {
			agrega(p, 'i', '\u0307')
			continue
		}
		switch l := unicode.ToLower(p.r); l {
		case 'ß':
			agrega(p, 's', 's')
		case 'ς':
			agrega(p, 'σ')
		case 'ſ':
			agrega(p, 's')
		default:
			agrega(p, l)
		}
	}
	return out
}

// normaliza es norm(): plegar(quitarTildes(clean(s))).
func normaliza(ps []pieza) []pieza {
	return pliega(sinTildes(limpia(ps)))
}
