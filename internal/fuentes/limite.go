package fuentes

import (
	"context"
	"time"
)

// esperaTurno bloquea hasta que la ventana deslizante de un minuto tenga hueco.
// Cada intento consume cupo, reintentos incluidos: el límite es del token, no de
// la ruta.
func (c *Cliente) esperaTurno(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.Lock()
		ahora := c.reloj.ahora()
		c.vence(ahora)
		if len(c.marcas) < c.cupo {
			c.marcas = append(c.marcas, ahora)
			c.mu.Unlock()
			return nil
		}
		espera := ventanaLimite - ahora.Sub(c.marcas[0])
		c.mu.Unlock()
		if err := c.reloj.dormir(ctx, espera); err != nil {
			return err
		}
	}
}

// vence descarta las marcas que ya salieron de la ventana. Se llama con c.mu
// tomado.
func (c *Cliente) vence(ahora time.Time) {
	caducas := 0
	for caducas < len(c.marcas) && !c.marcas[caducas].Add(ventanaLimite).After(ahora) {
		caducas++
	}
	if caducas > 0 {
		c.marcas = append(c.marcas[:0], c.marcas[caducas:]...)
	}
}
