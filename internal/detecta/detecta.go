// Package detecta encuentra en los registros los fragmentos que son datos
// personales: formas conocidas del registro (la capa principal), patrones con
// contexto y candidatos que el modelo clasifica después (D14, D30).
//
// Toda la búsqueda corre sobre el espacio normalizado del jurado —NFKC,
// confusables, sin tildes, minúsculas, dígitos sin separadores— con un mapa de
// offsets al original, porque el reemplazo es literal (D5, D6). La primera
// pasada recorre el corpus entero y devuelve al registro las formas que solo
// aparecen en el texto (D14).
package detecta

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/campos"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// ventana es cuántos bytes del texto normalizado se miran a cada lado para
// exigir contexto a los patrones.
const ventana = 60

// Opciones arma el detector: el registro aporta las formas conocidas y la
// allowlist lo que se conserva.
type Opciones struct {
	Indice    *registro.Indice
	Allowlist *Allowlist
}

// Candidato es un fragmento que las reglas proponen y el modelo clasifica
// (D30). Generador dice qué regla lo propuso.
type Candidato struct {
	Registro  string
	Fuente    modelo.Fuente
	Campo     modelo.CampoTexto
	Inicio    int
	Fin       int
	Valor     string
	Generador string
}

// Detector busca formas conocidas y patrones.
type Detector struct {
	indice *registro.Indice
	allow  *Allowlist
	ac     *ac
	agujas []aguja
	reglas []patron
}

// aguja es una forma normalizada con las personas y los tipos que la declaran.
type aguja struct {
	formas []modelo.Forma
	llaves []string
}

// Nuevo indexa todas las formas del registro. Falla sin registro: detectar sin
// saber qué personas existen no protege a nadie.
func Nuevo(o Opciones) (*Detector, error) {
	if o.Indice == nil {
		return nil, errSinRegistro
	}
	allow := o.Allowlist
	if allow == nil {
		allow = PorDefecto()
	}
	porValor := map[string]*aguja{}
	orden := []string{}
	for _, p := range o.Indice.Personas() {
		for _, f := range p.Formas {
			clave := norm.Norm(f.Valor)
			if clave == "" {
				continue
			}
			a, ok := porValor[clave]
			if !ok {
				a = &aguja{}
				porValor[clave] = a
				orden = append(orden, clave)
			}
			a.formas = append(a.formas, f)
			a.llaves = appendUnico(a.llaves, p.Llave)
		}
	}
	sort.Strings(orden)
	agujas := make([]aguja, 0, len(orden))
	textos := make([][]byte, 0, len(orden))
	for _, clave := range orden {
		agujas = append(agujas, *porValor[clave])
		textos = append(textos, []byte(clave))
	}
	return &Detector{
		indice: o.Indice,
		allow:  allow,
		ac:     nuevoAC(textos),
		agujas: agujas,
		reglas: patronesDeReglas(),
	}, nil
}

// Detecta devuelve las menciones de todos los registros en orden de entrada. El
// contexto entre mensajes del mismo hilo se encadena para clasificar respuestas
// de solo dígitos (CE-16).
func (d *Detector) Detecta(_ context.Context, recs []modelo.Registro) ([]modelo.Mencion, error) {
	var out []modelo.Mencion
	previoPorHilo := map[string]string{}
	for _, r := range recs {
		anterior := ""
		hilo := ""
		if chat, ok := r.(modelo.ChatRecord); ok {
			hilo = chat.ThreadID
			anterior = previoPorHilo[hilo]
		}
		for _, campo := range campos.Lista(r) {
			out = append(out, d.enCampo(r, campo, anterior)...)
		}
		if hilo != "" {
			previoPorHilo[hilo] = recortaContexto(anterior + " " + textoDe(r))
		}
	}
	return out, nil
}

// Candidatos propone los fragmentos que se escapan de las formas conocidas: los
// cuatro generadores de D30, insensibles a mayúsculas donde el idioma lo permite.
func (d *Detector) Candidatos(recs []modelo.Registro) []Candidato {
	var out []Candidato
	for _, r := range recs {
		for _, campo := range campos.Lista(r) {
			if campo.Valor == "" {
				continue
			}
			conocidas := d.enCampo(r, campo, "")
			for _, generador := range generadores {
				for _, c := range generador.busca(campo.Valor) {
					if _, ok := d.allow.Conserva(c.valor); ok {
						continue
					}
					if solapaAlguna(conocidas, c.inicio, c.fin) {
						continue
					}
					out = append(out, Candidato{
						Registro:  r.IDRegistro(),
						Fuente:    r.FuenteRegistro(),
						Campo:     campo.Ruta,
						Inicio:    c.inicio,
						Fin:       c.fin,
						Valor:     campo.Valor[c.inicio:c.fin],
						Generador: generador.nombre,
					})
				}
			}
		}
	}
	return out
}

