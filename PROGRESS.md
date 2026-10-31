# Progreso · implementación

Tablero vivo para coordinar a varios agentes en paralelo. Lo que se decide va a `DISEÑO.md` §7; los casos esquina
a `CASOS_ESQUINA.md`; aquí solo vive el **estado del trabajo**.

## Estado ahora

- **Fase:** diseño cerrado (D1–D28). Arranca la ola 0.
- **Próximo paso:** C0 (contratos). Al terminar C0 se abre toda la ola 1 en paralelo.
- **Corrida 1:** 15:30 (Bogotá). El usuario la lanza con VALIDAR ENTRYPOINT; el agente solo la propone.
- **Pendientes de decisión:** P1 y P2 (abajo). No bloquean la ola 0 ni la ola 1.
- **Bloqueos:** ninguno.

## Hitos de hoy (Bogotá, UTC−5)

- [ ] 12:45 · C0 hecha: `go.mod`, paquetes y tipos compartidos en master.
- [ ] 13:30 · Servicio en Railway con `/v1/health` y `/v1/anonymize` de eco (C1).
- [ ] 14:15 · Pipeline completo sobre `sample/`: validador OK y 13/13 tests (C17).
- [ ] 14:45 · Pipeline sobre `fuentes/`: validador OK, barrido 0, dos corridas idénticas (C18).
- [ ] 15:00 · **Congelar** y desplegar. Después de esta hora solo entran arreglos de C18.
- [ ] 15:15 · Smoke test público: `/v1/health` y `/v1/anonymize` con un registro de la muestra.
- [ ] 15:30 · **Corrida 1** (la lanza el usuario).
- [ ] Tras el crecimiento: corridas 2 y 3 (las propone el agente; las lanza el usuario).

## Reglas de coordinación

1. **Tomar una tarea.** Solo se toma una tarea en estado `lista`. Se toma editando su fila (Estado `en curso`,
   Responsable, hora) y haciendo commit inmediato en master con `chore: toma Cn` (solo `PROGRESS.md`). Si el push o
   el commit chocan con otro agente que la tomó primero, se suelta y se elige otra.
2. **Aislamiento.** Cada tarea trabaja en su worktree `../31102026-wt/Cn` sobre la rama `cn-<slug>`
   (`git worktree add ../31102026-wt/Cn -b cn-<slug> master`). Nunca dos agentes en el mismo worktree.
3. **Propiedad de rutas.** Cada tarea solo escribe en las rutas de su columna *Toca*. Si necesita cambiar una ruta
   de otra tarea, escribe en la bitácora y espera; no la edita.
4. **Contratos.** `internal/modelo` (tipos e interfaces compartidos) es de C0. Después de C0, un cambio de contrato
   se pide en la bitácora con `CONTRATO:`; lo aplica quien orquesta en un commit aparte y avisa a los afectados.
   Mientras tanto, la tarea usa un adaptador local, no edita `internal/modelo`.
5. **Terminar.** Una tarea está `hecha` cuando: `go test ./...` en verde, su criterio de *Hecha cuando* se cumple
   con una prueba que compara contra un esperado, rebase sobre master, merge fast-forward a master, y su fila
   actualizada con el hash del commit. Commits atómicos y convencionales (ver `AGENTS.md`).
6. **Abrir dependientes.** Al cerrar una tarea, quien la cierra pasa a `lista` toda tarea cuyas dependencias quedaron
   todas `hecha`, en el mismo commit que actualiza su fila.
7. **Bloqueos.** Si una tarea no puede seguir: Estado `bloqueada`, una línea en la bitácora con el motivo y lo que
   necesita. No se queda en `en curso` sin avanzar más de 20 minutos sin nota.
8. **Sin datos reales aquí.** Este archivo, la bitácora y los logs llevan ids de registro y conteos, nunca nombres,
   cédulas, correos ni celulares reales.
9. **Casos esquina.** Un caso nuevo descubierto se agrega a `CASOS_ESQUINA.md` con su juez antes de corregirlo.
10. **Decisiones.** Cambiar una decisión de `DISEÑO.md` §7 lo decide el usuario; la tarea anota la propuesta en la
    bitácora con `DECISIÓN:` y sigue con lo vigente.
11. **Tiempo.** Cada paso de red lleva timeout. Ningún agente lanza corridas oficiales.

Estados: `espera` (dependencias abiertas) · `lista` · `en curso` · `bloqueada` · `hecha`.

## Tablero C · Implementación (Go)

### Ola 0 · contratos (secuencial)

| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C0 | `go.mod` (Go 1.24, `modernc.org/sqlite`), árbol de paquetes vacío, y en `internal/modelo`: `ChatRecord`, `DocRecord`, `TableRecord`, `Persona` (llave, rol, género, formas), `Forma` (tipo, valor), `Perfil`, `Mencion` (offsets, tipo, persona, regla, confianza), `Entity`; interfaces `Almacen`, `Detector`, `Resolutor`, `Fabrica`, `Modelo` (LLM), `Nucleo.Anonymize(source, records)`. | `go.mod`, `internal/modelo/` | — | Compila; los tipos serializan a JSON igual que `sample/expected_output` (prueba de ida y vuelta) | — | lista | — |

