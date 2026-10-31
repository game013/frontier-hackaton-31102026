package almacen

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// abrir devuelve un almacén sobre una ruta temporal y lo cierra al final.
func abrir(t *testing.T, ruta string) *Almacen {
	t.Helper()
	a, err := Open(ruta)
	if err != nil {
		t.Fatalf("Open(%s): %v", ruta, err)
	}
	t.Cleanup(func() { a.Cerrar() })
	return a
}

// personaDePrueba es una persona inventada: ninguna forma coincide con el
// corpus real.
func personaDePrueba(llave string) modelo.Persona {
	return modelo.Persona{
		Llave:  llave,
		Rol:    modelo.RolCliente,
		Genero: modelo.GeneroF,
		Formas: []modelo.Forma{
			{Tipo: modelo.FormaNombreCompleto, Valor: "Ana Pérez Rojas", Registro: "chat_001"},
			{Tipo: modelo.FormaCorreo, Valor: "ana.perez@example.test", Registro: "tabla_clientes:001"},
		},
	}
}

func perfilDePrueba(nombre string) modelo.Perfil {
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

// La base guarda la bóveda y el registro de personas: cerrar y abrir la misma
// ruta tiene que devolver exactamente lo mismo (D4, D12).
func TestReaperturaConservaPersonasYPerfiles(t *testing.T) {
	ctx := context.Background()
	ruta := filepath.Join(t.TempDir(), "datos", "boveda.db")

	a := abrir(t, ruta)
	quieroPersona := personaDePrueba("p_cl_1")
	quieroPerfil := perfilDePrueba("Marta Gómez Díaz")
	if err := a.GuardarPersona(ctx, quieroPersona); err != nil {
		t.Fatalf("guardar persona: %v", err)
	}
	if err := a.GuardarPerfil(ctx, quieroPersona.Llave, quieroPerfil); err != nil {
		t.Fatalf("guardar perfil: %v", err)
	}
	if err := a.Cerrar(); err != nil {
		t.Fatalf("cerrar: %v", err)
	}

	b := abrir(t, ruta)
	persona, existe, err := b.Persona(ctx, quieroPersona.Llave)
	if err != nil || !existe {
		t.Fatalf("persona tras reabrir: existe=%v err=%v", existe, err)
	}
	if persona.Llave != quieroPersona.Llave || persona.Rol != quieroPersona.Rol || persona.Genero != quieroPersona.Genero {
		t.Fatalf("persona distinta tras reabrir: %+v", persona)
	}
	if len(persona.Formas) != len(quieroPersona.Formas) {
		t.Fatalf("formas %d, se esperaban %d", len(persona.Formas), len(quieroPersona.Formas))
	}
	for i, forma := range quieroPersona.Formas {
		if persona.Formas[i] != forma {
			t.Fatalf("forma %d: %+v, se esperaba %+v", i, persona.Formas[i], forma)
		}
	}
	perfil, existe, err := b.Perfil(ctx, quieroPersona.Llave)
	if err != nil || !existe {
		t.Fatalf("perfil tras reabrir: existe=%v err=%v", existe, err)
	}
	if perfil != quieroPerfil {
		t.Fatalf("perfil distinto tras reabrir:\nobtenido: %+v\nesperado: %+v", perfil, quieroPerfil)
	}
}

// Cerrar deja la base legible en solo lectura y sin los auxiliares de WAL: el
// volumen de Railway se respalda tal cual (C24).
func TestCerrarDejaLaBaseEnSoloLecturaSinAuxiliares(t *testing.T) {
	ctx := context.Background()
	ruta := filepath.Join(t.TempDir(), "boveda.db")

	a := abrir(t, ruta)
	if err := a.GuardarPerfil(ctx, "p_cl_2", perfilDePrueba("Marta Gómez Díaz")); err != nil {
		t.Fatalf("guardar perfil: %v", err)
	}
	if err := a.Cerrar(); err != nil {
		t.Fatalf("cerrar: %v", err)
	}

	for _, sufijo := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(ruta + sufijo); !os.IsNotExist(err) {
			t.Fatalf("%s quedó en disco: %v", sufijo, err)
		}
	}

	uri := url.URL{Scheme: "file", Path: ruta}
	q := url.Values{}
	q.Set("mode", "ro")
	uri.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		t.Fatalf("abrir en solo lectura: %v", err)
	}
	defer db.Close()
	var nombre string
	if err := db.QueryRow("SELECT nombre FROM perfiles WHERE llave = 'p_cl_2'").Scan(&nombre); err != nil {
		t.Fatalf("leer en solo lectura: %v", err)
	}
	if nombre != "Marta Gómez Díaz" {
		t.Fatalf("nombre %q en solo lectura", nombre)
	}
	if _, err := db.Exec("INSERT INTO jobs (id, estado, creado) VALUES ('job_ro', 'queued', '2026-01-01T00:00:00Z')"); err == nil {
		t.Fatal("la base en solo lectura aceptó una escritura")
	}
}

