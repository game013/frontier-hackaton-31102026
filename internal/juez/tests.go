package juez

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Test es un test de fuga o utilidad, con la forma de tests_sample.json y
// tests_public.json: el registro y el campo donde mirar, el dato y lo que debe
// pasar con él.
type Test struct {
	ID        string  `json:"id"`
	Familia   string  `json:"family"`
	Mecanismo string  `json:"mechanism"`
	Peso      float64 `json:"weight"`
	Publico   bool    `json:"public"`
	Fuente    string  `json:"source"`
	Locator   struct {
		Registro string `json:"record"`
		Campo    string `json:"field"`
	} `json:"locator"`
	Payload     []string          `json:"payload"`
	Esperado    string            `json:"expected"`
	Persona     string            `json:"person"`
	Persons     []string          `json:"persons"`
	Extra       map[string]any    `json:"extra"`
	Forms       map[string]string `json:"forms"`
	LookupForms []string          `json:"lookup_forms"`
	Records     []string          `json:"records"`
	JoinRecords []string          `json:"join_records"`
}

// CargaTests lee una lista de tests (JSON) de cualquiera de los dos archivos.
func CargaTests(ruta string) ([]Test, error) {
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		return nil, fmt.Errorf("no puedo leer los tests %s: %w", ruta, err)
	}
	var tests []Test
	if err := json.Unmarshal(crudo, &tests); err != nil {
		return nil, fmt.Errorf("%s no es una lista de tests válida: %w", ruta, err)
	}
	return tests, nil
}

// Estado del test frente a la salida.
type Estado string

const (
	Pasa  Estado = "pasa"
	Falla Estado = "falla"
	Omite Estado = "omite"
)

// ResultadoTest es el veredicto de un test con su detalle legible.
type ResultadoTest struct {
	Test    Test
	Estado  Estado
	Detalle string
}

// Evalúa aplica el test a la salida. Las familias que C11 todavía no mide
// (parecido, categoría, estructura, reporta a) quedan omitidas con su motivo.
// Un test cuyo registro no está en la salida también se omite: corriendo la
// muestra no se puede juzgar un test de las fuentes, y al revés.
func Evalúa(s *Salida, t Test) ResultadoTest {
	res := ResultadoTest{Test: t}
	registro, tieneRegistro := s.Registros[t.Locator.Registro]
	switch t.Familia {
	case "directa":
		if !tieneRegistro {
			return omite(res, "el registro %s no está en la salida", t.Locator.Registro)
		}
		return evaluaDirecta(s, t)
	case "oculta":
		if !tieneRegistro {
			return omite(res, "el registro %s no está en la salida", t.Locator.Registro)
		}
		return evaluaOculta(s, registro, t)
	case "indirecta":
		if !tieneRegistro {
			return omite(res, "el registro %s no está en la salida", t.Locator.Registro)
		}
		return evaluaIndirecta(s, registro, t)
	case "falso_positivo":
		if !tieneRegistro {
			return omite(res, "el registro %s no está en la salida", t.Locator.Registro)
		}
		return evaluaFalsoPositivo(registro, t)
	case "homonimos":
		return evaluaHomonimos(s, t)
	case "consistencia":
		return evaluaConsistencia(s, t)
	default:
		return omite(res, "la familia %q no tiene juez en C11 (ver CASOS_ESQUINA.md)", t.Familia)
	}
}

func omite(res ResultadoTest, formato string, args ...any) ResultadoTest {
	res.Estado = Omite
	res.Detalle = fmt.Sprintf(formato, args...)
	return res
}

func falla(res ResultadoTest, formato string, args ...any) ResultadoTest {
	res.Estado = Falla
	res.Detalle = fmt.Sprintf(formato, args...)
	return res
}

func pasa(res ResultadoTest, formato string, args ...any) ResultadoTest {
	res.Estado = Pasa
	res.Detalle = fmt.Sprintf(formato, args...)
	return res
}

// evaluaDirecta exige que el dato no quede en ningún registro de la salida ni
// como hash sin sal (md5, sha1 o sha256 del dato o de sus dígitos).
func evaluaDirecta(s *Salida, t Test) ResultadoTest {
	res := ResultadoTest{Test: t}
	textos := s.Textos()
	ids := s.Orden
	for _, dato := range t.Payload {
		for i, texto := range textos {
			if Contiene(texto, dato) {
				return falla(res, "el dato sigue en el registro %s", ids[i])
			}
		}
		for _, huella := range huellasDe(dato) {
			for i, texto := range textos {
				if ContieneExacto(texto, huella) {
					return falla(res, "el dato sigue como hash sin sal en el registro %s", ids[i])
				}
			}
		}
	}
	return pasa(res, "%d dato(s) ausentes de %d registros", len(t.Payload), len(textos))
}

