package juez

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
)

// Medicion es el resultado de un juez base: cuántas cosas de cuántas pasan y el
// detalle de lo que falla.
type Medicion struct {
	ID        string
	Nombre    string
	Dimension string
	Pasan     int
	Total     int
	Detalles  []string
}

// Ratio es la fracción que pasa; 1 cuando no había nada que medir.
func (m Medicion) Ratio() float64 {
	if m.Total == 0 {
		return 1
	}
	return float64(m.Pasan) / float64(m.Total)
}

// Pasa dice si el juez está en verde.
func (m Medicion) Pasa() bool {
	return m.Pasan == m.Total
}

// J-GLO: barrido global. Ninguna forma real declarada en entities.json puede
// seguir en ningún registro de la salida, ni cambiando mayúsculas, tildes,
// confusables, separadores de dígitos o caracteres invisibles (usa Contiene, el
// mismo criterio del jurado). Las entidades conservadas (kept) están exentas:
// su forma real es la que debe quedar.
func JuezGLO(s *Salida) Medicion {
	m := Medicion{ID: "J-GLO", Nombre: "barrido global", Dimension: "barrido_global"}
	textos := s.Textos()
	for _, e := range s.Entities {
		if e.Kept {
			continue
		}
		for _, original := range e.Originals {
			m.Total++
			encontrados := 0
			for i, texto := range textos {
				if norm.Contiene(texto, original) {
					encontrados++
					if encontrados <= 3 {
						m.Detalles = append(m.Detalles, fmt.Sprintf("%s sigue en %s (%s)", enmascara(original), s.Orden[i], e.EntityID))
					}
				}
			}
			if encontrados == 0 {
				m.Pasan++
			} else if encontrados > 3 {
				m.Detalles = append(m.Detalles, fmt.Sprintf("%s sigue en %d registros más (%s)", enmascara(original), encontrados-3, e.EntityID))
			}
		}
	}
	return m
}

// J-DIC: honestidad del diccionario. Toda forma conocida de una persona (las
// `forms` y `lookup_forms` de los tests) tiene que estar declarada en su
// `originals` de entities.json: si no está, el diccionario miente y ninguna
// compuerta puede confiar en él.
func JuezDIC(s *Salida, tests []Test) Medicion {
	m := Medicion{ID: "J-DIC", Nombre: "diccionario", Dimension: "diccionario"}
	vistas := map[string]bool{}
	for _, t := range tests {
		formas := append(append([]string{}, formasDe(t.Forms)...), t.LookupForms...)
		if len(formas) == 0 {
			continue
		}
		entidad := entidadDelTest(s, t, formas)
		if entidad == nil {
			// El test no aplica a esta salida (p. ej. los públicos de las
			// fuentes corriendo sobre la muestra): no hay diccionario que medir.
			continue
		}
		for _, forma := range formas {
			clave := t.ID + "|" + norm.Norm(forma)
			if vistas[clave] {
				continue
			}
			vistas[clave] = true
			m.Total++
			if declaraForma(*entidad, forma) {
				m.Pasan++
				continue
			}
			m.Detalles = append(m.Detalles, fmt.Sprintf("%s: la forma %q no está en originals", entidad.EntityID, forma))
		}
	}
	return m
}

// entidadDelTest busca en la salida la entidad de la persona del test: por id
// si el test lo trae, si no por las formas que declara.
func entidadDelTest(s *Salida, t Test, formas []string) *modelo.Entity {
	if t.Persona != "" {
		for i := range s.Entities {
			if s.Entities[i].EntityID == t.Persona {
				return &s.Entities[i]
			}
		}
		return nil
	}
	entidad, _, ambigua := entidadDe(s, formas)
	if ambigua {
		return nil
	}
	return entidad
}

// J-REC: cobertura. Toda persona de la verdad (la salida ideal de la muestra)
// tiene que estar detectada y seudonimizada en la salida que se juzga (D34).
// J-GLO no ve a quien nadie detectó: por eso se mide aparte y la meta es 100 %.
func JuezREC(s *Salida, verdad []modelo.Entity) Medicion {
	m := Medicion{ID: "J-REC", Nombre: "cobertura", Dimension: "cobertura"}
	for _, esperada := range verdad {
		if esperada.Type != modelo.EntidadPersona || esperada.Kept {
			continue
		}
		m.Total++
		if cubierta(s, esperada) {
			m.Pasan++
			continue
		}
		m.Detalles = append(m.Detalles, fmt.Sprintf("la persona %s no está detectada", esperada.EntityID))
	}
	return m
}

// cubierta dice si la salida tiene una entidad seudonimizada de esa persona: se
// reconoce por compartir registros o alguna forma declarada.
func cubierta(s *Salida, esperada modelo.Entity) bool {
	for _, e := range s.Entities {
		if e.Kept || len(e.Pseudonyms) == 0 || e.Type != modelo.EntidadPersona {
			continue
		}
		for _, original := range esperada.Originals {
			if declaraForma(e, original) {
				return true
			}
		}
		for _, registro := range esperada.Records {
			for _, propio := range e.Records {
				if registro == propio {
					return true
				}
			}
		}
	}
	return false
}

// OrdenaDetalles deja el detalle del reporte estable entre corridas.
func OrdenaDetalles(d []string) []string {
	copia := append([]string{}, d...)
	sort.Strings(copia)
	return copia
}

// ResumeDetalles recorta el detalle a las primeras n líneas y cuenta el resto.
func ResumeDetalles(d []string, n int) []string {
	d = OrdenaDetalles(d)
	if len(d) <= n {
		return d
	}
	out := append([]string{}, d[:n]...)
	return append(out, fmt.Sprintf("… y %d más", len(d)-n))
}

// Linea es el renglón de un juez en el reporte.
func (m Medicion) Linea() string {
	return fmt.Sprintf("%-6s %-16s %d/%d (%.1f %%)", m.ID, m.Nombre, m.Pasan, m.Total, 100*m.Ratio())
}

// enmascara deja la forma irreconocible en los detalles del reporte: la regla 8
// de PROGRESS.md pide ids y conteos en los logs, nunca valores reales.
func enmascara(forma string) string {
	h := sha256.Sum256([]byte(forma))
	return fmt.Sprintf("forma#%x (%d caracteres)", h[:3], utf8.RuneCountInString(forma))
}
