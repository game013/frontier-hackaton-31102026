# Progreso · implementación

Tablero vivo para coordinar a varios agentes en paralelo. Lo que se decide va a `DISEÑO.md` §7; los casos esquina
a `CASOS_ESQUINA.md`; aquí solo vive el **estado del trabajo**.

## Estado ahora

- **Fase:** ola 3, sobre `integracion/ola3`. C19, C21 y C24 integradas; C23 espera las cifras de la corrida 1.
- **Próximo paso:** C23, que solo necesita la corrida 1; C20 y C22 siguen en `espera` de C27 (fixtures).
- **Corrida 1:** 15:30 (Bogotá). El usuario la lanza con VALIDAR ENTRYPOINT; el agente solo la propone.
- **Pendientes de decisión:** ninguno (P1 → D13, P2 → D29).
- **Bloqueos:** ninguno.

## Hitos de hoy (Bogotá, UTC−5)

- [x] 12:45 · C0 hecha: `go.mod`, paquetes y tipos compartidos en `integracion/c0-c11`.
- [x] 13:30 · Servicio en Railway con `/v1/health` y `/v1/anonymize` de eco (C1): `frontier-31102026-production.up.railway.app`.
- [x] 14:15 · Pipeline completo sobre `sample/`: validador OK, 12/13 tests (`sample_08` pasó a C19) y J-REC 100 % (C17).
- [x] 14:45 · Pipeline sobre `fuentes/`: validador OK, dos corridas idénticas; J-GLO 99,8 % aceptado en CE-33 (C18).
- [ ] 15:00 · **Congelar** y desplegar. Después de esta hora solo entran arreglos de C18.
- [x] 15:15 · Smoke test público: `/v1/health` 200 y `/v1/anonymize` con un registro de la muestra, ya verificado en Railway.
- [ ] 15:30 · **Corrida 1** (la lanza el usuario).
- [ ] Tras el crecimiento: corridas 2 y 3 (las propone el agente; las lanza el usuario).

## Reglas de coordinación

0. **Rama de integración.** El trabajo entra a `integracion/ola3`, no a `master` ni a las ramas de las olas
   anteriores (`integracion/ola2`, `integracion/c0-c11`). Las tareas se cortan de esa rama, se integran ahí y `master`
   solo se mueve cuando el usuario lo pida (merge de la rama completa o PR). Motivo: aísla el trabajo de la ola 0–1,
   permite revisar el conjunto antes de publicar y deja `master` desplegable.
1. **Tomar una tarea.** Solo se toma una tarea en estado `lista`. Se toma editando su fila (Estado `en curso`,
   Responsable, hora) y haciendo commit inmediato en `integracion/ola3` con `chore: toma Cn` (solo `PROGRESS.md`). Si
   el commit choca con otro agente que la tomó primero, se suelta y se elige otra.
2. **Aislamiento.** Cada tarea trabaja en su worktree `../31102026-wt/Cn` sobre la rama `cn-<slug>`
   (`git worktree add ../31102026-wt/Cn -b cn-<slug> integracion/ola3`). Nunca dos agentes en el mismo worktree.
3. **Propiedad de rutas.** Cada tarea solo escribe en las rutas de su columna *Toca*. Si necesita cambiar una ruta
   de otra tarea, escribe en la bitácora y espera; no la edita.
4. **Contratos.** `internal/modelo` (tipos e interfaces compartidos) es de C0. Después de C0, un cambio de contrato
   se pide en la bitácora con `CONTRATO:`; lo aplica quien orquesta en un commit aparte y avisa a los afectados.
   Mientras tanto, la tarea usa un adaptador local, no edita `internal/modelo`.
5. **Terminar.** Una tarea está `hecha` cuando: `go test ./...` en verde, su criterio de *Hecha cuando* se cumple
   con una prueba que compara contra un esperado, merge de la punta de `integracion/ola3` a su rama, y merge a
   `integracion/ola3`. La fila (Estado `hecha`, hash del commit) la actualiza quien integra, no cada tarea, para
   evitar conflictos en este archivo. Commits atómicos y convencionales (ver `AGENTS.md`).
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

