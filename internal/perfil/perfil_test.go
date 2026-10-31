package perfil_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/catalogo"
	"github.com/game013/frontier-hackaton-31102026/internal/formas"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
	"github.com/game013/frontier-hackaton-31102026/internal/perfil"
)

// secreto es la semilla del HMAC en las pruebas: un valor fijo hace que el
// perfil sea reproducible entre corridas.
var secreto = []byte("secreto-de-prueba-para-la-fabrica")

// realConDatos es una persona real inventada con todas las formas que el
// corpus puede traer: dos nombres de pila, dos apellidos y los identificadores
// que la fábrica tiene que imitar en estructura.
func realConDatos() modelo.Perfil {
	return modelo.Perfil{
		Nombre:     "Ana Lucía Restrepo Villa",
		Correo:     "alrestrepo@correo.test",
		Celular:    "3105557788",
		Cedula:     "1029384756",
		Direccion:  "Calle 12 # 45-67 apto 801",
		Nacimiento: "1990-05-14",
		Placa:      "JKL321",
		Pasaporte:  "AP123456",
	}
}

// personaDe arma la persona real con las formas que renderiza su perfil real:
// es el mismo camino que sigue el registro de personas.
func personaDe(t *testing.T, llave string, rol modelo.Rol, genero modelo.Genero, real modelo.Perfil) modelo.Persona {
	t.Helper()
	formas, err := formas.Todas(real)
	if err != nil {
		t.Fatalf("formas.Todas(%q): %v", real.Nombre, err)
	}
	return modelo.Persona{Llave: llave, Rol: rol, Genero: genero, Formas: formas}
}

func fabricaDe(t *testing.T, opciones perfil.Opciones) *perfil.Fabrica {
	t.Helper()
	f, err := perfil.Nuevo(opciones)
	if err != nil {
		t.Fatalf("perfil.Nuevo: %v", err)
	}
	return f
}

// TestMismaLLaveMismoPerfil fija el requisito central (D4): el perfil es
// función de la llave y del secreto, no del orden ni del azar. Dos fábricas
// distintas y dos llamadas seguidas dan el mismo perfil, campo por campo.
func TestMismaLLaveMismoPerfil(t *testing.T) {
	p := personaDe(t, "p_cl_0001", modelo.RolCliente, modelo.GeneroF, realConDatos())
	opciones := perfil.Opciones{Secreto: secreto, Reales: p.Formas}

	primera := fabricaDe(t, opciones)
	segunda := fabricaDe(t, opciones)

	uno, err := primera.PerfilDe(context.Background(), p)
	if err != nil {
		t.Fatalf("PerfilDe: %v", err)
	}
	if uno.Nombre == "" {
		t.Fatalf("el perfil salió sin nombre: %+v", uno)
	}
	otraFabrica, err := segunda.PerfilDe(context.Background(), p)
	if err != nil {
		t.Fatalf("PerfilDe (otra fábrica): %v", err)
	}
	if otraFabrica != uno {
		t.Fatalf("dos fábricas con el mismo secreto dieron perfiles distintos:\n%+v\n%+v", uno, otraFabrica)
	}
	deNuevo, err := primera.PerfilDe(context.Background(), p)
	if err != nil {
		t.Fatalf("PerfilDe (segunda llamada): %v", err)
	}
	if deNuevo != uno {
		t.Fatalf("la segunda llamada cambió el perfil:\n%+v\n%+v", uno, deNuevo)
	}
}

// TestSecretoVacio es el único error de construcción: sin secreto no hay HMAC
// y el perfil dejaría de ser determinista entre corridas.
func TestSecretoVacio(t *testing.T) {
	if _, err := perfil.Nuevo(perfil.Opciones{}); err == nil {
		t.Fatal("Nuevo aceptó un secreto vacío")
	}
}

