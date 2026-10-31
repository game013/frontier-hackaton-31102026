package perfil_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/almacen"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/perfil"
)

// abreBoveda abre la bóveda real de SQLite en el directorio temporal de la
// prueba: el candado de "solo agrega" y la unicidad se prueban contra la base.
func abreBoveda(t *testing.T) *almacen.Almacen {
	t.Helper()
	base, err := almacen.Open(filepath.Join(t.TempDir(), "boveda.db"))
	if err != nil {
		t.Fatalf("almacen.Open: %v", err)
	}
	return base
}

// TestBovedaNoRepiteDatos es la unicidad de D4: con varias personas que
// comparten las mismas formas reales, ningún perfil guardado repite nombre,
// correo, cédula ni celular de otro.
func TestBovedaNoRepiteDatos(t *testing.T) {
	base := abreBoveda(t)
	defer base.Cerrar()
	ctx := context.Background()

	real := realConDatos()
	personas := []modelo.Persona{
		personaDe(t, "p_cl_0400", modelo.RolCliente, modelo.GeneroF, real),
		personaDe(t, "p_cl_0401", modelo.RolCliente, modelo.GeneroF, real),
		personaDe(t, "p_cl_0402", modelo.RolCliente, modelo.GeneroF, real),
		personaDe(t, "p_kp_0403", modelo.RolEmpleado, modelo.GeneroF, real),
		personaDe(t, "p_t_0404", modelo.RolDesconocido, modelo.GeneroF, real),
	}
	f := fabricaDe(t, perfil.Opciones{Secreto: secreto, Almacen: base, Reales: personas[0].Formas})

	perfiles := map[string]modelo.Perfil{}
	for _, p := range personas {
		obtenido, err := f.PerfilDe(ctx, p)
		if err != nil {
			t.Fatalf("PerfilDe(%q): %v", p.Llave, err)
		}
		perfiles[p.Llave] = obtenido
	}

	vistos := map[string]string{}
	for _, p := range personas {
		obtenido := perfiles[p.Llave]
		for _, dato := range []struct{ campo, valor string }{
			{"nombre", obtenido.Nombre},
			{"correo", strings.ToLower(obtenido.Correo)},
			{"cédula", soloDigitos(obtenido.Cedula)},
			{"celular", soloDigitos(obtenido.Celular)},
		} {
			clave := dato.campo + ":" + dato.valor
			if dueno, repetido := vistos[clave]; repetido {
				t.Errorf("el %s de %q se repite con %q", dato.campo, p.Llave, dueno)
			}
			vistos[clave] = p.Llave
		}
	}

	guardados, err := base.Perfiles(ctx)
	if err != nil {
		t.Fatalf("Perfiles: %v", err)
	}
	if len(guardados) != len(personas) {
		t.Fatalf("la bóveda tiene %d perfiles, se esperaban %d", len(guardados), len(personas))
	}
	for llave, obtenido := range perfiles {
		if guardados[llave] != obtenido {
			t.Errorf("la bóveda no guardó el perfil devuelto de %q:\n%+v\n%+v", llave, obtenido, guardados[llave])
		}
	}
}

// TestPerfilGuardadoNoCambia es la garantía que sostiene la consistencia entre
// corridas: con la bóveda cerrada y reabierta, la persona recibe el mismo
// perfil y la bóveda no lo recalcula.
func TestPerfilGuardadoNoCambia(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "boveda.db")
	ctx := context.Background()
	p := personaDe(t, "p_cl_0500", modelo.RolCliente, modelo.GeneroF, realConDatos())

	primera, err := almacen.Open(ruta)
	if err != nil {
		t.Fatalf("almacen.Open: %v", err)
	}
	antes, err := fabricaDe(t, perfil.Opciones{Secreto: secreto, Almacen: primera, Reales: p.Formas}).
		PerfilDe(ctx, p)
	if err != nil {
		t.Fatalf("PerfilDe: %v", err)
	}
	if err := primera.Cerrar(); err != nil {
		t.Fatalf("Cerrar: %v", err)
	}

	segunda, err := almacen.Open(ruta)
	if err != nil {
		t.Fatalf("almacen.Open (reapertura): %v", err)
	}
	defer segunda.Cerrar()
	despues, err := fabricaDe(t, perfil.Opciones{Secreto: secreto, Almacen: segunda, Reales: p.Formas}).
		PerfilDe(ctx, p)
	if err != nil {
		t.Fatalf("PerfilDe (reapertura): %v", err)
	}
	if despues != antes {
		t.Fatalf("el perfil cambió tras cerrar y reabrir la bóveda:\n%+v\n%+v", antes, despues)
	}
	guardados, err := segunda.Perfiles(ctx)
	if err != nil {
		t.Fatalf("Perfiles: %v", err)
	}
	if guardados[p.Llave] != antes {
		t.Fatalf("la bóveda no conserva el perfil asignado:\n%+v\n%+v", antes, guardados[p.Llave])
	}
}