// enCampo busca formas conocidas y patrones en un string del registro.
func (d *Detector) enCampo(r modelo.Registro, campo campos.Campo, anterior string) []modelo.Mencion {
	if campo.Valor == "" {
		return nil
	}
	mapa := norm.Normaliza(campo.Valor)
	conocidas := d.conocidas(r, campo, mapa, anterior)
	patrones := d.patrones(r, campo, mapa, anterior)
	return fusiona(conocidas, patrones)
}

// conocidas busca las formas del registro y se queda con la más larga en cada
// arranque: "Manuela Castaño Ruiz" gana a "Manuela Castaño".
func (d *Detector) conocidas(r modelo.Registro, campo campos.Campo, mapa norm.Mapa, anterior string) []modelo.Mencion {
	if len(d.agujas) == 0 {
		return nil
	}
	elegidas := eligeLargas(d.ac.busca(mapa.Texto), mapa.Texto)
	var out []modelo.Mencion
	for _, c := range elegidas {
		inicio, fin, ok := mapa.Original(c.inicio, c.fin)
		if !ok {
			continue
		}
		a := d.agujas[c.aguja]
		fragmento := campo.Valor[inicio:fin]
		if d.conservada(fragmento, campo.Valor, inicio, fin) {
			continue
		}
		forma := mejorForma(a.formas, fragmento)
		persona := ""
		regla := "forma_conocida"
		confianza := 1.0
		switch {
		case len(a.llaves) == 1:
			persona = a.llaves[0]
		default:
			regla = "forma_compartida"
			confianza = 0.6
		}
		// D25: un nombre de pila suelto solo se reemplaza cuando el registro
		// tiene un único dueño. Con dos candidatos decide la resolución, no la
		// cadena.
		if forma.Tipo == modelo.FormaNombrePila && persona == "" {
			continue
		}
		out = append(out, modelo.Mencion{
			Registro:  r.IDRegistro(),
			Fuente:    r.FuenteRegistro(),
			Campo:     campo.Ruta,
			Inicio:    inicio,
			Fin:       fin,
			Valor:     fragmento,
			Tipo:      modelo.MencionPersona,
			Forma:     forma.Tipo,
			Persona:   persona,
			Regla:     regla,
			Confianza: confianza,
		})
	}
	return out
}

// conservada aplica la allowlist y la regla de apellido-lugar (CE-14, CE-15).
func (d *Detector) conservada(fragmento, texto string, inicio, fin int) bool {
	if _, ok := d.allow.Conserva(fragmento); ok {
		return true
	}
	antes := texto[:inicio]
	despues := texto[fin:]
	if d.allow.ConservaApellidoLugar(fragmento, antes) {
		return true
	}
	if d.allow.EsPalabraComun(fragmento) {
		return true
	}
	if d.allow.EsPrefijoDeEntidad(antes) {
		return true
	}
	if d.allow.EsSufijoDeRazonSocial(antes, despues) {
		return true
	}
	return false
}

// patrones aplica las reglas con contexto al texto normalizado del campo.
func (d *Detector) patrones(r modelo.Registro, campo campos.Campo, mapa norm.Mapa, anterior string) []modelo.Mencion {
	if len(d.reglas) == 0 {
		return nil
	}
	contexto := strings.ToLower(anterior + " " + mapa.Texto)
	var out []modelo.Mencion
	for _, regla := range d.reglas {
		for _, tramo := range regla.re.FindAllStringIndex(mapa.Texto, -1) {
			if !regla.conContexto(mapa.Texto, tramo[0], tramo[1], anterior) {
				continue
			}
			inicio, fin, ok := mapa.Original(tramo[0], tramo[1])
			if !ok || inicio >= fin {
				continue
			}
			fragmento := campo.Valor[inicio:fin]
			if d.conservada(fragmento, campo.Valor, inicio, fin) {
				continue
			}
			forma := regla.formaPara(fragmento)
			out = append(out, modelo.Mencion{
				Registro:  r.IDRegistro(),
				Fuente:    r.FuenteRegistro(),
				Campo:     campo.Ruta,
				Inicio:    inicio,
				Fin:       fin,
				Valor:     fragmento,
				Tipo:      modelo.MencionPersona,
				Forma:     forma,
				Regla:     "patron:" + regla.nombre,
				Confianza: regla.confianza,
			})
		}
	}
	_ = contexto
	return out
}

