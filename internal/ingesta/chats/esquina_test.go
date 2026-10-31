package chats

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Los casos de testdata/esquina son un export mínimo con los casos esquina de la
// ingesta: mensaje editado con adjunto, adjunto sin nombre, respuesta en hilo
// (mismo día y al día siguiente), mensaje sin client_msg_id, usuario ausente de
// users.json, usuario borrado sin real_name, mensajes de sistema y hilo huérfano.

func cargarEsquina(t *testing.T) *Corpus {
	t.Helper()
	c, err := Cargar(filepath.Join("testdata", "esquina"))
	if err != nil {
		t.Fatalf("Cargar(testdata/esquina): %v", err)
	}
	return c
}

func registroPorID(t *testing.T, corpus *Corpus, id string) modelo.ChatRecord {
	t.Helper()
	for _, r := range corpus.Registros {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no hay registro %s", id)
	return modelo.ChatRecord{}
}

func incidenciasDe(corpus *Corpus, regla Regla) []Incidencia {
	var out []Incidencia
	for _, i := range corpus.Incidencias {
		if i.Regla == regla {
			out = append(out, i)
		}
	}
	return out
}

func TestHiloUsaLaRaizDelCanal(t *testing.T) {
	corpus := cargarEsquina(t)

	raiz := registroPorID(t, corpus, "id-raiz")
	if raiz.ThreadID != "canal-uno:1000000001.000100" {
		t.Errorf("raíz: thread_id = %q", raiz.ThreadID)
	}
	respuesta := registroPorID(t, corpus, "id-respuesta")
	if respuesta.ThreadID != raiz.ThreadID {
		t.Errorf("respuesta: thread_id = %q, quiero %q", respuesta.ThreadID, raiz.ThreadID)
	}

	// La respuesta del 4 de agosto pertenece al hilo abierto el 3: la raíz se
	// busca en todo el canal, no en el día.
	cruzada := registroPorID(t, corpus, "id-respuesta-cruzada")
	if quiero := "canal-dos:1000000101.000100"; cruzada.ThreadID != quiero {
		t.Errorf("respuesta cruzada: thread_id = %q, quiero %q", cruzada.ThreadID, quiero)
	}
	if huerfanas := incidenciasDe(corpus, ReglaHiloHuerfano); len(huerfanas) != 1 {
		t.Errorf("hilos huérfanos = %v, quiero solo el de canal-uno", huerfanas)
	}
}

func TestHiloHuerfanoConservaElThreadID(t *testing.T) {
	corpus := cargarEsquina(t)

	huerfano := registroPorID(t, corpus, "id-huerfano")
	if quiero := "canal-uno:1000000000.000100"; huerfano.ThreadID != quiero {
		t.Errorf("thread_id = %q, quiero %q (el que declara la fuente)", huerfano.ThreadID, quiero)
	}
	huerfanas := incidenciasDe(corpus, ReglaHiloHuerfano)
	if len(huerfanas) != 1 || huerfanas[0].Canal != "canal-uno" || huerfanas[0].TS != "1000000010.009100" {
		t.Errorf("incidencias de hilo huérfano = %v", huerfanas)
	}
}

func TestAdjuntosDelMensajeEditado(t *testing.T) {
	corpus := cargarEsquina(t)

	editado := registroPorID(t, corpus, "id-editado")
	if len(editado.Attachments) != 1 {
		t.Fatalf("adjuntos = %v, quiero 1", editado.Attachments)
	}
	if editado.Attachments[0].Filename != "comprobante_ejemplo.pdf" {
		t.Errorf("filename = %q", editado.Attachments[0].Filename)
	}
}

func TestAdjuntoSinNombreUsaElTitulo(t *testing.T) {
	corpus := cargarEsquina(t)

	conAdjuntos := registroPorID(t, corpus, "id-adjuntos")
	quiero := []string{"foto_cedula", ""}
	if len(conAdjuntos.Attachments) != len(quiero) {
		t.Fatalf("adjuntos = %v, quiero %d", conAdjuntos.Attachments, len(quiero))
	}
	for i, nombre := range quiero {
		if tengo := conAdjuntos.Attachments[i].Filename; tengo != nombre {
			t.Errorf("adjunto %d: filename = %q, quiero %q", i, tengo, nombre)
		}
	}
	sinNombre := incidenciasDe(corpus, ReglaAdjuntoSinNombre)
	if len(sinNombre) != 1 || sinNombre[0].TS != "1000000004.003100" {
		t.Errorf("incidencias de adjunto sin nombre = %v", sinNombre)
	}
}

func TestMensajeSinClientMsgIDSEDescarta(t *testing.T) {
	corpus := cargarEsquina(t)

	for _, r := range corpus.Registros {
		if r.TS == "1000000005.004100" {
			t.Fatalf("el mensaje sin client_msg_id no puede ser registro: %+v", r)
		}
	}
	sinID := incidenciasDe(corpus, ReglaMensajeSinID)
	if len(sinID) != 1 || sinID[0].TS != "1000000005.004100" {
		t.Errorf("incidencias de mensaje sin id = %v", sinID)
	}
	// Nada más se pierde: 13 mensajes en la fuente, 12 con id, uno descartado.
	if len(corpus.Registros) != 12 {
		t.Errorf("registros = %d, quiero 12", len(corpus.Registros))
	}
}

func TestUsuarioAusenteUsaNombreDesconocido(t *testing.T) {
	corpus := cargarEsquina(t)

	ausente := registroPorID(t, corpus, "id-ausente")
	if ausente.Author != AutorDesconocido {
		t.Errorf("author = %q, quiero %q", ausente.Author, AutorDesconocido)
	}
	inc := incidenciasDe(corpus, ReglaUsuarioAusente)
	if len(inc) != 1 || inc[0].TS != "1000000006.005100" {
		t.Errorf("incidencias de usuario ausente = %v", inc)
	}
}

func TestUsuarioBorradoSinNombreUsaDesconocido(t *testing.T) {
	corpus := cargarEsquina(t)

	borrado := registroPorID(t, corpus, "id-usuario-borrado")
	if borrado.Author != AutorDesconocido {
		t.Errorf("author = %q, quiero %q", borrado.Author, AutorDesconocido)
	}
	// Está en el índice: no es un usuario ausente, es una persona sin nombre.
	if inc := incidenciasDe(corpus, ReglaUsuarioAusente); len(inc) != 1 {
		t.Errorf("incidencias de usuario ausente = %v", inc)
	}
	u := corpus.Usuarios["UTEST0003"]
	if !u.Borrado || u.Rol != modelo.RolDesconocido {
		t.Errorf("usuario borrado en el índice = %+v", u)
	}
}

func TestMensajesDeSistemaSeConservan(t *testing.T) {
	corpus := cargarEsquina(t)

	// Borrado por la herramienta: sin user ni text, y sigue siendo un registro.
	borrado := registroPorID(t, corpus, "id-borrado-herramienta")
	if borrado.Author != AutorDesconocido || borrado.Text != "" || len(borrado.Attachments) != 0 {
		t.Errorf("mensaje borrado = %+v", borrado)
	}
	if quiero := "canal-uno:1000000008.007100"; borrado.ThreadID != quiero {
		t.Errorf("thread_id = %q, quiero %q", borrado.ThreadID, quiero)
	}

	// De sistema con usuario: se conserva el autor y el texto tal cual.
	alta := registroPorID(t, corpus, "id-sistema-alta")
	if alta.Author != "Beto Agente" {
		t.Errorf("author = %q, quiero %q", alta.Author, "Beto Agente")
	}
	if alta.Text != "<@UTEST0002> se unió al canal." {
		t.Errorf("text = %q", alta.Text)
	}

	sistema := incidenciasDe(corpus, ReglaSubtypeSistema)
	if len(sistema) != 2 {
		t.Errorf("incidencias de subtype de sistema = %v, quiero 2", sistema)
	}
	// El bot no es un subtype de sistema: es un autor normal.
	for _, i := range sistema {
		if i.TS == "1000000001.000100" {
			t.Errorf("el mensaje del bot no es una incidencia: %v", i)
		}
	}
}

// TestIndiceDeUsuarios comprueba que el índice uid → usuario lleva lo que el
// registro de personas (C12) necesita: nombre corto, handle, correo, cargo y rol.
func TestIndiceDeUsuarios(t *testing.T) {
	corpus := cargarEsquina(t)

	if len(corpus.Usuarios) != 4 {
		t.Fatalf("usuarios = %d, quiero 4", len(corpus.Usuarios))
	}
	cliente := corpus.Usuarios["UTEST0001"]
	quiero := Usuario{
		UID:            "UTEST0001",
		Nombre:         "Ana Prueba",
		NombreCompleto: "Ana Prueba Apellido",
		Usuario:        "prueba.persona",
		Correo:         "ana.prueba@ejemplo.test",
		Cargo:          "Cliente",
		Rol:            modelo.RolCliente,
	}
	if cliente != quiero {
		t.Errorf("cliente =\n%+v\nquiero\n%+v", cliente, quiero)
	}

	if agente := corpus.Usuarios["UTEST0002"]; agente.Rol != modelo.RolEmpleado || agente.Nombre != "Beto Agente" {
		t.Errorf("agente = %+v", agente)
	}
	bot := corpus.Usuarios["UTESTBOT01"]
	if !bot.EsBot || bot.Nombre != "Kipi" || bot.Rol != modelo.RolDesconocido {
		t.Errorf("bot = %+v", bot)
	}
}

// TestCanales lee channels.json: el nombre del canal es el que va en thread_id y
// la metadata queda disponible para el resto del pipeline.
func TestCanales(t *testing.T) {
	corpus := cargarEsquina(t)

	canal, ok := corpus.Canales["canal-uno"]
	if !ok {
		t.Fatalf("falta canal-uno: %v", corpus.Canales)
	}
	if canal.ID != "CTEST00001" || canal.Nombre != "canal-uno" || len(canal.Miembros) != 2 || canal.Archivado {
		t.Errorf("canal-uno = %+v", canal)
	}
}

func TestCanalSinMetadatosSeIngiere(t *testing.T) {
	corpus := cargarEsquina(t)

	registroPorID(t, corpus, "id-canal-tres")
	inc := incidenciasDe(corpus, ReglaCanalSinMetadatos)
	if len(inc) != 1 || inc[0].Canal != "canal-tres" {
		t.Errorf("incidencias de canal sin metadatos = %v", inc)
	}
	if _, ok := corpus.Canales["canal-tres"]; ok {
		t.Error("canal-tres no está en channels.json")
	}
}

func TestCargarSinUsersFalla(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "channels.json"), []byte("[]"), 0o600); err != nil {
		t.Fatalf("escribe channels.json: %v", err)
	}
	if _, err := Cargar(dir); err == nil {
		t.Fatal("sin users.json Cargar tiene que fallar")
	}
}

func TestCargarConArchivoIlegibleFalla(t *testing.T) {
	dir := t.TempDir()
	escribe := func(ruta, contenido string) {
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatalf("crea %s: %v", filepath.Dir(ruta), err)
		}
		if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
			t.Fatalf("escribe %s: %v", ruta, err)
		}
	}
	escribe(filepath.Join(dir, "users.json"), "[]")
	escribe(filepath.Join(dir, "channels.json"), "[]")
	escribe(filepath.Join(dir, "canal-uno", "2026-08-03.json"), "{no es json}")
	if _, err := Cargar(dir); err == nil {
		t.Fatal("un día ilegible no se puede dar por ingerido")
	}
}
