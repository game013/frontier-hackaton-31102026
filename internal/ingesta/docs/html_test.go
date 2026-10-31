package docs_test

import (
	"reflect"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
)

const htmlCompleto = `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <title>Circular de ejemplo</title>
  <meta name="author" content="Rita Ejemplo Mora">
  <meta name="description" content="Circular interna de prueba">
  <meta name="generator" content="Generador de prueba 1.0">
  <style>body{color:#000}</style>
</head>
<body>
<h1>Circular de ejemplo</h1>
<p>Primera línea<br>segunda línea con <strong>negrita</strong>.</p>
<h2>Contacto</h2>
<p>Escribe a <a href="mailto:soporte.ejemplo@ejemplo.test">soporte</a>.</p>
<ul>
  <li>Punto uno &amp; dos</li>
  <li>Punto dos</li>
</ul>
<table>
<thead><tr><th>Campo</th><th>Valor</th></tr></thead>
<tbody>
<tr><td>Estado</td><td><strong>Cerrado</strong></td></tr>
<tr><td>Radicado</td><td>PQR-0000</td></tr>
</tbody>
</table>
<div style="display:none">Borrador revisado por Rita Ejemplo Mora.</div>
<!-- generado por rita.ejemplo@ejemplo.test el 2026-01-31 -->
</body>
</html>`

const markdownEsperado = `# Circular de ejemplo

Primera línea
segunda línea con **negrita**.

## Contacto

Escribe a [soporte](mailto:soporte.ejemplo@ejemplo.test).

- Punto uno & dos
- Punto dos

| Campo | Valor |
| --- | --- |
| Estado | **Cerrado** |
| Radicado | PQR-0000 |

<div style="display:none">Borrador revisado por Rita Ejemplo Mora.</div>

<!-- generado por rita.ejemplo@ejemplo.test el 2026-01-31 -->
`

func TestMarkdownDeHTMLConservaComentariosYOcultos(t *testing.T) {
	cuerpo, metadata, err := docs.MarkdownDeHTML([]byte(htmlCompleto))
	if err != nil {
		t.Fatalf("MarkdownDeHTML: %v", err)
	}
	if cuerpo != markdownEsperado {
		t.Fatalf("cuerpo:\n%s\nesperaba:\n%s", cuerpo, markdownEsperado)
	}
	esperada := map[string]string{
		"title":       "Circular de ejemplo",
		"author":      "Rita Ejemplo Mora",
		"description": "Circular interna de prueba",
		"generator":   "Generador de prueba 1.0",
	}
	if !reflect.DeepEqual(metadata, esperada) {
		t.Fatalf("metadata = %#v, esperaba %#v", metadata, esperada)
	}
}

func TestMarkdownDeHTMLEnlaceConElCorreoComoTexto(t *testing.T) {
	entrada := `<html><body><p>Escríbenos a <a href="mailto:otro.ejemplo@ejemplo.test">otro.ejemplo@ejemplo.test</a>.</p></body></html>`
	cuerpo, _, err := docs.MarkdownDeHTML([]byte(entrada))
	if err != nil {
		t.Fatalf("MarkdownDeHTML: %v", err)
	}
	esperado := "Escríbenos a [otro.ejemplo@ejemplo.test](mailto:otro.ejemplo@ejemplo.test).\n"
	if cuerpo != esperado {
		t.Fatalf("cuerpo = %q, esperaba %q", cuerpo, esperado)
	}
}

func TestMarkdownDeHTMLListaNumeradaYEnlacesExternos(t *testing.T) {
	entrada := `<html><body><ol><li>Uno</li><li>Dos</li></ol><p>Ver <a href="https://ejemplo.test/x">el anexo</a>.</p></body></html>`
	cuerpo, _, err := docs.MarkdownDeHTML([]byte(entrada))
	if err != nil {
		t.Fatalf("MarkdownDeHTML: %v", err)
	}
	esperado := "1. Uno\n2. Dos\n\nVer [el anexo](https://ejemplo.test/x).\n"
	if cuerpo != esperado {
		t.Fatalf("cuerpo = %q, esperaba %q", cuerpo, esperado)
	}
}

func TestMarkdownDeHTMLGuardaElTextoOcultoConSuEtiqueta(t *testing.T) {
	entrada := `<html><body><p>Visible.</p><div style="display: none" class="aprobacion">Aprobado por Rita Ejemplo Mora</div></body></html>`
	cuerpo, _, err := docs.MarkdownDeHTML([]byte(entrada))
	if err != nil {
		t.Fatalf("MarkdownDeHTML: %v", err)
	}
	esperado := "Visible.\n\n<div style=\"display: none\" class=\"aprobacion\">Aprobado por Rita Ejemplo Mora</div>\n"
	if cuerpo != esperado {
		t.Fatalf("cuerpo = %q, esperaba %q", cuerpo, esperado)
	}
}
