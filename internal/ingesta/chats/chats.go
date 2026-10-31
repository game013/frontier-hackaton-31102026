// Package chats ingiere el export de chats de soporte de Kipu Pay (formato
// compatible con el de Slack) y lo traduce al esquema de chats.jsonl de output/:
// un modelo.ChatRecord por mensaje, con el mismo id (client_msg_id) y todos los
// mensajes que la fuente permite identificar.
//
// Disposición de la fuente: users.json y channels.json en la raíz, un
// directorio por canal (su nombre es el que va en thread_id) y un archivo
// <fecha>.json por día con los mensajes de ese día.
package chats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// AutorDesconocido es el author de un mensaje cuyo autor no se puede nombrar:
// usuario fuera de users.json sin username, mensaje borrado o de sistema. Nunca
// se usa el uid, y el mensaje no se descarta: la identidad real la resuelve el
// pipeline desde el texto o el maestro (C16).
const AutorDesconocido = "Desconocido"

// subtypeBot marca los mensajes del asistente virtual: son autores normales
// (Kipi se conserva como autor), no mensajes de sistema.
const subtypeBot = "bot_message"

// dominioEmpleados es el dominio de correo de la empresa: quien lo usa es empleado
// (analisis/02_chats.md §2).
const dominioEmpleados = "@kipupay.co"

// Usuario es una entrada del índice uid → usuario que consume el registro de
// personas (C12).
type Usuario struct {
	UID string
	// Nombre es la forma corta (nombre(s) + primer apellido): la que va en
	// author.
	Nombre string
	// NombreCompleto es el real_name de users.json tal cual; en usuarios
	// borrados viene vacío y la persona solo se puede identificar por el texto.
	NombreCompleto string
	// Usuario es el name de users.json: el handle, parte local del correo y
	// mejor llave usuario → fila del maestro.
	Usuario string
	Correo  string
	Cargo   string
	Rol     modelo.Rol
	EsBot   bool
	Borrado bool
}

// Canal es la metadata de channels.json.
type Canal struct {
	ID        string
	Nombre    string
	Miembros  []string
	Archivado bool
}

// Regla nombra cada resolución de excepción que aplica la ingesta.
type Regla string

const (
	// ReglaMensajeSinID: el mensaje no trae client_msg_id. Se descarta, porque el
	// id de registro sale de la fuente (docs/ENUNCIADO.md §ids) y un id
	// inventado sobraría frente a record_index.
	ReglaMensajeSinID Regla = "mensaje_sin_id"
	// ReglaSubtypeSistema: mensaje con subtype distinto de bot_message (de
	// sistema, borrado, editado por la herramienta). Se conserva tal cual, con
	// el texto y el autor que traiga.
	ReglaSubtypeSistema Regla = "subtype_sistema"
	// ReglaUsuarioAusente: el uid del mensaje no está en users.json. Se usa el
	// username o AutorDesconocido.
	ReglaUsuarioAusente Regla = "usuario_ausente"
	// ReglaHiloHuerfano: thread_ts sin mensaje raíz en el mismo canal. Se
	// conserva el thread_id declarado por la fuente.
	ReglaHiloHuerfano Regla = "hilo_huerfano"
	// ReglaAdjuntoSinNombre: el adjunto no trae name ni title. Se conserva el
	// adjunto con filename vacío: la lista mantiene el mismo número de adjuntos
	// que la entrada.
	ReglaAdjuntoSinNombre Regla = "adjunto_sin_nombre"
	// ReglaCanalSinMetadatos: hay directorio de canal sin entrada en
	// channels.json. Los mensajes se ingieren igual.
	ReglaCanalSinMetadatos Regla = "canal_sin_metadatos"
)

// Incidencia deja constancia de una excepción: dónde se aplicó qué regla. Nunca
// lleva texto ni nombres, solo canal y ts del mensaje.
type Incidencia struct {
	Regla Regla
	Canal string
	TS    string
}

// Corpus es el resultado de ingerir chats/.
type Corpus struct {
	// Registros va en orden de fuente: canales por nombre, días por fecha y
	// mensajes en el orden del archivo.
	Registros []modelo.ChatRecord
	// Usuarios es el índice uid → usuario de users.json, completo: incluye a
	// quienes nunca escriben.
	Usuarios map[string]Usuario
	// Canales es channels.json indexado por nombre de canal.
	Canales map[string]Canal
	// Autores va de id de registro (client_msg_id) a uid del autor. El registro
	// de personas y la resolución lo usan para no confundir homónimos cuando el
	// nombre del autor coincide con el de otra persona (D10, CE-02); el uid
	// nunca llega a output/.
	Autores map[string]string
	// Incidencias son las excepciones aplicadas, en orden de fuente.
	Incidencias []Incidencia
}

