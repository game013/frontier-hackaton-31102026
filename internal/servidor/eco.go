package servidor

import (
	"context"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Eco es el núcleo de C1: devuelve los registros tal cual y ninguna entidad,
// para probar el contrato de /v1/anonymize sin pipeline. C18 lo sustituye por
// el núcleo real; /run y /anonymize comparten siempre ese único núcleo (D8).
type Eco struct{}

// Anonymize devuelve el lote sin tocar, en el mismo orden.
func (Eco) Anonymize(_ context.Context, _ modelo.Fuente, registros []modelo.Registro) (modelo.Resultado, error) {
	return modelo.Resultado{Registros: registros, Entities: []modelo.Entity{}}, nil
}
