// Package registro construye el registro de personas del corpus: una entrada por
// persona con llave estable, género, rol y todas sus formas reales (D2, D18).
//
// La identidad sale de la evidencia, nunca del nombre solo: la fila del maestro
// (clientes.csv), la nómina (empleados.xml), el usuario de chat unido por la
// parte local del correo y los ids internos (`CL-`, `KP-`, `PQR-`, `TX-`). Dos
// personas con el mismo nombre son dos personas (homónimos, D10); los terceros
// sin fila reciben una llave `p_t_<hash>` derivada del nombre completo y su
// ancla, y se persisten (D22, D23).
//
// Las formas las renderiza internal/formas con el mismo código que la fábrica:
// así el reemplazo puede pedir el equivalente falso en el mismo estilo (C8).
package registro

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/catalogo"
	"github.com/game013/frontier-hackaton-31102026/internal/formas"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/chats"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
)

// dominioEmpleados marca la evidencia de empleado en users.json (D3, C6).
const dominioEmpleados = "@kipupay.co"

// Opciones son los índices de chats que no viajan en los registros canónicos:
// users.json (uid → usuario) y el mapa registro → uid que produce la ingesta.
// En /anonymize llegan vacíos: el lote trae `author` ya como nombre (D29).
type Opciones struct {
	Usuarios map[string]chats.Usuario
	Autores  map[string]string
	Canales  map[string]chats.Canal
}

// Ficha son los datos abstractos de una persona que el LLM puede ver para
// desempatar (D26): ids internos, ciudad y referencias, nunca el nombre.
type Ficha struct {
	Llave  string
	Rol    modelo.Rol
	Genero modelo.Genero
	Ciudad string
	Ref    []string
}

// Indice es el registro de personas con sus índices de búsqueda exacta.
type Indice struct {
	orden    []string
	personas map[string]*modelo.Persona
	fichas   map[string]*Ficha
	porForma map[string][]string
	porRef   map[string]string
	porUID   map[string]string
	autores  map[string]string
}

// Construye arma el registro a partir de los registros canónicos del corpus y
// de los índices de chats. El orden de las fuentes importa: primero el maestro,
// después los usuarios y al final el texto de los chats, que solo aporta formas
// y no crea personas nuevas (D23).
func Construye(recs []modelo.Registro, o Opciones) (*Indice, error) {
	i := nuevo()
	for _, r := range recs {
		tabla, ok := r.(modelo.TableRecord)
		if !ok {
			continue
		}
		if err := i.tabla(tabla); err != nil {
			return nil, err
		}
	}
	for _, r := range recs {
		doc, ok := r.(modelo.DocRecord)
		if !ok {
			continue
		}
		i.documento(doc)
	}
	i.usuarios(o.Usuarios)
	for _, r := range recs {
		chat, ok := r.(modelo.ChatRecord)
		if !ok {
			continue
		}
		i.chat(chat, o.Autores)
	}
	return i, nil
}

func nuevo() *Indice {
	return &Indice{
		personas: map[string]*modelo.Persona{},
		fichas:   map[string]*Ficha{},
		porForma: map[string][]string{},
		porRef:   map[string]string{},
		porUID:   map[string]string{},
		autores:  map[string]string{},
	}
}

// Personas devuelve una copia de las personas en el orden de sus llaves.
func (i *Indice) Personas() []modelo.Persona {
	out := make([]modelo.Persona, 0, len(i.orden))
	for _, llave := range i.orden {
		out = append(out, *i.personas[llave])
	}
	return out
}

// Persona devuelve una persona por llave.
func (i *Indice) Persona(llave string) (modelo.Persona, bool) {
	p, ok := i.personas[llave]
	if !ok {
		return modelo.Persona{}, false
	}
	return *p, true
}

// Busca devuelve las personas que declaran esa forma, comparando en el espacio
// normalizado del jurado (sin tildes, sin mayúsculas, dígitos sin separadores).
func (i *Indice) Busca(valor string) []modelo.Persona {
	llaves := i.porForma[norm.Norm(valor)]
	out := make([]modelo.Persona, 0, len(llaves))
	for _, llave := range llaves {
		if p, ok := i.personas[llave]; ok {
			out = append(out, *p)
		}
	}
	return out
}

