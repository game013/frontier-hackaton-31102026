package respaldo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/almacen"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/respaldo"
)

// ahoraFijo es la marca de tiempo con la que se nombra el respaldo: el nombre
// lleva UTC, no la hora local de la máquina.
var ahoraFijo = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// nombreEsperado es el archivo que la prueba espera para ahoraFijo.
const nombreEsperado = "boveda-20261031T120000Z.db"

// personaInventada es una persona que no existe: ninguna de sus formas sale del
// corpus real.
func personaInventada(llave, nombre, correo string) modelo.Persona {
	return modelo.Persona{
		Llave:  llave,
		Rol:    modelo.RolCliente,
		Genero: modelo.GeneroF,
		Formas: []modelo.Forma{
			{Tipo: modelo.FormaNombreCompleto, Valor: nombre, Registro: "clientes:" + llave},
			{Tipo: modelo.FormaCorreo, Valor: correo, Registro: "clientes:" + llave},
		},
	}
}

// perfilInventado es el perfil falso que la bóveda guarda para esa persona.
func perfilInventado(nombre string) modelo.Perfil {
	return modelo.Perfil{
		Nombre:     nombre,
		Correo:     "m.gomez@example.test",
		Celular:    "3001234567",
		Cedula:     "1020304050",
		Handle:     "m.gomez",
		Direccion:  "Calle 10 #20-30",
		Nacimiento: "1990-05-18",
		Placa:      "ABC123",
		Cuenta:     "000123456789",
		Pasaporte:  "PA1234567",
	}
}

// bovedaDePrueba abre la bóveda en <datos>/almacen.db y le guarda una persona
// inventada con su perfil. Es de solo agregar, así que la deja abierta para que
// la prueba corra sobre una base en uso (WAL).
func bovedaDePrueba(t *testing.T, datos string, personas ...modelo.Persona) *almacen.Almacen {
	t.Helper()
	a, err := almacen.Open(filepath.Join(datos, "almacen.db"))
	if err != nil {
		t.Fatalf("abrir la bóveda: %v", err)
	}
	t.Cleanup(func() { a.Cerrar() })
	ctx := context.Background()
	for i, p := range personas {
		if err := a.GuardarPersona(ctx, p); err != nil {
			t.Fatalf("guardar persona: %v", err)
		}
		if err := a.GuardarPerfil(ctx, p.Llave, perfilInventado("Marta Gómez Díaz")); err != nil {
			t.Fatalf("guardar perfil %d: %v", i, err)
		}
	}
	return a
}

// El respaldo de una bóveda en uso queda completo, con nombre UTC y permisos
// privados: es la única copia que se lleva a otro disco.
func TestRespaldaCopiaCoherenteDeUnaBovedaEnUso(t *testing.T) {
	raiz := t.TempDir()
	datos := filepath.Join(raiz, "datos")
	destino := filepath.Join(raiz, "respaldos")
	bovedaDePrueba(t, datos,
		personaInventada("p_cl_90001", "Ana Pérez Rojas", "ana.perez@example.test"),
		personaInventada("p_cl_90002", "Luis Santander Mora", "luis.santander@example.test"),
	)

	r, err := respaldo.Respalda(datos, destino, ahoraFijo)
	if err != nil {
		t.Fatalf("Respalda: %v", err)
	}
	if quiero := filepath.Join(destino, nombreEsperado); r.Ruta != quiero {
		t.Fatalf("ruta = %q, quiero %q", r.Ruta, quiero)
	}
	info, err := os.Stat(r.Ruta)
	if err != nil {
		t.Fatalf("stat del respaldo: %v", err)
	}
	if r.Bytes != info.Size() || r.Bytes == 0 {
		t.Fatalf("bytes = %d, el archivo mide %d", r.Bytes, info.Size())
	}
	if modo := info.Mode().Perm(); modo != 0o600 {
		t.Fatalf("permisos = %o, quiero 600: el respaldo lleva datos reales", modo)
	}
	datosArchivo, err := os.ReadFile(r.Ruta)
	if err != nil {
		t.Fatalf("leer el respaldo: %v", err)
	}
	suma := sha256.Sum256(datosArchivo)
	if r.SHA256 != hex.EncodeToString(suma[:]) {
		t.Fatalf("sha256 = %s, el archivo da %s", r.SHA256, hex.EncodeToString(suma[:]))
	}
	if r.Personas != 2 || r.Formas != 4 || r.Perfiles != 2 {
		t.Fatalf("conteos = %d personas, %d formas, %d perfiles; quiero 2, 4, 2",
			r.Personas, r.Formas, r.Perfiles)
	}
	// Un respaldo autocontenido no arrastra el WAL de la base viva.
	if _, err := os.Stat(r.Ruta + "-wal"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("el respaldo dejó un WAL al lado: %v", err)
	}
}

// La bóveda nunca se versiona: un destino dentro de un árbol git se rechaza
// antes de escribir nada.
func TestRespaldaRechazaDestinoDentroDeGit(t *testing.T) {
	raiz := t.TempDir()
	datos := filepath.Join(raiz, "datos")
	bovedaDePrueba(t, datos, personaInventada("p_cl_90001", "Ana Pérez Rojas", "ana.perez@example.test"))
	repo := filepath.Join(raiz, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o750); err != nil {
		t.Fatalf("armar el repo: %v", err)
	}
	destino := filepath.Join(repo, "respaldos", "hoy")

	if _, err := respaldo.Respalda(datos, destino, ahoraFijo); !errors.Is(err, respaldo.ErrDestinoEnGit) {
		t.Fatalf("err = %v, quiero ErrDestinoEnGit", err)
	}
	if _, err := os.Stat(destino); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("creó %s dentro del repo: %v", destino, err)
	}
}

