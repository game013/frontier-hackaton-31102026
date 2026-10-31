// Package respaldo copia la bóveda —registro de personas y perfiles— a un disco
// local fuera de git y la devuelve al volumen cuando haga falta (C24, D4).
//
// La bóveda vive en el volumen /data y es, con entities.json, el único lugar con
// datos reales. Si el volumen se pierde, los seudónimos cambian; por eso el
// respaldo se toma con la API de copia de SQLite (`VACUUM INTO`) sobre la base en
// uso y nunca copiando el .db a pelo: el WAL quedaría afuera.
package respaldo

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// archivoBoveda es el nombre con el que internal/almacen abre la base dentro de
// la carpeta --datos.
const archivoBoveda = "almacen.db"

// esperaEscritura acota la espera por la base en uso, en milisegundos: el
// respaldo puede correr mientras el pipeline escribe (D12).
const esperaEscritura = 5000

// Errores que el operador necesita distinguir para corregir el destino.
var (
	// ErrDestinoEnGit: el respaldo caería dentro de un árbol git.
	ErrDestinoEnGit = errors.New("respaldo: el destino está dentro del repo")
	// ErrBovedaEnUso: la restauración pisaría una bóveda existente.
	ErrBovedaEnUso = errors.New("respaldo: la bóveda destino ya existe y no se pisa")
	// ErrNoEsBoveda: el archivo no es una bóveda SQLite sana.
	ErrNoEsBoveda = errors.New("respaldo: el archivo no es una bóveda")
	// ErrRespaldoEnUso: el respaldo trae WAL o journal al lado, así que no es
	// autocontenido y copiarlo perdería escrituras.
	ErrRespaldoEnUso = errors.New("respaldo: el respaldo tiene archivos auxiliares")
)

// Respaldo es el archivo que dejó Respalda, con lo necesario para informarlo sin
// enseñar datos: ruta, tamaño, hash y conteos.
type Respaldo struct {
	Ruta     string
	Bytes    int64
	SHA256   string
	Personas int
	Formas   int
	Perfiles int
}

// Restauracion es la bóveda que dejó Restaura en <datos>/almacen.db.
type Restauracion struct {
	Ruta     string
	Bytes    int64
	SHA256   string
	Personas int
	Formas   int
	Perfiles int
}

// Respalda copia la bóveda de la carpeta datos a un archivo nuevo en destino, con
// marca de tiempo UTC en el nombre. La copia es coherente aunque el pipeline
// esté escribiendo: SQLite la arma dentro de una transacción de lectura.
func Respalda(datos, destino string, ahora time.Time) (Respaldo, error) {
	origen, err := filepath.Abs(filepath.Join(datos, archivoBoveda))
	if err != nil {
		return Respaldo{}, fmt.Errorf("respaldo: ruta de origen %q: %w", datos, err)
	}
	if _, err := os.Stat(origen); err != nil {
		return Respaldo{}, fmt.Errorf("respaldo: no hay bóveda en %s: %w", origen, err)
	}
	dir, err := filepath.Abs(destino)
	if err != nil {
		return Respaldo{}, fmt.Errorf("respaldo: destino %q: %w", destino, err)
	}
	if raiz := raizGit(dir); raiz != "" {
		return Respaldo{}, fmt.Errorf("%w: %s está dentro de %s; la bóveda lleva datos reales, elige un disco local fuera del repo", ErrDestinoEnGit, dir, raiz)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Respaldo{}, fmt.Errorf("respaldo: crear %s: %w", dir, err)
	}
	ruta, err := nombreLibre(dir, ahora)
	if err != nil {
		return Respaldo{}, err
	}

	if err := copiaCoherente(origen, ruta); err != nil {
		os.Remove(ruta)
		return Respaldo{}, err
	}
	copia, err := examina(ruta)
	if err != nil {
		os.Remove(ruta)
		return Respaldo{}, err
	}
	return Respaldo(copia), nil
}