Estados: `espera` (dependencias abiertas) · `lista` · `en curso` · `bloqueada` · `hecha` · `descartada` (no se hará; motivo en la bitácora).

## Tablero C · Implementación (Go)

### Ola 0 · contratos (secuencial)

| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C0 | `go.mod` (Go 1.24, `modernc.org/sqlite`), árbol de paquetes vacío, y en `internal/modelo`: `ChatRecord`, `DocRecord`, `TableRecord`, `Persona` (llave, rol, género, formas), `Forma` (tipo, valor), `Perfil`, `Mencion` (offsets, tipo, persona, regla, confianza), `Entity`; interfaces `Almacen`, `Detector`, `Resolutor`, `Fabrica`, `Modelo` (LLM), `Nucleo.Anonymize(source, records)`. | `go.mod`, `internal/modelo/` | — | Compila; los tipos serializan a JSON igual que `sample/expected_output` (prueba de ida y vuelta) | orquestador | hecha | 00c556c |

### Ola 1 · piezas independientes (paralelo tras C0)

| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C1 | Servicio: `cmd/servidor`, rutas de `ServeMux` (`/v1/health`, `/run`, `/run/{id}`, `/output/{archivo}`, `/anonymize`, `/entities`), jobs en goroutine, apagado limpio, `WriteTimeout` ≥ 70 s, captura de requests (`internal/captura`, sin PII en stdout). Dockerfile multi-etapa Go + Python (`pdfplumber`, `pypdf` con versión fija; comprobación del ayudante al arrancar, fuera del healthcheck; D33), `run.sh`, Railway (volumen `/data`, una réplica, healthcheck). `/run` usa `sources_url` y `token` del cuerpo; `.env` solo en local. | `cmd/servidor/`, `internal/servidor/`, `internal/captura/`, `Dockerfile`, `run.sh`, `railway.json` | C0 | `/v1/health` responde en Railway; `/anonymize` devuelve eco válido con el contrato | orquestador | hecha | 1b88ec1 |
| C2 | Almacén SQLite: `Open()` con migraciones por `user_version` (reuso de `../10102026`); tablas `jobs`, `personas`, `formas`, `perfiles` (bóveda, solo agrega), `personas_vivo` (D24), `cache_modelo`, `dudosos`, `captura`. | `internal/almacen/` | C0 | Pruebas de reapertura y de "solo agrega" (un perfil no cambia) | orquestador | hecha | 87e388f |
| C3 | Normalización: port exacto de `clean`, `quitarTildes`, `plegar`, `norm`, `digitosEn` y `contiene` de `docs/validar.mjs`, con mapa de offsets al texto original. | `internal/norm/` | C0 | Tabla de ≥40 casos que da el mismo resultado que `contiene()` de Node (fixture generado con `node`) | orquestador | hecha | 2b7a176 |
| C4 | Cliente del API de fuentes: límite 600/min, timeout y reintentos, `ETag`/`If-None-Match`, descarga completa en memoria. | `internal/fuentes/` | C0 | Contra un servidor de prueba local: paginación por canal y fecha, 304, 429 con reintento | orquestador | hecha | 91618a0 |
| C5 | Ingesta de tablas: `clientes.csv` (Latin-1, `;`, CRLF, comillas), `transacciones.csv`, `pqrs.xml` (atributos + CDATA; comentario XML fuera), `empleados.xml`. Todo como texto, mismas columnas. | `internal/ingesta/tablas/` | C0 | Sobre `sample/input`: mismos ids y mismas claves de `columns` que `sample/expected_output/tables.jsonl` | orquestador | hecha | 46319d5 |
| C6 | Ingesta de chats: `users.json`, `channels.json`, mensajes → `ChatRecord` (`author` = nombre corto, `thread_id` = `canal:ts raíz`, adjuntos, editados); índice uid → usuario para el registro. | `internal/ingesta/chats/` | C0 | Sobre `sample/input`: mismos ids, `thread_id` y número de adjuntos que la salida ideal | orquestador | hecha | 317468e |
| C7 | Ingesta de docs: MD con front matter; HTML → markdown con comentarios y `display:none`; PDF vía `tools/pdf/extraer.py` (pdfplumber cuerpo, pypdf metadata) como subproceso con timeout; respaldo pypdf texto plano y luego `body` vacío, nunca se pierde el registro (D33). Metadata con las mismas claves. | `internal/ingesta/docs/`, `tools/pdf/` | C0 | Sobre `sample/input`: mismas claves de metadata y similitud de `body` ≥ 0,9 con la salida ideal | orquestador | hecha | 7ea36ff |
| C8 | Formas: un renderizador por tipo (completo, corto, pila, MAYÚSCULAS sin tildes, slug, usuario, correo, correo deletreado, cédula con y sin puntos, `CC<num>`, celular en 3 formatos, dirección, fecha ISO / en palabras / US, placa, pasaporte). Lo usan el registro (formas reales) y la fábrica (formas falsas). | `internal/formas/` | C0 | Cada renderizador probado contra ejemplos de `sample/expected_output` | orquestador | hecha | 8b49233 |
| C9 | Catálogos embebidos: nombres de pila colombianos por género, unisex y extranjeros; apellidos con tildes; dominios públicos; barrios por ciudad; ocupación → 13 sectores (D18). | `internal/catalogo/` | C0 | Conteos mínimos (≥300 por género, ≥400 apellidos) y sin ninguna forma real de `fuentes/` | orquestador | hecha | 4314456 |
| C10 | Cliente de Jev 1.14: OpenRouter `POST /api/alpha/decisions` (`state`, `questions` `choice`/`noul`/`score`), una llamada por mención con su propio `state`, en paralelo con tope de 16 (D28), umbrales sobre `probabilities` (D31), snapshot fijo `typesafe/jev-1.14-20261014` en la llave de caché (D32), timeout, caché en SQLite por hash, contabilidad de tokens y costo, modelo por `LLM_MODEL`, enmascarado de formas conocidas (D15). | `internal/llm/` | C0 | Prueba con servidor falso: caché, timeout, JSON inválido; una prueba real opcional con datos inventados | orquestador | hecha | a2a21ec |
| C11 | Juez base: `cmd/juez` corre `validar.mjs`, los 13 tests de `tests_sample.json` y los 5 públicos por familia, J-GLO, J-DIC y **J-REC** (cobertura de personas contra `sample/expected_output/entities.json`, D34); imprime puntaje por juez. | `cmd/juez/`, `internal/juez/` | C0 | Sobre `sample/expected_output` da 13/13, J-GLO 0 y J-REC 100 % | orquestador | hecha | 0d26441 |

