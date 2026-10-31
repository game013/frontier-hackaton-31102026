package reemplazo

import (
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

func perfil() modelo.Perfil {
	return modelo.Perfil{
		Nombre: "Sofía Elena Zambrano", Correo: "szambrano@gmail.com",
		Celular: "3186423850", Cedula: "1107252808",
		Direccion: "Diagonal 121B # 96-28", Nacimiento: "1998-11-11",
	}
}

func aplica(t *testing.T, registro modelo.Registro, menciones []modelo.Mencion) modelo.Registro {
	t.Helper()
	r := Nuevo(Opciones{Perfiles: map[string]modelo.Perfil{"p_1": perfil()}})
	out, err := r.Aplica([]modelo.Registro{registro}, menciones)
	if err != nil {
		t.Fatalf("Aplica: %v", err)
	}
	return out[0]
}

// El reemplazo es literal y conserva el estilo de cada forma (D6).
func TestConservaEstiloYCaja(t *testing.T) {
	texto := "Hola Manuela Castaño, escribe a mcastano@gmail.com o al 3148729552"
	adjunto := "extracto_manuela_castano.pdf"
	chat := modelo.ChatRecord{
		ID: "m1", Author: "MANUELA CASTAÑO RUIZ",
		Text: texto, Attachments: []modelo.Adjunto{{Filename: adjunto}},
	}
	mencion := func(campo modelo.CampoTexto, valor string, forma modelo.TipoForma, en string) modelo.Mencion {
		inicio := strings.Index(en, valor)
		if inicio < 0 {
			t.Fatalf("el valor %q no está en %q", valor, en)
		}
		return modelo.Mencion{
			Registro: "m1", Campo: campo, Inicio: inicio, Fin: inicio + len(valor),
			Valor: valor, Forma: forma, Persona: "p_1",
		}
	}
	menciones := []modelo.Mencion{
		mencion("author", "MANUELA CASTAÑO RUIZ", modelo.FormaMayusculas, chat.Author),
		mencion("text", "Manuela Castaño", modelo.FormaNombreCorto, texto),
		mencion("text", "mcastano@gmail.com", modelo.FormaCorreo, texto),
		mencion("text", "3148729552", modelo.FormaCelular, texto),
		mencion("attachments.0.filename", "manuela_castano", modelo.FormaSlug, adjunto),
	}
	out := aplica(t, chat, menciones).(modelo.ChatRecord)
	if out.Author != "SOFIA ELENA ZAMBRANO" {
		t.Errorf("author = %q", out.Author)
	}
	quiero := "Hola Sofía Elena, escribe a szambrano@gmail.com o al 3186423850"
	if out.Text != quiero {
		t.Errorf("text = %q, quiero %q", out.Text, quiero)
	}
	if out.Attachments[0].Filename != "extracto_sofia_elena.pdf" {
		t.Errorf("adjunto = %q", out.Attachments[0].Filename)
	}
}

// El prefijo del documento ("CC ") se conserva.
func TestPrefijoDeDocumento(t *testing.T) {
	texto := "cédula CC 1.107.360.039"
	chat := modelo.ChatRecord{ID: "m1", Text: texto}
	valor := "CC 1.107.360.039"
	inicio := strings.Index(texto, valor)
	m := modelo.Mencion{
		Registro: "m1", Campo: "text", Inicio: inicio, Fin: inicio + len(valor),
		Valor: valor, Forma: modelo.FormaCedulaCC, Persona: "p_1",
	}
	out := aplica(t, chat, []modelo.Mencion{m}).(modelo.ChatRecord)
	if out.Text != "cédula CC 1107252808" {
		t.Fatalf("text = %q", out.Text)
	}
}

// Un perfil sin el dato no inventa: devuelve el fragmento y la compuerta decide.
func TestPerfilSinDatoConservaElFragmento(t *testing.T) {
	chat := modelo.ChatRecord{ID: "m1", Text: "placa ABC123"}
	m := modelo.Mencion{Registro: "m1", Campo: "text", Inicio: 6, Fin: 12, Valor: "ABC123", Forma: modelo.FormaPlaca, Persona: "p_1"}
	out := aplica(t, chat, []modelo.Mencion{m}).(modelo.ChatRecord)
	if out.Text != "placa ABC123" {
		t.Fatalf("text = %q", out.Text)
	}
}

// Una mención sin perfil en la bóveda es un error del pipeline, no un texto a
// medio reemplazar.
func TestSinPerfilFalla(t *testing.T) {
	r := Nuevo(Opciones{Perfiles: map[string]modelo.Perfil{}})
	_, err := r.Aplica([]modelo.Registro{modelo.ChatRecord{ID: "m1", Text: "Ana"}}, []modelo.Mencion{
		{Registro: "m1", Campo: "text", Inicio: 0, Fin: 3, Valor: "Ana", Forma: modelo.FormaNombrePila, Persona: "p_x"},
	})
	if err == nil {
		t.Fatal("quería error por perfil ausente")
	}
}

// Las menciones de un mismo campo se aplican de derecha a izquierda.
func TestVariasMencionesEnUnCampo(t *testing.T) {
	chat := modelo.ChatRecord{ID: "m1", Text: "Manuela y Manuela"}
	menciones := []modelo.Mencion{
		{Registro: "m1", Campo: "text", Inicio: 0, Fin: 7, Valor: "Manuela", Forma: modelo.FormaNombrePila, Persona: "p_1"},
		{Registro: "m1", Campo: "text", Inicio: 10, Fin: 17, Valor: "Manuela", Forma: modelo.FormaNombrePila, Persona: "p_1"},
	}
	out := aplica(t, chat, menciones).(modelo.ChatRecord)
	if out.Text != "Sofía y Sofía" {
		t.Fatalf("text = %q", out.Text)
	}
}

// El registro de entrada no se muta: el original sigue disponible para la
// compuerta y para el reporte.
func TestNoMutaElOriginal(t *testing.T) {
	chat := modelo.ChatRecord{ID: "m1", Text: "Manuela"}
	m := modelo.Mencion{Registro: "m1", Campo: "text", Inicio: 0, Fin: 7, Valor: "Manuela", Forma: modelo.FormaNombrePila, Persona: "p_1"}
	_ = aplica(t, chat, []modelo.Mencion{m})
	if chat.Text != "Manuela" {
		t.Fatalf("el original cambió: %q", chat.Text)
	}
}

// El índice no se usa para reemplazar (la persona ya está resuelta), pero el
// contrato lo admite.
func TestConIndice(t *testing.T) {
	indice, err := registro.Construye(nil, registro.Opciones{})
	if err != nil {
		t.Fatalf("registro: %v", err)
	}
	r := Nuevo(Opciones{Indice: indice, Perfiles: map[string]modelo.Perfil{"p_1": perfil()}})
	out, err := r.Aplica([]modelo.Registro{modelo.ChatRecord{ID: "m1", Text: "Ana"}}, []modelo.Mencion{
		{Registro: "m1", Campo: "text", Inicio: 0, Fin: 3, Valor: "Ana", Forma: modelo.FormaNombreCompleto, Persona: "p_1"},
	})
	if err != nil || out[0].(modelo.ChatRecord).Text != "Sofía Elena Zambrano" {
		t.Fatalf("out = %+v, err = %v", out, err)
	}
}