// Formas devuelve todas las formas reales sin repetir, en orden estable: las
// del registro y las que se descubran después. La compuerta y la máscara del
// LLM las recorren enteras (D15, D25).
func (i *Indice) Formas() []modelo.Forma {
	vistas := map[string]bool{}
	var out []modelo.Forma
	for _, llave := range i.orden {
		for _, f := range i.personas[llave].Formas {
			clave := string(f.Tipo) + "\x00" + f.Valor
			if vistas[clave] {
				continue
			}
			vistas[clave] = true
			out = append(out, f)
		}
	}
	return out
}

// AgregaPersona inserta una persona que aún no existe y devuelve false si ya
// estaba (la llave es la identidad: no se pisa). Rol y género quedan fijados en
// la primera escritura, igual que en la bóveda.
func (i *Indice) AgregaPersona(p modelo.Persona) bool {
	if p.Llave == "" {
		return false
	}
	if _, existe := i.personas[p.Llave]; existe {
		for _, f := range p.Formas {
			i.AgregaForma(p.Llave, f)
		}
		return false
	}
	copia := p
	i.personas[p.Llave] = &copia
	i.orden = append(i.orden, p.Llave)
	i.fichas[p.Llave] = &Ficha{Llave: p.Llave, Rol: p.Rol, Genero: p.Genero}
	for _, f := range p.Formas {
		i.AgregaForma(p.Llave, f)
	}
	return true
}

// AgregaForma indexa una forma nueva de una persona existente. Es idempotente.
func (i *Indice) AgregaForma(llave string, f modelo.Forma) {
	p, ok := i.personas[llave]
	if !ok || f.Valor == "" || f.Tipo == "" {
		return
	}
	for _, existente := range p.Formas {
		if existente.Tipo == f.Tipo && existente.Valor == f.Valor {
			return
		}
	}
	p.Formas = append(p.Formas, f)
	clave := norm.Norm(f.Valor)
	for _, otra := range i.porForma[clave] {
		if otra == llave {
			return
		}
	}
	i.porForma[clave] = append(i.porForma[clave], llave)
}

// Fusiona suma otro registro a este sin pisar lo existente: las personas y
// formas que ya están mandan (la bóveda es la verdad cuando crece el corpus,
// D4, D22).
func (i *Indice) Fusiona(otro *Indice) {
	if otro == nil {
		return
	}
	for _, llave := range otro.orden {
		p := otro.personas[llave]
		if _, existe := i.personas[llave]; !existe {
			i.AgregaPersona(*p)
		} else {
			for _, f := range p.Formas {
				i.AgregaForma(llave, f)
			}
			i.heredaFicha(llave, otro.fichas[llave])
		}
	}
	for uid, llave := range otro.porUID {
		if _, existe := i.porUID[uid]; !existe {
			i.porUID[uid] = llave
		}
	}
	for registro, llave := range otro.autores {
		if _, existe := i.autores[registro]; !existe {
			i.autores[registro] = llave
		}
	}
}

// Ficha devuelve los datos abstractos de una persona para el desempate del LLM.
func (i *Indice) Ficha(llave string) (Ficha, bool) {
	f, ok := i.fichas[llave]
	if !ok {
		return Ficha{}, false
	}
	copia := *f
	copia.Ref = append([]string{}, f.Ref...)
	return copia, true
}

// PorUID devuelve la persona dueña de un usuario de chat.
func (i *Indice) PorUID(uid string) (modelo.Persona, bool) {
	llave, ok := i.porUID[uid]
	if !ok {
		return modelo.Persona{}, false
	}
	return i.Persona(llave)
}

// AutorDe devuelve la persona que escribió un registro de chat, cuando el uid
// del mensaje la identifica (D10, CE-02).
func (i *Indice) AutorDe(registro string) (modelo.Persona, bool) {
	llave, ok := i.autores[registro]
	if !ok {
		return modelo.Persona{}, false
	}
	return i.Persona(llave)
}

// PorReferencia resuelve un identificador duro a su persona: cédula, correo,
// celular, `CL-`, `PQR-`, `TX-`, placa, uid o usuario de chat.
func (i *Indice) PorReferencia(valor string) (modelo.Persona, bool) {
	llave, ok := i.porRef[norm.Norm(valor)]
	if !ok {
		return modelo.Persona{}, false
	}
	return i.Persona(llave)
}

