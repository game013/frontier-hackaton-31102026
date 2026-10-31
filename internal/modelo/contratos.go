package modelo

import "context"

// Almacen es la persistencia en SQLite (volumen /data): bóveda y registro de
// personas. Un perfil asignado es para siempre: GuardarPerfil solo agrega.
type Almacen interface {
	// Persona devuelve una persona del registro por su llave.
	Persona(ctx context.Context, llave string) (Persona, bool, error)
	// GuardarPersona agrega una persona; si la llave existe, solo agrega formas
	// nuevas.
	GuardarPersona(ctx context.Context, p Persona) error
	// Perfil devuelve el perfil de la bóveda. false si no existe.
	Perfil(ctx context.Context, llave string) (Perfil, bool, error)
	// GuardarPerfil asigna el perfil de una persona que aún no lo tiene. Nunca
	// sobrescribe.
	GuardarPerfil(ctx context.Context, llave string, p Perfil) error
	// Cerrar libera la base.
	Cerrar() error
}

// Detector encuentra menciones en un conjunto de registros, en dos pasadas
// cuando hace falta (D14).
type Detector interface {
	Detecta(ctx context.Context, fuente Fuente, registros []Registro) ([]Mencion, error)
}

// Resolutor decide a qué persona pertenece cada mención: ids duros, contexto,
// nombre único, LLM con fichas abstractas o persona nueva dudosa (D19, D26,
// D29). Devuelve las menciones con Persona llena.
type Resolutor interface {
	Resuelve(ctx context.Context, menciones []Mencion) ([]Mencion, error)
}

// Fabrica produce el perfil falso determinista de una persona (D4, D22): mismo
// género y estructura, nunca igual a una forma real, único en la bóveda.
type Fabrica interface {
	Fabrica(ctx context.Context, p Persona) (Perfil, error)
}

// Pregunta es una pregunta cerrada para el modelo. Tipo es "choice", "noul" o
// "score" (endpoint de decisiones de Jev, D13).
type Pregunta struct {
	ID       string   `json:"id"`
	Tipo     string   `json:"type"`
	Texto    string   `json:"question"`
	Opciones []string `json:"options,omitempty"`
}

// Respuesta es la respuesta del modelo a una Pregunta, con probabilidades por
// opción para aplicar los umbrales de D31.
type Respuesta struct {
	Pregunta       string             `json:"pregunta"`
	Opcion         string             `json:"opcion"`
	Probabilidades map[string]float64 `json:"probabilidades,omitempty"`
}

// Modelo es el LLM de decisiones cerradas (Jev 1.14, D13). Solo clasifica o
// desempata; nunca genera ni reescribe texto.
type Modelo interface {
	// Nombre es el snapshot fijo en uso; /v1/health lo reporta (D32).
	Nombre() string
	// Decide hace una llamada por pregunta con su propio state (D28).
	Decide(ctx context.Context, state string, pregunta Pregunta) (Respuesta, error)
}