// La bóveda real de este repo también se rechaza: el guardia no depende del
// nombre del directorio, sino de encontrar el .git hacia arriba.
func TestRespaldaRechazaElRepoDeEsteProyecto(t *testing.T) {
	datos := filepath.Join(t.TempDir(), "datos")
	bovedaDePrueba(t, datos, personaInventada("p_cl_90001", "Ana Pérez Rojas", "ana.perez@example.test"))
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := respaldo.Respalda(datos, filepath.Join(repo, "tmp", "respaldos"), ahoraFijo); !errors.Is(err, respaldo.ErrDestinoEnGit) {
		t.Fatalf("err = %v, quiero ErrDestinoEnGit", err)
	}
}

// Restaurar reproduce la bóveda —personas, formas y perfiles— en un directorio
// nuevo, y no pisa una bóveda que ya existe.
func TestRestauraReproduceLaBovedaEnUnDirectorioNuevo(t *testing.T) {
	raiz := t.TempDir()
	datos := filepath.Join(raiz, "datos")
	quiero := []modelo.Persona{
		personaInventada("p_cl_90001", "Ana Pérez Rojas", "ana.perez@example.test"),
		personaInventada("p_cl_90002", "Luis Santander Mora", "luis.santander@example.test"),
	}
	original := bovedaDePrueba(t, datos, quiero...)
	r, err := respaldo.Respalda(datos, filepath.Join(raiz, "respaldos"), ahoraFijo)
	if err != nil {
		t.Fatalf("Respalda: %v", err)
	}
	if err := original.Cerrar(); err != nil {
		t.Fatalf("cerrar la bóveda original: %v", err)
	}

	nueva := filepath.Join(raiz, "boveda-restaurada")
	res, err := respaldo.Restaura(r.Ruta, nueva)
	if err != nil {
		t.Fatalf("Restaura: %v", err)
	}
	if quieroRuta := filepath.Join(nueva, "almacen.db"); res.Ruta != quieroRuta {
		t.Fatalf("ruta = %q, quiero %q", res.Ruta, quieroRuta)
	}
	if res.SHA256 != r.SHA256 || res.Personas != 2 || res.Perfiles != 2 {
		t.Fatalf("restauración = %+v, quiero el mismo sha256 y 2 personas con 2 perfiles", res)
	}
	info, err := os.Stat(res.Ruta)
	if err != nil {
		t.Fatalf("stat de la bóveda restaurada: %v", err)
	}
	if modo := info.Mode().Perm(); modo != 0o600 {
		t.Fatalf("permisos = %o, quiero 600: la bóveda lleva datos reales", modo)
	}
	restaurada, err := almacen.Open(res.Ruta)
	if err != nil {
		t.Fatalf("abrir la bóveda restaurada: %v", err)
	}
	defer restaurada.Cerrar()
	for _, p := range quiero {
		guardada, existe, err := restaurada.Persona(context.Background(), p.Llave)
		if err != nil || !existe {
			t.Fatalf("persona %s tras restaurar: existe=%v err=%v", p.Llave, existe, err)
		}
		if len(guardada.Formas) != len(p.Formas) {
			t.Fatalf("formas de %s = %d, quiero %d", p.Llave, len(guardada.Formas), len(p.Formas))
		}
	}
	if _, existe, err := restaurada.Perfil(context.Background(), quiero[0].Llave); err != nil || !existe {
		t.Fatalf("perfil tras restaurar: existe=%v err=%v", existe, err)
	}

	// Una segunda restauración en el mismo directorio pisaría la bóveda en uso.
	if _, err := respaldo.Restaura(r.Ruta, nueva); !errors.Is(err, respaldo.ErrBovedaEnUso) {
		t.Fatalf("err = %v, quiero ErrBovedaEnUso", err)
	}
}

// Un archivo que no es una bóveda no se restaura: mejor fallar que dejar una
// base a medias en el volumen.
func TestRestauraRechazaUnArchivoQueNoEsBoveda(t *testing.T) {
	raiz := t.TempDir()
	falso := filepath.Join(raiz, "cualquiera.db")
	if err := os.WriteFile(falso, []byte("esto no es una base\n"), 0o600); err != nil {
		t.Fatalf("escribir el falso: %v", err)
	}
	if _, err := respaldo.Restaura(falso, filepath.Join(raiz, "datos")); !errors.Is(err, respaldo.ErrNoEsBoveda) {
		t.Fatalf("err = %v, quiero ErrNoEsBoveda", err)
	}
	if _, err := os.Stat(filepath.Join(raiz, "datos", "almacen.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dejó una bóveda a medias: %v", err)
	}
}

// Un archivo con WAL al lado no es un respaldo autocontenido: copiarlo perdería
// lo que vive en el WAL.
func TestRestauraRechazaUnRespaldoConArchivosAuxiliares(t *testing.T) {
	raiz := t.TempDir()
	datos := filepath.Join(raiz, "datos")
	bovedaDePrueba(t, datos, personaInventada("p_cl_90001", "Ana Pérez Rojas", "ana.perez@example.test"))
	r, err := respaldo.Respalda(datos, filepath.Join(raiz, "respaldos"), ahoraFijo)
	if err != nil {
		t.Fatalf("Respalda: %v", err)
	}
	if err := os.WriteFile(r.Ruta+"-wal", []byte("sobras"), 0o600); err != nil {
		t.Fatalf("escribir el WAL: %v", err)
	}
	if _, err := respaldo.Restaura(r.Ruta, filepath.Join(raiz, "otra")); !errors.Is(err, respaldo.ErrRespaldoEnUso) {
		t.Fatalf("err = %v, quiero ErrRespaldoEnUso", err)
	}
}
