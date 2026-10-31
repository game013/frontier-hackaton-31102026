package juez

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func reporteVerde() Reporte {
	return Reporte{
		Salida:     "salida",
		Publico:    "publico",
		Validacion: ResultadoValidacion{Pasa: true, Ejecutada: true, Salida: "OK: la salida pasa el validador", Duracion: time.Second},
		Tests: []ResultadoTest{
			{Test: Test{ID: "s1", Familia: "directa", Peso: 3}, Estado: Pasa},
			{Test: Test{ID: "s2", Familia: "oculta", Peso: 2}, Estado: Pasa},
			{Test: Test{ID: "s3", Familia: "indirecta", Peso: 1}, Estado: Pasa},
			{Test: Test{ID: "s4", Familia: "falso_positivo", Peso: 1}, Estado: Pasa},
			{Test: Test{ID: "s5", Familia: "consistencia", Peso: 1}, Estado: Pasa},
		},
		Jueces: []Medicion{
			{ID: "J-GLO", Nombre: "barrido global", Dimension: "barrido_global", Pasan: 5, Total: 5},
			{ID: "J-DIC", Nombre: "diccionario", Dimension: "diccionario", Pasan: 4, Total: 4},
			{ID: "J-REC", Nombre: "cobertura", Dimension: "cobertura", Pasan: 6, Total: 6},
		},
	}
}

func TestMideFamilia(t *testing.T) {
	rep := reporteVerde()
	rep.Tests = append(rep.Tests,
		ResultadoTest{Test: Test{ID: "s6", Familia: "directa", Peso: 3}, Estado: Falla},
		ResultadoTest{Test: Test{ID: "s7", Familia: "directa", Peso: 3}, Estado: Omite},
	)
	m := MideFamilia(rep.Tests, "directa")
	if m.Pasan != 1 || m.Fallan != 1 || m.Omitidos != 1 {
		t.Fatalf("conteos inesperados: %+v", m)
	}
	if m.Ratio() != 0.5 {
		t.Fatalf("el peso de los tests debe ponderar la familia: %+v", m)
	}
	if otra := MideFamilia(rep.Tests, "parecido"); otra.Aplicables() || otra.Ratio() != 1 {
		t.Fatalf("una familia sin tests no se mide: %+v", otra)
	}
}

func TestCalculaPuntaje(t *testing.T) {
	rep := reporteVerde()
	total := rep.CalculaPuntaje()
	// Directa, oculta, indirecta, barrido, falso positivo, consistencia y
	// diccionario: lo único que C11 mide.
	esperado := 35 + 10 + 9 + 8 + 3
	if total.Posibles != float64(esperado) {
		t.Fatalf("puntos medidos posibles = %.1f, se esperaban %d", total.Posibles, esperado)
	}
	if total.Medidos != total.Posibles || total.Letra() != 100 {
		t.Fatalf("todo en verde debe dar 100 %% de lo medido: %+v", total)
	}

	// Barrido global en rojo: se pierden sus 10 puntos completos.
	rep.Jueces[0].Pasan = 0
	if got := rep.CalculaPuntaje().Medidos; got != total.Medidos-10 {
		t.Fatalf("con el barrido en rojo el puntaje debe bajar 10 puntos, bajó %.1f", total.Medidos-got)
	}
}

func TestHayFallo(t *testing.T) {
	if reporteVerde().HayFallo() {
		t.Fatal("un reporte en verde no puede tener fallos")
	}
	casos := map[string]func(*Reporte){
		"test en rojo":        func(r *Reporte) { r.Tests[0].Estado = Falla },
		"validador en rojo":   func(r *Reporte) { r.Validacion.Pasa = false },
		"validador no corrió": func(r *Reporte) { r.Validacion = ResultadoValidacion{Ejecutada: true, err: errDePrueba{}} },
		"juez en rojo":        func(r *Reporte) { r.Jueces[1].Pasan = 3 },
	}
	for nombre, rompe := range casos {
		rep := reporteVerde()
		rompe(&rep)
		if !rep.HayFallo() {
			t.Fatalf("%s: el reporte debe estar en rojo", nombre)
		}
	}
}

type errDePrueba struct{}

func (errDePrueba) Error() string { return "sin validador" }

func TestEscribe(t *testing.T) {
	var buf bytes.Buffer
	reporteVerde().Escribe(&buf)
	salida := buf.String()
	for _, esperado := range []string{"validador  OK", "directa", "J-GLO", "J-REC", "fuga", "utilidad", "consistencia", "VERDE", "Puntaje medido"} {
		if !strings.Contains(salida, esperado) {
			t.Fatalf("el reporte no menciona %q:\n%s", esperado, salida)
		}
	}

	buf.Reset()
	rep := reporteVerde()
	rep.Tests[0].Estado = Falla
	rep.Tests[0].Detalle = "el dato sigue en el registro c1"
	rep.Escribe(&buf)
	if !strings.Contains(buf.String(), "HAY FALLOS") || !strings.Contains(buf.String(), "FALLA s1: el dato sigue en el registro c1") {
		t.Fatalf("un fallo debe verse con su detalle:\n%s", buf.String())
	}
}
