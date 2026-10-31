package registro

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/chats"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/tablas"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
)

// raiz es la raíz del repo desde este paquete.
const raiz = "../.."

// muestra se salta la prueba si sample/ no está descargado (es un directorio
// ignorado por git).
func muestra(t *testing.T, partes ...string) string {
	t.Helper()
	ruta := filepath.Join(append([]string{raiz, "sample"}, partes...)...)
	if _, err := os.Stat(ruta); err != nil {
		t.Skipf("sin muestra %s: %v", ruta, err)
	}
	return ruta
}

// armaMuestra construye el registro del lote completo de sample/input.
func armaMuestra(t *testing.T) *Indice {
	t.Helper()
	tablasReales, err := tablas.Leer(muestra(t, "input", "tables"))
	if err != nil {
		t.Fatalf("lee tablas: %v", err)
	}
	corpus, err := chats.Cargar(muestra(t, "input", "chats"))
	if err != nil {
		t.Fatalf("lee chats: %v", err)
	}
	recs := make([]modelo.Registro, 0, len(tablasReales)+len(corpus.Registros))
	for _, r := range tablasReales {
		recs = append(recs, r)
	}
	for _, r := range corpus.Registros {
		recs = append(recs, r)
	}
	indice, err := Construye(recs, Opciones{Usuarios: corpus.Usuarios, Autores: corpus.Autores, Canales: corpus.Canales})
	if err != nil {
		t.Fatalf("Construye: %v", err)
	}
	return indice
}

// Cada persona de la muestra tiene que declarar las formas de su test de
// consistencia (C12: "cada persona de la muestra tiene las forms de sus tests").
func TestMuestraDeclaraLasFormasDeLosTests(t *testing.T) {
	indice := armaMuestra(t)
	casos := map[string][]string{
		"p_cl_88627": {
			"Manuela Castaño", "Manuela Castaño Ruiz", "Manuela",
			"1107360039", "1.107.360.039", "mcastano@gmail.com",
			"3148729552", "Diagonal 3A # 78-6 apto 1501",
			"1997-07-23", "manuela_castano",
		},
		"p_cl_19127": {
			"Nohora Salazar", "Nohora Salazar Ramírez", "32390356",
			"nsalazar@gmail.com", "3106128485", "Calle 98 Bis # 91-10 of 406",
			"1982-04-07",
		},
		"p_cl_92356": {"Esteban Darío Castaño Ospina", "94184267", "ecastano@gmail.com"},
		"p_cl_72856": {"Alejandra Castaño Mejía", "63263076", "acastano@gmail.com"},
	}
	for llave, queridas := range casos {
		p, ok := indice.Persona(llave)
		if !ok {
			t.Errorf("falta la persona %s", llave)
			continue
		}
		for _, querida := range queridas {
			if len(indice.Busca(querida)) == 0 {
				t.Errorf("%s no declara la forma %q", llave, querida)
			}
			encontrada := false
			for _, f := range p.Formas {
				if norm.Norm(f.Valor) == norm.Norm(querida) {
					encontrada = true
					break
				}
			}
			if !encontrada {
				t.Errorf("%s no tiene entre sus formas %q", llave, querida)
			}
		}
	}
}

// El usuario de chat se une al maestro por la parte local del correo, y cada
// mensaje queda atado a su autor por el uid (D10, CE-02).
func TestMuestraUneUsuariosYAutores(t *testing.T) {
	indice := armaMuestra(t)
	casos := map[string]string{
		"U0V6WYVYG": "p_cl_88627",
	}
	for uid, querida := range casos {
		p, ok := indice.PorUID(uid)
		if !ok {
			t.Errorf("el uid %s no tiene persona", uid)
			continue
		}
		if p.Llave != querida {
			t.Errorf("uid %s = %s, quiero %s", uid, p.Llave, querida)
		}
	}
	autor, ok := indice.AutorDe("fea922de-00f1-5c98-a036-4c7540983788")
	if !ok || autor.Llave != "p_cl_88627" {
		t.Fatalf("AutorDe = %+v, quiero p_cl_88627", autor)
	}
	// Los ids internos resuelven a su persona.
	for referencia, querida := range map[string]string{
		"CL-88627":   "p_cl_88627",
		"CL-19127":   "p_cl_19127",
		"1107360039": "p_cl_88627",
	} {
		p, ok := indice.PorReferencia(referencia)
		if !ok || p.Llave != querida {
			t.Errorf("PorReferencia(%q) = %+v, quiero %s", referencia, p, querida)
		}
	}
}

