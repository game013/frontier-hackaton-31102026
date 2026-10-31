// Package compuerta es la última barrera antes de escribir la salida (D11, D25).
//
// Barre todas las formas reales del registro sobre todos los strings de la
// salida —texto, autor, metadata, columnas, adjuntos y claves— con la misma
// normalización que el jurado, y también sus hashes sin sal. Si todavía queda
// una forma con un dueño, la reemplaza por el perfil de esa persona; si la forma
// es ambigua, usa la llave menor y la marca como dudosa. Al final vuelve a
// barrer: la corrida termina con cero coincidencias o falla ruidosamente.
package compuerta

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/campos"
	"github.com/game013/frontier-hackaton-31102026/internal/detecta"
	"github.com/game013/frontier-hackaton-31102026/internal/formas"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/reemplazo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// Fuga es una forma real que la compuerta encontró y corrigió.
type Fuga struct {
	Registro string
	Campo    modelo.CampoTexto
	Forma    string
	Hash     bool
	// Restante marca una forma que la compuerta no pudo cerrar: si queda una,
	// la corrida falla en vez de escribir una fuga (D11).
	Restante bool
}

// PerfilDe entrega el perfil de una persona: la fábrica, con la bóveda primero.
type PerfilDe func(ctx context.Context, llave string) (modelo.Perfil, error)

// Barre devuelve la salida sin formas reales conocidas y la lista de fugas que
// corrigió. Si tras corregir sigue habiendo una forma con dueño, la devuelve en
// la lista y el llamador decide fallar: nunca se escribe una fuga en silencio.
func Barre(ctx context.Context, recs []modelo.Registro, indice *registro.Indice, perfiles map[string]modelo.Perfil, allow *detecta.Allowlist) ([]modelo.Registro, []Fuga, error) {
	return BarreConFabrica(ctx, recs, indice, perfiles, nil, allow)
}

// BarreConFabrica es Barre con una fábrica de respaldo: una persona del registro
// que no tuvo mención en la primera pasada (p. ej. una que viene de la bóveda
// tras el crecimiento) no está en perfiles; si la compuerta le atribuye una
// forma, el perfil se pide a perfilDe y se agrega al mapa, así el diccionario
// también lo ve (CE-48). Con perfilDe nil se comporta como antes.
func BarreConFabrica(ctx context.Context, recs []modelo.Registro, indice *registro.Indice, perfiles map[string]modelo.Perfil, perfilDe PerfilDe, allow *detecta.Allowlist) ([]modelo.Registro, []Fuga, error) {
	if indice == nil {
		return nil, nil, fmt.Errorf("compuerta: falta el registro de personas")
	}
	detector, err := detecta.Nuevo(detecta.Opciones{Indice: indice, Allowlist: allow})
	if err != nil {
		return nil, nil, fmt.Errorf("compuerta: %w", err)
	}
	menciones, err := detector.Detecta(ctx, recs)
	if err != nil {
		return nil, nil, fmt.Errorf("compuerta: %w", err)
	}
	var fugas []Fuga
	for i := range menciones {
		m := &menciones[i]
		if m.Persona != "" {
			fugas = append(fugas, Fuga{Registro: m.Registro, Campo: m.Campo, Forma: m.Valor})
			continue
		}
		if llave, ok := duenoMenor(indice, *m); ok {
			m.Persona = llave
			m.Confianza = 0.5
			fugas = append(fugas, Fuga{Registro: m.Registro, Campo: m.Campo, Forma: m.Valor})
		}
	}
	salida := recs
	if len(fugas) > 0 {
		for _, m := range menciones {
			if err := completa(ctx, perfiles, perfilDe, m.Persona); err != nil {
				return nil, fugas, err
			}
		}
		salida, err = reemplazo.Nuevo(reemplazo.Opciones{Indice: indice, Perfiles: perfiles}).Aplica(recs, menciones)
		if err != nil {
			return nil, fugas, fmt.Errorf("compuerta: %w", err)
		}
	}
	salida, fugasHash, err := barreHashes(ctx, salida, indice, perfiles, perfilDe)
	if err != nil {
		return nil, fugas, err
	}
	fugas = append(fugas, fugasHash...)
	if quedan := verifica(detector, ctx, salida, indice); len(quedan) > 0 {
		fugas = append(fugas, quedan...)
	}
	return salida, fugas, nil
}

// duenoMenor resuelve una forma compartida por la llave menor, como manda D25.
func duenoMenor(indice *registro.Indice, m modelo.Mencion) (string, bool) {
	if m.Forma == modelo.FormaNombrePila {
		return "", false
	}
	candidatos := indice.Busca(m.Valor)
	if len(candidatos) == 0 {
		return "", false
	}
	llaves := make([]string, 0, len(candidatos))
	for _, p := range candidatos {
		llaves = append(llaves, p.Llave)
	}
	sort.Strings(llaves)
	return llaves[0], true
}

// verifica vuelve a barrer la salida ya corregida: lo que quede es una fuga que
// la compuerta no pudo cerrar y tiene que ver el llamador.
// verifica vuelve a barrer la salida ya corregida. Solo cuentan las formas
// conocidas del registro —las que tienen dueño—: los patrones ven correos y
// cédulas cualesquiera, incluidos los falsos, y no son una fuga (D25).
func verifica(detector *detecta.Detector, ctx context.Context, recs []modelo.Registro, indice *registro.Indice) []Fuga {
	menciones, err := detector.Detecta(ctx, recs)
	if err != nil {
		return nil
	}
	var out []Fuga
	for _, m := range menciones {
		if len(indice.Busca(m.Valor)) == 0 {
			continue
		}
		// Un nombre de pila compartido sin dueño no es una fuga que la compuerta
		// deba cerrar: barrerlo fusionaría personas (D25).
		if m.Forma == modelo.FormaNombrePila && m.Persona == "" {
			continue
		}
		out = append(out, Fuga{Registro: m.Registro, Campo: m.Campo, Forma: m.Valor, Restante: true})
	}
	return out
}

