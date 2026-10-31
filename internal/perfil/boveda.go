package perfil

import (
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
)

// boveda es lo que la fábrica necesita de la bóveda: el perfil propio, si ya
// está asignado, y los datos que otro perfil ocupa. Dos personas nunca
// comparten seudónimo ni identificadores falsos (D4).
type boveda struct {
	propio    *modelo.Perfil
	nombres   map[string]struct{}
	cortos    map[string]struct{}
	correos   map[string]struct{}
	cedulas   map[string]struct{}
	celulares map[string]struct{}
}

// nuevaBoveda deja la bóveda vacía: sin Almacen no hay nada asignado.
func nuevaBoveda() *boveda {
	return &boveda{
		nombres:   map[string]struct{}{},
		cortos:    map[string]struct{}{},
		correos:   map[string]struct{}{},
		cedulas:   map[string]struct{}{},
		celulares: map[string]struct{}{},
	}
}

// agrega anota los datos de un perfil ajeno.
func (b *boveda) agrega(p modelo.Perfil) {
	if p.Nombre != "" {
		b.nombres[norm.Norm(p.Nombre)] = struct{}{}
		if corto := nombreCorto(p.Nombre); corto != "" {
			b.cortos[norm.Norm(corto)] = struct{}{}
		}
	}
	if p.Correo != "" {
		b.correos[strings.ToLower(p.Correo)] = struct{}{}
	}
	if digitos := soloDigitos(p.Cedula); digitos != "" {
		b.cedulas[digitos] = struct{}{}
	}
	if digitos := soloDigitos(p.Celular); digitos != "" {
		b.celulares[digitos] = struct{}{}
		if len(digitos) == 12 && strings.HasPrefix(digitos, "57") {
			b.celulares[digitos[2:]] = struct{}{}
		}
	}
}

// repite dice si el candidato repite el nombre, el correo, la cédula o el
// celular de otro perfil de la bóveda.
func (b *boveda) repite(p modelo.Perfil) bool {
	if _, ocupado := b.nombres[norm.Norm(p.Nombre)]; ocupado {
		return true
	}
	// El nombre corto es el que se escribe en el chat y se declara: dos
	// perfiles que solo difieren en el último apellido chocan (CE-51).
	if corto := nombreCorto(p.Nombre); corto != "" {
		if _, ocupado := b.cortos[norm.Norm(corto)]; ocupado {
			return true
		}
	}
	if _, ocupado := b.correos[strings.ToLower(p.Correo)]; ocupado {
		return true
	}
	if digitos := soloDigitos(p.Cedula); digitos != "" {
		if _, ocupado := b.cedulas[digitos]; ocupado {
			return true
		}
	}
	if digitos := soloDigitos(p.Celular); digitos != "" {
		if _, ocupado := b.celulares[digitos]; ocupado {
			return true
		}
	}
	return false
}

// nombreCorto es la forma corta del nombre como la escribe formas: todas las
// palabras menos el último apellido.
func nombreCorto(nombre string) string {
	palabras := strings.Fields(nombre)
	if len(palabras) < 3 {
		return strings.Join(palabras, " ")
	}
	return strings.Join(palabras[:len(palabras)-1], " ")
}
