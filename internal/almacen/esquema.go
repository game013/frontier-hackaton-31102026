package almacen

// versionEsquema es el PRAGMA user_version que este paquete sabe leer. Sube en
// uno con cada migración; una base de otra versión no se abre.
const versionEsquema = 1

// Las marcas de tiempo son RFC 3339 en UTC y las columnas de texto opcionales
// guardan "" mientras no aplican, para no llenar de sql.NullString al resto del
// pipeline.
const esquemaV1 = `
-- jobs: una corrida de /run. Estado y conteos son los de GET /run/{id}.
CREATE TABLE jobs (
	id         TEXT PRIMARY KEY,
	estado     TEXT NOT NULL CHECK (estado IN ('queued', 'running', 'done', 'failed')),
	creado     TEXT NOT NULL,
	iniciado   TEXT NOT NULL DEFAULT '',
	terminado  TEXT NOT NULL DEFAULT '',
	chats      INTEGER NOT NULL DEFAULT 0,
	docs       INTEGER NOT NULL DEFAULT 0,
	tablas     INTEGER NOT NULL DEFAULT 0,
	tokens_in  INTEGER NOT NULL DEFAULT 0,
	tokens_out INTEGER NOT NULL DEFAULT 0,
	segundos   REAL NOT NULL DEFAULT 0,
	error      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX jobs_por_estado ON jobs (estado);

-- personas: registro del corpus que produce /run. La llave es la evidencia
-- (p_cl_, p_kp_, p_t_), nunca el nombre.
CREATE TABLE personas (
	llave  TEXT PRIMARY KEY,
	rol    TEXT NOT NULL,
	genero TEXT NOT NULL,
	creada TEXT NOT NULL
);

-- personas_vivo: personas nacidas en /anonymize (D24). Reservan seudónimo en
-- la bóveda, pero no entran al registro de /run ni a entities.json.
CREATE TABLE personas_vivo (
	llave  TEXT PRIMARY KEY,
	rol    TEXT NOT NULL,
	genero TEXT NOT NULL,
	creada TEXT NOT NULL
);

-- formas: cada manera en que una persona aparece. Es de solo agregar: la misma
-- (persona, origen, tipo, valor) no se duplica y conserva el primer registro
-- donde se vio. vivo = 1 marca las formas de personas_vivo (D24).
CREATE TABLE formas (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	llave    TEXT NOT NULL,
	vivo     INTEGER NOT NULL DEFAULT 0 CHECK (vivo IN (0, 1)),
	tipo     TEXT NOT NULL,
	valor    TEXT NOT NULL,
	registro TEXT NOT NULL DEFAULT '',
	UNIQUE (llave, vivo, tipo, valor)
);

-- perfiles: la bóveda (D4, D22). Solo agrega: un perfil asignado no cambia.
-- La llave puede ser de personas o de personas_vivo: ambas reservan perfil.
CREATE TABLE perfiles (
	llave      TEXT PRIMARY KEY,
	nombre     TEXT NOT NULL,
	correo     TEXT NOT NULL DEFAULT '',
	celular    TEXT NOT NULL DEFAULT '',
	cedula     TEXT NOT NULL DEFAULT '',
	handle     TEXT NOT NULL DEFAULT '',
	direccion  TEXT NOT NULL DEFAULT '',
	nacimiento TEXT NOT NULL DEFAULT '',
	placa      TEXT NOT NULL DEFAULT '',
	cuenta     TEXT NOT NULL DEFAULT '',
	pasaporte  TEXT NOT NULL DEFAULT '',
	asignado   TEXT NOT NULL
);

-- cache_modelo: respuesta de Jev por hash de la entrada. modelo es el snapshot
-- fijo (D32): si el snapshot cambia, la llave cambia y no se reutiliza nada.
CREATE TABLE cache_modelo (
	clave      TEXT PRIMARY KEY,
	modelo     TEXT NOT NULL,
	respuesta  TEXT NOT NULL,
	tokens_in  INTEGER NOT NULL DEFAULT 0,
	tokens_out INTEGER NOT NULL DEFAULT 0,
	creado     TEXT NOT NULL
);

-- dudosos: menciones de baja confianza (D20). Vive en /data, nunca en el repo;
-- alimenta CASOS_ESQUINA.md.
CREATE TABLE dudosos (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	registro  TEXT NOT NULL DEFAULT '',
	fuente    TEXT NOT NULL DEFAULT '',
	campo     TEXT NOT NULL DEFAULT '',
	inicio    INTEGER NOT NULL DEFAULT 0,
	fin       INTEGER NOT NULL DEFAULT 0,
	valor     TEXT NOT NULL,
	tipo      TEXT NOT NULL DEFAULT '',
	persona   TEXT NOT NULL DEFAULT '',
	regla     TEXT NOT NULL DEFAULT '',
	confianza REAL NOT NULL DEFAULT 0,
	creado    TEXT NOT NULL
);

-- captura: lo que manda el evaluador a /run y /anonymize, con los tokens ya
-- redactados. Es material real: solo en /data, como la bóveda.
CREATE TABLE captura (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	creado    TEXT NOT NULL,
	tipo      TEXT NOT NULL,
	metodo    TEXT NOT NULL,
	ruta      TEXT NOT NULL,
	peticion  TEXT NOT NULL,
	respuesta TEXT NOT NULL DEFAULT ''
);
`