### Ola 2 · pipeline

| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C12 | Registro de personas: maestro + empleados + users (`users.name` = parte local del correo; `@kipupay.co`), llaves `p_cl_`/`p_kp_`, todas las formas reales vía C8, índices exactos (cédula, correo, celular, `CL-`, `PQR-`, placa, uid, fecha+monto → tx). | `internal/registro/` | C3, C5, C6, C8 | Cada persona de la muestra tiene las `forms` de sus tests de consistencia | pi | hecha | 254e322 |
| C13 | Fábrica de perfiles: `HMAC(secreto, llave)`, mismo género y estructura, unicidad, exclusión de toda forma real, bóveda primero (D4, D22). | `internal/perfil/` | C2, C8, C9 | Determinista con la misma llave; nunca coincide con una forma real; perfil guardado no cambia | pi | hecha | ffee7c7 |
| C14 | Detección: Aho-Corasick en espacio normalizado, patrones con contexto (cédula, celular, correo, deletreado, placa, pasaporte, fechas, adjuntos), allowlist con reglas genéricas, cuatro generadores de candidatos insensibles a mayúsculas (D30); dos pasadas (D14). | `internal/detecta/` | C3, C12 | Casos CE-03, 04, 08, 14, 15, 16, 21, 22, 23, 29, 30, 31 en verde | pi | hecha | 905248e |
| C15 | Residuo con Jev: candidatos dudosos de C14 → pregunta cerrada (persona, organización, lugar, marca, otro) con la mención enmascarada; género y origen (D27). | `internal/detecta/residuo/` | C10, C14 | Con un modelo falso: integra respuestas; con timeout cae a reglas | pi | hecha | f4dd445 |
| C16 | Resolución: ids duros → contexto (autor, hilo, handoff, "soy X") → nombre único → Jev con fichas abstractas (D26) → D19; en vivo, D29; regla y confianza por mención; dudosos; terceros (D22, D23). | `internal/resuelve/` | C10, C12, C14 | CE-01, 02, 17, 18, 19, 28 en verde | pi | hecha | e64d8dc |
| C17 | Reemplazo con estilo (derecha a izquierda, todos los strings del registro), compuerta (D25, hashes sin sal), salida (`*.jsonl`, `entities.json` con todas las formas). | `internal/reemplazo/`, `internal/compuerta/`, `internal/salida/` | C3, C13, C16 | Muestra: validador OK, 12/13 (`sample_08` mide cuasi-identificadores y pasa a C19), J-GLO 0, J-REC 100 % | pi | hecha | 4074f72 |
| C18 | Núcleo y cableado: `Anonymize` único para `/run` y `/anonymize` (D8, D24, D28); job de `/run` = fuentes → ingesta → núcleo → salida en `/data`; costo en `/run/{id}`. Corrida completa sobre `fuentes/`, despliegue y smoke test. | `internal/nucleo/`, cableado en `internal/servidor/` | C1, C4, C7, C15, C17 | `fuentes/`: validador OK, dos corridas idénticas, `/run` probado contra un API falso; J-GLO 99,8 % aceptado en CE-33, `/run` en Railway lo mide C28 | pi | hecha | 20a6566 |

