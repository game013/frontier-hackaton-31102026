package detecta

import (
	"regexp"
	"strings"
)

// generador es una de las cuatro reglas de D30 que proponen fragmentos al
// modelo. Jev no extrae texto: un nombre que nadie propone se fuga sin que
// nadie lo vea, así que las reglas proponen de más y el modelo descarta.
type generador struct {
	nombre string
	re     *regexp.Regexp
}

// candidatoCrudo es una propuesta con offsets en bytes del valor original.
type candidatoCrudo struct {
	inicio int
	fin    int
	valor  string
}

// disparadores son las fórmulas con que el corpus presenta a una persona:
// "soy Yesenia", "me llamo Ana", "Prepared by Doris Lozano", "la señora Gloria".
var disparadores = []string{
	`soy`, `me llamo`, `mi nombre es`, `habla con`, `hablar con`,
	`mi hermana`, `mi hermano`, `mi esposo`, `mi esposa`, `mi mam[aá]`, `mi pap[aá]`,
	`se[nñ]or`, `se[nñ]ora`, `don`, `do[nñ]a`, `prepared by`, `atentamente,?`, `attn:?`,
}

// nombrePropio es un nombre de pila o un apellido: mayúscula inicial y resto en
// minúscula. El corpus escribe los nombres así incluso en mayúsculas de tabla
// solo dentro de `descripcion`, que se cubre con la forma en MAYÚSCULAS.
const nombrePropio = `\p{Lu}[\p{Ll}]+`

// generadores van de más preciso a más ruidoso: disparador, secuencia de
// mayúsculas y handle. El diccionario de D30a lo cubren las formas del registro,
// que ya se buscan sin mayúsculas ni tildes (CE-29).
var generadores = []generador{
	{
		nombre: "disparador",
		re: regexp.MustCompile(`(?i:\b` + strings.Join(disparadores, `|\b`) + `)\s+(` +
			nombrePropio + `(?:\s+` + nombrePropio + `){0,3})`),
	},
	{
		nombre: "mayusculas",
		// Dos a cuatro palabras con mayúscula inicial fuera de la allowlist.
		re: regexp.MustCompile(`\b(` + nombrePropio + `(?:\s+` + nombrePropio + `){1,3})`),
	},
	{
		nombre: "handle",
		// La parte local de un correo o un @handle.
		re: regexp.MustCompile(`\B@([a-z0-9][a-z0-9._]{2,})`),
	},
}

// busca devuelve las propuestas del generador en un campo. Solo el primer grupo
// de captura es el candidato.
func (g generador) busca(valor string) []candidatoCrudo {
	var out []candidatoCrudo
	for _, tramo := range g.re.FindAllStringSubmatchIndex(valor, -1) {
		if len(tramo) < 4 || tramo[2] < 0 {
			continue
		}
		inicio, fin := tramo[2], tramo[3]
		texto := valor[inicio:fin]
		if !util(texto, g.nombre) {
			continue
		}
		out = append(out, candidatoCrudo{inicio: inicio, fin: fin, valor: texto})
	}
	return out
}

// util descarta lo que ninguna regla debería proponer: palabras sueltas que son
// parte de una razón social, títulos, o menos de tres letras.
func util(texto, generador string) bool {
	if len([]rune(texto)) < 3 {
		return false
	}
	if generador == "handle" {
		return true
	}
	palabras := strings.Fields(texto)
	if len(palabras) == 0 {
		return false
	}
	// Una sola palabra tras un disparador puede ser un nombre de pila, pero un
	// pronombre o un artículo no.
	if len(palabras) == 1 {
		switch strings.ToLower(palabras[0]) {
		case "el", "la", "los", "las", "un", "una", "este", "esta", "eso", "esto",
			"cliente", "clienta", "usuario", "usuaria", "señor", "señora", "don", "doña":
			return false
		}
	}
	// Las secuencias de mayúsculas que empiezan por una palabra de negocio no
	// son personas: la allowlist genérica las cubre más adelante.
	switch strings.ToLower(palabras[0]) {
	case "banco", "fondo", "caja", "superfinanciera", "dian", "sic", "pse",
		"datacredito", "nequi", "daviplata", "bancolombia", "davivienda", "colpatria":
		return false
	}
	return true
}