// huellasDe devuelve los hashes sin sal que el enunciado prohíbe.
func huellasDe(dato string) []string {
	variantes := []string{dato}
	if digitos := Digitos(dato); digitos != "" && digitos != dato {
		variantes = append(variantes, digitos)
	}
	var out []string
	for _, v := range variantes {
		m := md5.Sum([]byte(v))
		s1 := sha1.Sum([]byte(v))
		s2 := sha256.Sum256([]byte(v))
		out = append(out, hex.EncodeToString(m[:]), hex.EncodeToString(s1[:]), hex.EncodeToString(s2[:]))
	}
	return out
}

// evaluaOculta busca el dato en los rincones del registro: el campo indicado,
// metadata, nombre de archivo, adjuntos y markup.
func evaluaOculta(_ *Salida, r Registro, t Test) ResultadoTest {
	res := ResultadoTest{Test: t}
	campos := append(r.Ocultos(), r.Textos(t.Locator.Campo)...)
	for _, dato := range t.Payload {
		for _, campo := range campos {
			if Contiene(campo, dato) {
				return falla(res, "el dato sigue en %s del registro %s", nombreDeCampo(t.Locator.Campo), r.ID)
			}
		}
	}
	return pasa(res, "%d dato(s) ausentes de %s", len(t.Payload), r.ID)
}

func nombreDeCampo(campo string) string {
	if campo == "" {
		return "el registro"
	}
	return campo
}

// evaluaIndirecta exige que los componentes del cuasi-identificador no
// co-ocurran ni en el registro ni en la fila del maestro que lo une.
func evaluaIndirecta(s *Salida, r Registro, t Test) ResultadoTest {
	res := ResultadoTest{Test: t}
	if len(t.Payload) == 0 {
		return omite(res, "el test no trae componentes del cuasi-identificador")
	}
	if todosPresentes(r.Texto(), t.Payload) {
		return falla(res, "los %d componentes co-ocurren en el registro %s", len(t.Payload), r.ID)
	}
	for _, join := range t.JoinRecords {
		fila, ok := s.Registros[join]
		if !ok {
			continue
		}
		if todosPresentes(fila.Texto(), t.Payload) {
			return falla(res, "los %d componentes co-ocurren en la fila %s", len(t.Payload), join)
		}
	}
	return pasa(res, "%d componentes separados en %d fila(s)", len(t.Payload), 1+len(t.JoinRecords))
}

func todosPresentes(texto string, datos []string) bool {
	for _, dato := range datos {
		if !Contiene(texto, dato) {
			return false
		}
	}
	return true
}

// evaluaFalsoPositivo exige que lo que debía conservarse siga intacto.
func evaluaFalsoPositivo(r Registro, t Test) ResultadoTest {
	res := ResultadoTest{Test: t}
	textos := r.Textos(t.Locator.Campo)
	if len(textos) == 0 {
		return falla(res, "el registro %s no tiene el campo %s", r.ID, nombreDeCampo(t.Locator.Campo))
	}
	junto := strings.Join(textos, "\n")
	for _, dato := range t.Payload {
		if !ContieneExacto(junto, dato) {
			return falla(res, "%q se perdió en %s", dato, nombreDeCampo(t.Locator.Campo))
		}
	}
	return pasa(res, "%d entidad(es) conservadas en %s", len(t.Payload), r.ID)
}

// evaluaHomonimos exige dos personas distintas, con seudónimos distintos, para
// el nombre homónimo.
func evaluaHomonimos(s *Salida, t Test) ResultadoTest {
	res := ResultadoTest{Test: t}
	if len(t.Payload) == 0 {
		return omite(res, "el test no trae el nombre homónimo")
	}
	nombre := t.Payload[0]
	var nombres []string
	for _, e := range s.Entities {
		if e.Kept || !declaraForma(e, nombre) {
			continue
		}
		seudonimo := strings.TrimSpace(e.Pseudonyms["name"])
		if seudonimo == "" {
			continue
		}
		nombres = append(nombres, seudonimo)
	}
	if len(nombres) == 0 {
		return omite(res, "ninguna entidad de la salida lleva ese nombre: no hay homónimo que juzgar")
	}
	if len(nombres) < 2 {
		return falla(res, "solo %d persona(s) con seudónimo para el nombre homónimo", len(nombres))
	}
	unicos := map[string]bool{}
	for _, n := range nombres {
		unicos[Normaliza(n)] = true
	}
	if len(unicos) < 2 {
		return falla(res, "%d personas con el mismo nombre recibieron el mismo seudónimo", len(nombres))
	}
	return pasa(res, "%d personas con %d seudónimos distintos", len(nombres), len(unicos))
}

