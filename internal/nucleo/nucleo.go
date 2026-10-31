// Package nucleo es el único camino de anonimización: lo usan /run y
// /anonymize, así que una persona recibe el mismo perfil en el corpus y en vivo
// (D8). Encadena las piezas en este orden:
//
//	registro → detección (pasada 1) → residuo (modelo) → resolución →
//	fábrica y bóveda → reemplazo → entities.json → compuerta (pasada 2)
//
// Dos pasadas son obligatorias (D14): una forma que aparece por primera vez en
// el registro 900 ya se habría escapado en el 12 si se reemplazara en la misma
// pasada. La compuerta final es la que decide si la corrida termina o falla
// ruidosamente (D11).
package nucleo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/compuerta"
	"github.com/game013/frontier-hackaton-31102026/internal/detecta"
	"github.com/game013/frontier-hackaton-31102026/internal/detecta/residuo"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/chats"
	"github.com/game013/frontier-hackaton-31102026/internal/llm"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/perfil"
	"github.com/game013/frontier-hackaton-31102026/internal/reemplazo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
	"github.com/game013/frontier-hackaton-31102026/internal/resuelve"
	"github.com/game013/frontier-hackaton-31102026/internal/salida"
)

// Opciones del núcleo. Almacen nil deja la corrida sin bóveda (pruebas); Modelo
// nil la deja determinista y sin LLM (D19).
type Opciones struct {
	Almacen   modelo.Almacen
	Modelo    modelo.Modelo
	Secreto   []byte
	Allowlist *detecta.Allowlist
	// Vivo marca las personas nuevas como de /anonymize (D24).
	Vivo bool
	// Plazo acota el lote; /anonymize usa 45 s (D28).
	Plazo time.Duration
}

// Entrada es el lote con los índices de chats que no viajan en los registros.
type Entrada struct {
	Registros []modelo.Registro
	// Usuarios es uid → usuario de users.json (C6) y Autores es id de registro →
	// uid. En /anonymize llegan vacíos: author ya es un nombre (D29).
	Usuarios map[string]chats.Usuario
	Autores  map[string]string
	Canales  map[string]chats.Canal
}

// Nucleo ejecuta el pipeline completo.
type Nucleo struct {
	almacen   modelo.Almacen
	modelo    modelo.Modelo
	secreto   []byte
	allowlist *detecta.Allowlist
	vivo      bool
	plazo     time.Duration
}

// Nuevo arma el núcleo. Sin secreto no hay perfiles deterministas, así que es
// error: un secreto vacío haría reversible el HMAC.
func Nuevo(o Opciones) (*Nucleo, error) {
	if len(o.Secreto) == 0 {
		return nil, errors.New("nucleo: falta el secreto del HMAC")
	}
	allow := o.Allowlist
	if allow == nil {
		allow = detecta.PorDefecto()
	}
	plazo := o.Plazo
	if plazo <= 0 {
		plazo = 45 * time.Second
	}
	return &Nucleo{
		almacen:   o.Almacen,
		modelo:    o.Modelo,
		secreto:   append([]byte(nil), o.Secreto...),
		allowlist: allow,
		vivo:      o.Vivo,
		plazo:     plazo,
	}, nil
}

// Anonymize cumple modelo.Nucleo: mismo núcleo para /run y /anonymize (D8).
// fuente solo etiqueta el lote; los registros pueden venir mezclados.
func (n *Nucleo) Anonymize(ctx context.Context, _ modelo.Fuente, registros []modelo.Registro) (modelo.Resultado, error) {
	return n.Corrida(ctx, Entrada{Registros: registros})
}

