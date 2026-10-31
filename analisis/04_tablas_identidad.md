# 04 · Tablas y resolución de identidad entre fuentes

Fuente: `fuentes/tables/*`, `fuentes/chats/users.json`, cruces con `chats/` y `docs/`. Scripts en scratchpad `tables/` (a1–a9, b1–b6).

## 1. Esquema por tabla

`record_index.json → tables` = 6.243 ids: `transacciones` 6.030, `clientes` 149, `empleados` 36, `pqrs` 28. Llaves confirmadas:

| Tabla | id de registro | Llave | Columnas (salida `columns`, todo texto) |
|---|---|---|---|
| clientes.csv (Latin-1, `;`, CRLF, notas con `;` entre comillas) | `clientes:CL-12538` | `cliente_id` | cliente_id, nombre_completo, tipo_doc, num_doc, correo, celular, ciudad, barrio, direccion, fecha_nacimiento, fecha_vinculacion, ocupacion, notas |
| transacciones.csv (UTF-8, `,`) | `transacciones:TX-000158` | `tx_id` | tx_id, fecha, cliente_id, tipo, monto, descripcion, canal, estado |
| pqrs.xml | `pqrs:PQR-2026-0451` | atributo `radicado` | atributos radicado, cliente_id, cedula, ciudad, fecha, agente, tipo, estado + hijos `descripcion` (CDATA), `respuesta` |
| empleados.xml | `empleados:KP-005` | atributo `id` | atributos id, cargo, ciudad, ingreso, reporta_a + hijos nombre, correo, celular, documento (attr `tipo`), fecha_nacimiento |

Ojo: pqrs.xml trae un comentario XML `<!-- Exportado por luis.santander@kipupay.co ... -->` (PII de empleado KP-005; no es un registro, pero si se reemite el XML hay que limpiarlo).

### Clasificación de columnas