// Cargar ingiere el árbol de chats/ (raíz con users.json, channels.json y un
// directorio por canal). Falla si falta la metadata o si un archivo no es una
// lista de mensajes: una fuente ilegible no se puede dar por ingerida.
func Cargar(raiz string) (*Corpus, error) {
	usuarios, err := cargarUsuarios(filepath.Join(raiz, "users.json"))
	if err != nil {
		return nil, err
	}
	canales, err := cargarCanales(filepath.Join(raiz, "channels.json"))
	if err != nil {
		return nil, err
	}
	entradas, err := os.ReadDir(raiz)
	if err != nil {
		return nil, fmt.Errorf("chats: lee %s: %w", raiz, err)
	}

	c := &Corpus{
		Registros: []modelo.ChatRecord{},
		Usuarios:  usuarios,
		Canales:   canales,
		Autores:   map[string]string{},
	}
	for _, e := range entradas {
		if !e.IsDir() {
			continue
		}
		canal := e.Name()
		if _, ok := canales[canal]; !ok {
			c.Incidencias = append(c.Incidencias, Incidencia{Regla: ReglaCanalSinMetadatos, Canal: canal})
		}
		mensajes, err := leerCanal(filepath.Join(raiz, canal))
		if err != nil {
			return nil, err
		}
		c.agregaCanal(canal, mensajes)
	}
	return c, nil
}

// mensaje es un mensaje del export reducido a los campos que llegan a output/.
// Los adjuntos van en files (también en los mensajes editados: files es del
// mensaje, no de la edición) y no hay otras claves con datos personales.
type mensaje struct {
	User        string    `json:"user"`
	Text        string    `json:"text"`
	TS          string    `json:"ts"`
	ClientMsgID string    `json:"client_msg_id"`
	ThreadTS    string    `json:"thread_ts"`
	Subtype     string    `json:"subtype"`
	Username    string    `json:"username"`
	Files       []archivo `json:"files"`
}

// archivo es la metadata de un adjunto. url_private repite el nombre, así que no
// se lee.
type archivo struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

// leerCanal lee los días de un canal en orden de fecha.
func leerCanal(dir string) ([]mensaje, error) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("chats: lee %s: %w", dir, err)
	}
	var mensajes []mensaje
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		ruta := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(ruta)
		if err != nil {
			return nil, fmt.Errorf("chats: lee %s: %w", ruta, err)
		}
		var dia []mensaje
		if err := json.Unmarshal(b, &dia); err != nil {
			return nil, fmt.Errorf("chats: %s no es una lista de mensajes: %w", ruta, err)
		}
		mensajes = append(mensajes, dia...)
	}
	return mensajes, nil
}

// agregaCanal traduce los mensajes de un canal a registros. Va en dos pasadas
// porque el hilo se resuelve contra las raíces de todo el canal, no de un día.
func (c *Corpus) agregaCanal(canal string, mensajes []mensaje) {
	raices := make(map[string]bool, len(mensajes))
	for _, m := range mensajes {
		if m.ThreadTS == "" || m.ThreadTS == m.TS {
			raices[m.TS] = true
		}
	}

	for _, m := range mensajes {
		if m.ClientMsgID == "" {
			c.anota(ReglaMensajeSinID, canal, m.TS)
			continue
		}
		if m.Subtype != "" && m.Subtype != subtypeBot {
			c.anota(ReglaSubtypeSistema, canal, m.TS)
		}
		raiz := m.ThreadTS
		if raiz == "" {
			raiz = m.TS
		}
		if !raices[raiz] {
			c.anota(ReglaHiloHuerfano, canal, m.TS)
		}
		adjuntos, sinNombre := adjuntos(m.Files)
		if sinNombre {
			c.anota(ReglaAdjuntoSinNombre, canal, m.TS)
		}
		if m.User != "" {
			c.Autores[m.ClientMsgID] = m.User
		}
		c.Registros = append(c.Registros, modelo.ChatRecord{
			ID:          m.ClientMsgID,
			ThreadID:    canal + ":" + raiz,
			Author:      c.autor(canal, m),
			TS:          m.TS,
			Text:        m.Text,
			Attachments: adjuntos,
		})
	}
}

