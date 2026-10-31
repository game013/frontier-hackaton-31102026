# 02 · Meta-análisis de los chats reales (`fuentes/chats/`)

Corpus: 6 canales, 2.028 mensajes, 320 hilos, 187 usuarios. Los ids se citan por los primeros 8 caracteres del `client_msg_id` (son UUID únicos; 0 duplicados; coinciden 1:1 con `record_index.chats`).
Scripts en `/private/tmp/claude-501/-Users-ozgaravito-Documents-Personal-Trabajo-Frontier-31102026/23f74c6f-eaca-48b8-b54b-27735c5647f1/scratchpad/chats/` (`s1.py`…`s9.py`, volcado legible por hilo en `all.txt`).

## 1. Estructura

| Canal | Mensajes | Hilos |
|---|---|---|
| soporte-tarjetas | 720 | 112 |
| soporte-cobros | 704 | 119 |
| soporte-onboarding | 531 | 82 |
| general | 29 | 3 |
| escalamientos | 22 | 2 |
| fraude-interno | 22 | 2 |

| Campo | Presente en | Nota |
|---|---|---|
| `type, user, text, ts, client_msg_id, team` | 2.028 | `type` siempre `message` |
| `thread_ts`, `parent_user_id` | 1.708 | `parent_user_id` siempre `null` |
| sin `thread_ts` (raíz) | 320 | = nº de hilos; raíz usa su propio `ts` |
| `subtype: bot_message` + `bot_id` + `username: "Kipi"` | 130 | todos `user=UKIPIBOT01`; 46 son raíz de hilo |
| `reactions` | 212 | `users` son uids (208 de clientes, 11 de empleados) |
| `edited {user, ts}` | 158 | `edited.user == user` siempre |
| `files` (+ `upload: true`) | 28 msgs / 36 archivos | `id,name,title,filetype,size,url_private` |
| `attachments`, `blocks`, `reply_count`, `replies` | 0 | no existen |

- `thread_id = <nombre canal>:<thread_ts o ts propio>`. Todas las raíces terminan en `.000100`, todos los ts en `…100`. Ningún `thread_ts` huérfano, ningún hilo cruza días/archivos.
- Bot: todos sus mensajes con autor "Kipi" (se conserva, ver sample). 46 hilos arrancan con Kipi.
- **PII fuera de `text`**: sólo `files[].name` (sale como `attachments[].filename`) y `title`/`url_private` (repiten el nombre, no salen en la salida). `reactions.users` y `edited.user` son uids (no salen, pero no hay uids fuera de users.json).
- Nombres de archivo con PII (8 de 36):

| Archivo | id | PII |
|---|---|---|
| `extracto_camila_restrepo_sep2026.pdf` | e570086e | nombre (test público pub_01) |
| `certificado_laboral_{camilo_castro, julieth_osorio, antonia_diaz, enrique_munoz, mateo_henao, rafael_cardona}.pdf` | ae2bb7f9, c1a187c5, 553491a6, 21d8e090, d5e16d9c, d75156d5 | nombre sin tildes, `_` |
| `pasaporte_ricardo_martinez.jpg` | 3dd0a061 | nombre |
| `SOAT_LTE758_2026.pdf` | 4f35efe3 | **placa** |
| `CC1022649507_certificado_ingresos.pdf` | e468c591 | **cédula pegada a prefijo `CC`** |

Sin PII: `CE_frente/reverso.jpg`, `IMG_####_cedula.jpg`, `cedula_frente.jpg`, `comprobante_pse_12ago.pdf`, `alerta_INC-2026-044.csv`, `dashboard_duplicados_KP-026.pdf` (KP-026 es id de empleado: es llave de tabla, se conserva como id).

## 2. users.json

187 perfiles: 1 bot (Kipi), 36 empleados (`@kipupay.co`, `is_restricted:false`, con `phone` y `title` de cargo), 149 clientes (`is_restricted:true`, `title:"Cliente"`, 78 con email personal, 0 con phone) y 1 **usuario borrado** `U07HF5JVK` (`real_name:""`, `deleted:true`, sin title).