| Tabla | PII directa (seudonimizar) | Cuasi-identificador | Llave / conservar | Texto libre (escanear) |
|---|---|---|---|---|
| clientes | nombre_completo, num_doc, correo, celular, direccion, fecha_nacimiento | ciudad, **barrio**, **ocupacion**, fecha_vinculacion | cliente_id, tipo_doc | notas |
| transacciones | — (ninguna columna) | — | tx_id, cliente_id, fecha, monto, tipo, canal, estado | descripcion (PII en ~15 filas) |
| pqrs | atributo `cedula`, atributo `agente` (usuario Slack de empleado, p. ej. `fernanda.vargas`) | ciudad | radicado, cliente_id, fecha, tipo, estado | descripcion, respuesta |
| empleados | nombre, correo, celular, documento, fecha_nacimiento, **reporta_a** (nombre corto del jefe) | cargo+ciudad+ingreso (36 filas: casi todo k=1, pero cargo es dato de negocio; conservar como en el organigrama) | id (KP-###) | — |

Muestra (`sample/expected_output/tables.jsonl`) fija la política de clientes: mismo cliente_id, **ciudad y fecha_vinculacion intactas**, ocupación → sector ("profesional de …"), **barrio reemplazado por otro barrio de la misma ciudad** (Granada→Santa Mónica, Envigado→Castilla), dirección/cédula/correo/celular/nacimiento seudónimos con formato válido, notas con nombres reemplazados y `(CL-88627)` intacto.

### PII en texto libre

**clientes.notas** (54 no vacías; 3 plantillas inocuas repetidas: "No llamar antes de las 9 a. m.", "Prefiere contacto por WhatsApp.", "Cliente referido por campaña Feria de las Flores.", "Actualizó dirección en 2026."):

| cliente | PII en nota |
|---|---|
| CL-14298 | pasaporte `AP395977` (también en chat) |
| CL-35202 | placa `LTE758` (chat: `SOAT_LTE758_2026.pdf`) |
| CL-72216 | placa `GTD596` (tx: `GTD-596`), "Esposo autorizado" (tercero sin nombre) |
| CL-87887 | "Referencia personal: Camilo Arango, 3044251059" → es el cliente CL-47288 (mismo celular) |
| CL-19774 | dirección completa "Calle 57 # 67-52 of 103" (= su propia dirección) |
| CL-31549 | "Amigo personal del CEO David Mejía" → empleado KP-001 |
| CL-24043, CL-65777, CL-12538, CL-66397, CL-23528… | cuasi-identificadores laborales: "área jurídica de Avianca", "Chef del restaurante del Carulla de la 85", "Crédito educativo Los Andes", "Piloto comercial", "trabaja remoto para empresa del exterior" |
| varias | radicados PQR, INC-2026-0xx, S-2608-0x, créditos 4471-xxxx, pólizas (datos de negocio: conservar) |

Trampas de falso positivo en notas: Plaza Botero (cliente Santiago **Botero**), Carulla (clienta Mariana **Carulla**), Seguros Bolívar (cliente Juan Pablo **Bolívar**), barrio Galán (clienta Emilia **Galán**), Éxito Envigado, Nequi, Avianca, Los Andes.

**transacciones.descripcion**: 518 textos distintos, casi todos plantilla comercio+barrio ("Compra Carulla Castilla", "Retiro cajero Bancolombia Galán", "Parqueadero Parque de Bolívar"). El barrio en la descripción no es el del cliente (314 coinciden vs 2.085 no) → conservar. Filas con PII:

| tx | descripción | PII → persona |
|---|---|---|
| TX-003992, 004117, 004404 (CL-60315) | `Transferencia a Santander - BRAYAN GOMEZ VARGAS` | nombre MAYÚSCULAS sin tildes → CL-13198 |
| TX-000363 (CL-21823) | `PSE recibido de LUIS SANTANDER MORENO desde Santander - cuota 7 …` | el propio cliente; "Santander" (banco/departamento) se conserva |
| TX-001215 (CL-61348) | `Acuerdo de pago cuota 3/10 - CC 1.018.922.290` | cédula con puntos del propio cliente |
| TX-001481 (CL-31549) | `Transferencia a Nequi 3045662486` | celular de CL-19668 (Vicente Mejía Arango) |
| TX-000292 (CL-45343) | `Transferencia recibida Nequi llave 3233388161` | celular de CL-39098 (Daniel Londoño Giraldo) |
| TX-005458 (CL-98729) | `Transferencia de Farid Rodríguez - cena` | empleado KP-019 |
| TX-006030 (CL-47288) | `Transferencia a cuenta 9120 9765 655 - A. Marulanda` | inicial+apellido → Ana Lucía Marulanda (CL-87887, su cuñada); n.º de cuenta |
| TX-000552/000685 (CL-72216) | `Abono crédito vehicular placa GTD-596` | placa |

**pqrs.descripcion/respuesta**: cédula con puntos (0480 `1.022.029.325`), cédula cruda (0506 `1010254412`), celular plano (0501 `3174268173`) y `+57 321 252 4068` (0520), correos (0524, 0550), dirección (0488), nombres de autofirma (0520 "Luis Felipe Córdoba", 0534 "Luis Santander", 0550 "Saúl Gómez", 0552 "Sebastián Giraldo"), ocupaciones en primera persona (0451 enfermera, 0467 contadora, 0472 "trabajo ahí" en Carulla, 0552 aerolínea). Todos coinciden exactamente con la fila del maestro del mismo `cliente_id`. Dos PQR en inglés (0515, 0555).

## 2. Formatos y suciedad

| Campo | Observado |
|---|---|
| tipo_doc | 149/149 `CC` (en chat 7 onboardings dicen "cédula de extranjería" + adjuntos `CE_frente.jpg`; el maestro igual dice CC). Empleados: 36 `CC`. |
| num_doc | solo dígitos; 8 díg. (75; 22.4M–99.8M) o 10 díg. (74; 10xx–11xx). Empleados 8 (19) / 10 (17). Sin duplicados. |
| celular | 10 díg., prefijos 31 (70), 30 (35), 32 (34), 35 (10). En texto aparece como `350-616-2589`, `+57 321 252 4068`, plano. |
| correo | dominios gmail 66, hotmail 25, yahoo 21, outlook 20, icloud 17. Local `inicial+apellido1` (136) o `nombre.apellido1` (13, justo los homónimos y algunos más). Empleados `nombre.apellido@kipupay.co`. En chat deletreado: "camila punto arango arroba gmail punto com" (8 casos). |
| direccion | `Cl/Calle/Cra/Carrera/Av/Diagonal/Tv/Transversal N[A-Z]?[ Bis] # N-N [apto/casa/of/torre …]` |
| fechas | ISO `YYYY-MM-DD`; nacimiento 1968–2000; vinculación 2019–2026 (2026: CL-78497, CL-87887, que no tienen transacciones). Tx 2026-07-01..10-16. En chat: "11 de septiembre de 1995", últimos 4 de cédula ("2961", "1806"). |
| placas | `LTE758`, `GTD596`/`GTD-596`, `KFY046` (chat), `JXA-971` (chat, en inglés) |
| pasaportes | `AP395977` (nota + chat), `AR752831` (chat + `pasaporte_ricardo_martinez.jpg`) |
| cuentas | `9120 9765 655` (tx), créditos `4471-0150` (negocio, conservar) |

Suciedad: clientes.csv está limpio (sin espacios sobrantes, sin mojibake al leer Latin-1; nombres en Title Case con tildes). La suciedad está fuera del maestro: MAYÚSCULAS sin tildes en tx (`BRAYAN GOMEZ VARGAS`), MAYÚSCULAS con tildes en el bot (`JULIANA MEJÍA CARDONA`), chats con letras acentuadas **borradas** ("cdula de extranjera", "qued mal escrito"), minúsculas ("soy camila arango"), nombres en filename (`certificado_laboral_mateo_henao.pdf`, `CC1022649507_certificado_ingresos.pdf`). La normalización para matching debe quitar tildes, pasar a minúsculas y tolerar letras faltantes.

## 3. Homónimos

No hay dos filas con nombre completo idéntico; los homónimos son **nombre + primer apellido** (la forma que se usa en chat, users.json y docs). `users.json` trunca `real_name` justo en estos 12 clientes y 4 empleados.

| Nombre corto | Personas | Desempate (otra fuente) |
|---|---|---|
| Camila Arango | CL-12538 Velásquez (Bogotá, uid U0W7UUU90 `camila.arango`) · CL-57176 Mejía (Medellín, uid U0W0XS2KC `carango`) | uid → correo local; en chat: "cédula terminada en 2961" (=CL-57176), "Mi cédula es 1.022.649.507" (=CL-12538), dirección `Cl 27B # 69-41 casa 57`, adjunto `CC1022649507_certificado_ingresos.pdf`. Test público pub_04. |
| Daniela Ordóñez | CL-56640 Flórez (Cali, `dordonez`) · CL-50124 Ramírez (Barranquilla, `daniela.ordonez`) | radicado: PQR-0451→CL-56640, PQR-0467→CL-50124 (informe_pqr_septiembre los nombra solo "Daniela Ordóñez"); chat escalamientos 2026-10-05 nombra ambas en un mismo mensaje y da CL-50124, `Av 17 Sur # 58-87`, celular 3044985093; tx TX-002263 356.000 (CL-50124). |
| Juliana Mejía | CL-50577 Cardona (`jmejia`) · CL-67191 Torres (`juliana.mejia`) | radicado: PQR-0541 (arriendo 1.350.000, 24-ago = TX-000004/5) → Cardona; PQR-0537 (420.000, 13-ago) → Torres. Bot: "JULIANA MEJÍA CARDONA". Correo deletreado `juliana punto mejia` → Torres. |
| Mateo Henao | CL-35202 Ospina (Bucaramanga, `mateo.henao`) · CL-37620 Martínez (Bogotá, `mhenao`) | placa LTE758 (nota CL-35202 + chat cobros); acta comité ago "S-2608-01 Mateo Henao … cuota inicial 1.200.000 el 25 de agosto" = TX-000006 (CL-37620); adjunto `certificado_laboral_mateo_henao.pdf` lo sube `mhenao` → CL-37620. |
| Luis Santander | CL-21823 Moreno (cliente, `lsantander`) · **KP-005** Ruiz (empleado, `luis.santander`) | carta_mora_luis_santander.pdf trae `CC 91.338.232`, dirección → cliente; PQR-0534 y TX-000363 → cliente; fraude-interno "PSE de Luis Santander … del 3 de septiembre" = TX-000363 → cliente; `reporta_a`, acta fraude, organigrama, comentario XML → empleado. |
| Andrés Gómez | CL-78497 Giraldo (cliente, `agomez`) · **KP-011** Ruiz (agente, `andres.gomez`) | ¡en el mismo hilo! onboarding 2026-09-29: cliente da "Andrés Gómez Giraldo, 1010254412…" y Kipi dice "Te atiende Andrés Gómez, agente de onboarding". PQR-0506: cliente CL-78497 con `agente="andres.gomez"`. Resolver por uid/rol, no por nombre. |
| Laura Ospina | CL-69452 Posada (`lospina`) · **KP-021** Salazar (`laura.ospina`) | calibracion_calidad: columna Agente vs Cliente (aparece en ambas columnas); PQR-0546 230.000 12-ago = TX-000003. |
| Sebastián Giraldo | CL-66397 Castaño (`sgiraldo`) · **KP-026** Betancur (`sebastian.giraldo`) | adjunto `dashboard_duplicados_KP-026.pdf` + canal general → empleado; bot "SEBASTIÁN GIRALDO CASTAÑO", PQR-0552 → cliente. |

Otros cruces de nombre a vigilar: Camilo Arango (referencia en nota/chat de CL-87887) = cliente CL-47288 por celular 3044251059 (no es tercero nuevo). "Fernanda (agente de Cali)" = KP-012 Vargas, no KP-004 Londoño. Nombres de pila repetidos entre empleados: David (KP-001/035), Farid (002/019), Fernanda (004/012), Clara (006/018), Nicolás (029/034); apodos compartidos en display_name ("Juli" = Julián Henao KP-030, Juliana Mejía ×2, Julián Vargas; "Dani", "Cami", "Sebas").

Apellidos que son lugar/marca (no tocar el lugar, sí la persona): Restrepo (Estación/barrio Restrepo ↔ Camila Restrepo), Galán, Castilla (barrio ↔ Felipe Castilla), Bolívar (Parque/Seguros/Simón ↔ Juan Pablo Bolívar), Botero (Plaza Botero ↔ Santiago Botero), Carulla, Santander (banco ↔ Luis Santander), Robledo (barrio ↔ agente Carlos Robledo; evaluación KP-009 tiene ambos), Boston, Envigado.

## 4. Empleados

- 36 empleados, `id` KP-001..036. `reporta_a` es **nombre corto** "Nombre Apellido1" (y "Juan Esteban Torres" no aparece como jefe); 9 valores, todos resuelven de forma única a un KP: Fernanda Londoño (12), David Mejía (8), Farid Díaz (6), Pamela Castaño (3), Luis Santander (3), Adrián Ospina, Amalia Aristizábal, Norberto Gutiérrez (1 c/u), CEO vacío. Para conservar la relación: `reporta_a` = seudónimo corto (nombre+apellido1) del jefe, el mismo que use el organigrama (doc_01, tabla Cargo/Persona/Reporta a).
- users.json: 36 cuentas con correo `@kipupay.co`, `name` = parte local del correo (llave de unión), `phone` = celular de nómina (36/36 idénticos). 4 `real_name` truncados (KP-005, 011, 021, 026: los homónimos). display_name con apodos ("Carlitos", "Andre", "Sebas", "Juli", "Nico").
- pqrs `agente` = `name` de Slack de 13 agentes/analistas.
- Docs con id de empleado: `evaluacion_desempeno_KP-009_2026.pdf` (Carlos Robledo Salazar, fecha de nacimiento 1995-12-14, "vive en Itagüí"; autora Fernanda Londoño en metadata), `evaluacion_desempeno_KP-022_2026.pdf` (Brayan Zapata, keywords en metadata), adjunto `dashboard_duplicados_KP-026.pdf`. El KP-### es llave (conservar), lo que hay que reemplazar es el nombre.
- Metadata PDF con empleados: Author Milton Rojas, Fernanda Londoño, Pamela Castaño, Emilia Arango, Amalia Aristizábal; Subject con correos `julian.henao@`, `amalia.aristizabal@`. Cédula del CEO `71.170.753` en vendor_agreement.

## 5. Transacciones

Unión: `cliente_id` → clientes (6.030 filas, 147 clientes con 23–60 tx; ningún cliente_id huérfano; sin tx: CL-78497 y CL-87887). Sin columnas PII; todo el riesgo está en `descripcion` (tabla §1). Las tx son la evidencia "fecha + monto" para desempatar PQR/chat (TX-000003↔PQR-0546, TX-000004/5↔PQR-0541, TX-000006↔S-2608-01, TX-000363↔carta mora/fraude-interno, TX-002263↔PQR-0451, TX-004839 1.480.000 Unicentro Pereira↔PQR-0548). Conservar fecha, monto, tipo, canal, estado y nombres de comercio/banco/lugar.

## 6. PQRS

28 `<pqr>` con `cliente_id` y `cedula` como atributos; 27/28 cédulas y ciudades coinciden con el maestro. **PQR-2026-0530 → CL-55481 no existe en clientes.csv**: es Felipe Castilla (acta comité sep "S-2609-05 | Felipe Castilla, electricista independiente … falta declaración de renta"; usuario Slack U06T6M659 `fcastilla`, Felipe Castilla Castro). Es una persona real con cliente_id que solo vive fuera del maestro (posible fila nueva cuando crezcan las fuentes). Vínculos: radicado ↔ notas de clientes (0451, 0539, 0540, 0544, 0547, 0548, 0550, 0553, 0555, 0556), informes PQR (PDF) por radicado, chats ("Ver ticket en #soporte-cobros", escalamientos 0451), `agente` ↔ users.json.

## 7. Cuasi-identificadores (clientes, n=149)

| Combinación | filas k=1 | k≤2 | k<5 |
|---|---|---|---|
| ciudad | 0 | 0 | 0 |
| ciudad+barrio | 9 | 27 | 101 |
| ocupación | 0 | 14 | 62 |
| ciudad+ocupación | 85 | 125 | 144 |
| ciudad+año | 12 | 30 | 68 |
| ocupación+año | 82 | 130 | 149 |
| barrio+ocupación | 139 | 149 | 149 |
| ciudad+barrio+año | 108 | 142 | 149 |
| **ciudad+barrio+ocupación+año** | **149** | 149 | 149 |
| ciudad+sector (13 sectores) | 31 | 61 | 110 |
| ciudad+sector+año | 109 | 137 | 149 |
| sector+año | 30 | 68 | 144 |

Conclusión: con 149 filas nada aguanta k≥5 si se conserva ocupación literal; la fecha_vinculacion completa (que la muestra deja intacta) ya es casi única. El test es de **co-ocurrencia** de los valores originales (p. ej. `["fotógrafa","Granada","2025"]` con `join_records: clientes:CL-88627`), así que basta con que la fila no conserve juntos ocupación original + barrio original: ocupación → sector **siempre** y barrio → barrio seudónimo de la misma ciudad (atado a la dirección seudónima), como en la muestra. El mismo trío debe romperse en chats/PQR ("soy contadora… desde 2024… Barranquilla"). Ojo: "Castilla" y "Boston" son barrio en dos ciudades.

Ocupaciones (35 distintas) y mapeo propuesto (estilo muestra):

| Sector (salida) | Ocupaciones (n) |
|---|---|
| profesional de las industrias creativas | fotógrafo (2), fotógrafa (2), diseñador gráfico (4), diseñadora gráfica (2) |
| profesional de medios | periodista (7), community manager (3) |
| profesional de servicios | administrador (5), administradora (5), estilista (3) |
| profesional del transporte | mecánico (7), conductor de plataforma (5), piloto comercial (5) |
| profesional de finanzas | contadora (8), contador (3), auxiliar contable (3) |
| profesional del sector legal | abogada (7), abogado (5) |
| profesional de la salud | fisioterapeuta (4), enfermera (3), odontóloga (2), psicóloga (2), veterinario (3), veterinaria (2) |
| profesional de la construcción | arquitecto (4), arquitecta (4), ingeniero civil (3), electricista (3) |
| profesional de tecnología | ingeniera de sistemas (5), ingeniero de sistemas (4) |
| profesional de la gastronomía | chef (5), panadero (6) |
| profesional de la educación | docente (7) |
| profesional del comercio | comerciante (10) |
| estudiante universitario | estudiante de derecho (4), estudiante de medicina (2) |

La ocupación también delata género (fotógrafa/fotógrafo): útil para elegir seudónimo del mismo género; el sector es neutro.

## 8. Personas y esquema de entity_id

| Grupo | n | Ancla estable |
|---|---|---|
| Clientes en maestro | 149 | cliente_id (148 con uid Slack por `name`=local del correo; CL-58402 Santiago Botero escribe con el usuario borrado U07HF5JVK, se resuelve por "CC 1.128.862.154" en el chat) |
| Cliente fuera del maestro | 1 | CL-55481 Felipe Castilla (PQR-0530, acta, uid fcastilla) |
| Empleados | 36 | KP-### (= uid `@kipupay.co`) |
| Bot Kipi | 0 | no es persona; conservar |
| Terceros nombrados ya vistos | ≥5 | Hernando Gómez (carta cobro jurídico), Kelly Marulanda (auditora), Pamela Agudelo + pagudelo@gmail.com (comentario HTML investor memo), Jesús Sánchez (rep. Veloz), "Vicente Mejía, socio del estudio" (mención de Santiago Botero; ¿es CL-19668? sin evidencia dura → tratar como ambiguo) |
| Terceros sin nombre | — | mamá, esposo, vecino, "abogado externo" (no requieren seudónimo) |
| Figuras públicas | conservar | Gabriel García Márquez, Shakira, etc. (`kept: true`) |

Total estimado: **~190–200 personas** (186 ancladas + terceros de chats/docs que identifiquen los otros análisis).

Esquema de `entity_id` propuesto (determinista, sobrevive al crecimiento de fuentes):

- Cliente: `p_cl_<num>` (p. ej. `p_cl_12538`), también para clientes que solo aparecen por cliente_id fuera del maestro (`p_cl_55481`).
- Empleado: `p_kp_<num>` (`p_kp_005`).
- Tercero: `p_t_<hash8(nombre_normalizado | record_id de primera aparición)>`; si después aparece en el maestro con identificador duro (cédula/celular/correo), se fusiona al `p_cl_` (pero la fusión debe ser estable entre corridas: guardar mapa).
- Organización/lugar/figura conservada: `o_<slug>`, `kept: true`, `reason`.
- Seudónimo = f(HMAC(secreto, entity_id)) sobre pools por género, con verificación de unicidad y persistencia del mapa (`pseudonyms.json`) para que las corridas después del cambio reutilicen exactamente lo ya asignado. Nunca derivar la identidad del nombre: homónimos se separan por ancla, y la resolución mención→entidad usa uid, cliente_id, radicado, cédula/celular/correo, placa, fecha+monto, filename con KP-/CC.

Formas a declarar por persona en `originals`: nombre completo, nombre+apellido1, nombre de pila (si se usa solo), MAYÚSCULAS sin tilde, token de filename (`mateo_henao`), cédula cruda y con puntos, últimos 4 de cédula cuando aparezcan, celular plano/con guiones/+57, correo y versión deletreada, dirección, fecha de nacimiento (ISO y en palabras), placa con y sin guion, pasaporte.
