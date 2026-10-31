package resuelve

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// falsoAlmacen guarda lo que la resolución persiste, sin SQLite.
type falsoAlmacen struct {
	corpus  []modelo.Persona
	vivo    []modelo.Persona
	dudosos []modelo.Mencion
	err     error
}

func (f *falsoAlmacen) GuardarPersona(_ context.Context, p modelo.Persona) error {
	f.corpus = append(f.corpus, p)
	return f.err
}

func (f *falsoAlmacen) GuardarPersonaVivo(_ context.Context, p modelo.Persona) error {
	f.vivo = append(f.vivo, p)
	return f.err
}

func (f *falsoAlmacen) AnotaDudoso(_ context.Context, m modelo.Mencion) error {
	f.dudosos = append(f.dudosos, m)
	return f.err
}

// falsoModelo responde la opción pedida con la probabilidad pedida.
type falsoModelo struct {
	opcion   string
	riesgo   float64
	estados  []string
	pregunta modelo.Pregunta
}

func (f *falsoModelo) Nombre() string { return "falso" }

func (f *falsoModelo) Decide(_ context.Context, estado string, p modelo.Pregunta) (modelo.Respuesta, error) {
	f.estados = append(f.estados, estado)
	f.pregunta = p
	return modelo.Respuesta{
		Pregunta: p.ID, Opcion: f.opcion,
		Probabilidades: map[string]float64{f.opcion: f.riesgo},
	}, nil
}

// dosHomonimas arma un registro con dos clientas que comparten nombre.
func dosHomonimas(t *testing.T) *registro.Indice {
	t.Helper()
	indice, err := registro.Construye([]modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-1", "nombre_completo": "Camila Arango Ruiz",
			"num_doc": "1000000001", "correo": "camila1@gmail.com", "ciudad": "Cali",
		}},
		modelo.TableRecord{ID: "clientes:CL-2", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-2", "nombre_completo": "Camila Arango Díaz",
			"num_doc": "1000000002", "correo": "camila2@gmail.com", "ciudad": "Bogotá",
		}},
	}, registro.Opciones{})
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	return indice
}

func mencion(registroID, campo, valor string, forma modelo.TipoForma) modelo.Mencion {
	return modelo.Mencion{
		Registro: registroID, Fuente: modelo.FuenteChats, Campo: modelo.CampoTexto(campo),
		Inicio: 0, Fin: len(valor), Valor: valor, Tipo: modelo.MencionPersona, Forma: forma,
	}
}

