# Diseño: Vende los datos sin vender a la gente

Versión inicial · 2026-10-31 · Reto Frontier, Kipu Pay

## 1. El problema en una frase

Entregar el mismo corpus de soporte (chats, documentos y tablas) **con cada persona real sustituida por una persona
falsa coherente**, la misma en todas las fuentes y estable cuando el corpus crece, sin tocar el contenido de negocio.

El jurado es un programa que puntúa fuga (45), utilidad (35) y consistencia (20), y además prueba en vivo con
registros que no hemos visto.

## 2. Qué tiene que resolver el sistema

| Requisito                                    | Origen          | Consecuencia de diseño                                     |
| -------------------------------------------- | --------------- | ---------------------------------------------------------- |
| Ningún dato real en ninguna parte de la salida (texto, metadata, nombres de archivo, claves) | Fuga 35 + 10 | Barrido final con la misma normalización que el jurado |
| Mismo seudónimo para la misma persona en todas las fuentes | Consistencia 8 | La unidad es la **persona**, no la cadena de texto |
| Dos personas con el mismo nombre reciben seudónimos distintos | Homónimos 5 | Identidad por evidencia (ids, contexto), nunca solo por el nombre |
| El seudónimo conserva categoría y formato      | Utilidad 8      | Perfil sintético completo por persona                      |
| El texto no personal no cambia                 | Utilidad 10     | Reemplazo literal de fragmentos, sin reescribir            |
| Bancos, marcas, lugares y figuras públicas se quedan | Utilidad 9 | Allowlist con reglas de contexto                         |
| Llaves, conteos por persona y quién habla con quién se mantienen | Utilidad 8 | Los ids internos no se tocan; el autor se seudonimiza como cualquier mención |
| "Reporta a" entre empleados                    | Consistencia 4  | `reporta_a` → seudónimo corto del jefe                     |
| `entities.json` honesto                        | Consistencia 3  | Se declaran todas las formas reales encontradas            |
| Las fuentes crecen sin aviso; los seudónimos viejos no cambian | Multiplica consistencia | Bóveda persistente de solo agregar |
| `/run` < 2 h; `/anonymize` < 60 s por lote de 50 | Contrato       | Reglas primero y LLM solo para el residuo, con caché      |

## 3. Supuestos

1. **El maestro es la verdad de identidad.** `clientes.csv` (149) y `empleados.xml` (36) anclan a las personas.
   `users.json` se une al maestro por `users.name` = parte local del correo (148 de 149) y por el correo
   `@kipupay.co` para empleados.
2. **Los ids internos no son PII.** `CL-`, `TX-`, `PQR-`, `KP-`, `INC-` y `S-` se conservan: son lo que une las
   tablas, y la salida ideal de la muestra los deja intactos.
3. **Los dominios de correo públicos y `kipupay.co` no son PII.** El dominio del correo falso se toma de un pool
   público; el corporativo se mantiene.
4. **El bot Kipi y la marca Kipu Pay no son personas.**
5. **Las fechas de negocio, los montos, las ciudades y las fechas de vinculación se conservan.** La muestra lo hace así.
6. **El reemplazo es literal.** La salida ideal deja "entré con ustedes en hace unos años": se cambia el fragmento y
   no se arregla la gramática.
7. **La salida ideal es la referencia de estilo.** Las reglas de formato (seudónimo corto o largo, slug, correo
   `inicial+apellido`, ocupación → sector, barrio → otro de la misma ciudad) se copian de `sample/expected_output`.

## 4. Casos difíciles (con nombre propio)

