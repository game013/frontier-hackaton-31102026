package nucleo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/chats"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/tablas"
)

// CargaCorpus ingesta las tres fuentes de una carpeta ya materializada —chats/,
// docs/ y tables/— y arma la entrada del núcleo en el orden de la fuente. La
// usan el job de /run y la herramienta local, para que el corpus se lea igual
// en los dos caminos (D8).
//
// Una carpeta sin ninguna fuente es error: un directorio equivocado no puede
// pasar por corpus vacío.
func CargaCorpus(ctx context.Context, raiz string, lector *docs.Lector) (Entrada, error) {
	entrada := Entrada{}
	chatsDir := filepath.Join(raiz, "chats")
	if existe(chatsDir) {
		corpus, err := chats.Cargar(chatsDir)
		if err != nil {
			return entrada, fmt.Errorf("chats: %w", err)
		}
		entrada.Usuarios, entrada.Autores, entrada.Canales = corpus.Usuarios, corpus.Autores, corpus.Canales
		for _, r := range corpus.Registros {
			entrada.Registros = append(entrada.Registros, r)
		}
	}
	docsDir := filepath.Join(raiz, "docs")
	if existe(docsDir) {
		if lector == nil {
			lector = docs.NuevoLector()
		}
		registros, err := lector.Directorio(ctx, docsDir)
		if err != nil {
			return entrada, fmt.Errorf("docs: %w", err)
		}
		for _, r := range registros {
			entrada.Registros = append(entrada.Registros, r)
		}
	}
	tablasDir := filepath.Join(raiz, "tables")
	if existe(tablasDir) {
		registros, err := tablas.Leer(tablasDir)
		if err != nil {
			return entrada, fmt.Errorf("tablas: %w", err)
		}
		for _, r := range registros {
			entrada.Registros = append(entrada.Registros, r)
		}
	}
	if len(entrada.Registros) == 0 {
		return entrada, fmt.Errorf("no encontré registros en %s", raiz)
	}
	return entrada, nil
}

func existe(ruta string) bool {
	_, err := os.Stat(ruta)
	return err == nil
}