// eligeLargas se queda con la coincidencia más larga en cada arranque y
// descarta las que se solapan con una ya elegida.
func eligeLargas(coincidencias []coincidencia, texto string) []coincidencia {
	ordenadas := append([]coincidencia{}, coincidencias...)
	sort.SliceStable(ordenadas, func(i, j int) bool {
		if ordenadas[i].inicio != ordenadas[j].inicio {
			return ordenadas[i].inicio < ordenadas[j].inicio
		}
		return ordenadas[i].fin > ordenadas[j].fin
	})
	var out []coincidencia
	fin := -1
	for _, c := range ordenadas {
		if c.inicio < fin {
			continue
		}
		if !bordesDePalabra(texto, c.inicio, c.fin) {
			continue
		}
		out = append(out, c)
		fin = c.fin
	}
	return out
}

// bordesDePalabra exige que una aguja alfabética no esté pegada a otra letra o
// dígito, y que una de solo dígitos no tenga otro dígito al lado (D5).
func bordesDePalabra(texto string, inicio, fin int) bool {
	if inicio > 0 && esPalabra(texto[inicio-1]) {
		return false
	}
	if fin < len(texto) && esPalabra(texto[fin]) {
		return false
	}
	return true
}

func esPalabra(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b >= 0x80
}

// mejorForma elige, entre las formas que comparten valor normalizado, la que
// tiene el mismo patrón de puntuación y caja que el fragmento real: así el
// reemplazo conserva puntos, MAYÚSCULAS y separadores.
func mejorForma(formas []modelo.Forma, fragmento string) modelo.Forma {
	if len(formas) == 1 {
		return formas[0]
	}
	patronFragmento := patronDe(fragmento)
	mejor := formas[0]
	mejorPuntaje := -1
	for _, f := range formas {
		puntaje := 0
		patron := patronDe(f.Valor)
		if patron == patronFragmento {
			puntaje += 100
		}
		if len(patron) == len(patronFragmento) {
			puntaje += 10
		}
		for i := 0; i < len(patron) && i < len(patronFragmento); i++ {
			if patron[i] == patronFragmento[i] {
				puntaje++
			}
		}
		if puntaje > mejorPuntaje {
			mejor, mejorPuntaje = f, puntaje
		}
	}
	return mejor
}

// patronDe reduce un valor a la forma de sus caracteres: d=dígito, A=mayúscula,
// a=minúscula, y el resto literal.
func patronDe(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteByte('d')
		case r >= 'A' && r <= 'Z' || r >= 'Á' && r <= 'Ú' || r == 'Ñ':
			b.WriteByte('A')
		case r == ' ' || r == '.' || r == '-' || r == '_' || r == '@' || r == '+' || r == '#' || r == '/':
			b.WriteRune(r)
		default:
			b.WriteByte('a')
		}
	}
	return b.String()
}

// fusiona deja las formas conocidas y descarta los patrones que caen dentro de
// una: la cédula de una persona conocida no necesita además el patrón.
func fusiona(conocidas, patrones []modelo.Mencion) []modelo.Mencion {
	out := append([]modelo.Mencion{}, conocidas...)
	for _, p := range patrones {
		if solapaAlguna(out, p.Inicio, p.Fin) {
			continue
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Campo != out[j].Campo {
			return out[i].Campo < out[j].Campo
		}
		return out[i].Inicio < out[j].Inicio
	})
	return out
}

func solapaAlguna(menciones []modelo.Mencion, inicio, fin int) bool {
	for _, m := range menciones {
		if inicio < m.Fin && m.Inicio < fin {
			return true
		}
	}
	return false
}

func textoDe(r modelo.Registro) string {
	var partes []string
	for _, campo := range campos.Lista(r) {
		if campo.Ruta == "text" || campo.Ruta == "body" {
			partes = append(partes, campo.Valor)
		}
	}
	return strings.Join(partes, " ")
}

