package docs_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
)

// rutaAyudante ubica tools/pdf/extraer.py desde este paquete.
func rutaAyudante(t *testing.T) string {
	t.Helper()
	_, archivo, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no pude ubicar este archivo de pruebas")
	}
	return filepath.Join(filepath.Dir(archivo), "..", "..", "..", "tools", "pdf", "extraer.py")
}

// pythonConLibrerias dice si el intérprete trae pdfplumber y pypdf.
func pythonConLibrerias(t *testing.T, python string) bool {
	t.Helper()
	ctx, listo := context.WithTimeout(context.Background(), 60*time.Second)
	defer listo()
	return exec.CommandContext(ctx, python, "-c", "import pdfplumber, pypdf").Run() == nil
}

// lectorDePrueba devuelve un Lector con el ayudante real y se salta las pruebas
// si el intérprete no trae pdfplumber y pypdf (exporta PDF_PYTHON al venv).
func lectorDePrueba(t *testing.T) *docs.Lector {
	t.Helper()
	lector := docs.NuevoLector()
	lector.Ayudante = rutaAyudante(t)
	if !pythonConLibrerias(t, lector.Python) {
		t.Skipf("sin pdfplumber y pypdf en %q: exporta PDF_PYTHON al venv para correr las pruebas de PDF", lector.Python)
	}
	return lector
}

// pdfDePrueba arma un PDF mínimo con la forma del generador del corpus:
// Helvetica, H1 de 16, H2 de 13, cuerpo de 10,5 y una tabla con rejilla.
func pdfDePrueba(t *testing.T) []byte {
	t.Helper()
	flujo := "" +
		"BT /F2 16 Tf 59 760 Td (Acta de prueba sintetica) Tj ET\n" +
		"BT /F1 10.5 Tf 59 720 Td (Primera linea del parrafo con Rita Ejemplo Mora.) Tj ET\n" +
		"BT /F1 10.5 Tf 59 704 Td (Segunda linea que continua el parrafo.) Tj ET\n" +
		"BT /F2 13 Tf 59 670 Td (Datos) Tj ET\n" +
		"0.5 G 0.5 w\n" +
		"59 600 m 320 600 l S 59 620 m 320 620 l S 59 640 m 320 640 l S 59 660 m 320 660 l S\n" +
		"59 600 m 59 660 l S 190 600 m 190 660 l S 320 600 m 320 660 l S\n" +
		"BT /F2 9 Tf 65 645 Td (Campo) Tj ET BT /F2 9 Tf 195 645 Td (Valor) Tj ET\n" +
		"BT /F1 9 Tf 65 625 Td (Estado) Tj ET BT /F1 9 Tf 195 625 Td (Cerrado) Tj ET\n" +
		"BT /F1 9 Tf 65 605 Td (Radicado) Tj ET BT /F1 9 Tf 195 605 Td (PQR-0000) Tj ET\n" +
		"BT /F1 10.5 Tf 59 570 Td (Cierre del acta.) Tj ET\n"

	objetos := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		"<< /Length " + strconv.Itoa(len(flujo)) + " >>\nstream\n" + flujo + "endstream",
		"<< /Title (Acta de prueba sintetica) /Author (Rita Ejemplo Mora) /Subject (Documento de prueba) /Keywords (uno, dos) /Creator (Generador de prueba) >>",
	}

	var salida bytes.Buffer
	salida.WriteString("%PDF-1.4\n")
	desplazamientos := make([]int, 0, len(objetos))
	for i, cuerpo := range objetos {
		desplazamientos = append(desplazamientos, salida.Len())
		fmt.Fprintf(&salida, "%d 0 obj\n%s\nendobj\n", i+1, cuerpo)
	}
	inicioXref := salida.Len()
	fmt.Fprintf(&salida, "xref\n0 %d\n", len(objetos)+1)
	salida.WriteString("0000000000 65535 f \n")
	for _, desplazamiento := range desplazamientos {
		fmt.Fprintf(&salida, "%010d 00000 n \n", desplazamiento)
	}
	fmt.Fprintf(&salida, "trailer\n<< /Size %d /Root 1 0 R /Info 7 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objetos)+1, inicioXref)
	return salida.Bytes()
}

const cuerpoEsperadoPDF = `# Acta de prueba sintetica

Primera linea del parrafo con Rita Ejemplo Mora. Segunda linea que continua el parrafo.

## Datos

| Campo | Valor |
| --- | --- |
| Estado | Cerrado |
| Radicado | PQR-0000 |

Cierre del acta.
`

