// Package resuelve decide a qué persona pertenece cada mención. La evidencia se
// ordena por fuerza (D26, D29):
//
//  1. identificador duro: cédula, correo, celular, placa, pasaporte, `CL-`,
//     `PQR-`, `TX-`, uid;
//  2. contexto: quién escribe el mensaje, participantes del hilo, rol ("soy X"),
//     y la fila del maestro cuando la mención cae en sus propios datos;
//  3. nombre único: solo si hay un candidato compatible;
//  4. el modelo, con fichas abstractas (ids y ciudad, nunca el nombre);
//  5. si nada vincula, una persona nueva marcada como dudosa (D19).
//
// Nunca se escoge "el más probable" sin evidencia: fusionar a dos personas
// rompe los homónimos, y separar se corrige después sin fuga.
package resuelve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/campos"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

// Almacen es la parte de la persistencia que la resolución necesita: personas
// nuevas del corpus, personas de /anonymize (D24) y la cola de dudosos (D20).
type Almacen interface {
	GuardarPersona(ctx context.Context, p modelo.Persona) error
	GuardarPersonaVivo(ctx context.Context, p modelo.Persona) error
	AnotaDudoso(ctx context.Context, m modelo.Mencion) error
}

// Opciones arma el resolutor. Modelo nil deja los empates en D19.
type Opciones struct {
	Indice  *registro.Indice
	Modelo  modelo.Modelo
	Almacen Almacen
	// Vivo marca las personas nacidas en /anonymize: van a personas_vivo y no
	// entran al registro del corpus (D24).
	Vivo bool
	// Timeout acota la llamada al modelo por mención (D28).
	Timeout time.Duration
}

// Resolutor llena la persona de cada mención.
type Resolutor struct {
	indice  *registro.Indice
	modelo  modelo.Modelo
	almacen Almacen
	vivo    bool
	timeout time.Duration
}

// Nuevo arma el resolutor.
func Nuevo(o Opciones) *Resolutor {
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Resolutor{indice: o.Indice, modelo: o.Modelo, almacen: o.Almacen, vivo: o.Vivo, timeout: timeout}
}

// Resuelve devuelve las menciones con Persona llena, en el mismo orden.
func (r *Resolutor) Resuelve(ctx context.Context, recs []modelo.Registro, menciones []modelo.Mencion) ([]modelo.Mencion, error) {
	if r.indice == nil {
		return nil, errors.New("resuelve: falta el registro de personas")
	}
	contexto := nuevoContexto(recs, r.indice)
	out := make([]modelo.Mencion, len(menciones))
	copy(out, menciones)
	for i := range out {
		m := &out[i]
		if m.Persona != "" {
			r.anotaDudoso(ctx, *m)
			continue
		}
		if err := r.resuelveUna(ctx, contexto, m); err != nil {
			return nil, err
		}
		r.anotaDudoso(ctx, *m)
	}
	return out, nil
}

// resuelveUna aplica la escalera de evidencia a una mención.
func (r *Resolutor) resuelveUna(ctx context.Context, c *contexto, m *modelo.Mencion) error {
	candidatos := r.indice.Busca(m.Valor)
	if llave, ok := r.porIdentificadorDuro(m, candidatos); ok {
		m.Persona, m.Regla, m.Confianza = llave, "id_duro", 0.95
		return nil
	}
	if llave, ok := c.porContexto(m, candidatos); ok {
		m.Persona, m.Regla, m.Confianza = llave, "contexto", 0.85
		return nil
	}
	if llave, ok := c.porLinea(m, r.indice, candidatos); ok {
		m.Persona, m.Regla, m.Confianza = llave, "linea", 0.85
		return nil
	}
	if len(candidatos) == 1 {
		m.Persona, m.Regla, m.Confianza = candidatos[0].Llave, "nombre_unico", 0.8
		return nil
	}
	if llave, ok := unicoDelMaestro(candidatos); ok {
		m.Persona, m.Regla, m.Confianza = llave, "maestro_sobre_tercero", 0.75
		return nil
	}
	if len(candidatos) > 1 {
		if llave, confianza, ok := r.desempata(ctx, c, m, candidatos); ok {
			m.Persona, m.Regla, m.Confianza = llave, "desempate_modelo", confianza
			return nil
		}
	}
	return r.personaNueva(ctx, m)
}

