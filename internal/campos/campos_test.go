package campos

import (
	"reflect"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// La lista tiene que ser estable y cubrir todos los strings del registro: si un
// campo queda fuera, la detección no lo ve y la compuerta lo tendría que tapar.
func TestListaCubreTodosLosCampos(t *testing.T) {
	casos := []struct {
		nombre   string
		registro modelo.Registro
		rutas    []modelo.CampoTexto
	}{
		{
			nombre: "chat con adjuntos",
			registro: modelo.ChatRecord{
				ID: "m1", Author: "Ana", Text: "hola",
				Attachments: []modelo.Adjunto{{Filename: "a.pdf"}, {Filename: "b.pdf"}},
			},
			rutas: []modelo.CampoTexto{"author", "text", "attachments.0.filename", "attachments.1.filename"},
		},
		{
			nombre:   "chat sin adjuntos",
			registro: modelo.ChatRecord{ID: "m2", Author: "Ana", Text: "hola"},
			rutas:    []modelo.CampoTexto{"author", "text"},
		},
		{
			nombre: "documento con metadata",
			registro: modelo.DocRecord{
				ID: "d1", Filename: "f.md", Title: "T", Body: "cuerpo",
				Metadata: map[string]string{"Author": "Ana", "Keywords": "x"},
			},
			rutas: []modelo.CampoTexto{"filename", "title", "body", "metadata.Author", "metadata.Keywords"},
		},
		{
			nombre: "tabla",
			registro: modelo.TableRecord{
				ID: "clientes:1", Table: "clientes",
				Columns: map[string]string{"nombre_completo": "Ana", "correo": "a@b.co"},
			},
			rutas: []modelo.CampoTexto{"columns.correo", "columns.nombre_completo"},
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			campos := Lista(c.registro)
			rutas := make([]modelo.CampoTexto, 0, len(campos))
			for _, campo := range campos {
				rutas = append(rutas, campo.Ruta)
			}
			if !reflect.DeepEqual(rutas, c.rutas) {
				t.Fatalf("rutas = %v, quiero %v", rutas, c.rutas)
			}
			otra := Lista(c.registro)
			if !reflect.DeepEqual(campos, otra) {
				t.Fatalf("la lista no es estable: %v y %v", campos, otra)
			}
		})
	}
}

// Aplica edita de derecha a izquierda y no toca el resto del texto.
func TestAplicaDerechaAIzquierda(t *testing.T) {
	chat := modelo.ChatRecord{ID: "m1", Author: "Ana Pérez", Text: "Ana y Ana"}
	nuevo, err := Aplica(chat, []Edicion{
		{Ruta: "text", Inicio: 0, Fin: 3, Nuevo: "Bea"},
		{Ruta: "text", Inicio: 6, Fin: 9, Nuevo: "Bea"},
		{Ruta: "author", Inicio: 4, Fin: 10, Nuevo: "Gómez"},
	})
	if err != nil {
		t.Fatalf("Aplica: %v", err)
	}
	out := nuevo.(modelo.ChatRecord)
	if out.Text != "Bea y Bea" || out.Author != "Ana Gómez" {
		t.Fatalf("chat = %+v", out)
	}
	if chat.Text != "Ana y Ana" || chat.Author != "Ana Pérez" {
		t.Fatalf("el original cambió: %+v", chat)
	}
}

// Un offset errado o solapado no puede pasar en silencio: la compuerta depende
// de que el reemplazo sea exacto.
func TestAplicaRechazaRangosInvalidos(t *testing.T) {
	chat := modelo.ChatRecord{ID: "m1", Text: "abc"}
	casos := []struct {
		nombre string
		eds    []Edicion
	}{
		{"fuera de rango", []Edicion{{Ruta: "text", Inicio: 0, Fin: 9, Nuevo: "x"}}},
		{"fin antes de inicio", []Edicion{{Ruta: "text", Inicio: 2, Fin: 1, Nuevo: "x"}}},
		{"solapadas", []Edicion{
			{Ruta: "text", Inicio: 0, Fin: 2, Nuevo: "x"},
			{Ruta: "text", Inicio: 1, Fin: 3, Nuevo: "y"},
		}},
		{"ruta inexistente", []Edicion{{Ruta: "metadata.Author", Inicio: 0, Fin: 1, Nuevo: "x"}}},
		{"adjunto inexistente", []Edicion{{Ruta: "attachments.3.filename", Inicio: 0, Fin: 1, Nuevo: "x"}}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, err := Aplica(chat, c.eds); err == nil {
				t.Fatalf("quería error con %+v", c.eds)
			}
		})
	}
}

// Los campos de una tabla conservan las demás columnas intactas.
func TestAplicaTablaConservaColumnas(t *testing.T) {
	tabla := modelo.TableRecord{
		ID: "clientes:CL-1", Table: "clientes",
		Columns: map[string]string{"nombre_completo": "Ana Pérez", "ciudad": "Cali"},
	}
	nuevo, err := Aplica(tabla, []Edicion{
		{Ruta: "columns.nombre_completo", Inicio: 0, Fin: 3, Nuevo: "Bea"},
	})
	if err != nil {
		t.Fatalf("Aplica: %v", err)
	}
	out := nuevo.(modelo.TableRecord)
	if out.Columns["nombre_completo"] != "Bea Pérez" || out.Columns["ciudad"] != "Cali" {
		t.Fatalf("columnas = %v", out.Columns)
	}
	if tabla.Columns["nombre_completo"] != "Ana Pérez" {
		t.Fatalf("el original cambió: %v", tabla.Columns)
	}
}

// Una edición a mitad de runa rompería el UTF-8 de la salida.
func TestAplicaRechazaMitadDeRuna(t *testing.T) {
	chat := modelo.ChatRecord{ID: "m1", Text: "Peña"}
	if _, err := Aplica(chat, []Edicion{{Ruta: "text", Inicio: 3, Fin: 4, Nuevo: "x"}}); err == nil {
		t.Fatal("quería error a mitad de runa")
	}
}