// TestLaLLaveSeparaPerfiles: dos personas distintas —aunque compartan nombre
// real y formas— nunca reciben el mismo perfil.
func TestLaLLaveSeparaPerfiles(t *testing.T) {
	real := realConDatos()
	una := personaDe(t, "p_cl_0001", modelo.RolCliente, modelo.GeneroF, real)
	otra := personaDe(t, "p_cl_0002", modelo.RolCliente, modelo.GeneroF, real)
	f := fabricaDe(t, perfil.Opciones{Secreto: secreto, Reales: una.Formas})

	perfilUno, err := f.PerfilDe(context.Background(), una)
	if err != nil {
		t.Fatalf("PerfilDe(%q): %v", una.Llave, err)
	}
	perfilDos, err := f.PerfilDe(context.Background(), otra)
	if err != nil {
		t.Fatalf("PerfilDe(%q): %v", otra.Llave, err)
	}
	if perfilUno.Nombre == perfilDos.Nombre {
		t.Fatalf("dos llaves distintas dieron el mismo nombre %q", perfilUno.Nombre)
	}
	if perfilUno.Correo == perfilDos.Correo {
		t.Fatalf("dos llaves distintas dieron el mismo correo %q", perfilUno.Correo)
	}
	if perfilUno.Cedula == perfilDos.Cedula {
		t.Fatalf("dos llaves distintas dieron la misma cédula %q", perfilUno.Cedula)
	}
	if perfilUno.Celular == perfilDos.Celular {
		t.Fatalf("dos llaves distintas dieron el mismo celular %q", perfilUno.Celular)
	}
}

// contiene dice si la lista trae el valor: es el contraste contra el catálogo,
// sin tocar los índices de la fábrica.
func contiene(lista []string, valor string) bool {
	for _, item := range lista {
		if item == valor {
			return true
		}
	}
	return false
}

// TestGeneroYEstructura fija lo que el perfil imita del nombre real (D18,
// §6.1.5): el género del catálogo, el mismo número de nombres de pila y dos
// apellidos, todos distintos entre sí.
func TestGeneroYEstructura(t *testing.T) {
	casos := []struct {
		nombre string
		genero modelo.Genero
		real   modelo.Perfil
		pila   int
	}{
		{"dos nombres de pila", modelo.GeneroF, realConDatos(), 2},
		{"un nombre de pila", modelo.GeneroM, modelo.Perfil{Nombre: "Hugo Cano Pineda"}, 1},
		{"unisex", modelo.GeneroUnisex, modelo.Perfil{Nombre: "Alex Rojas Medina"}, 1},
		{"indeterminado", modelo.GeneroIndeterminado, modelo.Perfil{Nombre: "Alex Rojas Medina"}, 1},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			p := personaDe(t, "p_cl_0100", modelo.RolCliente, caso.genero, caso.real)
			f := fabricaDe(t, perfil.Opciones{Secreto: secreto, Reales: p.Formas})
			obtenido, err := f.PerfilDe(context.Background(), p)
			if err != nil {
				t.Fatalf("PerfilDe: %v", err)
			}
			palabras := strings.Fields(obtenido.Nombre)
			if len(palabras) != caso.pila+2 {
				t.Fatalf("%q tiene %d palabras, se esperaban %d (dos apellidos y %d nombres de pila)",
					obtenido.Nombre, len(palabras), caso.pila+2, caso.pila)
			}
			pila := catalogo.NombresPila(caso.genero, catalogo.OrigenColombiano)
			for i, nombrePila := range palabras[:caso.pila] {
				if !contiene(pila, nombrePila) {
					t.Errorf("%q: el nombre de pila %d (%q) no es del catálogo del género %s",
						obtenido.Nombre, i+1, nombrePila, caso.genero)
				}
			}
			if palabras[caso.pila] == palabras[caso.pila+1] {
				t.Errorf("%q repite el apellido", obtenido.Nombre)
			}
			for _, apellido := range palabras[caso.pila:] {
				if !contiene(catalogo.Apellidos(), apellido) {
					t.Errorf("%q: %q no es del catálogo de apellidos", obtenido.Nombre, apellido)
				}
			}
			if caso.pila == 2 && palabras[0] == palabras[1] {
				t.Errorf("%q repite el nombre de pila", obtenido.Nombre)
			}
		})
	}
}

// Los patrones fijan el estilo de los datos falsos, deducido de la salida ideal
// (analisis/01_sample.md §1.2).
var (
	patronDireccion = regexp.MustCompile(`^(Calle|Cl|Cra|Diagonal|Tv|Transversal) \d{1,3}(A|B| Bis)?( Sur)? # \d{1,2}-\d{1,2}( (apto|of|casa) \d{2,3})?$`)
	patronPlaca     = regexp.MustCompile(`^[A-Z]{3}\d{3}$`)
	patronPasaporte = regexp.MustCompile(`^[A-Z]{2}\d{6}$`)
)