// El bot no es persona y nunca entra al registro (supuesto 4).
func TestMuestraNoRegistraElBot(t *testing.T) {
	indice := armaMuestra(t)
	for _, p := range indice.Personas() {
		for _, f := range p.Formas {
			if norm.Norm(f.Valor) == norm.Norm("Kipi") {
				t.Errorf("%s declara al bot como forma", p.Llave)
			}
		}
	}
}

// Dos filas con el mismo nombre son dos personas: la llave sale de la evidencia
// (D2, D10), y el género sale del catálogo (D18).
func TestHomonimosNoSeFusionan(t *testing.T) {
	recs := []modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-1", "nombre_completo": "Camila Arango Ruiz",
			"num_doc": "1000000001", "correo": "camila1@gmail.com", "ciudad": "Cali",
		}},
		modelo.TableRecord{ID: "clientes:CL-2", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-2", "nombre_completo": "Camila Arango Ruiz",
			"num_doc": "1000000002", "correo": "camila2@gmail.com", "ciudad": "Cali",
		}},
	}
	indice, err := Construye(recs, Opciones{})
	if err != nil {
		t.Fatalf("Construye: %v", err)
	}
	if len(indice.Personas()) != 2 {
		t.Fatalf("personas = %d, quiero 2", len(indice.Personas()))
	}
	if got := indice.Busca("Camila Arango"); len(got) != 2 {
		t.Fatalf("Busca(nombre compartido) = %d personas, quiero 2", len(got))
	}
	for _, p := range indice.Personas() {
		// El catálogo no puede repetir nombres reales (C9), así que sin
		// ocupación el género queda indeterminado: lo decide el modelo (D27).
		if p.Genero != modelo.GeneroIndeterminado && p.Genero != modelo.GeneroF {
			t.Errorf("%s: género = %q", p.Llave, p.Genero)
		}
		if p.Rol != modelo.RolCliente {
			t.Errorf("%s: rol = %q, quiero cliente", p.Llave, p.Rol)
		}
	}
}

// Fusionar la bóveda no pisa lo que ya está (D4, D22): la persona y sus formas
// viejas mandan, y las nuevas solo se suman.
func TestFusionaNoPisaLoExistente(t *testing.T) {
	viejo, err := Construye([]modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-1", "nombre_completo": "Ana Pérez Gómez",
			"num_doc": "1000000001", "correo": "ana@x.com",
		}},
	}, Opciones{})
	if err != nil {
		t.Fatalf("Construye viejo: %v", err)
	}
	nuevo, err := Construye([]modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-1", "nombre_completo": "Ana María Pérez Gómez",
			"num_doc": "1000000001", "correo": "ana@x.com",
		}},
	}, Opciones{})
	if err != nil {
		t.Fatalf("Construye nuevo: %v", err)
	}
	nuevo.Fusiona(viejo)
	p, _ := nuevo.Persona("p_cl_1")
	if p.Genero != viejo.Personas()[0].Genero {
		t.Fatalf("el género cambió al fusionar")
	}
	if len(nuevo.Busca("Ana Pérez")) == 0 {
		t.Fatalf("se perdió la forma vieja")
	}
	if len(nuevo.Busca("Ana María Pérez")) == 0 {
		t.Fatalf("no se sumó la forma nueva")
	}
}

// Un tercero que solo existe en el chat recibe llave p_t_ y se puede volver a
// encontrar por su nombre (D22, D23, CE-19).
func TestTerceroSinFilaRecibeLlaveDeContexto(t *testing.T) {
	usuarios := map[string]chats.Usuario{
		"U1": {UID: "U1", Nombre: "Hernando Gómez", NombreCompleto: "Hernando Gómez", Usuario: "hgomez"},
	}
	indice, err := Construye(nil, Opciones{Usuarios: usuarios})
	if err != nil {
		t.Fatalf("Construye: %v", err)
	}
	p, ok := indice.PorUID("U1")
	if !ok {
		t.Fatal("el tercero no quedó en el registro")
	}
	if len(p.Llave) < 5 || p.Llave[:4] != "p_t_" {
		t.Fatalf("llave = %q, quiero p_t_…", p.Llave)
	}
	if got := indice.Busca("Hernando Gómez"); len(got) != 1 || got[0].Llave != p.Llave {
		t.Fatalf("Busca = %+v", got)
	}
}

