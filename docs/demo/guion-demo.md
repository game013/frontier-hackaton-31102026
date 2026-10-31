# Guion de la demo

Demo de cinco minutos para el Demo day (`RUBRICA.md` §3: pitch en el turno, 15 % de la nota). El hilo es una sola
frase: **el corpus sale vendible sin dejar a nadie dentro, y cada persona es una sola persona falsa en las tres
fuentes**. Todo lo que se muestra sale del servicio desplegado y de lo que ya corrió; las cifras que se dicen son las
de `DECISIONS.md` §3.4 (tokens, dólares y segundos de la corrida 1), nunca estimaciones.

## Antes de empezar

```bash
export URL="${URL:-https://frontier-31102026-production.up.railway.app}"
export TOKEN="$TOKEN"                 # token del API de fuentes (.env, nunca al repo)
export SOURCES_API_URL="$SOURCES_API_URL"
```

Cuatro preparativos que evitan el 90 % de las demos fallidas:

1. **Una corrida ya terminada.** La demo no espera dos horas: se lanza `/run` antes de entrar y se guarda el
   `job_id`. El estado se consulta del `GET /v1/run/{id}` varias veces, no una.
2. **Un registro de la muestra para `/anonymize`.** Se saca de `sample/` (datos sintéticos, git-ignored) y no se
   escribe en ningún archivo del repo: viaja en la petición y en la respuesta.
3. **Nada de formas reales en pantalla.** `GET /v1/entities` es el diccionario que el jurado exige y **nunca viaja con
   el dataset**; contiene las formas reales. Se proyecta solo lo que el comando de abajo deja ver: pseudónimos,
   conteos y records. Si hay que enseñar la estructura, se cortan los `originals`.
4. **Un plan B.** Si el servicio no responde, se muestra la captura de la corrida (`/data/captura.jsonl`, una línea
   por petición con ids y códigos) y la salida en `output/`. Se dice qué falló; no se improvisa.

## Orden de la demo

| Minuto        | Qué se muestra                    | Qué se dice                                                                                                                      |
| ------------- | --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| 0:00 – 0:40   | Nada: el trato                    | El corpus de Kipu Pay no se puede vender sin resolver dos condiciones a la vez: que no se fugue ninguna persona real y que lo que queda siga sirviendo para entrenar. La rúbrica cobra 45 puntos de fuga, 35 de utilidad y 20 de consistencia; la fuga no se compensa con utilidad (`DECISIONS.md` §2.2). |
| 0:40 – 1:20   | `GET /v1/health`                  | El pipeline es por reglas y el modelo es la excepción (D13). Solo se le piden **preguntas cerradas** a un snapshot fijo, `typesafe/jev-1.14-20261014` (D32): si el alias se moviera, cambiarían decisiones entre corridas. El residuo que viaja al modelo es el riesgo declarado, con enmascarado incondicional (D15, `DECISIONS.md` §3.3). |
| 1:20 – 2:20   | `POST /v1/run` y `GET /v1/run/{id}` | Una corrida regenera `output/` desde el API de fuentes: descarga, ingesta de las tres fuentes, registro de personas, detección en dos pasadas, reemplazo, generalización de cuasi-identificadores y compuerta de fuga (D11, D14). El job reporta registros por fuente y su costo real en tokens y segundos; los números que se dicen son los del reporte, no una diapositiva. |
| 2:20 – 3:10   | `GET /v1/output/chats.jsonl` y `/v1/output/tables.jsonl` | El texto no personal queda igual y los ids internos se conservan: los joins entre tablas siguen uniendo las mismas filas. Un chat y su fila del maestro muestran la misma clienta con el mismo nombre falso; la metadata del PDF sale con las mismas claves que entró (`DECISIONS.md` §1.1). |
| 3:10 – 3:40   | `GET /v1/entities`                | El diccionario es la pieza que mide consistencia y honestidad: por persona, todas sus formas y su perfil falso; por organización, lugar o figura pública, `kept` y su `reason` (`DECISIONS.md` §2.1). |
| 3:40 – 4:40   | `POST /v1/anonymize` en vivo      | Mismo núcleo que `/run` (D8), así que **una persona ya vista recibe el mismo seudónimo** que en `output/`; una persona nueva recibe uno nuevo, único y de la misma categoría, y dos personas con el mismo nombre reciben dos. Es la prueba del jurado y el gancho: se anonimiza lo que escriba el público. |
| 4:40 – 5:00   | El log de la corrida              | Cierre con cifras: tokens, dólares y segundos de la corrida 1 (`DECISIONS.md` §3.4), y qué queda abierto: `provider.data_collection: deny` se envía pero no se puede verificar desde adentro (D15). |

