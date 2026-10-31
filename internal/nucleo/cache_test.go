package nucleo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/almacen"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// El cliente que arma el servicio guarda sus decisiones en cache_modelo de la
// bóveda (D32): tras reiniciar, la corrida 2 no vuelve a pagar ni a esperar las
// preguntas de la corrida 1. Sin caché, cada corrida repetía las ~4.700.
func TestElModeloDelServicioUsaLaCacheDeLaBoveda(t *testing.T) {
	var peticiones atomic.Int64
	proveedor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peticiones.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"typesafe/jev-1.14-20261014","answers":{"origen":{"type":"choice",`+
			`"choice":"otro","confidence":0.9,"probabilities":{"otro":0.9,"persona":0.1}}},`+
			`"usage":{"cost":0,"input_tokens":1,"output_tokens":1}}`)
	}))
	t.Cleanup(proveedor.Close)
	entorno := map[string]string{"OPENROUTER_API_KEY": "clave-de-prueba", "LLM_URL_BASE": proveedor.URL}
	getenv := func(k string) string { return entorno[k] }
	ruta := filepath.Join(t.TempDir(), "almacen.db")
	pregunta := modelo.Pregunta{ID: "origen", Tipo: "choice", Texto: "¿Qué es?",
		Opciones: []string{"persona", "organizacion", "lugar", "marca", "otro"}}

	for corrida := 1; corrida <= 2; corrida++ {
		base, err := almacen.Open(ruta)
		if err != nil {
			t.Fatalf("corrida %d: abre la bóveda: %v", corrida, err)
		}
		cache, err := base.CacheModelo()
		if err != nil {
			t.Fatalf("corrida %d: caché: %v", corrida, err)
		}
		m, err := ConfiguracionDelModelo(getenv, cache)
		if err != nil || m == nil {
			t.Fatalf("corrida %d: modelo = %v, %v", corrida, m, err)
		}
		if _, err := m.Decide(context.Background(), "Fragmento: «Kipu».", pregunta); err != nil {
			t.Fatalf("corrida %d: Decide: %v", corrida, err)
		}
		base.Cerrar()
	}
	if n := peticiones.Load(); n != 1 {
		t.Fatalf("%d peticiones al proveedor en dos corridas, se esperaba 1", n)
	}
}
