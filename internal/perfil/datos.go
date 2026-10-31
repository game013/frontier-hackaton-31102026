package perfil

import (
	"strings"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// datosReales es lo que la fábrica necesita saber de la persona real para
// imitar su estructura: cuántos nombres de pila tiene, y qué identificadores
// exhibe (el perfil falso solo inventa placa, pasaporte y fecha si existen).
type datosReales struct {
	pila          int
	cedula        string
	nacimiento    time.Time
	hayNacimiento bool
	placa         bool
	pasaporte     bool
}

// leeDatos recorre las formas de la persona. El nombre completo manda para
// contar los nombres de pila; si no está, se deduce del corto (nombre(s) + un
// apellido) o del de pila.
func leeDatos(formas []modelo.Forma) datosReales {
	var completo, corto, pila string
	var cedula, cedulaPuntos, cedulaCC string
	var fechaISO, fechaUS string
	var r datosReales
	for _, f := range formas {
		if f.Valor == "" {
			continue
		}
		switch f.Tipo {
		case modelo.FormaNombreCompleto:
			completo = primero(completo, f.Valor)
		case modelo.FormaNombreCorto:
			corto = primero(corto, f.Valor)
		case modelo.FormaNombrePila:
			pila = primero(pila, f.Valor)
		case modelo.FormaCedula:
			cedula = primero(cedula, f.Valor)
		case modelo.FormaCedulaPuntos:
			cedulaPuntos = primero(cedulaPuntos, f.Valor)
		case modelo.FormaCedulaCC:
			cedulaCC = primero(cedulaCC, f.Valor)
		case modelo.FormaFechaISO:
			fechaISO = primero(fechaISO, f.Valor)
		case modelo.FormaFechaUS:
			fechaUS = primero(fechaUS, f.Valor)
		case modelo.FormaPlaca:
			r.placa = true
		case modelo.FormaPasaporte:
			r.pasaporte = true
		}
	}
	r.pila = nombresDePilaReales(completo, corto, pila)
	for _, valor := range []string{cedula, cedulaPuntos, cedulaCC} {
		if digitos := soloDigitos(valor); digitos != "" {
			r.cedula = digitos
			break
		}
	}
	// La fecha se imita a partir de la forma que se pueda leer: el registro
	// renderiza la ISO siempre que la conoce, y el formato US es el otro que
	// trae los números en orden fijo.
	if fecha, err := time.Parse("2006-01-02", fechaISO); err == nil {
		r.nacimiento, r.hayNacimiento = fecha, true
	} else if fecha, err := time.Parse("01/02/2006", fechaUS); err == nil {
		r.nacimiento, r.hayNacimiento = fecha, true
	}
	return r
}

// primero conserva el primer valor visto: el orden de las formas es estable y
// así el dato elegido no depende de dar vueltas.
func primero(actual, nuevo string) string {
	if actual != "" {
		return actual
	}
	return nuevo
}

// nombresDePilaReales cuenta los nombres de pila: el resto del nombre completo
// son los dos apellidos. El resultado queda entre uno y tres, que es lo que
// cabe en un nombre colombiano sin inventar estructura.
func nombresDePilaReales(completo, corto, pila string) int {
	cuantos := 1
	switch {
	case strings.TrimSpace(completo) != "":
		cuantos = len(strings.Fields(completo)) - 2
	case strings.TrimSpace(corto) != "":
		cuantos = len(strings.Fields(corto)) - 1
	case strings.TrimSpace(pila) != "":
		cuantos = len(strings.Fields(pila))
	}
	if cuantos < 1 {
		cuantos = 1
	}
	if cuantos > 3 {
		cuantos = 3
	}
	return cuantos
}

// soloDigitos deja solo los dígitos del valor: la cédula aparece con puntos, con
// el prefijo CC o pegada, y todas son la misma cédula.
func soloDigitos(valor string) string {
	var b strings.Builder
	for _, r := range valor {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
