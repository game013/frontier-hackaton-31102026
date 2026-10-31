package llm

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// TestPruebaReal llama a OpenRouter con datos inventados. Solo corre con
// JEV_PRUEBA_REAL=1 y gasta como máximo dos llamadas: comprueba el contrato del
// endpoint y el costo real, no la calidad del pipeline.
func TestPruebaReal(t *testing.T) {
	if os.Getenv("JEV_PRUEBA_REAL") != "1" {
		t.Skip("prueba real apagada: exporta JEV_PRUEBA_REAL=1 para correrla")
	}
	c, err := Nueva(Configuracion{
		ClaveAPI:     os.Getenv("OPENROUTER_API_KEY"),
		Modelo:       os.Getenv("LLM_MODEL"),
		TiempoLimite: 20 * time.Second,
		Intentos:     1,
	})
	if err != nil {
		t.Fatalf("Nueva: %v", err)
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelar()

	// Nada de datos reales: el state es una frase inventada y ya enmascarada.
	state := "[PERSONA_1] escribe para pedir que le devuelvan el saldo de su cuenta, no una falla de la aplicación."

	clasifica := modelo.Pregunta{
		ID:       "tipo",
		Tipo:     "choice",
		Texto:    "¿A qué tipo de entidad corresponde el fragmento [PERSONA_1]?",
		Opciones: []string{"person", "organization_or_place", "brand", "public_figure", "other"},
	}
	r, err := c.DecideUmbral(ctx, state, clasifica, UmbralPersona)
	if err != nil && !errors.Is(err, ErrBajaConfianza) {
		t.Fatalf("clasificación: %v", err)
	}
	t.Logf("clasificación: opción %q P=%.2f (%v)", r.Opcion, r.Probabilidades[r.Opcion], err)

	genero := modelo.Pregunta{
		ID:       "genero",
		Tipo:     "score",
		Texto:    "¿Qué género corresponde al nombre [PERSONA_1]?",
		Opciones: []string{"M", "F", "indeterminado"},
	}
	rg, err := c.DecideUmbral(ctx, "El cliente firma como [PERSONA_1].", genero, UmbralGenero)
	if err != nil && !errors.Is(err, ErrBajaConfianza) {
		t.Fatalf("género: %v", err)
	}
	t.Logf("género: opción %q P=%.2f (%v)", rg.Opcion, rg.Probabilidades[rg.Opcion], err)

	uso := c.Uso()
	t.Logf("modelo %s · llamadas %d · tokens %d/%d · costo US$%.8f",
		c.Nombre(), uso.Llamadas, uso.TokensEntrada, uso.TokensSalida, uso.CostoUSD)
	if uso.Llamadas == 0 || uso.Llamadas > 2 {
		t.Fatalf("%d llamadas, la prueba real es de máximo 2", uso.Llamadas)
	}
	if uso.CostoUSD <= 0 {
		t.Fatalf("el proveedor no reportó costo: %+v", uso)
	}
}
