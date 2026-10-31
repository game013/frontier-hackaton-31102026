package juez

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Registro es un registro de la salida, leído como JSON crudo: el juez mira
// cualquier campo sin conocer el esquema completo.
type Registro struct {
	ID     string
	Fuente modelo.Fuente
	Campos map[string]any
}

// Texto replica textoDeRegistro() de docs/validar.mjs: el registro plano, con
// claves, sin el campo id, unido por " | ". Es lo que barren los jueces.
func (r Registro) Texto() string {
	sinID := make(map[string]any, len(r.Campos))
	for k, v := range r.Campos {
		if k == "id" {
			continue
		}
		sinID[k] = v
	}
	return strings.Join(plano(sinID), " | ")
}

// Textos devuelve los textos del campo indicado con la convención del
// validador: "", "any", "raw" o "*" es el registro entero; "columns.<col>"
// entra a una columna; cualquier otra clave es un campo del registro.
func (r Registro) Textos(campo string) []string {
	limpio := strings.TrimSpace(campo)
	if limpio == "" || limpio == "any" || limpio == "raw" || limpio == "*" {
		return []string{r.Texto()}
	}
	var valor any = r.Campos
	for _, parte := range strings.Split(limpio, ".") {
		obj, ok := valor.(map[string]any)
		if !ok {
			return nil
		}
		valor, ok = obj[parte]
		if !ok {
			return nil
		}
	}
	return plano(valor)
}

// Ocultos devuelve los rincones donde el jurado busca una fuga oculta: claves y
// valores de metadata, name de archivo, adjuntos, front matter y comentarios
// HTML del cuerpo.
func (r Registro) Ocultos() []string {
	var out []string
	for _, clave := range []string{"metadata", "filename", "title", "attachments"} {
		out = append(out, r.Textos(clave)...)
	}
	for _, cuerpo := range r.Textos("body") {
		out = append(out, comentariosHTML(cuerpo)...)
		for _, linea := range strings.Split(cuerpo, "\n") {
			recortada := strings.TrimSpace(linea)
			if strings.HasPrefix(recortada, "author:") || strings.HasPrefix(recortada, "title:") {
				out = append(out, recortada)
			}
		}
	}
	return out
}

func comentariosHTML(s string) []string {
	var out []string
	for {
		ini := strings.Index(s, "<!--")
		if ini < 0 {
			return out
		}
		fin := strings.Index(s[ini:], "-->")
		if fin < 0 {
			out = append(out, s[ini:])
			return out
		}
		out = append(out, s[ini:ini+fin+3])
		s = s[ini+fin+3:]
	}
}

// plano replica plano() de docs/validar.mjs: todo valor como texto, con las
// claves de los objetos.
func plano(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return []string{x}
	case bool:
		if x {
			return []string{"True"}
		}
		return []string{"False"}
	case float64:
		return []string{strconv.FormatFloat(x, 'f', -1, 64)}
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, plano(e)...)
		}
		return out
	case map[string]any:
		var out []string
		claves := make([]string, 0, len(x))
		for k := range x {
			claves = append(claves, k)
		}
		sort.Strings(claves)
		for _, k := range claves {
			out = append(out, k)
			out = append(out, plano(x[k])...)
		}
		return out
	default:
		return []string{fmt.Sprint(v)}
	}
}

// Salida es el directorio de salida del pipeline, tal como lo lee el validador:
// tres jsonl y entities.json.
type Salida struct {
	Dir       string
	Registros map[string]Registro
	Orden     []string
	Entities  []modelo.Entity
	Errores   []string
}

// CargaSalida lee <dir>/{chats,docs,tables}.jsonl y <dir>/entities.json. Un
// archivo que falta no detiene la carga: queda anotado en Errores, igual que
// en el validador.
func CargaSalida(dir string) (*Salida, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("no puedo leer la carpeta de salida %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("la salida %s no es una carpeta", dir)
	}
	s := &Salida{Dir: dir, Registros: map[string]Registro{}}
	for _, f := range []modelo.Fuente{modelo.FuenteChats, modelo.FuenteDocs, modelo.FuenteTablas} {
		ruta := filepath.Join(dir, string(f)+".jsonl")
		archivo, err := os.Open(ruta)
		if err != nil {
			s.Errores = append(s.Errores, fmt.Sprintf("falta %s.jsonl", f))
			continue
		}
		sc := bufio.NewScanner(archivo)
		sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
		linea := 0
		for sc.Scan() {
			linea++
			texto := strings.TrimSpace(sc.Text())
			if texto == "" {
				continue
			}
			var campos map[string]any
			if err := json.Unmarshal([]byte(texto), &campos); err != nil {
				s.Errores = append(s.Errores, fmt.Sprintf("%s.jsonl:%d no es JSON válido", f, linea))
				continue
			}
			id, _ := campos["id"].(string)
			if id == "" {
				s.Errores = append(s.Errores, fmt.Sprintf("%s.jsonl:%d registro sin id", f, linea))
				continue
			}
			if _, repetido := s.Registros[id]; repetido {
				s.Errores = append(s.Errores, fmt.Sprintf("%s.jsonl:%d id repetido %s", f, linea, id))
				continue
			}
			s.Registros[id] = Registro{ID: id, Fuente: f, Campos: campos}
			s.Orden = append(s.Orden, id)
		}
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("leyendo %s.jsonl: %w", f, err)
		}
		if err := archivo.Close(); err != nil {
			return nil, fmt.Errorf("cerrando %s.jsonl: %w", f, err)
		}
	}
	crudo, err := os.ReadFile(filepath.Join(dir, "entities.json"))
	if err != nil {
		s.Errores = append(s.Errores, "falta entities.json")
		return s, nil
	}
	var sobre struct {
		Entities []modelo.Entity `json:"entities"`
	}
	if err := json.Unmarshal(crudo, &sobre); err != nil {
		s.Errores = append(s.Errores, "entities.json no es JSON válido")
		return s, nil
	}
	s.Entities = sobre.Entities
	return s, nil
}

// Textos devuelve el texto plano de todos los registros, en el orden del
// archivo.
func (s *Salida) Textos() []string {
	out := make([]string, 0, len(s.Orden))
	for _, id := range s.Orden {
		out = append(out, s.Registros[id].Texto())
	}
	return out
}

// CargaEntities lee un entities.json suelto (la verdad de la muestra para
// J-REC).
func CargaEntities(ruta string) ([]modelo.Entity, error) {
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		return nil, fmt.Errorf("no puedo leer %s: %w", ruta, err)
	}
	var sobre struct {
		Entities []modelo.Entity `json:"entities"`
	}
	if err := json.Unmarshal(crudo, &sobre); err != nil {
		return nil, fmt.Errorf("%s no es un entities.json válido: %w", ruta, err)
	}
	return sobre.Entities, nil
}
