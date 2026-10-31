package perfil

import (
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
)

// realesSet son las formas reales del corpus indexadas para descartar un perfil
// que las repita. Se compara con norm.Norm —la vara del jurado (D5)— y, en los
// números, en dígitos: la misma cédula aparece con puntos, con el prefijo CC y
// pegada, y el celular con guiones o con +57.
type realesSet struct {
	textos  map[string]struct{}
	numeros map[string]struct{}
}

// nuevoRealesSet indexa las formas reales del corpus. Un valor vacío no dice
// nada y no bloquea ninguna elección.
func nuevoRealesSet(formas []modelo.Forma) *realesSet {
	r := &realesSet{textos: map[string]struct{}{}, numeros: map[string]struct{}{}}
	for _, forma := range formas {
		if forma.Valor == "" {
			continue
		}
		r.textos[norm.Norm(forma.Valor)] = struct{}{}
		if digitos := soloDigitos(forma.Valor); len(digitos) >= 6 {
			r.numeros[digitos] = struct{}{}
			if len(digitos) == 12 && strings.HasPrefix(digitos, "57") {
				r.numeros[digitos[2:]] = struct{}{}
			}
		}
	}
	return r
}

// choca dice si el valor repite una forma real.
func (r *realesSet) choca(valor string) bool {
	if _, ocupado := r.textos[norm.Norm(valor)]; ocupado {
		return true
	}
	digitos := soloDigitos(valor)
	if len(digitos) < 6 {
		return false
	}
	if _, ocupado := r.numeros[digitos]; ocupado {
		return true
	}
	if len(digitos) == 12 && strings.HasPrefix(digitos, "57") {
		_, ocupado := r.numeros[digitos[2:]]
		return ocupado
	}
	return false
}
