package juez

import (
	"fmt"
	"io"
	"strings"
)

// Pesos del jurado (docs/ENUNCIADO.md, "Cómo se puntúa"): 45 de fuga, 35 de
// utilidad y 20 de consistencia, repartidos por dimensión. Los tests puntuales
// de fuga (35) se reparten con los pesos de juez de CASOS_ESQUINA.md: directa 3,
// oculta 2, indirecta 1.
const (
	PuntosFuga         = 45.0
	PuntosUtilidad     = 35.0
	PuntosConsistencia = 20.0
)

// Dimension es una familia de medición con los puntos que vale.
type Dimension struct {
	Clave     string
	Categoria string
	Puntos    float64
}

// Dimensiones es la tabla de pesos del reporte. "cobertura" (J-REC) no lleva
// puntos propios: D34 la trata como compuerta (meta 100 %) porque mide personas
// que nadie detectó.
var Dimensiones = []Dimension{
	{Clave: "directa", Categoria: "fuga", Puntos: 35 * 3.0 / 6.0},
	{Clave: "oculta", Categoria: "fuga", Puntos: 35 * 2.0 / 6.0},
	{Clave: "indirecta", Categoria: "fuga", Puntos: 35 * 1.0 / 6.0},
	{Clave: "barrido_global", Categoria: "fuga", Puntos: 10},
	{Clave: "parecido", Categoria: "utilidad", Puntos: 10},
	{Clave: "falso_positivo", Categoria: "utilidad", Puntos: 9},
	{Clave: "categoria", Categoria: "utilidad", Puntos: 8},
	{Clave: "estructura", Categoria: "utilidad", Puntos: 8},
	{Clave: "consistencia", Categoria: "consistencia", Puntos: 8},
	{Clave: "homonimos", Categoria: "consistencia", Puntos: 5},
	{Clave: "reporta_a", Categoria: "consistencia", Puntos: 4},
	{Clave: "diccionario", Categoria: "consistencia", Puntos: 3},
	{Clave: "cobertura", Categoria: "compuerta", Puntos: 0},
}

func dimensionDe(clave string) (Dimension, bool) {
	for _, d := range Dimensiones {
		if d.Clave == clave {
			return d, true
		}
	}
	return Dimension{}, false
}

// Reporte reúne todo lo que midió una corrida del juez.
type Reporte struct {
	Salida     string
	Publico    string
	Validacion ResultadoValidacion
	Tests      []ResultadoTest
	Jueces     []Medicion
}

// MedicionDeFamilia agrega los tests de una familia ponderando por el peso de
// cada test. Los omitidos no cuentan; una familia sin tests aplicables no se
// mide.
type MedicionDeFamilia struct {
	Familia   string
	Pasan     int
	Fallan    int
	Omitidos  int
	PesoPasan float64
	PesoTotal float64
}

// MideFamilia agrega los tests de una familia.
func MideFamilia(res []ResultadoTest, familia string) MedicionDeFamilia {
	m := MedicionDeFamilia{Familia: familia}
	for _, r := range res {
		if r.Test.Familia != familia {
			continue
		}
		switch r.Estado {
		case Pasa:
			m.Pasan++
			m.PesoPasan += pesoDe(r.Test)
			m.PesoTotal += pesoDe(r.Test)
		case Falla:
			m.Fallan++
			m.PesoTotal += pesoDe(r.Test)
		default:
			m.Omitidos++
		}
	}
	return m
}

// Aplicables dice si la familia tenía algo que medir.
func (m MedicionDeFamilia) Aplicables() bool {
	return m.PesoTotal > 0
}

// Ratio es la fracción ponderada que pasa.
func (m MedicionDeFamilia) Ratio() float64 {
	if m.PesoTotal == 0 {
		return 1
	}
	return m.PesoPasan / m.PesoTotal
}

func pesoDe(t Test) float64 {
	if t.Peso <= 0 {
		return 1
	}
	return t.Peso
}

// FamiliasEnOrden es el orden en que el reporte lista las familias de tests.
var FamiliasEnOrden = []string{"directa", "oculta", "indirecta", "falso_positivo", "homonimos", "consistencia", "parecido", "categoria", "estructura", "reporta_a"}

// Puntaje es lo que suma el reporte: puntos medidos y puntos posibles.
type Puntaje struct {
	Medidos  float64
	Posibles float64
}

// Letra da el puntaje sobre 100 de lo que se pudo medir.
func (p Puntaje) Letra() float64 {
	if p.Posibles == 0 {
		return 0
	}
	return 100 * p.Medidos / p.Posibles
}

// CalculaPuntaje suma dimensión por dimensión, con los pesos del jurado.
func (r Reporte) CalculaPuntaje() Puntaje {
	total := Puntaje{}
	for _, d := range Dimensiones {
		ratio, medida := r.ratioDe(d.Clave)
		if !medida || d.Puntos == 0 {
			continue
		}
		total.Medidos += d.Puntos * ratio
		total.Posibles += d.Puntos
	}
	return total
}

func (r Reporte) ratioDe(clave string) (float64, bool) {
	for _, j := range r.Jueces {
		if j.Dimension == clave {
			return j.Ratio(), true
		}
	}
	m := MideFamilia(r.Tests, clave)
	if !m.Aplicables() {
		return 0, false
	}
	return m.Ratio(), true
}

// HayFallo dice si algo quedó en rojo: tests, validador, barrido o cobertura.
func (r Reporte) HayFallo() bool {
	if r.Validacion.Ejecutada && (r.Validacion.Error() != "" || !r.Validacion.Pasa) {
		return true
	}
	for _, t := range r.Tests {
		if t.Estado == Falla {
			return true
		}
	}
	for _, j := range r.Jueces {
		if !j.Pasa() {
			return true
		}
	}
	return false
}

