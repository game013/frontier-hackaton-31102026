package docs_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
)

// Documentos de prueba: datos inventados, ninguno del corpus.

func escribe(t *testing.T, carpeta, nombre, contenido string) string {
	t.Helper()
	ruta := filepath.Join(carpeta, nombre)
	if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
		t.Fatalf("no pude escribir %s: %v", nombre, err)
	}
	return ruta
}

const indiceDesordenado = `[
  {"id": "doc_b", "filename": "segundo.md", "format": "md"},
  {"id": "doc_a", "filename": "primero.html", "format": "html"}
]`

func TestLeeIndiceRespetaElOrdenDelArchivo(t *testing.T) {
	entradas, err := docs.LeeIndice(strings.NewReader(indiceDesordenado))
	if err != nil {
		t.Fatalf("LeeIndice: %v", err)
	}
	esperado := []docs.Entrada{
		{ID: "doc_b", Filename: "segundo.md", Format: "md"},
		{ID: "doc_a", Filename: "primero.html", Format: "html"},
	}
	if !reflect.DeepEqual(entradas, esperado) {
		t.Fatalf("LeeIndice = %+v, esperaba %+v", entradas, esperado)
	}
}

func TestLeeIndiceRechazaJSONRoto(t *testing.T) {
	if _, err := docs.LeeIndice(strings.NewReader(`{"no": "es una lista"}`)); err == nil {
		t.Fatal("LeeIndice aceptó un índice que no es una lista")
	}
}

const markdownConFrontMatter = `---
title: Acta de ejemplo
author: rita.ejemplo@ejemplo.test
version: 1.0
revisado_por: "@nadie.ejemplo"
fecha: 2026-01-31
---
# Acta de ejemplo

Cuerpo con Rita Ejemplo Mora (CL-00001).

| Campo | Valor |
| --- | --- |
| Estado | Cerrado |
`

func TestDocumentoMarkdownConservaElArchivoYSuFrontMatter(t *testing.T) {
	carpeta := t.TempDir()
	ruta := escribe(t, carpeta, "acta_ejemplo.md", markdownConFrontMatter)

	registro, err := docs.NuevoLector().Documento(context.Background(),
		docs.Entrada{ID: "doc_x", Filename: "acta_ejemplo.md", Format: "md"}, ruta)
	if err != nil {
		t.Fatalf("Documento: %v", err)
	}

	if registro.Body != markdownConFrontMatter {
		t.Fatalf("el cuerpo no es el archivo íntegro:\n%q", registro.Body)
	}
	metadata := map[string]string{
		"title":        "Acta de ejemplo",
		"author":       "rita.ejemplo@ejemplo.test",
		"version":      "1.0",
		"revisado_por": "@nadie.ejemplo",
		"fecha":        "2026-01-31",
	}
	if !reflect.DeepEqual(registro.Metadata, metadata) {
		t.Fatalf("metadata = %#v, esperaba %#v", registro.Metadata, metadata)
	}
	if registro.Title != "Acta de ejemplo" {
		t.Fatalf("title = %q, esperaba el del front matter", registro.Title)
	}
	if registro.ID != "doc_x" || registro.Format != "md" || registro.Filename != "acta_ejemplo.md" {
		t.Fatalf("id/format/filename salieron del índice: %+v", registro)
	}
}

func TestDocumentoMarkdownSinTitleUsaElPrimerH1(t *testing.T) {
	carpeta := t.TempDir()
	ruta := escribe(t, carpeta, "sin_title.md", "# Nota sin metadata\n\nTexto.\n")

	registro, err := docs.NuevoLector().Documento(context.Background(),
		docs.Entrada{ID: "doc_y", Filename: "sin_title.md", Format: "md"}, ruta)
	if err != nil {
		t.Fatalf("Documento: %v", err)
	}
	if len(registro.Metadata) != 0 {
		t.Fatalf("metadata = %#v, esperaba vacía si no hay front matter", registro.Metadata)
	}
	if registro.Title != "Nota sin metadata" {
		t.Fatalf("title = %q, esperaba el primer H1", registro.Title)
	}
}

func TestDocumentoMarkdownSinTituloUsaElNombreDelArchivo(t *testing.T) {
	carpeta := t.TempDir()
	ruta := escribe(t, carpeta, "nota_suelta.md", "Texto sin encabezado.\n")

	registro, err := docs.NuevoLector().Documento(context.Background(),
		docs.Entrada{ID: "doc_z", Filename: "nota_suelta.md", Format: "md"}, ruta)
	if err != nil {
		t.Fatalf("Documento: %v", err)
	}
	if registro.Title != "nota suelta" {
		t.Fatalf("title = %q, esperaba el nombre del archivo sin extensión", registro.Title)
	}
}

func TestDocumentoRechazaFormatoDesconocido(t *testing.T) {
	carpeta := t.TempDir()
	ruta := escribe(t, carpeta, "raro.xls", "x")

	_, err := docs.NuevoLector().Documento(context.Background(),
		docs.Entrada{ID: "doc_w", Filename: "raro.xls", Format: "xls"}, ruta)
	if err == nil {
		t.Fatal("Documento aceptó un formato que no es pdf, html ni md")
	}
}

func TestDirectorioIngestaEnElOrdenDelIndice(t *testing.T) {
	carpeta := t.TempDir()
	escribe(t, carpeta, "index.json", indiceDesordenado)
	escribe(t, carpeta, "segundo.md", markdownConFrontMatter)
	escribe(t, carpeta, "primero.html", `<html><head><title>Primero</title></head><body><h1>Primero</h1><p>Texto.</p></body></html>`)

	registros, err := docs.NuevoLector().Directorio(context.Background(), carpeta)
	if err != nil {
		t.Fatalf("Directorio: %v", err)
	}
	if len(registros) != 2 {
		t.Fatalf("ingirió %d documentos, esperaba 2", len(registros))
	}
	if registros[0].ID != "doc_b" || registros[1].ID != "doc_a" {
		t.Fatalf("orden = %s, %s; esperaba el del índice (doc_b, doc_a)", registros[0].ID, registros[1].ID)
	}
	if registros[0].Metadata["author"] != "rita.ejemplo@ejemplo.test" {
		t.Fatalf("el .md perdió su metadata: %#v", registros[0].Metadata)
	}
	if registros[1].Title != "Primero" || !strings.Contains(registros[1].Body, "# Primero") {
		t.Fatalf("el .html salió mal: %+v", registros[1])
	}
}

func TestDirectorioAvisaSiFaltaUnDocumentoDelIndice(t *testing.T) {
	carpeta := t.TempDir()
	escribe(t, carpeta, "index.json", indiceDesordenado)

	if _, err := docs.NuevoLector().Directorio(context.Background(), carpeta); err == nil {
		t.Fatal("Directorio aceptó un índice con archivos que no existen")
	}
}
