package juez

import (
	"strings"
	"testing"
)

// testDeFamilia arma un Test con la forma de tests_sample.json.
func testDeFamilia(familia, registro, campo string, payload []string) Test {
	t := Test{ID: "t_" + familia, Familia: familia, Peso: 1}
	t.Locator.Registro = registro
	t.Locator.Campo = campo
	t.Payload = payload
	return t
}

func TestDirecta(t *testing.T) {
	s := salidaDePrueba(t)
	// El dato sigue en el registro que lo tenía y además en otro: falla igual.
	res := Evalúa(s, testDeFamilia("directa", "c2", "text", []string{"Banco Falso"}))
	if res.Estado != Falla {
		t.Fatalf("un dato presente debe fallar, estado %s (%s)", res.Estado, res.Detalle)
	}
	res = Evalúa(s, testDeFamilia("directa", "c1", "text", []string{"1.000.000.001"}))
	if res.Estado != Pasa {
		t.Fatalf("un dato ausente debe pasar, estado %s (%s)", res.Estado, res.Detalle)
	}
	// Como hash sin sal también cuenta como fuga.
	for _, huella := range huellasDe("Ana Prueba") {
		if !ContieneExacto(huella, huella) {
			t.Fatal("la huella debería contenerse a sí misma")
		}
	}
	if len(huellasDe("1.000.000.001")) != 6 {
		t.Fatal("se esperan md5, sha1 y sha256 del dato y de sus dígitos")
	}
	// Un registro que no está en la salida no se puede juzgar.
	res = Evalúa(s, testDeFamilia("directa", "no-existe", "text", []string{"x"}))
	if res.Estado != Omite {
		t.Fatalf("registro ausente debe omitirse, estado %s", res.Estado)
	}
}

func TestOculta(t *testing.T) {
	s := salidaDePrueba(t)
	res := Evalúa(s, testDeFamilia("oculta", "c1", "attachments", []string{"carla_falsa"}))
	if res.Estado != Falla {
		t.Fatalf("el slug en el adjunto debe fallar, estado %s (%s)", res.Estado, res.Detalle)
	}
	res = Evalúa(s, testDeFamilia("oculta", "d1", "body", []string{"carla.falsa@ejemplo.test"}))
	if res.Estado != Falla {
		t.Fatalf("el correo en metadata debe fallar, estado %s (%s)", res.Estado, res.Detalle)
	}
	res = Evalúa(s, testDeFamilia("oculta", "c1", "attachments", []string{"nombre-que-no-esta"}))
	if res.Estado != Pasa {
		t.Fatalf("un dato ausente debe pasar, estado %s (%s)", res.Estado, res.Detalle)
	}
}

func TestIndirecta(t *testing.T) {
	s := salidaDePrueba(t)
	cuasi := []string{"fotógrafa", "Centro", "2025"}
	res := Evalúa(s, testDeFamilia("indirecta", "clientes:CX-1", "columns.ocupacion", cuasi))
	if res.Estado != Falla {
		t.Fatalf("los componentes juntos en la fila deben fallar, estado %s (%s)", res.Estado, res.Detalle)
	}
	// Solo dos de los tres componentes: no identifica, pasa.
	res = Evalúa(s, testDeFamilia("indirecta", "c2", "text", []string{"fotógrafa", "Centro", "2025"}))
	if res.Estado != Pasa {
		t.Fatalf("componentes separados deben pasar, estado %s (%s)", res.Estado, res.Detalle)
	}
	// El cuasi-identificador se mide sobre la unión con la fila del maestro.
	test := testDeFamilia("indirecta", "c1", "text", []string{"Centro", "fotógrafa"})
	test.JoinRecords = []string{"clientes:CX-1"}
	res = Evalúa(s, test)
	if res.Estado != Falla {
		t.Fatalf("la unión con el maestro debe fallar, estado %s (%s)", res.Estado, res.Detalle)
	}
}

