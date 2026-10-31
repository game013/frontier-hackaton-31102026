package formas

import (
	"errors"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// perfilRealMuestra es la identidad real de p_057 armada desde
// sample/tests_sample.json (forms) y el maestro de clientes: el registro
// construye un perfil así y pide sus formas.
func perfilRealMuestra() modelo.Perfil {
	return modelo.Perfil{
		Nombre:     "Nicolás Bermúdez Cárdenas",
		Correo:     "nbermudez@ejemplo.test",
		Celular:    "3001234567",
		Cedula:     "98765432",
		Handle:     "nbermudez",
		Direccion:  "Calle 99 # 1-2 apto 3",
		Nacimiento: "1990-02-03",
		Placa:      "ABC123",
		Pasaporte:  "XZ000000",
	}
}

func TestRenderPorTipo(t *testing.T) {
	casos := []struct {
		tipo   modelo.TipoForma
		quiero string
	}{
		{modelo.FormaNombreCompleto, "Nicolás Bermúdez Cárdenas"},
		{modelo.FormaNombreCorto, "Nicolás Bermúdez"},
		{modelo.FormaNombrePila, "Nicolás"},
		{modelo.FormaMayusculas, "NICOLAS BERMUDEZ CARDENAS"},
		{modelo.FormaSlug, "nicolas_bermudez"},
		{modelo.FormaUsuario, "nbermudez"},
		{modelo.FormaCorreo, "nbermudez@ejemplo.test"},
		{modelo.FormaCorreoDeletreado, "nbermudez arroba ejemplo punto test"},
		{modelo.FormaCedula, "98765432"},
		{modelo.FormaCedulaPuntos, "98.765.432"},
		{modelo.FormaCedulaCC, "CC98765432"},
		{modelo.FormaCelular, "3001234567"},
		{modelo.FormaCelularE164, "+57 300 123 4567"},
		{modelo.FormaCelularLocal, "300-123-4567"},
		{modelo.FormaDireccion, "Calle 99 # 1-2 apto 3"},
		{modelo.FormaFechaISO, "1990-02-03"},
		{modelo.FormaFechaPalabras, "3 de febrero de 1990"},
		{modelo.FormaFechaUS, "02/03/1990"},
		{modelo.FormaPlaca, "ABC123"},
		{modelo.FormaPasaporte, "XZ000000"},
	}
	for _, c := range casos {
		t.Run(string(c.tipo), func(t *testing.T) {
			f, err := Render(perfilRealMuestra(), c.tipo)
			if err != nil {
				t.Fatalf("Render(%s): %v", c.tipo, err)
			}
			if f.Tipo != c.tipo {
				t.Fatalf("Tipo = %q, quiero %q", f.Tipo, c.tipo)
			}
			if f.Valor != c.quiero {
				t.Fatalf("Valor = %q, quiero %q", f.Valor, c.quiero)
			}
		})
	}
}

// TestRenderPerfilFalso usa el perfil falso de p_057 en entities.json
// (nombre con dos nombres de pila y dos apellidos, tildes).
func TestRenderPerfilFalso(t *testing.T) {
	falso := modelo.Perfil{
		Nombre:     "Sofía Elena Zambrano Naranjo",
		Correo:     "szambrano@gmail.com",
		Celular:    "3186423850",
		Cedula:     "1107252808",
		Direccion:  "Diagonal 121B # 96-28",
		Nacimiento: "1998-11-11",
	}
	casos := []struct {
		tipo   modelo.TipoForma
		quiero string
	}{
		{modelo.FormaNombreCompleto, "Sofía Elena Zambrano Naranjo"},
		{modelo.FormaNombreCorto, "Sofía Elena Zambrano"},
		{modelo.FormaNombrePila, "Sofía Elena"},
		{modelo.FormaMayusculas, "SOFIA ELENA ZAMBRANO NARANJO"},
		{modelo.FormaSlug, "sofia_elena_zambrano"},
		{modelo.FormaCorreo, "szambrano@gmail.com"},
		{modelo.FormaCorreoDeletreado, "szambrano arroba gmail punto com"},
		{modelo.FormaCedulaPuntos, "1.107.252.808"},
		{modelo.FormaCelularE164, "+57 318 642 3850"},
		{modelo.FormaFechaPalabras, "11 de noviembre de 1998"},
		{modelo.FormaFechaUS, "11/11/1998"},
	}
	for _, c := range casos {
		t.Run(string(c.tipo), func(t *testing.T) {
			f, err := Render(falso, c.tipo)
			if err != nil {
				t.Fatalf("Render(%s): %v", c.tipo, err)
			}
			if f.Valor != c.quiero {
				t.Fatalf("Valor = %q, quiero %q", f.Valor, c.quiero)
			}
		})
	}
}

// TestRenderNombreDeDosPalabras cubre el nombre truncado de users.json
// ("Marcela Pardo"): sin segundo apellido, corto y completo coinciden.
func TestRenderNombreDeDosPalabras(t *testing.T) {
	dos := modelo.Perfil{Nombre: "Marcela Pardo"}
	casos := []struct {
		tipo   modelo.TipoForma
		quiero string
	}{
		{modelo.FormaNombreCompleto, "Marcela Pardo"},
		{modelo.FormaNombreCorto, "Marcela Pardo"},
		{modelo.FormaNombrePila, "Marcela"},
		{modelo.FormaSlug, "marcela_pardo"},
	}
	for _, c := range casos {
		f, err := Render(dos, c.tipo)
		if err != nil {
			t.Fatalf("Render(%s): %v", c.tipo, err)
		}
		if f.Valor != c.quiero {
			t.Fatalf("%s: Valor = %q, quiero %q", c.tipo, f.Valor, c.quiero)
		}
	}
}

// TestRenderNormalizaEntrada: las formas se derivan siempre del dato canónico
// del perfil, aunque llegue con separadores a la vista (cédula con puntos,
// celular con +57 y espacios, espacios de más en el nombre).
func TestRenderNormalizaEntrada(t *testing.T) {
	sucio := modelo.Perfil{
		Nombre:  "  Nicolás   Bermúdez Cárdenas ",
		Cedula:  "98.765.432",
		Celular: "+57 300 123 4567",
		Correo:  " ana.pruebas@ejemplo.test ",
	}
	casos := []struct {
		tipo   modelo.TipoForma
		quiero string
	}{
		{modelo.FormaNombreCompleto, "Nicolás Bermúdez Cárdenas"},
		{modelo.FormaNombreCorto, "Nicolás Bermúdez"},
		{modelo.FormaCedula, "98765432"},
		{modelo.FormaCedulaCC, "CC98765432"},
		{modelo.FormaCelular, "3001234567"},
		{modelo.FormaCelularLocal, "300-123-4567"},
		{modelo.FormaCorreo, "ana.pruebas@ejemplo.test"},
		{modelo.FormaCorreoDeletreado, "ana punto pruebas arroba ejemplo punto test"},
	}
	for _, c := range casos {
		f, err := Render(sucio, c.tipo)
		if err != nil {
			t.Fatalf("Render(%s): %v", c.tipo, err)
		}
		if f.Valor != c.quiero {
			t.Fatalf("%s: Valor = %q, quiero %q", c.tipo, f.Valor, c.quiero)
		}
	}
}

func TestRenderFaltaElDato(t *testing.T) {
	vacio := modelo.Perfil{}
	tipos := []modelo.TipoForma{
		modelo.FormaNombreCompleto, modelo.FormaCorreo, modelo.FormaCedula,
		modelo.FormaCelular, modelo.FormaDireccion, modelo.FormaFechaISO,
		modelo.FormaPlaca, modelo.FormaPasaporte, modelo.FormaUsuario,
	}
	for _, tipo := range tipos {
		if _, err := Render(vacio, tipo); !errors.Is(err, ErrSinDato) {
			t.Fatalf("Render(%s) sin dato: err = %v, quiero ErrSinDato", tipo, err)
		}
	}
}

func TestRenderDatoInvalido(t *testing.T) {
	if _, err := Render(modelo.Perfil{Nacimiento: "23/07/1997"}, modelo.FormaFechaISO); err == nil || errors.Is(err, ErrSinDato) {
		t.Fatalf("fecha no ISO: err = %v, quiero error de formato", err)
	}
	if _, err := Render(modelo.Perfil{Celular: "314872"}, modelo.FormaCelularE164); err == nil {
		t.Fatal("celular corto: quiero error")
	}
	if _, err := Render(modelo.Perfil{Correo: "sin-arroba"}, modelo.FormaCorreoDeletreado); err == nil {
		t.Fatal("correo sin arroba: quiero error")
	}
}

func TestRenderTipoDesconocido(t *testing.T) {
	if _, err := Render(perfilRealMuestra(), modelo.TipoForma("inventado")); err == nil {
		t.Fatal("tipo desconocido: quiero error")
	}
}

func TestTodas(t *testing.T) {
	formas, err := Todas(perfilRealMuestra())
	if err != nil {
		t.Fatalf("Todas: %v", err)
	}
	if len(formas) != len(Tipos) {
		t.Fatalf("Todas = %d formas, quiero %d", len(formas), len(Tipos))
	}
	vistos := map[modelo.TipoForma]bool{}
	for i, f := range formas {
		if f.Tipo != Tipos[i] {
			t.Fatalf("forma %d = %q, quiero %q", i, f.Tipo, Tipos[i])
		}
		if f.Valor == "" {
			t.Fatalf("%s: valor vacío", f.Tipo)
		}
		if vistos[f.Tipo] {
			t.Fatalf("%s repetido", f.Tipo)
		}
		vistos[f.Tipo] = true
	}

	// Sin datos no hay formas ni error: la fábrica puede perfiles incompletos.
	vacias, err := Todas(modelo.Perfil{})
	if err != nil {
		t.Fatalf("Todas vacío: %v", err)
	}
	if len(vacias) != 0 {
		t.Fatalf("Todas vacío = %d formas, quiero 0", len(vacias))
	}
}

func TestUsuarioSaleDelCorreo(t *testing.T) {
	p := modelo.Perfil{Correo: "ana.pruebas@ejemplo.test"}
	f, err := Render(p, modelo.FormaUsuario)
	if err != nil {
		t.Fatalf("Render(usuario): %v", err)
	}
	if f.Valor != "ana.pruebas" {
		t.Fatalf("usuario = %q, quiero %q", f.Valor, "ana.pruebas")
	}
}
