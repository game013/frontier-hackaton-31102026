// Package docs ingesta la fuente de documentos: cada entrada de index.json se
// vuelve un modelo.DocRecord con el cuerpo en markdown —front matter, HTML
// convertido o PDF extraído— y la metadata del original (D7, D33).
package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// ayudantePorDefecto es el extractor de PDF relativo a la raíz del repo; en la
// imagen el servicio corre desde /app, donde también vive tools/pdf.
const ayudantePorDefecto = "tools/pdf/extraer.py"

// timeoutPorDefecto acota cada llamada al ayudante de PDF.
const timeoutPorDefecto = 30 * time.Second

// Entrada es un documento del índice de la fuente (docs/index.json), en el
// orden en que lo sirve el API.
type Entrada struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Format   string `json:"format"`
}

// LeeIndice decodifica docs/index.json respetando su orden: el índice no viene
// ordenado por id y el orden es el de la fuente.
func LeeIndice(r io.Reader) ([]Entrada, error) {
	var entradas []Entrada
	if err := json.NewDecoder(r).Decode(&entradas); err != nil {
		return nil, fmt.Errorf("docs: índice ilegible: %w", err)
	}
	return entradas, nil
}

// Lector ingesta documentos. Python y Ayudante son las dos piezas del ayudante
// de PDF y Timeout acota cada llamada al subproceso; el respaldo tiene la suya.
type Lector struct {
	// Python es el intérprete con pdfplumber y pypdf. Sale de PDF_PYTHON y, si
	// no está, de python3.
	Python string
	// Ayudante es la ruta a tools/pdf/extraer.py.
	Ayudante string
	// Timeout es el máximo de cada llamada al ayudante, incluida la del
	// respaldo en texto plano.
	Timeout time.Duration
}

// NuevoLector arma un Lector con el intérprete del entorno y los valores por
// defecto del repo.
func NuevoLector() *Lector {
	python := os.Getenv("PDF_PYTHON")
	if python == "" {
		python = "python3"
	}
	return &Lector{Python: python, Ayudante: ayudantePorDefecto, Timeout: timeoutPorDefecto}
}

// Directorio ingesta todos los documentos de carpeta en el orden de index.json.
func (l *Lector) Directorio(ctx context.Context, carpeta string) ([]modelo.DocRecord, error) {
	archivo, err := os.Open(filepath.Join(carpeta, "index.json"))
	if err != nil {
		return nil, fmt.Errorf("docs: no pude abrir el índice de %s: %w", carpeta, err)
	}
	entradas, err := LeeIndice(archivo)
	archivo.Close()
	if err != nil {
		return nil, err
	}
	registros := make([]modelo.DocRecord, 0, len(entradas))
	for _, entrada := range entradas {
		registro, err := l.Documento(ctx, entrada, filepath.Join(carpeta, entrada.Filename))
		if err != nil {
			return nil, err
		}
		registros = append(registros, registro)
	}
	return registros, nil
}

// Documento ingesta un documento del índice leyendo el archivo indicado. Un PDF
// que falle no pierde el registro: el cuerpo queda vacío (D33).
func (l *Lector) Documento(ctx context.Context, entrada Entrada, ruta string) (modelo.DocRecord, error) {
	if _, err := os.Stat(ruta); err != nil {
		return modelo.DocRecord{}, fmt.Errorf("docs: %s ilegible: %w", entrada.ID, err)
	}
	registro := modelo.DocRecord{ID: entrada.ID, Format: entrada.Format, Filename: entrada.Filename}
	switch strings.ToLower(entrada.Format) {
	case "md":
		contenido, err := os.ReadFile(ruta)
		if err != nil {
			return modelo.DocRecord{}, fmt.Errorf("docs: %s ilegible: %w", entrada.ID, err)
		}
		registro.Body, registro.Metadata = leeMarkdown(string(contenido))
	case "html", "htm":
		contenido, err := os.ReadFile(ruta)
		if err != nil {
			return modelo.DocRecord{}, fmt.Errorf("docs: %s ilegible: %w", entrada.ID, err)
		}
		cuerpo, metadata, err := MarkdownDeHTML(contenido)
		if err != nil {
			return modelo.DocRecord{}, fmt.Errorf("docs: %s html ilegible: %w", entrada.ID, err)
		}
		registro.Body, registro.Metadata = cuerpo, metadata
	case "pdf":
		registro.Body, registro.Metadata = l.extraePDF(ctx, ruta)
	default:
		return modelo.DocRecord{}, fmt.Errorf("docs: %s tiene formato desconocido %q", entrada.ID, entrada.Format)
	}
	registro.Title = titulo(registro.Metadata, registro.Body, entrada.Filename)
	return registro, nil
}

// leeMarkdown devuelve el archivo íntegro como cuerpo, front matter incluido
// como en la muestra, y el front matter como metadata: todas sus claves, con el
// valor tal cual está escrito (2.3 sigue siendo "2.3" y una fecha no se tipa).
func leeMarkdown(contenido string) (string, map[string]string) {
	metadata := map[string]string{}
	for _, linea := range lineasFrontMatter(contenido) {
		clave, valor, hay := cortaClaveValor(linea)
		if !hay {
			continue
		}
		metadata[clave] = valor
	}
	return contenido, metadata
}

// lineasFrontMatter devuelve las líneas entre los dos --- del encabezado; nada
// si el archivo no abre con front matter.
func lineasFrontMatter(contenido string) []string {
	normalizado := strings.ReplaceAll(contenido, "\r\n", "\n")
	if !strings.HasPrefix(normalizado, "---\n") {
		return nil
	}
	lineas := strings.Split(normalizado, "\n")
	for i := 1; i < len(lineas); i++ {
		if linea := strings.TrimSpace(lineas[i]); linea == "---" || linea == "..." {
			return lineas[1:i]
		}
	}
	return nil
}

// cortaClaveValor parte una línea `clave: valor` del front matter.
func cortaClaveValor(linea string) (string, string, bool) {
	if strings.HasPrefix(strings.TrimSpace(linea), "#") {
		return "", "", false
	}
	partes := strings.SplitN(linea, ":", 2)
	if len(partes) != 2 {
		return "", "", false
	}
	clave := strings.TrimSpace(partes[0])
	valor := strings.TrimSpace(partes[1])
	if clave == "" {
		return "", "", false
	}
	if len(valor) >= 2 {
		comilla := valor[0]
		if (comilla == '"' || comilla == '\'') && valor[len(valor)-1] == comilla {
			valor = valor[1 : len(valor)-1]
		}
	}
	return clave, valor, true
}

// titulo arma el title del registro: la metadata del original (title del front
// matter, <title> del HTML o /Title del PDF), el primer H1 si falta y, en
// última instancia, el nombre del archivo.
func titulo(metadata map[string]string, cuerpo, filename string) string {
	if valor := strings.TrimSpace(metadata["title"]); valor != "" {
		return valor
	}
	for _, linea := range strings.Split(cuerpo, "\n") {
		if strings.HasPrefix(linea, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(linea, "# "))
		}
	}
	sinExtension := strings.TrimSuffix(filename, filepath.Ext(filename))
	return strings.TrimSpace(strings.ReplaceAll(sinExtension, "_", " "))
}
