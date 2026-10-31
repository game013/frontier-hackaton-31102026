package docs

import (
	"html"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MarkdownDeHTML convierte el HTML del corpus a markdown y extrae su metadata:
// <title> y los <meta name> (author, description, generator). Conserva los
// comentarios <!-- --> y los bloques con display:none (CE-06): ahí también hay
// PII que el núcleo tiene que anonimizar, así que la ingesta no puede perderla.
func MarkdownDeHTML(fuente []byte) (string, map[string]string, error) {
	raiz := arbolHTML(string(fuente))
	dibujo := &dibujoHTML{metadata: map[string]string{}}
	dibujo.bloquesDe(raiz)
	cuerpo := ""
	if len(dibujo.bloques) > 0 {
		cuerpo = strings.Join(dibujo.bloques, "\n\n") + "\n"
	}
	return cuerpo, dibujo.metadata, nil
}

// nodo es un nodo del árbol mínimo que necesita la conversión: etiqueta, sus
// atributos y los hijos. Los comentarios y el texto son hojas.
type nodo struct {
	etiqueta   string
	atributos  map[string]string
	crudo      string // etiqueta de apertura tal cual venía
	texto      string // solo en hojas
	comentario bool
	hijos      []*nodo
}

// vacias son las etiquetas sin contenido ni cierre.
var vacias = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "hr": true, "img": true,
	"input": true, "link": true, "meta": true, "source": true, "wbr": true,
}

// arbolHTML arma el árbol del documento. No valida: el corpus sale del mismo
// generador y está bien formado.
func arbolHTML(fuente string) *nodo {
	raiz := &nodo{etiqueta: "#raiz"}
	pila := []*nodo{raiz}
	abierto := func() *nodo { return pila[len(pila)-1] }

	resto := fuente
	for resto != "" {
		switch {
		case strings.HasPrefix(resto, "<!--"):
			fin := strings.Index(resto[4:], "-->")
			comentario := resto
			if fin >= 0 {
				comentario = resto[:4+fin+3]
				resto = resto[4+fin+3:]
			} else {
				resto = ""
			}
			abierto().hijos = append(abierto().hijos, &nodo{comentario: true, texto: comentario})
		case strings.HasPrefix(resto, "<!"), strings.HasPrefix(resto, "<?"):
			fin := strings.IndexByte(resto, '>')
			if fin < 0 {
				resto = ""
			} else {
				resto = resto[fin+1:]
			}
		case strings.HasPrefix(resto, "</"):
			fin := strings.IndexByte(resto, '>')
			if fin < 0 {
				resto = ""
				continue
			}
			nombre := strings.ToLower(strings.TrimSpace(resto[2:fin]))
			for i := len(pila) - 1; i > 0; i-- {
				if pila[i].etiqueta == nombre {
					pila = pila[:i]
					break
				}
			}
			resto = resto[fin+1:]
		case strings.HasPrefix(resto, "<"):
			fin := finEtiqueta(resto)
			if fin < 0 {
				abierto().hijos = append(abierto().hijos, &nodo{texto: resto})
				resto = ""
				continue
			}
			etiqueta, atributos := cortaEtiqueta(resto[1:fin])
			crudo := resto[:fin+1]
			resto = resto[fin+1:]
			nuevo := &nodo{etiqueta: etiqueta, atributos: atributos, crudo: crudo}
			abierto().hijos = append(abierto().hijos, nuevo)
			if !vacias[etiqueta] {
				pila = append(pila, nuevo)
			}
		default:
			fin := strings.IndexByte(resto, '<')
			texto := resto
			if fin >= 0 {
				texto = resto[:fin]
				resto = resto[fin:]
			} else {
				resto = ""
			}
			abierto().hijos = append(abierto().hijos, &nodo{texto: html.UnescapeString(texto)})
		}
	}
	return raiz
}

// finEtiqueta devuelve la posición del > que cierra la etiqueta, sin confundirlo
// con los > que van dentro de un valor entre comillas.
func finEtiqueta(resto string) int {
	var comilla byte
	for i := 1; i < len(resto); i++ {
		switch caracter := resto[i]; {
		case comilla != 0:
			if caracter == comilla {
				comilla = 0
			}
		case caracter == '"' || caracter == '\'':
			comilla = caracter
		case caracter == '>':
			return i
		}
	}
	return -1
}

// cortaEtiqueta separa el nombre de la etiqueta de sus atributos.
func cortaEtiqueta(interior string) (string, map[string]string) {
	interior = strings.TrimSuffix(strings.TrimSpace(interior), "/")
	nombre := interior
	siguiente := len(interior)
	for i, caracter := range interior {
		if caracter == ' ' || caracter == '\t' || caracter == '\n' || caracter == '\r' {
			nombre, siguiente = interior[:i], i
			break
		}
	}
	return strings.ToLower(nombre), atributosDe(interior[siguiente:])
}