func TestFalsoPositivo(t *testing.T) {
	s := salidaDePrueba(t)
	res := Evalúa(s, testDeFamilia("falso_positivo", "c2", "text", []string{"Banco Falso"}))
	if res.Estado != Pasa {
		t.Fatalf("lo conservado sigue ahí: debería pasar, estado %s (%s)", res.Estado, res.Detalle)
	}
	res = Evalúa(s, testDeFamilia("falso_positivo", "c2", "text", []string{"Superintendencia"}))
	if res.Estado != Falla {
		t.Fatalf("una entidad perdida debe fallar, estado %s (%s)", res.Estado, res.Detalle)
	}
	res = Evalúa(s, testDeFamilia("falso_positivo", "c2", "no-existe", []string{"Banco Falso"}))
	if res.Estado != Falla {
		t.Fatalf("un campo ausente debe fallar, estado %s (%s)", res.Estado, res.Detalle)
	}
}

func TestHomonimos(t *testing.T) {
	// Mismo nombre, dos entidades, dos seudónimos distintos: pasa.
	dir := escribeSalida(t,
		[]string{`{"id":"c1","thread_id":"t","author":"Ana Prueba","ts":"1","text":"Hola","attachments":[]}`},
		nil, nil,
		`{"entities":[
			{"entity_id":"p_1","type":"person","originals":["Ana Prueba"],"pseudonyms":{"name":"Dora Uno"},"records":["c1"]},
			{"entity_id":"p_2","type":"person","originals":["Ana Prueba"],"pseudonyms":{"name":"Elsa Dos"},"records":["c1"]}
		]}`,
	)
	s, err := CargaSalida(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res := Evalúa(s, testDeFamilia("homonimos", "", "", []string{"Ana Prueba"})); res.Estado != Pasa {
		t.Fatalf("dos homónimos con seudónimos distintos deben pasar: %s (%s)", res.Estado, res.Detalle)
	}

	// El mismo seudónimo para los dos: falla.
	dir = escribeSalida(t, nil, nil, nil,
		`{"entities":[
			{"entity_id":"p_1","type":"person","originals":["Ana Prueba"],"pseudonyms":{"name":"Dora Uno"},"records":[]},
			{"entity_id":"p_2","type":"person","originals":["Ana Prueba"],"pseudonyms":{"name":"dora uno"},"records":[]}
		]}`,
	)
	s, err = CargaSalida(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res := Evalúa(s, testDeFamilia("homonimos", "", "", []string{"Ana Prueba"})); res.Estado != Falla {
		t.Fatalf("el mismo seudónimo debe fallar: %s (%s)", res.Estado, res.Detalle)
	}

	// Una sola persona con ese nombre: falta la otra, falla.
	dir = escribeSalida(t, nil, nil, nil,
		`{"entities":[{"entity_id":"p_1","type":"person","originals":["Ana Prueba"],"pseudonyms":{"name":"Dora Uno"},"records":[]}]}`,
	)
	s, err = CargaSalida(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res := Evalúa(s, testDeFamilia("homonimos", "", "", []string{"Ana Prueba"})); res.Estado != Falla {
		t.Fatalf("un homónimo sin separar debe fallar: %s (%s)", res.Estado, res.Detalle)
	}

	// El nombre no existe en la salida: el test no aplica.
	if res := Evalúa(salidaDePrueba(t), testDeFamilia("homonimos", "", "", []string{"Nadie Aquí"})); res.Estado != Omite {
		t.Fatalf("un nombre ausente debe omitirse: %s (%s)", res.Estado, res.Detalle)
	}
}

func testDeConsistencia(formas []string, registros []string) Test {
	test := Test{ID: "t_consistencia", Familia: "consistencia", Peso: 1, LookupForms: formas, Records: registros}
	return test
}

func TestConsistencia(t *testing.T) {
	dir := escribeSalida(t,
		[]string{
			`{"id":"c1","thread_id":"t","author":"Dora Ejemplo","ts":"1","text":"Hola, soy Dora Ejemplo","attachments":[]}`,
			`{"id":"c2","thread_id":"t","author":"Dora Ejemplo","ts":"2","text":"Gracias, Dora Ejemplo","attachments":[]}`,
		},
		[]string{
			`{"id":"d1","format":"md","filename":"memo.md","title":"Memo","body":"Preparado por Dora Ejemplo.","metadata":{}}`,
		}, nil,
		`{"entities":[{"entity_id":"p_1","type":"person","originals":["Ana Prueba","ana.prueba@ejemplo.test"],"pseudonyms":{"name":"Dora Ejemplo","email":"dejemplo@ejemplo.test"},"records":["c1","c2","d1"]}]}`,
	)
	s, err := CargaSalida(dir)
	if err != nil {
		t.Fatal(err)
	}
	test := testDeConsistencia([]string{"ana.prueba@ejemplo.test", "Ana Prueba"}, []string{"c1", "c2", "d1"})
	if res := Evalúa(s, test); res.Estado != Pasa {
		t.Fatalf("un solo seudónimo en todas las fuentes debe pasar: %s (%s)", res.Estado, res.Detalle)
	}

	// La persona no está declarada en la salida: el test no aplica.
	testOtro := testDeConsistencia([]string{"nadie@ejemplo.test"}, nil)
	if res := Evalúa(s, testOtro); res.Estado != Omite {
		t.Fatalf("una persona ausente debe omitirse: %s (%s)", res.Estado, res.Detalle)
	}
}

func TestConsistenciaDetectaDosSeudonimos(t *testing.T) {
	// La persona quedó partida: su nombre aparece en un solo registro y otro
	// nombre la domina en el resto.
	dir := escribeSalida(t,
		[]string{
			`{"id":"c1","thread_id":"t","author":"Dora Uno","ts":"1","text":"Hola, soy Dora Uno","attachments":[]}`,
			`{"id":"c2","thread_id":"t","author":"Elsa Dos","ts":"2","text":"Hola, soy Elsa Dos, otra vez","attachments":[]}`,
			`{"id":"c3","thread_id":"t","author":"Elsa Dos","ts":"3","text":"Elsa Dos al habla","attachments":[]}`,
		},
		nil, nil,
		`{"entities":[
			{"entity_id":"p_1","type":"person","originals":["Ana Prueba"],"pseudonyms":{"name":"Dora Uno"},"records":["c1","c2","c3"]},
			{"entity_id":"p_2","type":"person","originals":["Otra Persona"],"pseudonyms":{"name":"Elsa Dos"},"records":["c2","c3"]}
		]}`,
	)
	s, err := CargaSalida(dir)
	if err != nil {
		t.Fatal(err)
	}
	test := testDeConsistencia([]string{"Ana Prueba"}, []string{"c1", "c2", "c3"})
	res := Evalúa(s, test)
	if res.Estado != Falla || !strings.Contains(res.Detalle, "Elsa Dos") {
		t.Fatalf("la persona partida en dos seudónimos debe fallar: %s (%s)", res.Estado, res.Detalle)
	}
}

func TestConsistenciaSeudonimoEsFormaReal(t *testing.T) {
	dir := escribeSalida(t,
		[]string{`{"id":"c1","thread_id":"t","author":"Ana Prueba","ts":"1","text":"Ana Prueba","attachments":[]}`},
		nil, nil,
		`{"entities":[
			{"entity_id":"p_1","type":"person","originals":["Ana Prueba"],"pseudonyms":{"name":"Ana Prueba"},"records":["c1"]},
			{"entity_id":"p_2","type":"person","originals":["Luz Real"],"pseudonyms":{"name":"Luz Real"},"records":[]}
		]}`,
	)
	s, err := CargaSalida(dir)
	if err != nil {
		t.Fatal(err)
	}
	res := Evalúa(s, testDeConsistencia([]string{"Ana Prueba"}, []string{"c1"}))
	if res.Estado != Falla {
		t.Fatalf("un seudónimo igual a una forma real debe fallar: %s (%s)", res.Estado, res.Detalle)
	}
}

func TestFamiliaSinJuez(t *testing.T) {
	res := Evalúa(salidaDePrueba(t), testDeFamilia("parecido", "c1", "text", nil))
	if res.Estado != Omite {
		t.Fatalf("una familia sin juez en C11 debe omitirse, estado %s", res.Estado)
	}
}