### Ola 3 · después de la corrida 1

| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C19 | Cuasi-identificadores en texto y maestro: ocupación → sector, barrio → ciudad, año de la relación → "hace unos años", en el texto y en la fila del maestro. Incluye `sample_08` (movido aquí desde C17 por decisión del usuario: la corrida 1 no lleva cuasi en texto, D21). | `internal/cuasi/` | C17 | CE-12, CE-13 con J-IND en verde; `sample_08` en verde | orquestador | hecha | 8cd99e5 |
| C20 | Jueces especializados restantes (J-DIR, J-OCU, J-IND, J-FP, J-CAT, J-EST, J-PAR, J-CON, J-HOM, J-REP, J-CRE, J-VIV) que corren los fixtures de C27. | `internal/juez/` | C11, C18, C27 | Todos los casos corren; reporte por juez | — | espera | — |
| C21 | Monitor de crecimiento: `record_index` con `If-None-Match`; avisa cuando cambia. | `cmd/monitor/` | C4 | Detecta un cambio simulado | orquestador | hecha | 8019a54 |
| C22 | Hold-out 80/20 y prueba de crecimiento (J-CRE). | `internal/juez/`, `scripts/` | C18, C20 | 0 seudónimos viejos cambian | — | espera | — |
| C23 | `DECISIONS.md`: llenar las cifras de costo en tokens y tiempo sobre el esqueleto de C29; guion de demo. | `DECISIONS.md`, `docs/demo/` | C18, C29 | Responde las 3 preguntas del enunciado con cifras de la corrida | orquestador | en curso | — |
| C24 | Respaldo de la bóveda a disco local fuera de git tras cada corrida. | `scripts/` | C2, C18 | Restaurar el respaldo reproduce `entities.json` | orquestador | hecha | 2de54bd |

C23 sigue `en curso` y le falta una sola cosa: las seis cifras de `DECISIONS.md` §3.4 con la corrida 1 (tokens de
entrada y de salida, llamadas y aciertos de caché, dólares, segundos de `/run` y segundos del lote de 50 en
`/anonymize`). El esqueleto, la tabla de comandos y el guion de demo ya están escritos.