// tabla indexa una fila del maestro o de la nómina y, en transacciones y
// quejas, solo agrega referencias a personas ya conocidas.
func (i *Indice) tabla(r modelo.TableRecord) error {
	switch r.Table {
	case "clientes":
		return i.cliente(r)
	case "empleados":
		return i.empleado(r)
	case "transacciones":
		i.transaccion(r)
	case "pqrs":
		i.pqr(r)
	}
	return nil
}

func (i *Indice) cliente(r modelo.TableRecord) error {
	id := col(r, "cliente_id")
	if id == "" {
		return fmt.Errorf("registro: fila de clientes sin cliente_id")
	}
	llave := llaveCliente(id)
	perfil := modelo.Perfil{
		Nombre:     col(r, "nombre_completo"),
		Correo:     correoLimpio(col(r, "correo")),
		Celular:    celularLimpio(col(r, "celular")),
		Cedula:     digitos(col(r, "num_doc")),
		Direccion:  strings.TrimSpace(col(r, "direccion")),
		Nacimiento: fechaISO(col(r, "fecha_nacimiento")),
	}
	perfil.Handle = local(perfil.Correo)
	if perfil.Nombre == "" {
		return fmt.Errorf("registro: cliente %s sin nombre", id)
	}
	i.agregaPersona(llave, modelo.RolCliente, perfil, col(r, "ciudad"), col(r, "ocupacion"), []string{
		id, r.ID, perfil.Cedula, perfil.Correo, perfil.Celular,
	})
	return nil
}

func (i *Indice) empleado(r modelo.TableRecord) error {
	id := col(r, "id")
	if id == "" {
		return fmt.Errorf("registro: fila de empleados sin id")
	}
	llave := llaveEmpleado(id)
	perfil := modelo.Perfil{
		Nombre:     col(r, "nombre"),
		Correo:     correoLimpio(col(r, "correo")),
		Celular:    celularLimpio(col(r, "celular")),
		Cedula:     digitos(col(r, "documento")),
		Nacimiento: fechaISO(col(r, "fecha_nacimiento")),
	}
	perfil.Handle = local(perfil.Correo)
	if perfil.Nombre == "" {
		return fmt.Errorf("registro: empleado %s sin nombre", id)
	}
	i.agregaPersona(llave, modelo.RolEmpleado, perfil, col(r, "ciudad"), col(r, "cargo"), []string{
		id, r.ID, perfil.Cedula, perfil.Correo, perfil.Celular, perfil.Handle,
	})
	// reporta_a es el nombre corto del jefe (CE-25): no es una forma de la
	// persona, es una referencia que la resolución usa.
	return nil
}

// transaccion no crea personas: une el id de la transacción con su cliente y
// deja la fecha y el monto como evidencia para desempatar homónimos (CE-01).
func (i *Indice) transaccion(r modelo.TableRecord) {
	cliente := col(r, "cliente_id")
	if cliente == "" {
		return
	}
	if p, ok := i.PorReferencia(cliente); ok {
		i.porRef[norm.Norm(col(r, "tx_id"))] = p.Llave
	} else {
		i.porRef[norm.Norm(col(r, "tx_id"))] = llaveCliente(cliente)
	}
}

// pqr liga el radicado con su cliente y su agente, y crea la persona cuando la
// queja viene de alguien fuera del maestro (CL-55481, CE-18).
func (i *Indice) pqr(r modelo.TableRecord) {
	radicado := col(r, "radicado")
	cliente := col(r, "cliente_id")
	cedula := digitos(col(r, "cedula"))
	llave := ""
	if cliente != "" {
		llave = llaveCliente(cliente)
		if _, existe := i.personas[llave]; !existe && cedula != "" {
			perfil := modelo.Perfil{Cedula: cedula}
			i.agregaPersona(llave, modelo.RolCliente, perfil, col(r, "ciudad"), "", []string{cliente, cedula})
		}
	}
	if llave != "" {
		i.porRef[norm.Norm(radicado)] = llave
	}
	if agente := col(r, "agente"); agente != "" {
		if p, ok := i.PorReferencia(agente); ok {
			if llave == "" {
				i.porRef[norm.Norm(radicado)] = p.Llave
			}
		}
	}
	if cedula != "" {
		if p, ok := i.PorReferencia(cedula); ok && radicado != "" {
			i.porRef[norm.Norm(radicado)] = p.Llave
		}
	}
}