// sinTildes prueba a la letra: el correo usa el nombre sin tildes.
func sinTildes(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case 'á', 'à', 'ä', 'â', 'ã':
			return 'a'
		case 'é', 'è', 'ë', 'ê':
			return 'e'
		case 'í', 'ì', 'ï', 'î':
			return 'i'
		case 'ó', 'ò', 'ö', 'ô', 'õ':
			return 'o'
		case 'ú', 'ù', 'ü', 'û':
			return 'u'
		case 'ñ':
			return 'n'
		}
		return r
	}, strings.ToLower(s))
}

// TestDatosCoherentes comprueba que el perfil es una identidad completa y
// coherente consigo misma: el correo sale del nombre, la cédula conserva la
// longitud real, el celular es 3xx de 10 dígitos, la dirección es colombiana,
// la fecha queda cerca de la real y placa y pasaporte solo existen si la
// persona real los tiene.
func TestDatosCoherentes(t *testing.T) {
	real := realConDatos()
	p := personaDe(t, "p_cl_0200", modelo.RolCliente, modelo.GeneroF, real)
	f := fabricaDe(t, perfil.Opciones{Secreto: secreto, Reales: p.Formas})
	obtenido, err := f.PerfilDe(context.Background(), p)
	if err != nil {
		t.Fatalf("PerfilDe: %v", err)
	}

	palabras := strings.Fields(obtenido.Nombre)
	local := string([]rune(sinTildes(palabras[0]))[:1]) + sinTildes(palabras[2])
	arroba := strings.LastIndex(obtenido.Correo, "@")
	if arroba <= 0 {
		t.Fatalf("correo sin arroba: %q", obtenido.Correo)
	}
	if obtenido.Correo[:arroba] != local {
		t.Errorf("el correo %q no sale de la inicial y el primer apellido del nombre %q (se esperaba %q)",
			obtenido.Correo, obtenido.Nombre, local)
	}
	if dominio := obtenido.Correo[arroba+1:]; !contiene(catalogo.Dominios(), dominio) {
		t.Errorf("el dominio %q de un cliente no es del pool público", dominio)
	}

	if largo, esperado := len(obtenido.Cedula), len(soloDigitos(real.Cedula)); largo != esperado {
		t.Errorf("la cédula %q tiene %d dígitos, se esperaban %d", obtenido.Cedula, largo, esperado)
	}
	if soloDigitos(obtenido.Cedula) != obtenido.Cedula {
		t.Errorf("la cédula %q no es de solo dígitos", obtenido.Cedula)
	}

	if len(obtenido.Celular) != 10 || !strings.HasPrefix(obtenido.Celular, "3") {
		t.Errorf("el celular %q no es 3xx de 10 dígitos", obtenido.Celular)
	}

	if !patronDireccion.MatchString(obtenido.Direccion) {
		t.Errorf("la dirección %q no es una dirección colombiana", obtenido.Direccion)
	}

	fechaReal, err := time.Parse("2006-01-02", real.Nacimiento)
	if err != nil {
		t.Fatalf("fecha real %q: %v", real.Nacimiento, err)
	}
	fechaFalsa, err := time.Parse("2006-01-02", obtenido.Nacimiento)
	if err != nil {
		t.Fatalf("la fecha falsa %q no está en ISO: %v", obtenido.Nacimiento, err)
	}
	if fechaFalsa.Equal(fechaReal) {
		t.Errorf("la fecha de nacimiento no cambió: %q", obtenido.Nacimiento)
	}
	if distancia := fechaFalsa.Sub(fechaReal); distancia > 4*365*24*time.Hour || distancia < -4*365*24*time.Hour {
		t.Errorf("la fecha falsa %q está a %v de la real, más de cuatro años", obtenido.Nacimiento, distancia)
	}

	if !patronPlaca.MatchString(obtenido.Placa) {
		t.Errorf("la placa %q no es colombiana (tres letras y tres dígitos)", obtenido.Placa)
	}
	if !patronPasaporte.MatchString(obtenido.Pasaporte) {
		t.Errorf("el pasaporte %q no tiene el formato del corpus (dos letras y seis dígitos)", obtenido.Pasaporte)
	}
}

