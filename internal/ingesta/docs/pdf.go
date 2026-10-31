package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// salidaAyudante es el JSON que escribe tools/pdf/extraer.py, en las dos formas
// (markdown con pdfplumber y texto plano con pypdf).
type salidaAyudante struct {
	Body     string            `json:"body"`
	Metadata map[string]string `json:"metadata"`
}

// extraePDF llama al ayudante de PDF: primero con pdfplumber, que arma el
// markdown, y si falla o se pasa del timeout, con pypdf en texto plano. Si
// también falla, el cuerpo queda vacío: nunca se pierde el registro (D33).
func (l *Lector) extraePDF(ctx context.Context, ruta string) (string, map[string]string) {
	cuerpo, metadata, err := l.llamaAyudante(ctx, ruta)
	if err == nil {
		return cuerpo, metadata
	}
	if ctx.Err() != nil {
		return "", clavesVacias()
	}
	cuerpo, metadata, err = l.llamaAyudante(ctx, ruta, "--texto")
	if err != nil {
		return "", clavesVacias()
	}
	return cuerpo, metadata
}

// clavesVacias conserva las claves de metadata del contrato aunque no se haya
// podido leer el PDF: un registro sin `author` no vale (ENUNCIADO, "Qué entregas").
func clavesVacias() map[string]string {
	return map[string]string{"title": "", "author": "", "subject": "", "keywords": ""}
}

// llamaAyudante corre <Python> <Ayudante> [banderas] <ruta> con su propio
// timeout y devuelve el cuerpo y la metadata que el ayudante escribió.
func (l *Lector) llamaAyudante(ctx context.Context, ruta string, banderas ...string) (string, map[string]string, error) {
	espera := l.Timeout
	if espera <= 0 {
		espera = timeoutPorDefecto
	}
	ctx, cancela := context.WithTimeout(ctx, espera)
	defer cancela()

	argumentos := append(append([]string{l.Ayudante}, banderas...), ruta)
	salida, err := exec.CommandContext(ctx, l.Python, argumentos...).Output()
	if err != nil {
		return "", nil, fmt.Errorf("docs: el ayudante de PDF falló: %w", err)
	}
	var extraida salidaAyudante
	if err := json.Unmarshal(salida, &extraida); err != nil {
		return "", nil, fmt.Errorf("docs: el ayudante de PDF respondió algo ilegible: %w", err)
	}
	if extraida.Metadata == nil {
		extraida.Metadata = map[string]string{}
	}
	return extraida.Body, extraida.Metadata, nil
}
