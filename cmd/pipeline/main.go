// Comando pipeline: corre el núcleo completo sobre un corpus ya descargado y
// escribe output/ en el esquema del contrato. Es la herramienta con la que se
// prueba la corrida sin gastar una corrida oficial del jurado:
//
//	go run ./cmd/pipeline --fuentes sample/input --salida tmp/salida-muestra --sin-llm
//
// No lanza corridas oficiales: eso lo hace el usuario con VALIDAR ENTRYPOINT.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/almacen"
	"github.com/game013/frontier-hackaton-31102026/internal/detecta"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/nucleo"
	"github.com/game013/frontier-hackaton-31102026/internal/salida"
)

// secretoPorDefecto es solo para pruebas locales: en Railway el secreto del
// HMAC llega por variable de entorno y nunca vive en el repo.
const secretoPorDefecto = "prueba-local-no-usar-en-produccion"

func main() {
	fuentes := flag.String("fuentes", "fuentes", "carpeta con chats/, docs/ y tables/")
	salidaDir := flag.String("salida", "output", "carpeta de salida")
	datos := flag.String("datos", filepath.Join("tmp", "datos"), "carpeta de la bóveda y la base")
	allowlistRuta := flag.String("allowlist", "", "ruta a la allowlist JSON (vacío: la de por defecto)")
	sinLLM := flag.Bool("sin-llm", false, "no consultar el modelo (corrida determinista)")
	vivo := flag.Bool("vivo", false, "tratar el lote como /anonymize (personas en tabla aparte)")
	plazo := flag.Duration("plazo", 30*time.Minute, "plazo máximo de la corrida")
	flag.Parse()

	if err := corre(*fuentes, *salidaDir, *datos, *allowlistRuta, *sinLLM, *vivo, *plazo); err != nil {
		fmt.Fprintf(os.Stderr, "pipeline: %v\n", err)
		os.Exit(1)
	}
}

func corre(fuentes, salidaDir, datos, allowlistRuta string, sinLLM, vivo bool, plazo time.Duration) error {
	cargaEntorno(".env")
	ctx, cancelar := context.WithTimeout(context.Background(), plazo)
	defer cancelar()

	entrada, err := nucleo.CargaCorpus(ctx, fuentes, docs.NuevoLector())
	if err != nil {
		return err
	}
	allow, err := cargaAllowlist(allowlistRuta)
	if err != nil {
		return err
	}
	base, err := almacen.Open(filepath.Join(datos, "almacen.db"))
	if err != nil {
		return fmt.Errorf("abre la bóveda: %w", err)
	}
	defer base.Cerrar()

	var clienteModelo modelo.Modelo
	if !sinLLM {
		cliente, err := nucleo.ConfiguracionDelModelo(os.Getenv)
		if err != nil {
			return fmt.Errorf("modelo: %w", err)
		}
		clienteModelo = cliente
	}
	secreto := os.Getenv("SECRETO_HMAC")
	if secreto == "" {
		secreto = secretoPorDefecto
	}
	n, err := nucleo.Nuevo(nucleo.Opciones{
		Almacen: base, Modelo: clienteModelo, Secreto: []byte(secreto),
		Allowlist: allow, Vivo: vivo, Plazo: plazo,
	})
	if err != nil {
		return err
	}
	inicio := time.Now()
	resultado, err := n.Corrida(ctx, entrada)
	if err != nil {
		return err
	}
	if err := salida.Escribe(salidaDir, resultado.Registros, resultado.Entities); err != nil {
		return err
	}
	fmt.Printf("pipeline: %d registros, %d entidades, %d personas, %.1f s → %s\n",
		len(resultado.Registros), len(resultado.Entities), cuentaPersonas(resultado.Entities),
		time.Since(inicio).Seconds(), salidaDir)
	return nil
}

func cargaAllowlist(ruta string) (*detecta.Allowlist, error) {
	if ruta == "" {
		return detecta.PorDefecto(), nil
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return nil, fmt.Errorf("allowlist: %w", err)
	}
	return detecta.CargaJSON(datos)
}

func cuentaPersonas(ents []modelo.Entity) int {
	n := 0
	for _, e := range ents {
		if e.Type == modelo.EntidadPersona && !e.Kept {
			n++
		}
	}
	return n
}

// cargaEntorno lee un .env local si existe. No pisa variables ya definidas y
// nunca imprime valores (regla 8 de PROGRESS.md).
func cargaEntorno(ruta string) {
	archivo, err := os.Open(ruta)
	if err != nil {
		return
	}
	defer archivo.Close()
	sc := bufio.NewScanner(archivo)
	for sc.Scan() {
		linea := strings.TrimSpace(sc.Text())
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		clave, valor, ok := strings.Cut(linea, "=")
		if !ok {
			continue
		}
		clave = strings.TrimSpace(clave)
		valor = strings.Trim(strings.TrimSpace(valor), `"'`)
		if _, definida := os.LookupEnv(clave); definida || clave == "" {
			continue
		}
		os.Setenv(clave, valor)
	}
}
