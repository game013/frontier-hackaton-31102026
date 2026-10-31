package llm

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// La caché vive en SQLite y sobrevive a la reapertura: la corrida 2 no vuelve a
// pagar lo que la corrida 1 ya decidió (D15, D32).
func TestCacheSQLiteSobreviveLaReapertura(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "cache.db")
	ctx := context.Background()

	db, err := sql.Open("sqlite", "file:"+ruta)
	if err != nil {
		t.Fatalf("abre: %v", err)
	}
	cache, err := NuevaCacheSQLite(db)
	if err != nil {
		t.Fatalf("NuevaCacheSQLite: %v", err)
	}
	if err := cache.Guarda(ctx, "llave-1", `{"opcion":"person"}`); err != nil {
		t.Fatalf("Guarda: %v", err)
	}
	valor, hay, err := cache.Lee(ctx, "llave-1")
	if err != nil || !hay || valor != `{"opcion":"person"}` {
		t.Fatalf("Lee: %q %v %v", valor, hay, err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("cierra: %v", err)
	}

	db2, err := sql.Open("sqlite", "file:"+ruta)
	if err != nil {
		t.Fatalf("reabre: %v", err)
	}
	defer db2.Close()
	cache2, err := NuevaCacheSQLite(db2)
	if err != nil {
		t.Fatalf("NuevaCacheSQLite: %v", err)
	}
	valor, hay, err = cache2.Lee(ctx, "llave-1")
	if err != nil || !hay || valor != `{"opcion":"person"}` {
		t.Fatalf("tras reabrir: %q %v %v", valor, hay, err)
	}
	if _, hay, err := cache2.Lee(ctx, "no-existe"); err != nil || hay {
		t.Fatalf("llave ausente: %v %v", hay, err)
	}
}

// Guardar dos veces la misma llave no revienta: el mismo hash da la misma
// decisión y la que queda es la primera.
func TestCacheSQLiteNoSobrescribe(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("abre: %v", err)
	}
	defer db.Close()
	cache, err := NuevaCacheSQLite(db)
	if err != nil {
		t.Fatalf("NuevaCacheSQLite: %v", err)
	}
	ctx := context.Background()
	if err := cache.Guarda(ctx, "llave-1", "primera"); err != nil {
		t.Fatalf("Guarda: %v", err)
	}
	if err := cache.Guarda(ctx, "llave-1", "segunda"); err != nil {
		t.Fatalf("Guarda repetida: %v", err)
	}
	valor, _, err := cache.Lee(ctx, "llave-1")
	if err != nil {
		t.Fatalf("Lee: %v", err)
	}
	if valor != "primera" {
		t.Fatalf("valor %q, se esperaba la primera", valor)
	}
}

// La caché en memoria existe para las pruebas y para el respaldo sin disco.
func TestCacheMemoria(t *testing.T) {
	cache := NuevaCacheMemoria()
	ctx := context.Background()
	if _, hay, err := cache.Lee(ctx, "k"); err != nil || hay {
		t.Fatalf("vacía: %v %v", hay, err)
	}
	if err := cache.Guarda(ctx, "k", "v"); err != nil {
		t.Fatalf("Guarda: %v", err)
	}
	if valor, hay, err := cache.Lee(ctx, "k"); err != nil || !hay || valor != "v" {
		t.Fatalf("Lee: %q %v %v", valor, hay, err)
	}
}
