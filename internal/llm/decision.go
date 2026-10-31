package llm

import (
	"math"
	"strconv"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Los tipos de este archivo son el JSON del endpoint de decisiones de
// OpenRouter, uno a uno (ver el comentario del paquete). Se mantienen aparte de
// modelo.* porque el contrato de C0 es de cara al pipeline, no al proveedor.

// tipos de pregunta del endpoint.
const (
	tipoChoice = "choice"
	tipoNoul   = "noul"
	tipoScore  = "score"
)

// preguntaDecision es una entrada de `questions`.
type preguntaDecision struct {
	Tipo          string `json:"type"`
	Instrucciones string `json:"instructions"`
	Criterios     any    `json:"criteria,omitempty"`
}

// proveedorPreferencias pide un proveedor sin retención (D15).
type proveedorPreferencias struct {
	RecoleccionDatos string `json:"data_collection"`
}

// cuerpoDecision es el cuerpo que se envía. Estado es any porque el esquema
// acepta string, objeto o arreglo; el cliente siempre manda el string que
// recibió.
type cuerpoDecision struct {
	Modelo    string                      `json:"model"`
	Proveedor proveedorPreferencias       `json:"provider"`
	Estado    any                         `json:"state"`
	Preguntas map[string]preguntaDecision `json:"questions"`
}

// respuestaDecision es el cuerpo que se espera.
type respuestaDecision struct {
	ID         string                   `json:"id"`
	Modelo     string                   `json:"model"`
	Proveedor  string                   `json:"provider"`
	Respuestas map[string]respuestaItem `json:"answers"`
	Uso        usoDecision              `json:"usage"`
}

// respuestaItem es un answer; los campos no usados por su tipo quedan en cero.
type respuestaItem struct {
	Tipo           string             `json:"type"`
	Opcion         string             `json:"choice"`
	Confianza      float64            `json:"confidence"`
	Probabilidades map[string]float64 `json:"probabilities"`
	Puntaje        float64            `json:"score"`
	NoUl           float64            `json:"noul"`
	Leyenda        map[string]string  `json:"legend"`
}

// usoDecision es la contabilidad que reporta el proveedor.
type usoDecision struct {
	Costo   float64 `json:"cost"`
	Entrada int     `json:"input_tokens"`
	Salida  int     `json:"output_tokens"`
}

// questions arma la entrada de `questions` de una pregunta del contrato. Las
// opciones de choice viajan como claves de criteria con null: el contrato de C0
// no trae descripción por opción.
func preguntaDecisionDe(p modelo.Pregunta) (preguntaDecision, error) {
	if err := validaPregunta(p); err != nil {
		return preguntaDecision{}, err
	}
	switch p.Tipo {
	case tipoChoice:
		criterios := make(map[string]any, len(p.Opciones))
		for _, opcion := range p.Opciones {
			criterios[opcion] = nil
		}
		return preguntaDecision{Tipo: tipoChoice, Instrucciones: p.Texto, Criterios: criterios}, nil
	case tipoScore:
		criterios := make([]string, len(p.Opciones))
		copy(criterios, p.Opciones)
		return preguntaDecision{Tipo: tipoScore, Instrucciones: p.Texto, Criterios: criterios}, nil
	default: // noul: sin criterios, la pregunta ya es booleana
		return preguntaDecision{Tipo: tipoNoul, Instrucciones: p.Texto}, nil
	}
}

// aRespuesta traduce un answer a modelo.Respuesta: opción legible por el
// pipeline y probabilidades por opción, para aplicar los umbrales de D31.
func aRespuesta(p modelo.Pregunta, item respuestaItem) (modelo.Respuesta, float64, error) {
	r := modelo.Respuesta{Pregunta: p.ID, Probabilidades: map[string]float64{}}
	confianza := item.Confianza

	switch item.Tipo {
	case tipoChoice:
		if item.Opcion == "" {
			return modelo.Respuesta{}, 0, ErrRespuestaInvalida
		}
		r.Opcion = item.Opcion
		for opcion, prob := range item.Probabilidades {
			r.Probabilidades[opcion] = prob
		}
	case tipoNoul:
		r.Probabilidades["true"] = item.NoUl
		r.Probabilidades["false"] = 1 - item.NoUl
		r.Opcion = "false"
		if item.NoUl >= 0.5 {
			r.Opcion = "true"
		}
		confianza = math.Max(item.NoUl, 1-item.NoUl)
	case tipoScore:
		r.Probabilidades = etiquetaConLeyenda(p, item)
		r.Opcion = mayorProbabilidad(r.Probabilidades)
		if r.Opcion == "" {
			return modelo.Respuesta{}, 0, ErrRespuestaInvalida
		}
	default:
		return modelo.Respuesta{}, 0, ErrRespuestaInvalida
	}

	if len(r.Probabilidades) == 0 {
		return modelo.Respuesta{}, 0, ErrRespuestaInvalida
	}
	return r, confianza, nil
}

// etiquetaConLeyenda reetiqueta las probabilidades de un score con el criterio
// que da el legend: las claves numéricas no le dicen nada al pipeline.
func etiquetaConLeyenda(p modelo.Pregunta, item respuestaItem) map[string]float64 {
	out := make(map[string]float64, len(item.Probabilidades))
	for clave, prob := range item.Probabilidades {
		criterio := clave
		if texto, hay := item.Leyenda[clave]; hay {
			criterio = texto
		} else if i, err := strconv.Atoi(clave); err == nil && i >= 0 && i < len(p.Opciones) {
			criterio = p.Opciones[i]
		}
		out[criterio] = prob
	}
	return out
}

// mayorProbabilidad devuelve la opción más probable; "" si no hay.
func mayorProbabilidad(probabilidades map[string]float64) string {
	mejor, pico := "", math.Inf(-1)
	for opcion, prob := range probabilidades {
		if prob > pico || (prob == pico && opcion < mejor) {
			mejor, pico = opcion, prob
		}
	}
	return mejor
}

// probabilidadDeLaOpcion es la probabilidad que D31 compara contra el umbral.
func probabilidadDeLaOpcion(r modelo.Respuesta, confianza float64) float64 {
	if prob, hay := r.Probabilidades[r.Opcion]; hay {
		return prob
	}
	return confianza
}
