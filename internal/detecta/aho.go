package detecta

// ac es un Aho-Corasick mínimo sobre bytes: recorre el texto una sola vez y
// avisa de cada aguja que termina en cada posición. La detección de formas
// conocidas corre sobre el texto normalizado del jurado (D5), que es ASCII en
// la práctica; las agujas ya vienen normalizadas.
type ac struct {
	hijos   []map[byte]int
	fallo   []int
	salidas [][]int
	largos  []int
}

func nuevoAC(agujas [][]byte) *ac {
	a := &ac{
		hijos:   []map[byte]int{{}},
		fallo:   []int{0},
		salidas: [][]int{{}},
		largos:  make([]int, len(agujas)),
	}
	for i, aguja := range agujas {
		a.largos[i] = len(aguja)
		estado := 0
		for _, b := range aguja {
			siguiente, ok := a.hijos[estado][b]
			if !ok {
				siguiente = len(a.hijos)
				a.hijos = append(a.hijos, map[byte]int{})
				a.fallo = append(a.fallo, 0)
				a.salidas = append(a.salidas, nil)
				a.hijos[estado][b] = siguiente
			}
			estado = siguiente
		}
		a.salidas[estado] = append(a.salidas[estado], i)
	}
	a.enlaza()
	return a
}

// enlaza calcula los enlaces de fallo en anchura y hereda las salidas del
// estado al que se cae, para no recorrer la cadena en cada byte.
func (a *ac) enlaza() {
	cola := make([]int, 0, len(a.hijos))
	for _, hijo := range a.hijos[0] {
		a.fallo[hijo] = 0
		cola = append(cola, hijo)
	}
	for len(cola) > 0 {
		estado := cola[0]
		cola = cola[1:]
		for b, hijo := range a.hijos[estado] {
			destino := a.fallo[estado]
			for destino != 0 {
				if siguiente, ok := a.hijos[destino][b]; ok {
					destino = siguiente
					break
				}
				destino = a.fallo[destino]
			}
			if siguiente, ok := a.hijos[destino][b]; ok && siguiente != hijo {
				destino = siguiente
			}
			a.fallo[hijo] = destino
			a.salidas[hijo] = append(a.salidas[hijo], a.salidas[destino]...)
			cola = append(cola, hijo)
		}
	}
}

// coincidencia es una aguja que termina en una posición del texto.
type coincidencia struct {
	inicio int
	fin    int
	aguja  int
}

// busca devuelve todas las coincidencias, en orden de fin. La lista puede
// solaparse: quien llama decide cuál se queda.
func (a *ac) busca(texto string) []coincidencia {
	var out []coincidencia
	estado := 0
	for i := 0; i < len(texto); i++ {
		b := texto[i]
		for {
			if siguiente, ok := a.hijos[estado][b]; ok {
				estado = siguiente
				break
			}
			if estado == 0 {
				break
			}
			estado = a.fallo[estado]
		}
		for _, aguja := range a.salidas[estado] {
			out = append(out, coincidencia{inicio: i + 1 - a.largos[aguja], fin: i + 1, aguja: aguja})
		}
	}
	return out
}