// documento suma las personas que solo aparecen en un documento: el autor de la
// metadata (Author de un PDF, author del front matter) y los correos del
// cuerpo. De la parte local del correo sale el nombre que el texto usa después
// ("kelly.castano" → "Kelly Castaño"), y así el nombre deja de ser un
// desconocido (CE-30, CE-31).
func (i *Indice) documento(r modelo.DocRecord) {
	claves := make([]string, 0, len(r.Metadata))
	for clave := range r.Metadata {
		claves = append(claves, clave)
	}
	sort.Strings(claves)
	for _, clave := range claves {
		valor := strings.TrimSpace(r.Metadata[clave])
		if valor == "" {
			continue
		}
		if strings.Contains(valor, "@") {
			i.correoDeDocumento(valor)
			continue
		}
		if esNombre(valor) && esClaveDeAutor(clave) {
			i.nombreDeDocumento(valor)
		}
	}
	for _, correo := range correosEn(r.Body) {
		i.correoDeDocumento(correo)
	}
}

// correoDeDocumento une el correo con su persona o crea una: los del dominio de
// la empresa son empleados (D3), el resto, terceros.
func (i *Indice) correoDeDocumento(correo string) {
	correo = correoLimpio(correo)
	if correo == "" {
		return
	}
	if p, ok := i.PorReferencia(correo); ok {
		i.AgregaForma(p.Llave, modelo.Forma{Tipo: modelo.FormaCorreo, Valor: correo})
		return
	}
	llave := llaveDesdeCorreo(correo)
	if _, existe := i.personas[llave]; !existe {
		rol := modelo.RolDesconocido
		if strings.HasSuffix(strings.ToLower(correo), dominioEmpleados) {
			rol = modelo.RolEmpleado
		}
		perfil := modelo.Perfil{Nombre: nombreDesdeCorreo(correo), Correo: correo, Handle: local(correo)}
		i.agregaPersona(llave, rol, perfil, "", "", []string{correo})
	}
	i.AgregaForma(llave, modelo.Forma{Tipo: modelo.FormaCorreo, Valor: correo})
	i.AgregaForma(llave, modelo.Forma{Tipo: modelo.FormaUsuario, Valor: local(correo)})
}

// nombreDeDocumento agrega el nombre que declara un autor de documento cuando ya
// hay una persona única con ese nombre; si no, no inventa una persona por un
// nombre suelto.
func (i *Indice) nombreDeDocumento(nombre string) {
	if candidatos := i.Busca(nombre); len(candidatos) == 1 {
		i.AgregaForma(candidatos[0].Llave, modelo.Forma{Tipo: modelo.FormaNombreCompleto, Valor: nombre})
	}
}

// usuarios une users.json con el maestro por la parte local del correo (D3) y
// crea personas de chat que no están en ninguna tabla (terceros y clientes
// fuera del maestro, D22).
func (i *Indice) usuarios(usuarios map[string]chats.Usuario) {
	uids := make([]string, 0, len(usuarios))
	for uid := range usuarios {
		uids = append(uids, uid)
	}
	sort.Strings(uids)
	for _, uid := range uids {
		u := usuarios[uid]
		if u.EsBot {
			continue
		}
		llave, ok := i.llaveDeUsuario(u)
		if !ok {
			if u.NombreCompleto == "" || u.Borrado {
				continue
			}
			llave = i.agregaTercero(u.NombreCompleto, uid)
		}
		i.porUID[uid] = llave
		i.porRef[norm.Norm(uid)] = llave
		if u.Usuario != "" {
			i.porRef[norm.Norm(u.Usuario)] = llave
			i.AgregaForma(llave, modelo.Forma{Tipo: modelo.FormaUsuario, Valor: u.Usuario})
		}
		if u.Correo != "" {
			i.AgregaForma(llave, modelo.Forma{Tipo: modelo.FormaCorreo, Valor: correoLimpio(u.Correo)})
		}
		if u.NombreCompleto != "" {
			i.AgregaForma(llave, modelo.Forma{Tipo: modelo.FormaNombreCompleto, Valor: u.NombreCompleto})
		}
		if u.Nombre != "" {
			i.AgregaForma(llave, modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: u.Nombre})
		}
	}
}