// Corrida anonimiza un lote completo y devuelve los registros y el diccionario.
func (n *Nucleo) Corrida(ctx context.Context, entrada Entrada) (modelo.Resultado, error) {
	lote, cancelar := context.WithTimeout(ctx, n.plazo)
	defer cancelar()

	indice, err := registro.Construye(entrada.Registros, registro.Opciones{
		Usuarios: entrada.Usuarios, Autores: entrada.Autores, Canales: entrada.Canales,
	})
	if err != nil {
		return modelo.Resultado{}, fmt.Errorf("nucleo: registro: %w", err)
	}
	indice, err = n.mezclaBoveda(lote, indice)
	if err != nil {
		return modelo.Resultado{}, err
	}

	// Pasada 1: descubrir formas en el texto y clasificar el residuo.
	detector, err := detecta.Nuevo(detecta.Opciones{Indice: indice, Allowlist: n.allowlist})
	if err != nil {
		return modelo.Resultado{}, fmt.Errorf("nucleo: detector: %w", err)
	}
	menciones, err := detector.Detecta(lote, entrada.Registros)
	if err != nil {
		return modelo.Resultado{}, fmt.Errorf("nucleo: detección: %w", err)
	}
	candidatos := detector.Candidatos(entrada.Registros)
	clasificador := residuo.Nuevo(residuo.Opciones{
		Modelo:    n.modelo,
		Timeout:   15 * time.Second,
		Enmascara: enmascarador(indice.Formas()),
	})
	if len(candidatos) > 0 {
		menciones = append(menciones, clasificador.Clasifica(lote, candidatos)...)
	}

	resolutor := resuelve.Nuevo(resuelve.Opciones{
		Indice: indice, Modelo: n.modelo, Almacen: n.almacen, Vivo: n.vivo,
	})
	menciones, err = resolutor.Resuelve(lote, entrada.Registros, menciones)
	if err != nil {
		return modelo.Resultado{}, fmt.Errorf("nucleo: resolución: %w", err)
	}

	perfiles, err := n.perfilesDe(lote, indice, menciones)
	if err != nil {
		return modelo.Resultado{}, err
	}

	reemplazados, err := reemplazo.Nuevo(reemplazo.Opciones{Indice: indice, Perfiles: perfiles}).
		Aplica(entrada.Registros, menciones)
	if err != nil {
		return modelo.Resultado{}, fmt.Errorf("nucleo: reemplazo: %w", err)
	}
	entidades := salida.Entidades(indice, menciones, perfiles, n.allowlist)

	// Pasada 2: la compuerta barre lo que haya quedado y lo corrige con la
	// bóveda ya completa (D11, D25).
	limpios, fugas, err := compuerta.Barre(lote, reemplazados, indice, perfiles, n.allowlist)
	if err != nil {
		return modelo.Resultado{}, fmt.Errorf("nucleo: compuerta: %w", err)
	}
	if pendientes := pendientesDe(fugas); pendientes > 0 {
		return modelo.Resultado{}, fmt.Errorf("nucleo: la compuerta no pudo cerrar %d fugas (%s)",
			pendientes, compuerta.Resume(fugas))
	}
	return modelo.Resultado{Registros: limpios, Entities: entidades}, nil
}

// mezclaBoveda carga el registro persistido y lo fusiona sin pisar lo nuevo
// (D4, D22): la bóveda es la verdad cuando el corpus crece.
func (n *Nucleo) mezclaBoveda(ctx context.Context, indice *registro.Indice) (*registro.Indice, error) {
	if n.almacen == nil {
		return indice, nil
	}
	guardadas, err := n.almacen.Personas(ctx)
	if err != nil {
		return nil, fmt.Errorf("nucleo: registro persistido: %w", err)
	}
	if len(guardadas) == 0 {
		return indice, nil
	}
	previo, err := registro.Construye(nil, registro.Opciones{})
	if err != nil {
		return nil, fmt.Errorf("nucleo: registro persistido: %w", err)
	}
	for _, p := range guardadas {
		previo.AgregaPersona(p)
	}
	// La bóveda manda: el registro persistido conserva rol y género, y las
	// formas nuevas solo se suman (D4, D22).
	previo.Fusiona(indice)
	return previo, nil
}

// perfilesDe pide a la fábrica el perfil de cada persona que intervino. La
// bóveda primero: un perfil asignado es para siempre (D4).
func (n *Nucleo) perfilesDe(ctx context.Context, indice *registro.Indice, menciones []modelo.Mencion) (map[string]modelo.Perfil, error) {
	fabrica, err := perfil.Nuevo(perfil.Opciones{
		Secreto: n.secreto,
		Almacen: n.almacen,
		Reales:  indice.Formas(),
	})
	if err != nil {
		return nil, fmt.Errorf("nucleo: fábrica: %w", err)
	}
	llaves := map[string]bool{}
	for _, m := range menciones {
		if m.Persona != "" {
			llaves[m.Persona] = true
		}
	}
	perfiles := make(map[string]modelo.Perfil, len(llaves))
	for llave := range llaves {
		persona, ok := indice.Persona(llave)
		if !ok {
			return nil, fmt.Errorf("nucleo: la mención apunta a una persona fuera del registro (%s)", llave)
		}
		p, err := fabrica.PerfilDe(ctx, persona)
		if err != nil {
			return nil, fmt.Errorf("nucleo: perfil de %s: %w", llave, err)
		}
		perfiles[llave] = p
	}
	return perfiles, nil
}

// enmascarador deja las formas conocidas fuera del texto que viaja al modelo
// (D15).
func enmascarador(formas []modelo.Forma) func(string) string {
	valores := make([]string, 0, len(formas))
	for _, f := range formas {
		valores = append(valores, f.Valor)
	}
	mascara := llm.NuevaMascara(valores)
	return mascara.Enmascara
}

// pendientesDe cuenta las fugas que la compuerta no pudo cerrar.
func pendientesDe(fugas []compuerta.Fuga) int {
	pendientes := 0
	for _, f := range fugas {
		if f.Restante {
			pendientes++
		}
	}
	return pendientes
}

// ConfiguracionDelModelo arma el cliente de Jev desde el entorno (D32). Se
// expone para el servicio y para la herramienta local; devuelve nil sin llave.
func ConfiguracionDelModelo(getenv func(string) string) (*llm.Cliente, error) {
	if getenv("OPENROUTER_API_KEY") == "" {
		return nil, nil
	}
	return llm.NuevaDesdeEntorno(getenv)
}