| Caso                                                     | Ejemplo del corpus                                            | Cómo se resuelve                                         |
| -------------------------------------------------------- | ------------------------------------------------------------- | -------------------------------------------------------- |
| Homónimos cliente–cliente                                | 2 Camila Arango, 2 Daniela Ordóñez, 2 Juliana Mejía, 2 Mateo Henao | Evidencia en otra fuente: radicado PQR, fecha + monto en transacciones, cédula, placa, adjunto |
| Agente que se llama como un cliente, en el mismo hilo     | Agente Andrés Gómez y cliente Andrés Gómez Giraldo            | Primero uid y rol del autor; luego "soy X" o el handoff del bot |
| MAYÚSCULAS sin tildes                                     | `BRAYAN GOMEZ VARGAS` en transacciones                        | Búsqueda en espacio normalizado (sin tildes, minúsculas) |
| Correo deletreado                                         | "juan punto perez arroba gmail punto com"                     | Detector propio; el reemplazo se emite en el mismo estilo |
| PII fuera del texto visible                               | Author/Keywords del PDF, comentarios HTML, `display:none`, `mailto:`, front matter, rutas de imagen, URLs de Loom, `files[].name`, `filename` | La ingesta lleva todo a campos de texto que se anonimizan igual que el cuerpo |
| PII disfrazada                                            | Cédula "de ejemplo" del manual (es real), celular del CEO en el memo a inversionistas | Las formas conocidas del registro se barren en todo el texto, sin depender de palabras clave |
| Cuasi-identificador sin nombre                            | "contadora, Barranquilla, desde 2024" (CL-50124)              | Generalización en el texto **y** en la fila del maestro |
| Apellido que es lugar o marca                             | "PSE de Luis Santander desde Santander", "Mariana Carulla … compra en Carulla" | Persona si va precedido de un nombre de pila o coincide con una forma conocida; si no, se conserva |
| Nombre común que es palabra                               | "Claro" (39 veces) frente a la empleada Clara                 | El nombre de pila suelto solo se reemplaza si el hilo o documento lo ata a una persona |
| Respuestas de solo dígitos                                | "1022649507" después de "¿me confirmas tu cédula?"            | Se clasifican con la pregunta anterior del hilo          |
| Usuario borrado                                           | U07HF5JVK sin `real_name`                                     | Se identifica por la cédula que da en el texto (CL-58402) |
| Persona fuera del maestro                                 | CL-55481 (Felipe Castilla): PQR y usuario de chat             | Entidad creada a partir de la evidencia, con llave `CL-55481` |
| Terceros sin fila                                         | Hernando Gómez, Kelly Marulanda, Pamela Agudelo, Jesús Sánchez | Entidad nueva con llave de contexto, persistida         |
| Documentos en inglés                                      | Security report, contrato Veloz, memo a inversionistas        | Mismos detectores más los marcadores "Prepared by", "ID No."; el LLM cubre el residuo |

## 5. Qué dejamos fuera

| Fuera de alcance                              | Por qué                                                          |
| --------------------------------------------- | ---------------------------------------------------------------- |
| Reescribir o "pulir" el texto con un LLM      | Pierde parecido con la salida ideal y puede inventar datos       |
| Arreglar la gramática tras generalizar        | La salida ideal no lo hace                                       |
| OCR de imágenes                               | No hay imágenes con texto en el corpus                           |
| Desplazar fechas o montos de negocio          | Destruye utilidad; no son PII por sí solos                       |
| k-anonimato formal con umbral                 | Con 149 filas, cada combinación es única (k=1); se generaliza siempre, como en la muestra |
| Traducir los documentos en inglés             | No lo pide el reto                                               |

## 6. Arquitectura

```mermaid
flowchart TD
    API[(API de fuentes)] -->|ETag| ING[1 · Ingesta y normalización<br/>chats · docs · tablas → registros canónicos]
    ING --> REG[2 · Registro de personas<br/>maestro + empleados + users.json<br/>variantes de cada forma]
    ING --> DET[3 · Detección de menciones<br/>a. formas conocidas<br/>b. patrones<br/>c. nombres desconocidos + LLM<br/>d. allowlist]
    REG --> DET
    DET --> RES[4 · Resolución mención → persona<br/>ids duros > hilo/rol > LLM desempate]
    RES --> BOV[(5 · Bóveda SQLite<br/>persona → perfil falso<br/>solo agrega)]
    BOV --> REP[6 · Reemplazo con estilo]
    RES --> REP
    REP --> GEN[7 · Generalización de<br/>cuasi-identificadores]
    GEN --> GATE[8 · Compuerta de fuga<br/>port de contiene del validador]
    GATE --> OUT[(output/<br/>chats · docs · tables · entities)]
    LIVE[POST /anonymize] --> DET
    GATE --> LIVE
```

