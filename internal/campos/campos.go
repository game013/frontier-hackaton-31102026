// Package campos recorre y edita los strings de un registro canónico.
//
// Todo el pipeline que toca texto —detección, reemplazo, compuerta y salida—
// necesita la misma lista de campos: el cuerpo de un chat o documento, el autor,
// el título, el nombre de archivo, las claves de metadata (Author, Keywords de
// un PDF), el front matter ya volcado en body, las columnas de una tabla y los
// adjuntos (D4, D6). Tener una sola definición evita que un campo quede fuera
// de la detección en una fuente y dentro en otra.
//
// Las rutas usan el formato de modelo.CampoTexto: "text", "author", "filename",
// "title", "body", "metadata.<clave>", "columns.<columna>" y
// "attachments.<índice>.filename".
package campos

import (
	"fmt"
	"sort"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Campo es un string de un registro con la ruta que lo identifica.
type Campo struct {
	Ruta  modelo.CampoTexto
	Valor string
}

// Lista devuelve los campos de texto de un registro en orden estable: el mismo
// registro produce siempre la misma lista, para que la salida sea determinista.
func Lista(r modelo.Registro) []Campo {
	switch v := r.(type) {
	case modelo.ChatRecord:
		campos := make([]Campo, 0, 2+len(v.Attachments))
		campos = append(campos, Campo{Ruta: "author", Valor: v.Author}, Campo{Ruta: "text", Valor: v.Text})
		for i, a := range v.Attachments {
			campos = append(campos, Campo{Ruta: rutaAdjunto(i), Valor: a.Filename})
		}
		return campos
	case *modelo.ChatRecord:
		return Lista(*v)
	case modelo.DocRecord:
		campos := make([]Campo, 0, 3+len(v.Metadata))
		campos = append(campos,
			Campo{Ruta: "filename", Valor: v.Filename},
			Campo{Ruta: "title", Valor: v.Title},
			Campo{Ruta: "body", Valor: v.Body},
		)
		for _, clave := range clavesOrdenadas(v.Metadata) {
			campos = append(campos, Campo{Ruta: rutaMetadata(clave), Valor: v.Metadata[clave]})
		}
		return campos
	case *modelo.DocRecord:
		return Lista(*v)
	case modelo.TableRecord:
		campos := make([]Campo, 0, len(v.Columns))
		for _, clave := range clavesOrdenadas(v.Columns) {
			campos = append(campos, Campo{Ruta: rutaColumna(clave), Valor: v.Columns[clave]})
		}
		return campos
	case *modelo.TableRecord:
		return Lista(*v)
	}
	return nil
}

// Edicion reemplaza el fragmento [Inicio, Fin) de un campo por Nuevo. Los
// offsets son bytes sobre el valor original del campo y Fin es exclusivo.
type Edicion struct {
	Ruta   modelo.CampoTexto
	Inicio int
	Fin    int
	Nuevo  string
}

// Aplica devuelve una copia del registro con las ediciones aplicadas. Las
// ediciones de un mismo campo se aplican de derecha a izquierda, así los
// offsets calculados sobre el original siguen siendo válidos (D6). Falla si una
// ruta no existe o si un rango se sale del valor: un offset errado no puede
// pasar en silencio.
func Aplica(r modelo.Registro, ediciones []Edicion) (modelo.Registro, error) {
	if len(ediciones) == 0 {
		return r, nil
	}
	porRuta := make(map[modelo.CampoTexto][]Edicion, len(ediciones))
	for _, e := range ediciones {
		porRuta[e.Ruta] = append(porRuta[e.Ruta], e)
	}
	switch v := r.(type) {
	case modelo.ChatRecord:
		return aplicaChat(v, porRuta)
	case *modelo.ChatRecord:
		return aplicaChat(*v, porRuta)
	case modelo.DocRecord:
		return aplicaDoc(v, porRuta)
	case *modelo.DocRecord:
		return aplicaDoc(*v, porRuta)
	case modelo.TableRecord:
		return aplicaTabla(v, porRuta)
	case *modelo.TableRecord:
		return aplicaTabla(*v, porRuta)
	}
	return nil, fmt.Errorf("campos: registro de tipo %T desconocido", r)
}

func aplicaChat(v modelo.ChatRecord, porRuta map[modelo.CampoTexto][]Edicion) (modelo.Registro, error) {
	if eds, ok := porRuta["author"]; ok {
		nuevo, err := aplicaTexto("author", v.Author, eds)
		if err != nil {
			return nil, err
		}
		v.Author = nuevo
	}
	if eds, ok := porRuta["text"]; ok {
		nuevo, err := aplicaTexto("text", v.Text, eds)
		if err != nil {
			return nil, err
		}
		v.Text = nuevo
	}
	adjuntos := make([]modelo.Adjunto, len(v.Attachments))
	copy(adjuntos, v.Attachments)
	for ruta, eds := range porRuta {
		if ruta == "author" || ruta == "text" {
			continue
		}
		i, ok := indiceAdjunto(ruta)
		if !ok {
			return nil, fmt.Errorf("campos: ruta %q no existe en un mensaje", ruta)
		}
		if i < 0 || i >= len(adjuntos) {
			return nil, fmt.Errorf("campos: adjunto %d fuera de rango", i)
		}
		nuevo, err := aplicaTexto(ruta, adjuntos[i].Filename, eds)
		if err != nil {
			return nil, err
		}
		adjuntos[i].Filename = nuevo
	}
	v.Attachments = adjuntos
	return v, nil
}

func aplicaDoc(v modelo.DocRecord, porRuta map[modelo.CampoTexto][]Edicion) (modelo.Registro, error) {
	if eds, ok := porRuta["filename"]; ok {
		nuevo, err := aplicaTexto("filename", v.Filename, eds)
		if err != nil {
			return nil, err
		}
		v.Filename = nuevo
	}
	if eds, ok := porRuta["title"]; ok {
		nuevo, err := aplicaTexto("title", v.Title, eds)
		if err != nil {
			return nil, err
		}
		v.Title = nuevo
	}
	if eds, ok := porRuta["body"]; ok {
		nuevo, err := aplicaTexto("body", v.Body, eds)
		if err != nil {
			return nil, err
		}
		v.Body = nuevo
	}
	metadata := make(map[string]string, len(v.Metadata))
	for clave, valor := range v.Metadata {
		metadata[clave] = valor
	}
	for ruta, eds := range porRuta {
		if ruta == "filename" || ruta == "title" || ruta == "body" {
			continue
		}
		clave, ok := claveMetadata(ruta)
		if !ok {
			return nil, fmt.Errorf("campos: ruta %q no existe en un documento", ruta)
		}
		if _, existe := metadata[clave]; !existe {
			return nil, fmt.Errorf("campos: metadata %q no existe", clave)
		}
		nuevo, err := aplicaTexto(ruta, metadata[clave], eds)
		if err != nil {
			return nil, err
		}
		metadata[clave] = nuevo
	}
	v.Metadata = metadata
	return v, nil
}

func aplicaTabla(v modelo.TableRecord, porRuta map[modelo.CampoTexto][]Edicion) (modelo.Registro, error) {
	columnas := make(map[string]string, len(v.Columns))
	for clave, valor := range v.Columns {
		columnas[clave] = valor
	}
	for ruta, eds := range porRuta {
		clave, ok := claveColumna(ruta)
		if !ok {
			return nil, fmt.Errorf("campos: ruta %q no aplica a una tabla", ruta)
		}
		if _, existe := columnas[clave]; !existe {
			return nil, fmt.Errorf("campos: columna %q no existe", clave)
		}
		nuevo, err := aplicaTexto(ruta, columnas[clave], eds)
		if err != nil {
			return nil, err
		}
		columnas[clave] = nuevo
	}
	v.Columns = columnas
	return v, nil
}

// aplicaTexto aplica las ediciones de un campo de derecha a izquierda para no
// invalidar los offsets del original.
func aplicaTexto(ruta modelo.CampoTexto, valor string, eds []Edicion) (string, error) {
	ordenadas := make([]Edicion, len(eds))
	copy(ordenadas, eds)
	sort.SliceStable(ordenadas, func(i, j int) bool {
		if ordenadas[i].Inicio != ordenadas[j].Inicio {
			return ordenadas[i].Inicio > ordenadas[j].Inicio
		}
		return ordenadas[i].Fin > ordenadas[j].Fin
	})
	for i, e := range ordenadas {
		if e.Inicio < 0 || e.Fin < e.Inicio || e.Fin > len(valor) {
			return "", fmt.Errorf("campos: rango [%d,%d) fuera de %q (%d bytes)", e.Inicio, e.Fin, ruta, len(valor))
		}
		if i > 0 {
			anterior := ordenadas[i-1]
			if e.Fin > anterior.Inicio {
				return "", fmt.Errorf("campos: ediciones solapadas en %q ([%d,%d) y [%d,%d))", ruta, e.Inicio, e.Fin, anterior.Inicio, anterior.Fin)
			}
		}
	}
	for _, e := range ordenadas {
		if !uti8Limite(valor, e.Inicio) || !uti8Limite(valor, e.Fin) {
			return "", fmt.Errorf("campos: offset a mitad de runa en %q ([%d,%d))", ruta, e.Inicio, e.Fin)
		}
	}
	salida := valor
	for _, e := range ordenadas {
		salida = salida[:e.Inicio] + e.Nuevo + salida[e.Fin:]
	}
	return salida, nil
}

// uti8Limite dice si el offset cae en el borde de una runa.
func uti8Limite(s string, i int) bool {
	if i <= 0 || i >= len(s) {
		return true
	}
	return s[i]&0xC0 != 0x80
}

func rutaAdjunto(i int) modelo.CampoTexto {
	return modelo.CampoTexto(fmt.Sprintf("attachments.%d.filename", i))
}

func indiceAdjunto(ruta modelo.CampoTexto) (int, bool) {
	s := string(ruta)
	if !strings.HasPrefix(s, "attachments.") || !strings.HasSuffix(s, ".filename") {
		return 0, false
	}
	medio := strings.TrimSuffix(strings.TrimPrefix(s, "attachments."), ".filename")
	var i int
	if _, err := fmt.Sscanf(medio, "%d", &i); err != nil {
		return 0, false
	}
	return i, true
}

func rutaMetadata(clave string) modelo.CampoTexto {
	return modelo.CampoTexto("metadata." + clave)
}

func claveMetadata(ruta modelo.CampoTexto) (string, bool) {
	s := string(ruta)
	if !strings.HasPrefix(s, "metadata.") {
		return "", false
	}
	return strings.TrimPrefix(s, "metadata."), true
}

func rutaColumna(columna string) modelo.CampoTexto {
	return modelo.CampoTexto("columns." + columna)
}

func claveColumna(ruta modelo.CampoTexto) (string, bool) {
	s := string(ruta)
	if !strings.HasPrefix(s, "columns.") {
		return "", false
	}
	return strings.TrimPrefix(s, "columns."), true
}

func clavesOrdenadas(mapa map[string]string) []string {
	claves := make([]string, 0, len(mapa))
	for clave := range mapa {
		claves = append(claves, clave)
	}
	sort.Strings(claves)
	return claves
}
