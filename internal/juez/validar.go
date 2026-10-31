package juez

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Validacion describe la corrida del validador oficial. Cada paso externo
// (node) lleva timeout y devuelve un error legible: un juez que se cuelga no
// puede tumbar la corrida.
type Validacion struct {
	// Bin es el ejecutable de Node ("node").
	Bin string
	// Script es la ruta a docs/validar.mjs.
	Script string
	// Salida es el directorio que se valida.
	Salida string
	// Publico es el directorio con record_index.json (fuentes/ o sample/).
	Publico string
	// Timeout acota la corrida completa del validador.
	Timeout time.Duration
}

// ResultadoValidacion es la salida del validador con su veredicto.
type ResultadoValidacion struct {
	Pasa      bool
	Salida    string
	Duracion  time.Duration
	Ejecutada bool
	err       error
}

// Error devuelve el fallo de ejecución (no el veredicto), ya en español.
func (r ResultadoValidacion) Error() string {
	if r.err == nil {
		return ""
	}
	return r.err.Error()
}

// Valida corre `node <script> <salida> --public <publico>` y devuelve si pasó.
// Un veredicto negativo no es error: la salida trae el detalle. Un timeout o un
// binario ausente sí lo son.
func Valida(ctx context.Context, v Validacion) ResultadoValidacion {
	timeout := v.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	bin := v.Bin
	if bin == "" {
		bin = "node"
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	inicio := time.Now()
	cmd := exec.CommandContext(ctx, bin, v.Script, v.Salida, "--public", v.Publico)
	// WaitDelay cierra las tuberías aunque un hijo de node siga vivo: sin esto
	// un timeout se quedaría esperando la salida de un proceso nieto.
	cmd.WaitDelay = 2 * time.Second
	salida, err := cmd.CombinedOutput()
	res := ResultadoValidacion{Salida: strings.TrimSpace(string(salida)), Duracion: time.Since(inicio), Ejecutada: true}
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		res.err = fmt.Errorf("el validador no terminó en %s (node %s): revisa que el corpus no sea demasiado grande", timeout, v.Script)
		return res
	case errors.Is(err, exec.ErrNotFound):
		res.err = fmt.Errorf("no encuentro el ejecutable %q: instala Node o pasa --node con la ruta", bin)
		return res
	}
	var salidaErr *exec.ExitError
	if errors.As(err, &salidaErr) {
		res.Pasa = false
		if res.Salida == "" {
			res.Salida = salidaErr.Error()
		}
		return res
	}
	if err != nil {
		res.err = fmt.Errorf("no pude correr el validador (%s %s): %w", bin, v.Script, err)
		return res
	}
	res.Pasa = true
	return res
}

// ResumenValidacion es la primera línea del veredicto, para el reporte.
func ResumenValidacion(r ResultadoValidacion) string {
	if r.Error() != "" {
		return "no se pudo correr: " + r.Error()
	}
	lineas := strings.SplitN(r.Salida, "\n", 2)
	return strings.TrimSpace(lineas[0])
}
