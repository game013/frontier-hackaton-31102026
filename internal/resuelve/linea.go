package resuelve

import (
	"regexp"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// rxReferencia reconoce los ids internos que ligan una línea con una persona:
// cliente, cliente migrado, empleado, queja y transacción.
var rxReferencia = regexp.MustCompile(`\b(?:CL|PG|KP|PQR|TX)-[0-9]+(?:-[0-9]+)?\b`)

// porLinea desempata homónimos con la evidencia que comparte línea o fila con
// la mención (CE-52): un id del maestro en la misma línea ("Camila Arango
// (CL-2)", la fila de una tabla markdown con su radicado) y `reporta_a`, que
// siempre nombra a un empleado. Decide solo si deja un único candidato.
func (c *contexto) porLinea(m *modelo.Mencion, indice *registro.Indice, candidatos []modelo.Persona) (string, bool) {
	if len(candidatos) < 2 {
		return "", false
	}
	if m.Campo == "columns.reporta_a" {
		return unicoConRol(candidatos, modelo.RolEmpleado)
	}
	linea := c.lineaDe(m)
	if linea == "" {
		return "", false
	}
	elegida := ""
	for _, ref := range rxReferencia.FindAllString(linea, -1) {
		p, ok := indice.PorReferencia(ref)
		if !ok || !esCandidato(p.Llave, candidatos) {
			continue
		}
		if elegida != "" && elegida != p.Llave {
			return "", false
		}
		elegida = p.Llave
	}
	return elegida, elegida != ""
}

// lineaDe devuelve la línea del campo donde cae la mención.
func (c *contexto) lineaDe(m *modelo.Mencion) string {
	valor := c.textos[m.Registro][m.Campo]
	if m.Inicio < 0 || m.Fin > len(valor) || m.Inicio > m.Fin {
		return ""
	}
	desde := strings.LastIndexByte(valor[:m.Inicio], '\n') + 1
	hasta := strings.IndexByte(valor[m.Fin:], '\n')
	if hasta < 0 {
		return valor[desde:]
	}
	return valor[desde : m.Fin+hasta]
}

func esCandidato(llave string, candidatos []modelo.Persona) bool {
	for _, p := range candidatos {
		if p.Llave == llave {
			return true
		}
	}
	return false
}

func unicoConRol(candidatos []modelo.Persona, rol modelo.Rol) (string, bool) {
	llave := ""
	for _, p := range candidatos {
		if p.Rol != rol {
			continue
		}
		if llave != "" {
			return "", false
		}
		llave = p.Llave
	}
	return llave, llave != ""
}
