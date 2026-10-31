package almacen

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Un perfil asignado es para siempre: la segunda escritura no lo cambia y lo
// avisa con un error propio (D4).
func TestGuardarPerfilNoSobrescribe(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	primero := perfilDePrueba("Marta Gómez Díaz")
	segundo := perfilDePrueba("Otro Nombre Distinto")
	if err := a.GuardarPerfil(ctx, "p_cl_10", primero); err != nil {
		t.Fatalf("primer perfil: %v", err)
	}
	err := a.GuardarPerfil(ctx, "p_cl_10", segundo)
	if !errors.Is(err, ErrPerfilYaAsignado) {
		t.Fatalf("segundo perfil: %v, se esperaba ErrPerfilYaAsignado", err)
	}
	perfil, existe, err := a.Perfil(ctx, "p_cl_10")
	if err != nil || !existe {
		t.Fatalf("perfil: existe=%v err=%v", existe, err)
	}
	if perfil != primero {
		t.Fatalf("el perfil cambió:\nobtenido: %+v\nesperado: %+v", perfil, primero)
	}
}

// Volver a guardar el mismo perfil tampoco lo reescribe: el error es la señal
// de que la bóveda ya lo tenía.
func TestGuardarPerfilRepetidoDevuelveError(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	perfil := perfilDePrueba("Marta Gómez Díaz")
	if err := a.GuardarPerfil(ctx, "p_cl_11", perfil); err != nil {
		t.Fatalf("primer perfil: %v", err)
	}
	if err := a.GuardarPerfil(ctx, "p_cl_11", perfil); !errors.Is(err, ErrPerfilYaAsignado) {
		t.Fatalf("perfil repetido: %v, se esperaba ErrPerfilYaAsignado", err)
	}
}

// La bóveda sobrevive a la corrida: guardar un perfil no lo pierde al cerrar.
func TestGuardarPerfilSobreviveConErrorDeDuplicado(t *testing.T) {
	ctx := context.Background()
	ruta := filepath.Join(t.TempDir(), "boveda.db")
	a := abrir(t, ruta)
	perfil := perfilDePrueba("Marta Gómez Díaz")
	if err := a.GuardarPerfil(ctx, "p_kp_7", perfil); err != nil {
		t.Fatalf("perfil: %v", err)
	}
	if err := a.GuardarPerfil(ctx, "p_kp_7", perfilDePrueba("Otro Nombre Distinto")); !errors.Is(err, ErrPerfilYaAsignado) {
		t.Fatalf("duplicado: %v", err)
	}
	if err := a.Cerrar(); err != nil {
		t.Fatalf("cerrar: %v", err)
	}
	b := abrir(t, ruta)
	guardado, existe, err := b.Perfil(ctx, "p_kp_7")
	if err != nil || !existe {
		t.Fatalf("perfil tras reabrir: existe=%v err=%v", existe, err)
	}
	if guardado != perfil {
		t.Fatalf("perfil distinto tras reabrir: %+v", guardado)
	}
}

// Guardar dos veces a la misma persona agrega solo las formas que faltan, sin
// duplicar las conocidas y sin tocar rol ni género.
func TestGuardarPersonaAgregaFormasSinDuplicar(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	original := personaDePrueba("p_cl_12")
	if err := a.GuardarPersona(ctx, original); err != nil {
		t.Fatalf("primera persona: %v", err)
	}
	segunda := modelo.Persona{
		Llave:  "p_cl_12",
		Rol:    modelo.RolAgente,
		Genero: modelo.GeneroM,
		Formas: []modelo.Forma{
			{Tipo: modelo.FormaNombreCompleto, Valor: "Ana Pérez Rojas", Registro: "chat_099"},
			{Tipo: modelo.FormaNombrePila, Valor: "Ana", Registro: "chat_099"},
			{Tipo: modelo.FormaMayusculas, Valor: "ANA PEREZ ROJAS", Registro: "doc_07"},
		},
	}
	if err := a.GuardarPersona(ctx, segunda); err != nil {
		t.Fatalf("segunda persona: %v", err)
	}
	persona, existe, err := a.Persona(ctx, "p_cl_12")
	if err != nil || !existe {
		t.Fatalf("persona: existe=%v err=%v", existe, err)
	}
	if persona.Rol != original.Rol || persona.Genero != original.Genero {
		t.Fatalf("rol/género cambiaron: %+v", persona)
	}
	quiero := []modelo.Forma{
		original.Formas[0],
		original.Formas[1],
		{Tipo: modelo.FormaNombrePila, Valor: "Ana", Registro: "chat_099"},
		{Tipo: modelo.FormaMayusculas, Valor: "ANA PEREZ ROJAS", Registro: "doc_07"},
	}
	if !reflect.DeepEqual(persona.Formas, quiero) {
		t.Fatalf("formas:\nobtenido: %+v\nesperado: %+v", persona.Formas, quiero)
	}
}

