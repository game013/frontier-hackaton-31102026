// Package reemplazo sustituye cada mención por la forma equivalente del perfil
// falso, en el mismo estilo y de derecha a izquierda (D6, §6.1.6).
//
// El reemplazo es literal: solo cambian los fragmentos detectados y el resto del
// texto queda byte a byte igual. El estilo sale del tipo de forma que reconoció
// la detección (cédula con puntos, correo deletreado, MAYÚSCULAS sin tildes,
// slug, CC con su prefijo) y de la caja del fragmento real.
package reemplazo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/game013/frontier-hackaton-31102026/internal/campos"
	"github.com/game013/frontier-hackaton-31102026/internal/formas"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// Opciones del reemplazo: el registro para los datos y la bóveda para los
// perfiles falsos.
type Opciones struct {
	Indice   *registro.Indice
	Perfiles map[string]modelo.Perfil
}

// Reemplazo aplica los perfiles a las menciones resueltas.
type Reemplazo struct {
	indice   *registro.Indice
	perfiles map[string]modelo.Perfil
}

// Nuevo arma el reemplazo.
func Nuevo(o Opciones) *Reemplazo {
	return &Reemplazo{indice: o.Indice, perfiles: o.Perfiles}
}

// Aplica devuelve los registros con las menciones sustituidas, en el mismo
// orden. Una mención sin persona no se toca: esa decisión la toma la resolución.
func (r *Reemplazo) Aplica(recs []modelo.Registro, menciones []modelo.Mencion) ([]modelo.Registro, error) {
	porRegistro := map[string][]modelo.Mencion{}
	for _, m := range menciones {
		if m.Persona == "" || m.Inicio >= m.Fin {
			continue
		}
		porRegistro[m.Registro] = append(porRegistro[m.Registro], m)
	}
	out := make([]modelo.Registro, 0, len(recs))
	for _, original := range recs {
		ediciones, err := r.ediciones(original.IDRegistro(), porRegistro[original.IDRegistro()])
		if err != nil {
			return nil, err
		}
		if len(ediciones) == 0 {
			out = append(out, original)
			continue
		}
		nuevo, err := campos.Aplica(original, ediciones)
		if err != nil {
			return nil, fmt.Errorf("reemplazo: %s: %w", original.IDRegistro(), err)
		}
		out = append(out, nuevo)
	}
	return out, nil
}

// ediciones arma los cambios de un registro, uno por mención resuelta.
func (r *Reemplazo) ediciones(registroID string, menciones []modelo.Mencion) ([]campos.Edicion, error) {
	ediciones := make([]campos.Edicion, 0, len(menciones))
	for _, m := range menciones {
		perfil, ok := r.perfiles[m.Persona]
		if !ok {
			return nil, fmt.Errorf("reemplazo: %s sin perfil en la bóveda", m.Persona)
		}
		nuevo, err := reemplazoDe(perfil, m)
		if err != nil {
			return nil, fmt.Errorf("reemplazo: %s %q: %w", m.Persona, m.Valor, err)
		}
		ediciones = append(ediciones, campos.Edicion{
			Ruta: m.Campo, Inicio: m.Inicio, Fin: m.Fin, Nuevo: nuevo,
		})
	}
	return ediciones, nil
}

// reemplazoDe pide al perfil la forma del mismo tipo y le ajusta la caja y el
// prefijo al fragmento real. Si el perfil no tiene el dato (placa de alguien que
// no tenía placa), devuelve el original: la compuerta se encarga de lo que
// quede, y un dato inventado sería peor que uno conservado.
func reemplazoDe(perfil modelo.Perfil, m modelo.Mencion) (string, error) {
	tipo := m.Forma
	if tipo == "" {
		tipo = infiereTipo(m.Valor)
	}
	forma, err := formas.Render(perfil, tipo)
	if err != nil {
		if errors.Is(err, formas.ErrSinDato) {
			return m.Valor, nil
		}
		return "", err
	}
	valor := forma.Valor
	switch tipo {
	case modelo.FormaCedulaCC:
		// El perfil renderiza "CC<dígitos>"; el fragmento real puede traer
		// "CC " o "C.C. " delante, así que el prefijo manda.
		valor = prefijoDocumento(m.Valor) + soloDigitos(perfil.Cedula)
	case modelo.FormaNombreCompleto, modelo.FormaNombreCorto, modelo.FormaNombrePila,
		modelo.FormaMayusculas, modelo.FormaDireccion:
		valor = ajustaCaja(m.Valor, valor)
	}
	return valor, nil
}

// infiereTipo adivina el estilo cuando la detección no lo trajo.
func infiereTipo(valor string) modelo.TipoForma {
	switch {
	case strings.Contains(valor, "@"):
		return modelo.FormaCorreo
	case strings.Contains(strings.ToLower(valor), " arroba "):
		return modelo.FormaCorreoDeletreado
	case strings.Contains(valor, "_"):
		return modelo.FormaSlug
	}
	d := soloDigitos(valor)
	switch {
	case len(d) >= 6 && len(d) <= 11 && d == valor:
		return modelo.FormaCedula
	case len(d) >= 6 && strings.Contains(valor, "."):
		return modelo.FormaCedulaPuntos
	case len(d) == 10 && strings.HasPrefix(d, "3"):
		return modelo.FormaCelular
	}
	if valor == strings.ToUpper(valor) && valor != strings.ToLower(valor) {
		return modelo.FormaMayusculas
	}
	return modelo.FormaNombreCompleto
}

// ajustaCaja conserva MAYÚSCULAS y minúsculas del fragmento real (CE-03, CE-29).
func ajustaCaja(original, nuevo string) string {
	if original == strings.ToUpper(original) && original != strings.ToLower(original) {
		return strings.ToUpper(nuevo)
	}
	if original == strings.ToLower(original) && nuevo != strings.ToLower(nuevo) {
		return strings.ToLower(nuevo)
	}
	return nuevo
}

// prefijoDocumento devuelve lo que va antes del primer dígito ("CC ", "C.C. ").
func prefijoDocumento(valor string) string {
	for i, r := range valor {
		if r >= '0' && r <= '9' {
			return valor[:i]
		}
	}
	return ""
}

func soloDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
