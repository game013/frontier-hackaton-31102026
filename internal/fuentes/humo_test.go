package fuentes

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestHumoIndiceDeRegistros es la única prueba que sale a la red, y solo si la
// piden: FUENTES_HUMO=1 y TOKEN en el entorno. Compara el record_index del API
// con la copia local de fuentes/ (las fuentes solo crecen, así que lo local debe
// caber en lo remoto) y comprueba que el ETag devuelve 304 en la segunda visita.
func TestHumoIndiceDeRegistros(t *testing.T) {
	if testing.Short() {
		t.Skip("humo: -short")
	}
	if os.Getenv("FUENTES_HUMO") == "" {
		t.Skip("humo: define FUENTES_HUMO=1 para consultar el API real")
	}
	token := os.Getenv("TOKEN")
	if token == "" {
		t.Skip("humo: falta TOKEN en el entorno")
	}
	base := os.Getenv("SOURCES_API_URL")
	if base == "" {
		base = "https://reto-pii-fuentes-production.up.railway.app/v1"
	}
	c, err := Nuevo(Opciones{Base: base, Token: token, Timeout: 60 * time.Second})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelar()

	var tipo struct {
		Chats  []string `json:"chats"`
		Docs   []string `json:"docs"`
		Tablas []string `json:"tables"`
	}
	resp, err := c.IndiceRegistros(ctx)
	if err != nil {
		t.Fatalf("record_index: %v", err)
	}
	if resp.SinCambios {
		t.Fatal("SinCambios en la primera visita: falta el ETag")
	}
	if err := json.Unmarshal(resp.Cuerpo, &tipo); err != nil {
		t.Fatalf("record_index no es JSON: %v", err)
	}
	if len(tipo.Chats) == 0 || len(tipo.Docs) == 0 || len(tipo.Tablas) == 0 {
		t.Fatalf("record_index incompleto: chats=%d docs=%d tables=%d",
			len(tipo.Chats), len(tipo.Docs), len(tipo.Tablas))
	}
	if resp.ETag == "" {
		t.Fatal("record_index sin ETag: el monitor de crecimiento no podría detectar cambios")
	}
	t.Logf("record_index: chats=%d docs=%d tables=%d", len(tipo.Chats), len(tipo.Docs), len(tipo.Tablas))

	otra, err := c.IndiceRegistros(ctx)
	if err != nil {
		t.Fatalf("segunda visita: %v", err)
	}
	if !otra.SinCambios {
		t.Fatal("la segunda visita no fue 304: If-None-Match no se está respetando")
	}

	local := filepath.Join("..", "..", "fuentes", "record_index.json")
	crudo, err := os.ReadFile(local)
	if err != nil {
		t.Logf("sin copia local %s, no se compara: %v", local, err)
		return
	}
	var copia struct {
		Chats  []string `json:"chats"`
		Docs   []string `json:"docs"`
		Tablas []string `json:"tables"`
	}
	if err := json.Unmarshal(crudo, &copia); err != nil {
		t.Fatalf("%s no es JSON: %v", local, err)
	}
	faltan := map[string][]string{}
	faltan["chats"] = ausentes(copia.Chats, tipo.Chats)
	faltan["docs"] = ausentes(copia.Docs, tipo.Docs)
	faltan["tables"] = ausentes(copia.Tablas, tipo.Tablas)
	for fuente, ids := range faltan {
		if len(ids) != 0 {
			t.Fatalf("%s: %d ids de la copia local no están en el API, p. ej. %v", fuente, len(ids), ids[:3])
		}
	}
}

// ausentes devuelve los ids de local que no aparecen en remoto.
func ausentes(local, remoto []string) []string {
	hay := make(map[string]bool, len(remoto))
	for _, id := range remoto {
		hay[id] = true
	}
	var faltan []string
	for _, id := range local {
		if !hay[id] {
			faltan = append(faltan, id)
		}
	}
	return faltan
}