### Ola 1 · piezas independientes (paralelo tras C0)

| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C1 | Servicio: `cmd/servidor`, rutas de `ServeMux` (`/v1/health`, `/run`, `/run/{id}`, `/output/{archivo}`, `/anonymize`, `/entities`), jobs en goroutine, apagado limpio, `WriteTimeout` ≥ 70 s, captura de requests (`internal/captura`, sin PII en stdout). Dockerfile Go + Python (pdfplumber, pypdf), `run.sh`, Railway (volumen `/data`, una réplica, healthcheck). `/run` usa `sources_url` y `token` del cuerpo; `.env` solo en local. | `cmd/servidor/`, `internal/servidor/`, `internal/captura/`, `Dockerfile`, `run.sh`, `railway.json` | C0 | `/v1/health` responde en Railway; `/anonymize` devuelve eco válido con el contrato | — | espera | — |
| C2 | Almacén SQLite: `Open()` con migraciones por `user_version` (reuso de `../10102026`); tablas `jobs`, `personas`, `formas`, `perfiles` (bóveda, solo agrega), `personas_vivo` (D24), `cache_modelo`, `dudosos`, `captura`. | `internal/almacen/` | C0 | Pruebas de reapertura y de "solo agrega" (un perfil no cambia) | — | espera | — |
| C3 | Normalización: port exacto de `clean`, `quitarTildes`, `plegar`, `norm`, `digitosEn` y `contiene` de `docs/validar.mjs`, con mapa de offsets al texto original. | `internal/norm/` | C0 | Tabla de ≥40 casos que da el mismo resultado que `contiene()` de Node (fixture generado con `node`) | — | espera | — |
| C4 | Cliente del API de fuentes: límite 600/min, timeout y reintentos, `ETag`/`If-None-Match`, descarga completa en memoria. | `internal/fuentes/` | C0 | Contra un servidor de prueba local: paginación por canal y fecha, 304, 429 con reintento | — | espera | — |
| C5 | Ingesta de tablas: `clientes.csv` (Latin-1, `;`, CRLF, comillas), `transacciones.csv`, `pqrs.xml` (atributos + CDATA; comentario XML fuera), `empleados.xml`. Todo como texto, mismas columnas. | `internal/ingesta/tablas/` | C0 | Sobre `sample/input`: mismos ids y mismas claves de `columns` que `sample/expected_output/tables.jsonl` | — | espera | — |
| C6 | Ingesta de chats: `users.json`, `channels.json`, mensajes → `ChatRecord` (`author` = nombre corto, `thread_id` = `canal:ts raíz`, adjuntos, editados); índice uid → usuario para el registro. | `internal/ingesta/chats/` | C0 | Sobre `sample/input`: mismos ids, `thread_id` y número de adjuntos que la salida ideal | — | espera | — |
| C7 | Ingesta de docs: MD con front matter; HTML → markdown con comentarios y `display:none`; PDF vía `tools/pdf/extraer.py` (pdfplumber cuerpo, pypdf metadata) como subproceso con timeout. Metadata con las mismas claves. | `internal/ingesta/docs/`, `tools/pdf/` | C0 | Sobre `sample/input`: mismas claves de metadata y similitud de `body` ≥ 0,9 con la salida ideal | — | espera | — |
| C8 | Formas: un renderizador por tipo (completo, corto, pila, MAYÚSCULAS sin tildes, slug, usuario, correo, correo deletreado, cédula con y sin puntos, `CC<num>`, celular en 3 formatos, dirección, fecha ISO / en palabras / US, placa, pasaporte). Lo usan el registro (formas reales) y la fábrica (formas falsas). | `internal/formas/` | C0 | Cada renderizador probado contra ejemplos de `sample/expected_output` | — | espera | — |
| C9 | Catálogos embebidos: nombres de pila colombianos por género, unisex y extranjeros; apellidos con tildes; dominios públicos; barrios por ciudad; ocupación → 13 sectores (D18). | `internal/catalogo/` | C0 | Conteos mínimos (≥300 por género, ≥400 apellidos) y sin ninguna forma real de `fuentes/` | — | espera | — |
| C10 | Cliente del modelo: OpenRouter con salida estructurada, timeout, caché en SQLite por hash, contabilidad de tokens y costo, modelo por `LLM_MODEL`, enmascarado de formas conocidas (D15). | `internal/llm/` | C0 | Prueba con servidor falso: caché, timeout, JSON inválido; una prueba real opcional con datos inventados | — | espera | — |
| C11 | Juez base: `cmd/juez` corre `validar.mjs`, los 13 tests de `tests_sample.json` y los 5 públicos por familia, J-GLO y J-DIC; imprime puntaje por juez. | `cmd/juez/`, `internal/juez/` | C0 | Sobre `sample/expected_output` da 13/13 y J-GLO 0 | — | espera | — |

### Ola 2 · pipeline

| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C12 | Registro de personas: maestro + empleados + users (`users.name` = parte local del correo; `@kipupay.co`), llaves `p_cl_`/`p_kp_`, todas las formas reales vía C8, índices exactos (cédula, correo, celular, `CL-`, `PQR-`, placa, uid, fecha+monto → tx). | `internal/registro/` | C3, C5, C6, C8 | Cada persona de la muestra tiene las `forms` de sus tests de consistencia | — | espera | — |
| C13 | Fábrica de perfiles: `HMAC(secreto, llave)`, mismo género y estructura, unicidad, exclusión de toda forma real, bóveda primero (D4, D22). | `internal/perfil/` | C2, C8, C9 | Determinista con la misma llave; nunca coincide con una forma real; perfil guardado no cambia | — | espera | — |
| C14 | Detección: Aho-Corasick en espacio normalizado, patrones con contexto (cédula, celular, correo, deletreado, placa, pasaporte, fechas, adjuntos), allowlist con reglas genéricas, candidatos a nombre desconocido; dos pasadas (D14). | `internal/detecta/` | C3, C12 | Casos CE-03, 04, 08, 14, 15, 16, 21, 22, 23 en verde | — | espera | — |
| C15 | Residuo con modelo: candidatos dudosos de C14 → modelo (enmascarado) → fragmentos y tipo; género y origen (D27). | `internal/detecta/residuo/` | C10, C14 | Con un modelo falso: integra respuestas; con timeout cae a reglas | — | espera | — |
| C16 | Resolución: ids duros → contexto (autor, hilo, handoff, "soy X") → nombre único → modelo con fichas abstractas (D26) → D19; regla y confianza por mención; dudosos; terceros (D22, D23). | `internal/resuelve/` | C10, C12, C14 | CE-01, 02, 17, 18, 19, 28 en verde | — | espera | — |
| C17 | Reemplazo con estilo (derecha a izquierda, todos los strings del registro), compuerta (D25, hashes sin sal), salida (`*.jsonl`, `entities.json` con todas las formas). | `internal/reemplazo/`, `internal/compuerta/`, `internal/salida/` | C3, C13, C16 | Muestra: validador OK, 13/13, J-GLO 0 | — | espera | — |
| C18 | Núcleo y cableado: `Anonymize` único para `/run` y `/anonymize` (D8, D24, D28); job de `/run` = fuentes → ingesta → núcleo → salida en `/data`; costo en `/run/{id}`. Corrida completa sobre `fuentes/`, despliegue y smoke test. | `internal/nucleo/`, cableado en `internal/servidor/` | C1, C4, C7, C15, C17 | `fuentes/`: validador OK, barrido 0, dos corridas idénticas, `/run` < 15 min en Railway | — | espera | — |

### Ola 3 · después de la corrida 1

| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C19 | Cuasi-identificadores en texto y maestro (opcional antes de la corrida 1 si sobra tiempo). | `internal/cuasi/` | C17 | CE-12, CE-13 con J-IND en verde | — | espera | — |
| C20 | Jueces especializados restantes (J-DIR, J-OCU, J-IND, J-FP, J-CAT, J-EST, J-PAR, J-CON, J-HOM, J-REP, J-CRE, J-VIV) y los 28 casos de `CASOS_ESQUINA.md` como fixtures. | `internal/juez/`, `tests/casos/` | C11, C18 | Todos los casos corren; reporte por juez | — | espera | — |
| C21 | Monitor de crecimiento: `record_index` con `If-None-Match`; avisa cuando cambia. | `cmd/monitor/` | C4 | Detecta un cambio simulado | — | espera | — |
| C22 | Hold-out 80/20 y prueba de crecimiento (J-CRE). | `internal/juez/`, `scripts/` | C18, C20 | 0 seudónimos viejos cambian | — | espera | — |
| C23 | `DECISIONS.md` (qué es PII, qué se conservó, costo en tokens y tiempo) y guion de demo. | `DECISIONS.md`, `docs/demo/` | C18 | Responde las 3 preguntas del enunciado con cifras de la corrida | — | espera | — |
| C24 | Respaldo de la bóveda a disco local fuera de git tras cada corrida. | `scripts/` | C2, C18 | Restaurar el respaldo reproduce `entities.json` | — | espera | — |

## Pendientes de decisión

| Id | Pregunta | Afecta | Estado |
| --- | --- | --- | --- |
| P1 | ¿Qué modelo usa C10? En OpenRouter solo existe `typesafe/jev-router`, que reparte cada petición a otro modelo (en la prueba: GPT-6 Luna y Sonnet 5.5, 2–4 s, costo variable ×30). Jev nativo (70–500 ms, $0,042/M) está en el API de TypeSafe con lista de espera. | C10, D13, D15 | abierta |
| P2 | En `/anonymize`, `author` llega como **nombre**, no como uid: ¿cómo se separan dos homónimos en vivo sin uid? | C16, C18 | abierta |

## Bitácora

Una línea por evento, la más nueva arriba: `HH:MM · Cn · agente · qué pasó`. Prefijos: `CONTRATO:`, `DECISIÓN:`,
`BLOQUEO:`, `CASO:`.

- 12:20 · — · claude · Tablero creado; diseño cerrado en D1–D28; C0 lista.
