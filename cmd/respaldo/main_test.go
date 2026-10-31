package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/almacen"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// nombreInventado es la forma real que la bóveda guarda: no puede aparecer en la
// consola, ni siquiera cuando el respaldo sale bien.
const nombreInventado = "Ana Pérez Rojas"

// bovedaDePrueba deja una bóveda con una persona y su perfil inventados.
func bovedaDePrueba(t *testing.T, datos string) {
	t.Helper()
	base, err := almacen.Open(filepath.Join(datos, "almacen.db"))
	if err != nil {
		t.Fatalf("abrir la bóveda: %v", err)
	}
	defer base.Cerrar()
	ctx := context.Background()
	persona := modelo.Persona{
		Llave:  "p_cl_90001",
		Rol:    modelo.RolCliente,
		Genero: modelo.GeneroF,
		Formas: []modelo.Forma{{Tipo: modelo.FormaNombreCompleto, Valor: nombreInventado, Registro: "clientes:90001"}},
	}
	if err := base.GuardarPersona(ctx, persona); err != nil {
		t.Fatalf("guardar persona: %v", err)
	}
	if err := base.GuardarPerfil(ctx, persona.Llave, modelo.Perfil{Nombre: "Marta Gómez Díaz"}); err != nil {
		t.Fatalf("guardar perfil: %v", err)
	}
}

// El operador lee rutas, tamaños, conteos y hashes; nunca formas ni nombres.
func TestRespaldoYRestauraSoloImprimenRutasYConteos(t *testing.T) {
	raiz := t.TempDir()
	datos := filepath.Join(raiz, "datos")
	bovedaDePrueba(t, datos)

	var salida, errores bytes.Buffer
	if codigo := corre([]string{"respaldo", "--datos", datos, "--destino", filepath.Join(raiz, "respaldos")}, &salida, &errores); codigo != 0 {
		t.Fatalf("código = %d, stderr = %s", codigo, errores.String())
	}
	linea := salida.String()
	for _, quiero := range []string{"respaldo:", "bytes", "sha256", "personas 1", "perfiles 1"} {
		if !strings.Contains(linea, quiero) {
			t.Fatalf("stdout sin %q: %s", quiero, linea)
		}
	}
	if strings.Contains(linea, nombreInventado) || strings.Contains(linea, "Marta") {
		t.Fatalf("stdout con formas o nombres: %s", linea)
	}

	archivos, err := os.ReadDir(filepath.Join(raiz, "respaldos"))
	if err != nil || len(archivos) != 1 {
		t.Fatalf("respaldos = %v, err = %v", archivos, err)
	}
	salida.Reset()
	errores.Reset()
	if codigo := corre([]string{"restaura", "--respaldo", filepath.Join(raiz, "respaldos", archivos[0].Name()), "--datos", filepath.Join(raiz, "nueva")}, &salida, &errores); codigo != 0 {
		t.Fatalf("código = %d, stderr = %s", codigo, errores.String())
	}
	if strings.Contains(salida.String(), nombreInventado) || !strings.Contains(salida.String(), "restaura:") {
		t.Fatalf("stdout de restaura: %s", salida.String())
	}
}

// Un destino dentro del repo falla con mensaje claro y sin escribir nada.
func TestRespaldoRechazaElRepoConMensajeClaro(t *testing.T) {
	datos := filepath.Join(t.TempDir(), "datos")
	bovedaDePrueba(t, datos)
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	var salida, errores bytes.Buffer
	codigo := corre([]string{"respaldo", "--datos", datos, "--destino", filepath.Join(repo, "tmp", "respaldos-prueba")}, &salida, &errores)
	if codigo != 1 {
		t.Fatalf("código = %d, quiero 1; stdout = %s", codigo, salida.String())
	}
	if !strings.Contains(errores.String(), "dentro del repo") {
		t.Fatalf("stderr sin el motivo: %s", errores.String())
	}
	if salida.Len() != 0 {
		t.Fatalf("stdout no debería llevar nada: %s", salida.String())
	}
}