// El esquema es el contrato de las otras tareas: las ocho tablas existen y la
// versión queda marcada para que la migración sea idempotente.
func TestEsquemaTieneLasTablasDelContrato(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "boveda.db")
	a := abrir(t, ruta)

	filas, err := a.db.Query("SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
	if err != nil {
		t.Fatalf("listar tablas: %v", err)
	}
	defer filas.Close()
	tablas := map[string]bool{}
	for filas.Next() {
		var nombre string
		if err := filas.Scan(&nombre); err != nil {
			t.Fatalf("leer tabla: %v", err)
		}
		tablas[nombre] = true
	}
	if err := filas.Err(); err != nil {
		t.Fatalf("recorrer tablas: %v", err)
	}
	for _, quiero := range []string{
		"jobs", "personas", "formas", "perfiles", "personas_vivo", "cache_modelo", "dudosos", "captura",
	} {
		if !tablas[quiero] {
			t.Fatalf("falta la tabla %q; hay %v", quiero, tablas)
		}
	}
	var version int
	if err := a.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("leer user_version: %v", err)
	}
	if version != versionEsquema {
		t.Fatalf("user_version %d, se esperaba %d", version, versionEsquema)
	}
}

// Un arranque sobre una base ya migrada no vuelve a aplicar el esquema.
func TestAbrirDosVecesNoReaplicaLaMigracion(t *testing.T) {
	ctx := context.Background()
	ruta := filepath.Join(t.TempDir(), "boveda.db")
	a := abrir(t, ruta)
	if err := a.GuardarPersona(ctx, personaDePrueba("p_cl_3")); err != nil {
		t.Fatalf("guardar persona: %v", err)
	}
	if err := a.Cerrar(); err != nil {
		t.Fatalf("cerrar: %v", err)
	}
	b := abrir(t, ruta)
	if _, existe, err := b.Persona(ctx, "p_cl_3"); err != nil || !existe {
		t.Fatalf("la migración perdió el registro: existe=%v err=%v", existe, err)
	}
}

// Una base de una versión futura no se toca: mejor fallar al arrancar que
// corromper la bóveda.
func TestOpenRechazaVersionDesconocida(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "futura.db")
	db, err := sql.Open("sqlite", ruta)
	if err != nil {
		t.Fatalf("crear base: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 99; CREATE TABLE sobra (id TEXT)"); err != nil {
		db.Close()
		t.Fatalf("preparar base futura: %v", err)
	}
	db.Close()

	a, err := Open(ruta)
	if err == nil {
		a.Cerrar()
		t.Fatal("se aceptó una base de versión desconocida")
	}
}

// Una migración que falla a mitad no deja la base a medias: el esquema y la
// versión quedan como estaban.
func TestMigracionFallidaNoDejaEsquemaParcial(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "rota.db")
	db, err := sql.Open("sqlite", ruta)
	if err != nil {
		t.Fatalf("crear base: %v", err)
	}
	// Una tabla previa con el nombre de una del esquema hace fallar la migración.
	if _, err := db.Exec("CREATE TABLE personas (llave TEXT PRIMARY KEY)"); err != nil {
		db.Close()
		t.Fatalf("preparar base: %v", err)
	}
	db.Close()

	if a, err := Open(ruta); err == nil {
		a.Cerrar()
		t.Fatal("se migró sobre una tabla en conflicto")
	}

	db, err = sql.Open("sqlite", ruta)
	if err != nil {
		t.Fatalf("reabrir base: %v", err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 0 {
		t.Fatalf("user_version %d tras el fallo: %v", version, err)
	}
	var tablas int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'").Scan(&tablas); err != nil {
		t.Fatalf("contar tablas: %v", err)
	}
	if tablas != 1 {
		t.Fatalf("quedaron %d tablas tras el fallo, se esperaba solo la previa", tablas)
	}
}

func TestOpenRechazaRutaVacia(t *testing.T) {
	if a, err := Open(""); err == nil {
		a.Cerrar()
		t.Fatal("se aceptó una ruta vacía")
	}
}

