package cuasi

import (
	"sort"
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/catalogo"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
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

// filaDeLaPersona es una fila del maestro inventada: ningún dato sale del
// corpus. Es la fila que la heurística del jurado une con un registro de chat
// (join_records).
func filaDeLaPersona() modelo.TableRecord {
	return modelo.TableRecord{
		ID: "clientes:CL-10001", Table: "clientes",
		Columns: map[string]string{
			"cliente_id": "CL-10001", "nombre_completo": "Elena Perez Rojas",
			"ciudad": "Cali", "barrio": "Granada", "ocupacion": "fotógrafa",
			"fecha_vinculacion": "2024-03-11", "notas": "Sin novedad.",
		},
	}
}

// En la fila del maestro la ocupación va a su sector y el barrio a otro barrio
// de la misma ciudad; el resto de las columnas queda igual (D9, DECISIONS §1.3).
func TestGeneralizaFilaDelMaestro(t *testing.T) {
	fila := filaDeLaPersona()
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
		"fecha_vinculacion": "2024-03-11",
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
	granada := filaDeLaPersona()
	otra := filaDeLaPersona()
	otra.ID = "clientes:CL-10002"
	otra.Columns["cliente_id"] = "CL-10002"
	otra.Columns["barrio"] = "El Peñón"

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

func chatDePrueba(texto string) modelo.ChatRecord {
	return modelo.ChatRecord{
		ID: "m-0001", ThreadID: "hilo-1", Author: "Elena Perez", TS: "1.0", Text: texto,
	}
}

func textoDe(t *testing.T, rec modelo.Registro) string {
	t.Helper()
	return rec.(modelo.ChatRecord).Text
}

// La plantilla «soy X, vivo en Y, desde Z» no deja el cuasi-identificador en el
// texto: la ocupación va a su sector, el barrio a su ciudad y el año a "hace
// unos años" (D9, CE-13).
func TestGeneralizaPlantillaEnElTexto(t *testing.T) {
	rec := chatDePrueba("Buenas, soy fotógrafa, vivo en Granada. y llevo con ustedes desde 2024 sin un solo atraso.")
	quiero := "Buenas, soy profesional de las industrias creativas, vivo en Cali. y llevo con ustedes desde hace unos años sin un solo atraso."
	out := generaliza(t, []modelo.Registro{rec})
	if got := textoDe(t, out[0]); got != quiero {
		t.Fatalf("texto =\n %q\nquiero\n %q", got, quiero)
	}
}

// La descripción de un agente también lleva el cuasi-identificador: «es X,
// lleva con nosotros desde Z» va a sector y a "hace unos años" (CE-12).
func TestGeneralizaDescripcionDeAgente(t *testing.T) {
	rec := chatDePrueba("Listo. Es contadora, lleva con nosotros desde 2024, y le molesta más el tono que el dato.")
	quiero := "Listo. Es profesional de finanzas, lleva con nosotros desde hace unos años, y le molesta más el tono que el dato."
	out := generaliza(t, []modelo.Registro{rec})
	if got := textoDe(t, out[0]); got != quiero {
		t.Fatalf("texto =\n %q\nquiero\n %q", got, quiero)
	}
}

// sinLosTres falla si los componentes del cuasi-identificador co-ocurren en el
// mismo texto, que es exactamente lo que mide J-IND.
func sinLosTres(t *testing.T, donde, texto string, datos []string) {
	t.Helper()
	for _, dato := range datos {
		if !norm.Contiene(texto, dato) {
			return
		}
	}
	t.Fatalf("%s conserva los %d componentes del cuasi-identificador: %q", donde, len(datos), texto)
}

// CE-13: la plantilla «soy X, vivo en Y, desde Z» queda generalizada en el texto
// y en la fila del maestro, y ningún texto deja los tres componentes juntos.
func TestCE13PlantillaSinCoocurrencia(t *testing.T) {
	rec := chatDePrueba("Buenas, soy fotógrafa, vivo en Granada. y llevo con ustedes desde 2024 sin un solo atraso.")
	rec.Author = "Elena Perez"
	fila := filaDeLaPersona()
	out := generaliza(t, []modelo.Registro{rec, fila})

	quiero := "Buenas, soy profesional de las industrias creativas, vivo en Cali. y llevo con ustedes desde hace unos años sin un solo atraso."
	if got := textoDe(t, out[0]); got != quiero {
		t.Fatalf("texto =\n %q\nquiero\n %q", got, quiero)
	}
	maestro := out[1].(modelo.TableRecord)
	quieroColumna := map[string]string{
		"ciudad":            "Cali",
		"ocupacion":         "profesional de las industrias creativas",
		"fecha_vinculacion": "2024-03-11",
		"nombre_completo":   "Elena Perez Rojas",
		"notas":             "Sin novedad.",
	}
	for columna, valor := range quieroColumna {
		if maestro.Columns[columna] != valor {
			t.Errorf("columns[%s] = %q, quiero %q", columna, maestro.Columns[columna], valor)
		}
	}
	payload := []string{"fotógrafa", "Granada", "2024"}
	sinLosTres(t, "el chat m-0001", textoDe(t, out[0]), payload)
	sinLosTres(t, "la fila clientes:CL-10001", columnasDe(maestro), payload)
}

// CE-12: la descripción del agente también deja el cuasi-identificador partido,
// en el chat y en la fila del maestro.
func TestCE12DescripcionSinCoocurrencia(t *testing.T) {
	rec := chatDePrueba("Hablé con la clienta. Es contadora, lleva con nosotros desde 2021, y quiere el detalle por escrito.")
	fila := filaDeLaPersona()
	fila.Columns["ocupacion"] = "contadora"
	fila.Columns["barrio"] = "San Fernando"
	fila.Columns["fecha_vinculacion"] = "2021-06-30"
	out := generaliza(t, []modelo.Registro{rec, fila})

	quiero := "Hablé con la clienta. Es profesional de finanzas, lleva con nosotros desde hace unos años, y quiere el detalle por escrito."
	if got := textoDe(t, out[0]); got != quiero {
		t.Fatalf("texto =\n %q\nquiero\n %q", got, quiero)
	}
	maestro := out[1].(modelo.TableRecord)
	if maestro.Columns["ocupacion"] != "profesional de finanzas" {
		t.Errorf("ocupacion = %q, quiero %q", maestro.Columns["ocupacion"], "profesional de finanzas")
	}
	if maestro.Columns["fecha_vinculacion"] != "2021-06-30" {
		t.Errorf("fecha_vinculacion = %q: la fecha de negocio se conserva", maestro.Columns["fecha_vinculacion"])
	}
	payload := []string{"contadora", "San Fernando", "2021"}
	sinLosTres(t, "el chat m-0001", textoDe(t, out[0]), payload)
	sinLosTres(t, "la fila clientes:CL-10001", columnasDe(maestro), payload)
}

// columnasDe junta las columnas de una fila como las lee el jurado.
func columnasDe(r modelo.TableRecord) string {
	claves := make([]string, 0, len(r.Columns))
	for clave := range r.Columns {
		claves = append(claves, clave)
	}
	sort.Strings(claves)
	valores := make([]string, 0, len(claves))
	for _, clave := range claves {
		valores = append(valores, r.Columns[clave])
	}
	return strings.Join(valores, " ")
}

// El año solo se generaliza cuando hay una relación de la persona delante: un
// «desde» suelto es la fecha de negocio de la empresa y se conserva (D1,
// DECISIONS §1.4). Y la puntuación no ASCII no pega palabras: «2024…» sigue
// siendo un año.
func TestAnioDeLaRelacionNecesitaAncla(t *testing.T) {
	casos := []struct {
		nombre string
		texto  string
		quiero string
	}{
		{
			"plantilla",
			"Buenas, soy fotógrafa, vivo en Granada. y llevo con ustedes desde 2024 sin un solo atraso.",
			"Buenas, soy profesional de las industrias creativas, vivo en Cali. y llevo con ustedes desde hace unos años sin un solo atraso.",
		},
		{
			"llevo con ustees",
			"y llevo con ustees desde 2023 sin un solo atraso, por si eso cuenta!!",
			"y llevo con ustees desde hace unos años sin un solo atraso, por si eso cuenta!!",
		},
		{
			"estoy con",
			"la clienta de Cali está con nosotros desde 2021 y pide el detalle",
			"la clienta de Cali está con nosotros desde hace unos años y pide el detalle",
		},
		{
			"cliente desde",
			"Te la exonero por 12 meses por ser cliente desde 2021",
			"Te la exonero por 12 meses por ser cliente desde hace unos años",
		},
		{
			"cliente nuestro desde",
			"Titular: chef de Medellín, cliente nuestro desde 2021. Niega conocer al afectado.",
			"Titular: chef de Medellín, cliente nuestro desde hace unos años. Niega conocer al afectado.",
		},
		{
			"vinculado desde",
			"Cliente de Bucaramanga, arquitecto, vinculado desde 2021",
			"Cliente de Bucaramanga, arquitecto, vinculado desde hace unos años",
		},
		{
			"con nosotros sin verbo en la ventana",
			"una agente de soporte en Barranquilla, con nosotros desde 2024; para efectos de la ARL",
			"una agente de soporte en Barranquilla, con nosotros desde hace unos años; para efectos de la ARL",
		},
		{
			"puntos suspensivos pegados al año",
			"y llevo con ustedes desde 2024… sin un solo atraso",
			"y llevo con ustedes desde hace unos años… sin un solo atraso",
		},
		{
			"puntos suspensivos pegados a la preposición",
			"llevo con ustedes…desde 2020 sin un solo atraso",
			"llevo con ustedes…desde hace unos años sin un solo atraso",
		},
		{
			"fecha de negocio de la empresa",
			"Kipu Pay opera desde 2020.",
			"Kipu Pay opera desde 2020.",
		},
		{
			"apertura de la empresa",
			"la empresa abrió desde 2019.",
			"la empresa abrió desde 2019.",
		},
		{
			"desde suelto",
			"desde 2020.",
			"desde 2020.",
		},
		{
			"vinculada en el acta",
			"Antecedentes de la afectada: ingeniera de sistemas, vinculada en 2023, sin reclamos previos.",
			"Antecedentes de la afectada: profesional de tecnología, vinculada en hace unos años, sin reclamos previos.",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			rec := chatDePrueba(c.texto)
			out := generaliza(t, []modelo.Registro{rec})
			if got := textoDe(t, out[0]); got != c.quiero {
				t.Fatalf("texto =\n %q\nquiero\n %q", got, c.quiero)
			}
		})
	}
}

// Lo que no es cuasi-identificador de nadie se queda: la ocupación en aposición
// tras una coma, las fechas de negocio y los nombres comunes en minúscula
// (DECISIONS §1.4, J-PAR).
func TestConservaTextoQueNoEsCuasiIdentificador(t *testing.T) {
	casos := []struct {
		nombre string
		texto  string
	}{
		{"ocupación en aposición tras coma", "Preparado por Elena Perez, Contadora. Cierre de julio de 2026."},
		{"nombre común en minúscula", "Porfa actualicen sus datos en el portal de talento antes del 30."},
		{"lugar suelto", "El informe queda en el drive y la oficina está en Chapinero."},
		{"año de un id interno", "Abro INC-2026-044 y entro por PQR-2025-0548."},
		{"ocupación fuera del catálogo", "Soy cartógrafa y llevo con ustedes desde siempre."},
		{"año de negocio", "Se entrega el 15 de agosto de 2026 y el monto es 1.480.000."},
		{"año de nacimiento", "Les paso el otro caso: la clienta de Cali, enfermera, nacida en 1974, que ya reclamó dos veces."},
		{"año de un contrato", "El contrato quedó en firme en 2023 y se renovó en 2025."},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			rec := chatDePrueba(c.texto)
			out := generaliza(t, []modelo.Registro{rec})
			if got := textoDe(t, out[0]); got != c.texto {
				t.Fatalf("texto =\n %q\nquiero\n %q", got, c.texto)
			}
		})
	}
}