func (c *Corpus) anota(regla Regla, canal, ts string) {
	c.Incidencias = append(c.Incidencias, Incidencia{Regla: regla, Canal: canal, TS: ts})
}

// autor resuelve el nombre del autor: la forma corta del índice, el username del
// propio mensaje (bot) o AutorDesconocido. Nunca el uid.
func (c *Corpus) autor(canal string, m mensaje) string {
	usuario, enIndice := c.Usuarios[m.User]
	if m.User != "" && !enIndice {
		c.anota(ReglaUsuarioAusente, canal, m.TS)
	}
	switch {
	case usuario.Nombre != "":
		return usuario.Nombre
	case m.Username != "":
		return m.Username
	default:
		return AutorDesconocido
	}
}

// adjuntos traduce files a attachments. El nombre se conserva como campo y se
// anonimiza como texto después; si no hay name se usa el title, y si tampoco hay
// ninguno el adjunto queda igual en la lista con filename vacío.
func adjuntos(files []archivo) ([]modelo.Adjunto, bool) {
	lista := make([]modelo.Adjunto, 0, len(files))
	sinNombre := false
	for _, f := range files {
		nombre := f.Name
		if nombre == "" {
			nombre = f.Title
		}
		if nombre == "" {
			sinNombre = true
		}
		lista = append(lista, modelo.Adjunto{Filename: nombre})
	}
	return lista, sinNombre
}

// usuarioCrudo es una entrada de users.json. Las claves que no llegan al índice
// no se leen.
type usuarioCrudo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"real_name"`
	IsBot       bool   `json:"is_bot"`
	Restringido bool   `json:"is_restricted"`
	Deleted     bool   `json:"deleted"`
	Profile     struct {
		RealName string `json:"real_name"`
		Title    string `json:"title"`
		Email    string `json:"email"`
	} `json:"profile"`
}

func cargarUsuarios(ruta string) (map[string]Usuario, error) {
	var crudos []usuarioCrudo
	if err := leerJSON(ruta, &crudos); err != nil {
		return nil, err
	}
	usuarios := make(map[string]Usuario, len(crudos))
	for _, u := range crudos {
		if u.ID == "" {
			continue
		}
		completo := u.RealName
		if completo == "" {
			completo = u.Profile.RealName
		}
		usuarios[u.ID] = Usuario{
			UID:            u.ID,
			Nombre:         NombreCorto(completo),
			NombreCompleto: completo,
			Usuario:        u.Name,
			Correo:         u.Profile.Email,
			Cargo:          u.Profile.Title,
			Rol:            rolDe(u),
			EsBot:          u.IsBot,
			Borrado:        u.Deleted,
		}
	}
	return usuarios, nil
}

// rolDe saca el rol de la evidencia de users.json: el bot y los usuarios
// borrados no son personas, el cliente viene restringido y el empleado usa el
// dominio de la empresa.
func rolDe(u usuarioCrudo) modelo.Rol {
	switch {
	case u.IsBot, u.Deleted:
		return modelo.RolDesconocido
	case u.Restringido:
		return modelo.RolCliente
	case strings.HasSuffix(strings.ToLower(u.Profile.Email), dominioEmpleados):
		return modelo.RolEmpleado
	default:
		return modelo.RolDesconocido
	}
}

// canalCrudo es una entrada de channels.json.
type canalCrudo struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Members    []string `json:"members"`
	IsArchived bool     `json:"is_archived"`
}

func cargarCanales(ruta string) (map[string]Canal, error) {
	var crudos []canalCrudo
	if err := leerJSON(ruta, &crudos); err != nil {
		return nil, err
	}
	canales := make(map[string]Canal, len(crudos))
	for _, c := range crudos {
		if c.Name == "" {
			continue
		}
		canales[c.Name] = Canal{
			ID:        c.ID,
			Nombre:    c.Name,
			Miembros:  c.Members,
			Archivado: c.IsArchived,
		}
	}
	return canales, nil
}

func leerJSON(ruta string, destino any) error {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return fmt.Errorf("chats: lee %s: %w", ruta, err)
	}
	if err := json.Unmarshal(b, destino); err != nil {
		return fmt.Errorf("chats: %s no es JSON válido: %w", ruta, err)
	}
	return nil
}
