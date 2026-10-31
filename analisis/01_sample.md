# Meta-análisis de `sample/`

El dataset de muestra trae 1 hilo de chat (10 mensajes), 1 documento Markdown y 4 filas de `clientes.csv`, con su
salida ideal y 13 tests. Es chico pero fija las **convenciones** que el jurado considera "salida ideal" (10 puntos de
utilidad salen del parecido de texto con ella).

## 1. Reglas de reemplazo que se deducen

### 1.1 Un seudónimo es un **perfil falso completo**, no un nombre suelto

Cada persona recibe una identidad sintética coherente: nombre, correo, celular, cédula, dirección y fecha de
nacimiento. Todas sus formas se derivan de ese perfil.

| Forma original (Manuela, p_057)    | Dónde              | Salida                                     | Regla                                              |
| ---------------------------------- | ------------------ | ------------------------------------------ | -------------------------------------------------- |
| `Manuela Castaño Ruiz`             | tabla              | `Sofía Elena Zambrano Naranjo`             | nombre completo → seudónimo completo (2 apellidos) |
| `Manuela Castaño`                  | chat, doc, notas   | `Sofía Elena Zambrano`                     | nombre(s) + 1er apellido → igual en el seudónimo   |
| `Manuela` (saludo del agente)      | chat               | `Sofía Elena`                              | nombre de pila → nombre(s) de pila del seudónimo   |
| `manuela_castano` (adjunto)        | `files[].name`     | `sofia_elena_zambrano`                     | slug sin tildes, `_`, mismo patrón                 |
| `1.107.360.039`                    | chat               | `1.107.252.808`                            | conserva el formato con puntos                     |
| `1107360039`                       | tabla              | `1107252808`                               | misma longitud, conserva prefijo (`1107`)          |
| `mcastano@gmail.com`               | chat, tabla        | `szambrano@gmail.com`                      | inicial + 1er apellido @ dominio público           |
| `3148729552`                       | tabla              | `3186423850`                               | celular 3xx de 10 dígitos                          |
| `Diagonal 3A # 78-6 apto 1501`     | tabla              | `Diagonal 121B # 96-28`                    | dirección colombiana sintética                     |
| `1997-07-23`                       | tabla              | `1998-11-11`                               | fecha cercana (±2 años)                            |

El `entity.pseudonyms.name` es la forma corta (`Sofía Elena Zambrano`); la tabla usa la forma larga.

### 1.2 Formatos de cada seudónimo

- **Nombre:** mismo género y región. El seudónimo puede tener 1 o 2 nombres de pila, independiente del original
  (Manuela → Sofía Elena; Esteban Darío → Hugo).
- **Correo de cliente:** `inicial + primer apellido` en minúscula y sin tildes. El dominio **no siempre se conserva**
  (gmail → gmail, gmail → outlook, gmail → yahoo, gmail → icloud): se toma de un pool de dominios públicos.
- **Correo corporativo:** `nombre.apellido@kipupay.co` con el seudónimo (`kelly.castano@` → `doris.lozano@`).
  El dominio de la empresa se conserva.
- **Cédula:** misma longitud y mismo prefijo de 2 a 4 dígitos (32390356 → 43062949 es la excepción: cambia el prefijo
  pero sigue en un rango plausible de cédula de mujer). Formato con puntos si el original los tenía.
- **Celular:** 10 dígitos, empieza por 3, otro operador.
- **Dirección:** tipo de vía colombiano (Calle, Cl, Cra, Diagonal, Tv, Transversal) con formato `# NN-NN` y
  complemento opcional (apto, of, casa).
- **Fecha de nacimiento:** otra fecha plausible, misma década aproximada.

### 1.3 Cuasi-identificadores: se **generalizan**, no se borran