func TestExtraePDFConCuerpoYMetadata(t *testing.T) {
	carpeta := t.TempDir()
	ruta := filepath.Join(carpeta, "acta_prueba.pdf")
	if err := os.WriteFile(ruta, pdfDePrueba(t), 0o600); err != nil {
		t.Fatalf("no pude escribir el PDF de prueba: %v", err)
	}

	registro, err := lectorDePrueba(t).Documento(context.Background(),
		docs.Entrada{ID: "doc_pdf", Filename: "acta_prueba.pdf", Format: "pdf"}, ruta)
	if err != nil {
		t.Fatalf("Documento: %v", err)
	}
	if registro.Body != cuerpoEsperadoPDF {
		t.Fatalf("cuerpo:\n%s\nesperaba:\n%s", registro.Body, cuerpoEsperadoPDF)
	}
	esperada := map[string]string{
		"title":    "Acta de prueba sintetica",
		"author":   "Rita Ejemplo Mora",
		"subject":  "Documento de prueba",
		"keywords": "uno, dos",
	}
	if !reflect.DeepEqual(registro.Metadata, esperada) {
		t.Fatalf("metadata = %#v, esperaba %#v", registro.Metadata, esperada)
	}
	if registro.Title != "Acta de prueba sintetica" {
		t.Fatalf("title = %q, esperaba /Title del PDF", registro.Title)
	}
}

func TestPDFCorruptoNoPierdeElRegistro(t *testing.T) {
	carpeta := t.TempDir()
	ruta := filepath.Join(carpeta, "roto.pdf")
	if err := os.WriteFile(ruta, []byte("esto no es un pdf"), 0o600); err != nil {
		t.Fatalf("no pude escribir el PDF corrupto: %v", err)
	}

	registro, err := lectorDePrueba(t).Documento(context.Background(),
		docs.Entrada{ID: "doc_roto", Filename: "roto.pdf", Format: "pdf"}, ruta)
	if err != nil {
		t.Fatalf("Documento abortó con un PDF corrupto: %v", err)
	}
	if registro.ID != "doc_roto" || registro.Format != "pdf" || registro.Filename != "roto.pdf" {
		t.Fatalf("el registro se perdió: %+v", registro)
	}
	if registro.Body != "" {
		t.Fatalf("cuerpo = %q, esperaba vacío cuando pdfplumber y pypdf fallan", registro.Body)
	}
}

// ayudanteQueFalla escribe un guion que imita el contrato de extraer.py: sin
// --texto falla, y en modo texto responde el JSON de respaldo.
func ayudanteQueFalla(t *testing.T) string {
	t.Helper()
	guion := `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "--texto" ]; then
    echo '{"body": "cuerpo de respaldo", "metadata": {"title": "De respaldo"}}'
    exit 0
  fi
done
echo "pdfplumber no respondió" >&2
exit 1
`
	ruta := filepath.Join(t.TempDir(), "ayudante_que_falla.sh")
	if err := os.WriteFile(ruta, []byte(guion), 0o700); err != nil {
		t.Fatalf("no pude escribir el guion: %v", err)
	}
	return ruta
}

func TestPDFCaeAlRespaldoTextoPlano(t *testing.T) {
	carpeta := t.TempDir()
	ruta := filepath.Join(carpeta, "acta_prueba.pdf")
	if err := os.WriteFile(ruta, []byte("%PDF-1.4 simulado"), 0o600); err != nil {
		t.Fatalf("no pude escribir el archivo: %v", err)
	}
	lector := docs.NuevoLector()
	lector.Python = "/bin/sh"
	lector.Ayudante = ayudanteQueFalla(t)

	registro, err := lector.Documento(context.Background(),
		docs.Entrada{ID: "doc_respaldo", Filename: "acta_prueba.pdf", Format: "pdf"}, ruta)
	if err != nil {
		t.Fatalf("Documento: %v", err)
	}
	if registro.Body != "cuerpo de respaldo" {
		t.Fatalf("cuerpo = %q, esperaba el del respaldo pypdf", registro.Body)
	}
	if registro.Title != "De respaldo" {
		t.Fatalf("title = %q, esperaba el del respaldo", registro.Title)
	}
}

func TestPDFQueAgotaElTimeoutNoPierdeElRegistro(t *testing.T) {
	carpeta := t.TempDir()
	ruta := filepath.Join(carpeta, "acta_prueba.pdf")
	if err := os.WriteFile(ruta, pdfDePrueba(t), 0o600); err != nil {
		t.Fatalf("no pude escribir el PDF de prueba: %v", err)
	}
	lector := lectorDePrueba(t)
	lector.Timeout = time.Nanosecond

	registro, err := lector.Documento(context.Background(),
		docs.Entrada{ID: "doc_lento", Filename: "acta_prueba.pdf", Format: "pdf"}, ruta)
	if err != nil {
		t.Fatalf("Documento abortó al agotarse el timeout: %v", err)
	}
	if registro.ID != "doc_lento" || registro.Body != "" {
		t.Fatalf("el registro se perdió o llegó cuerpo inesperado: %+v", registro)
	}
}
