package norm

import "unicode/utf8"

// Mapa es un texto con su forma normalizada y la procedencia de cada byte
// normalizado en el original. La detección busca sobre Texto y Original traduce
// la coincidencia a bytes del original, que es lo que necesita el reemplazo
// literal (D6): el original no se toca, solo se sustituyen los tramos hallados.
type Mapa struct {
	Texto string
	ini   []int // por byte de Texto: primer byte del original que lo produjo
	fin   []int // por byte de Texto: fin exclusivo en el original
}

// Normaliza arma el mapa de un texto. Texto es exactamente Norm(original).
func Normaliza(original string) Mapa {
	ps := normaliza(piezasDe(original))
	texto := make([]byte, 0, len(original))
	ini := make([]int, 0, len(original))
	fin := make([]int, 0, len(original))
	var buf [utf8.UTFMax]byte
	for _, p := range ps {
		n := utf8.EncodeRune(buf[:], p.r)
		texto = append(texto, buf[:n]...)
		for i := 0; i < n; i++ {
			ini = append(ini, p.ini)
			fin = append(fin, p.fin)
		}
	}
	return Mapa{Texto: string(texto), ini: ini, fin: fin}
}

// Original traduce el tramo [inicio, fin) en bytes de Texto al tramo de bytes
// del original que lo produjo. El tramo es el mínimo que cubre todas las runas
// del original que aportaron a la coincidencia, así que incluye lo que la
// normalización borró o juntó (tildes, caracteres de formato, espacios de más).
//
// ok es false si el tramo no cabe en Texto. Cuando una runa normalizada sale de
// varias runas del original (una ligadura, un ancho completo), el tramo también
// las cubre todas: no parte una runa del original.
func (m Mapa) Original(inicio, fin int) (int, int, bool) {
	if inicio < 0 || inicio >= fin || fin > len(m.Texto) {
		return 0, 0, false
	}
	return m.ini[inicio], m.fin[fin-1], true
}
