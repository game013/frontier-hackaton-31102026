package almacen

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// ErrPerfilYaAsignado avisa que la bóveda ya tenía perfil para esa llave. Es la
// señal de "no hubo efecto": el perfil guardado no cambia (D4).
var ErrPerfilYaAsignado = errors.New("almacen: la persona ya tiene perfil")

// Persona devuelve una persona del registro del corpus con sus formas, en el
// orden en que se descubrieron. existe es false si la llave no está.
func (a *Almacen) Persona(ctx context.Context, llave string) (modelo.Persona, bool, error) {
	var p modelo.Persona
	err := a.db.QueryRowContext(ctx,
		"SELECT llave, rol, genero FROM personas WHERE llave = ?", llave).
		Scan(&p.Llave, &p.Rol, &p.Genero)
	if errors.Is(err, sql.ErrNoRows) {
		return modelo.Persona{}, false, nil
	}
	if err != nil {
		return modelo.Persona{}, false, fmt.Errorf("almacen: leer persona %q: %w", llave, err)
	}
	formas, err := a.formas(ctx, llave)
	if err != nil {
		return modelo.Persona{}, false, err
	}
	p.Formas = formas
	return p, true, nil
}

// GuardarPersona agrega una persona; si la llave ya existe, solo agrega las
// formas nuevas. Rol y género se fijan con la primera escritura: el registro es
// de solo agregar.
func (a *Almacen) GuardarPersona(ctx context.Context, p modelo.Persona) error {
	if p.Llave == "" {
		return errors.New("almacen: persona sin llave")
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("almacen: guardar persona %q: %w", p.Llave, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO personas (llave, rol, genero, creada) VALUES (?, ?, ?, ?)
ON CONFLICT (llave) DO NOTHING`, p.Llave, p.Rol, p.Genero, ahora()); err != nil {
		return fmt.Errorf("almacen: guardar persona %q: %w", p.Llave, err)
	}
	for _, forma := range p.Formas {
		if forma.Tipo == "" || forma.Valor == "" {
			return fmt.Errorf("almacen: persona %q con forma vacía: %+v", p.Llave, forma)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO formas (llave, vivo, tipo, valor, registro) VALUES (?, 0, ?, ?, ?)
ON CONFLICT (llave, vivo, tipo, valor) DO NOTHING`,
			p.Llave, forma.Tipo, forma.Valor, forma.Registro); err != nil {
			return fmt.Errorf("almacen: guardar forma de %q: %w", p.Llave, err)
		}
	}
	return tx.Commit()
}

// Perfil devuelve el perfil de la bóveda. existe es false si la persona aún no
// tiene uno.
func (a *Almacen) Perfil(ctx context.Context, llave string) (modelo.Perfil, bool, error) {
	var p modelo.Perfil
	err := a.db.QueryRowContext(ctx,
		`SELECT nombre, correo, celular, cedula, handle, direccion, nacimiento, placa, cuenta, pasaporte
FROM perfiles WHERE llave = ?`, llave).
		Scan(&p.Nombre, &p.Correo, &p.Celular, &p.Cedula, &p.Handle,
			&p.Direccion, &p.Nacimiento, &p.Placa, &p.Cuenta, &p.Pasaporte)
	if errors.Is(err, sql.ErrNoRows) {
		return modelo.Perfil{}, false, nil
	}
	if err != nil {
		return modelo.Perfil{}, false, fmt.Errorf("almacen: leer perfil %q: %w", llave, err)
	}
	return p, true, nil
}

// GuardarPerfil asigna el perfil de una persona que aún no lo tiene. Nunca
// sobrescribe: si ya había uno, devuelve ErrPerfilYaAsignado y no toca nada.
func (a *Almacen) GuardarPerfil(ctx context.Context, llave string, p modelo.Perfil) error {
	if llave == "" {
		return errors.New("almacen: perfil sin llave")
	}
	if p.Nombre == "" {
		return fmt.Errorf("almacen: perfil de %q sin nombre", llave)
	}
	res, err := a.db.ExecContext(ctx,
		`INSERT INTO perfiles (llave, nombre, correo, celular, cedula, handle, direccion, nacimiento, placa, cuenta, pasaporte, asignado)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (llave) DO NOTHING`,
		llave, p.Nombre, p.Correo, p.Celular, p.Cedula, p.Handle,
		p.Direccion, p.Nacimiento, p.Placa, p.Cuenta, p.Pasaporte, ahora())
	if err != nil {
		return fmt.Errorf("almacen: guardar perfil %q: %w", llave, err)
	}
	filas, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("almacen: guardar perfil %q: %w", llave, err)
	}
	if filas == 0 {
		return fmt.Errorf("almacen: guardar perfil %q: %w", llave, ErrPerfilYaAsignado)
	}
	return nil
}

// Perfiles devuelve la bóveda completa indexada por llave. La fábrica la lee
// para que dos personas nunca compartan seudónimo ni datos falsos (D4); con
// pocos cientos de filas traerla entera es más barato que consultar por fila.
func (a *Almacen) Perfiles(ctx context.Context) (map[string]modelo.Perfil, error) {
	filas, err := a.db.QueryContext(ctx,
		`SELECT llave, nombre, correo, celular, cedula, handle, direccion, nacimiento, placa, cuenta, pasaporte
FROM perfiles ORDER BY llave`)
	if err != nil {
		return nil, fmt.Errorf("almacen: leer la bóveda: %w", err)
	}
	defer filas.Close()

	perfiles := make(map[string]modelo.Perfil)
	for filas.Next() {
		var llave string
		var p modelo.Perfil
		if err := filas.Scan(&llave, &p.Nombre, &p.Correo, &p.Celular, &p.Cedula, &p.Handle,
			&p.Direccion, &p.Nacimiento, &p.Placa, &p.Cuenta, &p.Pasaporte); err != nil {
			return nil, fmt.Errorf("almacen: leer la bóveda: %w", err)
		}
		perfiles[llave] = p
	}
	if err := filas.Err(); err != nil {
		return nil, fmt.Errorf("almacen: leer la bóveda: %w", err)
	}
	return perfiles, nil
}

// formas devuelve las formas del corpus de una persona, en orden de
// descubrimiento. Las de personas_vivo (vivo = 1) no se mezclan (D24).
func (a *Almacen) formas(ctx context.Context, llave string) ([]modelo.Forma, error) {
	filas, err := a.db.QueryContext(ctx,
		"SELECT tipo, valor, registro FROM formas WHERE llave = ? AND vivo = 0 ORDER BY id", llave)
	if err != nil {
		return nil, fmt.Errorf("almacen: leer formas de %q: %w", llave, err)
	}
	defer filas.Close()

	var formas []modelo.Forma
	for filas.Next() {
		var f modelo.Forma
		if err := filas.Scan(&f.Tipo, &f.Valor, &f.Registro); err != nil {
			return nil, fmt.Errorf("almacen: leer formas de %q: %w", llave, err)
		}
		formas = append(formas, f)
	}
	if err := filas.Err(); err != nil {
		return nil, fmt.Errorf("almacen: leer formas de %q: %w", llave, err)
	}
	return formas, nil
}

// ahora es la marca de tiempo de las filas: RFC 3339 en UTC.
func ahora() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
