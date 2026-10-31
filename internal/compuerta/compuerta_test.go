package compuerta

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

func indiceDePrueba(t *testing.T) *registro.Indice {
	t.Helper()
	indice, err := registro.Construye([]modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-1", "nombre_completo": "Manuela Castaño Ruiz",
			"num_doc": "1107360039", "correo": "mcastano@gmail.com", "celular": "3148729552",
			"ciudad": "Cali", "direccion": "Diagonal 3A # 78-6 apto 1501",
			"fecha_nacimiento": "1997-07-23",
		}},
	}, registro.Opciones{})
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	return indice
}

func perfilesDePrueba() map[string]modelo.Perfil {
	return map[string]modelo.Perfil{
		"p_cl_1": {Nombre: "Sofía Elena Zambrano", Correo: "szambrano@gmail.com",
			Celular: "3186423850", Cedula: "1107252808"},
	}
}

// Una forma que se escapó del reemplazo se corrige y se reporta (D11, D25).
func TestBarreUnaFormaQueQuedo(t *testing.T) {
	indice := indiceDePrueba(t)
	recs := []modelo.Registro{modelo.ChatRecord{
		ID: "m1", Author: "Ana", Text: "El extracto es de Manuela Castaño, gracias",
	}}
	out, fugas, err := Barre(context.Background(), recs, indice, perfilesDePrueba(), nil)
	if err != nil {
		t.Fatalf("Barre: %v", err)
	}
	if len(fugas) != 1 {
		t.Fatalf("fugas = %+v, quiero 1", fugas)
	}
	texto := out[0].(modelo.ChatRecord).Text
	if strings.Contains(texto, "Manuela") {
		t.Fatalf("la forma sigue: %q", texto)
	}
	if !strings.Contains(texto, "Sofía Elena") {
		t.Fatalf("no reemplazó por el perfil: %q", texto)
	}
}

// Una salida limpia no reporta fugas ni cambia.
func TestSalidaLimpiaNoCambia(t *testing.T) {
	indice := indiceDePrueba(t)
	recs := []modelo.Registro{modelo.ChatRecord{ID: "m1", Author: "Ana", Text: "Todo en orden"}}
	out, fugas, err := Barre(context.Background(), recs, indice, perfilesDePrueba(), nil)
	if err != nil {
		t.Fatalf("Barre: %v", err)
	}
	if len(fugas) != 0 {
		t.Fatalf("fugas = %+v", fugas)
	}
	if out[0].(modelo.ChatRecord).Text != "Todo en orden" {
		t.Fatalf("cambió una salida limpia")
	}
}

// Un hash sin sal de una forma real también es PII (D25).
func TestBarreHashSinSal(t *testing.T) {
	indice := indiceDePrueba(t)
	suma := md5.Sum([]byte("1107360039"))
	digesto := hex.EncodeToString(suma[:])
	recs := []modelo.Registro{modelo.ChatRecord{
		ID: "m1", Author: "Ana", Text: "referencia " + digesto + " del cliente",
	}}
	out, fugas, err := Barre(context.Background(), recs, indice, perfilesDePrueba(), nil)
	if err != nil {
		t.Fatalf("Barre: %v", err)
	}
	texto := out[0].(modelo.ChatRecord).Text
	if strings.Contains(texto, digesto) {
		t.Fatalf("el hash sigue en la salida: %q", texto)
	}
	if len(fugas) != 1 || !fugas[0].Hash {
		t.Fatalf("fugas = %+v, quiero una de hash", fugas)
	}
}

// La compuerta no depende del orden de los registros para decidir el dueño:
// una forma compartida se resuelve por la llave menor (D25).
func TestFormaCompartidaUsaLaLlaveMenor(t *testing.T) {
	indice, err := registro.Construye([]modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-1", "nombre_completo": "Camila Arango Ruiz", "num_doc": "1000000001",
		}},
		modelo.TableRecord{ID: "clientes:CL-2", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-2", "nombre_completo": "Camila Arango Díaz", "num_doc": "1000000002",
		}},
	}, registro.Opciones{})
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	recs := []modelo.Registro{modelo.ChatRecord{ID: "m1", Text: "Camila Arango llamó"}}
	out, fugas, err := Barre(context.Background(), recs, indice, map[string]modelo.Perfil{
		"p_cl_1": {Nombre: "Bea Ruiz Díaz"},
		"p_cl_2": {Nombre: "Dora Pardo Gil"},
	}, nil)
	if err != nil {
		t.Fatalf("Barre: %v", err)
	}
	if len(fugas) == 0 {
		t.Fatal("no reporté la forma compartida")
	}
	texto := out[0].(modelo.ChatRecord).Text
	if strings.Contains(texto, "Camila Arango") {
		t.Fatalf("la forma compartida sigue: %q", texto)
	}
}
