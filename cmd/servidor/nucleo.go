package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/almacen"
	"github.com/game013/frontier-hackaton-31102026/internal/detecta"
	"github.com/game013/frontier-hackaton-31102026/internal/llm"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/nucleo"
)

// plazoAnonimiza es el presupuesto del lote en vivo: el contrato da 60 s y el
// respaldo sin LLM tiene que alcanzar dentro de 50 (D28).
const plazoAnonimiza = 50 * time.Second

// plazoCorrida es el presupuesto del lote completo de /run. El job se corta a
// los 110 min (plazoCorrida del servidor), así que la descarga y la escritura
// caben antes de que el núcleo se quede sin tiempo con el modelo.
const plazoCorrida = 90 * time.Minute

// permiteNucleo es la pieza que el servicio necesita: los dos núcleos que
// comparten la bóveda y la base que los respalda. Nucleo es el de /anonymize
// (las personas nuevas van a personas_vivo, D24) y Corrida el de /run (van al
// registro).
type permiteNucleo struct {
	Nucleo  modelo.Nucleo
	Corrida *nucleo.Nucleo
	// Uso es la contabilidad del modelo que /run reporta en /run/{id}; sin
	// llave devuelve ceros, porque no hubo llamadas (D13, D19).
	Uso    func() llm.Uso
	cerrar func() error
}

// contadorUso es lo que reporta su contabilidad; el cliente de Jev la
// implementa con internal/llm.Uso.
type contadorUso interface {
	Uso() llm.Uso
}

// Cerrar libera la base al apagar.
func (p *permiteNucleo) Cerrar() error {
	if p.cerrar == nil {
		return nil
	}
	return p.cerrar()
}

// abreNucleo arma el núcleo real del servicio: la bóveda en el volumen, el
// secreto del HMAC, la allowlist y el modelo si hay llave (D4, D8, D32).
//
// Sin SECRETO_HMAC se genera uno la primera vez y se guarda en el volumen: los
// perfiles son deterministas entre reinicios sin obligar a configurar la
// variable, y el secreto nunca viaja al repo.
func abreNucleo(datosDir string, getenv func(string) string) (*permiteNucleo, error) {
	if strings.TrimSpace(datosDir) == "" {
		return nil, errors.New("núcleo: falta el volumen de datos")
	}
	if err := os.MkdirAll(datosDir, 0o700); err != nil {
		return nil, fmt.Errorf("núcleo: crear %s: %w", datosDir, err)
	}
	base, err := almacen.Open(filepath.Join(datosDir, "almacen.db"))
	if err != nil {
		return nil, fmt.Errorf("núcleo: bóveda: %w", err)
	}
	secreto, err := secretoDe(datosDir, getenv("SECRETO_HMAC"))
	if err != nil {
		base.Cerrar()
		return nil, err
	}
	allow, err := allowlistDe(getenv("ALLOWLIST_PATH"))
	if err != nil {
		base.Cerrar()
		return nil, err
	}
	cache, err := base.CacheModelo()
	if err != nil {
		base.Cerrar()
		return nil, fmt.Errorf("núcleo: caché del modelo: %w", err)
	}
	// /run y /anonymize comparten el cliente y su caché en la bóveda (D32).
	modeloLLM, err := nucleo.ConfiguracionDelModelo(getenv, cache)
	if err != nil {
		base.Cerrar()
		return nil, fmt.Errorf("núcleo: modelo: %w", err)
	}
	vivo, err := nucleo.Nuevo(nucleo.Opciones{
		Almacen:   base,
		Modelo:    modeloLLM,
		Secreto:   secreto,
		Allowlist: allow,
		// /anonymize persiste a las personas nuevas en personas_vivo (D24).
		Vivo:  true,
		Plazo: plazoAnonimiza,
	})
	if err != nil {
		base.Cerrar()
		return nil, err
	}
	corrida, err := nucleo.Nuevo(nucleo.Opciones{
		Almacen:   base,
		Modelo:    modeloLLM,
		Secreto:   secreto,
		Allowlist: allow,
		// /run suma a las personas nuevas al registro, no a personas_vivo.
		Vivo:  false,
		Plazo: plazoCorrida,
	})
	if err != nil {
		base.Cerrar()
		return nil, err
	}
	uso := func() llm.Uso { return llm.Uso{} }
	if contador, ok := modeloLLM.(contadorUso); ok {
		uso = contador.Uso
	}
	return &permiteNucleo{Nucleo: vivo, Corrida: corrida, Uso: uso, cerrar: base.Cerrar}, nil
}

// secretoDe devuelve el secreto del HMAC: el del entorno, el del volumen o uno
// nuevo que queda guardado con permisos privados.
func secretoDe(datosDir, delEntorno string) ([]byte, error) {
	if valor := strings.TrimSpace(delEntorno); valor != "" {
		return []byte(valor), nil
	}
	ruta := filepath.Join(datosDir, "secreto_hmac")
	if datos, err := os.ReadFile(ruta); err == nil && len(strings.TrimSpace(string(datos))) > 0 {
		return []byte(strings.TrimSpace(string(datos))), nil
	}
	crudo := make([]byte, 32)
	if _, err := rand.Read(crudo); err != nil {
		return nil, fmt.Errorf("núcleo: generar el secreto: %w", err)
	}
	secreto := hex.EncodeToString(crudo)
	if err := os.WriteFile(ruta, []byte(secreto+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("núcleo: guardar el secreto: %w", err)
	}
	return []byte(secreto), nil
}

// allowlistDe carga la allowlist de C26 si el archivo está; si no, usa las
// reglas genéricas. Un archivo ilegible no puede pasar en silencio.
func allowlistDe(ruta string) (*detecta.Allowlist, error) {
	if strings.TrimSpace(ruta) == "" {
		ruta = filepath.Join("internal", "detecta", "allowlist.json")
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return detecta.PorDefecto(), nil
		}
		return nil, fmt.Errorf("núcleo: allowlist %s: %w", ruta, err)
	}
	allow, err := detecta.CargaJSON(datos)
	if err != nil {
		return nil, fmt.Errorf("núcleo: allowlist %s: %w", ruta, err)
	}
	return allow, nil
}

// contextoDeArranque existe para que el log de arranque no dependa del reloj en
// las pruebas.
var _ = context.Background
