// Package formas renderiza las formas de una persona a partir de su perfil.
//
// Es el mismo código para los dos lados del pipeline: el registro arma un
// modelo.Perfil con la identidad real (maestro, empleados, users) y pide sus
// formas reales; la fábrica arma el perfil falso y pide las falsas. Cada tipo
// tiene un único estilo canónico, deducido de sample/expected_output y de
// analisis/01_sample.md §1.2 y §5; los estilos alternativos de un mismo dato son
// tipos distintos (cédula, cédula_con_puntos, cedula_cc).
//
// El nombre completo del perfil trae nombre(s) de pila y dos apellidos, que es
// la convención del maestro y de los perfiles falsos: por eso el nombre corto
// es "quitar la última palabra" y el de pila, "quitar los dos apellidos". Un
// nombre de dos palabras (el real_name truncado de users.json) conserva ambos.
//
// No hay parser inverso: el registro no necesita adivinar datos desde una forma,
// porque los recibe estructurados del maestro.
package formas

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// ErrSinDato indica que el perfil no trae el dato que la forma necesita. Todas
// lo omite en vez de fallar: un perfil puede no tener placa ni pasaporte.
var ErrSinDato = errors.New("formas: falta el dato en el perfil")

// Tipos es el orden canónico de renderizado y lo que recorre Todas.
var Tipos = []modelo.TipoForma{
	modelo.FormaNombreCompleto,
	modelo.FormaNombreCorto,
	modelo.FormaNombrePila,
	modelo.FormaMayusculas,
	modelo.FormaSlug,
	modelo.FormaUsuario,
	modelo.FormaCorreo,
	modelo.FormaCorreoDeletreado,
	modelo.FormaCedula,
	modelo.FormaCedulaPuntos,
	modelo.FormaCedulaCC,
	modelo.FormaCelular,
	modelo.FormaCelularE164,
	modelo.FormaCelularLocal,
	modelo.FormaDireccion,
	modelo.FormaFechaISO,
	modelo.FormaFechaPalabras,
	modelo.FormaFechaUS,
	modelo.FormaPlaca,
	modelo.FormaPasaporte,
}

// Render devuelve la forma t del perfil en su estilo canónico. Registro queda
// vacío: lo llena quien descubrió la forma en un registro.
func Render(p modelo.Perfil, t modelo.TipoForma) (modelo.Forma, error) {
	valor, err := valor(p, t)
	if err != nil {
		return modelo.Forma{}, err
	}
	return modelo.Forma{Tipo: t, Valor: valor}, nil
}

// Todas devuelve las formas que el perfil permite renderizar, en el orden de
// Tipos.
func Todas(p modelo.Perfil) ([]modelo.Forma, error) {
	formas := make([]modelo.Forma, 0, len(Tipos))
	for _, t := range Tipos {
		f, err := Render(p, t)
		if errors.Is(err, ErrSinDato) {
			continue
		}
		if err != nil {
			return nil, err
		}
		formas = append(formas, f)
	}
	return formas, nil
}

func valor(p modelo.Perfil, t modelo.TipoForma) (string, error) {
	switch t {
	case modelo.FormaNombreCompleto:
		return nombreCompleto(p.Nombre)
	case modelo.FormaNombreCorto:
		return nombreCorto(p.Nombre)
	case modelo.FormaNombrePila:
		return nombrePila(p.Nombre)
	case modelo.FormaMayusculas:
		nombre, err := nombreCompleto(p.Nombre)
		if err != nil {
			return "", err
		}
		return strings.ToUpper(sinTildes(nombre)), nil
	case modelo.FormaSlug:
		return slug(p.Nombre)
	case modelo.FormaUsuario:
		return usuario(p)
	case modelo.FormaCorreo:
		return correo(p)
	case modelo.FormaCorreoDeletreado:
		return correoDeletreado(p)
	case modelo.FormaCedula:
		return cedula(p)
	case modelo.FormaCedulaPuntos:
		d, err := cedula(p)
		if err != nil {
			return "", err
		}
		return agrupar(d), nil
	case modelo.FormaCedulaCC:
		d, err := cedula(p)
		if err != nil {
			return "", err
		}
		return "CC" + d, nil
	case modelo.FormaCelular:
		return celular(p)
	case modelo.FormaCelularE164:
		d, err := celular(p)
		if err != nil {
			return "", err
		}
		if len(d) != 10 {
			return "", fmt.Errorf("formas: celular %q no tiene 10 dígitos", d)
		}
		return "+57 " + d[:3] + " " + d[3:6] + " " + d[6:], nil
	case modelo.FormaCelularLocal:
		d, err := celular(p)
		if err != nil {
			return "", err
		}
		if len(d) != 10 {
			return "", fmt.Errorf("formas: celular %q no tiene 10 dígitos", d)
		}
		return d[:3] + "-" + d[3:6] + "-" + d[6:], nil
	case modelo.FormaDireccion:
		return texto(p.Direccion, "dirección")
	case modelo.FormaFechaISO:
		f, err := nacimiento(p)
		if err != nil {
			return "", err
		}
		return f.Format("2006-01-02"), nil
	case modelo.FormaFechaPalabras:
		f, err := nacimiento(p)
		if err != nil {
			return "", err
		}
		meses := [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio",
			"agosto", "septiembre", "octubre", "noviembre", "diciembre"}
		return fmt.Sprintf("%d de %s de %d", f.Day(), meses[int(f.Month())-1], f.Year()), nil
	case modelo.FormaFechaUS:
		f, err := nacimiento(p)
		if err != nil {
			return "", err
		}
		return f.Format("01/02/2006"), nil
	case modelo.FormaPlaca:
		return texto(p.Placa, "placa")
	case modelo.FormaPasaporte:
		return texto(p.Pasaporte, "pasaporte")
	}
	return "", fmt.Errorf("formas: tipo de forma desconocido %q", t)
}