// Escribe imprime el reporte completo: validador, tests por familia, jueces y
// el resumen con conteos y pesos.
func (r Reporte) Escribe(w io.Writer) {
	fmt.Fprintf(w, "Juez C11 · salida %s · público %s\n", r.Salida, r.Publico)
	switch {
	case !r.Validacion.Ejecutada:
		fmt.Fprintln(w, "  validador  no corrido")
	case r.Validacion.Error() != "":
		fmt.Fprintf(w, "  validador  NO CORRIÓ: %s\n", r.Validacion.Error())
	case r.Validacion.Pasa:
		fmt.Fprintf(w, "  validador  OK (%s)\n", ResumenValidacion(r.Validacion))
	default:
		fmt.Fprintf(w, "  validador  FALLA (%s)\n", ResumenValidacion(r.Validacion))
	}

	fmt.Fprintln(w, "\nTests")
	aplicables, pasan := 0, 0
	for _, familia := range FamiliasEnOrden {
		m := MideFamilia(r.Tests, familia)
		if m.Pasan+m.Fallan+m.Omitidos == 0 {
			continue
		}
		if m.Aplicables() {
			aplicables += m.Pasan + m.Fallan
			pasan += m.Pasan
			fmt.Fprintf(w, "  %-16s %-12s %s", familia, fmt.Sprintf("%d/%d", m.Pasan, m.Pasan+m.Fallan), barra(m.Ratio()))
		} else {
			fmt.Fprintf(w, "  %-16s %-12s", familia, "sin medir")
		}
		if m.Omitidos > 0 {
			fmt.Fprintf(w, " (%d omitido(s))", m.Omitidos)
		}
		fmt.Fprintln(w)
		for _, t := range r.Tests {
			if t.Test.Familia != familia || t.Estado != Falla {
				continue
			}
			fmt.Fprintf(w, "      FALLA %s: %s\n", t.Test.ID, t.Detalle)
		}
		for _, t := range r.Tests {
			if t.Test.Familia != familia || t.Estado != Omite {
				continue
			}
			fmt.Fprintf(w, "      omite %s: %s\n", t.Test.ID, t.Detalle)
		}
	}
	fmt.Fprintf(w, "  total            %d/%d aplicables\n", pasan, aplicables)

	fmt.Fprintln(w, "\nJueces base")
	for _, j := range r.Jueces {
		fmt.Fprintf(w, "  %s %s\n", j.Linea(), veredicto(j.Pasa()))
		for _, d := range ResumeDetalles(j.Detalles, 5) {
			fmt.Fprintf(w, "      %s\n", d)
		}
	}

	fmt.Fprintln(w, "\nResumen")
	fmt.Fprintf(w, "  %-16s %-13s %6s %10s %10s\n", "dimensión", "categoría", "puntos", "medido", "puntaje")
	porCategoria := map[string]float64{}
	posiblesPorCategoria := map[string]float64{}
	for _, d := range Dimensiones {
		ratio, medida := r.ratioDe(d.Clave)
		medido := "sin tests"
		puntos := "—"
		if medida {
			medido = fmt.Sprintf("%.1f %%", 100*ratio)
			puntos = fmt.Sprintf("%.1f", d.Puntos*ratio)
			porCategoria[d.Categoria] += d.Puntos * ratio
			posiblesPorCategoria[d.Categoria] += d.Puntos
		}
		fmt.Fprintf(w, "  %-16s %-13s %6.1f %10s %10s\n", d.Clave, d.Categoria, d.Puntos, medido, puntos)
	}
	for _, cat := range []struct {
		clave  string
		nombre string
		peso   float64
	}{
		{"fuga", "fuga", PuntosFuga},
		{"utilidad", "utilidad", PuntosUtilidad},
		{"consistencia", "consistencia", PuntosConsistencia},
	} {
		posibles := posiblesPorCategoria[cat.clave]
		nota := ""
		if posibles < cat.peso {
			nota = fmt.Sprintf(" (%.0f de %.0f puntos con juez en C11)", posibles, cat.peso)
		}
		fmt.Fprintf(w, "  %-16s %-13s %6.0f %10s %10.1f%s\n", "", cat.nombre, cat.peso, "", porCategoria[cat.clave], nota)
	}
	total := r.CalculaPuntaje()
	fmt.Fprintf(w, "\n  Puntaje medido: %.1f de %.0f puntos medidos (%.1f %%) · total del jurado 100\n",
		total.Medidos, total.Posibles, total.Letra())
	if cob, ok := r.cobertura(); ok {
		fmt.Fprintf(w, "  Compuerta J-REC (D34): %.1f %% de personas detectadas, meta 100 %%\n", 100*cob)
	}
	if r.HayFallo() {
		fmt.Fprintln(w, "  Veredicto: HAY FALLOS")
	} else {
		fmt.Fprintln(w, "  Veredicto: VERDE")
	}
}

func (r Reporte) cobertura() (float64, bool) {
	for _, j := range r.Jueces {
		if j.Dimension == "cobertura" {
			return j.Ratio(), true
		}
	}
	return 0, false
}

func barra(ratio float64) string {
	llenos := int(ratio*10 + 0.5)
	if llenos > 10 {
		llenos = 10
	}
	return strings.Repeat("█", llenos) + strings.Repeat("·", 10-llenos)
}

func veredicto(pasa bool) string {
	if pasa {
		return "verde"
	}
	return "ROJO"
}
