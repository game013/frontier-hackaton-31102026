package juez

import (
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

func TestJuezGLO(t *testing.T) {
	s := salidaDePrueba(t)
	m := JuezGLO(s)
	if m.Pasa() {
		t.Fatalf("una forma real que sigue en la salida debe romper el barrido: %+v", m)
	}
	if m.Total != 3 {
		t.Fatalf("se esperaban 3 formas barridas, hay %d", m.Total)
	}
	// La entidad conservada no se barre: su forma real es la que debe quedar.
	for _, d := range m.Detalles {
		if d == "" || strings.Contains(d, "Banco Falso") {
			t.Fatalf("una entidad kept no puede aparecer en el barrido: %q", d)
		}
	}
}

func TestJuezGLOEnVerde(t *testing.T) {
	dir := escribeSalida(t,
		[]string{`{"id":"c1","thread_id":"t","author":"Dora Ejemplo","ts":"1","text":"Hola","attachments":[]}`},
		nil, nil,
		`{"entities":[
			{"entity_id":"p_1","type":"person","originals":["Ana Prueba","ana.prueba@ejemplo.test"],"pseudonyms":{"name":"Dora Ejemplo"},"records":["c1"]},
			{"entity_id":"kept_1","type":"organization_or_place","originals":["Banco Falso"],"pseudonyms":{},"records":[],"kept":true}
		]}`,
	)
	s, err := CargaSalida(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m := JuezGLO(s); !m.Pasa() {
		t.Fatalf("sin formas reales en la salida el barrido debe estar en verde: %+v", m)
	}
}

func TestJuezGLOBarreConVariantes(t *testing.T) {
	// MAYÚSCULAS sin tildes y separadores de dígitos cuentan como la misma forma.
	dir := escribeSalida(t,
		[]string{
			`{"id":"c1","thread_id":"t","author":"X","ts":"1","text":"SOY ANA PRUEBA","attachments":[]}`,
			`{"id":"c2","thread_id":"t","author":"X","ts":"2","text":"mi cédula es 1.000.000.001","attachments":[]}`,
		},
		nil, nil,
		`{"entities":[{"entity_id":"p_1","type":"person","originals":["Ana Prueba","1000000001"],"pseudonyms":{"name":"Dora Ejemplo"},"records":["c1","c2"]}]}`,
	)
	s, err := CargaSalida(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := JuezGLO(s)
	if m.Pasan != 0 || m.Total != 2 {
		t.Fatalf("las dos variantes deben contarse como fuga: %+v", m)
	}
}

func TestJuezDIC(t *testing.T) {
	s := salidaDePrueba(t)
	completo := Test{ID: "t1", Persona: "p_1", Forms: map[string]string{
		"full": "Carla Falsa", "email": "carla.falsa@ejemplo.test", "slug": "carla_falsa",
	}}
	m := JuezDIC(s, []Test{completo})
	if !m.Pasa() || m.Total != 3 {
		t.Fatalf("todas las formas declaradas deben pasar: %+v", m)
	}

	incompleto := Test{ID: "t2", Persona: "p_1", Forms: map[string]string{
		"full": "Carla Falsa", "phone": "3000000000",
	}}
	m = JuezDIC(s, []Test{incompleto})
	if m.Pasa() {
		t.Fatalf("una forma sin declarar debe romper el diccionario: %+v", m)
	}
	if len(m.Detalles) != 1 || !strings.Contains(m.Detalles[0], "3000000000") {
		t.Fatalf("el detalle debe nombrar la forma que falta: %v", m.Detalles)
	}

	// Un test de otra salida (persona que no está) no se mide.
	ajeno := Test{ID: "t3", Persona: "p_999", LookupForms: []string{"nadie@ejemplo.test"}}
	if m := JuezDIC(s, []Test{ajeno}); m.Total != 0 {
		t.Fatalf("un test que no aplica no debe contar: %+v", m)
	}
}

func TestJuezREC(t *testing.T) {
	s := salidaDePrueba(t)
	verdad := []modelo.Entity{
		{EntityID: "p_1", Type: modelo.EntidadPersona, Originals: []string{"Carla Falsa"}},
		{EntityID: "p_2", Type: modelo.EntidadPersona, Originals: []string{"Nadie Detectado"}, Records: []string{"c9"}},
		{EntityID: "kept_1", Type: modelo.EntidadOrganizacionOLugar, Originals: []string{"Banco Falso"}, Kept: true},
	}
	m := JuezREC(s, verdad)
	if m.Total != 2 {
		t.Fatalf("solo se miden las personas de la verdad, hay %d", m.Total)
	}
	if m.Pasan != 1 || m.Pasa() {
		t.Fatalf("una persona sin detectar debe bajar la cobertura: %+v", m)
	}
	if len(m.Detalles) != 1 || !strings.Contains(m.Detalles[0], "p_2") {
		t.Fatalf("el detalle debe nombrar la persona que falta: %v", m.Detalles)
	}

	// Reconocer por registros compartidos también cuenta como cobertura.
	verdad = []modelo.Entity{{EntityID: "p_1", Type: modelo.EntidadPersona, Records: []string{"c2"}}}
	if m := JuezREC(s, verdad); !m.Pasa() {
		t.Fatalf("compartir registros es cobertura: %+v", m)
	}
}

func TestJuezRECSinVerdad(t *testing.T) {
	m := JuezREC(salidaDePrueba(t), nil)
	if m.Total != 0 || !m.Pasa() || m.Ratio() != 1 {
		t.Fatalf("sin verdad no hay nada que medir: %+v", m)
	}
}