// porIdentificadorDuro reconoce las formas que identifican sin ambigüedad.
func (r *Resolutor) porIdentificadorDuro(m *modelo.Mencion, candidatos []modelo.Persona) (string, bool) {
	if !esIdentificadorFuerte(m.Forma) {
		return "", false
	}
	if len(candidatos) == 1 {
		return candidatos[0].Llave, true
	}
	if p, ok := r.indice.PorReferencia(m.Valor); ok {
		return p.Llave, true
	}
	return "", false
}

// esIdentificadorFuerte dice qué formas bastan solas: un nombre no basta nunca.
func esIdentificadorFuerte(t modelo.TipoForma) bool {
	switch t {
	case modelo.FormaCedula, modelo.FormaCedulaPuntos, modelo.FormaCedulaCC,
		modelo.FormaCorreo, modelo.FormaCorreoDeletreado,
		modelo.FormaCelular, modelo.FormaCelularE164, modelo.FormaCelularLocal,
		modelo.FormaPlaca, modelo.FormaPasaporte, modelo.FormaUsuario:
		return true
	}
	return false
}

// desempata pregunta al modelo entre las personas que comparten la forma, con
// fichas abstractas; sin confianza suficiente, D19.
func (r *Resolutor) desempata(ctx context.Context, c *contexto, m *modelo.Mencion, candidatos []modelo.Persona) (string, float64, bool) {
	if r.modelo == nil {
		return "", 0, false
	}
	if len(candidatos) > 4 {
		candidatos = candidatos[:4]
	}
	estado, opciones := c.fichas(m, r.indice, candidatos)
	llamada, cancelar := context.WithTimeout(ctx, r.timeout)
	defer cancelar()
	respuesta, err := r.modelo.Decide(llamada, estado, modelo.Pregunta{
		ID:       "desempate",
		Tipo:     "choice",
		Texto:    "¿A qué ficha se refiere el fragmento? Si ninguna encaja, responde ninguna.",
		Opciones: opciones,
	})
	if err != nil {
		return "", 0, false
	}
	riesgo := respuesta.Probabilidades[respuesta.Opcion]
	if respuesta.Opcion == "" || norm.Norm(respuesta.Opcion) == "ninguna" || riesgo < 0.7 {
		return "", 0, false
	}
	for i, opcion := range opciones {
		if opcion == respuesta.Opcion && i < len(candidatos) {
			return candidatos[i].Llave, riesgo, true
		}
	}
	return "", 0, false
}

// personaNueva crea la persona que la evidencia no alcanzó a identificar y la
// marca dudosa (D19). La llave sale del valor y su ancla, nunca del orden.
func (r *Resolutor) personaNueva(ctx context.Context, m *modelo.Mencion) error {
	// Un nombre de pila suelto o una fecha sin dueño no identifican a nadie por
	// sí solos: inventar una persona a partir de ellos contamina la bóveda y la
	// compuerta barrería fechas de negocio (D25, CE-23).
	switch m.Forma {
	case modelo.FormaNombrePila, modelo.FormaFechaISO, modelo.FormaFechaUS, modelo.FormaFechaPalabras:
		m.Regla, m.Confianza = "sin_evidencia", 0
		return nil
	}
	llave := llaveNueva(m)
	persona := modelo.Persona{
		Llave:  llave,
		Rol:    modelo.RolDesconocido,
		Genero: modelo.GeneroIndeterminado,
		Formas: []modelo.Forma{{Tipo: m.Forma, Valor: m.Valor, Registro: m.Registro}},
	}
	if !r.indice.AgregaPersona(persona) {
		m.Persona, m.Regla, m.Confianza = llave, "persona_existente", 0.6
		return nil
	}
	if r.almacen != nil {
		var err error
		if r.vivo {
			err = r.almacen.GuardarPersonaVivo(ctx, persona)
		} else {
			err = r.almacen.GuardarPersona(ctx, persona)
		}
		if err != nil {
			return err
		}
	}
	m.Persona, m.Regla, m.Confianza = llave, "persona_nueva", 0.3
	return nil
}