func nombreCompleto(nombre string) (string, error) {
	palabras := strings.Fields(nombre)
	if len(palabras) == 0 {
		return "", fmt.Errorf("%w: nombre", ErrSinDato)
	}
	return strings.Join(palabras, " "), nil
}

func nombreCorto(nombre string) (string, error) {
	palabras := strings.Fields(nombre)
	switch len(palabras) {
	case 0:
		return "", fmt.Errorf("%w: nombre", ErrSinDato)
	case 1, 2:
		return strings.Join(palabras, " "), nil
	}
	return strings.Join(palabras[:len(palabras)-1], " "), nil
}

func nombrePila(nombre string) (string, error) {
	palabras := strings.Fields(nombre)
	switch len(palabras) {
	case 0:
		return "", fmt.Errorf("%w: nombre", ErrSinDato)
	case 1, 2:
		return palabras[0], nil
	}
	return strings.Join(palabras[:len(palabras)-2], " "), nil
}

// slug es el token de filename ("manuela_castano", "certificado_laboral_..."
// del corpus): nombre corto en minúsculas, sin tildes y con guion bajo.
func slug(nombre string) (string, error) {
	corto, err := nombreCorto(nombre)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	separado := true
	for _, r := range strings.ToLower(sinTildes(corto)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			separado = false
			continue
		}
		if !separado {
			b.WriteByte('_')
			separado = true
		}
	}
	return strings.TrimSuffix(b.String(), "_"), nil
}

// usuario es el name de users.json: el local del correo ("nbermudez",
// "camila.arango"). Handle lo trae el perfil cuando la fuente ya lo declara.
func usuario(p modelo.Perfil) (string, error) {
	if h := strings.TrimSpace(p.Handle); h != "" {
		return h, nil
	}
	c, err := correo(p)
	if err != nil {
		return "", fmt.Errorf("%w: usuario", ErrSinDato)
	}
	return c[:strings.LastIndex(c, "@")], nil
}

func correo(p modelo.Perfil) (string, error) {
	c := strings.TrimSpace(p.Correo)
	if c == "" {
		return "", fmt.Errorf("%w: correo", ErrSinDato)
	}
	return c, nil
}

// correoDeletreado es la forma del chat ("juliana punto mejia arroba gmail punto
// com"): puntos por " punto " y arroba por " arroba ", el resto igual.
func correoDeletreado(p modelo.Perfil) (string, error) {
	c, err := correo(p)
	if err != nil {
		return "", err
	}
	arroba := strings.LastIndex(c, "@")
	if arroba <= 0 || arroba == len(c)-1 {
		return "", fmt.Errorf("formas: correo %q sin arroba válida", c)
	}
	local := strings.ReplaceAll(c[:arroba], ".", " punto ")
	dominio := strings.ReplaceAll(c[arroba+1:], ".", " punto ")
	return local + " arroba " + dominio, nil
}

func cedula(p modelo.Perfil) (string, error) {
	d := soloDigitosDe(p.Cedula)
	if d == "" {
		return "", fmt.Errorf("%w: cédula", ErrSinDato)
	}
	return d, nil
}

// agrupar pone puntos de miles desde la derecha: 1107360039 → 1.107.360.039,
// 32390356 → 32.390.356, como aparece en el chat.
func agrupar(d string) string {
	var grupos []string
	for len(d) > 3 {
		grupos = append([]string{d[len(d)-3:]}, grupos...)
		d = d[:len(d)-3]
	}
	return strings.Join(append([]string{d}, grupos...), ".")
}

// celular devuelve los 10 dígitos del celular; acepta el prefijo país.
func celular(p modelo.Perfil) (string, error) {
	d := soloDigitosDe(p.Celular)
	if d == "" {
		return "", fmt.Errorf("%w: celular", ErrSinDato)
	}
	if len(d) == 12 && strings.HasPrefix(d, "57") {
		d = d[2:]
	}
	return d, nil
}

func nacimiento(p modelo.Perfil) (time.Time, error) {
	s := strings.TrimSpace(p.Nacimiento)
	if s == "" {
		return time.Time{}, fmt.Errorf("%w: fecha de nacimiento", ErrSinDato)
	}
	f, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("formas: fecha de nacimiento %q no está en ISO: %w", s, err)
	}
	return f, nil
}

func texto(valor, campo string) (string, error) {
	v := strings.TrimSpace(valor)
	if v == "" {
		return "", fmt.Errorf("%w: %s", ErrSinDato, campo)
	}
	return v, nil
}

func soloDigitosDe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sinTildes quita las tildes del español conservando la caja, para el slug y
// para la forma en MAYÚSCULAS.
var acentos = map[rune]rune{
	'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'ã': 'a',
	'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e',
	'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i',
	'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'õ': 'o',
	'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u',
	'ñ': 'n', 'ç': 'c',
}

func sinTildes(s string) string {
	return strings.Map(func(r rune) rune {
		if base, ok := acentos[unicode.ToLower(r)]; ok {
			if unicode.IsUpper(r) {
				return unicode.ToUpper(base)
			}
			return base
		}
		return r
	}, s)
}
