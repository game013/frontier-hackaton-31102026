package servidor_test

import (
	"github.com/game013/frontier-hackaton-31102026/internal/llm"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/servidor"
)

func TestCargaConfig(t *testing.T) {
	entorno := map[string]string{}
	carga := func(clave string) string { return entorno[clave] }

	cfg, err := servidor.Carga(carga)
	if err != nil {
		t.Fatalf("Carga con todo por defecto: %v", err)
	}
	if cfg.Puerto != "9000" || cfg.DatosDir != "var" {
		t.Fatalf("valores por defecto mal: %+v", cfg)
	}
	if cfg.Equipo == "" || cfg.Version == "" || cfg.PythonBin == "" || cfg.AyudantePDF == "" {
		t.Fatalf("config incompleta: %+v", cfg)
	}
	if len(cfg.Modelos) != 1 || cfg.Modelos[0] != llm.SnapshotJev {
		t.Fatalf("sin variables, models debe ser el snapshot fijo %s: %+v", llm.SnapshotJev, cfg.Modelos)
	}

	// LLM_MODEL manda cuando no hay MODELOS: es lo que /run usa de verdad (D32).
	entorno["LLM_MODEL"] = "typesafe/jev-de-prueba"
	if cfg, err = servidor.Carga(carga); err != nil {
		t.Fatalf("Carga con LLM_MODEL: %v", err)
	}
	if len(cfg.Modelos) != 1 || cfg.Modelos[0] != "typesafe/jev-de-prueba" {
		t.Fatalf("LLM_MODEL no se reportó: %+v", cfg.Modelos)
	}
	delete(entorno, "LLM_MODEL")

	entorno["PORT"] = "8080"
	entorno["DATA_DIR"] = "/data"
	entorno["TEAM"] = "equipo-x"
	entorno["VERSION"] = "1.2.3"
	entorno["MODELOS"] = "typesafe/jev-1.14-20261014, otro/modelo"
	cfg, err = servidor.Carga(carga)
	if err != nil {
		t.Fatalf("Carga con valores: %v", err)
	}
	if cfg.Puerto != "8080" || cfg.DatosDir != "/data" || cfg.Equipo != "equipo-x" || cfg.Version != "1.2.3" {
		t.Fatalf("valores mal: %+v", cfg)
	}
	if len(cfg.Modelos) != 2 || cfg.Modelos[0] != "typesafe/jev-1.14-20261014" || cfg.Modelos[1] != "otro/modelo" {
		t.Fatalf("modelos mal: %+v", cfg.Modelos)
	}

	entorno["PORT"] = "ochenta"
	if _, err := servidor.Carga(carga); err == nil {
		t.Fatal("PORT no numérico debería fallar al arrancar")
	}
}
