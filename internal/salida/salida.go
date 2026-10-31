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
	"strings"
	"time"
	"unicode"

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
// reales, los seudónimos que efectivamente quedaron escritos y los registros
// donde aparece; más una entrada conservada por cada entidad de la allowlist.
//
// De cada tipo de seudónimo se declara el valor que aparece en más registros: el
// validador exige que el valor declarado esté en la salida, y los nombres se
// usan en formas distintas (completo en la fila del maestro, corto en el autor
// del chat). Declarar el completo cuando solo se usó el corto miente.
func Entidades(indice *registro.Indice, menciones []modelo.Mencion, perfiles map[string]modelo.Perfil, allow *detecta.Allowlist, sinDueno map[string]bool) []modelo.Entity {
	return EntidadesConPrevias(indice, menciones, perfiles, allow, sinDueno, nil)
}

// Previas son los seudónimos que declaró la corrida anterior: llave de persona
// → campo de pseudonyms → valor.
type Previas map[string]map[string]string

// EntidadesConPrevias es Entidades anclada a la corrida anterior: si un valor
// declarado antes sigue en los registros de la persona, se vuelve a declarar
// aunque ahora otra variante sea más frecuente (CE-46).
func EntidadesConPrevias(indice *registro.Indice, menciones []modelo.Mencion, perfiles map[string]modelo.Perfil, allow *detecta.Allowlist, sinDueno map[string]bool, previas Previas) []modelo.Entity {
	registros := map[string]map[string]bool{}
	valores := map[string]map[string]map[string]map[string]bool{}
	if sinDueno == nil {
		sinDueno = map[string]bool{}
	}
	orden := map[string]bool{}
	for _, m := range menciones {
		if m.Persona == "" {
			// Una mención sin dueño sigue en el texto: no se declara como
			// reemplazada (D25: un nombre de pila suelto no se barre).
			sinDueno[norm.Norm(m.Valor)] = true
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
		clave, valor, ok := pseudonimoDe(perfil, m.Forma)
		if !ok || valor == "" {
			continue
		}
		if valores[m.Persona] == nil {
			valores[m.Persona] = map[string]map[string]map[string]bool{}
		}
		if valores[m.Persona][clave] == nil {
			valores[m.Persona][clave] = map[string]map[string]bool{}
		}
		if valores[m.Persona][clave][valor] == nil {
			valores[m.Persona][clave][valor] = map[string]bool{}
		}
		valores[m.Persona][clave][valor][m.Registro] = true
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
		seudonimos := eligeValores(valores[llave], previas[llave])
		if len(seudonimos) == 0 {
			continue
		}
		out = append(out, modelo.Entity{
			EntityID:   llave,
			Type:       modelo.EntidadPersona,
			Originals:  formasReales(p, sinDueno),
			Pseudonyms: seudonimos,
			Records:    listaOrdenada(registros[llave]),
		})
	}
	out = append(out, conservadas(allow)...)
	return out
}

// eligeValores se queda con el valor de cada tipo que aparece en más registros;
// en caso de empate gana la forma más larga, que suele ser el nombre completo.
func eligeValores(porClave map[string]map[string]map[string]bool, previo map[string]string) map[string]string {
	out := map[string]string{}
	claves := make([]string, 0, len(porClave))
	for clave := range porClave {
		claves = append(claves, clave)
	}
	sort.Strings(claves)
	for _, clave := range claves {
		if antes, ok := previo[clave]; ok && len(porClave[clave][antes]) > 0 {
			out[clave] = antes
			continue
		}
		mejor, mejorCuenta := "", -1
		valores := make([]string, 0, len(porClave[clave]))
		for valor := range porClave[clave] {
			valores = append(valores, valor)
		}
		sort.Strings(valores)
		for _, valor := range valores {
			cuenta := len(porClave[clave][valor])
			if cuenta > mejorCuenta || (cuenta == mejorCuenta && len(valor) > len(mejor)) {
				mejor, mejorCuenta = valor, cuenta
			}
		}
		if mejor != "" {
			out[clave] = mejor
		}
	}
	return out
}

// pseudonimoDe traduce el tipo de forma al campo de pseudonyms del contrato.
func pseudonimoDe(perfil modelo.Perfil, tipo modelo.TipoForma) (string, string, bool) {
	if tipo == "" {
		// Candidato sin estilo reconocido: la detección no supo qué forma era,
		// así que se declara el nombre completo solo si el reemplazo lo usó.
		return "name", perfil.Nombre, perfil.Nombre != ""
	}
	switch tipo {
	case modelo.FormaNombreCompleto:
		return "name", perfil.Nombre, perfil.Nombre != ""
	case modelo.FormaNombreCorto, modelo.FormaNombrePila,
		modelo.FormaMayusculas, modelo.FormaSlug, modelo.FormaUsuario:
		// El valor declarado tiene que ser el que quedó en el registro: un
		// slug o un nombre corto no pueden sustituirse por el nombre completo.
		forma, err := formas.Render(perfil, tipo)
		if err != nil {
			return "name", perfil.Nombre, perfil.Nombre != ""
		}
		return "name", forma.Valor, forma.Valor != ""
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
// Deja fuera las que la resolución no pudo atribuir: declararlas sería declarar
// una sustitución que no ocurrió (D25, J-DIC).
func formasReales(p modelo.Persona, sinDueno map[string]bool) []string {
	vistas := map[string]bool{}
	var out []string
	for _, f := range p.Formas {
		if f.Valor == "" || vistas[f.Valor] || sinDueno[norm.Norm(f.Valor)] {
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

// LeePrevias carga los seudónimos declarados en un entities.json anterior. Solo
// las personas: las conservadas no tienen seudónimo que anclar.
func LeePrevias(ruta string) (Previas, error) {
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		return nil, fmt.Errorf("salida: previas: %w", err)
	}
	var doc struct {
		Entities []modelo.Entity `json:"entities"`
	}
	if err := json.Unmarshal(crudo, &doc); err != nil {
		return nil, fmt.Errorf("salida: previas %s: %w", ruta, err)
	}
	out := Previas{}
	for _, e := range doc.Entities {
		if e.Type != modelo.EntidadPersona || len(e.Pseudonyms) == 0 {
			continue
		}
		out[e.EntityID] = e.Pseudonyms
	}
	return out, nil
}

// PreviasMasRecientes busca el entities.json más reciente en las carpetas de
// corrida bajo raiz, sin contar la carpeta excluida (la corrida en curso). Sin
// corrida anterior devuelve nil sin error.
func PreviasMasRecientes(raiz, excluida string) (Previas, string, error) {
	entradas, err := os.ReadDir(raiz)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("salida: previas: %w", err)
	}
	mejor, mejorHora := "", time.Time{}
	for _, e := range entradas {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(raiz, e.Name())
		if filepath.Clean(dir) == filepath.Clean(excluida) {
			continue
		}
		info, err := os.Stat(filepath.Join(dir, "entities.json"))
		if err != nil {
			continue
		}
		if info.ModTime().After(mejorHora) {
			mejor, mejorHora = filepath.Join(dir, "entities.json"), info.ModTime()
		}
	}
	if mejor == "" {
		return nil, "", nil
	}
	previas, err := LeePrevias(mejor)
	return previas, mejor, err
}

// AnclaPrevias vuelve a declarar el valor de la corrida anterior cuando sigue
// escrito en alguno de los registros de la persona, aunque ya no salga de una
// mención contada (CE-46). Solo cambia valores que el validador acepta: el
// seudónimo tiene que estar en un registro de la propia entidad.
func AnclaPrevias(ents []modelo.Entity, recs []modelo.Registro, previas Previas) {
	if len(previas) == 0 {
		return
	}
	textos := make(map[string]string, len(recs))
	for _, r := range recs {
		crudo, err := json.Marshal(r)
		if err != nil {
			continue
		}
		// Se compara contra el texto sin escapar: un acento en JSON puede salir
		// como á si el registro lo trae así.
		textos[r.IDRegistro()] = desescapa(crudo)
	}
	for i := range ents {
		e := &ents[i]
		previo, ok := previas[e.EntityID]
		if !ok || e.Type != modelo.EntidadPersona {
			continue
		}
		for clave, valor := range previo {
			if valor == "" || e.Pseudonyms[clave] == valor {
				continue
			}
			for _, id := range e.Records {
				if strings.Contains(textos[id], valor) {
					e.Pseudonyms[clave] = valor
					break
				}
			}
		}
	}
}

// desescapa devuelve los strings de un JSON concatenados, sin escapes.
func desescapa(crudo []byte) string {
	var v any
	if err := json.Unmarshal(crudo, &v); err != nil {
		return string(crudo)
	}
	var b strings.Builder
	var recorre func(any)
	recorre = func(x any) {
		switch t := x.(type) {
		case string:
			b.WriteString(t)
			b.WriteByte('\n')
		case map[string]any:
			for _, y := range t {
				recorre(y)
			}
		case []any:
			for _, y := range t {
				recorre(y)
			}
		}
	}
	recorre(v)
	return b.String()
}

// textosPorID arma el texto plano de cada registro, sin escapes de JSON.
func textosPorID(recs []modelo.Registro) map[string]string {
	textos := make(map[string]string, len(recs))
	for _, r := range recs {
		crudo, err := json.Marshal(r)
		if err != nil {
			continue
		}
		textos[r.IDRegistro()] = desescapa(crudo)
	}
	return textos
}

// PodaAusentes quita los seudónimos declarados que no aparecen en ningún
// registro de la persona: el validador exige que estén, y uno ausente es un
// seudónimo viejo que la compuerta tuvo que cambiar porque coincidía con un
// dato real llegado con el crecimiento (CE-47).
func PodaAusentes(ents []modelo.Entity, recs []modelo.Registro) {
	textos := textosPorID(recs)
	for i := range ents {
		e := &ents[i]
		if e.Type != modelo.EntidadPersona {
			continue
		}
		for clave, valor := range e.Pseudonyms {
			presente := false
			for _, id := range e.Records {
				if estaEn(textos[id], valor) {
					presente = true
					break
				}
			}
			if !presente && len(e.Pseudonyms) > 1 {
				delete(e.Pseudonyms, clave)
			}
		}
	}
}

// estaEn decide si un seudónimo está en el texto como lo vería el validador:
// con letras, contiene() con bordes (un correo dentro de otro no cuenta); solo
// dígitos y signos, coincidencia directa, porque la regla de dígitos de
// contiene() descarta fechas y cédulas que sí están escritas tal cual.
func estaEn(texto, valor string) bool {
	if strings.IndexFunc(valor, unicode.IsLetter) >= 0 {
		return norm.Contiene(texto, valor)
	}
	return strings.Contains(texto, valor)
}
