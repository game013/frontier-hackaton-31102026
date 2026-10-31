// Package perfil fabrica la identidad falsa de una persona (D4, D22): mismo
// género y estructura que la real, datos coherentes entre sí, sin coincidir con
// ninguna forma real ni con otro perfil de la bóveda, y siempre igual para la
// misma llave.
//
// La semilla es HMAC-SHA256(secreto, llave) y de ahí salen índices enteros
// sobre el catálogo embebido (D18): no hay azar ni reloj, así que el perfil de
// una persona es byte a byte el mismo en cada corrida mientras la bóveda no
// cambie. Cuando un dato choca con una forma real o con otro perfil, se avanza
// el índice determinista hasta un perfil libre: el choque no se resuelve por
// orden de llegada.
//
// La bóveda manda: un perfil ya asignado se devuelve tal cual y nunca se
// recalcula, porque un seudónimo que cambia entre corridas rompe la
// consistencia del dataset.
package perfil

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/catalogo"
	"github.com/game013/frontier-hackaton-31102026/internal/formas"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// Opciones configuran la fábrica.
type Opciones struct {
	// Secreto es la llave del HMAC: sin él no hay perfil determinista.
	Secreto []byte
	// Almacen es la bóveda. nil deja la fábrica sin persistencia (pruebas de
	// determinismo y de exclusión de formas reales).
	Almacen modelo.Almacen
	// Reales son todas las formas reales conocidas del corpus: ninguna forma
	// del perfil puede coincidir con una de ellas.
	Reales []modelo.Forma
}

// Fabrica deriva perfiles falsos. Es de solo lectura después de Nuevo: varias
// goroutines pueden pedir perfiles a la vez.
type Fabrica struct {
	secreto []byte
	almacen modelo.Almacen
	reales  *realesSet
}

// Nuevo arma la fábrica. El secreto vacío es un error de construcción: los
// perfiles dejarían de ser reproducibles entre corridas.
func Nuevo(o Opciones) (*Fabrica, error) {
	if len(o.Secreto) == 0 {
		return nil, errors.New("perfil: falta el secreto del HMAC")
	}
	return &Fabrica{
		secreto: append([]byte(nil), o.Secreto...),
		almacen: o.Almacen,
		reales:  nuevoRealesSet(o.Reales),
	}, nil
}

// PerfilDe devuelve el perfil de la persona: el de la bóveda si ya está
// asignado, y si no, uno derivado de su llave que se guarda antes de devolverlo.
func (f *Fabrica) PerfilDe(ctx context.Context, p modelo.Persona) (modelo.Perfil, error) {
	if p.Llave == "" {
		return modelo.Perfil{}, errors.New("perfil: persona sin llave")
	}
	if err := ctx.Err(); err != nil {
		return modelo.Perfil{}, fmt.Errorf("perfil: persona %q: %w", p.Llave, err)
	}
	asignados, err := f.leeBoveda(ctx, p.Llave)
	if err != nil {
		return modelo.Perfil{}, err
	}
	if asignados.propio != nil {
		return *asignados.propio, nil
	}
	perfil, err := f.deriva(p, asignados)
	if err != nil {
		return modelo.Perfil{}, err
	}
	if f.almacen == nil {
		return perfil, nil
	}
	if err := f.almacen.GuardarPerfil(ctx, p.Llave, perfil); err != nil {
		// La bóveda solo agrega: si otra corrida asignó perfil mientras tanto
		// (ErrPerfilYaAsignado), manda el guardado y este se descarta.
		if guardado, existe, errLectura := f.almacen.Perfil(ctx, p.Llave); errLectura != nil {
			return modelo.Perfil{}, fmt.Errorf("perfil: releer el perfil de %q: %w", p.Llave, errLectura)
		} else if existe {
			return guardado, nil
		}
		return modelo.Perfil{}, fmt.Errorf("perfil: guardar el perfil de %q: %w", p.Llave, err)
	}
	return perfil, nil
}

// leeBoveda trae el estado de la bóveda para la llave: su perfil si ya está
// asignado y los datos que ocupan las demás personas.
func (f *Fabrica) leeBoveda(ctx context.Context, llave string) (*boveda, error) {
	asignados := nuevaBoveda()
	if f.almacen == nil {
		return asignados, nil
	}
	propio, existe, err := f.almacen.Perfil(ctx, llave)
	if err != nil {
		return nil, fmt.Errorf("perfil: leer el perfil de %q: %w", llave, err)
	}
	if existe {
		asignados.propio = &propio
		return asignados, nil
	}
	todos, err := f.almacen.Perfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("perfil: leer la bóveda: %w", err)
	}
	for otra, perfil := range todos {
		if otra == llave {
			continue
		}
		asignados.agrega(perfil)
	}
	return asignados, nil
}

// intentosMaximos acota la búsqueda del perfil libre: cada intento mueve todos
// los índices, así que agotarlos significa un corpus que no deja ninguna
// combinación libre y hay que verlo, no taparlo.
const intentosMaximos = 64

