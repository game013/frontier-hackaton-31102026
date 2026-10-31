package cuasi

import (
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/catalogo"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// indice arma el registro de personas de un lote de prueba.
func indice(t *testing.T, recs []modelo.Registro) *registro.Indice {
	t.Helper()
	i, err := registro.Construye(recs, registro.Opciones{})
	if err != nil {
		t.Fatalf("registro.Construye: %v", err)
	}
	return i
}

// filaDeCliente es una fila del maestro inventada: ningún dato sale del corpus.
func filaDeCliente(barrio string) modelo.TableRecord {
	return modelo.TableRecord{
		ID: "clientes:CL-10001", Table: "clientes",
		Columns: map[string]string{
			"cliente_id": "CL-10001", "nombre_completo": "Elena Perez Rojas",
			"ciudad": "Cali", "barrio": barrio, "ocupacion": "fotógrafa",
			"fecha_vinculacion": "2025-08-21", "notas": "Sin novedad.",
		},
	}
}

// En la fila del maestro la ocupación va a su sector y el barrio a otro barrio
// de la misma ciudad; el resto de las columnas queda igual (D9, DECISIONS §1.3).
func TestGeneralizaFilaDelMaestro(t *testing.T) {
	fila := filaDeCliente("Granada")
	recs := []modelo.Registro{fila}
	out, err := Generaliza(recs, indice(t, recs))
	if err != nil {
		t.Fatalf("Generaliza: %v", err)
	}
	got := out[0].(modelo.TableRecord)

	quiero := map[string]string{
		"cliente_id":        "CL-10001",
		"nombre_completo":   "Elena Perez Rojas",
		"ciudad":            "Cali",
		"ocupacion":         "profesional de las industrias creativas",
		"fecha_vinculacion": "2025-08-21",
		"notas":             "Sin novedad.",
	}
	for columna, valor := range quiero {
		if got.Columns[columna] != valor {
			t.Errorf("columns[%s] = %q, quiero %q", columna, got.Columns[columna], valor)
		}
	}
	if len(got.Columns) != len(fila.Columns) {
		t.Fatalf("%d columnas, quiero %d: no se agregan ni se caen columnas",
			len(got.Columns), len(fila.Columns))
	}
	barrios, ok := catalogo.Barrios("Cali")
	if !ok {
		t.Fatal("Cali no está en el catálogo de barrios")
	}
	nuevo := got.Columns["barrio"]
	if nuevo == "Granada" {
		t.Errorf("barrio = %q: el barrio real no sobrevive en el maestro", nuevo)
	}
	if !enLista(barrios, nuevo) {
		t.Errorf("barrio = %q, quiero uno de Cali", nuevo)
	}
}

// El barrio del maestro no depende del orden del lote ni del azar: la misma
// persona recibe siempre el mismo barrio, sola o entre otras filas.
func TestBarrioDelMaestroEsEstable(t *testing.T) {
	granada := filaDeCliente("Granada")
	otra := filaDeCliente("El Peñón")
	otra.ID = "clientes:CL-10002"
	otra.Columns["cliente_id"] = "CL-10002"

	sola := generaliza(t, []modelo.Registro{granada})
	juntas := generaliza(t, []modelo.Registro{granada, otra})
	reves := generaliza(t, []modelo.Registro{otra, granada})
	delante := juntas[0].(modelo.TableRecord).Columns["barrio"]
	atras := reves[1].(modelo.TableRecord).Columns["barrio"]
	siSola := sola[0].(modelo.TableRecord).Columns["barrio"]
	if delante != siSola || atras != siSola {
		t.Fatalf("barrio = %q, %q y %q: la generalización tiene que ser determinista", siSola, delante, atras)
	}
	if a, b := juntas[1].(modelo.TableRecord).Columns["barrio"], reves[0].(modelo.TableRecord).Columns["barrio"]; a != b {
		t.Fatalf("barrio = %q y %q: la segunda fila tampoco puede cambiar con el orden", a, b)
	}
}

func generaliza(t *testing.T, recs []modelo.Registro) []modelo.Registro {
	t.Helper()
	out, err := Generaliza(recs, indice(t, recs))
	if err != nil {
		t.Fatalf("Generaliza: %v", err)
	}
	return out
}

func enLista(lista []string, valor string) bool {
	for _, v := range lista {
		if v == valor {
			return true
		}
	}
	return false
}