// Restaura deja el respaldo en <datos>/almacen.db. La bóveda destino tiene que
// ser nueva: pisar una base en uso cambiaría los seudónimos a mitad de corrida.
func Restaura(archivo, datos string) (Restauracion, error) {
	respaldo, err := filepath.Abs(archivo)
	if err != nil {
		return Restauracion{}, fmt.Errorf("respaldo: archivo %q: %w", archivo, err)
	}
	if _, err := os.Stat(respaldo); err != nil {
		return Restauracion{}, fmt.Errorf("respaldo: no encuentro %s: %w", respaldo, err)
	}
	for _, sufijo := range []string{"-wal", "-journal"} {
		if _, err := os.Stat(respaldo + sufijo); err == nil {
			return Restauracion{}, fmt.Errorf("%w: %s%s; ciérralo y vuelve a respaldar con la API de copia", ErrRespaldoEnUso, respaldo, sufijo)
		}
	}
	dir, err := filepath.Abs(datos)
	if err != nil {
		return Restauracion{}, fmt.Errorf("respaldo: datos %q: %w", datos, err)
	}
	destino := filepath.Join(dir, archivoBoveda)
	if _, err := os.Stat(destino); err == nil {
		return Restauracion{}, fmt.Errorf("%w: %s; restaura en una carpeta nueva", ErrBovedaEnUso, destino)
	}
	if _, err := examina(respaldo); err != nil {
		return Restauracion{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Restauracion{}, fmt.Errorf("respaldo: crear %s: %w", dir, err)
	}
	if err := copiaArchivo(respaldo, destino); err != nil {
		return Restauracion{}, err
	}
	copia, err := examina(destino)
	if err != nil {
		os.Remove(destino)
		return Restauracion{}, err
	}
	return Restauracion(copia), nil
}

// copiaCoherente arma el respaldo con VACUUM INTO: SQLite escribe una base
// completa y consistente en el archivo nuevo, sin copiar el WAL ni sus páginas
// libres, y la deja cerrada.
func copiaCoherente(origen, destino string) error {
	db, err := abre(origen, "ro")
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec("VACUUM INTO ?", destino); err != nil {
		return fmt.Errorf("respaldo: copiar %s: %w", origen, err)
	}
	// VACUUM INTO crea el archivo con el umask del proceso; la bóveda lleva
	// datos reales y solo la lee su dueño.
	if err := os.Chmod(destino, 0o600); err != nil {
		return fmt.Errorf("respaldo: permisos de %s: %w", destino, err)
	}
	return nil
}

// copiaArchivo deja el respaldo en destino de una sola vez: escribe un temporal
// al lado y lo renombra, así el volumen nunca ve una base a medias.
func copiaArchivo(origen, destino string) error {
	entrada, err := os.Open(origen)
	if err != nil {
		return fmt.Errorf("respaldo: abrir %s: %w", origen, err)
	}
	defer entrada.Close()
	temporal := destino + ".parcial"
	salida, err := os.OpenFile(temporal, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("respaldo: crear %s: %w", temporal, err)
	}
	if _, err := io.Copy(salida, entrada); err != nil {
		salida.Close()
		os.Remove(temporal)
		return fmt.Errorf("respaldo: escribir %s: %w", temporal, err)
	}
	if err := salida.Sync(); err != nil {
		salida.Close()
		os.Remove(temporal)
		return fmt.Errorf("respaldo: sincronizar %s: %w", temporal, err)
	}
	if err := salida.Close(); err != nil {
		os.Remove(temporal)
		return fmt.Errorf("respaldo: cerrar %s: %w", temporal, err)
	}
	if err := os.Rename(temporal, destino); err != nil {
		os.Remove(temporal)
		return fmt.Errorf("respaldo: dejar %s: %w", destino, err)
	}
	return nil
}

// examina abre la base en solo lectura y devuelve su tamaño, su hash y sus
// conteos. Una base ilegible o sin las tablas de la bóveda es ErrNoEsBoveda.
func examina(ruta string) (Restauracion, error) {
	info, err := os.Stat(ruta)
	if err != nil {
		return Restauracion{}, fmt.Errorf("respaldo: %s: %w", ruta, err)
	}
	suma, err := hashDe(ruta)
	if err != nil {
		return Restauracion{}, err
	}
	db, err := abre(ruta, "ro")
	if err != nil {
		return Restauracion{}, fmt.Errorf("%w: %s: %w", ErrNoEsBoveda, ruta, err)
	}
	defer db.Close()

	var integridad string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integridad); err != nil {
		return Restauracion{}, fmt.Errorf("%w: %s: %w", ErrNoEsBoveda, ruta, err)
	}
	if integridad != "ok" {
		return Restauracion{}, fmt.Errorf("%w: %s no pasa integrity_check: %s", ErrNoEsBoveda, ruta, integridad)
	}
	out := Restauracion{Ruta: ruta, Bytes: info.Size(), SHA256: suma}
	err = db.QueryRow(`SELECT (SELECT COUNT(*) FROM personas),
       (SELECT COUNT(*) FROM formas),
       (SELECT COUNT(*) FROM perfiles)`).Scan(&out.Personas, &out.Formas, &out.Perfiles)
	if err != nil {
		return Restauracion{}, fmt.Errorf("%w: %s: %w", ErrNoEsBoveda, ruta, err)
	}
	return out, nil
}

// abre conecta con la base en ruta. modo es "ro" o "rw"; el respaldo lee, así
// que no necesita escribir en el volumen.
func abre(ruta, modo string) (*sql.DB, error) {
	uri := url.URL{Scheme: "file", Path: ruta}
	q := url.Values{}
	q.Set("mode", modo)
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", esperaEscritura))
	uri.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, fmt.Errorf("respaldo: abrir %s: %w", ruta, err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// hashDe devuelve el sha256 del archivo, para que el operador pueda comprobar
// que la copia que se llevó al disco es la que se restauró.
func hashDe(ruta string) (string, error) {
	archivo, err := os.Open(ruta)
	if err != nil {
		return "", fmt.Errorf("respaldo: abrir %s: %w", ruta, err)
	}
	defer archivo.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, archivo); err != nil {
		return "", fmt.Errorf("respaldo: leer %s: %w", ruta, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// nombreLibre arma el nombre del respaldo con la marca de tiempo UTC y no pisa
// uno anterior: dos respaldos en el mismo segundo se numeran.
func nombreLibre(dir string, ahora time.Time) (string, error) {
	base := "boveda-" + ahora.UTC().Format("20060102T150405Z")
	for i := 1; i <= 99; i++ {
		nombre := base + ".db"
		if i > 1 {
			nombre = fmt.Sprintf("%s-%d.db", base, i)
		}
		ruta := filepath.Join(dir, nombre)
		if _, err := os.Stat(ruta); errors.Is(err, os.ErrNotExist) {
			return ruta, nil
		}
	}
	return "", fmt.Errorf("respaldo: %s ya tiene 99 respaldos de %s", dir, base)
}

// raizGit sube desde ruta hasta encontrar un .git y devuelve ese directorio; ""
// si ruta no está dentro de un árbol git.
func raizGit(ruta string) string {
	dir := filepath.Clean(ruta)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			return ""
		}
		dir = padre
	}
}