// Una forma repetida conserva el registro donde se vio primero.
func TestFormaRepetidaConservaSuPrimerRegistro(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	if err := a.GuardarPersona(ctx, personaDePrueba("p_cl_13")); err != nil {
		t.Fatalf("persona: %v", err)
	}
	repetida := modelo.Persona{
		Llave:  "p_cl_13",
		Rol:    modelo.RolCliente,
		Genero: modelo.GeneroF,
		Formas: []modelo.Forma{{Tipo: modelo.FormaNombreCompleto, Valor: "Ana Pérez Rojas", Registro: "doc_42"}},
	}
	if err := a.GuardarPersona(ctx, repetida); err != nil {
		t.Fatalf("persona repetida: %v", err)
	}
	persona, _, err := a.Persona(ctx, "p_cl_13")
	if err != nil {
		t.Fatalf("persona: %v", err)
	}
	if len(persona.Formas) != 2 {
		t.Fatalf("formas %d, se esperaban 2: %+v", len(persona.Formas), persona.Formas)
	}
	if persona.Formas[0].Registro != "chat_001" {
		t.Fatalf("registro de origen %q, se esperaba chat_001", persona.Formas[0].Registro)
	}
}

// El registro de personas es de solo agregar: guardar de nuevo no borra formas
// ya conocidas.
func TestGuardarPersonaNoBorraFormas(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	original := personaDePrueba("p_cl_14")
	if err := a.GuardarPersona(ctx, original); err != nil {
		t.Fatalf("persona: %v", err)
	}
	vacia := modelo.Persona{Llave: "p_cl_14", Rol: modelo.RolCliente, Genero: modelo.GeneroF}
	if err := a.GuardarPersona(ctx, vacia); err != nil {
		t.Fatalf("persona sin formas: %v", err)
	}
	persona, _, err := a.Persona(ctx, "p_cl_14")
	if err != nil {
		t.Fatalf("persona: %v", err)
	}
	if !reflect.DeepEqual(persona.Formas, original.Formas) {
		t.Fatalf("formas:\nobtenido: %+v\nesperado: %+v", persona.Formas, original.Formas)
	}
}

// La misma cadena puede ser dos formas distintas de la misma persona (slug y
// usuario, por ejemplo): se distinguen por tipo.
func TestFormasDelMismoValorConTipoDistinto(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	persona := modelo.Persona{
		Llave:  "p_t_ab12cd",
		Rol:    modelo.RolDesconocido,
		Genero: modelo.GeneroIndeterminado,
		Formas: []modelo.Forma{
			{Tipo: modelo.FormaSlug, Valor: "ana-perez"},
			{Tipo: modelo.FormaUsuario, Valor: "ana-perez"},
		},
	}
	if err := a.GuardarPersona(ctx, persona); err != nil {
		t.Fatalf("persona: %v", err)
	}
	guardada, _, err := a.Persona(ctx, "p_t_ab12cd")
	if err != nil {
		t.Fatalf("persona: %v", err)
	}
	if len(guardada.Formas) != 2 {
		t.Fatalf("formas %d, se esperaban 2: %+v", len(guardada.Formas), guardada.Formas)
	}
}

func TestConsultasDeLoQueNoExiste(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	persona, existe, err := a.Persona(ctx, "p_cl_99")
	if err != nil || existe || persona.Llave != "" || len(persona.Formas) != 0 {
		t.Fatalf("persona inexistente: %+v existe=%v err=%v", persona, existe, err)
	}
	perfil, existe, err := a.Perfil(ctx, "p_cl_99")
	if err != nil || existe || perfil != (modelo.Perfil{}) {
		t.Fatalf("perfil inexistente: %+v existe=%v err=%v", perfil, existe, err)
	}
}

// Basura en la entrada no entra a la bóveda: sin llave no hay persona y sin
// nombre no hay perfil.
func TestGuardarRechazaDatosIncompletos(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	if err := a.GuardarPersona(ctx, modelo.Persona{Rol: modelo.RolCliente}); err == nil {
		t.Fatal("se guardó una persona sin llave")
	}
	if err := a.GuardarPerfil(ctx, "", perfilDePrueba("Marta Gómez Díaz")); err == nil {
		t.Fatal("se guardó un perfil sin llave")
	}
	if err := a.GuardarPerfil(ctx, "p_cl_15", modelo.Perfil{Correo: "m.gomez@example.test"}); err == nil {
		t.Fatal("se guardó un perfil sin nombre")
	}
}