// TestCorreoCorporativo es el dominio de la empresa: los empleados conservan
// kipupay.co y el resto cae a un dominio público.
func TestCorreoCorporativo(t *testing.T) {
	real := realConDatos()
	empleado := personaDe(t, "p_kp_0001", modelo.RolEmpleado, modelo.GeneroF, real)
	f := fabricaDe(t, perfil.Opciones{Secreto: secreto, Reales: empleado.Formas})
	obtenido, err := f.PerfilDe(context.Background(), empleado)
	if err != nil {
		t.Fatalf("PerfilDe: %v", err)
	}
	if !strings.HasSuffix(obtenido.Correo, "@kipupay.co") {
		t.Errorf("el correo de un empleado %q no es del dominio de la empresa", obtenido.Correo)
	}
	if local := localDe(t, obtenido.Correo); strings.Contains(local, ".") {
		t.Errorf("el correo de empleado %q no lleva inicial y apellido pegados", obtenido.Correo)
	}

	agente := personaDe(t, "p_kp_0002", modelo.RolAgente, modelo.GeneroF, real)
	perfilAgente, err := f.PerfilDe(context.Background(), agente)
	if err != nil {
		t.Fatalf("PerfilDe: %v", err)
	}
	if dominio := dominioDe(t, perfilAgente.Correo); !contiene(catalogo.Dominios(), dominio) {
		t.Errorf("el dominio %q de un agente no es del pool público", dominio)
	}
}

// TestSoloLosDatosQueLaPersonaTiene: sin fecha, placa ni pasaporte reales, el
// perfil no inventa esos datos; el resto de la identidad sigue completa.
func TestSoloLosDatosQueLaPersonaTiene(t *testing.T) {
	real := modelo.Perfil{Nombre: "Ana Lucía Restrepo Villa"}
	p := personaDe(t, "p_t_0001", modelo.RolDesconocido, modelo.GeneroF, real)
	f := fabricaDe(t, perfil.Opciones{Secreto: secreto, Reales: p.Formas})
	obtenido, err := f.PerfilDe(context.Background(), p)
	if err != nil {
		t.Fatalf("PerfilDe: %v", err)
	}
	if obtenido.Nacimiento != "" || obtenido.Placa != "" || obtenido.Pasaporte != "" {
		t.Errorf("el perfil inventó datos que la persona real no tiene: %+v", obtenido)
	}
	if obtenido.Nombre == "" || obtenido.Correo == "" || obtenido.Cedula == "" ||
		obtenido.Celular == "" || obtenido.Direccion == "" {
		t.Errorf("el perfil quedó incompleto: %+v", obtenido)
	}
}

// localDe y dominioDe parten el correo sin dejar que una prueba reviente: si
// no hay arroba, el fallo es del perfil.
func localDe(t *testing.T, correo string) string {
	t.Helper()
	arroba := strings.LastIndex(correo, "@")
	if arroba <= 0 {
		t.Fatalf("correo sin arroba válida: %q", correo)
	}
	return correo[:arroba]
}

func dominioDe(t *testing.T, correo string) string {
	t.Helper()
	return correo[strings.LastIndex(correo, "@")+1:]
}

