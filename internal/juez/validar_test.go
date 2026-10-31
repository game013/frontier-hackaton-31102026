package juez

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// guionDePrueba deja un script ejecutable en un directorio temporal, para no
// depender de Node en las pruebas unitarias.
func guionDePrueba(t *testing.T, cuerpo string) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "validador.sh")
	if err := os.WriteFile(ruta, []byte("#!/bin/sh\n"+cuerpo+"\n"), 0o700); err != nil {
		t.Fatalf("escribiendo el guion: %v", err)
	}
	return ruta
}

func TestValidaOK(t *testing.T) {
	guion := guionDePrueba(t, `echo "OK: la salida pasa el validador"`)
	res := Valida(context.Background(), Validacion{Bin: "sh", Script: guion, Salida: "s", Publico: "p", Timeout: 5 * time.Second})
	if res.Error() != "" {
		t.Fatalf("no debía fallar la ejecución: %s", res.Error())
	}
	if !res.Pasa {
		t.Fatalf("el veredicto debía ser positivo: %+v", res)
	}
	if !strings.HasPrefix(ResumenValidacion(res), "OK") {
		t.Fatalf("el resumen debe ser la primera línea: %q", ResumenValidacion(res))
	}
}

func TestValidaVeredictoNegativo(t *testing.T) {
	guion := guionDePrueba(t, `echo "FALLA: 2 problema(s)"; echo " - chats: faltan 1 registros" >&2; exit 1`)
	res := Valida(context.Background(), Validacion{Bin: "sh", Script: guion, Salida: "s", Publico: "p", Timeout: 5 * time.Second})
	if res.Error() != "" {
		t.Fatalf("un veredicto negativo no es error de ejecución: %s", res.Error())
	}
	if res.Pasa {
		t.Fatal("el validador falló y el resultado dice que pasó")
	}
	if !strings.Contains(res.Salida, "FALLA: 2 problema(s)") || !strings.Contains(res.Salida, "faltan") {
		t.Fatalf("se pierde la salida del validador (stdout y stderr): %q", res.Salida)
	}
	if !strings.HasPrefix(ResumenValidacion(res), "FALLA") {
		t.Fatalf("resumen inesperado: %q", ResumenValidacion(res))
	}
}

func TestValidaTimeout(t *testing.T) {
	guion := guionDePrueba(t, `sleep 5`)
	inicio := time.Now()
	res := Valida(context.Background(), Validacion{Bin: "sh", Script: guion, Salida: "s", Publico: "p", Timeout: 200 * time.Millisecond})
	if res.Error() == "" || !strings.Contains(res.Error(), "no terminó") {
		t.Fatalf("el timeout debe ser un error legible, fue %q", res.Error())
	}
	if time.Since(inicio) > 3*time.Second {
		t.Fatalf("el timeout no cortó la corrida: %s", time.Since(inicio))
	}
}

func TestValidaSinBinario(t *testing.T) {
	res := Valida(context.Background(), Validacion{Bin: "node-que-no-existe", Script: "x.mjs", Salida: "s", Publico: "p", Timeout: time.Second})
	if res.Error() == "" || !strings.Contains(res.Error(), "no encuentro el ejecutable") {
		t.Fatalf("un binario ausente debe dar un error legible, fue %q", res.Error())
	}
}

func TestValidaContextoCancelado(t *testing.T) {
	guion := guionDePrueba(t, `sleep 5`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := Valida(ctx, Validacion{Bin: "sh", Script: guion, Salida: "s", Publico: "p", Timeout: 5 * time.Second})
	if res.Error() == "" {
		t.Fatal("un contexto cancelado no puede devolver verde")
	}
}
