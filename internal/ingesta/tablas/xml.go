package tablas

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// leerTablaXML lee un XML donde cada registro es un elemento plano: sus
// atributos son columnas y cada hijo aporta su texto y sus atributos como
// <hijo>_<atributo> (documento tipo="CC" → documento + documento_tipo).
//
// Los comentarios y los atributos de la raíz no son columnas. Todas las filas
// salen con las mismas columnas: la unión de las que aparecen en el archivo, en
// orden de primera aparición, y "" donde un registro no la trae.
func leerTablaXML(ruta string, tabla tablaXML) ([]modelo.TableRecord, error) {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return nil, err
	}
	decodificador := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(datos, marcaOrdenBytes)))

	var filas []*fila
	for {
		token, err := decodificador.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ruta, err)
		}
		inicio, esInicio := token.(xml.StartElement)
		if !esInicio || inicio.Name.Local != tabla.elemento {
			continue
		}
		leida, err := leerRegistro(decodificador, inicio)
		if err != nil {
			return nil, fmt.Errorf("%s: <%s>: %w", ruta, tabla.elemento, err)
		}
		filas = append(filas, leida)
	}

	columnas := unionDeColumnas(filas)
	registros := make([]modelo.TableRecord, 0, len(filas))
	for i, leida := range filas {
		valores := make(map[string]string, len(columnas))
		for _, columna := range columnas {
			valores[columna] = leida.columnas[columna]
		}
		if valores[tabla.llave] == "" {
			return nil, fmt.Errorf("%s: el registro %d no trae el atributo llave %q", ruta, i+1, tabla.llave)
		}
		registros = append(registros, modelo.TableRecord{
			ID:      tabla.nombre + ":" + valores[tabla.llave],
			Table:   tabla.nombre,
			Columns: valores,
		})
	}
	return registros, nil
}

// leerRegistro consume un registro hasta su cierre. Los elementos anidados a más
// de un nivel son error: aplanarlos perdería el nombre del elemento interno.
func leerRegistro(decodificador *xml.Decoder, inicio xml.StartElement) (*fila, error) {
	leida := &fila{columnas: map[string]string{}}
	for _, atributo := range inicio.Attr {
		leida.agregar(atributo.Name.Local, atributo.Value)
	}

	profundidad := 0
	hijo := ""
	for {
		token, err := decodificador.Token()
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			profundidad++
			if profundidad > 1 {
				return nil, fmt.Errorf("el elemento <%s> está anidado en <%s>", token.Name.Local, hijo)
			}
			hijo = token.Name.Local
			// Un hijo vacío también es una columna, con texto "".
			leida.agregar(hijo, "")
			for _, atributo := range token.Attr {
				leida.agregar(hijo+"_"+atributo.Name.Local, atributo.Value)
			}
		case xml.CharData:
			if profundidad == 1 {
				leida.agregarTexto(hijo, string(token))
			}
		case xml.EndElement:
			if profundidad == 0 {
				return leida, nil
			}
			profundidad--
		}
	}
}

// fila es un registro XML a medio armar: el mapa de columnas y el orden en que
// aparecieron en el documento.
type fila struct {
	columnas map[string]string
	orden    []string
}

func (f *fila) agregar(columna, valor string) {
	if _, existe := f.columnas[columna]; !existe {
		f.orden = append(f.orden, columna)
	}
	f.columnas[columna] = valor
}

// agregarTexto acumula el texto de un hijo: XML puede partir el contenido en
// varios tramos (CDATA y texto suelto).
func (f *fila) agregarTexto(columna, texto string) {
	f.agregar(columna, f.columnas[columna]+texto)
}

func unionDeColumnas(filas []*fila) []string {
	vistas := map[string]bool{}
	var columnas []string
	for _, leida := range filas {
		for _, columna := range leida.orden {
			if vistas[columna] {
				continue
			}
			vistas[columna] = true
			columnas = append(columnas, columna)
		}
	}
	return columnas
}