**Un solo núcleo, dos entradas.** `/run` y `/anonymize` llaman la misma función `Anonymize(records)`. La ingesta
solo traduce los formatos crudos al esquema normalizado, que es exactamente el que recibe `/anonymize`.

### 6.1 Componentes

**1. Ingesta.** Convierte cada fuente al esquema de `output/`.
- Chats: `author` = nombre corto del usuario (nombre(s) + 1er apellido), `thread_id` = `canal:thread_ts` (o el `ts`
  propio si es la raíz), `attachments` = `files[].name`.
- Documentos: MD tal cual; HTML → markdown conservando comentarios y bloques ocultos; PDF → markdown con
  encabezados por tamaño de letra y tablas por celdas. La metadata conserva las mismas claves que la entrada.
- Tablas: `clientes.csv` se decodifica desde Latin-1; los atributos XML (`radicado`, `cedula`, `agente`) son columnas;
  todo como texto.

**2. Registro de personas.** Una entrada por persona con llave estable (`p_cl_<num>`, `p_kp_<num>`,
`p_t_<hash>`), género, rol (cliente o empleado) y **todas sus variantes**:
nombre completo, corto, de pila, MAYÚSCULAS sin tildes, slug (`manuela_castano`), usuario (`mcastano`), correo,
correo deletreado, cédula con y sin puntos, `CC<num>`, celular pegado / `+57 3xx xxx xxxx` / con guiones,
dirección, fecha de nacimiento en ISO, en palabras y en formato US, placa con y sin guion, pasaporte.

**3. Detección.** Toda la búsqueda corre sobre un **espacio normalizado idéntico al del jurado** (NFKC, confusables,
sin tildes, minúsculas, dígitos sin separadores) con un mapa de offsets para volver al texto original.
- a. **Formas conocidas** (Aho-Corasick sobre las variantes del registro): la capa principal.
- b. **Patrones**: cédula (con contexto, para no atrapar montos ni el NIT), celular, correo, correo deletreado,
  placa, pasaporte, cuenta, dirección, fechas, URLs con usuario, tokens en nombres de archivo.
- c. **Nombres desconocidos**: diccionario de nombres y apellidos colombianos más mayúscula inicial. Lo que queda
  dudoso va a **Claude Haiku 5.5**, que devuelve fragmentos y tipo, nunca texto reescrito.
- d. **Allowlist** de entidades a conservar (≈30 vistas), con la regla de apellido-lugar.

**4. Resolución.** Para cada mención, la evidencia se ordena por fuerza:
1. Identificador duro: cédula, correo, celular, `CL-`, `PQR-` → `pqrs.xml`, placa, fecha + monto → `transacciones`.
2. Contexto: autor del mensaje (uid), participantes del hilo, rol ("soy X", handoff del bot), cliente del hilo.
3. Nombre: solo si hay un único candidato compatible.
4. LLM: solo para desempatar entre candidatos, con el contexto del registro.
Si nada vincula, se crea una persona nueva con llave de contexto, que se persiste.

**5. Fábrica de seudónimos y bóveda.**
- Perfil completo por persona: nombre del mismo género (1 o 2 nombres de pila + 2 apellidos), correo
  `inicial+apellido@<dominio público>` o `nombre.apellido@kipupay.co`, cédula de la misma longitud y prefijo,
  celular 3xx, dirección colombiana, fecha de nacimiento cercana, placa, pasaporte, barrio de la misma ciudad.
- Semilla `HMAC(secreto, llave estable)`.
- Restricciones: seudónimo único y **sin coincidencias con ninguna forma real** del registro (tampoco números de
  otra persona).
- La bóveda es SQLite en el volumen de Railway, **solo agrega**, y se exporta como respaldo al repo.

**6. Reemplazo con estilo.** Los fragmentos se reemplazan de derecha a izquierda con la variante equivalente del
perfil, respetando mayúsculas, tildes, puntos, separadores, "arroba/punto" y slug. Recorre todos los strings del
registro: texto, nombres de archivo, metadata, front matter, comentarios.