// WAL y busy_timeout son lo que permite que /anonymize escriba mientras /run
// corre (D12).
func TestPragmasDeArranque(t *testing.T) {
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	casos := []struct {
		pragma string
		quiero string
	}{
		{"PRAGMA journal_mode", "wal"},
		{"PRAGMA busy_timeout", "5000"},
		{"PRAGMA foreign_keys", "1"},
	}
	for _, c := range casos {
		var obtenido string
		if err := a.db.QueryRow(c.pragma).Scan(&obtenido); err != nil {
			t.Fatalf("%s: %v", c.pragma, err)
		}
		if obtenido != c.quiero {
			t.Fatalf("%s = %q, se esperaba %q", c.pragma, obtenido, c.quiero)
		}
	}
}

// Dos escritores simultáneos (el job de /run y un lote de /anonymize) no
// pierden filas ni disparan data races.
func TestEscritoresConcurrentesNoPierdenFilas(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "boveda.db"))
	const escritores = 8
	const porEscritor = 5
	var wg sync.WaitGroup
	errs := make(chan error, escritores*porEscritor)
	for e := 0; e < escritores; e++ {
		wg.Add(1)
		go func(e int) {
			defer wg.Done()
			for i := 0; i < porEscritor; i++ {
				llave := "p_cl_" + string(rune('a'+e)) + string(rune('0'+i))
				if err := a.GuardarPersona(ctx, personaDePrueba(llave)); err != nil {
					errs <- err
					return
				}
				if err := a.GuardarPerfil(ctx, llave, perfilDePrueba("Marta Gómez Díaz")); err != nil {
					errs <- err
					return
				}
			}
		}(e)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("escritura concurrente: %v", err)
	}
	var personas, perfiles int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM personas").Scan(&personas); err != nil {
		t.Fatalf("contar personas: %v", err)
	}
	if err := a.db.QueryRow("SELECT COUNT(*) FROM perfiles").Scan(&perfiles); err != nil {
		t.Fatalf("contar perfiles: %v", err)
	}
	if personas != escritores*porEscritor || perfiles != escritores*porEscritor {
		t.Fatalf("personas %d, perfiles %d; se esperaban %d y %d", personas, perfiles, escritores*porEscritor, escritores*porEscritor)
	}
}

// Las personas de /anonymize van a su tabla y sus formas quedan marcadas: no
// entran al registro del corpus ni a entities.json (D24).
func TestPersonaVivoNoSeMezclaConElCorpus(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "vivo.db"))
	p := modelo.Persona{
		Llave: "p_t_vivo1", Rol: modelo.RolDesconocido, Genero: modelo.GeneroUnisex,
		Formas: []modelo.Forma{{Tipo: modelo.FormaNombreCompleto, Valor: "Ana Prueba Uno", Registro: "m1"}},
	}
	if err := a.GuardarPersonaVivo(ctx, p); err != nil {
		t.Fatalf("GuardarPersonaVivo: %v", err)
	}
	if _, existe, err := a.Persona(ctx, p.Llave); err != nil || existe {
		t.Fatalf("la persona viva entró al registro del corpus (existe=%v, err=%v)", existe, err)
	}
	var vivo int
	if err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM formas WHERE llave = ? AND vivo = 1", p.Llave).Scan(&vivo); err != nil {
		t.Fatalf("cuenta formas vivas: %v", err)
	}
	if vivo != 1 {
		t.Fatalf("formas vivas = %d, quiero 1", vivo)
	}
}

// La cola de dudosos guarda la mención con su regla y confianza (D20).
func TestAnotaDudoso(t *testing.T) {
	ctx := context.Background()
	a := abrir(t, filepath.Join(t.TempDir(), "vivo.db"))
	m := modelo.Mencion{
		Registro: "m1", Fuente: modelo.FuenteChats, Campo: "text", Inicio: 2, Fin: 9,
		Valor: "Ana", Tipo: modelo.MencionPersona, Regla: "residuo:disparador", Confianza: 0.55,
	}
	if err := a.AnotaDudoso(ctx, m); err != nil {
		t.Fatalf("AnotaDudoso: %v", err)
	}
	var regla string
	var confianza float64
	if err := a.db.QueryRowContext(ctx, "SELECT regla, confianza FROM dudosos WHERE registro = 'm1'").Scan(&regla, &confianza); err != nil {
		t.Fatalf("lee dudoso: %v", err)
	}
	if regla != m.Regla || confianza != m.Confianza {
		t.Fatalf("dudoso = %q %v", regla, confianza)
	}
}
