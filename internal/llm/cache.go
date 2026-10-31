package llm

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// Cache guarda respuestas por hash de entrada (D15, D32). La implementa la
// tabla cache_modelo del almacén SQLite de /data (C2); CacheSQLite es la versión
// autónoma sobre la misma tabla. Un fallo de caché nunca tumba una decisión: el
// cliente lo trata como un fallo de lectura.
type Cache interface {
	// Lee devuelve el valor guardado. false si la llave no está.
	Lee(ctx context.Context, llave string) (string, bool, error)
	// Guarda agrega el valor; si la llave existe, no la sobrescribe.
	Guarda(ctx context.Context, llave string, valor string) error
}

// CacheSQLite es la caché persistente: sobrevive al reinicio del servicio, así
// que la corrida 2 no vuelve a pagar una decisión de la corrida 1.
type CacheSQLite struct {
	db *sql.DB
}

// NuevaCacheSQLite prepara la tabla cache_modelo en la base dada.
func NuevaCacheSQLite(db *sql.DB) (*CacheSQLite, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cache_modelo (
		llave TEXT PRIMARY KEY,
		respuesta TEXT NOT NULL,
		creada_en TEXT NOT NULL
	)`); err != nil {
		return nil, err
	}
	return &CacheSQLite{db: db}, nil
}

// Lee una respuesta guardada.
func (c *CacheSQLite) Lee(ctx context.Context, llave string) (string, bool, error) {
	var valor string
	err := c.db.QueryRowContext(ctx, `SELECT respuesta FROM cache_modelo WHERE llave = ?`, llave).Scan(&valor)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return valor, true, nil
}

// Guarda una respuesta nueva sin pisar la existente: la misma llave implica la
// misma entrada y la misma decisión.
func (c *CacheSQLite) Guarda(ctx context.Context, llave string, valor string) error {
	_, err := c.db.ExecContext(ctx,
		`INSERT INTO cache_modelo (llave, respuesta, creada_en) VALUES (?, ?, ?)
		 ON CONFLICT(llave) DO NOTHING`,
		llave, valor, time.Now().UTC().Format(time.RFC3339))
	return err
}

// CacheMemoria es una caché en memoria para pruebas y para el respaldo sin
// disco. No persiste entre corridas.
type CacheMemoria struct {
	mu      sync.Mutex
	valores map[string]string
}

// NuevaCacheMemoria arma una caché vacía.
func NuevaCacheMemoria() *CacheMemoria {
	return &CacheMemoria{valores: map[string]string{}}
}

// Lee un valor de memoria.
func (c *CacheMemoria) Lee(_ context.Context, llave string) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	valor, hay := c.valores[llave]
	return valor, hay, nil
}

// Guarda un valor en memoria sin pisar el existente.
func (c *CacheMemoria) Guarda(_ context.Context, llave string, valor string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, hay := c.valores[llave]; !hay {
		c.valores[llave] = valor
	}
	return nil
}
