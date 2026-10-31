package detecta

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/chats"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/tablas"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

const raiz = "../.."

func muestra(t *testing.T, partes ...string) string {
	t.Helper()
	ruta := filepath.Join(append([]string{raiz, "sample"}, partes...)...)
	if _, err := os.Stat(ruta); err != nil {
		t.Skipf("sin muestra %s: %v", ruta, err)
	}
	return ruta
}

// corpusMuestra arma registros e índice de sample/input.
func corpusMuestra(t *testing.T) ([]modelo.Registro, *registro.Indice) {
	t.Helper()
	tablasReales, err := tablas.Leer(muestra(t, "input", "tables"))
	if err != nil {
		t.Fatalf("tablas: %v", err)
	}
	chatCorpus, err := chats.Cargar(muestra(t, "input", "chats"))
	if err != nil {
		t.Fatalf("chats: %v", err)
	}
	docsReales, err := docs.NuevoLector().Directorio(context.Background(), muestra(t, "input", "docs"))
	if err != nil {
		t.Fatalf("docs: %v", err)
	}
	var recs []modelo.Registro
	for _, r := range tablasReales {
		recs = append(recs, r)
	}
	for _, r := range chatCorpus.Registros {
		recs = append(recs, r)
	}
	for _, r := range docsReales {
		recs = append(recs, r)
	}
	indice, err := registro.Construye(recs, registro.Opciones{
		Usuarios: chatCorpus.Usuarios, Autores: chatCorpus.Autores, Canales: chatCorpus.Canales,
	})
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	return recs, indice
}

func allowDeMuestra(t *testing.T) *Allowlist {
	t.Helper()
	a, err := Nueva([]Entrada{
		{Nombre: "Bancolombia", Tipo: "organización", Motivo: "banco"},
		{Nombre: "Superfinanciera", Tipo: "organización", Motivo: "regulador"},
	}, nil, nil, nil, []string{"Claro"})
	if err != nil {
		t.Fatalf("allowlist: %v", err)
	}
	return a
}

// Halla devuelve la mención que cubre el valor en ese registro, si existe.
func halla(menciones []modelo.Mencion, registro, valor string) (modelo.Mencion, bool) {
	for _, m := range menciones {
		if m.Registro == registro && m.Valor == valor {
			return m, true
		}
	}
	return modelo.Mencion{}, false
}

// Cada forma de la persona de la muestra se detecta con su tipo, para que el
// reemplazo conserve el estilo (C14, CE-03, CE-04, CE-08, CE-21).
func TestMuestraDetectaFormasConEstilo(t *testing.T) {
	recs, indice := corpusMuestra(t)
	d, err := Nuevo(Opciones{Indice: indice, Allowlist: allowDeMuestra(t)})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	menciones, err := d.Detecta(context.Background(), recs)
	if err != nil {
		t.Fatalf("Detecta: %v", err)
	}
	casos := []struct {
		registro string
		valor    string
		forma    modelo.TipoForma
		persona  string
	}{
		{"fea922de-00f1-5c98-a036-4c7540983788", "Manuela Castaño", modelo.FormaNombreCorto, "p_cl_88627"},
		{"05004714-695f-52be-a0a9-c39224133858", "1.107.360.039", modelo.FormaCedulaPuntos, "p_cl_88627"},
		{"d8a981c7-acae-5cf9-82f6-051144479154", "mcastano@gmail.com", modelo.FormaCorreo, "p_cl_88627"},
		{"9e0e0572-3746-5a1a-ba83-560a64bdddbe", "manuela_castano", modelo.FormaSlug, "p_cl_88627"},
		{"clientes:CL-19127", "Manuela Castaño", modelo.FormaNombreCorto, "p_cl_88627"},
		{"sdoc_01", "Kelly Castaño", modelo.FormaNombreCompleto, ""},
		{"5eb0e9a6-fb79-5de9-8170-6d055433f6e6", "Nohora Salazar", modelo.FormaNombreCorto, "p_cl_19127"},
	}
	for _, c := range casos {
		m, ok := halla(menciones, c.registro, c.valor)
		if !ok {
			t.Errorf("%s: no detecté %q", c.registro, c.valor)
			continue
		}
		if c.forma != "" && m.Forma != c.forma {
			t.Errorf("%s: forma de %q = %q, quiero %q", c.registro, c.valor, m.Forma, c.forma)
		}
		if c.persona != "" && m.Persona != c.persona {
			t.Errorf("%s: persona de %q = %q, quiero %q", c.registro, c.valor, m.Persona, c.persona)
		}
	}
}

// El slug del adjunto se detecta como forma conocida, no como texto cualquiera:
// es la PII oculta de CE-08.
func TestMuestraDetectaElSlugDelAdjunto(t *testing.T) {
	recs, indice := corpusMuestra(t)
	d, _ := Nuevo(Opciones{Indice: indice, Allowlist: allowDeMuestra(t)})
	menciones, _ := d.Detecta(context.Background(), recs)
	encontrado := false
	for _, m := range menciones {
		if m.Registro == "9e0e0572-3746-5a1a-ba83-560a64bdddbe" && m.Campo == "attachments.0.filename" {
			if m.Forma != modelo.FormaSlug {
				t.Errorf("forma del adjunto = %q, quiero slug", m.Forma)
			}
			encontrado = true
		}
	}
	if !encontrado {
		t.Fatal("no detecté nada en el nombre del adjunto")
	}
}

