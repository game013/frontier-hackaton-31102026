package detecta

// La allowlist de C26 viaja compilada en el binario, como los catálogos: la
// corrida no depende de la ruta de trabajo ni de un archivo suelto al lado.

import (
	_ "embed"
	"sync"
)

//go:embed allowlist.json
var datosAllowlist []byte

var (
	unaVezAllowlist sync.Once
	embebida        *Allowlist
	errEmbebida     error
)

// Embebida devuelve la allowlist de C26 (allowlist.json). El JSON se valida con
// CargaJSON una sola vez y el resultado se comparte: una entrada mal escrita
// rompe la carga en vez de perderse en silencio.
func Embebida() (*Allowlist, error) {
	unaVezAllowlist.Do(func() {
		embebida, errEmbebida = CargaJSON(datosAllowlist)
	})
	return embebida, errEmbebida
}
