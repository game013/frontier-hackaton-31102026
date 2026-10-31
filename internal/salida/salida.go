// Package salida escribe el corpus anonimizado en el esquema de output/ y arma
// entities.json con todas las formas reales de cada persona (D11, D25, D34).
//
// Los registros salen en el orden en que entraron y con las mismas claves que
// pide docs/api/participante.openapi.yaml: chats.jsonl, docs.jsonl,
// tables.jsonl y entities.json. Nada de lo que se escribe depende del orden de
// un mapa, para que dos corridas sean byte a byte iguales.
package salida

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/game013/frontier-hackaton-31102026/internal/detecta"
	"github.com/game013/frontier-hackaton-31102026/internal/formas"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// Archivos que espera el validador, en su orden.
var archivos = []struct {
	nombre string
	fuente modelo.Fuente
}{
	{"chats.jsonl", modelo.FuenteChats},
	{"docs.jsonl", modelo.FuenteDocs},
	{"tables.jsonl", modelo.FuenteTablas},
}

// Escribe la salida completa en dir. Crea el directorio si no existe.
func Escribe(dir string, recs []modelo.Registro, ents []modelo.Entity) error {
	if dir == "" {
		return fmt.Errorf("salida: directorio vacío")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("salida: crear %s: %w", dir, err)
	}
	for _, a := range archivos {
		if err := escribeJSONL(filepath.Join(dir, a.nombre), recs, a.fuente); err != nil {
			return err
		}
	}
	return escribeEntidades(filepath.Join(dir, "entities.json"), ents)
}

func escribeJSONL(ruta string, recs []modelo.Registro, fuente modelo.Fuente) error {
	archivo, err := os.Create(ruta)
	if err != nil {
		return fmt.Errorf("salida: crear %s: %w", ruta, err)
	}
	escritor := bufio.NewWriterSize(archivo, 1<<16)
	encoder := json.NewEncoder(escritor)
	for _, r := range recs {
		if r.FuenteRegistro() != fuente {
			continue
		}
		if err := encoder.Encode(r); err != nil {
			archivo.Close()
			return fmt.Errorf("salida: escribir %s: %w", ruta, err)
		}
	}
	if err := escritor.Flush(); err != nil {
		archivo.Close()
		return fmt.Errorf("salida: escribir %s: %w", ruta, err)
	}
	return archivo.Close()
}

func escribeEntidades(ruta string, ents []modelo.Entity) error {
	sobre := struct {
		Entities []modelo.Entity `json:"entities"`
	}{Entities: ents}
	if sobre.Entities == nil {
		sobre.Entities = []modelo.Entity{}
	}
	datos, err := json.MarshalIndent(sobre, "", "  ")
	if err != nil {
		return fmt.Errorf("salida: entities.json: %w", err)
	}
	datos = append(datos, '\n')
	if err := os.WriteFile(ruta, datos, 0o640); err != nil {
		return fmt.Errorf("salida: escribir %s: %w", ruta, err)
	}
	return nil
}

// Entidades arma el diccionario: una entrada por persona con todas sus formas
// reales, los seudónimos que efectivamente se usaron y los registros donde
// aparece; más una entrada conservada por cada entidad de la allowlist.
func Entidades(indice *registro.Indice, menciones []modelo.Mencion, perfiles map[string]modelo.Perfil, allow *detecta.Allowlist) []modelo.Entity {
	registros := map[string]map[string]bool{}
	seudonimos := map[string]map[string]string{}
	orden := map[string]bool{}
	for _, m := range menciones {
		if m.Persona == "" {
			continue
		}
		perfil, ok := perfiles[m.Persona]
		if !ok {
			continue
		}
		orden[m.Persona] = true
		if registros[m.Persona] == nil {
			registros[m.Persona] = map[string]bool{}
		}
		registros[m.Persona][m.Registro] = true
		if seudonimos[m.Persona] == nil {
			seudonimos[m.Persona] = map[string]string{}
		}
		clave, valor, ok := pseudonimoDe(perfil, m.Forma)
		if !ok {
			continue
		}
		// El nombre completo gana al corto: entities.json declara la identidad
		// completa cuando se usó alguna de sus formas.
		if clave == "name" && seudonimos[m.Persona][clave] != "" && m.Forma != modelo.FormaNombreCompleto {
			continue
		}
		seudonimos[m.Persona][clave] = valor
	}
	llaves := make([]string, 0, len(orden))
	for llave := range orden {
		llaves = append(llaves, llave)
	}
	sort.Strings(llaves)
	var out []modelo.Entity
	for _, llave := range llaves {
		p, ok := indice.Persona(llave)
		if !ok {
			continue
		}
		out = append(out, modelo.Entity{
			EntityID:   llave,
			Type:       modelo.EntidadPersona,
			Originals:  formasReales(p),
			Pseudonyms: seudonimos[llave],
			Records:    listaOrdenada(registros[llave]),
		})
	}
	out = append(out, conservadas(allow)...)
	return out
}

