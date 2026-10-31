// Package cuasi generaliza los cuasi-identificadores del corpus (D9,
// DECISIONS §1.3): ocupación, barrio y año de vinculación no son datos
// personales por sí solos, pero coinciden sobre una sola persona en la unión de
// fuentes. No se borran: se generalizan, y el texto queda byte a byte igual en
// todo lo demás (D6).
//
//	maestro (tabla clientes): ocupación → sector, barrio → otro barrio de la
//	misma ciudad, en todas las filas;
//	texto: la ocupación ligada a una persona → su sector, el barrio → su
//	ciudad y el año de la relación → "hace unos años".
//
// Corre después del reemplazo y antes de la compuerta: los nombres falsos ya
// están puestos y lo que se generaliza no es una forma de persona, así que la
// compuerta solo tiene que barrer lo que quede.
package cuasi

import (
	"fmt"
	"sort"

	"github.com/game013/frontier-hackaton-31102026/internal/campos"
	"github.com/game013/frontier-hackaton-31102026/internal/catalogo"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// Generaliza devuelve los registros con sus cuasi-identificadores generalizados,
// en el mismo orden. indice es el registro de personas: aporta la llave estable
// con que se elige el barrio falso de una fila del maestro y la ciudad de la
// persona, que desempata los barrios que existen en varias ciudades.
func Generaliza(recs []modelo.Registro, indice *registro.Indice) ([]modelo.Registro, error) {
	out := make([]modelo.Registro, 0, len(recs))
	for _, r := range recs {
		ediciones := edicionesDe(r, indice)
		if len(ediciones) == 0 {
			out = append(out, r)
			continue
		}
		nuevo, err := campos.Aplica(r, ediciones)
		if err != nil {
			return nil, fmt.Errorf("cuasi: %s: %w", r.IDRegistro(), err)
		}
		out = append(out, nuevo)
	}
	return out, nil
}

// edicionesDe arma los tramos a generalizar de un registro.
func edicionesDe(r modelo.Registro, indice *registro.Indice) []campos.Edicion {
	switch v := r.(type) {
	case modelo.TableRecord:
		return filaMaestro(v, indice)
	case modelo.ChatRecord:
		return edicionesTexto("text", v.Text, ciudadDeLaPersona(r, indice))
	case modelo.DocRecord:
		preferida := ciudadDeLaPersona(r, indice)
		return append(edicionesTexto("body", v.Body, preferida), edicionesTexto("title", v.Title, preferida)...)
	}
	return nil
}

// edicionesTexto generaliza los cuasi-identificadores de un campo de texto: la
// ocupación ligada a una persona va a su sector, el barrio a su ciudad y el año
// de la relación a "hace unos años" (D9). Un barrio detrás de «en» genérico
// solo se generaliza dentro de la plantilla, es decir, cuando el mismo texto ya
// declara la ocupación de alguien: así un lugar suelto se conserva.
func edicionesTexto(campo modelo.CampoTexto, texto, preferida string) []campos.Edicion {
	if texto == "" {
		return nil
	}
	ps := palabras(texto)
	ocupaciones := ocupacionesEn(ps)
	tramos := append([]tramo{}, ocupaciones...)
	for _, barrio := range barriosEn(ps, preferida) {
		if barrio.vivienda || len(ocupaciones) > 0 {
			tramos = append(tramos, barrio.tramo)
		}
	}
	tramos = append(tramos, aniosEn(texto, ps)...)
	sort.SliceStable(tramos, func(i, j int) bool { return tramos[i].desde < tramos[j].desde })

	ediciones := make([]campos.Edicion, 0, len(tramos))
	for _, t := range tramos {
		ediciones = append(ediciones, campos.Edicion{
			Ruta: campo, Inicio: ps[t.desde].inicio, Fin: ps[t.hasta-1].fin, Nuevo: t.nuevo,
		})
	}
	return ediciones
}

// ciudadDeLaPersona es la ciudad que la ficha de la persona guarda del maestro;
// desempata los barrios que existen en varias ciudades. Vacía si no se conoce.
func ciudadDeLaPersona(r modelo.Registro, indice *registro.Indice) string {
	if indice == nil {
		return ""
	}
	persona, ok := indice.AutorDe(r.IDRegistro())
	if !ok {
		return ""
	}
	ficha, ok := indice.Ficha(persona.Llave)
	if !ok {
		return ""
	}
	return ficha.Ciudad
}

// filaMaestro generaliza la fila de un cliente: la ocupación a su sector y el
// barrio a otro de la misma ciudad (D9). Una ocupación o un barrio que el
// catálogo no conoce se quedan como están: aquí no se inventan sectores.
func filaMaestro(r modelo.TableRecord, indice *registro.Indice) []campos.Edicion {
	if r.Table != "clientes" {
		return nil
	}
	var ediciones []campos.Edicion
	if sector, ok := catalogo.Sector(r.Columns["ocupacion"]); ok {
		ediciones = append(ediciones, entero("columns.ocupacion", r.Columns["ocupacion"], sector))
	}
	if barrio, ok := barrioAlterno(r, indice); ok {
		ediciones = append(ediciones, entero("columns.barrio", r.Columns["barrio"], barrio))
	}
	return ediciones
}

// barrioAlterno elige otro barrio de la misma ciudad. El índice sale de la llave
// de la persona (no del orden de las filas), así el barrio de una persona no
// cambia entre corridas (D4).
func barrioAlterno(r modelo.TableRecord, indice *registro.Indice) (string, bool) {
	actual := r.Columns["barrio"]
	if actual == "" {
		return "", false
	}
	llave := r.ID
	if indice != nil {
		if persona, ok := indice.PorReferencia(r.Columns["cliente_id"]); ok {
			llave = persona.Llave
		}
	}
	return catalogo.BarrioAlterno(r.Columns["ciudad"], actual, indiceDeLlave(llave))
}

// entero reemplaza el valor completo de un campo del registro.
func entero(ruta modelo.CampoTexto, valor, nuevo string) campos.Edicion {
	return campos.Edicion{Ruta: ruta, Inicio: 0, Fin: len(valor), Nuevo: nuevo}
}
