package modelo

import "encoding/json"

// Rol es el papel de una persona en el corpus. Sale de evidencia (tabla de
// empleados, users.json de soporte, hilo), nunca del nombre.
type Rol string

const (
	RolCliente     Rol = "cliente"
	RolAgente      Rol = "agente"
	RolEmpleado    Rol = "empleado"
	RolDesconocido Rol = "desconocido"
)

// Genero decide la forma falsa: la fábrica conserva género y estructura. Unisex
// e indeterminado comparten catálogo (D18, D27).
type Genero string

const (
	GeneroM             Genero = "M"
	GeneroF             Genero = "F"
	GeneroUnisex        Genero = "unisex"
	GeneroIndeterminado Genero = "indeterminado"
)

// TipoForma es cada manera en que una persona aparece en el corpus.
type TipoForma string

const (
	FormaNombreCompleto   TipoForma = "nombre_completo"
	FormaNombreCorto      TipoForma = "nombre_corto"
	FormaNombrePila       TipoForma = "nombre_pila"
	FormaMayusculas       TipoForma = "mayusculas_sin_tildes"
	FormaSlug             TipoForma = "slug"
	FormaUsuario          TipoForma = "usuario"
	FormaCorreo           TipoForma = "correo"
	FormaCorreoDeletreado TipoForma = "correo_deletreado"
	FormaCedula           TipoForma = "cedula"
	FormaCedulaPuntos     TipoForma = "cedula_con_puntos"
	FormaCedulaCC         TipoForma = "cedula_cc"
	FormaCelular          TipoForma = "celular"
	FormaCelularE164      TipoForma = "celular_e164"
	FormaCelularLocal     TipoForma = "celular_local"
	FormaDireccion        TipoForma = "direccion"
	FormaFechaISO         TipoForma = "fecha_iso"
	FormaFechaPalabras    TipoForma = "fecha_en_palabras"
	FormaFechaUS          TipoForma = "fecha_us"
	FormaPlaca            TipoForma = "placa"
	FormaPasaporte        TipoForma = "pasaporte"
)

// Forma es un valor literal con que una persona aparece. Valor es tal cual está
// en el texto (sin normalizar): el reemplazo es literal.
type Forma struct {
	Tipo  TipoForma `json:"tipo"`
	Valor string    `json:"valor"`
	// Registro es el id del registro donde se descubrió la forma, si aplica.
	Registro string `json:"registro,omitempty"`
}

// Persona es la unidad de seudonimización. Dos personas con el mismo nombre son
// dos personas: la llave sale de evidencia, no del nombre.
// Llaves: p_cl_<num> (cliente), p_kp_<num> (empleado), p_t_<hash> (tercero).
type Persona struct {
	Llave  string  `json:"llave"`
	Rol    Rol     `json:"rol"`
	Genero Genero  `json:"genero"`
	Formas []Forma `json:"formas"`
}

// Perfil es la identidad falsa completa y coherente de una persona (bóveda).
// Las claves replican pseudonyms de entities.json. Nombre es obligatorio.
type Perfil struct {
	Nombre     string `json:"name"`
	Correo     string `json:"email,omitempty"`
	Celular    string `json:"phone,omitempty"`
	Cedula     string `json:"cedula,omitempty"`
	Handle     string `json:"handle,omitempty"`
	Direccion  string `json:"address,omitempty"`
	Nacimiento string `json:"birth_iso,omitempty"`
	Placa      string `json:"plate,omitempty"`
	Cuenta     string `json:"account,omitempty"`
	Pasaporte  string `json:"passport,omitempty"`
}

// TipoEntidad distingue personas de lo que se conserva (organización, lugar,
// marca, figura pública).
type TipoEntidad string

const (
	EntidadPersona            TipoEntidad = "person"
	EntidadOrganizacionOLugar TipoEntidad = "organization_or_place"
)

// Entity es una entrada de entities.json.
type Entity struct {
	EntityID   string            `json:"entity_id"`
	Type       TipoEntidad       `json:"type"`
	Originals  []string          `json:"originals"`
	Pseudonyms map[string]string `json:"pseudonyms"`
	Records    []string          `json:"records"`
	Kept       bool              `json:"kept,omitempty"`
	Reason     string            `json:"reason,omitempty"`
}

// MarshalJSON garantiza los contenedores del contrato: originals y records como
// listas, pseudonyms como objeto (aunque esté vacío, como en las entidades que
// se conservan).
func (e Entity) MarshalJSON() ([]byte, error) {
	type alias Entity
	if e.Originals == nil {
		e.Originals = []string{}
	}
	if e.Records == nil {
		e.Records = []string{}
	}
	if e.Pseudonyms == nil {
		e.Pseudonyms = map[string]string{}
	}
	return json.Marshal(alias(e))
}