// llaveNueva deriva p_t_<hash> del valor normalizado y su ancla (D22).
func llaveNueva(m *modelo.Mencion) string {
	suma := sha256.Sum256([]byte(norm.Norm(m.Valor) + "|" + m.Registro))
	return "p_t_" + hex.EncodeToString(suma[:8])
}

// anotaDudoso manda a la cola las menciones de baja confianza (D20).
func (r *Resolutor) anotaDudoso(ctx context.Context, m modelo.Mencion) {
	if r.almacen == nil || m.Confianza >= 0.6 || m.Persona == "" {
		return
	}
	_ = r.almacen.AnotaDudoso(ctx, m)
}

// contexto guarda lo que la resolución necesita de los registros: autor de cada
// mensaje, participantes de cada hilo, la fila del maestro y el texto de cada
// campo para reconocer "soy X".
type contexto struct {
	autores         map[string]string
	hilos           map[string][]string
	hiloPorRegistro map[string]string
	filas           map[string]string
	textos          map[string]map[modelo.CampoTexto]string
}

func nuevoContexto(recs []modelo.Registro, indice *registro.Indice) *contexto {
	c := &contexto{
		autores:         map[string]string{},
		hilos:           map[string][]string{},
		hiloPorRegistro: map[string]string{},
		filas:           map[string]string{},
		textos:          map[string]map[modelo.CampoTexto]string{},
	}
	for _, r := range recs {
		mapaCampos := map[modelo.CampoTexto]string{}
		for _, campo := range campos.Lista(r) {
			mapaCampos[campo.Ruta] = campo.Valor
		}
		c.textos[r.IDRegistro()] = mapaCampos
		chat, ok := r.(modelo.ChatRecord)
		if !ok {
			continue
		}
		c.hiloPorRegistro[chat.ID] = chat.ThreadID
		if p, ok := indice.AutorDe(chat.ID); ok {
			c.autores[chat.ID] = p.Llave
			c.hilos[chat.ThreadID] = appendUnico(c.hilos[chat.ThreadID], p.Llave)
			continue
		}
		// En vivo el uid no viaja: `author` ya es un nombre (D29). Solo se usa
		// cuando hay un único candidato con ese nombre.
		if chat.Author != "" {
			if candidatos := indice.Busca(chat.Author); len(candidatos) == 1 {
				c.autores[chat.ID] = candidatos[0].Llave
				c.hilos[chat.ThreadID] = appendUnico(c.hilos[chat.ThreadID], candidatos[0].Llave)
			}
		}
	}
	for _, r := range recs {
		tabla, ok := r.(modelo.TableRecord)
		if !ok {
			continue
		}
		if p, ok := indice.PorReferencia(tabla.ID); ok {
			c.filas[tabla.ID] = p.Llave
		} else if p, ok := indice.PorReferencia(tabla.Columns["cliente_id"]); ok && tabla.Table == "pqrs" {
			// La fila de una queja es de su cliente (CE-52).
			c.filas[tabla.ID] = p.Llave
		}
	}
	return c
}