// soloDigitos replica la regla del formato: la cédula se compara sin puntos.
func soloDigitos(valor string) string {
	var b strings.Builder
	for _, r := range valor {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// chocaEnPrueba replica la vara del jurado (norm.Norm y dígitos sin
// separadores) sin usar el código bajo prueba: es el oráculo del contraste.
func chocaEnPrueba(valor string, prohibidas []string) bool {
	d := soloDigitos(valor)
	for _, prohibida := range prohibidas {
		if norm.Norm(valor) == norm.Norm(prohibida) {
			return true
		}
		pd := soloDigitos(prohibida)
		if len(d) >= 6 && len(pd) >= 6 {
			if d == pd {
				return true
			}
			if len(pd) == 12 && strings.HasPrefix(pd, "57") && d == pd[2:] {
				return true
			}
		}
	}
	return false
}

// TestEvitaFormasReales es el requisito de la compuerta visto desde la fábrica
// (D4): ninguna forma del perfil coincide con una forma real, ni con la forma
// que el primer intento ya produjo.
func TestEvitaFormasReales(t *testing.T) {
	t.Run("formas de la persona", func(t *testing.T) {
		real := realConDatos()
		p := personaDe(t, "p_cl_0300", modelo.RolCliente, modelo.GeneroF, real)
		f := fabricaDe(t, perfil.Opciones{Secreto: secreto, Reales: p.Formas})
		obtenido, err := f.PerfilDe(context.Background(), p)
		if err != nil {
			t.Fatalf("PerfilDe: %v", err)
		}
		prohibidas := valoresDe(t, p.Formas)
		for _, forma := range formasDePerfil(t, obtenido) {
			if chocaEnPrueba(forma.Valor, prohibidas) {
				t.Errorf("la forma %s=%q repite una forma real", forma.Tipo, forma.Valor)
			}
		}
	})

	t.Run("el primer intento ya está ocupado", func(t *testing.T) {
		real := realConDatos()
		p := personaDe(t, "p_cl_0301", modelo.RolCliente, modelo.GeneroF, real)
		limpia := fabricaDe(t, perfil.Opciones{Secreto: secreto, Reales: p.Formas})
		primero, err := limpia.PerfilDe(context.Background(), p)
		if err != nil {
			t.Fatalf("PerfilDe: %v", err)
		}

		// Las formas del primer perfil entran al registro como formas reales:
		// la fábrica tiene que avanzar a otro perfil completo.
		prohibidas := append(valoresDe(t, p.Formas), valoresDe(t, formasDePerfil(t, primero))...)
		ocupada := fabricaDe(t, perfil.Opciones{Secreto: secreto, Reales: formasConValor(prohibidas)})
		segundo, err := ocupada.PerfilDe(context.Background(), p)
		if err != nil {
			t.Fatalf("PerfilDe (con el primero ocupado): %v", err)
		}
		for _, forma := range formasDePerfil(t, segundo) {
			if chocaEnPrueba(forma.Valor, prohibidas) {
				t.Errorf("la forma %s=%q choca con una forma ya ocupada", forma.Tipo, forma.Valor)
			}
		}
		if segundo == primero {
			t.Fatalf("la fábrica devolvió el perfil ocupado: %+v", primero)
		}
		if segundo.Nombre == primero.Nombre || segundo.Correo == primero.Correo ||
			segundo.Cedula == primero.Cedula || segundo.Celular == primero.Celular {
			t.Errorf("el perfil avanzado repite datos del ocupado:\n%+v\n%+v", primero, segundo)
		}
	})

	t.Run("nombre de pila que sale del catálogo", func(t *testing.T) {
		// La persona real se llama como el primer nombre del catálogo: si la
		// fábrica no descartara la forma real, lo volvería a elegir.
		pila := catalogo.NombresPila(modelo.GeneroF, catalogo.OrigenColombiano)[0]
		real := modelo.Perfil{Nombre: pila + " Restrepo Villa"}
		p := personaDe(t, "p_cl_0302", modelo.RolCliente, modelo.GeneroF, real)
		f := fabricaDe(t, perfil.Opciones{Secreto: secreto, Reales: p.Formas})
		obtenido, err := f.PerfilDe(context.Background(), p)
		if err != nil {
			t.Fatalf("PerfilDe: %v", err)
		}
		if nombre := strings.Fields(obtenido.Nombre)[0]; nombre == pila {
			t.Errorf("el perfil %q reusa el nombre de pila real %q", obtenido.Nombre, pila)
		}
	})
}

// valoresDe extrae los valores de las formas.
func valoresDe(t *testing.T, formas []modelo.Forma) []string {
	t.Helper()
	valores := make([]string, 0, len(formas))
	for _, forma := range formas {
		valores = append(valores, forma.Valor)
	}
	return valores
}

// formasConValor envuelve los valores como formas reales del corpus.
func formasConValor(valores []string) []modelo.Forma {
	formas := make([]modelo.Forma, 0, len(valores))
	for _, valor := range valores {
		formas = append(formas, modelo.Forma{Valor: valor})
	}
	return formas
}

// formasDePerfil renderiza todas las formas del perfil falso.
func formasDePerfil(t *testing.T, p modelo.Perfil) []modelo.Forma {
	t.Helper()
	lista, err := formas.Todas(p)
	if err != nil {
		t.Fatalf("formas.Todas(%q): %v", p.Nombre, err)
	}
	return lista
}