## Los comandos

**Salud.** Dice con qué modelo se tomó cada decisión.

```bash
curl -s "$URL/v1/health" | jq
```

**Lanzar la corrida.** Idempotente mientras hay una activa: un segundo `POST` devuelve el mismo job.

```bash
JOB=$(curl -s -X POST "$URL/v1/run" -H 'Content-Type: application/json' \
  -d "{\"sources_url\":\"$SOURCES_API_URL\",\"token\":\"$TOKEN\"}" | jq -r .job_id)
echo "$JOB"
```

**Estado y costo.** Mientras corre: `running` y los conteos. Al terminar: `done`, los registros por fuente y
`cost.tokens_in`, `cost.tokens_out` y `cost.seconds`.

```bash
curl -s "$URL/v1/run/$JOB" | jq '{status, records, cost}'
```

**Salida.** Un registro de chat y su fila del maestro, para enseñar el antes y el después en la misma pantalla: se
saca primero el original del corpus local (`fuentes/`, git-ignored) y se abre al lado el de `output/`.

```bash
curl -s "$URL/v1/output/chats.jsonl" | head -1 | jq
curl -s "$URL/v1/output/tables.jsonl" | grep '"clientes:' | head -1 | jq
```

**Diccionario, sin proyectar las formas reales.**

```bash
curl -s "$URL/v1/entities" \
  | jq '{personas: [.entities[] | select(.type == "person")] | length,
         conservadas: [.entities[] | select(.kept == true)] | length,
         ejemplo: (.entities[] | select(.type == "person") | {entity_id, pseudonyms, records: (.records | length)})}'
```

**`/anonymize` en vivo, con un registro de la muestra.** El `author` es una forma real de una clienta ya vista: se
toma del diccionario en el momento (la primera forma del primer `person`, que en la mayoría de los casos es el nombre
completo) y no se escribe en el guion. La respuesta tiene que traer **el mismo** seudónimo.

```bash
NOMBRE=$(curl -s "$URL/v1/entities" | jq -r '[.entities[] | select(.type == "person") | .originals[0]][0]')
curl -s -X POST "$URL/v1/anonymize" -H 'Content-Type: application/json' -d @- <<JSON | jq
{"source": "chats", "records": [{"id": "demo-1", "thread_id": "demo:1", "author": "$NOMBRE",
  "ts": "1785769952.001100", "text": "Hola, me llamo $NOMBRE. Necesito mi certificado.",
  "attachments": []}]}
JSON
```

Para la segunda mitad del gancho —persona nueva, seudónimo nuevo— se repite el `POST` con un nombre que **escriba el
público** en el mismo molde y se compara: otro nombre, otro perfil, misma categoría.

## El argumento, en una frase por dimensión

- **Fuga (45).** La compuerta final es un port de `contiene()` del validador, con la misma normalización que el
  jurado (D5), y barre formas, hashes y claves de objeto; una fuga que no se puede cerrar tumba la corrida en vez de
  entregar el archivo (D11). Los lugares no obvios —metadata de PDF, comentarios HTML, nombres de adjuntos, el
  usuario borrado que solo se identifica por la cédula del texto— están en `DECISIONS.md` §1.1.
- **Utilidad (35).** Se reemplaza el fragmento, no se reescribe el texto: el resto queda byte a byte igual. Se
  conservan ids internos, organizaciones, lugares, marcas, figuras públicas, montos y fechas de negocio
  (`DECISIONS.md` §1.4 y §2), y el seudónimo conserva la categoría y el estilo del original (`DECISIONS.md` §3.2).
- **Consistencia (20).** La identidad sale de evidencia —cédula, correo, `CL-`, transacción, uid, rol en el hilo—,
  nunca del nombre solo: dos personas con el mismo nombre son dos personas. Una persona tiene un solo perfil, y la
  bóveda solo agrega (D4), así que la corrida 2 y la 3 conservan los mismos seudónimos (`DECISIONS.md` §1, D22).

## Lo que no se muestra

- Formas reales en pantalla: `entities.json` es para el jurado y no viaja con el dataset.
- Tokens y claves: el `Authorization` del API de fuentes y `OPENROUTER_API_KEY` van redactados en logs y capturas.
- Cifras de la corrida antes de que existan: si la corrida 1 aún no se lanzó, se dice "pendiente" y se enseña el
  mecanismo (`GET /v1/run/{id}`), no un número.