// CE-41: el barrio solo se generaliza cuando es el de la persona. El lugar de
// una cosa —la sede, la oficina, el consultorio— se conserva (D1, DECISIONS
// §1.4), aunque el texto tenga la ocupación de alguien.
func TestCE41BarrioSolamenteSiEsDeLaPersona(t *testing.T) {
	casos := []struct {
		nombre string
		texto  string
		quiero string
	}{
		{
			"sede de la empresa tras un locativo",
			"Soy contadora y la sede de Kipu Pay queda en Envigado.",
			"Soy profesional de finanzas y la sede de Kipu Pay queda en Envigado.",
		},
		{
			"oficina como sujeto de cosa",
			"no!! yo estaba trabajando, soy diseñadora, estaba en la oficina en Buenos Aires todo el dia",
			"no!! yo estaba trabajando, soy profesional de las industrias creativas, estaba en la oficina en Buenos Aires todo el dia",
		},
		{
			"consultorio que queda en el barrio",
			"soy odontóloga, el consultorio queda en Castilla y casi todo el día atiendo ahí",
			"soy profesional de la salud, el consultorio queda en Castilla y casi todo el día atiendo ahí",
		},
		{
			"sample_08: trabajo cerca del barrio",
			"De una. Yo soy fotógrafa, trabajo cerca, en Granada, y entré con ustedes en 2025, así que paso cualquier tarde.",
			"De una. Yo soy profesional de las industrias creativas, trabajo cerca, en Cali, y entré con ustedes en hace unos años, así que paso cualquier tarde.",
		},
		{
			"vive en el barrio",
			"soy veterinaria, vivo en Sotomayor.",
			"soy profesional de la salud, vivo en Bucaramanga.",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			rec := chatDePrueba(c.texto)
			out := generaliza(t, []modelo.Registro{rec})
			if got := textoDe(t, out[0]); got != c.quiero {
				t.Fatalf("texto =\n %q\nquiero\n %q", got, c.quiero)
			}
		})
	}
}

