package perfil

import (
	"strconv"
	"strings"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/catalogo"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// dominioCorporativo es el de Kipu Pay: el empleado conserva el dominio de la
// empresa y el resto cae al pool público (analisis/01_sample.md §1.2).
const dominioCorporativo = "kipupay.co"

var (
	// vias, sufijos y complementos componen direcciones con el estilo de la
	// salida ideal: "Cl 108 # 66-81 casa 52", "Diagonal 41 Sur # 71-52 of 502".
	vias         = []string{"Calle", "Cl", "Cra", "Diagonal", "Tv", "Transversal"}
	sufijos      = []string{"", "A", "B", " Bis"}
	complementos = []string{"", " apto ", " of ", " casa "}
	// consonantes arma placas y pasaportes: las placas colombianas no llevan
	// vocales (LTE758, GTD596) y los pasaportes del corpus son dos letras.
	consonantes = []rune("BCDFGHJKLMNPQRSTVWXYZ")
)

// dominio es el del correo: el de la empresa para un empleado y uno público
// para el resto.
func (f *Fabrica) dominio(rol modelo.Rol, llave string, intento int) string {
	if rol == modelo.RolEmpleado {
		return dominioCorporativo
	}
	return catalogo.Dominio(f.numero(llave, "dominio", intento))
}

// correoDe arma "<inicial><apellido>@<dominio>" en minúsculas y sin tildes.
func correoDe(pila, apellido, dominio string) string {
	return inicial(pila) + plano(apellido) + "@" + dominio
}

// inicial es la primera letra del nombre de pila, sin tilde.
func inicial(nombre string) string {
	runas := []rune(plano(nombre))
	if len(runas) == 0 {
		return nombre[:1]
	}
	return string(runas[0])
}

// plano deja el texto en minúsculas y sin tildes, con solo letras y dígitos:
// es la forma que cabe en un correo y en un nombre de usuario.
func plano(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'á', 'à', 'â', 'ä', 'ã':
			r = 'a'
		case 'é', 'è', 'ê', 'ë':
			r = 'e'
		case 'í', 'ì', 'î', 'ï':
			r = 'i'
		case 'ó', 'ò', 'ô', 'ö', 'õ':
			r = 'o'
		case 'ú', 'ù', 'û', 'ü':
			r = 'u'
		case 'ñ':
			r = 'n'
		case 'ç':
			r = 'c'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// cifras devuelve los dígitos deterministas del campo: cada byte del HMAC
// aporta uno, y la etiqueta separa un campo del otro.
func (f *Fabrica) cifras(llave, etiqueta string, intento, cuantas int) string {
	mac := f.mac(llave, etiqueta, intento)
	var b strings.Builder
	for i := 0; b.Len() < cuantas; i++ {
		b.WriteByte('0' + mac[i%len(mac)]%10)
	}
	return b.String()
}

// celular es un móvil colombiano: 3xx de 10 dígitos, de otro operador.
func (f *Fabrica) celular(llave string, intento int) string {
	operador := byte(int('0') + f.numero(llave, "celular:operador", intento)%6)
	return "3" + string(operador) + f.cifras(llave, "celular", intento, 8)
}

// cedula conserva la longitud de la cédula real y, cuando la trae, sus dos
// primeros dígitos: la falsa cae en el mismo rango que la real (§1.2). Sin
// cédula real usa diez dígitos, la longitud más común del corpus.
func (f *Fabrica) cedula(llave, real string, intento int) string {
	largo := len(real)
	if largo < 6 || largo > 11 {
		largo = 10
	}
	cifras := []byte(f.cifras(llave, "cedula", intento, largo))
	cifras[0] = byte(int('1') + f.numero(llave, "cedula:inicial", intento)%9)
	if len(real) >= 8 {
		copy(cifras[:2], real[:2])
	}
	return string(cifras)
}

// direccion es una dirección colombiana sintética: tipo de vía, número, cruce
// con placa y, a veces, unidad cerrada.
func (f *Fabrica) direccion(llave string, intento int) string {
	var b strings.Builder
	b.WriteString(vias[f.numero(llave, "direccion:via", intento)%len(vias)])
	b.WriteString(" ")
	b.WriteString(strconv.Itoa(1 + f.numero(llave, "direccion:numero", intento)%150))
	b.WriteString(sufijos[f.numero(llave, "direccion:sufijo", intento)%len(sufijos)])
	if f.numero(llave, "direccion:sur", intento)%2 == 0 {
		b.WriteString(" Sur")
	}
	b.WriteString(" # ")
	b.WriteString(strconv.Itoa(1 + f.numero(llave, "direccion:cruce", intento)%98))
	b.WriteString("-")
	b.WriteString(strconv.Itoa(1 + f.numero(llave, "direccion:placa", intento)%98))
	if complemento := complementos[f.numero(llave, "direccion:complemento", intento)%len(complementos)]; complemento != "" {
		b.WriteString(complemento)
		b.WriteString(strconv.Itoa(10 + f.numero(llave, "direccion:unidad", intento)%990))
	}
	return b.String()
}

// nacimiento queda a pocos años de la fecha real, nunca igual: mueve el año
// entre uno y tres en cualquier dirección y suma hasta cuatro meses.
func (f *Fabrica) nacimiento(llave string, real time.Time, intento int) string {
	años := 1 + f.numero(llave, "nacimiento:años", intento)%3
	if f.numero(llave, "nacimiento:signo", intento)%2 == 0 {
		años = -años
	}
	dias := 1 + f.numero(llave, "nacimiento:dias", intento)%120
	if f.numero(llave, "nacimiento:signo-dias", intento)%2 == 0 {
		dias = -dias
	}
	return real.AddDate(años, 0, dias).Format("2006-01-02")
}

// placa es una placa colombiana: tres consonantes y tres dígitos.
func (f *Fabrica) placa(llave string, intento int) string {
	return f.letrasDe(llave, "placa", intento, 3) + f.cifras(llave, "placa", intento, 3)
}

// pasaporte es un pasaporte del corpus: dos letras y seis dígitos.
func (f *Fabrica) pasaporte(llave string, intento int) string {
	return f.letrasDe(llave, "pasaporte", intento, 2) + f.cifras(llave, "pasaporte", intento, 6)
}

// letrasDe arma cuantas consonantes deterministas del campo.
func (f *Fabrica) letrasDe(llave, etiqueta string, intento, cuantas int) string {
	var b strings.Builder
	for i := 0; i < cuantas; i++ {
		indice := f.numero(llave, etiqueta+":letra:"+strconv.Itoa(i), intento) % len(consonantes)
		b.WriteRune(consonantes[indice])
	}
	return b.String()
}