| Atributo           | En la tabla                                          | En el chat                     |
| ------------------ | ---------------------------------------------------- | ------------------------------ |
| Ocupación          | Generalizada a sector en **todas** las filas         | Igual: `fotógrafa` → `profesional de las industrias creativas` |
| Barrio             | **Reemplazado por otro barrio de la misma ciudad** (Granada → Santa Mónica) | Generalizado a la ciudad (`Granada` → `Cali`) |
| Año de vinculación | `fecha_vinculacion` **intacta**                      | `2025` → `hace unos años`      |
| Ciudad             | Intacta                                              | Intacta                        |

Mapeo ocupación → sector visto: fotógrafa → profesional de las industrias creativas; community manager →
profesional de medios; administradora → profesional de servicios; mecánico → profesional del transporte. El sector es
neutro en género.

**El reemplazo es literal, sin reescribir la frase.** La salida ideal dice "entré con ustedes en hace unos años": se
cambia el fragmento y se deja la gramática rota. Esto confirma que el parecido se mide con reemplazo de fragmentos y
que un LLM que reescribe pierde puntos.

### 1.4 Qué se conserva

- **Ids internos:** `CL-88627`, `CL-19127` siguen igual en tabla, notas y documento (estructura y joins).
- **Organizaciones y reguladores:** Bancolombia, Superfinanciera, PSE, Kipu Pay.
- **El bot `Kipi`:** se conserva como autor y no figura en `entities.json`.
- **Ciudades, fechas de negocio, montos, conteos:** intactos (`1.240 pagos`, `38 reversiones`, `28 de julio`,
  `15 de agosto`, `fecha: 2026-08-01`).
- **Texto vacío:** `notas` vacío sigue como `""`.

## 2. Resolución de identidad

- **Autor del chat:** el `author` es el nombre de la persona derivado de `users.json`, en forma corta
  (nombre(s) + 1er apellido). Yesenia Ramírez Zapata → `Marcela Pardo`.
- **Agente nombrada por su nombre de pila:** "soy Yesenia de soporte" → "soy Marcela de soporte".
- **Tercero mencionado en el chat** ("Nohora Salazar") **se vincula al maestro** (`CL-19127`) y recibe el mismo perfil
  que su fila. El vínculo sale de las notas de la tabla ("Familiar de Manuela Castaño (CL-88627)") y de la tabla del
  memo ("Nohora Salazar (CL-19127)").
- **Empleada que solo aparece en un documento** (Kelly Castaño, autora del memo) es otra entidad (`p_016`) con
  seudónimo propio, aunque comparta apellido con dos clientas.
- **Apellido compartido no implica persona compartida.** "Castaño" aparece en 4 personas de la muestra (Manuela,
  Alejandra, Esteban, Kelly) y en `users.json` hay 8 personas más con Castaño o Salazar. Cada una recibe un apellido
  falso distinto. Nunca se mapea apellido → apellido.
- **`records` de una entidad:** todo registro donde aparece cualquier forma suya, incluidos los mensajes que escribió
  (vía `author`), los que la nombran (el saludo "Hola Manuela" del agente cuenta para Manuela) y las filas de otras
  personas cuyas notas la mencionan (`clientes:CL-19127` está en los records de Manuela).

## 3. `entities.json`: lo que espera el jurado

```json
{ "entity_id": "p_057", "type": "person",
  "originals": ["1.107.360.039", "1107360039", "1997-07-23", "3148729552", "Diagonal 3A # 78-6 apto 1501",
                "Manuela", "Manuela Castaño", "Manuela Castaño Ruiz", "manuela_castano", "mcastano@gmail.com"],
  "pseudonyms": { "name": "Sofía Elena Zambrano", "email": "szambrano@gmail.com", "phone": "3186423850",
                  "cedula": "1107252808", "birth_iso": "1998-11-11", "address": "Diagonal 121B # 96-28" },
  "records": ["…ids de chats…", "clientes:CL-19127", "clientes:CL-88627", "sdoc_01"] }
```

- **`originals` = todas las formas que aparecen en los registros de entrada**, incluidas variantes de superficie
  (con y sin puntos, nombre de pila suelto, slug de archivo). No incluye datos que solo están en `users.json`: para
  Yesenia solo aparecen `Yesenia` y `Yesenia Ramírez`, no su correo ni su celular.