// atributosDe lee los atributos de una etiqueta; las claves van en minúscula.
func atributosDe(texto string) map[string]string {
	atributos := map[string]string{}
	i := 0
	for i < len(texto) {
		for i < len(texto) && (texto[i] == ' ' || texto[i] == '\t' || texto[i] == '\n' || texto[i] == '\r' || texto[i] == '/') {
			i++
		}
		inicio := i
		for i < len(texto) && texto[i] != '=' && texto[i] != ' ' && texto[i] != '\t' && texto[i] != '\n' && texto[i] != '\r' && texto[i] != '/' {
			i++
		}
		if inicio == i {
			break
		}
		clave := strings.ToLower(texto[inicio:i])
		for i < len(texto) && (texto[i] == ' ' || texto[i] == '\t' || texto[i] == '\n' || texto[i] == '\r') {
			i++
		}
		valor := ""
		if i < len(texto) && texto[i] == '=' {
			i++
			for i < len(texto) && (texto[i] == ' ' || texto[i] == '\t' || texto[i] == '\n' || texto[i] == '\r') {
				i++
			}
			if i < len(texto) && (texto[i] == '"' || texto[i] == '\'') {
				comilla := texto[i]
				i++
				inicio = i
				for i < len(texto) && texto[i] != comilla {
					i++
				}
				valor = texto[inicio:i]
				if i < len(texto) {
					i++
				}
			} else {
				inicio = i
				for i < len(texto) && texto[i] != ' ' && texto[i] != '\t' && texto[i] != '\n' && texto[i] != '\r' {
					i++
				}
				valor = texto[inicio:i]
			}
		}
		atributos[clave] = html.UnescapeString(valor)
	}
	return atributos
}

// dibujoHTML acumula los bloques markdown y la metadata mientras recorre el
// árbol.
type dibujoHTML struct {
	bloques  []string
	metadata map[string]string
}

// titulos son las etiquetas de encabezado por nivel.
var titulos = map[string]int{"h1": 1, "h2": 2, "h3": 3, "h4": 4, "h5": 5, "h6": 6}

// bloquesDe recorre un nodo contenedor y agrega un bloque por cada hijo con
// contenido de nivel de bloque.
func (d *dibujoHTML) bloquesDe(n *nodo) {
	for _, hijo := range n.hijos {
		d.bloqueDe(hijo)
	}
}

func (d *dibujoHTML) bloqueDe(n *nodo) {
	if n.comentario {
		d.agrega(n.texto)
		return
	}
	if n.etiqueta == "" {
		d.agrega(colapsa(n.texto))
		return
	}
	if nivel, esTitulo := titulos[n.etiqueta]; esTitulo {
		d.agrega(strings.Repeat("#", nivel) + " " + inlineDe(n.hijos))
		return
	}
	switch n.etiqueta {
	case "head":
		d.cabeceraDe(n)
	case "style", "script", "noscript":
		// El estilo del generador no es contenido del documento.
	case "p", "blockquote":
		d.agrega(inlineDe(n.hijos))
	case "ul", "ol":
		d.listaDe(n)
	case "table":
		d.tablaDe(n)
	case "hr":
		d.agrega("---")
	case "div", "section", "article", "main", "body", "html", "#raiz":
		if oculto(n) {
			d.agrega(n.crudo + inlineDe(n.hijos) + "</" + n.etiqueta + ">")
			return
		}
		d.bloquesDe(n)
	case "br":
		// Un <br> suelto solo separa líneas dentro de un bloque.
	default:
		d.agrega(inlineDe([]*nodo{n}))
	}
}

// cabeceraDe toma el title y los <meta name> del <head>.
func (d *dibujoHTML) cabeceraDe(n *nodo) {
	for _, hijo := range n.hijos {
		switch hijo.etiqueta {
		case "title":
			if valor := colapsa(textoPlano(hijo)); valor != "" {
				d.metadata["title"] = valor
			}
		case "meta":
			nombre := strings.ToLower(strings.TrimSpace(hijo.atributos["name"]))
			if nombre == "" {
				continue
			}
			if valor := colapsa(hijo.atributos["content"]); valor != "" {
				d.metadata[nombre] = valor
			}
		}
	}
}

// listaDe arma un bloque con los items de una lista, uno por línea.
func (d *dibujoHTML) listaDe(n *nodo) {
	var lineas []string
	for _, hijo := range n.hijos {
		if hijo.etiqueta != "li" {
			continue
		}
		marca := "- "
		if n.etiqueta == "ol" {
			marca = strconv.Itoa(len(lineas)+1) + ". "
		}
		lineas = append(lineas, marca+inlineDe(hijo.hijos))
	}
	if len(lineas) > 0 {
		d.agrega(strings.Join(lineas, "\n"))
	}
}

