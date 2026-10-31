package modelo

// TipoMencion clasifica un fragmento encontrado en el texto. Solo persona y
// organization_or_place entran a entities.json; el resto se conserva.
type TipoMencion string

const (
	MencionPersona            TipoMencion = "person"
	MencionOrganizacionOLugar TipoMencion = "organization_or_place"
	MencionMarca              TipoMencion = "brand"
	MencionFiguraPublica      TipoMencion = "public_figure"
	MencionOtro               TipoMencion = "other"
)

// CampoTexto nombra la parte de un registro donde vive una mención, para que el
// reemplazo sepa qué string tocar: "text", "author", "title", "body",
// "columns.<columna>", "metadata.<clave>", "filename", "attachments[i].filename".
type CampoTexto string

// Mencion es un fragmento real detectado en un registro, con offsets en bytes
// sobre el string original del campo (no sobre el texto normalizado). Fin es
// exclusivo.
type Mencion struct {
	Registro string      `json:"registro"`
	Fuente   Fuente      `json:"fuente"`
	Campo    CampoTexto  `json:"campo"`
	Inicio   int         `json:"inicio"`
	Fin      int         `json:"fin"`
	Valor    string      `json:"valor"`
	Tipo     TipoMencion `json:"tipo"`
	// Forma es el tipo de forma que se reconoció (nombre_completo, cedula,
	// correo_deletreado…). El reemplazo la usa para pedirle a la fábrica el
	// mismo estilo del perfil; vacía cuando el fragmento no casa con una forma
	// del registro y el reemplazo tiene que inferir el estilo.
	Forma     TipoForma `json:"forma,omitempty"`
	Persona   string    `json:"persona,omitempty"`
	Regla     string    `json:"regla"`
	Confianza float64   `json:"confianza"`
}