func recortaContexto(s string) string {
	const max = 600
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

func appendUnico(lista []string, valor string) []string {
	for _, v := range lista {
		if v == valor {
			return lista
		}
	}
	return append(lista, valor)
}

// errSinRegistro evita un detector que no sabe a quién proteger.
var errSinRegistro = errSinRegistroT{}

type errSinRegistroT struct{}

func (errSinRegistroT) Error() string { return "detecta: falta el registro de personas" }

// patron describe una regla con contexto. contexto vacío significa que la regla
// no lo necesita (correo, por ejemplo); si no, basta una de las palabras en la
// ventana.
type patron struct {
	nombre    string
	re        *regexp.Regexp
	forma     modelo.TipoForma
	contexto  []string
	confianza float64
}

// conContexto mira la ventana a la izquierda (y algo a la derecha) del tramo: la
// mayoría de los patrones solo esconde un dato cuando la frase lo anuncia.
func (p patron) conContexto(texto string, inicio, fin int, anterior string) bool {
	if len(p.contexto) == 0 {
		return true
	}
	desde := inicio - ventana
	if desde < 0 {
		desde = 0
	}
	hasta := fin + ventana
	if hasta > len(texto) {
		hasta = len(texto)
	}
	zona := texto[desde:hasta]
	if anterior != "" {
		zona = strings.ToLower(anterior) + " " + zona
	}
	for _, palabra := range p.contexto {
		if strings.Contains(zona, palabra) {
			return true
		}
	}
	return false
}

// formaPara ajusta el estilo del reemplazo al fragmento encontrado.
func (p patron) formaPara(fragmento string) modelo.TipoForma {
	switch p.forma {
	case modelo.FormaCelular:
		switch {
		case strings.Contains(fragmento, "+57"):
			return modelo.FormaCelularE164
		case strings.Contains(fragmento, "-"):
			return modelo.FormaCelularLocal
		}
	case modelo.FormaCedula:
		switch {
		case strings.Contains(strings.ToLower(fragmento), "cc "):
			return modelo.FormaCedulaCC
		case strings.Contains(fragmento, "."):
			return modelo.FormaCedulaPuntos
		}
	}
	return p.forma
}

// patronesDeReglas compila una sola vez las reglas de detección.
func patronesDeReglas() []patron {
	must := func(expr string) *regexp.Regexp { return regexp.MustCompile(expr) }
	meses := "(enero|febrero|marzo|abril|mayo|junio|julio|agosto|septiembre|octubre|noviembre|diciembre)"
	documento := []string{"cedula", "documento", "identificacion", "cc ", "id no", "numero de documento", "me confirmas", "cual es tu"}
	nacimiento := []string{"nacimiento", "naci", "dob", "cumple", "fecha de nacimiento"}
	contacto := []string{"celular", "telefono", "whatsapp", "llamar", "llamame", "contacto", "+57"}
	return []patron{
		{nombre: "correo_deletreado", re: must(`[a-z0-9]+(?:\s+punto\s+[a-z0-9]+)+\s+arroba\s+[a-z0-9]+(?:\s+punto\s+[a-z0-9]+)+`), forma: modelo.FormaCorreoDeletreado, confianza: 0.9},
		{nombre: "correo", re: must(`[a-z0-9][a-z0-9._%+-]*@[a-z0-9.-]+\.(com|co|net|org|edu|gov|io|info|biz|us|uk|es|mx|ar|cl|pe|ec)`), forma: modelo.FormaCorreo, confianza: 0.95},
		{nombre: "celular", re: must(`(?:\+57[\s.-]*)?3\d{2}[\s.-]?\d{3}[\s.-]?\d{4}`), forma: modelo.FormaCelular, contexto: contacto, confianza: 0.8},
		{nombre: "cedula_puntos", re: must(`\d{1,3}(?:\.\d{3}){1,3}`), forma: modelo.FormaCedula, contexto: documento, confianza: 0.8},
		{nombre: "cedula", re: must(`(?:cc\s*)?\d{6,10}`), forma: modelo.FormaCedula, contexto: documento, confianza: 0.7},
		{nombre: "fecha_iso", re: must(`\d{4}-\d{2}-\d{2}`), forma: modelo.FormaFechaISO, contexto: nacimiento, confianza: 0.7},
		{nombre: "fecha_us", re: must(`\d{2}/\d{2}/\d{4}`), forma: modelo.FormaFechaUS, contexto: nacimiento, confianza: 0.7},
		{nombre: "fecha_palabras", re: must(`\d{1,2} de ` + meses + ` de \d{4}`), forma: modelo.FormaFechaPalabras, contexto: nacimiento, confianza: 0.7},
		{nombre: "placa", re: must(`[a-z]{3}[\s-]?\d{3}`), forma: modelo.FormaPlaca, contexto: []string{"placa", "vehiculo", "carro", "moto"}, confianza: 0.7},
		{nombre: "pasaporte", re: must(`[a-z]{1,2}\d{6,8}`), forma: modelo.FormaPasaporte, contexto: []string{"pasaporte"}, confianza: 0.7},
	}
}