// pseudonimoDe traduce el tipo de forma al campo de pseudonyms del contrato.
func pseudonimoDe(perfil modelo.Perfil, tipo modelo.TipoForma) (string, string, bool) {
	if tipo == "" {
		return "name", perfil.Nombre, perfil.Nombre != ""
	}
	switch tipo {
	case modelo.FormaNombreCompleto, modelo.FormaNombreCorto, modelo.FormaNombrePila,
		modelo.FormaMayusculas, modelo.FormaSlug, modelo.FormaUsuario:
		return "name", perfil.Nombre, perfil.Nombre != ""
	case modelo.FormaCorreo, modelo.FormaCorreoDeletreado:
		return "email", perfil.Correo, perfil.Correo != ""
	case modelo.FormaCedula, modelo.FormaCedulaPuntos, modelo.FormaCedulaCC:
		return "cedula", perfil.Cedula, perfil.Cedula != ""
	case modelo.FormaCelular, modelo.FormaCelularE164, modelo.FormaCelularLocal:
		return "phone", perfil.Celular, perfil.Celular != ""
	case modelo.FormaDireccion:
		return "address", perfil.Direccion, perfil.Direccion != ""
	case modelo.FormaFechaISO, modelo.FormaFechaPalabras, modelo.FormaFechaUS:
		return "birth_iso", perfil.Nacimiento, perfil.Nacimiento != ""
	case modelo.FormaPlaca:
		return "plate", perfil.Placa, perfil.Placa != ""
	case modelo.FormaPasaporte:
		return "passport", perfil.Pasaporte, perfil.Pasaporte != ""
	}
	return "name", perfil.Nombre, perfil.Nombre != ""
}

// formasReales devuelve las formas de la persona sin repetir y en orden estable.
func formasReales(p modelo.Persona) []string {
	vistas := map[string]bool{}
	var out []string
	for _, f := range p.Formas {
		if f.Valor == "" || vistas[f.Valor] {
			continue
		}
		vistas[f.Valor] = true
		out = append(out, f.Valor)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ni, nj := norm.Norm(out[i]), norm.Norm(out[j])
		if ni != nj {
			return ni < nj
		}
		return out[i] < out[j]
	})
	return out
}

// conservadas arma las entidades de la allowlist con su motivo.
func conservadas(allow *detecta.Allowlist) []modelo.Entity {
	if allow == nil {
		return nil
	}
	var out []modelo.Entity
	for i, e := range allow.Canonica() {
		motivo := e.Motivo
		if motivo == "" {
			motivo = "organización, lugar, marca o figura pública; no es persona natural"
		}
		out = append(out, modelo.Entity{
			EntityID:  fmt.Sprintf("kept_%03d", i+1),
			Type:      modelo.EntidadOrganizacionOLugar,
			Originals: []string{e.Nombre},
			Records:   []string{},
			Kept:      true,
			Reason:    motivo,
		})
	}
	return out
}

func listaOrdenada(conjunto map[string]bool) []string {
	out := make([]string, 0, len(conjunto))
	for valor := range conjunto {
		out = append(out, valor)
	}
	sort.Strings(out)
	return out
}

// RenderDe es el valor falso de una forma, para las pruebas y para quien
// necesite el mismo mapeo de estilo.
func RenderDe(perfil modelo.Perfil, tipo modelo.TipoForma) (string, error) {
	forma, err := formas.Render(perfil, tipo)
	if err != nil {
		return "", err
	}
	return forma.Valor, nil
}