// La allowlist protege bancos y reguladores aunque compartan contexto con una
// persona (CE-11, sample_11 y sample_12).
func TestAllowlistConservaEntidades(t *testing.T) {
	recs, indice := corpusMuestra(t)
	d, _ := Nuevo(Opciones{Indice: indice, Allowlist: allowDeMuestra(t)})
	menciones, _ := d.Detecta(context.Background(), recs)
	for _, m := range menciones {
		if m.Valor == "Bancolombia" || m.Valor == "Superfinanciera" {
			t.Errorf("detecté %q como PII", m.Valor)
		}
	}
}

// Los patrones con contexto encuentran datos que no están en el maestro, y no
// tocan montos ni fechas de negocio (CE-16, CE-21, CE-23).
func TestPatronesConContexto(t *testing.T) {
	d, err := Nuevo(Opciones{Indice: vacio(t)})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	casos := []struct {
		texto string
		valor string
		forma modelo.TipoForma
	}{
		{"¿Cuál es tu cédula? 1022649507", "1022649507", modelo.FormaCedula},
		{"¿Me confirmas tu cédula? 10.226.495", "10.226.495", modelo.FormaCedulaPuntos},
		{"Mi correo es ana punto perez arroba gmail punto com", "ana punto perez arroba gmail punto com", modelo.FormaCorreoDeletreado},
		{"Me llamo de nuevo, mi celular es 314 872 9552", "314 872 9552", modelo.FormaCelular},
		{"DOB 01/08/1993", "01/08/1993", modelo.FormaFechaUS},
		{"nací el 8 de enero de 1993", "8 de enero de 1993", modelo.FormaFechaPalabras},
	}
	for _, c := range casos {
		recs := []modelo.Registro{modelo.ChatRecord{ID: "m1", Author: "Ana", Text: c.texto}}
		menciones, _ := d.Detecta(context.Background(), recs)
		if _, ok := halla(menciones, "m1", c.valor); !ok {
			t.Errorf("%q no detectado en %q (menciones: %+v)", c.valor, c.texto, menciones)
		}
	}
	// Sin contexto, un monto con puntos no es una cédula.
	recs := []modelo.Registro{modelo.ChatRecord{ID: "m2", Author: "Ana", Text: "el pago fue de 1.480.000 pesos"}}
	menciones, _ := d.Detecta(context.Background(), recs)
	if len(menciones) != 0 {
		t.Fatalf("detecté un monto como PII: %+v", menciones)
	}
}

// Una respuesta de solo dígitos se clasifica con la pregunta anterior del hilo
// (CE-16).
func TestContextoDelHiloClasificaRespuestas(t *testing.T) {
	d, err := Nuevo(Opciones{Indice: vacio(t)})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	recs := []modelo.Registro{
		modelo.ChatRecord{ID: "m1", ThreadID: "hilo", Author: "Ana", Text: "¿Me confirmas tu cédula para validar?"},
		modelo.ChatRecord{ID: "m2", ThreadID: "hilo", Author: "Bea", Text: "1022649507"},
	}
	menciones, _ := d.Detecta(context.Background(), recs)
	if _, ok := halla(menciones, "m2", "1022649507"); !ok {
		t.Fatalf("no clasifiqué la respuesta de solo dígitos: %+v", menciones)
	}
}

// Los generadores proponen lo que las formas no ven: disparadores, secuencias
// con mayúscula y handles (D30).
func TestCandidatosPorGenerador(t *testing.T) {
	d, err := Nuevo(Opciones{Indice: vacio(t)})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	recs := []modelo.Registro{
		modelo.ChatRecord{ID: "m1", Author: "Ana", Text: "la señora Gloria me contó"},
		modelo.ChatRecord{ID: "m2", Author: "Ana", Text: "Prepared by Doris Lozano, Contadora"},
		modelo.ChatRecord{ID: "m3", Author: "Ana", Text: "escríbele a @maria.gomez"},
	}
	cands := d.Candidatos(recs)
	quiero := map[string][]string{
		"Gloria":       {"disparador"},
		"Doris Lozano": {"disparador", "mayusculas"},
		"maria.gomez":  {"handle"},
	}
	for valor, generadores := range quiero {
		encontrado := false
		for _, c := range cands {
			if c.Valor != valor {
				continue
			}
			encontrado = true
			ok := false
			for _, g := range generadores {
				if c.Generador == g {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%q generador = %q, quiero %v", valor, c.Generador, generadores)
			}
		}
		if !encontrado {
			t.Errorf("no propuse %q como candidato", valor)
		}
	}
}

// vacio es un registro sin personas: los patrones y generadores funcionan
// aunque no haya maestro.
func vacio(t *testing.T) *registro.Indice {
	t.Helper()
	indice, err := registro.Construye(nil, registro.Opciones{})
	if err != nil {
		t.Fatalf("registro vacío: %v", err)
	}
	return indice
}

// Un nombre de pila compartido por dos personas no se toca suelto (D25).
func TestNombreDePilaCompartidoNoSeToca(t *testing.T) {
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
	d, err := Nuevo(Opciones{Indice: indice})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	recs := []modelo.Registro{modelo.ChatRecord{ID: "m1", Author: "Ana", Text: "Hola Camila, ¿cómo estás?"}}
	menciones, _ := d.Detecta(context.Background(), recs)
	if len(menciones) != 0 {
		t.Fatalf("toqué un nombre de pila compartido: %+v", menciones)
	}
}