- **`pseudonyms`** declara solo los tipos que se usaron (Yesenia solo tiene `name`).
- **Entidades conservadas:** `kept: true`, `type: organization_or_place`, `pseudonyms: {}`, `records: []` y `reason`.
- Los `entity_id` de la muestra (`p_008`, `p_016`, `p_057`) son de un catálogo global del jurado; no hace falta
  replicarlos, pero sí que sean estables entre corridas.

## 4. Los tests: qué mide cada familia

| Familia          | Mecanismos vistos (sample + públicos)                                                     | Cómo se verifica                                                                 |
| ---------------- | ----------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `directa`        | cedula_con_puntos, cedula_explicita_con_puntos, correo, nombre_completo_presentacion, tercero_mencionado, autora_del_memo, tercero_en_nota | El payload no aparece en el registro **ni en ningún otro**, ni como hash sin sal |
| `oculta`         | nombre_en_filename_adjunto, frontmatter_author_email                                      | El payload no aparece en el campo de metadata, filename, adjunto o markup        |
| `indirecta`      | ocupacion_barrio_anio                                                                     | Los componentes no co-ocurren todos en el registro **ni en `join_records`** (la fila del maestro) |
| `falso_positivo` | banco_en_contexto, entidad_reguladora, billetera_digital (Nequi)                          | El payload sigue presente, sin modificar                                         |
| `homonimos`      | dos_filas_mismo_nombre_distinta_ciudad                                                    | Las dos personas reciben seudónimos distintos                                    |
| `consistencia`   | misma_persona_mismo_token                                                                 | Con `lookup_forms` encuentra la entidad; un solo seudónimo presente en todos sus registros |

Pesos: directa 3, oculta 2, el resto 1. Los tests de consistencia traen `forms` con las claves `full`, `full2`,
`first`, `cedula`, `cedula_dots`, `email`, `phone`, `address`, `birth_iso` y `filename_token`. Esa es la lista de
variantes que el pipeline debe generar y declarar.

**Consecuencia para consistencia:** el jurado busca la entidad por `lookup_forms` (correo, cédula, cédula con puntos,
celular, nombre completo largo y corto) dentro de nuestro `entities.json`. Si falta una de esas formas en
`originals`, el test no encuentra a la persona o la encuentra partida en dos.

## 5. Decisiones que esto fija para el diseño

1. **Unidad de seudonimización = persona con perfil sintético completo**, guardado y estable entre corridas.
2. **Motor de reemplazo de fragmentos** con un renderizador por tipo de forma (completo, corto, pila, slug, correo,
   cédula con o sin puntos, celular con o sin separadores) que respete el estilo del original (mayúsculas, sin tildes).
3. **Diccionario de variantes por persona** a partir del maestro: se usa para detectar y luego se vuelca a
   `originals`.
4. **Ocupación generalizada a sector en todas las filas** y **barrio reemplazado** en tablas. En el texto: barrio →
   ciudad, año → "hace unos años", ocupación → sector.
5. **Los `records` incluyen menciones indirectas** (saludos con el nombre de pila, notas de terceros).
6. **No reescribir con LLM:** solo reemplazo literal de fragmentos.

## 6. Preguntas abiertas para el corpus real

- ¿El `title` de documentos sale de otro lado? En la muestra la salida dice "Memo cierre de julio" y el front matter
  "Memo cierre de julio 2026".
- ¿Cómo se parte un nombre de 3 palabras? "Yesenia Ramírez Zapata" es 1 nombre + 2 apellidos, pero el seudónimo
  "Sofía Elena Zambrano" es 2 nombres + 1 apellido. Hipótesis: en el maestro y en `users.json` siempre hay
  2 apellidos, así que la forma corta es "quitar la última palabra".
- ¿El barrio sintético de la tabla se elige solo entre barrios de la misma ciudad? En la muestra, el barrio falso de
  Hugo Cano (Granada) es el barrio real de Manuela: el barrio solo no se trata como PII.
