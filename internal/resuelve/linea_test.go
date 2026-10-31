package resuelve

import (
	"context"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// resuelveEn corre una sola mención del nombre corto compartido por las dos
// homónimas de dosHomonimas, más un empleado homónimo opcional (CE-52).
func resuelveEn(t *testing.T, recs []modelo.Registro, m modelo.Mencion) modelo.Mencion {
	t.Helper()
	indice := dosHomonimas(t)
	indice.AgregaForma("p_cl_1", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	indice.AgregaForma("p_cl_2", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	indice.AgregaPersona(modelo.Persona{Llave: "p_kp_9", Rol: modelo.RolEmpleado})
	indice.AgregaForma("p_kp_9", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	out, err := Nuevo(Opciones{Indice: indice}).Resuelve(context.Background(), recs, []modelo.Mencion{m})
	if err != nil {
		t.Fatalf("Resuelve: %v", err)
	}
	return out[0]
}

// Un id del maestro en la misma línea desempata: "Camila Arango (CL-2)" o la
// fila de una tabla markdown con su radicado.
func TestIdEnLaMismaLineaDesempataHomonimas(t *testing.T) {
	texto := "| PQR-1 | Otra | x |\n| CL-2 | Camila Arango | Cobro |"
	recs := []modelo.Registro{modelo.DocRecord{ID: "doc_1", Body: texto}}
	m := mencion("doc_1", "body", "Camila Arango", modelo.FormaNombreCorto)
	m.Inicio = len("| PQR-1 | Otra | x |\n| CL-2 | ")
	m.Fin = m.Inicio + len("Camila Arango")
	if got := resuelveEn(t, recs, m); got.Persona != "p_cl_2" {
		t.Fatalf("mención = %+v, quiero p_cl_2 por el id de su línea", got)
	}
}

// Dos ids de candidatas distintas en la línea no deciden nada.
func TestDosIdsEnLaLineaNoDesempatan(t *testing.T) {
	texto := "Camila Arango (CL-1) y Camila Arango (CL-2)"
	recs := []modelo.Registro{modelo.DocRecord{ID: "doc_1", Body: texto}}
	m := mencion("doc_1", "body", "Camila Arango", modelo.FormaNombreCorto)
	if got := resuelveEn(t, recs, m); got.Regla == "linea" {
		t.Fatalf("mención = %+v, no debe decidir con dos ids", got)
	}
}

// La fila de pqrs pertenece al cliente de su cliente_id.
func TestFilaDePQRDesempataPorClienteID(t *testing.T) {
	recs := []modelo.Registro{modelo.TableRecord{ID: "pqrs:PQR-7", Table: "pqrs", Columns: map[string]string{
		"radicado": "PQR-7", "cliente_id": "CL-1", "descripcion": "Atentamente, Camila Arango.",
	}}}
	m := mencion("pqrs:PQR-7", "columns.descripcion", "Camila Arango", modelo.FormaNombreCorto)
	if got := resuelveEn(t, recs, m); got.Persona != "p_cl_1" {
		t.Fatalf("mención = %+v, quiero p_cl_1 por cliente_id", got)
	}
}

// reporta_a nombra siempre a un empleado.
func TestReportaANombraEmpleado(t *testing.T) {
	recs := []modelo.Registro{modelo.TableRecord{ID: "empleados:KP-1", Table: "empleados", Columns: map[string]string{
		"id": "KP-1", "nombre": "Ana Pérez", "reporta_a": "Camila Arango",
	}}}
	m := mencion("empleados:KP-1", "columns.reporta_a", "Camila Arango", modelo.FormaNombreCorto)
	if got := resuelveEn(t, recs, m); got.Persona != "p_kp_9" {
		t.Fatalf("mención = %+v, quiero el empleado p_kp_9", got)
	}
}
