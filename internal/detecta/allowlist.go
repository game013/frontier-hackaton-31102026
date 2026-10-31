package detecta

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/norm"
)

// Entrada es una entidad que se conserva: banco, marca, lugar, figura pública o
// identificador interno. El motivo viaja a entities.json (C26, D11).
type Entrada struct {
	Nombre string `json:"nombre"`
	Tipo   string `json:"tipo,omitempty"`
	Motivo string `json:"motivo,omitempty"`
}

// Allowlist es la lista de lo que no se toca, más las reglas genéricas que
// cubren lo que llegue con el crecimiento del corpus (C14, §6.1.3d).
type Allowlist struct {
	Entradas []Entrada
	// Sufijos conservan la razón social: "… S.A.S.".
	Sufijos []string
	// Prefijos conservan la entidad: "Banco …".
	Prefijos []string
	// ApellidoLugar son los apellidos que también son lugar o marca: solo se
	// conservan cuando no van precedidos de un nombre de pila.
	ApellidoLugar []string
	// Palabras son formas que coinciden con palabras comunes: nunca se tocan
	// sueltas ("Claro").
	Palabras  []string
	porNombre map[string]Entrada
}

// CargaJSON lee la allowlist de C26. Acepta el objeto con `entidades` y también
// una lista suelta, para que el archivo de datos pueda crecer sin romper el
// cargador; una entrada sin nombre o repetida es error, porque una allowlist
// silenciosamente incompleta es una fuga silenciosa.
func CargaJSON(datos []byte) (*Allowlist, error) {
	var sobre struct {
		Entidades     []Entrada `json:"entidades"`
		Sufijos       []string  `json:"sufijos"`
		Prefijos      []string  `json:"prefijos"`
		ApellidoLugar []string  `json:"apellido_lugar"`
		Palabras      []string  `json:"palabras"`
	}
	if err := json.Unmarshal(datos, &sobre); err != nil || (len(sobre.Entidades) == 0 && len(sobre.Sufijos) == 0 && len(sobre.Palabras) == 0) {
		var lista []Entrada
		if errLista := json.Unmarshal(datos, &lista); errLista != nil {
			if err != nil {
				return nil, fmt.Errorf("allowlist: JSON ilegible: %w", err)
			}
			return nil, fmt.Errorf("allowlist: sin entidades ni reglas")
		}
		sobre.Entidades = lista
	}
	return Nueva(sobre.Entidades, sobre.Sufijos, sobre.Prefijos, sobre.ApellidoLugar, sobre.Palabras)
}

// Nueva valida y arma una allowlist. Los nombres se comparan sin tildes ni
// mayúsculas, como todo el pipeline (D5).
func Nueva(entradas []Entrada, sufijos, prefijos, apellidoLugar, palabras []string) (*Allowlist, error) {
	a := &Allowlist{
		Entradas:      append([]Entrada{}, entradas...),
		Sufijos:       append([]string{}, sufijos...),
		Prefijos:      append([]string{}, prefijos...),
		ApellidoLugar: append([]string{}, apellidoLugar...),
		Palabras:      append([]string{}, palabras...),
		porNombre:     make(map[string]Entrada, len(entradas)),
	}
	for i, e := range entradas {
		if strings.TrimSpace(e.Nombre) == "" {
			return nil, fmt.Errorf("allowlist: entrada %d sin nombre", i)
		}
		clave := norm.Norm(e.Nombre)
		if _, repetida := a.porNombre[clave]; repetida {
			return nil, fmt.Errorf("allowlist: entidad repetida %q", e.Nombre)
		}
		a.porNombre[clave] = e
	}
	return a, nil
}

// PorDefecto es la allowlist que se usa cuando nadie pasa una: los datos de
// C26 (allowlist.json), compilados en el binario. Si ese archivo no cargara,
// queda la lista fija de reglas genéricas para que la corrida no se caiga.
func PorDefecto() *Allowlist {
	if a, err := Embebida(); err == nil {
		return a
	}
	return porDefectoFijo()
}