- Campos: `id, team_id, name, real_name, is_bot, is_restricted, tz, profile{real_name, display_name, title, email, phone}`.
- `name` del cliente = parte local del correo en `clientes.csv` (148/149 casan; falla sólo `fcastilla`, Felipe Castilla Castro **no está en clientes.csv**). Es la mejor llave usuario→fila del maestro.
- `real_name` de usuario es a veces **más corto** que el maestro: homónimos traen 2 tokens ("Camila Arango") y el texto/tabla trae 3 ("JULIANA MEJÍA CARDONA", "Daniela Ordóñez Flórez", "Andrés Gómez Giraldo").
- `display_name` son apodos (Cami, Juanpa, Pipe, Anita, Richi, Gabi, Alejo, Camo, Carlitos) y se repiten entre personas (Cami ×3, Dani ×4, Juli ×4).
- Todos los uids de mensajes, reacciones y ediciones existen en users.json. 8 usuarios nunca escriben: 4 empleados (Milton Rojas, Bibiana Giraldo, Emilia Arango, David López) y 4 clientes que sólo aparecen como **terceros** en el texto (Vicente Mejía, Kevin Díaz, Daniel Giraldo, Camilo Arango).
- El borrado escribe 7 mensajes (hilo `soporte-cobros:1791305515`); su identidad sale del texto ("soy Santiago Botero", CC 1.128.862.154, cel 3125492640) → CL-58402 Santiago Botero Londoño. **El autor hay que resolverlo desde el texto o el maestro.**

### Homónimos (mismo `real_name` normalizado)

