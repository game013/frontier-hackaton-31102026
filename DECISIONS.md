# Decisiones: qué es PII, qué se conservó y cuánto costó

Este documento responde las tres preguntas del jurado. Las decisiones numeradas que lo sostienen viven en
`DISEÑO.md` §7 (se citan como D13, D15, etc.); los casos esquina que las prueban, en `CASOS_ESQUINA.md`. Aquí no hay
formas reales: solo mecanismos, decisiones e ids de registro.

## 1. Qué es PII aquí

**Un dato personal es cualquier forma que, en la unión de las tres fuentes, permita señalar a una persona natural
concreta.** No decidimos por el texto de la cadena sino por la persona: un token es PII si pertenece a alguien de
nuestro registro de personas, o si el registro donde aparece lo presenta como una persona. Eso incluye lo que el
enunciado llama PII disfrazada y las combinaciones que individualizan sin nombrar a nadie.

La unidad de decisión es la persona, no la cadena (D2): la misma forma en un chat, en un PDF y en una tabla es la
misma persona; dos personas con el mismo nombre completo son dos personas.

### 1.1 Dónde se busca

La ingesta no deja ningún string sin leer: todo valor de un registro es texto a anonimizar y ninguna clave se cae.
Los lugares obvios son el cuerpo de los documentos y el texto de los mensajes; los que se escapan son estos:

| Fuente | Lugares no obvios                                                                                       |
| ------ | ------------------------------------------------------------------------------------------------------- |
| Chats  | `files[].name` de los adjuntos (nombre, placa, `CC<cédula>`), URLs con usuario, mensajes editados, el handoff del bot y las repeticiones de los agentes que nombran a terceros, y el usuario borrado que solo se identifica por la cédula que da en el texto |
| Docs   | metadata del PDF (`/Author`, `/Title`, `/Subject`, `/Keywords`), comentarios HTML `<!-- -->`, bloques `display:none`, `href="mailto:"` (el correo puede estar **solo** en el href), front matter (author, handles), rutas de imagen, URL de Loom, `filename` y `title` |
| Tablas | `notas`, `descripcion` (MAYÚSCULAS sin tildes, celulares, inicial más apellido), los atributos XML `cedula`, `agente` y `radicado`, `reporta_a`, y los comentarios de la cabecera del XML |

Tres consecuencias de diseño: la metadata del PDF sale con **las mismas claves** que entró, con el valor
seudonimizado; los comentarios HTML y los bloques ocultos se conservan en el `body` y se anonimizan como cualquier
otro texto; y el nombre de un adjunto se anonimiza como texto, no se descarta.

### 1.2 PII disfrazada

Estos casos no se detectan por palabra clave y por eso tienen su propio juez (`CASOS_ESQUINA.md`, CE-09 a CE-11,
CE-16, CE-21, CE-22):

- **El dato "de ejemplo" es real.** La cédula del manual y el celular del memo a inversionistas están en el corpus.
  Se reemplazan porque son formas conocidas del registro, no porque haya una palabra clave delante.
- **Respuestas de solo dígitos.** Un mensaje que es únicamente un número se clasifica por la pregunta anterior del
  hilo, no por su forma.
- **Acentos borrados.** Hay decenas de mensajes con letras acentuadas perdidas ("cdula", sin la ú). La detección
  corre sobre el espacio normalizado del jurado (D5), así que no depende del deletreo de la palabra clave.
- **Formas derivadas de una persona.** MAYÚSCULAS sin tildes, correo deletreado con "arroba" y "punto", slug,
  usuario, cédula con y sin puntos, celular con y sin separadores, placa con y sin guion: todas se guardan como
  variantes de la persona y todas se barren.
- **Personas fuera del maestro.** Una persona con radicado y usuario de chat pero sin fila en `clientes.csv`, y los
  terceros que solo aparecen en notas y mensajes, siguen siendo personas: se crean con llave de contexto y se
  persisten.

### 1.3 Cuasi-identificadores