// llaveDeUsuario busca la persona de un usuario por correo, usuario o nombre
// completo único; no une por nombre cuando hay dos candidatos (D10).
func (i *Indice) llaveDeUsuario(u chats.Usuario) (string, bool) {
	for _, valor := range []string{u.Correo, u.Usuario, u.NombreCompleto, u.Nombre} {
		if valor == "" {
			continue
		}
		if p, ok := i.PorReferencia(valor); ok {
			return p.Llave, true
		}
		if candidatos := i.Busca(valor); len(candidatos) == 1 {
			return candidatos[0].Llave, true
		}
	}
	return "", false
}

// chat usa el uid del mensaje para atar al autor con su persona y, si el uid no
// está en users.json, con el nombre único del registro (CE-02).
func (i *Indice) chat(r modelo.ChatRecord, autores map[string]string) {
	uid := autores[r.ID]
	var llave string
	if uid != "" {
		if l, ok := i.porUID[uid]; ok {
			llave = l
		}
	}
	if llave == "" && r.Author != "" && r.Author != chats.AutorDesconocido {
		if candidatos := i.Busca(r.Author); len(candidatos) == 1 {
			llave = candidatos[0].Llave
			if uid != "" {
				i.porUID[uid] = llave
			}
		}
	}
	if llave == "" {
		return
	}
	i.autores[r.ID] = llave
	if r.Author != "" && r.Author != chats.AutorDesconocido {
		i.AgregaForma(llave, modelo.Forma{Tipo: modelo.FormaNombreCorto, Valor: r.Author, Registro: r.ID})
	}
}

// agregaTercero crea la persona de un nombre que no está en el maestro, con
// llave `p_t_<hash>` del nombre normalizado y su ancla (D22). Si ya existe por
// el mismo nombre, la reutiliza: un tercero que reaparece es la misma persona
// mientras no haya evidencia dura en contra (D23).
func (i *Indice) agregaTercero(nombreCompleto, ancla string) string {
	if candidatos := i.Busca(nombreCompleto); len(candidatos) == 1 {
		return candidatos[0].Llave
	}
	llave := llaveTercero(nombreCompleto, ancla)
	perfil := modelo.Perfil{Nombre: nombreCompleto}
	i.agregaPersona(llave, modelo.RolDesconocido, perfil, "", "", []string{ancla})
	return llave
}

// agregaPersona arma las formas con internal/formas y fija la ficha.
func (i *Indice) agregaPersona(llave string, rol modelo.Rol, perfil modelo.Perfil, ciudad, pistaGenero string, refs []string) {
	i.AgregaPersona(modelo.Persona{Llave: llave, Rol: rol, Genero: generoDe(perfil.Nombre, pistaGenero)})
	if ficha, ok := i.fichas[llave]; ok {
		if ficha.Ciudad == "" {
			ficha.Ciudad = strings.TrimSpace(ciudad)
		}
		ficha.Ref = appendUnicos(ficha.Ref, refs...)
	}
	formasReales, err := formas.Todas(perfil)
	if err != nil {
		// Un perfil con un dato imposible (fecha inválida, celular corto) no
		// puede tumbar el registro: se agrega con las formas que sí renderizan.
		formasReales = nil
	}
	for _, f := range formasReales {
		i.AgregaForma(llave, f)
	}
	for _, f := range formasExtra(perfil) {
		i.AgregaForma(llave, f)
	}
	for _, ref := range refs {
		if ref != "" {
			i.porRef[norm.Norm(ref)] = llave
		}
	}
}

// formasExtra agrega los estilos que internal/formas no cubre y que el corpus
// usa: la cédula con prefijo y espacio, y el usuario del correo como handle.
func formasExtra(perfil modelo.Perfil) []modelo.Forma {
	var out []modelo.Forma
	if d := digitos(perfil.Cedula); d != "" {
		out = append(out,
			modelo.Forma{Tipo: modelo.FormaCedulaCC, Valor: "CC " + d},
			modelo.Forma{Tipo: modelo.FormaCedulaCC, Valor: "CC " + agrupa(d)},
			modelo.Forma{Tipo: modelo.FormaCedulaCC, Valor: "C.C. " + agrupa(d)},
		)
	}
	if h := local(perfil.Correo); h != "" {
		out = append(out, modelo.Forma{Tipo: modelo.FormaUsuario, Valor: h})
	}
	return out
}