| Nombre | uids | Tipo | Cómo desempatar (dato en chat → maestro) |
|---|---|---|---|
| Camila Arango | U0W0XS2KC `carango` / U0W7UUU90 `camila.arango` | cli/cli | cédula …2961, nac. 11-sep-1995, Buenos Aires → CL-57176 (Mejía) / cédula 1.022.649.507, cel 350-616-2589, Los Andes → CL-12538 (Velásquez). Hilo 07118882: agente confunde a una con la otra |
| Daniela Ordóñez | U0HHPE7CD `dordonez` / U02WAWSQS `daniela.ordonez` | cli/cli | cc 31401486, dordonez@outlook → CL-56640 (Flórez, Cali, enfermera 1974) / cel 3044985093 → CL-50124 (Ramírez, Barranquilla, contadora 2024). Escalamiento c710e9b6: PQR-2026-0451 |
| Juliana Mejía | U051SHCDN `jmejia` / U0AEFKZCA `juliana.mejia` | cli/cli | cc 31.066.233, PQR-2026-0541, "JULIANA MEJÍA CARDONA" → CL-50577 / juliana.mejia@gmail → CL-67191 (Torres) |
| Mateo Henao | U0YAXSPDR `mateo.henao` / U08ZKXVHX `mhenao` | cli/cli | CL-35202, placa LTE758, 313-207-1166 (panadero) / certificado laboral → CL-37620 (Martínez) |
| Luis Santander | U0L9BMWKL emp (Coord. Fraude) / U0S1DU992 cli | emp/cli | canal (fraude-interno vs soporte); cliente → CL-21823 (dirección Cl 126B # 39-65 en 2c3351e8) |
| Andrés Gómez | U0EJA9VK2 emp (agente, 72 msgs) / U0V1FY7NS cli | emp/cli | **mismo hilo** `soporte-onboarding:1790724211`: el bot dice "Te atiende Andrés Gómez" (09ae3801) a un cliente que se presenta "Andrés Gómez Giraldo" (37ef928e) → CL-78497 |
| Laura Ospina | U01Z5A2DX emp (agente, 95 msgs) / U0LCXRSXT cli | emp/cli | "Soy Laura Ospina" (937e10c1), CL-69452, lospina arroba hotmail. **La agente que se llama igual que una clienta** |
| Sebastián Giraldo | U0N2TCNJA emp (Analista Datos) / U0U6TVW78 cli | emp/cli | cliente: cc 99112352 → CL-66397; empleado nombrado en #general (780df4aa) |

También comparten "nombre+primer nombre": Juan Esteban (emp Torres Quintero / cli Pérez Velásquez), Nicolás Andrés (emp Correa / cli Londoño), Luis Felipe (2 clientes), Esteban Darío (2 clientes); y nombres de pila entre empleados: Fernanda ×2 (Londoño = líder, Vargas = agente), Farid ×2, David ×2, Clara ×2.

## 3. Menciones

Sólo formato `<@UID>`: **10 menciones, 8 uids, todas a empleados**. 0 menciones `<@UID|nombre>`, 0 `<#canal>`, 0 `<url|texto>`.

| id | Mención | Persona |
|---|---|---|
| 511be226, 3387b3a7, ec357348 | `<@U0LAKVS3G>` | Adrián Ospina Posada |
| 0689bd77 | `<@U0L9BMWKL>` | Luis Santander (emp, homónimo de cliente) |
| 72969623 | `<@U0BT62SWJ>` | Fernanda Londoño Jiménez |
| 7797d559 | `<@U094AVHWH>` | Farid Rodríguez Vargas ("primer Analista de Operaciones", viene de Rappi) |

Resto: U06NPN5UQ, U05T3URJH, U02ATPHCU, U0PDEQMDZ. Decisión de diseño: la mención es un uid (no PII por sí sola), pero la persona cuenta en el registro; reemplazar por el seudónimo (`@Nombre`) o dejar el uid y declarar el registro en la entidad.

## 4. PII en `text` (regex, conteos)

| Tipo | Hits / msgs | Formatos vistos | Ejemplos (id) |
|---|---|---|---|
| Cédula con puntos | 21 únicas (el regex `\d{1,3}(\.\d{3}){2,3}` da 68 hits: **montos** 1.480.000, 6.840.000 comparten formato) | `1.020.316.175`, `63.447.309`, `CC 1.128.862.154` | b79a5cc7, 2338e86d, 968db369 (pub_02) |
| Cédula sin puntos | ~30 (8 y 10 dígitos) | sola en el mensaje, `cc 1022906508.`, `CC1022649507` (filename) | f8a31411, 6e9c7a3c (`80901316` dado por empleada), 410da6be |
| Últimos 4 de cédula/celular | 26 mensajes `^\d{4}$` + 8 "termina en ####" | `3654`, `1806 y 22 de abril…`, "cédula terminada en 2961" | 3ae2d492, 07118882 |
| Celular 10 dígitos pegado | 34 | `3044985093`, "mi celular es 3221331159" (empleado) | ed8e120d, 0c486a8c |
| Celular `+57` con espacios | 10 | `+57 317 426 8173` | 7ae650b1, d4a4f5e2 |
| Celular con guiones | 2 | `313-207-1166`, `350-616-2589` | aec1b074, 831677b8 |
| Correo normal | 55 hits / 50 únicos | `mruiz@gmail.com`, `carlos.robledo@kipupay.co` (empleado) | f3782345, dda2f994 |
| Correo deletreado | 8 | "jcastro arroba gmail punto com", "juliana punto mejia arroba gmail punto com", "camila punto arango arroba gmail punto com" | c2b395c4, c74ce2a6, 79e4febd |
| Correo en inglés `at/dot` | 0 | | |
| Placa | 4 | `LTE758`, `GTD596`, `KFY046`, `JXA-971` (+ `SOAT_LTE758` en filename) | e5fe637b, 3e8929b9, ea88fd5f |
| Pasaporte | 2 | `AR752831`, `AP395977` | 3dd0a061, 9afe8c98 |
| Cuenta bancaria | 1 | `2410 7087 407` (Bancolombia) | 6155f6aa |
| Tarjeta 16 dígitos | 0 | | |
| Dirección | 19 | `Cra 25 # 98-84, Bogotá`, `Tv 46 # 37-32 torre 5 apto 601`, `Transversal 56 Bis # 83-31 casa 56`, `Cl 27B # 69-41 casa 57`, `Av 17 Sur # 58-87`, "EAFIT, bloque 19" | d1992650, 0cd14bb6, cc48399a |
| Fecha de nacimiento en palabras | 18 | "14 de agosto de 1996", "2961 y 11 de septiembre de 1995" | b46912ae, 5c52a114 |
| Fecha numérica | 3 | `1999-02-19`, **`DOB 01/08/1993` y `01/23/1978` en formato US (MM/DD)** | 37ef928e, 02c8134d, 63c0f17b |
| Edad / año nacimiento | 2 | "tengo 54 años", "nacida en 1974" | 61dd333c, 538cd293 |
| Nombre MAYÚSCULAS | 10 | "Encontré una cuenta a nombre de JULIANA MEJÍA CARDONA" (bot) — **con tildes** en chats | d8d8325d, ad8a127f, 2260091c |
| URL con PII | 3 | `?ref=agomez`, `?u=khenao`, `/comercios/gabriela-londono/` | fc11f21d, b640ff9d, 6f94abf3 |
| Referencia PSE | 7 | `PSE-76224471` (no es PII personal; conservar) | 154b916f |

No hay: dígitos dictados en palabras, espacios invisibles (0 de U+200B/00A0/00AD/FEFF…), homoglifos, `<@UID|nombre>`. Únicos no-ASCII: emojis y `—`.

**Ruido**: ~54+ mensajes de clientes con letras borradas ("Ah est el problema… qued mal escrito", "Me estn llamando de un nmero", "targeta", "bloqarla"). Revisado: **no afecta a nombres ni datos** (0 nombres con una letra menos), pero rompe regex léxicas tipo "cédula"/"dirección" ("cdula de extranjera", "direccin").

## 5. Presentación, handoff y terceros

- Empleado se presenta por nombre de pila: "soy X / habla X / te atiende X / le saluda X / mi nombre es X" (170 msgs), "X por acá" (49), firma "— Laura" (38). El cliente responde con el nombre en minúscula: "gracias celeste", "mil gracias brayan" (30).
- Handoff del bot con **nombre + primer apellido del agente**: "Te atiende Samuel Henao / Fernanda Vargas / Isabel Díaz / Andrés Gómez, del equipo de soporte" (7; 4ed44195, 8506ee3e, 09ae3801).
- Cliente se presenta con nombre completo (27 self full + 29 nombre+apellido): "soy Alicia Mejía Jiménez, cc 1004193042" (8c17ae40), "Hola, soy Santiago Botero" (8caaab09). Los onboarding piden "nombre completo como aparece en el documento" → mensaje que es solo el nombre (f70cd25f).
- Agente repite datos del cliente: "Lo tenemos registrado como dordonez@outlook.com" (d9fd1ee8), "¿El celular registrado sigue siendo 3122163544?" (7560f0ab), "¿La enviamos a Cra 32 Sur # 64-23?" (8bbd1019).
- **Terceros** (24 hits de parentesco):

| id | Texto | Persona (match en maestro) |
|---|---|---|
| 0104ddc0 | "mi referencia personal es mi cuñado Camilo Arango, cel 3044251059" | CL-47288 Camilo Arango Osorio |
| 2d157f3b | "lo usa ahora mi socio del estudio, Vicente Mejía" | CL-19668 Vicente Mejía Arango |
| 72869727 | "mi jefe Kevin Díaz (… está en Bucaramanga)" | CL-25026 Kevin Díaz Hoyos |
| b454cf3e | "¿Puedo llevar a mi novia Daniel Giraldo? Es clienta también" | CL-98729 Daniel Giraldo Hoyos — **nombre masculino con referencia femenina** |
| 24c3459d | "my sister in law María Fernanda Díaz" | probablemente CL-55931 María Fernanda Díaz Marulanda (Cartagena, como Kelly) |
| 69001c5c / b386f0c5 | "a nombre de un tal Brayan Gómez", "la de Ignacio Zapata … +57 317 301 9375" | CL-13198, CL-74990 (clientes con hilos propios) |
| 0689bd77 | "cuenta de Juan Pablo Bolívar (CL-60315)" + cc + celular + correo en hilo | CL-60315 |
| 410da6be | "filtren por Esteban Giraldo, cc 1019315624" (mensaje `edited` pero aún con la cc) | CL-19676 |
| 9684c29d | "mi hermano Daniel me transfirió desde su llave 3233388161" | sin apellido |
| a60670d5, 0bb332d5 | "a nombre de Silvia Sánchez / Nohora Marulanda" (forma corta del propio cliente) | autor |

Empleados nombrados sólo por nombre de pila en texto interno: "Fernanda (agente de Cali)" (ambigua entre 2 Fernandas), "Emilia solo corrió el proceso" (Emilia Arango, no escribe), "Isabel pegó la cédula", "nuestra gerente de riesgo cumple años el 20 de julio" (ad4a617e → Pamela Castaño, cargo único + fecha).

## 6. Ids cruzables

| Tipo | Conteo | Ejemplos |
|---|---|---|
| `PQR-2026-####` | 7 hits / 6 únicos | 0451, 0467, 0541, 0548, 0550, 0556 |
| `CL-#####` | 10 | CL-50124/56640 (cc48399a), CL-60315, CL-30787, CL-69452, CL-35202, CL-72216, CL-78497, CL-23528, CL-87887 |
| `INC-2026-###` | 2 | INC-2026-031, INC-2026-044 (+ filename) |
| `KP-###` | 1 (filename) | KP-026 |
| `TX-` | 0 | — |
| Montos | muchos | 1.350.000 el 24-ago (PQR-0541), 612.400, 230.000, 1.480.000 en Unicentro 6-oct |

Los ids se conservan (son llaves); sirven para unir chat ↔ maestro y desempatar homónimos.

## 7. Cuasi-identificadores (ocupación + barrio + año)

| id | Texto | Autor |
|---|---|---|
| af9ccd6e | estudiante de derecho, Buenos Aires, desde 2025 | Enrique Muñoz |
| 8e61cf6b | veterinaria, Sotomayor, 2021 | Inés Pérez |
| 9cb80452 | contadora, Bocagrande, 2021 | Sara Vargas |
| 21cc8404 | psicóloga, Manga, 2023 | Antonia Díaz |
| f29b48a2 | fotógrafo, El Prado, 2020 | Lisandro Díaz |
| 2f087832 | abogado, **Robledo**, 2025 (Robledo es también apellido del agente Carlos Robledo) | Rafael Cardona |
| 4ac61aed | odontóloga, consultorio en Castilla, desde 2021 | Gabriela Londoño |
| 2193586a | panadero, local de Real de Minas, desde 2025 | Mateo Henao (CL-35202) |
| 61dd333c | 54 años, diseñador gráfico freelance, Teusaquillo | Ricardo Martínez |
| a892b40d | "trabajas en Nutresa como administradora y vives en Manrique" (lo dice la agente) | Ana Lucía Marulanda |
| 08490a1d / eddc4f95 | diseñadora, oficina en Buenos Aires / facultad de derecho Los Andes | Camila Arango ×2 |
| 80e0b943 | "la de Barranquilla… contadora… desde 2024" (empleada sobre tercera) | → CL-50124 |
| 56ff388c | "administrador, vive en Castilla, cliente desde 2020" | → CL-19774 Mario Hoyos (cc 80901316 en 6e9c7a3c) |
| 538cd293 | "clienta de Cali, enfermera, nacida en 1974" | → CL-56640 |
| 69001c5c | "arquitecto, vive en Chapinero" | → CL-60315 |
| f7d5204b | "clienta de Barranquilla … abogada, desde 2024" | sin nombre |
| 792d1a4c | "cliente desde 2021" + Plaza Botero | Santiago Botero |

Patrón muy regular en tarjetas: "soy <ocupación>, vivo en <barrio>. y llevo con ustedes desde <año>". Hay que generalizar en chat **y** en `clientes.csv` (la indirecta se evalúa sobre la unión).

## 8. Falsos positivos a conservar

| Clase | Términos (nº msgs) |
|---|---|
| Bancos / pagos | Bancolombia 29, PSE 23, Davivienda 15, Banco de Occidente 2, Santander 2 (banco), Colpatria 1, Nequi 2 (pub_03, b6d50ca1), Daviplata 0 |
| Empresas / marcas | Kipu Pay 67, Kipi 96, Juan Valdez 3, Nutresa 2, Rappi 2, SURA 1, "póliza de Bolívar" 1, Éxito, Unicentro, Walgreens, EAFIT, Los Andes, Veloz, Android/iPhone 6 |
| Reguladores | DIAN 8, SIC 2, Superfinanciera 1 |
| Figuras públicas | Gabriel García Márquez, Jorge Isaacs, Egan Bernal, Shakira |
| Lugares | Plaza Botero, Parque Lleras, Feria de las Flores, ciudades (Bogotá 12, Barranquilla 13, Medellín 9…), Lisboa, Miami, Panamá, España |
| Palabras = nombres | "Claro" (≈ Clara, 39), "muy clara la explicación" (6, empleada Clara), "Celeste" (agente, 27; también color), "Paz" (Verónica Paz), campaña "María" |

**Apellidos que son lugar/marca** (siguen siendo persona): Luis **Santander** (+ "PSE de Luis Santander desde Santander", d40d8c0a), Juan Pablo **Bolívar** (vs "póliza de Bolívar", 594351d5), Santiago **Botero** (vs Plaza Botero en el mismo hilo), Mariana **Carulla** (marca Carulla; su nota en maestro dice "Carulla de la 85"), Felipe **Castilla** (barrio Castilla en 2 msgs), Carlos **Robledo** (barrio Robledo), Luis Felipe **Córdoba**.

## 9. Trampas y casos raros

1. **Usuario borrado** U07HF5JVK con `real_name:""`: el autor debe resolverse por el texto/maestro (Santiago Botero, CL-58402).
2. **Mismo nombre cliente↔agente en el mismo hilo** (Andrés Gómez, 1790724211): resolver por uid; el handoff del bot nombra al agente, no al cliente.
3. **Homónimos con `real_name` truncado**: 4 pares cliente-cliente + 4 empleado-cliente. El texto da el desempate (cédula, cel, CL, PQR, placa, correo) y `users.name` = local del correo en el maestro.
4. **Empleados filtrando PII de clientes**: cédulas (b79a5cc7, 2338e86d, 410da6be, 6e9c7a3c), celulares (ed8e120d, 7ae650b1, b386f0c5), correos (7ae650b1), direcciones (cc48399a, 3de66935) en canales internos. Y empleados filtrando PII propia: celular 3221331159 (Carlos Robledo, 0c486a8c), dirección de casa del CEO (c948dda2), correo corporativo (dda2f994), cumpleaños de la gerente de riesgo.
5. **Bot imprime nombre en MAYÚSCULAS** con el 3er apellido que no está en users.json.
6. **Fechas US** (`DOB 01/08/1993` = 8-ene-1993 en maestro) y mensajes en **inglés** (2 hilos: Kelly Henao, Milton Pérez; "passport number", "ID").
7. **Cédula en filename** pegada a `CC` y **placa** en filename SOAT.
8. Respuestas sueltas de solo dígitos (26 de 4 dígitos, 22 de 10, 8 con puntos, 7 de 8): el tipo (cédula vs celular vs últimos-4) sólo se deduce de la pregunta previa del hilo; los 10 dígitos que empiezan por 3 son celulares, por 1 cédulas.
9. Mensaje `edited` que todavía contiene la cédula (410da6be) — la edición no la borró.
10. URLs internas con usuario/slug del cliente (`ref=agomez`, `u=khenao`, `gabriela-londono`).
11. Nombre de género ambiguo ("mi novia Daniel Giraldo"): elegir seudónimo por el maestro, no por el contexto.
12. Formas cortas del mismo cliente: "Renata Velásquez", "Silvia Sánchez", "Nohora Marulanda", "Ana Lucía", "Juli" (firma), "G. Londoño" (inicial + apellido, 145fbb45).
