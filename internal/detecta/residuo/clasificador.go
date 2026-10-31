// Package residuo clasifica los candidatos que las reglas no pueden decidir.
//
// Es la única parte del pipeline que consulta al modelo para decidir si un
// fragmento es una persona. Jev 1.14 solo responde preguntas cerradas (D13): no
// extrae ni reescribe texto. Antes de viajar, toda forma conocida se enmascara y
// solo va una ventana de contexto alrededor del candidato (D15). Si el modelo
// falla o se pasa del tiempo, el candidato se conserva: equivocarse por
// conservar cuesta un punto de utilidad; equivocarse por reemplazar rompe una
// entidad de negocio y, peor, inventa una persona.
package residuo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/detecta"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
)

// Opciones del clasificador. Modelo nil deja todo a las reglas, que es el
// respaldo determinista de /anonymize (D19).
type Opciones struct {
	Modelo  modelo.Modelo
	Timeout time.Duration
	// Enmascara reemplaza las formas conocidas por etiquetas antes de que el
	// texto salga hacia el proveedor (D15). nil deja el texto tal cual.
	Enmascara func(string) string
}

// Clasificador decide el tipo de cada candidato.
type Clasificador struct {
	modelo    modelo.Modelo
	timeout   time.Duration
	enmascara func(string) string
}

// Nuevo arma el clasificador con un timeout por defecto corto: en /anonymize el
// presupuesto total es de 45 s (D28).
func Nuevo(o Opciones) *Clasificador {
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	enmascara := o.Enmascara
	if enmascara == nil {
		enmascara = func(s string) string { return s }
	}
	return &Clasificador{modelo: o.Modelo, timeout: timeout, enmascara: enmascara}
}

// Clasifica devuelve una mención por cada candidato que resulta persona. Los que
// no resultan persona no se mencionan: el texto se conserva tal cual.
func (c *Clasificador) Clasifica(ctx context.Context, cands []detecta.Candidato) []modelo.Mencion {
	if len(cands) == 0 {
		return nil
	}
	var out []modelo.Mencion
	for _, cand := range cands {
		tipo, confianza := c.decide(ctx, cand)
		if tipo != modelo.MencionPersona {
			continue
		}
		out = append(out, modelo.Mencion{
			Registro:  cand.Registro,
			Fuente:    cand.Fuente,
			Campo:     cand.Campo,
			Inicio:    cand.Inicio,
			Fin:       cand.Fin,
			Valor:     cand.Valor,
			Tipo:      tipo,
			Persona:   "", // lo decide la resolución, nunca el modelo
			Regla:     "residuo:" + cand.Generador,
			Confianza: confianza,
		})
	}
	return out
}

// decide pregunta al modelo y, si no hay modelo o falla, aplica la regla del
// generador. La confianza viaja a la cola de dudosos (D20).
func (c *Clasificador) decide(ctx context.Context, cand detecta.Candidato) (modelo.TipoMencion, float64) {
	if c.modelo == nil {
		return porRegla(cand)
	}
	llamada, cancelar := context.WithTimeout(ctx, c.timeout)
	defer cancelar()
	estado := c.estadoDe(cand)
	pregunta := modelo.Pregunta{
		ID:       "origen",
		Tipo:     "choice",
		Texto:    "¿Qué es el fragmento entre comillas en este contexto? Es una sola categoría.",
		Opciones: []string{"persona", "organizacion", "lugar", "marca", "otro"},
	}
	respuesta, err := c.modelo.Decide(llamada, estado, pregunta)
	if err != nil {
		return porRegla(cand)
	}
	return c.interpreta(respuesta)
}

// estadoDe arma el único texto que ve el proveedor: el candidato y su ventana,
// con las formas conocidas enmascaradas (D15, D28).
func (c *Clasificador) estadoDe(cand detecta.Candidato) string {
	ventana := cand.Valor
	if cand.Contexto != "" {
		ventana = cand.Contexto
	}
	return "Fragmento: «" + cand.Valor + "».\nContexto: «" + c.enmascara(ventana) + "»."
}

// interpreta aplica el umbral de D31 sobre la probabilidad de "persona": por
// debajo de 0,5 la respuesta no es confiable y el fragmento se conserva.
func (c *Clasificador) interpreta(r modelo.Respuesta) (modelo.TipoMencion, float64) {
	if r.Opcion == "" {
		return modelo.MencionOtro, 0
	}
	probabilidad := r.Probabilidades["persona"]
	if opcion := norm.Norm(r.Opcion); opcion == "persona" && probabilidad == 0 {
		probabilidad = 1
	}
	switch {
	case norm.Norm(r.Opcion) == "persona" && probabilidad > 0.5:
		return modelo.MencionPersona, probabilidad
	case probabilidad > 0.5:
		// La probabilidad de persona gana aunque la opción elegida sea "lugar":
		// el umbral se aplica a la probabilidad, no a la etiqueta (D31).
		return modelo.MencionPersona, probabilidad
	}
	if norm.Norm(r.Opcion) == "persona" {
		return modelo.MencionOtro, probabilidad
	}
	return tipoDeEtiqueta(r.Opcion), probabilidad
}

func tipoDeEtiqueta(opcion string) modelo.TipoMencion {
	switch norm.Norm(opcion) {
	case "organizacion":
		return modelo.MencionOrganizacionOLugar
	case "lugar":
		return modelo.MencionOrganizacionOLugar
	case "marca":
		return modelo.MencionMarca
	}
	return modelo.MencionOtro
}

// porRegla es el respaldo sin modelo: un disparador ("soy X", "la señora X")
// presenta a una persona; una secuencia de mayúsculas o un handle no alcanzan
// solos, porque confundirlos con personas rompe la utilidad.
func porRegla(cand detecta.Candidato) (modelo.TipoMencion, float64) {
	if cand.Generador == "disparador" && cuentaPalabras(cand.Valor) >= 1 {
		return modelo.MencionPersona, 0.55
	}
	return modelo.MencionOtro, 0
}

func cuentaPalabras(s string) int { return len(strings.Fields(s)) }

// ErrSinModelo queda para quien quiera distinguir el respaldo de una respuesta.
var ErrSinModelo = errors.New("residuo: sin modelo")