Ciudad, barrio, ocupación y año **no son PII por sí solos**. Lo son cuando coinciden sobre una sola persona: en
`clientes.csv` cada combinación de las 149 filas es única, y el jurado evalúa la unión de fuentes, no cada fuente por
separado. No se borran, se generalizan:

- **Tablas:** en la fila del maestro (`clientes`), `ocupacion` → sector y `barrio` → otro barrio de la misma ciudad.
  `empleados.cargo` no se toca: es el rol del empleado dentro de la organización, que el enunciado manda conservar,
  no un cuasi-identificador suelto que lo señale fuera de ella.
- **Texto:** cuando ocupación, barrio y año aparecen ligados a una persona, la ocupación va a sector, el barrio a
  ciudad y el año de la relación a "hace unos años". Se aplica también a la fila del maestro, porque romperlo solo en
  el chat deja la combinación viva en la tabla. El año exige el ancla de la relación («llevo con ustedes desde»,
  «cliente desde»): un "desde" suelto es una fecha de negocio y se conserva (§1.4).

### 1.4 Lo que no es PII, y por eso no se toca

| Categoría                          | Por qué no es dato personal                                                                                        |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Ids internos `CL-`, `TX-`, `PQR-`, `KP-`, `INC-`, `S-` | Son claves de registro: unen filas entre tablas y sostienen los conteos por persona. Una vez que el maestro está seudonimizado, el id no apunta a nadie real; quitarlo o cambiarlo rompe los joins y la estructura |
| Organizaciones                     | Personas jurídicas: bancos, aseguradoras, comercios, universidades, medios                                          |
| Lugares                            | Barrios, parques, estaciones, centros comerciales, ciudades                                                        |
| Marcas                             | Nombres comerciales, incluido el de Kipu Pay y el bot Kipi                                                         |
| Figuras públicas                   | Artistas, deportistas, escritores: no son personas del corpus ni clientes                                          |
| Montos y fechas de negocio         | Valores de una transacción, un radicado, un vencimiento o una vinculación: son el contenido que el comprador entrena |
| Dominios de correo                 | El corporativo se mantiene y el público se toma de un pool conocido (`DISEÑO.md` §3, Supuesto 3)                    |

## 2. Qué se conservó y por qué

Conservar es una decisión con precio en las dos direcciones. La rúbrica paga 9 puntos por falsos positivos intactos
y 10 por parecido del texto no personal; borrar de más cuesta utilidad y estructura.

| Se conserva                         | Por qué                                                                                  | Qué se arriesga si se conserva de más                          |
| ----------------------------------- | ---------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Ids internos                        | Los joins entre tablas unen las mismas filas y cada persona mantiene su número de registros; el validador exige las mismas columnas y claves | Nada: el id no identifica a nadie real una vez seudonimizado el maestro |
| Organizaciones, lugares, marcas, figuras públicas | No son personas naturales y el enunciado pide conservarlas                  | Que el detector de nombres se trague una razón social          |
| Montos y fechas de negocio          | Son la información vendible; desplazarlos no reduce la identificación y sí destruye la utilidad | Nada: no identifican por sí solos                       |
| Ciudad y fecha de vinculación       | La salida ideal de la muestra las deja intactas                                          | Entran al cuasi-identificador solo en combinación (§1.3)       |
| Bot, marca y dominios de correo     | No son personas; el dominio corporativo es parte del formato del correo falso            | Nada                                                          |

### 2.1 La allowlist y sus reglas

La allowlist es **datos, no código**: unas 30 entidades del corpus, cada una con su `reason`, más reglas genéricas para
lo que llegue cuando las fuentes crezcan. Nada se conserva sin un motivo escrito al lado, y cada entidad conservada
tiene su juez de falso positivo (J-FP en `CASOS_ESQUINA.md`). Las reglas:

