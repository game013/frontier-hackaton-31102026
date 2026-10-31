package llm

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Enmascarador sustituye las formas reales conocidas por marcadores antes de
// enviar el state al proveedor (D15). El cliente le entrega exactamente lo que
// iba a viajar: la ventana mínima alrededor del residuo, nunca el registro
// completo.
type Enmascarador interface {
	// Enmascara devuelve el texto con las formas conocidas reemplazadas.
	Enmascara(texto string) string
}

// Mascara reemplaza cada forma conocida por [PERSONA_n], sin distinguir
// mayúsculas ni tildes: el mismo nombre en MAYÚSCULAS o con otra acentuación
// lleva el mismo marcador.
type Mascara struct {
	formas     [][]rune
	marcadores []string
}

// NuevaMascara indexa las formas conocidas. La i-ésima forma distinta se vuelve
// [PERSONA_i], en el orden en que llega. Las formas vacías se ignoran.
func NuevaMascara(formas []string) *Mascara {
	vistas := map[string]int{}
	m := &Mascara{}
	for _, forma := range formas {
		plegada := plegar(forma)
		plegada = strings.TrimSpace(plegada)
		if plegada == "" {
			continue
		}
		if _, hay := vistas[plegada]; hay {
			continue
		}
		vistas[plegada] = len(m.formas) + 1
		m.formas = append(m.formas, []rune(plegada))
		m.marcadores = append(m.marcadores, "[PERSONA_"+strconv.Itoa(len(m.formas))+"]")
	}
	// Las formas más largas primero: "Camila Restrepo" antes que "Camila".
	orden := make([]int, len(m.formas))
	for i := range orden {
		orden[i] = i
	}
	sort.SliceStable(orden, func(a, b int) bool { return len(m.formas[orden[a]]) > len(m.formas[orden[b]]) })
	formasOrdenadas := make([][]rune, len(orden))
	marcadoresOrdenados := make([]string, len(orden))
	for i, indice := range orden {
		formasOrdenadas[i] = m.formas[indice]
		marcadoresOrdenados[i] = m.marcadores[indice]
	}
	m.formas, m.marcadores = formasOrdenadas, marcadoresOrdenados
	return m
}

// Enmascara recorre el texto una vez, sin cambiar nada fuera de las formas.
func (m *Mascara) Enmascara(texto string) string {
	if m == nil || len(m.formas) == 0 || texto == "" {
		return texto
	}
	original := []rune(texto)
	plegado := make([]rune, len(original))
	for i, r := range original {
		plegado[i] = pliegaRuna(r)
	}

	var salida strings.Builder
	salida.Grow(len(texto))
	for i := 0; i < len(original); {
		reemplazado := false
		for f, forma := range m.formas {
			if i+len(forma) > len(original) {
				continue
			}
			if string(plegado[i:i+len(forma)]) == string(forma) {
				salida.WriteString(m.marcadores[f])
				i += len(forma)
				reemplazado = true
				break
			}
		}
		if !reemplazado {
			salida.WriteRune(original[i])
			i++
		}
	}
	return salida.String()
}

// plegar pasa a minúsculas sin tildes conservando la longitud en runas: cada
// runa se pliega a una sola, así los índices del texto plegado sirven sobre el
// original.
func plegar(texto string) string {
	var b strings.Builder
	b.Grow(len(texto))
	for _, r := range texto {
		b.WriteRune(pliegaRuna(r))
	}
	return b.String()
}

// pliegaRuna es minúscula y sin diacríticos para el español.
func pliegaRuna(r rune) rune {
	if r == 'ñ' || r == 'Ñ' {
		return 'n'
	}
	if sinTilde, hay := sinTildes[r]; hay {
		return sinTilde
	}
	return unicode.ToLower(r)
}

// sinTildes es la tabla mínima de vocales acentuadas y diéresis.
var sinTildes = map[rune]rune{
	'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'ã': 'a', 'Á': 'a', 'À': 'a', 'Ä': 'a', 'Â': 'a', 'Ã': 'a',
	'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e', 'É': 'e', 'È': 'e', 'Ë': 'e', 'Ê': 'e',
	'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i', 'Í': 'i', 'Ì': 'i', 'Ï': 'i', 'Î': 'i',
	'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'õ': 'o', 'Ó': 'o', 'Ò': 'o', 'Ö': 'o', 'Ô': 'o', 'Õ': 'o',
	'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u', 'Ú': 'u', 'Ù': 'u', 'Ü': 'u', 'Û': 'u',
	'ç': 'c', 'Ç': 'c',
}