// porDefectoFijo es la red de seguridad de PorDefecto: las reglas genéricas y
// las marcas que nunca son personas (supuesto 4), sin depender del archivo.
func porDefectoFijo() *Allowlist {
	a, err := Nueva([]Entrada{
		// El asistente virtual y la marca no son personas (supuesto 4).
		{Nombre: "Kipi", Tipo: "marca", Motivo: "asistente virtual de Kipu Pay, no es persona"},
		{Nombre: "Kipu Pay", Tipo: "marca", Motivo: "marca de la empresa, no es persona"},
	},
		[]string{"S.A.S.", "S.A.", "LTDA", "E.U."},
		[]string{"Banco", "Banco de", "Fondo", "Caja"},
		[]string{"Santander", "Bolívar", "Botero", "Carulla", "Castilla", "Robledo", "Restrepo", "Galán", "Córdoba"},
		[]string{"Claro", "clara", "celeste"},
	)
	if err != nil {
		panic("allowlist por defecto inválida: " + err.Error())
	}
	return a
}

// Conserva dice si el valor se conserva. motivo explica por qué; en las reglas
// genéricas sale una frase corta, en las entidades, el motivo declarado.
func (a *Allowlist) Conserva(valor string) (string, bool) {
	if a == nil {
		return "", false
	}
	if e, ok := a.porNombre[norm.Norm(valor)]; ok {
		if e.Motivo != "" {
			return e.Motivo, true
		}
		return "organización, lugar, marca o figura pública", true
	}
	return "", false
}

// ConservaApellidoLugar decide el caso "Luis Santander … desde Santander": el
// apellido se conserva solo cuando no va precedido de un nombre de pila, que es
// la regla del catálogo de casos (CE-14).
func (a *Allowlist) ConservaApellidoLugar(valor, antes string) bool {
	if a == nil {
		return false
	}
	clave := norm.Norm(valor)
	esApellidoLugar := false
	for _, apellido := range a.ApellidoLugar {
		if norm.Norm(apellido) == clave {
			esApellidoLugar = true
			break
		}
	}
	if !esApellidoLugar {
		return false
	}
	return !terminaEnNombreDePila(antes)
}

// EsSufijoDeRazonSocial dice si el texto conserva la forma por ser parte de una
// razón social ("Veloz S.A.S.").
func (a *Allowlist) EsSufijoDeRazonSocial(antes, despues string) bool {
	if a == nil {
		return false
	}
	for _, sufijo := range a.Sufijos {
		if strings.HasPrefix(norm.Norm(despues), norm.Norm(sufijo)) {
			return true
		}
	}
	return false
}

// EsPrefijoDeEntidad dice si el texto conserva la forma por ir precedida de un
// prefijo de entidad ("Banco Davivienda").
func (a *Allowlist) EsPrefijoDeEntidad(antes string) bool {
	if a == nil {
		return false
	}
	n := strings.TrimSpace(norm.Norm(antes))
	for _, prefijo := range a.Prefijos {
		if strings.HasSuffix(n, norm.Norm(prefijo)) {
			return true
		}
	}
	return false
}

// EsPalabraComun dice si la forma coincide con una palabra del idioma y no se
// puede tocar suelta (CE-15).
func (a *Allowlist) EsPalabraComun(valor string) bool {
	if a == nil {
		return false
	}
	clave := norm.Norm(valor)
	for _, palabra := range a.Palabras {
		if norm.Norm(palabra) == clave {
			return true
		}
	}
	return false
}

// Canonica devuelve las entradas ordenadas para que entities.json sea estable.
func (a *Allowlist) Canonica() []Entrada {
	if a == nil {
		return nil
	}
	out := append([]Entrada{}, a.Entradas...)
	sort.SliceStable(out, func(i, j int) bool { return norm.Norm(out[i].Nombre) < norm.Norm(out[j].Nombre) })
	return out
}

// terminaEnNombreDePila dice si el texto termina en una palabra con mayúscula
// inicial que no es un título ni una preposición: la señal de "nombre + apellido".
func terminaEnNombreDePila(antes string) bool {
	campos := strings.Fields(strings.TrimSpace(antes))
	if len(campos) == 0 {
		return false
	}
	ultima := campos[len(campos)-1]
	noSonNombres := map[string]bool{
		"la": true, "el": true, "los": true, "las": true, "de": true, "del": true,
		"en": true, "con": true, "por": true, "desde": true, "hacia": true, "y": true,
		"señor": true, "señora": true, "don": true, "doña": true, "banco": true,
	}
	if noSonNombres[strings.ToLower(ultima)] {
		return false
	}
	r := []rune(ultima)[0]
	return r >= 'A' && r <= 'Z' || r == 'Á' || r == 'É' || r == 'Í' || r == 'Ó' || r == 'Ú' || r == 'Ñ'
}
