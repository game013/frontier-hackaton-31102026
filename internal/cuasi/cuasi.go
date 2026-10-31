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
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/game013/frontier-hackaton-31102026/internal/campos"
	"github.com/game013/frontier-hackaton-31102026/internal/catalogo"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// Generaliza devuelve los registros con sus cuasi-identificadores generalizados,
// en el mismo orden. indice es el registro de personas: solo aporta la llave
// estable con que se elige el barrio falso de una fila del maestro.
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
	if tabla, ok := r.(modelo.TableRecord); ok {
		return filaMaestro(tabla, indice)
	}
	return nil
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
	return catalogo.BarrioAlterno(r.Columns["ciudad"], actual, indiceDe(llave))
}

// indiceDe deriva un índice estable del hash de la llave.
func indiceDe(llave string) int {
	suma := sha256.Sum256([]byte(llave))
	return int(binary.BigEndian.Uint32(suma[:4]) % 1000)
}

// entero reemplaza el valor completo de un campo del registro.
func entero(ruta modelo.CampoTexto, valor, nuevo string) campos.Edicion {
	return campos.Edicion{Ruta: ruta, Inicio: 0, Fin: len(valor), Nuevo: nuevo}
}