// evaluaConsistencia exige que la persona tenga un solo seudónimo en todas sus
// fuentes: una sola entidad reclama sus formas, el seudónimo aparece en sus
// registros y es el nombre que más veces aparece en ellos. No se exige que
// aparezca en cada registro: hay registros que identifican a la persona solo
// por su id interno o que nombran a un familiar, y eso es correcto.
func evaluaConsistencia(s *Salida, t Test) ResultadoTest {
	res := ResultadoTest{Test: t}
	formas := t.LookupForms
	if len(formas) == 0 {
		formas = formasDe(t.Forms)
	}
	if len(formas) == 0 {
		return omite(res, "el test no trae formas con que identificar a la persona")
	}
	entidad, cubiertas, ambigua := entidadDe(s, formas)
	switch {
	case ambigua:
		return falla(res, "más de una entidad declara las formas de la persona")
	case entidad == nil:
		return omite(res, "ninguna entidad de la salida declara sus formas (%d de %d): el test no aplica a esta salida", cubiertas, len(formas))
	}
	seudonimo := strings.TrimSpace(entidad.Pseudonyms["name"])
	if seudonimo == "" {
		return falla(res, "la persona %s no tiene seudónimo name", entidad.EntityID)
	}
	for _, otra := range s.Entities {
		for _, original := range otra.Originals {
			if Normaliza(original) == Normaliza(seudonimo) {
				return falla(res, "el seudónimo %q es una forma real declarada", seudonimo)
			}
		}
	}
	registros := t.Records
	if len(registros) == 0 {
		registros = entidad.Records
	}
	var textos []string
	for _, id := range registros {
		if r, ok := s.Registros[id]; ok {
			textos = append(textos, r.Texto())
		}
	}
	if len(textos) == 0 {
		return omite(res, "ninguno de los registros de la persona está en la salida")
	}
	if veces := cuentaEn(textos, seudonimo); veces == 0 {
		return falla(res, "el seudónimo no aparece en ninguno de sus %d registros", len(textos))
	}
	dueno, masVeces, otro := tokenDominante(s, textos, entidad)
	if otro {
		return falla(res, "el nombre %q aparece más que el seudónimo %q de %s", dueno, seudonimo, entidad.EntityID)
	}
	return pasa(res, "un solo seudónimo: %d apariciones, la más frecuente, en %d registros", masVeces, len(textos))
}

// cuentaEn dice en cuántos de los textos aparece el nombre.
func cuentaEn(textos []string, nombre string) int {
	veces := 0
	for _, texto := range textos {
		if Contiene(texto, nombre) {
			veces++
		}
	}
	return veces
}

// tokenDominante dice qué nombre de persona aparece más en los registros de una
// persona: el suyo tiene que estar entre los que más aparecen. Un registro
// puede nombrar a un familiar (p. ej. "en mi familia … también tiene cuenta"),
// así que el empate se acepta; que otro nombre domine es señal de que la
// persona quedó partida en dos seudónimos.
func tokenDominante(s *Salida, textos []string, entidad *modelo.Entity) (string, int, bool) {
	propio := strings.TrimSpace(entidad.Pseudonyms["name"])
	ajeno, ajenoVeces := "", 0
	for i := range s.Entities {
		e := &s.Entities[i]
		if e.Kept {
			continue
		}
		nombre := strings.TrimSpace(e.Pseudonyms["name"])
		if nombre == "" || Normaliza(nombre) == Normaliza(propio) {
			continue
		}
		if veces := cuentaEn(textos, nombre); veces > ajenoVeces {
			ajeno, ajenoVeces = nombre, veces
		}
	}
	vecesPropias := cuentaEn(textos, propio)
	return ajeno, vecesPropias, ajenoVeces > vecesPropias
}

// entidadDe elige la entidad que más formas del test declara. Ambiguo dice si
// dos entidades empatan declarando las mismas formas (unicidad rota).
func entidadDe(s *Salida, formas []string) (*modelo.Entity, int, bool) {
	var mejor *modelo.Entity
	mejorCubiertas := 0
	empate := false
	for i := range s.Entities {
		e := &s.Entities[i]
		if e.Kept {
			continue
		}
		cubiertas := 0
		for _, forma := range formas {
			if declaraForma(*e, forma) {
				cubiertas++
			}
		}
		switch {
		case cubiertas > mejorCubiertas:
			mejor, mejorCubiertas, empate = e, cubiertas, false
		case cubiertas == mejorCubiertas && cubiertas > 0:
			empate = true
		}
	}
	if empate {
		return nil, mejorCubiertas, true
	}
	return mejor, mejorCubiertas, false
}

// declaraForma dice si la entidad declara esa forma exacta en originals.
func declaraForma(e modelo.Entity, forma string) bool {
	objetivo := Normaliza(forma)
	for _, original := range e.Originals {
		if Normaliza(original) == objetivo {
			return true
		}
	}
	return false
}

// formasDe devuelve los valores del bloque forms de un test, ordenados para que
// el reporte sea determinista.
func formasDe(forms map[string]string) []string {
	claves := make([]string, 0, len(forms))
	for k := range forms {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	out := make([]string, 0, len(claves))
	for _, k := range claves {
		if v := strings.TrimSpace(forms[k]); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// EvalúaTests corre todos los tests contra la salida.
func EvalúaTests(s *Salida, tests []Test) []ResultadoTest {
	out := make([]ResultadoTest, 0, len(tests))
	for _, t := range tests {
		out = append(out, Evalúa(s, t))
	}
	return out
}