func (i *Indice) heredaFicha(llave string, otra *Ficha) {
	if otra == nil {
		return
	}
	ficha, ok := i.fichas[llave]
	if !ok {
		copia := *otra
		i.fichas[llave] = &copia
		return
	}
	if ficha.Ciudad == "" {
		ficha.Ciudad = otra.Ciudad
	}
	ficha.Ref = appendUnicos(ficha.Ref, otra.Ref...)
}

// llaveDesdeCorreo da la llave de una persona que solo consta por correo: los
// empleados cuelgan de p_kp_, el resto de p_t_ (D22).
func llaveDesdeCorreo(correo string) string {
	suma := sha256.Sum256([]byte(norm.Norm(correo)))
	digesto := hex.EncodeToString(suma[:6])
	if strings.HasSuffix(strings.ToLower(correo), dominioEmpleados) {
		return "p_kp_" + digesto
	}
	return "p_t_" + digesto
}

// nombreDesdeCorreo arma el nombre que el corpus escribe en el texto a partir de
// la parte local: "kelly.castano" → "Kelly Castaño".
func nombreDesdeCorreo(correo string) string {
	parte := local(correo)
	if parte == "" {
		return ""
	}
	tokens := strings.FieldsFunc(parte, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == '+'
	})
	nombre := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		runas := []rune(strings.ToLower(token))
		runas[0] = []rune(strings.ToUpper(string(runas[0])))[0]
		nombre = append(nombre, string(runas))
	}
	if len(nombre) > 2 {
		nombre = nombre[:2]
	}
	return strings.Join(nombre, " ")
}

// esClaveDeAutor marca las claves de metadata donde el corpus declara quién
// escribió el documento.
func esClaveDeAutor(clave string) bool {
	switch strings.ToLower(clave) {
	case "author", "autor", "creator", "prepared_by", "prepared by", "owner", "from":
		return true
	}
	return false
}

// esNombre descarta fechas, números y frases largas de la metadata.
func esNombre(valor string) bool {
	campos := strings.Fields(valor)
	if len(campos) < 2 || len(campos) > 5 {
		return false
	}
	for _, campo := range campos {
		for _, r := range campo {
			if r >= '0' && r <= '9' {
				return false
			}
		}
	}
	return true
}

// correosEn extrae las direcciones de correo del cuerpo de un documento.
func correosEn(texto string) []string {
	var out []string
	for _, campo := range strings.FieldsFunc(texto, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '.' || r == '_' || r == '%' || r == '+' || r == '-' || r == '@')
	}) {
		if strings.Contains(campo, "@") {
			out = append(out, campo)
		}
	}
	return out
}

// llaveCliente convierte CL-19127 en p_cl_19127.
func llaveCliente(id string) string {
	num := strings.TrimSpace(id)
	num = strings.TrimPrefix(strings.ToUpper(num), "CL-")
	return "p_cl_" + strings.ToLower(num)
}

// llaveEmpleado convierte KP-001 en p_kp_001.
func llaveEmpleado(id string) string {
	num := strings.TrimSpace(id)
	num = strings.TrimPrefix(strings.ToUpper(num), "KP-")
	return "p_kp_" + strings.ToLower(num)
}

// llaveTercero deriva p_t_<hash> del nombre normalizado y su ancla (D22).
func llaveTercero(nombre, ancla string) string {
	suma := sha256.Sum256([]byte(norm.Norm(nombre) + "|" + norm.Norm(ancla)))
	return "p_t_" + hex.EncodeToString(suma[:8])
}

// generoDe busca el nombre de pila en el catálogo y, si el catálogo no lo
// conoce (no puede tener tokens de nombres reales: C9), cae a la ocupación
// flexionada del maestro (D18): "fotógrafa" es F, "mecánico" es M.
func generoDe(nombreCompleto, ocupacion string) modelo.Genero {
	pila := primeraPalabra(nombreCompleto)
	if pila == "" {
		return generoDeOcupacion(ocupacion)
	}
	objetivo := norm.Norm(pila)
	grupos := []struct {
		genero modelo.Genero
		origen catalogo.Origen
	}{
		{modelo.GeneroM, catalogo.OrigenColombiano},
		{modelo.GeneroF, catalogo.OrigenColombiano},
		{modelo.GeneroUnisex, catalogo.OrigenColombiano},
		{modelo.GeneroM, catalogo.OrigenExtranjero},
		{modelo.GeneroF, catalogo.OrigenExtranjero},
		{modelo.GeneroUnisex, catalogo.OrigenExtranjero},
	}
	for _, g := range grupos {
		for _, conocido := range catalogo.NombresPila(g.genero, g.origen) {
			if norm.Norm(conocido) == objetivo {
				return g.genero
			}
		}
	}
	if g := generoDeOcupacion(ocupacion); g != modelo.GeneroIndeterminado {
		return g
	}
	return modelo.GeneroIndeterminado
}

