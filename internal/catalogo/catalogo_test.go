package catalogo

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// norm replica la normalización del jurado (minúsculas, sin tildes, sin
// espacios de más) sin usar el código bajo prueba: es el oráculo del contraste.
func norm(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'á', 'à', 'â', 'ä', 'ã':
			r = 'a'
		case 'é', 'è', 'ê', 'ë':
			r = 'e'
		case 'í', 'ì', 'î', 'ï':
			r = 'i'
		case 'ó', 'ò', 'ô', 'ö', 'õ':
			r = 'o'
		case 'ú', 'ù', 'û', 'ü':
			r = 'u'
		case 'ñ':
			r = 'n'
		case 'ç':
			r = 'c'
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// grupos son los conjuntos de nombres de pila que publica el catálogo.
func grupos() []struct {
	nombre string
	genero modelo.Genero
	origen Origen
	minimo int
} {
	return []struct {
		nombre string
		genero modelo.Genero
		origen Origen
		minimo int
	}{
		{"colombiano M", modelo.GeneroM, OrigenColombiano, 300},
		{"colombiano F", modelo.GeneroF, OrigenColombiano, 300},
		{"colombiano unisex", modelo.GeneroUnisex, OrigenColombiano, 10},
		{"extranjero M", modelo.GeneroM, OrigenExtranjero, 100},
		{"extranjero F", modelo.GeneroF, OrigenExtranjero, 100},
	}
}

// TestConteosMinimos fija los mínimos del ticket: 300 nombres por género, 400
// apellidos y 8 ciudades con barrios.
func TestConteosMinimos(t *testing.T) {
	for _, g := range grupos() {
		if n := len(NombresPila(g.genero, g.origen)); n < g.minimo {
			t.Fatalf("%s: %d nombres, se esperaban al menos %d", g.nombre, n, g.minimo)
		}
	}
	if n := len(Apellidos()); n < 400 {
		t.Fatalf("%d apellidos, se esperaban al menos 400", n)
	}
	if n := len(Dominios()); n < 10 {
		t.Fatalf("%d dominios públicos, se esperaban al menos 10", n)
	}
	ciudades := Ciudades()
	if len(ciudades) < 8 {
		t.Fatalf("%d ciudades con barrios, se esperaban al menos 8: %v", len(ciudades), ciudades)
	}
	for _, ciudad := range ciudades {
		barrios, ok := Barrios(ciudad)
		if !ok {
			t.Fatalf("la ciudad %q está en Ciudades pero Barrios no la encuentra", ciudad)
		}
		if len(barrios) < 8 {
			t.Fatalf("%s tiene %d barrios, se esperaban al menos 8", ciudad, len(barrios))
		}
	}
	if n := len(Sectores()); n != 13 {
		t.Fatalf("%d sectores, D9 fija 13: %v", n, Sectores())
	}
}

// TestSinDuplicados compara normalizado: dos entradas que solo difieren en
// tildes o mayúsculas son la misma entrada repetida.
func TestSinDuplicados(t *testing.T) {
	revisa := func(origen string, lista []string) {
		t.Helper()
		vistos := map[string]int{}
		for i, valor := range lista {
			k := norm(valor)
			if j, ok := vistos[k]; ok {
				t.Fatalf("%s: %q (%d) repite %q (%d)", origen, valor, i, lista[j], j)
			}
			vistos[k] = i
		}
	}
	for _, g := range grupos() {
		revisa(g.nombre, NombresPila(g.genero, g.origen))
	}
	revisa("apellidos", Apellidos())
	revisa("dominios", Dominios())
	revisa("sectores", Sectores())
	for _, ciudad := range Ciudades() {
		barrios, _ := Barrios(ciudad)
		revisa("barrios de "+ciudad, barrios)
	}
}

// TestEntradasLimpias exige el contrato de forma de los datos: nombres,
// apellidos y dominios son una sola palabra sin dígitos ni "@"; barrios y
// sectores pueden tener espacios; todo es UTF-8 bien formado y sin mojibake.
func TestEntradasLimpias(t *testing.T) {
	mojibake := []string{"Ã", "Â", "â€", "©", "¢"}
	limpio := func(donde, valor string, permite string) {
		t.Helper()
		if valor == "" {
			t.Fatalf("%s: entrada vacía", donde)
		}
		if !utf8.ValidString(valor) {
			t.Fatalf("%s: %q no es UTF-8 válido", donde, valor)
		}
		if valor != strings.TrimSpace(valor) {
			t.Fatalf("%s: %q tiene espacios en los extremos", donde, valor)
		}
		if strings.Contains(valor, "  ") {
			t.Fatalf("%s: %q tiene espacios dobles", donde, valor)
		}
		for _, m := range mojibake {
			if strings.Contains(valor, m) {
				t.Fatalf("%s: %q parece mojibake", donde, valor)
			}
		}
		for _, r := range valor {
			if unicode.IsDigit(r) {
				t.Fatalf("%s: %q tiene un dígito", donde, valor)
			}
			if r == '@' {
				t.Fatalf("%s: %q tiene arroba", donde, valor)
			}
			if r != ' ' && !unicode.IsLetter(r) && !strings.ContainsRune(permite, r) {
				t.Fatalf("%s: %q tiene el carácter %q", donde, valor, r)
			}
			if r == ' ' && !strings.ContainsRune(permite, ' ') {
				t.Fatalf("%s: %q tiene un espacio interno", donde, valor)
			}
		}
	}
	// Los nombres propios viajan en forma de título; los dominios y los
	// sectores, en minúsculas como en la salida ideal.
	titulo := func(donde, valor string) {
		t.Helper()
		if r, _ := utf8.DecodeRuneInString(valor); !unicode.IsUpper(r) {
			t.Fatalf("%s: %q no empieza en mayúscula", donde, valor)
		}
		if valor == strings.ToUpper(valor) {
			t.Fatalf("%s: %q está todo en mayúsculas", donde, valor)
		}
	}
	for _, g := range grupos() {
		for _, nombre := range NombresPila(g.genero, g.origen) {
			limpio(g.nombre, nombre, "")
			titulo(g.nombre, nombre)
		}
	}
	for _, apellido := range Apellidos() {
		limpio("apellidos", apellido, "")
		titulo("apellidos", apellido)
	}
	for _, dominio := range Dominios() {
		if !strings.Contains(dominio, ".") || dominio != strings.ToLower(dominio) ||
			strings.HasPrefix(dominio, ".") || strings.HasSuffix(dominio, ".") {
			t.Fatalf("dominios: %q no es un dominio público en minúsculas", dominio)
		}
		limpio("dominios", dominio, ".")
	}
	for _, ciudad := range Ciudades() {
		limpio("ciudades", ciudad, " ")
		titulo("ciudades", ciudad)
		barrios, _ := Barrios(ciudad)
		for _, barrio := range barrios {
			limpio("barrios de "+ciudad, barrio, " ")
			titulo("barrios de "+ciudad, barrio)
		}
	}
	for _, sector := range Sectores() {
		if sector != strings.ToLower(sector) {
			t.Fatalf("sectores: %q debe viajar en minúsculas, como en la salida ideal", sector)
		}
		limpio("sectores", sector, " ")
	}
}

// TestTildesCorrectas contrasta la ortografía de entradas con tilde y eñe: un
// catálogo sin tildes delata el seudónimo y pierde parecido.
func TestTildesCorrectas(t *testing.T) {
	contiene := func(lista []string, valor string) bool {
		for _, v := range lista {
			if v == valor {
				return true
			}
		}
		return false
	}
	casos := []struct {
		donde   string
		lista   []string
		valores []string
	}{
		{"nombres M", NombresPila(modelo.GeneroM, OrigenColombiano), []string{"Aarón", "Abdías", "Abrahán", "Álvaro", "Gustavo"}},
		{"nombres F", NombresPila(modelo.GeneroF, OrigenColombiano), []string{"Aída", "Ángela", "Bárbara", "Fátima", "Jazmín"}},
		{"apellidos", Apellidos(), []string{"Acuña", "Estupiñán", "Gámez", "Peña", "Zúñiga"}},
		{"ciudades", Ciudades(), []string{"Bogotá", "Medellín", "Itagüí"}},
	}
	for _, c := range casos {
		for _, valor := range c.valores {
			if !contiene(c.lista, valor) {
				t.Fatalf("%s: falta %q con su tilde", c.donde, valor)
			}
		}
	}
	barrios, ok := Barrios("Bogotá")
	if !ok || !contiene(barrios, "La Candelaria") {
		t.Fatal("Bogotá sin el barrio La Candelaria")
	}
	barrios, _ = Barrios("Cali")
	if !contiene(barrios, "El Peñón") {
		t.Fatal("Cali sin el barrio El Peñón")
	}
}

// TestIndices cubre la petición determinista por índice de la fábrica (D4): el
// índice sale de la semilla HMAC en C13, el catálogo solo lo recorre.
func TestIndices(t *testing.T) {
	for _, g := range grupos() {
		lista := NombresPila(g.genero, g.origen)
		for i := range lista {
			if got := NombrePila(g.genero, g.origen, i); got != lista[i] {
				t.Fatalf("%s: índice %d da %q, se esperaba %q", g.nombre, i, got, lista[i])
			}
			if got := NombrePila(g.genero, g.origen, i); got != NombrePila(g.genero, g.origen, i) {
				t.Fatalf("%s: índice %d no es determinista", g.nombre, i)
			}
			if i+1 < len(lista) && NombrePila(g.genero, g.origen, i) == NombrePila(g.genero, g.origen, i+1) {
				t.Fatalf("%s: los índices %d y %d repiten nombre", g.nombre, i, i+1)
			}
		}
		if got, quiero := NombrePila(g.genero, g.origen, len(lista)), lista[0]; got != quiero {
			t.Fatalf("%s: el índice %d debe dar la vuelta a %q, dio %q", g.nombre, len(lista), quiero, got)
		}
		if got, quiero := NombrePila(g.genero, g.origen, -1), lista[len(lista)-1]; got != quiero {
			t.Fatalf("%s: el índice -1 debe dar %q, dio %q", g.nombre, quiero, got)
		}
	}
	if NombrePila(modelo.GeneroM, OrigenColombiano, 0) != NombrePila(modelo.GeneroM, "", 0) {
		t.Fatal("el origen vacío debe valer por colombiano")
	}
	if NombrePila(modelo.GeneroIndeterminado, OrigenColombiano, 3) != NombrePila(modelo.GeneroUnisex, OrigenColombiano, 3) {
		t.Fatal("indeterminado y unisex deben compartir catálogo (D18, D27)")
	}
	if NombrePila(modelo.GeneroUnisex, OrigenExtranjero, 0) != "" {
		t.Fatal("un grupo vacío debe devolver cadena vacía, no entrar en pánico")
	}
	apellidos := Apellidos()
	for i := range apellidos {
		if got := Apellido(i); got != apellidos[i] {
			t.Fatalf("apellido %d: %q, se esperaba %q", i, got, apellidos[i])
		}
	}
	if Apellido(-1) != apellidos[len(apellidos)-1] {
		t.Fatal("el apellido -1 debe dar el último")
	}
	dominios := Dominios()
	if Dominio(0) != dominios[0] || Dominio(len(dominios)) != dominios[0] {
		t.Fatal("Dominio no recorre la lista de dominios por índice")
	}
}

// TestBarrios exige el reemplazo por otro barrio de la misma ciudad: el valor
// elegido cambia y sigue perteneciendo a la misma ciudad (D9).
func TestBarrios(t *testing.T) {
	if _, ok := Barrios("Atlantis"); ok {
		t.Fatal("una ciudad inventada no debe tener barrios")
	}
	barrios, ok := Barrios("MEDELLÍN")
	if !ok {
		t.Fatal("la búsqueda de ciudad debe ignorar mayúsculas y tildes")
	}
	for _, barrio := range barrios {
		for i := 0; i < len(barrios)+2; i++ {
			otro, ok := BarrioAlterno("Medellín", barrio, i)
			if !ok {
				t.Fatalf("BarrioAlterno no encontró alternativa para %q", barrio)
			}
			if norm(otro) == norm(barrio) {
				t.Fatalf("BarrioAlterno repitió el barrio original %q", barrio)
			}
			if got, ok := Barrio("Medellín", i); !ok || got != barrios[i%len(barrios)] {
				t.Fatalf("Barrio(%d) = %q, se esperaba %q", i, got, barrios[i%len(barrios)])
			}
		}
	}
	if _, ok := BarrioAlterno("Atlantis", "Centro", 0); ok {
		t.Fatal("BarrioAlterno no debe inventar barrios para una ciudad desconocida")
	}
}

// TestSector cubre el mapa ocupación → 13 sectores (D9, D18). El sector es
// neutro y viaja en minúsculas, como en la salida ideal.
func TestSector(t *testing.T) {
	casos := []struct{ ocupacion, quiero string }{
		{"fotógrafa", "profesional de las industrias creativas"},
		{"diseñador gráfico", "profesional de las industrias creativas"},
		{"community manager", "profesional de medios"},
		{"administradora", "profesional de servicios"},
		{"mecánico", "profesional del transporte"},
		{"contadora", "profesional de finanzas"},
		{"abogada", "profesional del sector legal"},
		{"enfermera", "profesional de la salud"},
		{"ingeniero civil", "profesional de la construcción"},
		{"ingeniera de sistemas", "profesional de tecnología"},
		{"panadero", "profesional de la gastronomía"},
		{"docente", "profesional de la educación"},
		{"comerciante", "profesional del comercio"},
		{"estudiante de derecho", "estudiante universitario"},
	}
	sectores := map[string]bool{}
	for _, s := range Sectores() {
		sectores[s] = true
	}
	for _, c := range casos {
		got, ok := Sector(c.ocupacion)
		if !ok {
			t.Fatalf("sin sector para %q", c.ocupacion)
		}
		if got != c.quiero {
			t.Fatalf("%q: sector %q, se esperaba %q", c.ocupacion, got, c.quiero)
		}
		if !sectores[got] {
			t.Fatalf("%q: sector %q fuera de los 13 de D9", c.ocupacion, got)
		}
	}
	if _, ok := Sector("astronauta"); ok {
		t.Fatal("una ocupación fuera del mapa no debe inventar sector")
	}
	if got, ok := Sector("  MECANICO "); !ok || got != "profesional del transporte" {
		t.Fatalf("la búsqueda debe ignorar mayúsculas, tildes y espacios: %q,%v", got, ok)
	}
	data, err := archivos.ReadFile("datos/sectores.json")
	if err != nil {
		t.Fatalf("lee sectores.json: %v", err)
	}
	var doc struct {
		Sectores    []string          `json:"sectores"`
		Ocupaciones map[string]string `json:"ocupaciones"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("sectores.json: %v", err)
	}
	usados := map[string]int{}
	for ocupacion, sector := range doc.Ocupaciones {
		if !sectores[sector] {
			t.Fatalf("%q apunta al sector %q, que no está entre los 13", ocupacion, sector)
		}
		if ocupacion != strings.ToLower(ocupacion) {
			t.Fatalf("la ocupación %q debe viajar en minúsculas", ocupacion)
		}
		usados[sector]++
	}
	for _, sector := range doc.Sectores {
		if usados[sector] == 0 {
			t.Fatalf("el sector %q no tiene ninguna ocupación", sector)
		}
	}
}

// fuentes devuelve el directorio del corpus real; la prueba se salta si no
// está, porque `fuentes/` no viaja en el repo.
func fuentes(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "fuentes")
	if _, err := os.Stat(filepath.Join(dir, "tables", "clientes.csv")); err != nil {
		t.Skipf("sin corpus en %s: %v", dir, err)
	}
	return dir
}

// latin1 decodifica el CSV del corpus, que viene en latin-1.
func latin1(crudo []byte) string {
	var b strings.Builder
	for _, c := range crudo {
		b.WriteRune(rune(c))
	}
	return b.String()
}

// formasReales junta las formas personales del corpus que el catálogo no puede
// contener: nombres completos, correos, cédulas y celulares. Los mensajes de
// error no las imprimen (regla de logs sin PII); solo cuentan e indican índices.
type formasReales struct {
	nombres   []string
	correos   []string
	cedulas   []string
	celulares []string
}

func (f *formasReales) nombre(s string) {
	campos := strings.Fields(s)
	if len(campos) >= 2 {
		f.nombres = append(f.nombres, strings.Join(campos, " "))
	}
}

func (f *formasReales) llenar(dir string, t *testing.T) {
	t.Helper()
	crudo, err := os.ReadFile(filepath.Join(dir, "tables", "clientes.csv"))
	if err != nil {
		t.Fatalf("lee clientes.csv: %v", err)
	}
	lector := csv.NewReader(strings.NewReader(latin1(crudo)))
	lector.Comma = ';'
	filas, err := lector.ReadAll()
	if err != nil {
		t.Fatalf("parsea clientes.csv: %v", err)
	}
	columna := map[string]int{}
	for i, c := range filas[0] {
		columna[c] = i
	}
	for _, fila := range filas[1:] {
		f.nombre(fila[columna["nombre_completo"]])
		f.correos = append(f.correos, fila[columna["correo"]])
		f.cedulas = append(f.cedulas, fila[columna["num_doc"]])
		f.celulares = append(f.celulares, fila[columna["celular"]])
	}

	var empleados struct {
		Empleados []struct {
			Nombre    string `xml:"nombre"`
			Correo    string `xml:"correo"`
			Celular   string `xml:"celular"`
			Documento string `xml:"documento"`
		} `xml:"empleado"`
	}
	crudo, err = os.ReadFile(filepath.Join(dir, "tables", "empleados.xml"))
	if err != nil {
		t.Fatalf("lee empleados.xml: %v", err)
	}
	if err := xml.Unmarshal(crudo, &empleados); err != nil {
		t.Fatalf("parsea empleados.xml: %v", err)
	}
	for _, e := range empleados.Empleados {
		f.nombre(e.Nombre)
		f.correos = append(f.correos, e.Correo)
		f.celulares = append(f.celulares, e.Celular)
		f.cedulas = append(f.cedulas, e.Documento)
	}

	var usuarios []struct {
		Nombre   string `json:"name"`
		RealName string `json:"real_name"`
		EsBot    bool   `json:"is_bot"`
		Profile  struct {
			RealName string `json:"real_name"`
			Email    string `json:"email"`
			Phone    string `json:"phone"`
		} `json:"profile"`
	}
	crudo, err = os.ReadFile(filepath.Join(dir, "chats", "users.json"))
	if err != nil {
		t.Fatalf("lee users.json: %v", err)
	}
	if err := json.Unmarshal(crudo, &usuarios); err != nil {
		t.Fatalf("parsea users.json: %v", err)
	}
	for _, u := range usuarios {
		if u.EsBot {
			continue
		}
		f.nombre(u.RealName)
		f.nombre(u.Profile.RealName)
		f.correos = append(f.correos, u.Profile.Email)
		f.celulares = append(f.celulares, u.Profile.Phone)
	}

	var pqrs struct {
		PQRS []struct {
			Cedula string `xml:"cedula,attr"`
			Agente string `xml:"agente,attr"`
		} `xml:"pqr"`
	}
	crudo, err = os.ReadFile(filepath.Join(dir, "tables", "pqrs.xml"))
	if err != nil {
		t.Fatalf("lee pqrs.xml: %v", err)
	}
	if err := xml.Unmarshal(crudo, &pqrs); err != nil {
		t.Fatalf("parsea pqrs.xml: %v", err)
	}
	for _, p := range pqrs.PQRS {
		f.cedulas = append(f.cedulas, p.Cedula)
		f.nombre(strings.ReplaceAll(p.Agente, ".", " "))
	}
}

// tokens junta los tokens de los nombres completos reales, normalizados.
func (f *formasReales) tokens() map[string]bool {
	tokens := map[string]bool{}
	for _, nombre := range f.nombres {
		for _, token := range strings.Fields(nombre) {
			tokens[norm(token)] = true
		}
	}
	return tokens
}

// pares junta los pares de tokens consecutivos de los nombres reales ("nombre
// de pila + primer apellido", "primer + segundo apellido").
func (f *formasReales) pares() map[string]bool {
	pares := map[string]bool{}
	for _, nombre := range f.nombres {
		campos := strings.Fields(nombre)
		for i := 0; i+1 < len(campos); i++ {
			pares[norm(campos[i])+" "+norm(campos[i+1])] = true
		}
	}
	return pares
}

// TestSinFormasReales es la prueba de fuga del catálogo: ninguna forma personal
// del corpus aparece en los datos embebidos. Se salta si `fuentes/` no está.
func TestSinFormasReales(t *testing.T) {
	dir := fuentes(t)
	var reales formasReales
	reales.llenar(dir, t)
	if len(reales.nombres) == 0 || len(reales.correos) == 0 {
		t.Fatal("no se leyeron formas reales del corpus")
	}

	// 1. Ninguna entrada del catálogo contiene una forma real. Los mensajes
	//    solo cuentan e indican el índice: el valor ofensor es un dato real.
	todo := []string{}
	for _, g := range grupos() {
		todo = append(todo, NombresPila(g.genero, g.origen)...)
	}
	todo = append(todo, Apellidos()...)
	todo = append(todo, Dominios()...)
	todo = append(todo, Sectores()...)
	for _, ciudad := range Ciudades() {
		barrios, _ := Barrios(ciudad)
		todo = append(todo, barrios...)
	}
	formas := [][]string{reales.nombres, reales.correos, reales.cedulas, reales.celulares}
	etiquetas := []string{"nombre completo", "correo", "cédula", "celular"}
	for i, lista := range formas {
		for _, forma := range lista {
			if strings.TrimSpace(forma) == "" {
				continue
			}
			for k, valor := range todo {
				if strings.Contains(norm(valor), norm(forma)) {
					t.Fatalf("un %s del corpus aparece en la entrada %d del catálogo", etiquetas[i], k)
				}
			}
		}
	}

	// 2. Ningún token del catálogo coincide con un token de un nombre real: con
	//    eso, ninguna combinación de nombre y apellido del catálogo puede
	//    reproducir un nombre completo del corpus.
	tokens := reales.tokens()
	for _, g := range grupos() {
		lista := NombresPila(g.genero, g.origen)
		for i, nombre := range lista {
			if tokens[norm(nombre)] {
				t.Fatalf("%s[%d]: el catálogo copia un token de nombre real del corpus", g.nombre, i)
			}
		}
	}
	apellidos := Apellidos()
	for i, apellido := range apellidos {
		if tokens[norm(apellido)] {
			t.Fatalf("apellidos[%d]: el catálogo copia un token de nombre real del corpus", i)
		}
	}

	// 3. Ningún nombre completo falso de dos palabras coincide con un par
	//    consecutivo de un nombre real.
	pares := reales.pares()
	for _, g := range grupos() {
		for i, nombre := range NombresPila(g.genero, g.origen) {
			for j, apellido := range apellidos {
				if pares[norm(nombre)+" "+norm(apellido)] {
					t.Fatalf("%s[%d] + apellidos[%d] reproduce un nombre real", g.nombre, i, j)
				}
			}
		}
	}
}

// TestCorpusCubierto comprueba contra el corpus real que las ciudades y los
// barrios se resuelven y que las ocupaciones tienen sector.
func TestCorpusCubierto(t *testing.T) {
	dir := fuentes(t)
	crudo, err := os.ReadFile(filepath.Join(dir, "tables", "clientes.csv"))
	if err != nil {
		t.Fatalf("lee clientes.csv: %v", err)
	}
	lector := csv.NewReader(strings.NewReader(latin1(crudo)))
	lector.Comma = ';'
	filas, err := lector.ReadAll()
	if err != nil {
		t.Fatalf("parsea clientes.csv: %v", err)
	}
	columna := map[string]int{}
	for i, c := range filas[0] {
		columna[c] = i
	}
	vistos := map[string]bool{}
	for n, fila := range filas[1:] {
		ciudad, barrio, ocupacion := fila[columna["ciudad"]], fila[columna["barrio"]], fila[columna["ocupacion"]]
		if _, ok := Sector(ocupacion); !ok {
			t.Fatalf("fila %d: la ocupación %q no tiene sector", n, ocupacion)
		}
		if _, ok := Barrios(ciudad); !ok {
			t.Fatalf("fila %d: la ciudad %q no tiene barrios en el catálogo", n, ciudad)
		}
		clave := norm(ciudad) + "|" + norm(barrio)
		if vistos[clave] {
			continue
		}
		vistos[clave] = true
		if _, ok := Barrio(ciudad, n); !ok {
			t.Fatalf("fila %d: Barrio(%q, %d) no resolvió", n, ciudad, n)
		}
		otro, ok := BarrioAlterno(ciudad, barrio, n)
		if !ok {
			t.Fatalf("fila %d: sin barrio alterno en %q", n, ciudad)
		}
		if norm(otro) == norm(barrio) {
			t.Fatalf("fila %d: el barrio alterno repite el original en %q", n, ciudad)
		}
	}
}