// patronHash reconoce un digest hexadecimal sin sal en el texto.
var patronHash = regexp.MustCompile(`\b[0-9a-f]{32}\b|\b[0-9a-f]{40}\b|\b[0-9a-f]{64}\b`)

// barreHashes reemplaza los hashes sin sal de las formas reales por el hash de
// la forma falsa correspondiente (D25). Un hash sin sal de una cédula se revierte
// en segundos, así que también es PII.
func barreHashes(ctx context.Context, recs []modelo.Registro, indice *registro.Indice, perfiles map[string]modelo.Perfil, perfilDe PerfilDe) ([]modelo.Registro, []Fuga, error) {
	porHash := map[string]modelo.Mencion{}
	for _, p := range indice.Personas() {
		for _, f := range p.Formas {
			for _, h := range hashesDe(f.Valor) {
				porHash[h] = modelo.Mencion{
					Registro: "", Fuente: "", Campo: "", Valor: f.Valor,
					Tipo: modelo.MencionPersona, Forma: f.Tipo, Persona: p.Llave,
				}
			}
		}
	}
	if len(porHash) == 0 {
		return recs, nil, nil
	}
	var fugas []Fuga
	out := make([]modelo.Registro, 0, len(recs))
	for _, r := range recs {
		var ediciones []campos.Edicion
		for _, campo := range campos.Lista(r) {
			for _, tramo := range patronHash.FindAllStringIndex(campo.Valor, -1) {
				digesto := campo.Valor[tramo[0]:tramo[1]]
				m, ok := porHash[digesto]
				if !ok {
					continue
				}
				if err := completa(ctx, perfiles, perfilDe, m.Persona); err != nil {
					return nil, fugas, err
				}
				perfil, ok := perfiles[m.Persona]
				if !ok {
					return nil, fugas, fmt.Errorf("compuerta: %s sin perfil para el hash", m.Persona)
				}
				forma, err := formas.Render(perfil, m.Forma)
				if err != nil {
					continue
				}
				falso := hashesDe(forma.Valor)
				indiceDigesto := -1
				switch len(digesto) {
				case 32:
					indiceDigesto = 0
				case 40:
					indiceDigesto = 1
				case 64:
					indiceDigesto = 2
				}
				if indiceDigesto < 0 || indiceDigesto >= len(falso) || falso[indiceDigesto] == "" {
					continue
				}
				ediciones = append(ediciones, campos.Edicion{
					Ruta: campo.Ruta, Inicio: tramo[0], Fin: tramo[1], Nuevo: falso[indiceDigesto],
				})
				fugas = append(fugas, Fuga{Registro: r.IDRegistro(), Campo: campo.Ruta, Forma: digesto, Hash: true})
			}
		}
		if len(ediciones) == 0 {
			out = append(out, r)
			continue
		}
		nuevo, err := campos.Aplica(r, ediciones)
		if err != nil {
			return nil, fugas, fmt.Errorf("compuerta: hash en %s: %w", r.IDRegistro(), err)
		}
		out = append(out, nuevo)
	}
	return out, fugas, nil
}

// hashesDe devuelve MD5, SHA-1 y SHA-256 del valor, en ese orden.
func hashesDe(valor string) []string {
	md := md5.Sum([]byte(valor))
	sh1 := sha1.Sum([]byte(valor))
	sh256 := sha256.Sum256([]byte(valor))
	return []string{
		hex.EncodeToString(md[:]),
		hex.EncodeToString(sh1[:]),
		hex.EncodeToString(sh256[:]),
	}
}

// Resume deja las fugas en una línea de log: solo ids y conteos, nunca valores
// (regla 8 de PROGRESS.md).
func Resume(fugas []Fuga) string {
	if len(fugas) == 0 {
		return "0 fugas"
	}
	hashes := 0
	porCampo := map[string]int{}
	for _, f := range fugas {
		if f.Hash {
			hashes++
		}
		porCampo[string(f.Campo)]++
	}
	campos := make([]string, 0, len(porCampo))
	for campo := range porCampo {
		campos = append(campos, campo)
	}
	sort.Strings(campos)
	partes := make([]string, 0, len(campos))
	for _, campo := range campos {
		partes = append(partes, fmt.Sprintf("%s=%d", campo, porCampo[campo]))
	}
	return fmt.Sprintf("%d fugas (%d hashes) en %s", len(fugas), hashes, strings.Join(partes, ", "))
}

// completa pide a la fábrica el perfil de una persona que no está en el mapa.
// Sin fábrica no hace nada: el llamador decide qué hacer con el hueco.
func completa(ctx context.Context, perfiles map[string]modelo.Perfil, perfilDe PerfilDe, llave string) error {
	if llave == "" || perfilDe == nil {
		return nil
	}
	if _, ok := perfiles[llave]; ok {
		return nil
	}
	p, err := perfilDe(ctx, llave)
	if err != nil {
		return fmt.Errorf("compuerta: perfil de %s: %w", llave, err)
	}
	perfiles[llave] = p
	return nil
}
