// Package modelo define los tipos y contrato compartidos del pipeline.
//
// Los tipos de registro replican uno a uno el esquema de output/ que exige
// docs/api/participante.openapi.yaml y que valida docs/validar.mjs: mismas
// claves, mismos tipos, sin campos extra. El resto del pipeline depende de
// este contrato; cualquier cambio se pide con CONTRATO: en la bitácora de
// PROGRESS.md y lo aplica quien orquesta.
package modelo

import (
	"context"
	"encoding/json"
)

// Fuente es el origen de un registro.
type Fuente string

const (
	FuenteChats  Fuente = "chats"
	FuenteDocs   Fuente = "docs"
	FuenteTablas Fuente = "tables"
)

// Adjunto es un archivo adjunto de un mensaje. El nombre se conserva como campo
// y se anonimiza como texto; nunca se omite.
type Adjunto struct {
	Filename string `json:"filename"`
}

// ChatRecord es un mensaje de soporte (chats.jsonl).
type ChatRecord struct {
	ID          string    `json:"id"`
	ThreadID    string    `json:"thread_id"`
	Author      string    `json:"author"`
	TS          string    `json:"ts"`
	Text        string    `json:"text"`
	Attachments []Adjunto `json:"attachments"`
}

// DocRecord es un documento interno (docs.jsonl). Format es pdf, html o md.
type DocRecord struct {
	ID       string            `json:"id"`
	Format   string            `json:"format"`
	Filename string            `json:"filename"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Metadata map[string]string `json:"metadata"`
}

// TableRecord es una fila de clientes, transacciones, pqrs o empleados
// (tables.jsonl). Todas las columnas originales viajan como texto.
type TableRecord struct {
	ID      string            `json:"id"`
	Table   string            `json:"table"`
	Columns map[string]string `json:"columns"`
}

// MarshalJSON garantiza que attachments viaje como [] y nunca como null.
func (r ChatRecord) MarshalJSON() ([]byte, error) {
	type alias ChatRecord
	if r.Attachments == nil {
		r.Attachments = []Adjunto{}
	}
	return json.Marshal(alias(r))
}

// MarshalJSON garantiza que metadata viaje como objeto y nunca como null.
func (r DocRecord) MarshalJSON() ([]byte, error) {
	type alias DocRecord
	if r.Metadata == nil {
		r.Metadata = map[string]string{}
	}
	return json.Marshal(alias(r))
}

// MarshalJSON garantiza que columns viaje como objeto y nunca como null.
func (r TableRecord) MarshalJSON() ([]byte, error) {
	type alias TableRecord
	if r.Columns == nil {
		r.Columns = map[string]string{}
	}
	return json.Marshal(alias(r))
}

// Registro es cualquier registro canónico del corpus.
type Registro interface {
	// IDRegistro es el id de output/: client_msg_id, id de index.json o
	// <tabla>:<llave>.
	IDRegistro() string
	// FuenteRegistro dice de qué fuente viene.
	FuenteRegistro() Fuente
}

func (r ChatRecord) IDRegistro() string  { return r.ID }
func (r DocRecord) IDRegistro() string   { return r.ID }
func (r TableRecord) IDRegistro() string { return r.ID }
func (r ChatRecord) FuenteRegistro() Fuente {
	return FuenteChats
}
func (r DocRecord) FuenteRegistro() Fuente { return FuenteDocs }
func (r TableRecord) FuenteRegistro() Fuente {
	return FuenteTablas
}

// Resultado es lo que devuelve el núcleo: los registros ya reemplazados, en el
// mismo orden en que entraron, y las entidades que intervinieron.
type Resultado struct {
	Registros []Registro `json:"records"`
	Entities  []Entity   `json:"entities"`
}

// Nucleo es el único camino de anonimización: lo usan /run y /anonymize (D8).
type Nucleo interface {
	Anonymize(ctx context.Context, fuente Fuente, registros []Registro) (Resultado, error)
}