// porContexto aplica el paso 2 de la escalera.
func (c *contexto) porContexto(m *modelo.Mencion, candidatos []modelo.Persona) (string, bool) {
	// El autor de un mensaje se resuelve por su uid, no por su nombre.
	if m.Campo == "author" {
		if llave, ok := c.autores[m.Registro]; ok {
			return llave, true
		}
	}
	// "soy X", "me llamo X": la mención es del autor del mensaje.
	if autor, ok := c.autores[m.Registro]; ok && c.anunciaSer(m) {
		return autor, true
	}
	// Si la mención cae en la fila de una persona del maestro, la fila manda
	// sobre el nombre cuando hay homónimos.
	if llave, ok := c.filas[m.Registro]; ok {
		for _, p := range candidatos {
			if p.Llave == llave {
				return llave, true
			}
		}
	}
	// Participantes del hilo: si uno solo de los candidatos habla en él, es suyo.
	if hilo, ok := c.hilos[c.hiloPorRegistro[m.Registro]]; ok && len(hilo) > 0 {
		var dentro []string
		for _, p := range candidatos {
			for _, llave := range hilo {
				if p.Llave == llave {
					dentro = append(dentro, p.Llave)
				}
			}
		}
		if len(dentro) == 1 {
			return dentro[0], true
		}
	}
	return "", false
}

// anunciaSer dice si la mención va precedida de "soy" o "me llamo" en el mismo
// campo: la señal de que quien escribe habla de sí mismo (CE-02).
func (c *contexto) anunciaSer(m *modelo.Mencion) bool {
	campos := c.textos[m.Registro]
	if campos == nil {
		return false
	}
	valor := campos[m.Campo]
	if m.Inicio <= 0 || m.Inicio > len(valor) {
		return false
	}
	desde := m.Inicio - 24
	if desde < 0 {
		desde = 0
	}
	antes := " " + norm.Norm(valor[desde:m.Inicio]) + " "
	for _, formula := range []string{" soy ", " me llamo ", " mi nombre es "} {
		if strings.Contains(antes, formula) {
			return true
		}
	}
	return false
}

// fichas arma el estado enmascarado y las opciones del desempate (D26). De cada
// candidato viajan ids, rol y ciudad; nunca su nombre.
func (c *contexto) fichas(m *modelo.Mencion, indice *registro.Indice, candidatos []modelo.Persona) (string, []string) {
	var b strings.Builder
	b.WriteString("Fragmento: «")
	b.WriteString(m.Valor)
	b.WriteString("».\n")
	opciones := make([]string, 0, len(candidatos)+1)
	for i, p := range candidatos {
		opcion := string(rune('A' + i))
		opciones = append(opciones, opcion)
		b.WriteString(opcion)
		b.WriteString(": ")
		b.WriteString(describeFicha(indice, p.Llave))
		b.WriteString("\n")
	}
	opciones = append(opciones, "ninguna")
	return b.String(), opciones
}

// describeFicha resume una persona con datos abstractos.
func describeFicha(indice *registro.Indice, llave string) string {
	ficha, ok := indice.Ficha(llave)
	if !ok {
		return "persona sin datos"
	}
	partes := []string{string(ficha.Rol)}
	if ficha.Ciudad != "" {
		partes = append(partes, "ciudad "+ficha.Ciudad)
	}
	if len(ficha.Ref) > 0 {
		partes = append(partes, "referencias "+strings.Join(ficha.Ref, ", "))
	}
	return strings.Join(partes, " · ")
}

func appendUnico(lista []string, valor string) []string {
	for _, v := range lista {
		if v == valor {
			return lista
		}
	}
	return append(lista, valor)
}

// unicoDelMaestro elige a la única persona del maestro (cliente o empleado)
// entre candidatos que, por lo demás, son terceros. Un tercero sale de una
// mención dudosa y queda en la bóveda; si empata con un cliente, el empate crea
// otro tercero y la corrida siguiente hereda más ambigüedad (CE-49).
func unicoDelMaestro(candidatos []modelo.Persona) (string, bool) {
	if len(candidatos) < 2 {
		return "", false
	}
	llave := ""
	for _, p := range candidatos {
		if p.Rol != modelo.RolCliente && p.Rol != modelo.RolEmpleado {
			continue
		}
		if llave != "" {
			return "", false
		}
		llave = p.Llave
	}
	return llave, llave != ""
}
