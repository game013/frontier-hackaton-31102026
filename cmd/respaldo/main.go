// Comando respaldo: copia la bóveda —registro de personas y perfiles— a un disco
// local fuera de git y la devuelve al volumen cuando haga falta (C24, D4).
//
//	go run ./cmd/respaldo --datos /data --destino ~/respaldos/kipupay
//	go run ./cmd/respaldo restaura --respaldo ~/respaldos/kipupay/boveda-20261031T200312Z.db --datos /data-nuevo
//
// Solo imprime rutas, tamaños, conteos y hashes: nunca formas ni nombres.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/respaldo"
)

const uso = `respaldo: copia coherente de la bóveda (registro de personas y perfiles), que es
el único lugar con datos reales junto con entities.json.

Uso:
  respaldo --datos <carpeta> --destino <carpeta>
      Copia <datos>/almacen.db a <destino>/boveda-<UTC>.db sin cerrar la base.
      El destino no puede estar dentro de un repo git: la bóveda no se versiona.

  respaldo restaura --respaldo <archivo> --datos <carpeta>
      Deja el respaldo en <carpeta>/almacen.db. La carpeta tiene que ser nueva:
      una bóveda en uso no se pisa.

Solo imprime rutas, tamaños, conteos y hashes: nunca formas ni nombres.
`

func main() {
	os.Exit(corre(os.Args[1:], os.Stdout, os.Stderr))
}

// corre ejecuta un subcomando y devuelve el código de salida.
func corre(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, uso)
		return 2
	}
	switch args[0] {
	case "respaldo":
		return correRespaldo(args[1:], stdout, stderr)
	case "restaura":
		return correRestaura(args[1:], stdout, stderr)
	case "-h", "--help", "ayuda":
		fmt.Fprint(stdout, uso)
		return 0
	default:
		fmt.Fprintf(stderr, "respaldo: subcomando desconocido %q\n\n%s", args[0], uso)
		return 2
	}
}

func correRespaldo(args []string, stdout, stderr io.Writer) int {
	banderas := flag.NewFlagSet("respaldo", flag.ContinueOnError)
	banderas.SetOutput(stderr)
	datos := banderas.String("datos", "", "carpeta de la bóveda (la que recibe --datos del pipeline)")
	destino := banderas.String("destino", "", "carpeta de respaldos, fuera del repo")
	if err := banderas.Parse(args); err != nil {
		return 2
	}
	if *datos == "" || *destino == "" {
		fmt.Fprint(stderr, "respaldo: faltan --datos y --destino\n\n"+uso)
		return 2
	}
	r, err := respaldo.Respalda(*datos, *destino, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "respaldo: %s · %d bytes · sha256 %s · personas %d · formas %d · perfiles %d\n",
		r.Ruta, r.Bytes, r.SHA256, r.Personas, r.Formas, r.Perfiles)
	return 0
}

func correRestaura(args []string, stdout, stderr io.Writer) int {
	banderas := flag.NewFlagSet("restaura", flag.ContinueOnError)
	banderas.SetOutput(stderr)
	archivo := banderas.String("respaldo", "", "archivo boveda-<UTC>.db que dejó el respaldo")
	datos := banderas.String("datos", "", "carpeta nueva donde queda la bóveda")
	if err := banderas.Parse(args); err != nil {
		return 2
	}
	if *archivo == "" || *datos == "" {
		fmt.Fprint(stderr, "respaldo: faltan --respaldo y --datos\n\n"+uso)
		return 2
	}
	r, err := respaldo.Restaura(*archivo, *datos)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "restaura: %s · %d bytes · sha256 %s · personas %d · formas %d · perfiles %d\n",
		r.Ruta, r.Bytes, r.SHA256, r.Personas, r.Formas, r.Perfiles)
	return 0
}
