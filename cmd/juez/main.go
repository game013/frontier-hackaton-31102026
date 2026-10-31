// Comando juez: corre el juez base del pipeline sobre una carpeta de salida y
// dice cuánto puntúa. No anonimiza nada; solo mide.
//
//	go run ./cmd/juez --salida sample/expected_output --publico sample
//
// Mide: el validador oficial (node docs/validar.mjs), los tests de fuga y
// utilidad (los 13 de la muestra y los 5 públicos de las fuentes, por familia),
// y los jueces base J-GLO (barrido global de formas conocidas), J-DIC
// (honestidad del diccionario) y J-REC (cobertura de personas, D34).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/juez"
)

func main() {
	salida := flag.String("salida", "output", "carpeta de salida a juzgar (chats.jsonl, docs.jsonl, tables.jsonl, entities.json)")
	publico := flag.String("publico", "fuentes", "carpeta con record_index.json del corpus: fuentes o sample")
	validador := flag.String("validador", "docs/validar.mjs", "ruta a docs/validar.mjs")
	node := flag.String("node", "node", "ejecutable de Node")
	listaTests := flag.String("tests", "sample/tests_sample.json,fuentes/tests_public.json", "archivos de tests separados por coma")
	verdad := flag.String("verdad", "sample/expected_output/entities.json", "entities.json de la verdad para J-REC (D34)")
	sinValidador := flag.Bool("sin-validador", false, "no correr node docs/validar.mjs")
	timeout := flag.Duration("timeout", 5*time.Minute, "timeout de la corrida del validador")
	flag.Parse()

	if err := corre(*salida, *publico, *validador, *node, *listaTests, *verdad, *sinValidador, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "juez: %v\n", err)
		os.Exit(2)
	}
}

func corre(salida, publico, validador, node, listaTests, verdad string, sinValidador bool, timeout time.Duration) error {
	// La salida se carga primero: si no existe, nada más tiene sentido.
	lectura, err := juez.CargaSalida(salida)
	if err != nil {
		return err
	}

	reporte := juez.Reporte{Salida: salida, Publico: publico}
	if !sinValidador {
		if _, err := os.Stat(validador); err != nil {
			return fmt.Errorf("no encuentro el validador %s: %w", validador, err)
		}
		reporte.Validacion = juez.Valida(context.Background(), juez.Validacion{
			Bin:     node,
			Script:  validador,
			Salida:  salida,
			Publico: publico,
			Timeout: timeout,
		})
		if err := reporte.Validacion.Error(); err != "" {
			fmt.Fprintf(os.Stderr, "juez: %s\n", err)
		}
	}

	// Los tests que falten en disco no son un fallo: la muestra y las fuentes
	// no siempre están descargadas, y el reporte dice qué se midió.
	var tests []juez.Test
	for _, ruta := range strings.Split(listaTests, ",") {
		ruta = strings.TrimSpace(ruta)
		if ruta == "" {
			continue
		}
		if _, err := os.Stat(ruta); err != nil {
			fmt.Fprintf(os.Stderr, "juez: omito los tests de %s: no está en disco\n", ruta)
			continue
		}
		cargados, err := juez.CargaTests(ruta)
		if err != nil {
			return err
		}
		tests = append(tests, cargados...)
	}
	reporte.Tests = juez.EvalúaTests(lectura, tests)

	reporte.Jueces = append(reporte.Jueces, juez.JuezGLO(lectura), juez.JuezDIC(lectura, tests))
	if _, err := os.Stat(verdad); err == nil {
		esperadas, err := juez.CargaEntities(verdad)
		if err != nil {
			return err
		}
		reporte.Jueces = append(reporte.Jueces, juez.JuezREC(lectura, esperadas))
	} else {
		fmt.Fprintf(os.Stderr, "juez: sin verdad en %s, no mido J-REC\n", verdad)
	}

	reporte.Escribe(os.Stdout)
	for _, e := range lectura.Errores {
		fmt.Fprintf(os.Stdout, "  salida: %s\n", e)
	}
	if reporte.HayFallo() {
		os.Exit(1)
	}
	return nil
}