// generoDeOcupacion usa la flexión del español como respaldo barato: solo
// decide cuando la terminación es inequívoca y deja el resto al modelo (D27).
func generoDeOcupacion(ocupacion string) modelo.Genero {
	campos := strings.Fields(strings.ToLower(strings.TrimSpace(ocupacion)))
	if len(campos) == 0 {
		return modelo.GeneroIndeterminado
	}
	ultima := campos[len(campos)-1]
	neutras := []string{"ista", "ente", "ante", "ense", "tra", "manager", "medios",
		"servicios", "ventas", "logística", "logistica", "de", "del", "la", "el"}
	for _, neutra := range neutras {
		if strings.HasSuffix(ultima, neutra) || ultima == neutra {
			return modelo.GeneroIndeterminado
		}
	}
	switch {
	case strings.HasSuffix(ultima, "a"), strings.HasSuffix(ultima, "ora"), strings.HasSuffix(ultima, "isa"):
		return modelo.GeneroF
	case strings.HasSuffix(ultima, "o"), strings.HasSuffix(ultima, "or"),
		strings.HasSuffix(ultima, "ón"), strings.HasSuffix(ultima, "an"):
		return modelo.GeneroM
	}
	return modelo.GeneroIndeterminado
}

func primeraPalabra(nombre string) string {
	campos := strings.Fields(nombre)
	if len(campos) == 0 {
		return ""
	}
	return campos[0]
}

func col(r modelo.TableRecord, clave string) string {
	return strings.TrimSpace(r.Columns[clave])
}

// correoLimpio descarta lo que no es un correo: la salida exige correos válidos
// y una forma basura solo generaría falsos positivos.
func correoLimpio(valor string) string {
	v := strings.TrimSpace(valor)
	arroba := strings.LastIndex(v, "@")
	if arroba <= 0 || arroba == len(v)-1 || !strings.Contains(v[arroba+1:], ".") {
		return ""
	}
	return v
}

func local(correo string) string {
	if arroba := strings.LastIndex(correo, "@"); arroba > 0 {
		return correo[:arroba]
	}
	return ""
}

// celularLimpio deja los diez dígitos del celular; si no los tiene, lo omite
// para no inventar formatos que el corpus no usa.
func celularLimpio(valor string) string {
	d := digitos(valor)
	if len(d) == 12 && strings.HasPrefix(d, "57") {
		d = d[2:]
	}
	if len(d) != 10 {
		return ""
	}
	return d
}

func digitos(valor string) string {
	var b strings.Builder
	for _, r := range valor {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// fechaISO acepta la fecha del maestro en ISO y, si viene en otro formato,
// intenta los dos que el corpus usa; si no, la deja vacía.
func fechaISO(valor string) string {
	v := strings.TrimSpace(valor)
	if len(v) == 10 && v[4] == '-' && v[7] == '-' {
		return v
	}
	// El corpus trae las fechas sueltas en formato US (analisis/00_resumen.md):
	// 01/08/1993 es el 8 de enero, no el 1 de agosto.
	if len(v) == 10 && v[2] == '/' && v[5] == '/' {
		return v[6:10] + "-" + v[0:2] + "-" + v[3:5]
	}
	return ""
}

func agrupa(d string) string {
	var grupos []string
	for len(d) > 3 {
		grupos = append([]string{d[len(d)-3:]}, grupos...)
		d = d[:len(d)-3]
	}
	return strings.Join(append([]string{d}, grupos...), ".")
}

func appendUnicos(base []string, nuevos ...string) []string {
	vistos := make(map[string]bool, len(base))
	for _, v := range base {
		vistos[v] = true
	}
	for _, v := range nuevos {
		if v == "" || vistos[v] {
			continue
		}
		vistos[v] = true
		base = append(base, v)
	}
	return base
}