| Regla                                                   | Cuándo se aplica                                                                 | Guarda contra                                                      |
| ------------------------------------------------------- | -------------------------------------------------------------------------------- | ------------------------------------------------------------------ |
| Entidad listada con `reason`                            | Coincide una forma que no es de persona                                          | Que el detector trate un banco, un comercio o una artista como nombre propio |
| Sufijos societarios (`S.A.S.`, `S.A.`, `Ltda.`, `& Cía.`) | La mayúscula termina en sufijo societario                                       | Leer una razón social como nombre y apellido                       |
| "Banco …", "Seguros …", "Fundación …"                   | El nombre empieza por un término genérico de organización                        | Clasificar la entidad por su segunda palabra                       |
| Apellido que también es lugar o marca                   | **Persona** si va precedido de un nombre de pila, si coincide con una forma conocida de una persona o si el registro lo ata a ella; **lugar o marca** si aparece solo nombrando el lugar o la marca | El doble uso del mismo token en la misma frase                     |
| Nombre común que es palabra                             | Solo se reemplaza si el hilo o el documento lo ata a una persona conocida         | Falsos positivos en masa (la palabra aparece decenas de veces como nombre de operador) |
| Mayúscula sin nombre de pila delante                    | Una palabra capitalizada suelta, sin disparador, sin correo y sin ancla no se trata como nombre completo: se conserva | Razones sociales nuevas que llegan con el crecimiento              |

Dos guardas mantienen esas reglas honestas. La primera: el **registro de personas manda** sobre cualquier regla de
conservación; si una forma conocida coincide, gana la persona (D25). La segunda: la regla de la mayúscula suelta no
es una amnistía; un nombre que nadie propone se fuga sin que nadie lo vea, así que los cuatro generadores de
candidatos (D30) siguen proponiendo, incluidos los que aparecen en minúsculas, los que siguen a un disparador y los
nombres extranjeros.

### 2.2 El costo de conservar de más frente a conservar de menos

- **Conservar de más es fuga, y la fuga pesa más.** Una forma real que sobrevive se cobra en cada registro de esa
  persona (35 puntos de fuga puntual) y otra vez en el barrido global (10). Además es el único criterio que decide si
  el trato se cierra: "ninguna persona real expuesta" no admite compensación con utilidad.
- **Conservar de menos es utilidad, y la utilidad se puede recuperar.** Un banco convertido en persona cuesta 9
  puntos de falsos positivos y algo de parecido, y se detecta mirando la salida; una persona que se quedó en el texto
  no se detecta mirando la salida, porque no hay forma de saber que faltaba.

Por eso la asimetría no se resuelve conservando por defecto: se resuelve preguntando. Cada candidato dudoso pasa por
una pregunta cerrada al modelo (§3.2) y el umbral de esa pregunta está calibrado con la muestra y el catálogo (D31).
Cuando la duda es de propiedad de una mención, y no de clasificación, el criterio es explícito: **una fuga cuesta más
que un seudónimo equivocado** (D25).

## 3. Costo y exposición al modelo

### 3.1 Reglas primero

El pipeline es por reglas y el modelo es la excepción (D1, D13). El corpus pesa unos 435 mil tokens y no cabe en una
ventana de contexto; además hay dos relojes de contrato (2 horas por `/run`, 60 segundos por lote de `/anonymize`) y
una exigencia de determinismo: dos corridas seguidas sin modelo deben dar la salida byte a byte igual.

Las reglas hacen el trabajo principal: formas conocidas del registro, patrones (cédula con contexto, celular, correo,
correo deletreado, placa, pasaporte, cuenta, dirección, fechas, URLs con usuario), catálogo de nombres y apellidos,
allowlist, resolución de menciones, reemplazo con estilo, generalización de cuasi-identificadores y compuerta final de
fuga. Reescribir el texto con un modelo está fuera de alcance: pierde parecido y puede inventar o borrar datos de
negocio.

### 3.2 Jev 1.14 solo para el residuo (D13)

Al modelo solo van **preguntas cerradas**, nunca la tarea de generar ni de reescribir texto:

| Pregunta                                              | Respuesta                                | Umbral (D31)                                  |
| ----------------------------------------------------- | ---------------------------------------- | --------------------------------------------- |
| ¿Qué es este candidato?                               | persona / organización / lugar / marca / otro | persona si P(persona) > 0,5                |
| ¿Cuál de estos dos candidatos es? (fichas abstractas: ids, ciudad, radicado, transacción; nunca el nombre real) | A / B / ninguna | desempate aceptado con confianza ≥ 0,7; si no, persona nueva dudosa (D19) |
| ¿Qué género corresponde a este nombre?                | M / F / indeterminado                    | confianza < 0,6 → seudónimo unisex             |
| ¿Cuál es el origen del nombre?                        | nacional / extranjero (D27)              | —                                              |

El modelo es `typesafe/jev-1.14`, con snapshot fijo `typesafe/jev-1.14-20261014` que se reporta en `/v1/health`
(D32): si el alias se moviera, cambiarían decisiones nuevas entre corridas. Descartamos `jev-router` porque reparte
cada llamada a otro modelo y otro proveedor —más exposición y más costo— y descartamos un modelo grande porque solo
volvería si algún caso exigiera extraer texto libre que las reglas no propongan.

En `/anonymize` hay una llamada por mención, con su propio `state`, en paralelo con tope de 16, timeout de 40 s y
respaldo determinista (D28): el modelo nunca se convierte en punto único de falla de la prueba en vivo.

### 3.3 D15 sin maquillaje: el residuo sale de la casa

**Lo declaramos tal cual: el residuo viaja a OpenRouter y desde ahí al proveedor que sirve el modelo.** Residuo es,
por definición, lo que nuestras propias reglas no supieron capturar; por hipótesis contiene datos reales de personas
que nadie más detectó. No hay forma de anonimizar eso sin enviarlo: es exactamente el texto que no sabemos reconocer.

Mitigaciones, en orden de efecto:

1. **Enmascarado de formas conocidas.** Antes de enviar, toda forma del registro se sustituye por `[PERSONA_n]`, sin
   distinguir mayúsculas ni tildes y probando primero las formas más largas. Ni nombres, ni cédulas, ni celulares, ni
   correos, ni direcciones de personas conocidas salen en claro.
2. **Ventana mínima.** Viaja el fragmento y el contexto necesario para responder, nunca el registro completo.
3. **Proveedor sin retención.** Cada petición envía `provider.data_collection: "deny"`.
4. **Caché por hash.** La misma entrada no se envía dos veces; la llave incluye el snapshot del modelo, así que la
   corrida 2 no vuelve a pagar ni a exponer las decisiones de la corrida 1.
5. **Techo de exposición.** Si el modelo no contesta, no se reintenta con más texto: entra el respaldo determinista o
   una persona nueva marcada como dudosa (D19, D28).

**Lo que no está resuelto:** `provider.data_collection: deny` se acepta pero **no está probado que se cumpla**. El
pipeline manda la bandera en cada llamada; no puede verificar desde adentro qué retiene el proveedor, ni hay auditoría
ni garantía contractual que lo respalde. Por eso el enmascarado es incondicional y no depende de la bandera, y por eso
el modelo nunca recibe un registro completo. El riesgo se acepta con mitigación, no se declara cerrado.

### 3.4 Cifras de la corrida

Estas cifras las llena C23 con la corrida 1 ya hecha. Hasta entonces este documento no afirma ningún número de
corrida; los huecos están marcados para que nadie los rellene con una estimación.