**7. Cuasi-identificadores.**
- Tablas: ocupación → sector (13 sectores) en todas las filas; barrio → otro barrio de la misma ciudad.
- Texto: cuando ocupación, barrio y año aparecen ligados a una persona → ocupación a sector, barrio a ciudad,
  año a "hace unos años". Cubre la plantilla "soy X, vivo en Y, desde Z" y las descripciones de los agentes.

**8. Compuerta de fuga.** Port de `contiene()` de `validar.mjs`. Barre todas las formas originales del registro sobre
toda la salida (incluidas claves de objetos) y busca también hashes sin sal (MD5/SHA-1/SHA-256) de las formas. Si
encuentra algo, lo reemplaza y lo registra. La corrida termina con 0 fugas conocidas o falla ruidosamente en el log.

### 6.2 Servicio

| Endpoint             | Implementación                                                                    |
| -------------------- | --------------------------------------------------------------------------------- |
| `GET /v1/health`     | `{status, team, version, models: ["anthropic/claude-haiku-5.5 vía OpenRouter"]}`                           |
| `POST /v1/run`       | Crea un job en SQLite y lo corre en una goroutine; idempotente si ya hay uno corriendo |
| `GET /v1/run/{id}`   | Estado, conteos y costo (tokens de entrada y salida, segundos)                    |
| `GET /v1/output/*`   | Sirve los archivos de la última corrida terminada desde el volumen                |
| `POST /v1/anonymize` | Mismo núcleo, carga el registro y la bóveda; persiste personas nuevas; timeout interno de 45 s con respaldo sin LLM |
| `GET /v1/entities`   | `entities.json` de la última corrida                                               |

### 6.3 Stack

- **Go** (net/http con chi), **SQLite** con `modernc.org/sqlite` (sin cgo, imagen simple).
- **LLM:** Claude Haiku 5.5 vía **OpenRouter** (`OPENROUTER_API_KEY`), API compatible con OpenAI llamada con un
  cliente HTTP propio. Modelo configurable por `LLM_MODEL`; caché por hash de entrada en SQLite.
- **PDF:** las librerías de Go pierden espacios en celdas partidas. Decisión: un ayudante en Python con pdfplumber
  invocado como subproceso solo para extraer texto y metadata (ver D7).
- **Railway** con Dockerfile y volumen montado en `/data` (bóveda, caché LLM, output).

## 7. Decisiones y alternativas descartadas

| #  | Decisión                                                  | Alternativa descartada                          | Por qué                                                                                       |
| -- | --------------------------------------------------------- | ----------------------------------------------- | --------------------------------------------------------------------------------------------- |
| D1 | Reglas y registro primero; LLM solo para el residuo       | NER con LLM sobre todo el corpus                | ~435 mil tokens de costo, no determinista, riesgo de reescritura; por reglas ya se llega a ~88 |
| D2 | La unidad de seudonimización es la persona                | Mapear cadena → cadena                          | Una cadena no distingue homónimos y fusiona personas (70 puntos, según el enunciado)          |
| D3 | Perfil falso completo y coherente                         | Seudónimo independiente por campo               | Un correo que no corresponde al nombre aleja de la salida ideal y rompe la coherencia        |
| D4 | Bóveda persistente + semilla determinista                 | Solo HMAC determinista                          | Resolver colisiones depende del orden; con fuentes nuevas podría cambiar a una persona vieja  |
| D5 | Búsqueda en el espacio normalizado del jurado             | Regex sobre texto crudo                         | Se escapan MAYÚSCULAS sin tildes, confusables y dígitos con separadores                       |
| D6 | Reemplazo literal de fragmentos                           | Redactar con `[REDACTED]` o reescribir con LLM  | Redactar da 56 puntos; reescribir pierde parecido y puede borrar datos de negocio             |
| D7 | Go para el servicio + pdfplumber como subproceso          | Todo en Go (ledongthuc/pdf, pdfcpu) / todo en Python | Go es el stack fuerte del equipo; pdfplumber fue la única librería que extrajo bien tablas y celdas partidas en el análisis |
| D8 | Un solo núcleo para `/run` y `/anonymize`                 | Dos caminos separados                           | Garantiza el mismo seudónimo en `output/` y en vivo                                           |
| D9 | Ocupación y barrio se generalizan en todas las filas      | Solo donde k < umbral                           | k=1 en las 149 filas; la salida ideal generaliza todo; el test evalúa la unión de fuentes     |
| D10| Homónimos se desempatan con evidencia de otra fuente      | Por ciudad o cargo junto al nombre              | El enunciado dice que no aparecen; los 8 pares tienen desempate en PQR, transacciones o adjuntos |
| D11| Compuerta de fuga obligatoria al final                    | Confiar en la detección                         | Un falso negativo es fuga en cada registro de la persona; el barrido es barato               |
| D12| SQLite en volumen                                          | Postgres gestionado / archivos JSON             | Un solo proceso, sin red extra; JSON no da transacciones si `/anonymize` y `/run` coinciden |
| D13| Haiku 5.5 vía OpenRouter para el residuo                  | Sonnet / Opus                                   | La tarea es extraer fragmentos con contexto corto; Haiku basta y cabe en el límite de 60 s; OpenRouter es la llave disponible |

