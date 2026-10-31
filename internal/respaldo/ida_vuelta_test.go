package respaldo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/almacen"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/nucleo"
	"github.com/game013/frontier-hackaton-31102026/internal/respaldo"
	"github.com/game013/frontier-hackaton-31102026/internal/salida"
)

// secretoPrueba es el HMAC de la prueba: las dos corridas usan el mismo, como en
// producción.
const secretoPrueba = "secreto-de-prueba-ida-y-vuelta"

// terceroInventado es una persona que no sale del corpus: la bóveda le asigna un
// perfil antes de la corrida, como haría una corrida anterior.
const terceroInventado = "p_t_9000000000000001"

// perfilDelTercero es su perfil ya asignado: si el respaldo se pierde, este
// seudónimo cambia.
func perfilDelTercero() modelo.Perfil {
	return modelo.Perfil{
		Nombre:     "Nicolás Arrieta Bustos",
		Correo:     "n.arrieta@example.test",
		Celular:    "3005558877",
		Cedula:     "1098765432",
		Handle:     "n.arrieta",
		Direccion:  "Carrera 45 #12-09",
		Nacimiento: "1985-02-11",
		Placa:      "XYZ789",
		Cuenta:     "000987654321",
		Pasaporte:  "PA9876543",
	}
}

// loteInventado es un lote mínimo: una clienta del maestro y dos mensajes que la
// mencionan a ella y al tercero de la bóveda. Nada de esto sale del corpus real.
func loteInventado() []modelo.Registro {
	return []modelo.Registro{
		modelo.TableRecord{
			ID:    "clientes:CL-90001",
			Table: "clientes",
			Columns: map[string]string{
				"cliente_id":      "CL-90001",
				"nombre_completo": "Marisol Quintana Herrera",
				"correo":          "marisol.quintana@example.test",
				"celular":         "3001234567",
				"num_doc":         "1020304050",
				"ciudad":          "Bogotá",
				"ocupacion":       "contadora",
			},
		},
		modelo.ChatRecord{
			ID:       "msg_90001",
			ThreadID: "zulip:sintetico:1",
			Author:   "Marisol Quintana",
			TS:       "1",
			Text:     "Buen día, soy Marisol Quintana Herrera, escribo por el cobro.",
		},
		modelo.ChatRecord{
			ID:       "msg_90002",
			ThreadID: "zulip:sintetico:1",
			Author:   "Soporte Kipu",
			TS:       "2",
			Text:     "Me escribe Tobías Vergara Nieto, el representante legal, pidiendo la factura.",
		},
	}
}

// siembra deja en la bóveda al tercero inventado con su perfil ya asignado.
func siembra(t *testing.T, datos string) {
	t.Helper()
	base, err := almacen.Open(filepath.Join(datos, "almacen.db"))
	if err != nil {
		t.Fatalf("abrir la bóveda %s: %v", datos, err)
	}
	defer base.Cerrar()
	ctx := context.Background()
	persona := modelo.Persona{
		Llave:  terceroInventado,
		Rol:    modelo.RolDesconocido,
		Genero: modelo.GeneroM,
		Formas: []modelo.Forma{{
			Tipo: modelo.FormaNombreCompleto, Valor: "Tobías Vergara Nieto", Registro: "chat_90002",
		}},
	}
	if err := base.GuardarPersona(ctx, persona); err != nil {
		t.Fatalf("sembrar la persona: %v", err)
	}
	if err := base.GuardarPerfil(ctx, terceroInventado, perfilDelTercero()); err != nil {
		t.Fatalf("sembrar el perfil: %v", err)
	}
}

// corre el núcleo sobre el lote con la bóveda de <datos> y devuelve el
// entities.json que quedó en <salidaDir>.
func corre(t *testing.T, datos, salidaDir string) []byte {
	t.Helper()
	base, err := almacen.Open(filepath.Join(datos, "almacen.db"))
	if err != nil {
		t.Fatalf("abrir la bóveda %s: %v", datos, err)
	}
	defer base.Cerrar()
	n, err := nucleo.Nuevo(nucleo.Opciones{
		Almacen: base, Secreto: []byte(secretoPrueba), Plazo: 2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("núcleo: %v", err)
	}
	resultado, err := n.Corrida(context.Background(), nucleo.Entrada{Registros: loteInventado()})
	if err != nil {
		t.Fatalf("corrida: %v", err)
	}
	if err := salida.Escribe(salidaDir, resultado.Registros, resultado.Entities); err != nil {
		t.Fatalf("escribir la salida: %v", err)
	}
	datosEntidades, err := os.ReadFile(filepath.Join(salidaDir, "entities.json"))
	if err != nil {
		t.Fatalf("leer entities.json: %v", err)
	}
	return datosEntidades
}

// El respaldo es la copia de la bóveda que sobrevive a la pérdida del volumen:
// restaurarlo en otro directorio y volver a correr el mismo lote con el mismo
// secreto tiene que producir un entities.json byte a byte igual (D4, D22).
func TestRestaurarElRespaldoReproduceEntitiesJSON(t *testing.T) {
	raiz := t.TempDir()
	datosA := filepath.Join(raiz, "boveda-a")
	siembra(t, datosA)
	entidadesA := corre(t, datosA, filepath.Join(raiz, "salida-a"))

	// El tercero de la bóveda tiene que aparecer con el perfil que ya tenía: sin
	// eso la comparación de abajo no probaría nada.
	seudonimo, ok := seudonimoDe(t, entidadesA, terceroInventado)
	if !ok {
		t.Fatalf("entities.json no declara al tercero de la bóveda:\n%s", entidadesA)
	}
	if quiero := perfilDelTercero().Nombre; seudonimo != quiero {
		t.Fatalf("seudónimo = %q, quiero el perfil guardado %q", seudonimo, quiero)
	}

	r, err := respaldo.Respalda(datosA, filepath.Join(raiz, "respaldos"), ahoraFijo)
	if err != nil {
		t.Fatalf("respaldo: %v", err)
	}
	if r.Personas == 0 || r.Perfiles == 0 {
		t.Fatalf("la bóveda respaldada vino vacía: %+v", r)
	}

	datosB := filepath.Join(raiz, "boveda-b")
	if _, err := respaldo.Restaura(r.Ruta, datosB); err != nil {
		t.Fatalf("restaura: %v", err)
	}
	entidadesB := corre(t, datosB, filepath.Join(raiz, "salida-b"))
	if !bytes.Equal(entidadesA, entidadesB) {
		t.Fatalf("entities.json cambió tras restaurar el respaldo:\n%s\n---\n%s", entidadesA, entidadesB)
	}
}

// seudonimoDe devuelve el nombre falso declarado para la llave en entities.json.
func seudonimoDe(t *testing.T, datos []byte, llave string) (string, bool) {
	t.Helper()
	var sobre struct {
		Entities []struct {
			EntityID   string            `json:"entity_id"`
			Pseudonyms map[string]string `json:"pseudonyms"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(datos, &sobre); err != nil {
		t.Fatalf("entities.json: %v", err)
	}
	for _, e := range sobre.Entities {
		if e.EntityID == llave {
			return e.Pseudonyms["name"], true
		}
	}
	return "", false
}