// tablaDe arma una tabla de pipes con la primera fila como cabecera.
func (d *dibujoHTML) tablaDe(n *nodo) {
	var filas [][]string
	var acumula func(*nodo)
	acumula = func(contenedor *nodo) {
		for _, hijo := range contenedor.hijos {
			switch hijo.etiqueta {
			case "tr":
				var celdas []string
				for _, celda := range hijo.hijos {
					if celda.etiqueta == "td" || celda.etiqueta == "th" {
						celdas = append(celdas, colapsa(inlineDe(celda.hijos)))
					}
				}
				if len(celdas) > 0 {
					filas = append(filas, celdas)
				}
			case "thead", "tbody", "tfoot":
				acumula(hijo)
			}
		}
	}
	acumula(n)
	if len(filas) == 0 {
		return
	}

	ancho := 0
	for _, fila := range filas {
		if len(fila) > ancho {
			ancho = len(fila)
		}
	}
	var lineas []string
	for i, fila := range filas {
		for len(fila) < ancho {
			fila = append(fila, " ")
		}
		lineas = append(lineas, "| "+strings.Join(fila, " | ")+" |")
		if i == 0 {
			separador := make([]string, ancho)
			for j := range separador {
				separador[j] = "---"
			}
			lineas = append(lineas, "| "+strings.Join(separador, " | ")+" |")
		}
	}
	d.agrega(strings.Join(lineas, "\n"))
}

// agrega suma un bloque si tiene contenido, sin espacios de sobra.
func (d *dibujoHTML) agrega(bloque string) {
	lineas := strings.Split(bloque, "\n")
	for i, linea := range lineas {
		lineas[i] = strings.TrimSpace(linea)
	}
	bloque = strings.Join(lineas, "\n")
	if strings.TrimSpace(bloque) == "" {
		return
	}
	d.bloques = append(d.bloques, bloque)
}

// inlineDe convierte el contenido de un nodo a markdown en línea.
func inlineDe(nodos []*nodo) string {
	var salida strings.Builder
	for _, n := range nodos {
		if n.comentario {
			salida.WriteString(n.texto)
			continue
		}
		if n.etiqueta == "" {
			salida.WriteString(espacios(n.texto))
			continue
		}
		switch n.etiqueta {
		case "br":
			salida.WriteString("\n")
		case "a":
			direccion := n.atributos["href"]
			texto := inlineDe(n.hijos)
			if direccion == "" {
				salida.WriteString(texto)
			} else {
				salida.WriteString("[" + texto + "](" + direccion + ")")
			}
		case "strong", "b":
			salida.WriteString("**" + inlineDe(n.hijos) + "**")
		case "em", "i":
			salida.WriteString("*" + inlineDe(n.hijos) + "*")
		case "code":
			salida.WriteString("`" + inlineDe(n.hijos) + "`")
		case "style", "script", "noscript":
			// No es contenido.
		default:
			salida.WriteString(inlineDe(n.hijos))
		}
	}
	return salida.String()
}

// textoPlano junta el texto de un nodo, sin marcas de markdown.
func textoPlano(n *nodo) string {
	var salida strings.Builder
	for _, hijo := range n.hijos {
		if hijo.etiqueta == "" && !hijo.comentario {
			salida.WriteString(hijo.texto)
			continue
		}
		if hijo.etiqueta != "" {
			salida.WriteString(textoPlano(hijo))
		}
	}
	return salida.String()
}

// oculto dice si el nodo se dibuja con display:none.
func oculto(n *nodo) bool {
	estilo := strings.ToLower(strings.ReplaceAll(n.atributos["style"], " ", ""))
	return strings.Contains(estilo, "display:none")
}

// colapsa convierte cualquier racha de espacios en uno solo.
func colapsa(texto string) string {
	return strings.Join(strings.Fields(texto), " ")
}

// espacios colapsa las rachas de espacios sin recortar los extremos: en línea,
// el espacio que separa una palabra de una marca de markdown viene del texto.
func espacios(texto string) string {
	palabras := strings.Fields(texto)
	if len(palabras) == 0 {
		if texto == "" {
			return ""
		}
		return " "
	}
	var salida strings.Builder
	if primera, _ := utf8.DecodeRuneInString(texto); unicode.IsSpace(primera) {
		salida.WriteByte(' ')
	}
	salida.WriteString(strings.Join(palabras, " "))
	if ultima, _ := utf8.DecodeLastRuneInString(texto); unicode.IsSpace(ultima) {
		salida.WriteByte(' ')
	}
	return salida.String()
}
