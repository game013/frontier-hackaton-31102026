package almacen

import "github.com/game013/frontier-hackaton-31102026/internal/llm"

// CacheModelo es la caché del cliente de Jev sobre la tabla cache_modelo de
// esta base (D32): vive en /data, así que la corrida 2 no vuelve a pagar ni a
// esperar una decisión de la corrida 1. Comparte la conexión única del almacén.
func (a *Almacen) CacheModelo() (*llm.CacheSQLite, error) {
	return llm.NuevaCacheSQLite(a.db)
}