C24 se cierra con el respaldo encadenado a mano tras cada corrida: el volumen de Railway no empuja nada, así que el
operador baja `almacen.db` a una carpeta local fuera de git y desde ahí corre `scripts/boveda.sh respaldo --datos
<copia> --destino <disco local>`. El script no entra a Railway, no lanza corridas, no programa cron y no imprime
formas reales; el detalle está en la cabecera de `scripts/boveda.sh`.

### Pista paralela · sin dependencias (lista desde ya)

No dependen de la ola 1 y acortan la ola 2 o desbloquean hitos. Se cortan de `integracion/ola3` como cualquier
tarea (reglas 0–2). C25 hecha. Siguen C26 y C27 (y C29 para el esqueleto de `DECISIONS.md`).

| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C25 | **Calibrar los umbrales de Jev (D31).** Etiquetar candidatos de `sample/` con su verdad (`expected_output/entities.json`: persona o no, género), incluir negativos (marcas, bancos, lugares, apellido-lugar, "Claro"), pasarlos por `typesafe/jev-1.14-20261014` y medir precisión y cobertura por umbral para persona, desempate y género. Solo ids y conteos en el informe. | `scripts/calibrar_jev.py`, `analisis/06_calibracion.md` | — | Informe con la curva por umbral y un valor recomendado por decisión; si difiere de D31, propuesta `DECISIÓN:` en la bitácora | claude | hecha | 10a87e5 |
| C26 | **Allowlist como datos.** Las ~30 entidades a conservar (`analisis/00_resumen.md`) con `reason`, más las reglas genéricas (sufijos `S.A.S.`, "Banco …", mayúscula sin nombre de pila delante, apellido-lugar). C14 solo la carga; no edita este archivo. | `internal/detecta/allowlist.json` | — | JSON válido con prueba de esquema; cubre todas las entidades de `analisis/00_resumen.md` y los `kept` de `sample/expected_output/entities.json` | pista 2 | hecha | ca001d0 |
| C27 | **Fixtures de los casos esquina.** Cada caso de `CASOS_ESQUINA.md` (CE-01 a CE-32) como archivo con id de registro, campo, entrada y esperado (qué debe desaparecer, qué debe quedar, qué juez lo mide). Es la parte de datos de C20. | `tests/casos/` | — | Un fixture por caso, legible por `cmd/juez`; ningún dato real fuera de los ids y de lo estrictamente necesario para el esperado | pi | en curso | — |
| C28 | **Railway listo.** Proyecto, volumen en `/data`, una réplica, Serverless apagado, reinicio `ON_FAILURE`, healthcheck `/v1/health`; variables `LLM_MODEL=typesafe/jev-1.14-20261014`, `OPENROUTER_API_KEY`, secreto del HMAC. | Panel de Railway (fuera del repo) | — | C1 solo tiene que desplegar; variables verificadas sin mostrarlas | usuario | lista | — |
| C29 | **Esqueleto de `DECISIONS.md`.** Las tres preguntas del enunciado: qué es PII, qué se conservó y por qué, costo. Declara D15 (filtración al modelo) y que `provider.data_collection: deny` se acepta pero no está probado que se cumpla. Deja huecos marcados para las cifras de la corrida. | `DECISIONS.md` | — | Las tres secciones escritas; solo faltan cifras (las llena C23) | pi | hecha | 3deb043 |

## Pendientes de decisión

| Id | Pregunta | Afecta | Estado |
| --- | --- | --- | --- |
| P1 | ¿Qué modelo usa C10? | C10, D13, D15 | cerrada → D13: `typesafe/jev-1.14` por `/api/alpha/decisions` |
| P2 | En `/anonymize`, `author` llega como nombre: ¿cómo se separan homónimos en vivo? | C16, C18 | cerrada → D29 |

## Bitácora