| Qué                                        | De dónde sale                                     | Corrida 1 |
| ------------------------------------------ | ------------------------------------------------- | --------- |
| Tokens de entrada al modelo                | `GET /v1/run/{id}` → `cost.tokens_in`             | <!-- C23: tokens de entrada de la corrida --> |
| Tokens de salida del modelo                | `GET /v1/run/{id}` → `cost.tokens_out`            | <!-- C23: tokens de salida de la corrida --> |
| Decisiones pedidas y cuántas salieron de caché | log de la corrida (una línea por job) y la tabla `cache_modelo` | <!-- C23: llamadas al modelo y aciertos de caché --> |
| Costo en dólares                           | log de la corrida, contrastado con tokens por el precio del snapshot | <!-- C23: dólares de la corrida --> |
| Segundos de `/run`                         | `GET /v1/run/{id}` → `cost.seconds`               | <!-- C23: segundos de /run --> |
| Segundos por lote de 50 en `/anonymize`    | `curl -w '%{time_total}'` sobre el lote (token redactado) | <!-- C23: segundos de /anonymize --> |

Los tokens que se reportan son los que devolvió el proveedor en cada llamada: los acumula el cliente de Jev
(`internal/llm`, `Uso`) y viajan con los conteos de la corrida. Una corrida **sin llave** reporta cero tokens y cero
dólares porque no hizo ninguna llamada (D19): ese cero es la cifra, no un hueco.

#### Cómo se llenan las seis cifras

Se corre una vez, se consulta el job y se leen los seis valores. Ningún número de esta tabla se estima.

```bash
# 1. Una corrida (idempotente mientras hay una activa). SOURCES_API_URL y TOKEN
# salen de .env, que no entra al repo.
export URL=https://frontier-31102026-production.up.railway.app
JOB=$(curl -s -X POST "$URL/v1/run" -H 'Content-Type: application/json' \
  -d "{\"sources_url\":\"$SOURCES_API_URL\",\"token\":\"$TOKEN\"}" | jq -r .job_id)

# 2. Se espera a que el contrato termine: done o failed (tope 2 h).
curl -s "$URL/v1/run/$JOB" | jq '{status, records, cost}'
```

| Cifra | Comando |
| ----- | ------- |
| Tokens de entrada | `curl -s "$URL/v1/run/$JOB" \| jq '.cost.tokens_in'` |
| Tokens de salida | `curl -s "$URL/v1/run/$JOB" \| jq '.cost.tokens_out'` |
| Segundos de `/run` | `curl -s "$URL/v1/run/$JOB" \| jq '.cost.seconds'` |
| Llamadas y aciertos de caché | el log del servicio en Railway (`railway logs \| grep "corrida $JOB"`) imprime una línea por corrida con `N llamadas, M de caché`; contraste en el volumen: `sqlite3 /data/almacen.db "SELECT count(*) FROM cache_modelo"` antes y después de la corrida da las decisiones que costaron una llamada |
| Dólares | la misma línea del log trae `US$ x`; se contrasta con `tokens_in × precio_entrada + tokens_out × precio_salida` del snapshot (precio por token en `https://openrouter.ai/api/v1/models`, entrada del id exacto que reporta `GET /v1/health`) |
| Segundos por lote de 50 en `/anonymize` | ver abajo |

El lote son registros ya anonimizados, en el esquema exacto del contrato: mide el tiempo de resolución y de modelo del
caso real, y las personas que ve son seudónimos ya emitidos, que quedan reservados en `personas_vivo` (D24) sin tocar
el registro de `/run` ni `entities.json`.

```bash
# Lote de 50 en el esquema del contrato, tomado de la salida ya anonimizada; se
# mide el tiempo de pared del POST (el contrato da 60 s por lote).
curl -s "$URL/v1/output/chats.jsonl" | head -50 | jq -s '{source: "chats", records: .}' > /tmp/lote50.json
curl -s -o /dev/null -w 'anonymize 50: %{time_total}s\n' -X POST "$URL/v1/anonymize" \
  -H 'Content-Type: application/json' -d @/tmp/lote50.json
```

Lo único medido hasta ahora es del diseño, con datos inventados y no con el corpus: 0,23–0,5 s y del orden de
US$0,00002 por decisión (`DISEÑO.md` §6.3). Sirve para dimensionar el orden de magnitud; no es la corrida.
