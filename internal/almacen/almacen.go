// Package almacen guarda en SQLite el registro de personas y la bóveda de
// perfiles (D4, D12) junto con lo que el servicio necesita entre reinicios:
// jobs, personas de /anonymize (D24), caché del modelo, cola de dudosos (D20) y
// captura de requests.
//
// La base vive en el volumen /data. Es, con entities.json, el único lugar donde
// se aceptan datos reales, y nunca se versiona. Un perfil asignado es para
// siempre: GuardarPerfil solo agrega.
package almacen

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	_ "modernc.org/sqlite"
)

// esperaEscritura es el busy_timeout en milisegundos: /anonymize escribe
// mientras corre /run y ninguno de los dos puede perder la vez (D12).
const esperaEscritura = 5000

// Almacen es la persistencia del pipeline en SQLite. Cumple modelo.Almacen.
// Una sola conexión serializa las escrituras; no necesita mutex propio.
type Almacen struct {
	db *sql.DB
}

var _ modelo.Almacen = (*Almacen)(nil)

// Open abre la base en ruta (un archivo, no un DSN) y le aplica las migraciones
// pendientes. El archivo se crea con permisos privados y un archivo existente
// nunca se trunca.
func Open(ruta string) (*Almacen, error) {
	if ruta == "" {
		return nil, errors.New("almacen: ruta vacía")
	}
	abs, err := filepath.Abs(ruta)
	if err != nil {
		return nil, fmt.Errorf("almacen: ruta %q: %w", ruta, err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return nil, fmt.Errorf("almacen: crear directorio de %q: %w", abs, err)
	}
	archivo, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("almacen: crear %q: %w", abs, err)
	}
	if err := archivo.Close(); err != nil {
		return nil, fmt.Errorf("almacen: crear %q: %w", abs, err)
	}

	uri := url.URL{Scheme: "file", Path: abs}
	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", esperaEscritura))
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Set("_txlock", "immediate")
	uri.RawQuery = q.Encode()

	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, fmt.Errorf("almacen: abrir %q: %w", abs, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	a := &Almacen{db: db}
	if err := a.migrar(); err != nil {
		db.Close()
		return nil, fmt.Errorf("almacen: abrir %q: %w", abs, err)
	}
	return a, nil
}

// Cerrar libera la base y su WAL: después del cierre, el archivo queda legible
// en solo lectura y sin auxiliares colgados.
func (a *Almacen) Cerrar() error {
	if a == nil || a.db == nil {
		return nil
	}
	return a.db.Close()
}

// migrar lleva la base a versionEsquema dentro de una transacción: si una
// migración falla, la base queda como estaba.
func (a *Almacen) migrar() error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("leer user_version: %w", err)
	}
	switch version {
	case 0:
		if _, err := tx.Exec(esquemaV1); err != nil {
			return fmt.Errorf("aplicar esquema v1: %w", err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", versionEsquema)); err != nil {
			return fmt.Errorf("marcar versión %d: %w", versionEsquema, err)
		}
	case versionEsquema:
	default:
		return fmt.Errorf("versión de esquema desconocida: %d", version)
	}
	return tx.Commit()
}