Una línea por evento, la más nueva arriba: `HH:MM · Cn · agente · qué pasó`. Prefijos: `CONTRATO:`, `DECISIÓN:`,
`BLOQUEO:`, `CASO:`.

- 16:40 · C24 · worker · El respaldo queda encadenado tras cada corrida y escrito como lo que es: un jalado desde la
  máquina del operador, porque el volumen de Railway no puede empujar nada. `scripts/boveda.sh` y la nota de C24 en
  este tablero dicen la secuencia (job en `done`, copia local del volumen, respaldo al disco) y qué no hace el script:
  no entra a Railway, no lanza corridas, no programa cron y no escribe dentro del repo.
- 16:35 · C19 · worker · `CASO:` CE-38: una fecha de negocio detrás de un "desde" suelto se conservaba por casualidad.
  El año del texto ahora exige ancla de relación ("llevo con ustedes", "cliente desde", "vinculado desde", "con
  nosotros"); "Kipu Pay opera desde 2020" y "la empresa abrió desde 2019" se quedan. La puntuación no ASCII dejó de
  pegar palabras, así que "desde 2024…" también se reconoce. Sobre `fuentes/` la salida es byte a byte igual que la
  punta de la rama: los cinco registros de CE-13 siguen generalizados y no se perdió ninguna fecha de negocio.
- 16:32 · — · worker · Revisión de la rama atendida: los arreglos entran en commits atómicos sobre
  `integracion/ola3`; `go build`, `go vet` y `go test ./...` en verde, muestra 13/13 con J-GLO y J-REC 100 % y
  validador OK, corpus con el mismo cuadro que la punta de la rama. Quedaron fuera los olores opcionales del review
  (`cargaEntorno` repetido, el switch de ETag de `cmd/monitor`, el switch sobre `modelo.Registro`).
- 16:28 · C23 · worker · El costo de `/run/{id}` es el de esa corrida: el ejecutor fotografía la contabilidad antes de
  la primera llamada y publica la diferencia al terminar, porque el mismo cliente de Jev también carga
  `/anonymize`. Prueba con acumulado creciente: 1000/100 en la primera corrida, 500/80 en la segunda y 0/0 en la que
  sirvió todo de caché.
- 16:22 · C19 · worker · `DECISIÓN:` los cuasi-identificadores quedan como están —barrio alterno de la misma ciudad
  en `clientes`, barrio → ciudad y año → "hace unos años" en el texto— y las tres afirmaciones se estrechan al código
  real: `clientes.ocupacion` y no "todas las filas", con `empleados.cargo` conservado como rol del empleado en la
  organización. `DECISIONS.md` §1.3, D9 y CE-12 dicen ahora lo mismo.
- 16:18 · C23 · worker · `DECISIONS.md` §3.4 ya no tiene huecos de comentario HTML: cada celda dice "pendiente de la
  corrida 1", así el entregable del jurado no muestra una columna en blanco.
- 16:12 · — · worker · `AGENTS.md`: la tabla de directorios nombra `cmd/servidor/`, `cmd/pipeline/`, `cmd/juez/`,
  `cmd/monitor/`, `cmd/respaldo/`, `internal/cuasi/`, `internal/juez/` e `internal/respaldo/`, y `docs/demo/` entra a
  la excepción de solo lectura de `docs/`. `tests/` todavía no existe: se agrega cuando llegue con C27.
- 16:05 · C19 · worker · El cableado de cuasi tiene prueba de punta a punta en `internal/nucleo`: un lote sintético
  con la ocupación de una persona y el año de su relación sale generalizado y la fecha de negocio del mismo texto se
  conserva. Antes, C19 solo se ejercía a mano.
- 15:55 · C19, C21, C23, C24 · pi · Ola 3 tomada sobre `integracion/ola3` (cortada de `master` `ef6b879`): la
  frontera que deja la corrida 1. Cuatro implementadores en paralelo, cada uno en su worktree `../31102026-wt/Cn`
  sobre su rama; C20 y C22 siguen en espera de C27 (fixtures). C23 incluye el pendiente de C18
  (`cost.tokens_in/tokens_out` en 0) y sus cifras quedan para la corrida oficial.
- 15:05 · C18 · pi · Ola 2 cerrada. `/run` es real (`20a6566`): descarga el API, materializa `chats/ docs/ tables/`
  en el volumen, ingesta con `nucleo.CargaCorpus`, anonimiza con el núcleo de corrida (`Vivo:false`, D24), escribe
  `output/` y compara la salida con `record_index` antes de responder. Prueba contra un API falso con la muestra.
  Medido en local por HTTP contra un API que sirve `fuentes/`: 2028 chats, 26 docs, 6243 tablas, 3,3 s, validador OK.
  Dos corridas con bóvedas distintas sobre `fuentes/`: byte a byte idénticas (`diff -r` vacío).
- 15:00 · C18 · pi · `CASO:` CE-33 y CE-34 quedan **aceptados con motivo** en `CASOS_ESQUINA.md`. CE-33: J-GLO
  3909/3917 (99,8 %) sobre `fuentes/`; las 8 formas son 4 nombres de pila compartidos por dos personas cada uno
  (dos pares en `doc_22`, dos pares en 20 y 7 mensajes), y D25 dice no barrer nombres de pila sueltos. CE-34: el
  test público `pub_04` no es satisfacible sin declarar una forma que no es de la persona.
- 14:55 · C18 · pi · `CASO:` cuatro hallazgos de la corrida real ya están corregidos y con caso: fecha de negocio
  tomada como nacimiento (CE-35: la compuerta barría 289 fechas de transacciones), placa que parte una palabra
  (CE-36: `sep202` dentro de `sep2026`), y formas de personas nuevas sin indexar (CE-37: `AgregaPersona` copiaba
  `Formas` antes de indexarlas). El caso del monto que parecía cédula ya vive en CE-23: hoy el contexto solo mira
  delante y una cédula que es `monto` de `transacciones` se descarta (`sinMontos`), sin crear tres personas por la
  misma cadena.
- 14:15 · — · pi · Estado para el relevo: muestra con validador OK, 12/13 (sólo `sample_08`, cuasi, va con C19),
  J-GLO 100 %, J-REC 100 %, J-DIC 100 %; corpus con validador OK, 4/5 públicos (`pub_04` es dato del test) y
  J-GLO 99,8 % (8 nombres de pila compartidos en `doc_22`). `/v1/anonymize` ya anonimiza de verdad; `/run` sigue con
  `EjecutorPendiente`. Juez completo: 320 s → 10 s tras indexar la salida en `internal/norm`.
- 13:35 · — · pi · Ola 2 tomada (C12–C18) sobre `integracion/ola2` desde `master` (`dbb4bef`), más C27 y C29 de la
  pista paralela. C26 no se toca: la trabaja otro agente. `DECISIÓN:` C17 ya no exige 13/13: `sample_08` mide
  cuasi-identificadores (`fotógrafa`/`Granada`/`2025`) y pasa a C19, en línea con D21 (corrida 1 sin cuasi en texto).

- 15:00 · — · claude · `master` recibe `integracion/c0-c11` por fast-forward (71 commits, sin merge commit). Sin push:
  publicar primero exige limpiar del historial los blobs con datos de la muestra anteriores a `8f809f2`.
- 14:52 · — · claude · Tercera revisión atendida: los 422 ya no citan valores, `/v1/health` cae al snapshot fijo de Jev
  si no hay `MODELOS` ni `LLM_MODEL` (con prueba), la metadata del PDF tiene prueba propia, y fuera de `scripts/` los
  comentarios con cédulas de la muestra. Cruzadas 921 formas de la muestra y del corpus contra lo versionado: solo
  quedan coincidencias en `analisis/*.md` (notas de diseño que citan registros y son anteriores a esta tanda) y en
  nombres de pila genéricos del catálogo.
- 14:40 · — · claude · Segunda revisión atendida: `/anonymize` responde 422 y tolera campos de más, la metadata del PDF
  conserva sus claves aunque falle la extracción, `/v1/health` reporta `LLM_MODEL` cuando no hay `MODELOS` (D32), el
  nombre corto de chats y formas coincide, J-GLO barre también las formas de menos de 3 caracteres y la prueba real de
  Jev compara contra `person`. Rechazados con motivo: reescribir el historial (rama sin publicar; los datos de
  `sample/` son sintéticos; el árbol actual está limpio), triggers en la bóveda, bucle de reintentos del cliente y
  los olores de estilo.
- 14:28 · — · claude · Revisión de la ola atendida: fuera del repo las identidades reales que quedaban en tests y
  fixtures (inventadas en su lugar), detalle del juez enmascarado, `Close` del lector con error, y la caché de
  `internal/llm` alineada con la tabla `cache_modelo` de C2. Queda anotado: la tabla `captura` de C2 admite cuerpos
  con PII (decidir en C18), las pruebas de PDF se saltan sin `PDF_PYTHON`, y los 5 tests públicos se omiten contra
  la muestra hasta tener `output/` real.
- 14:20 · — · claude · Ola 1 cerrada: C1–C11 integradas y verdes. Juez sobre la muestra: validador OK, 13/13, J-GLO 0,
  J-DIC y J-REC 100 %. Servicio desplegado en `frontier-31102026-production.up.railway.app` (health y `/anonymize`
  de eco verificados). Faltan en el panel de Railway: variables (`LLM_MODEL`, `OPENROUTER_API_KEY`, secreto HMAC) y
  volumen en `/data`; los pone el usuario.
- 14:13 · C25 · claude · C25 hecha (rama `c25-calibracion-jev`, punta `10a87e5`): 344 candidatos de `sample/` (290 persona / 39 no persona / 15 desempate),
  344 llamadas, US$ 0,006788. Precisión 1,000 en persona y desempate; exactitud 1,000 en género con cobertura
  0,988 en el 0,60 vigente. La muestra no separa clases: se mantiene D31 y se recalibra con el corpus real.
- 14:13 · C25 · claude · DECISIÓN: D31 se mantiene (persona > 0,50, desempate ≥ 0,70, género ≥ 0,60) porque la
  muestra de C25 no separa clases (precisión y exactitud en 1,000 en toda la rejilla y óptimo de la curva en el
  borde 0,30); se recalibra con el corpus real antes de mover umbrales.
- 14:13 · C25 · claude · CASO: CE-32 (nombre unisex) no se resuelve por el umbral de género: Jev decide M/F con
  probabilidad alta; el seudónimo unisex tiene que salir del catálogo (D18).
- 14:10 · — · claude · Ola 1 integrada: C1–C10 en `integracion/c0-c11` con `go build`/`go vet`/`go test`
  en verde; smoke local de C1 (health, `/anonymize` de eco, 404 de `/output`). C11 sigue en curso.
- 14:05 · C0 · claude · Revisión de C0 atendida: fixture versionada para la prueba de ida y vuelta, tags JSON de
  `Resultado` y `Respuesta`, `internal/` en la tabla de `AGENTS.md`. Contratos de `Almacen` y `Modelo` se amplían
  cuando C2 y C10 reporten su `CONTRATO:`.
- 13:40 · — · claude · Alcance fijado: solo C0–C11. Trabajo sobre `integracion/c0-c11` (regla 0); las filas las
  cierra quien integra. C0 en curso.
- 12:50 · — · claude · Ronda 4 del ataque cerrada: D28 corregida, D30–D34; Jev 1.14 con snapshot fijo.
- 12:47 · — · claude · Pista paralela C25–C29 agregada en `lista` (sin dependencias). C20 y C23 ahora dependen de C27 y C29.
- 12:35 · — · claude · P1 cerrada (Jev 1.14, D13) y P2 cerrada (D29).
- 12:20 · — · claude · Tablero creado; diseño cerrado en D1–D28; C0 lista.