// deriva construye el perfil con los índices que salen de la llave y avanza el
// intento mientras alguna de sus formas choque con una forma real del corpus o
// con los datos de otra persona de la bóveda.
func (f *Fabrica) deriva(p modelo.Persona, asignados *boveda) (modelo.Perfil, error) {
	real := leeDatos(p.Formas)
	for intento := 0; intento < intentosMaximos; intento++ {
		candidato := f.perfil(p, real, intento)
		choque, err := f.choca(candidato)
		if err != nil {
			return modelo.Perfil{}, err
		}
		if choque || asignados.repite(candidato) {
			continue
		}
		return candidato, nil
	}
	return modelo.Perfil{}, fmt.Errorf("perfil: %q no alcanzó un perfil libre en %d intentos", p.Llave, intentosMaximos)
}

// choca dice si alguna forma del perfil repite una forma real conocida.
func (f *Fabrica) choca(candidato modelo.Perfil) (bool, error) {
	lista, err := formas.Todas(candidato)
	if err != nil {
		return false, fmt.Errorf("perfil: formas del perfil %q: %w", candidato.Nombre, err)
	}
	for _, forma := range lista {
		if f.reales.choca(forma.Valor) {
			return true, nil
		}
	}
	return false, nil
}

// perfil arma el perfil completo del intento: nombre con el mismo número de
// nombres de pila que la persona real y siempre dos apellidos, correo, cédula,
// celular y dirección. La fecha, la placa y el pasaporte solo existen si la
// persona real los tiene: el perfil no inventa datos que nunca se usaron.
func (f *Fabrica) perfil(p modelo.Persona, real datosReales, intento int) modelo.Perfil {
	nombres := f.nombresDePila(p.Llave, p.Genero, real.pila, intento)
	apellidos := f.apellidos(p.Llave, intento)
	perfil := modelo.Perfil{
		Nombre:    nombreCompleto(nombres, apellidos),
		Correo:    correoDe(nombres[0], apellidos[0], f.dominio(p.Rol, p.Llave, intento)),
		Celular:   f.celular(p.Llave, intento),
		Cedula:    f.cedula(p.Llave, real.cedula, intento),
		Direccion: f.direccion(p.Llave, intento),
	}
	if real.hayNacimiento {
		perfil.Nacimiento = f.nacimiento(p.Llave, real.nacimiento, intento)
	}
	if real.placa {
		perfil.Placa = f.placa(p.Llave, intento)
	}
	if real.pasaporte {
		perfil.Pasaporte = f.pasaporte(p.Llave, intento)
	}
	return perfil
}

// nombreCompleto une nombres de pila y apellidos con un espacio.
func nombreCompleto(nombres, apellidos []string) string {
	palabras := make([]string, 0, len(nombres)+len(apellidos))
	palabras = append(palabras, nombres...)
	palabras = append(palabras, apellidos...)
	return strings.Join(palabras, " ")
}

// nombresDePila devuelve tantos nombres de pila como la persona real, todos
// distintos, del catálogo del género (unisex e indeterminado comparten, D27).
func (f *Fabrica) nombresDePila(llave string, genero modelo.Genero, cuantos, intento int) []string {
	lista := catalogo.NombresPila(genero, catalogo.OrigenColombiano)
	nombres := make([]string, 0, cuantos)
	for i := 0; i < cuantos; i++ {
		base := f.numero(llave, "pila:"+strconv.Itoa(i), intento)
		nombres = append(nombres, eligeLibre(lista, base, nombres))
	}
	return nombres
}

// apellidos devuelve dos apellidos distintos: el perfil falso siempre lleva dos.
func (f *Fabrica) apellidos(llave string, intento int) []string {
	lista := catalogo.Apellidos()
	primero := eligeLibre(lista, f.numero(llave, "apellido:0", intento), nil)
	segundo := eligeLibre(lista, f.numero(llave, "apellido:1", intento), []string{primero})
	return []string{primero, segundo}
}

// eligeLibre recorre la lista desde el índice base y devuelve el primer valor
// que no esté usado: dos nombres de pila nunca se repiten dentro del perfil.
func eligeLibre(lista []string, base int, usados []string) string {
	for salto := 0; salto < len(lista); salto++ {
		candidato := lista[(base+salto)%len(lista)]
		repetido := false
		for _, usado := range usados {
			if candidato == usado {
				repetido = true
				break
			}
		}
		if !repetido {
			return candidato
		}
	}
	return ""
}

// numero devuelve el entero determinista de (llave, etiqueta, intento): los
// primeros 31 bits del HMAC, así el índice nunca es negativo.
func (f *Fabrica) numero(llave, etiqueta string, intento int) int {
	suma := f.mac(llave, etiqueta, intento)
	return int(binary.BigEndian.Uint32(suma[:4]) >> 1)
}

// mac es HMAC-SHA256(secreto, llave‖etiqueta‖intento). La etiqueta separa los
// campos, así el índice de la cédula no depende del del apellido.
func (f *Fabrica) mac(llave, etiqueta string, intento int) []byte {
	h := hmac.New(sha256.New, f.secreto)
	h.Write([]byte(llave))
	h.Write([]byte{0})
	h.Write([]byte(etiqueta))
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(intento)))
	return h.Sum(nil)
}