## 8. Plan para medir

Todo corre en local antes de gastar una corrida oficial.

| # | Prueba                              | Cómo                                                                                  | Meta                       |
| - | ----------------------------------- | ------------------------------------------------------------------------------------- | -------------------------- |
| 1 | Forma de la salida                  | `node docs/validar.mjs output/ --public fuentes/`                                     | OK                         |
| 2 | Muestra                             | Correr el pipeline sobre `sample/input` y aplicar los 13 tests de `tests_sample.json` | 13/13                      |
| 3 | Parecido con la salida ideal        | Similitud de texto por registro contra `sample/expected_output`                        | ≥ 0,95 de media            |
| 4 | Barrido de fuga                     | Todas las formas del registro sobre toda la salida, con `contiene()` portado           | 0 coincidencias            |
| 5 | Set dorado                          | ~60 casos a mano: 8 pares de homónimos, 29 cuasi-identificadores, 8 correos deletreados, adjuntos, usuario borrado, PII disfrazada, apellidos-lugar | 100 % |
| 6 | Falsos positivos                    | Cada entidad de la allowlist sigue en sus registros                                    | 100 %                      |
| 7 | Consistencia                        | Un seudónimo por persona; unicidad; `reporta_a` resuelve al jefe correcto; los joins entre tablas unen las mismas filas | 100 % |
| 8 | Honestidad del diccionario          | Para cada persona, todas las `lookup_forms` posibles (estilo `forms` de los tests) están en `originals` | 100 % |
| 9 | Crecimiento                         | Correr con el 80 % de las fechas y luego con el 100 %; comparar `entities.json`       | 0 seudónimos viejos cambian |
| 10| Carga en vivo                       | Lotes de 50 registros, incluidas copias alteradas y homónimos nuevos                   | < 60 s, mismo seudónimo    |
| 11| Tiempo y costo de `/run`            | Medido en Railway                                                                      | < 15 min, costo reportado  |

### Uso de las tres corridas oficiales

1. **Corrida 1, pronto**, con el pipeline por reglas cuando las pruebas 1–4 estén en verde. Asegura el 30 % antes del
   cambio de fuentes.
2. Un monitor consulta `record_index` con `If-None-Match` para detectar el crecimiento.
3. **Corridas 2 y 3 después del cambio**: una para medir y la última, que cubre el 100 % de los tests ocultos, para
   cerrar.

## 9. Riesgos

| Riesgo                                            | Mitigación                                                              |
| ------------------------------------------------- | ----------------------------------------------------------------------- |
| El cambio de fuentes llega antes de la corrida 1  | Priorizar una versión por reglas desplegable lo antes posible           |
| El volumen de Railway se pierde y cambian los seudónimos | Exportar la bóveda al repo tras cada corrida; semilla determinista como respaldo |
| El LLM falla o tarda en `/anonymize`              | Timeout de 45 s y respaldo por reglas                                   |
| Falsos positivos por nombres comunes ("Claro")    | Nombre de pila suelto solo con contexto; set dorado de falsos positivos |
| Markdown de PDF distinto al de la salida ideal    | Solo afecta el parecido; se ajusta con lo que muestre la corrida 1     |