// Las formas salen en orden estable y sin repetir: la salida y la compuerta lo
// necesitan para ser deterministas.
func TestFormasEsEstable(t *testing.T) {
	indice := armaMuestra(t)
	primera := indice.Formas()
	segunda := indice.Formas()
	if len(primera) != len(segunda) {
		t.Fatalf("formas = %d y %d", len(primera), len(segunda))
	}
	vistas := map[string]bool{}
	for i := range primera {
		if primera[i] != segunda[i] {
			t.Fatalf("forma %d cambió entre llamadas", i)
		}
		clave := string(primera[i].Tipo) + "\x00" + primera[i].Valor
		if vistas[clave] {
			t.Fatalf("forma repetida: %s %q", primera[i].Tipo, primera[i].Valor)
		}
		vistas[clave] = true
	}
}

// La ficha que ve el LLM no lleva el nombre: solo ids, ciudad y referencias
// (D26).
func TestFichaNoLlevaElNombre(t *testing.T) {
	indice := armaMuestra(t)
	ficha, ok := indice.Ficha("p_cl_88627")
	if !ok {
		t.Fatal("sin ficha")
	}
	if ficha.Ciudad != "Cali" {
		t.Errorf("ciudad = %q, quiero Cali", ficha.Ciudad)
	}
	if len(ficha.Ref) == 0 {
		t.Errorf("ficha sin referencias")
	}
	p, _ := indice.Persona("p_cl_88627")
	for _, ref := range ficha.Ref {
		if norm.Norm(ref) == norm.Norm(p.Formas[0].Valor) {
			t.Errorf("la ficha lleva una forma real")
		}
	}
}

// Las dos llaves de una misma persona no se duplican al fusionar: el registro
// crece sin crear homónimos falsos.
func TestFusionaConservaOrden(t *testing.T) {
	a, _ := Construye([]modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-2", Table: "clientes", Columns: map[string]string{"cliente_id": "CL-2", "nombre_completo": "Bea Ruiz Díaz"}},
	}, Opciones{})
	b, _ := Construye([]modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes", Columns: map[string]string{"cliente_id": "CL-1", "nombre_completo": "Ana Pérez Gómez"}},
	}, Opciones{})
	a.Fusiona(b)
	llaves := make([]string, 0)
	for _, p := range a.Personas() {
		llaves = append(llaves, p.Llave)
	}
	if !sort.StringsAreSorted(llaves) && len(llaves) != 2 {
		t.Fatalf("llaves = %v", llaves)
	}
	if len(llaves) != 2 {
		t.Fatalf("llaves = %v, quiero 2", llaves)
	}
}

// Un cliente migrado de Págalo con la cédula de un cliente de Kipu es la misma
// persona: conserva la llave vieja (y con ella su seudónimo en la bóveda), y el
// id PG- de clientes, transacciones y PQR resuelve a esa persona (CE-45).
func TestClienteMigradoConLaMismaCedulaEsLaMismaPersona(t *testing.T) {
	fila := func(id string) modelo.TableRecord {
		return modelo.TableRecord{ID: "clientes:" + id, Table: "clientes", Columns: map[string]string{
			"cliente_id": id, "nombre_completo": "Rosa Elvira Quintana", "tipo_doc": "CC",
			"num_doc": "52.111.222", "correo": "rquintana@mail.com", "celular": "3001112233",
		}}
	}
	indice, err := Construye([]modelo.Registro{
		fila("CL-10001"),
		fila("PG-2001"),
		modelo.TableRecord{ID: "transacciones:TX-1", Table: "transacciones", Columns: map[string]string{
			"tx_id": "TX-1", "cliente_id": "PG-2001",
		}},
		modelo.TableRecord{ID: "pqrs:PQR-1", Table: "pqrs", Columns: map[string]string{
			"radicado": "PQR-1", "cliente_id": "PG-2001", "cedula": "52111222",
		}},
	}, Opciones{})
	if err != nil {
		t.Fatalf("Construye: %v", err)
	}
	if _, ok := indice.Persona("p_cl_pg-2001"); ok {
		t.Fatalf("creé una segunda persona para el cliente migrado")
	}
	for _, ref := range []string{"PG-2001", "TX-1", "PQR-1", "CL-10001"} {
		p, ok := indice.PorReferencia(ref)
		if !ok || p.Llave != "p_cl_10001" {
			t.Errorf("%s resuelve a %q, quiero p_cl_10001", ref, p.Llave)
		}
	}
	if dueños := indice.Busca("Rosa Elvira Quintana"); len(dueños) != 1 {
		t.Errorf("el nombre tiene %d dueños, quiero 1", len(dueños))
	}
}
