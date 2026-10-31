package salida

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/detecta"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// registrosDePrueba cubre las tres fuentes con todas sus claves.
func registrosDePrueba() []modelo.Registro {
	return []modelo.Registro{
		modelo.ChatRecord{ID: "m1", ThreadID: "c:1", Author: "Ana", TS: "1", Text: "hola",
			Attachments: []modelo.Adjunto{{Filename: "a.pdf"}}},
		modelo.DocRecord{ID: "d1", Format: "md", Filename: "f.md", Title: "T", Body: "cuerpo",
			Metadata: map[string]string{"author": "ana@x.com"}},
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes",
			Columns: map[string]string{"cliente_id": "CL-1", "nombre_completo": "Ana"}},
	}
}

// La salida tiene los tres JSONL en el orden de entrada y un entities.json con
// el contenedor que exige el validador.
func TestEscribeElEsquemaDelContrato(t *testing.T) {
	dir := t.TempDir()
	ents := []modelo.Entity{{EntityID: "p_1", Type: modelo.EntidadPersona,
		Originals: []string{"Ana"}, Pseudonyms: map[string]string{"name": "Bea"}, Records: []string{"m1"}}}
	if err := Escribe(dir, registrosDePrueba(), ents); err != nil {
		t.Fatalf("Escribe: %v", err)
	}
	for _, nombre := range []string{"chats.jsonl", "docs.jsonl", "tables.jsonl", "entities.json"} {
		if _, err := os.Stat(filepath.Join(dir, nombre)); err != nil {
			t.Errorf("falta %s: %v", nombre, err)
		}
	}
	crudo, err := os.ReadFile(filepath.Join(dir, "entities.json"))
	if err != nil {
		t.Fatalf("lee entities: %v", err)
	}
	var sobre struct {
		Entities []modelo.Entity `json:"entities"`
	}
	if err := json.Unmarshal(crudo, &sobre); err != nil {
		t.Fatalf("entities no es JSON: %v", err)
	}
	if len(sobre.Entities) != 1 || sobre.Entities[0].EntityID != "p_1" {
		t.Fatalf("entities = %+v", sobre.Entities)
	}
	chats, err := os.ReadFile(filepath.Join(dir, "chats.jsonl"))
	if err != nil {
		t.Fatalf("lee chats: %v", err)
	}
	if !bytes.Contains(chats, []byte(`"attachments":[{"filename":"a.pdf"}]`)) {
		t.Errorf("chats = %s", chats)
	}
}

// Dos escrituras de la misma entrada son byte a byte iguales: el determinismo
// es parte del contrato (prueba 12 de DISEÑO §8.2).
func TestEscribeEsDeterminista(t *testing.T) {
	primero, segundo := t.TempDir(), t.TempDir()
	ents := []modelo.Entity{
		{EntityID: "p_2", Type: modelo.EntidadPersona, Originals: []string{"Bea"},
			Pseudonyms: map[string]string{"name": "Dora"}, Records: []string{"m1"}},
		{EntityID: "p_1", Type: modelo.EntidadPersona, Originals: []string{"Ana"},
			Pseudonyms: map[string]string{"name": "Bea"}, Records: []string{"m1"}},
	}
	if err := Escribe(primero, registrosDePrueba(), ents); err != nil {
		t.Fatalf("Escribe 1: %v", err)
	}
	if err := Escribe(segundo, registrosDePrueba(), ents); err != nil {
		t.Fatalf("Escribe 2: %v", err)
	}
	for _, nombre := range []string{"chats.jsonl", "docs.jsonl", "tables.jsonl", "entities.json"} {
		a, _ := os.ReadFile(filepath.Join(primero, nombre))
		b, _ := os.ReadFile(filepath.Join(segundo, nombre))
		if !bytes.Equal(a, b) {
			t.Errorf("%s no es determinista", nombre)
		}
	}
}

// Una salida sin personas sigue escribiendo los archivos vacíos: el validador
// exige los tres.
func TestEscribeSinPersonas(t *testing.T) {
	dir := t.TempDir()
	if err := Escribe(dir, nil, nil); err != nil {
		t.Fatalf("Escribe: %v", err)
	}
	crudo, _ := os.ReadFile(filepath.Join(dir, "entities.json"))
	if string(bytes.TrimSpace(crudo)) != "{\n  \"entities\": []\n}" {
		t.Fatalf("entities = %q", crudo)
	}
}

// entities.json declara todas las formas reales de la persona y solo los
// seudónimos que se usaron (J-DIC, validar.mjs).
func TestEntidadesDeclaraFormasYSeudonimosUsados(t *testing.T) {
	indice, err := registro.Construye([]modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-1", "nombre_completo": "Manuela Castaño Ruiz",
			"num_doc": "1107360039", "correo": "mcastano@gmail.com", "celular": "3148729552",
		}},
	}, registro.Opciones{})
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	perfiles := map[string]modelo.Perfil{
		"p_cl_1": {Nombre: "Sofía Elena Zambrano", Correo: "szambrano@gmail.com", Cedula: "1107252808"},
	}
	menciones := []modelo.Mencion{
		{Registro: "m1", Campo: "text", Valor: "Manuela Castaño", Forma: modelo.FormaNombreCorto, Persona: "p_cl_1"},
		{Registro: "m2", Campo: "text", Valor: "mcastano@gmail.com", Forma: modelo.FormaCorreo, Persona: "p_cl_1"},
	}
	al, err := detecta.Nueva([]detecta.Entrada{{Nombre: "Bancolombia", Motivo: "banco"}}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("allowlist: %v", err)
	}
	ents := Entidades(indice, menciones, perfiles, al, nil)
	if len(ents) != 2 {
		t.Fatalf("entidades = %d, quiero 2 (persona y conservada)", len(ents))
	}
	persona := ents[0]
	// La mención fue el nombre corto: se declara la forma que de verdad quedó.
	if persona.Pseudonyms["name"] != "Sofía Elena" || persona.Pseudonyms["email"] != "szambrano@gmail.com" {
		t.Fatalf("seudónimos = %v", persona.Pseudonyms)
	}
	if _, tieneCedula := persona.Pseudonyms["cedula"]; tieneCedula {
		t.Fatalf("declaré una cédula que no se usó: %v", persona.Pseudonyms)
	}
	originals := map[string]bool{}
	for _, o := range persona.Originals {
		originals[o] = true
	}
	for _, quiero := range []string{"Manuela Castaño Ruiz", "mcastano@gmail.com", "3148729552", "1.107.360.039"} {
		if !originals[quiero] {
			t.Errorf("falta la forma %q en originals: %v", quiero, persona.Originals)
		}
	}
	if len(persona.Records) != 2 {
		t.Errorf("records = %v", persona.Records)
	}
	conservada := ents[1]
	if !conservada.Kept || conservada.Originals[0] != "Bancolombia" || conservada.Reason == "" {
		t.Fatalf("conservada = %+v", conservada)
	}
}