// CE-42: si entre el verbo de entrada y el año hay otro «en» o un «de» que une
// otra frase, el año es de negocio y se conserva (D1, DECISIONS §1.4).
func TestCE42AnioDeNegocioTrasOtraFrase(t *testing.T) {
	casos := []struct {
		nombre string
		texto  string
		quiero string
	}{
		{
			"entró en vigencia",
			"La resolución entró en vigencia en 2024 y aplica desde este mes.",
			"La resolución entró en vigencia en 2024 y aplica desde este mes.",
		},
		{
			"de que une otra frase",
			"El radicado entró de nuevo en 2024 con la respuesta del área.",
			"El radicado entró de nuevo en 2024 con la respuesta del área.",
		},
		{
			"entrada de la persona, sin nada en medio",
			"De una. Yo soy fotógrafa, trabajo cerca, en Granada, y entré con ustedes en 2025, así que paso cualquier tarde.",
			"De una. Yo soy profesional de las industrias creativas, trabajo cerca, en Cali, y entré con ustedes en hace unos años, así que paso cualquier tarde.",
		},
		{
			"vinculada en el acta",
			"Antecedentes de la afectada: ingeniera de sistemas, vinculada en 2023, sin reclamos previos.",
			"Antecedentes de la afectada: profesional de tecnología, vinculada en hace unos años, sin reclamos previos.",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			rec := chatDePrueba(c.texto)
			out := generaliza(t, []modelo.Registro{rec})
			if got := textoDe(t, out[0]); got != c.quiero {
				t.Fatalf("texto =\n %q\nquiero\n %q", got, c.quiero)
			}
		})
	}
}

// Una fila con datos que el catálogo no conoce se queda como está: aquí no se
// inventan sectores ni barrios (D18).
func TestConservaFilaFueraDelCatalogo(t *testing.T) {
	fila := modelo.TableRecord{
		ID: "clientes:CL-10002", Table: "clientes",
		Columns: map[string]string{
			"cliente_id": "CL-10002", "nombre_completo": "Elsa Moreno Diaz",
			"ciudad": "Mompox", "barrio": "La Ermita", "ocupacion": "cartógrafa",
		},
	}
	out := generaliza(t, []modelo.Registro{fila})
	got := out[0].(modelo.TableRecord)
	for columna, valor := range fila.Columns {
		if got.Columns[columna] != valor {
			t.Errorf("columns[%s] = %q, quiero %q", columna, got.Columns[columna], valor)
		}
	}
}
