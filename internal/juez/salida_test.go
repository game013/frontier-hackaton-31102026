package juez

import (
	"os"
	"path/filepath"
	"testing"
)

// escribeSalida arma una carpeta de salida de prueba con los jsonl que se le
// pasan (una línea por registro) y un entities.json. Los nombres son inventados.
func escribeSalida(t *testing.T, chats, docs, tables []string, entities string) string {
	t.Helper()
	dir := t.TempDir()
	escribe := func(nombre string, lineas []string) {
		contenido := ""
		for _, l := range lineas {
			contenido += l + "\n"
		}
		if err := os.WriteFile(filepath.Join(dir, nombre), []byte(contenido), 0o600); err != nil {
			t.Fatalf("escribiendo %s: %v", nombre, err)
		}
	}
	escribe("chats.jsonl", chats)
	escribe("docs.jsonl", docs)
	escribe("tables.jsonl", tables)
	if entities != "" {
		if err := os.WriteFile(filepath.Join(dir, "entities.json"), []byte(entities), 0o600); err != nil {
			t.Fatalf("escribiendo entities.json: %v", err)
		}
	}
	return dir
}

// salidaDePrueba es el corpus mínimo que usan los tests del juez: una persona
// con formas repartidas por las tres fuentes, una entidad conservada y un
// adjunto con el slug del nombre.
func salidaDePrueba(t *testing.T) *Salida {
	t.Helper()
	dir := escribeSalida(t,
		[]string{
			`{"id":"c1","thread_id":"t1","author":"Carla Falsa","ts":"1","text":"Hola, soy Carla Falsa y vivo en el Centro.","attachments":[{"filename":"carla_falsa.pdf"}]}`,
			`{"id":"c2","thread_id":"t1","author":"Carla Falsa","ts":"2","text":"Pagué con Banco Falso.","attachments":[]}`,
			`{"id":"c3","thread_id":"t1","author":"Otro Agente","ts":"3","text":"Anotado.","attachments":[]}`,
		},
		[]string{
			`{"id":"d1","format":"md","filename":"memo.md","title":"Memo","body":"Preparado por Carla Falsa.\n<!-- autor: Carla Falsa -->","metadata":{"author":"carla.falsa@ejemplo.test"}}`,
		},
		[]string{
			`{"id":"clientes:CX-1","table":"clientes","columns":{"nombre_completo":"Carla Falsa","ocupacion":"fotógrafa","barrio":"Centro","fecha_vinculacion":"2025-03-01"}}`,
		},
		`{"entities":[
			{"entity_id":"p_1","type":"person","originals":["Carla Falsa","carla_falsa","carla.falsa@ejemplo.test"],"pseudonyms":{"name":"Dora Ejemplo","email":"dejemplo@ejemplo.test"},"records":["c1","c2","d1","clientes:CX-1"]},
			{"entity_id":"kept_1","type":"organization_or_place","originals":["Banco Falso"],"pseudonyms":{},"records":[],"kept":true}
		]}`,
	)
	s, err := CargaSalida(dir)
	if err != nil {
		t.Fatalf("cargando la salida de prueba: %v", err)
	}
	return s
}

func TestCargaSalida(t *testing.T) {
	s := salidaDePrueba(t)
	if len(s.Errores) != 0 {
		t.Fatalf("errores inesperados: %v", s.Errores)
	}
	if len(s.Registros) != 5 {
		t.Fatalf("registros = %d, se esperaban 5", len(s.Registros))
	}
	if len(s.Entities) != 2 {
		t.Fatalf("entidades = %d, se esperaban 2", len(s.Entities))
	}
	if s.Registros["c1"].Fuente != "chats" {
		t.Fatalf("la fuente de c1 debería ser chats, es %q", s.Registros["c1"].Fuente)
	}
}

func TestCargaSalidaSinArchivos(t *testing.T) {
	dir := t.TempDir()
	s, err := CargaSalida(dir)
	if err != nil {
		t.Fatalf("una salida incompleta no debe ser error fatal: %v", err)
	}
	if len(s.Errores) != 4 {
		t.Fatalf("se esperaban 4 faltantes anotados, hay %d: %v", len(s.Errores), s.Errores)
	}
	if _, err := CargaSalida(filepath.Join(dir, "no-existe")); err == nil {
		t.Fatal("una carpeta inexistente debe ser error")
	}
}

func TestRegistroTextoYCampo(t *testing.T) {
	s := salidaDePrueba(t)
	tabla := s.Registros["clientes:CX-1"]
	if textos := tabla.Textos("columns.ocupacion"); len(textos) != 1 || textos[0] != "fotógrafa" {
		t.Fatalf("columns.ocupacion = %v", textos)
	}
	if textos := tabla.Textos("columns.no-existe"); len(textos) != 0 {
		t.Fatalf("un campo ausente debe dar lista vacía, dio %v", textos)
	}
	if textos := tabla.Textos(""); len(textos) != 1 || textos[0] != tabla.Texto() {
		t.Fatalf("el campo vacío es el registro entero: %v", textos)
	}
	chat := s.Registros["c1"]
	if !Contiene(chat.Texto(), "carla_falsa") {
		t.Fatalf("el texto del registro debe incluir el adjunto: %q", chat.Texto())
	}
	if textos := chat.Textos("attachments"); len(textos) == 0 || !Contiene(textos[len(textos)-1], "carla_falsa.pdf") {
		t.Fatalf("attachments debería exponer el nombre del adjunto: %v", textos)
	}
}

func TestRegistroOcultos(t *testing.T) {
	s := salidaDePrueba(t)
	ocultos := s.Registros["d1"].Ocultos()
	junto := ""
	for _, o := range ocultos {
		junto += o + "\n"
	}
	for _, esperado := range []string{"carla.falsa@ejemplo.test", "<!-- autor: Carla Falsa -->", "memo.md"} {
		if !ContieneExacto(junto, esperado) {
			t.Fatalf("los rincones ocultos deberían incluir %q: %v", esperado, ocultos)
		}
	}
}
