package servidor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// plazoAyudantePDF acota la comprobación de arranque: si Python no responde en
// 20 s, el servicio igual arranca (D33).
const plazoAyudantePDF = 20 * time.Second

// comprobacionPython importa las versiones fijas y devuelve lo que encontró.
const comprobacionPython = `import pdfplumber, pypdf; print(pdfplumber.__version__, pypdf.__version__)`

// CompruebaPDF verifica al arrancar que el ayudante de extracción existe y que
// el intérprete importa pdfplumber y pypdf. El resultado solo va al log: un
// fallo de PDF no puede tumbar el servicio ni el healthcheck (D33), y la
// ingesta tiene su propio respaldo por documento.
func CompruebaPDF(ctx context.Context, pythonBin, ayudante string) (string, error) {
	var notas []string
	if _, err := os.Stat(ayudante); err != nil {
		notas = append(notas, fmt.Sprintf("ayudante %s ausente", ayudante))
	} else {
		notas = append(notas, fmt.Sprintf("ayudante %s presente", ayudante))
	}

	ctx, cancel := context.WithTimeout(ctx, plazoAyudantePDF)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonBin, "-c", comprobacionPython)
	salida, err := cmd.CombinedOutput()
	detalle := strings.Join(notas, ", ")
	if err != nil {
		return detalle, fmt.Errorf("%s: %s no importa pdfplumber/pypdf: %v (%s)",
			detalle, pythonBin, err, strings.TrimSpace(string(salida)))
	}
	return fmt.Sprintf("%s, python=%s", detalle, strings.TrimSpace(string(salida))), nil
}