// El autor del hilo desempata entre homónimas: evidencia de contexto, no de
// nombre (D10, CE-01, CE-02).
func TestContextoDelHiloDesempataHomonimas(t *testing.T) {
	// El uid U1 pertenece a CL-1; el mensaje que menciona el nombre compartido
	// va en su hilo, así que la mención es de CL-1.
	recs := []modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-1", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-1", "nombre_completo": "Camila Arango Ruiz",
			"num_doc": "1000000001", "correo": "camila1@gmail.com", "ciudad": "Cali",
		}},
		modelo.TableRecord{ID: "clientes:CL-2", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-2", "nombre_completo": "Camila Arango Díaz",
			"num_doc": "1000000002", "correo": "camila2@gmail.com", "ciudad": "Bogotá",
		}},
		modelo.ChatRecord{ID: "m1", ThreadID: "hilo", Author: "Camila Arango Ruiz", Text: "Hola, soy Camila"},
		modelo.ChatRecord{ID: "m2", ThreadID: "hilo", Author: "Ana", Text: "Hola Camila Arango"},
	}
	indice, err := registro.Construye(recs, registro.Opciones{Autores: map[string]string{"m1": "U1"}})
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	indice.AgregaForma("p_cl_1", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	indice.AgregaForma("p_cl_2", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	r := Nuevo(Opciones{Indice: indice})
	out, err := r.Resuelve(context.Background(), recs, []modelo.Mencion{
		mencion("m2", "text", "Camila Arango", modelo.FormaNombreCorto),
	})
	if err != nil {
		t.Fatalf("Resuelve: %v", err)
	}
	if out[0].Persona != "p_cl_1" || out[0].Regla != "contexto" {
		t.Fatalf("mención = %+v, quiero p_cl_1 por contexto", out[0])
	}
}

// La fila del maestro desempata cuando la mención cae en sus propios datos.
func TestFilaDelMaestroDesempata(t *testing.T) {
	indice := dosHomonimas(t)
	indice.AgregaForma("p_cl_1", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	indice.AgregaForma("p_cl_2", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	r := Nuevo(Opciones{Indice: indice})
	recs := []modelo.Registro{
		modelo.TableRecord{ID: "clientes:CL-2", Table: "clientes", Columns: map[string]string{
			"cliente_id": "CL-2", "nombre_completo": "Camila Arango Díaz",
			"notas": "Camila Arango pidió cambio de canal",
		}},
	}
	out, _ := r.Resuelve(context.Background(), recs, []modelo.Mencion{
		mencion("clientes:CL-2", "columns.notas", "Camila Arango", modelo.FormaNombreCorto),
	})
	if out[0].Persona != "p_cl_2" {
		t.Fatalf("mención = %+v, quiero p_cl_2", out[0])
	}
}

// El modelo desempata con fichas abstractas: el nombre no viaja y la respuesta
// se acepta solo con confianza suficiente (D26, D31).
func TestDesempateConModelo(t *testing.T) {
	indice := dosHomonimas(t)
	indice.AgregaForma("p_cl_1", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	indice.AgregaForma("p_cl_2", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	m := &falsoModelo{opcion: "B", riesgo: 0.88}
	r := Nuevo(Opciones{Indice: indice, Modelo: m})
	out, err := r.Resuelve(context.Background(), []modelo.Registro{
		modelo.ChatRecord{ID: "m1", Author: "Ana", Text: "Camila Arango llamó"},
	}, []modelo.Mencion{mencion("m1", "text", "Camila Arango", modelo.FormaNombreCorto)})
	if err != nil {
		t.Fatalf("Resuelve: %v", err)
	}
	if out[0].Persona != "p_cl_2" || out[0].Regla != "desempate_modelo" {
		t.Fatalf("mención = %+v", out[0])
	}
	if len(m.estados) != 1 {
		t.Fatalf("llamadas al modelo = %d", len(m.estados))
	}
	if containsFold(m.estados[0], "Camila Arango") == false {
		t.Logf("estado sin el fragmento: %q", m.estados[0])
	}
	for _, nombre := range []string{"Arango Ruiz", "Arango Díaz"} {
		if containsFold(m.estados[0], nombre) {
			t.Errorf("el estado lleva el nombre completo de una candidata: %q", m.estados[0])
		}
	}
}

// Sin confianza suficiente no se escoge: se crea una persona nueva dudosa (D19).
func TestEmpateSinConfianzaCreaPersonaDudosa(t *testing.T) {
	indice := dosHomonimas(t)
	indice.AgregaForma("p_cl_1", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	indice.AgregaForma("p_cl_2", modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: "Camila Arango"})
	almacen := &falsoAlmacen{}
	r := Nuevo(Opciones{Indice: indice, Modelo: &falsoModelo{opcion: "A", riesgo: 0.4}, Almacen: almacen})
	out, err := r.Resuelve(context.Background(), []modelo.Registro{
		modelo.ChatRecord{ID: "m1", Author: "Ana", Text: "Camila Arango llamó"},
	}, []modelo.Mencion{mencion("m1", "text", "Camila Arango", modelo.FormaNombreCorto)})
	if err != nil {
		t.Fatalf("Resuelve: %v", err)
	}
	if out[0].Persona == "" || out[0].Persona == "p_cl_1" || out[0].Persona == "p_cl_2" {
		t.Fatalf("mención = %+v, quiero una persona nueva", out[0])
	}
	if len(almacen.corpus) != 1 {
		t.Fatalf("personas guardadas = %d, quiero 1", len(almacen.corpus))
	}
	if len(almacen.dudosos) != 1 {
		t.Fatalf("dudosos = %d, quiero 1", len(almacen.dudosos))
	}
	if almacen.dudosos[0].Confianza >= 0.6 {
		t.Fatalf("el dudoso no es de baja confianza: %+v", almacen.dudosos[0])
	}
}

// Un nombre único se resuelve sin modelo.
func TestNombreUnicoNoConsultaElModelo(t *testing.T) {
	indice := dosHomonimas(t)
	m := &falsoModelo{opcion: "A", riesgo: 0.99}
	r := Nuevo(Opciones{Indice: indice, Modelo: m})
	out, _ := r.Resuelve(context.Background(), nil, []modelo.Mencion{
		mencion("m1", "text", "Camila Arango Ruiz", modelo.FormaNombreCompleto),
	})
	if out[0].Persona != "p_cl_1" || out[0].Regla != "nombre_unico" {
		t.Fatalf("mención = %+v", out[0])
	}
	if len(m.estados) != 0 {
		t.Fatalf("consulté al modelo sin necesidad")
	}
}

// Las personas de /anonymize van a la tabla viva (D24).
func TestPersonaNuevaEnVivoVaATablaVivo(t *testing.T) {
	indice, err := registro.Construye(nil, registro.Opciones{})
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	almacen := &falsoAlmacen{}
	r := Nuevo(Opciones{Indice: indice, Almacen: almacen, Vivo: true})
	_, err = r.Resuelve(context.Background(), nil, []modelo.Mencion{
		mencion("m1", "author", "Ana Gómez", modelo.FormaNombreCompleto),
	})
	if err != nil {
		t.Fatalf("Resuelve: %v", err)
	}
	if len(almacen.vivo) != 1 || len(almacen.corpus) != 0 {
		t.Fatalf("vivo=%d corpus=%d", len(almacen.vivo), len(almacen.corpus))
	}
}

// Un identificador duro no se cuestiona aunque haya homónimas.
func TestIdentificadorDuroGana(t *testing.T) {
	indice := dosHomonimas(t)
	r := Nuevo(Opciones{Indice: indice})
	out, _ := r.Resuelve(context.Background(), nil, []modelo.Mencion{
		mencion("m1", "text", "camila2@gmail.com", modelo.FormaCorreo),
	})
	if out[0].Persona != "p_cl_2" || out[0].Regla != "id_duro" {
		t.Fatalf("mención = %+v", out[0])
	}
}

// Un nombre de pila suelto sin dueño único no inventa una persona (D25).
func TestNombreDePilaSueltoNoCreaPersona(t *testing.T) {
	indice := dosHomonimas(t)
	almacen := &falsoAlmacen{}
	r := Nuevo(Opciones{Indice: indice, Almacen: almacen})
	out, _ := r.Resuelve(context.Background(), nil, []modelo.Mencion{
		mencion("m1", "text", "Camila", modelo.FormaNombrePila),
	})
	if out[0].Persona != "" {
		t.Fatalf("inventé una persona: %+v", out[0])
	}
	if len(almacen.corpus) != 0 || len(almacen.dudosos) != 0 {
		t.Fatalf("persistí algo: %+v %+v", almacen.corpus, almacen.dudosos)
	}
}

// Un error de la bóveda no se traga: la corrida falla ruidosamente.
func TestErrorDeAlmacenSePropaga(t *testing.T) {
	indice, _ := registro.Construye(nil, registro.Opciones{})
	r := Nuevo(Opciones{Indice: indice, Almacen: &falsoAlmacen{err: errors.New("disco lleno")}, Timeout: time.Second})
	_, err := r.Resuelve(context.Background(), nil, []modelo.Mencion{
		mencion("m1", "text", "Ana Gómez", modelo.FormaNombreCompleto),
	})
	if err == nil {
		t.Fatal("quería error del almacén")
	}
}

func containsFold(texto, sub string) bool {
	return len(sub) > 0 && len(texto) >= len(sub) && (indexFold(texto, sub) >= 0)
}

func indexFold(texto, sub string) int {
	for i := 0; i+len(sub) <= len(texto); i++ {
		if equalFold(texto[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