// TestLaBovedaManda: un perfil ya asignado se devuelve tal cual, sin recalcular
// nada, aunque la llave y el secreto producirían otro.
func TestLaBovedaManda(t *testing.T) {
	base := abreBoveda(t)
	defer base.Cerrar()
	ctx := context.Background()

	p := personaDe(t, "p_cl_0600", modelo.RolCliente, modelo.GeneroF, realConDatos())
	asignado := modelo.Perfil{
		Nombre:    "Bianca Ortiz Cuenca",
		Correo:    "bortiz@correo.test",
		Cedula:    "1122334455",
		Celular:   "3201112233",
		Direccion: "Cra 9 # 12-34",
	}
	if err := base.GuardarPerfil(ctx, p.Llave, asignado); err != nil {
		t.Fatalf("GuardarPerfil: %v", err)
	}
	obtenido, err := fabricaDe(t, perfil.Opciones{Secreto: secreto, Almacen: base, Reales: p.Formas}).
		PerfilDe(ctx, p)
	if err != nil {
		t.Fatalf("PerfilDe: %v", err)
	}
	if obtenido != asignado {
		t.Fatalf("la fábrica recalculó un perfil ya asignado:\n%+v\n%+v", asignado, obtenido)
	}
}

// almacenGanador simula la otra corrida que asigna el perfil primero: el
// guardado de esta corrida choca y la lectura devuelve el que ganó.
type almacenGanador struct {
	asignado modelo.Perfil
	escrito  bool
	falla    error
}

var _ modelo.Almacen = (*almacenGanador)(nil)

func (a *almacenGanador) Perfil(context.Context, string) (modelo.Perfil, bool, error) {
	if a.falla != nil {
		return modelo.Perfil{}, false, a.falla
	}
	if a.escrito {
		return a.asignado, true, nil
	}
	return modelo.Perfil{}, false, nil
}

func (a *almacenGanador) GuardarPerfil(context.Context, string, modelo.Perfil) error {
	a.escrito = true
	return almacen.ErrPerfilYaAsignado
}

func (a *almacenGanador) Perfiles(context.Context) (map[string]modelo.Perfil, error) {
	if a.escrito {
		return map[string]modelo.Perfil{"p_cl_0700": a.asignado}, nil
	}
	return map[string]modelo.Perfil{}, nil
}

func (a *almacenGanador) Persona(context.Context, string) (modelo.Persona, bool, error) {
	return modelo.Persona{}, false, nil
}

func (a *almacenGanador) Personas(context.Context) ([]modelo.Persona, error) { return nil, nil }

func (a *almacenGanador) GuardarPersona(context.Context, modelo.Persona) error { return nil }

func (a *almacenGanador) GuardarPersonaVivo(context.Context, modelo.Persona) error { return nil }

func (a *almacenGanador) AnotaDudoso(context.Context, modelo.Mencion) error { return nil }

func (a *almacenGanador) Cerrar() error { return nil }

// TestPerfilYaAsignadoSeRelee: si otra corrida guardó el perfil entre la
// lectura y el guardado, manda el guardado (D4, bóveda de solo agregar).
func TestPerfilYaAsignadoSeRelee(t *testing.T) {
	ganador := &almacenGanador{asignado: modelo.Perfil{Nombre: "Teresa Vargas Lugo"}}
	p := personaDe(t, "p_cl_0700", modelo.RolCliente, modelo.GeneroF, realConDatos())
	obtenido, err := fabricaDe(t, perfil.Opciones{Secreto: secreto, Almacen: ganador, Reales: p.Formas}).
		PerfilDe(context.Background(), p)
	if err != nil {
		t.Fatalf("PerfilDe: %v", err)
	}
	if obtenido != ganador.asignado {
		t.Fatalf("la fábrica no devolvió el perfil ya asignado:\n%+v\n%+v", ganador.asignado, obtenido)
	}
}

// TestErroresDeLaBoveda envuelve el fallo de la base en español y con el
// contexto de la persona: un error de almacenamiento no puede confundirse con
// un choque de formas.
func TestErroresDeLaBoveda(t *testing.T) {
	falla := errors.New("base rota")
	roto := &almacenGanador{falla: falla}
	p := personaDe(t, "p_cl_0800", modelo.RolCliente, modelo.GeneroF, realConDatos())
	_, err := fabricaDe(t, perfil.Opciones{Secreto: secreto, Almacen: roto, Reales: p.Formas}).
		PerfilDe(context.Background(), p)
	if err == nil {
		t.Fatal("PerfilDe no propagó el error de la bóveda")
	}
	if !errors.Is(err, falla) {
		t.Errorf("el error no envuelve el de la bóveda: %v", err)
	}
	if !strings.Contains(err.Error(), "perfil:") || !strings.Contains(err.Error(), p.Llave) {
		t.Errorf("el error no dice qué pasó ni con quién: %v", err)
	}
}
