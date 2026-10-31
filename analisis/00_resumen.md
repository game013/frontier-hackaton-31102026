# Resumen consolidado del meta-análisis

Detalle en `01_sample.md`, `02_chats.md`, `03_docs.md` y `04_tablas_identidad.md`.

## Números

| Qué                          | Valor                                                                  |
| ---------------------------- | ---------------------------------------------------------------------- |
| Personas estimadas           | 190–200: 149 clientes + CL-55481 (fuera del maestro) + 36 empleados + ≥5 terceros |
| Usuarios de chat             | 187 (1 bot, 36 empleados, 149 clientes, 1 borrado)                     |
| Pares de homónimos           | 8 (4 cliente–cliente, 4 empleado–cliente), todos con desempate en otra fuente |
| Cuasi-identificadores en texto | ~17 en chats, 12 en documentos; en clientes.csv k=1 en las 149 filas  |
| Documentos en inglés         | 3 (security report, contrato Veloz, memo inversionistas con `lang="es"`) |

## Llaves de identidad

- Llaves de registro: `clientes:cliente_id`, `transacciones:tx_id`, `pqrs:radicado` (atributo XML), `empleados:id`.
- Usuario de chat → cliente: `users.name` = parte local del correo en el maestro (148/149).
- Usuario de chat → empleado: correo `@kipupay.co`.
- `pqrs.agente` = usuario de Slack del empleado. `empleados.reporta_a` = nombre corto del jefe.
- `entity_id` estable: `p_cl_<num>`, `p_kp_<num>`, `p_t_<hash de contexto>`, `o_<slug>` para conservados.

## Dónde vive la PII (más allá del texto obvio)

| Fuente  | Lugares no obvios                                                                                     |
| ------- | ----------------------------------------------------------------------------------------------------- |
| Chats   | `files[].name` (nombre, placa, `CC<cédula>`), URLs con usuario, mensajes editados, usuario borrado identificable por el texto |
| Docs    | PDF Author/Title/Subject/Keywords; comentarios HTML; bloques `display:none`; `mailto:`; front matter (handles, correos, author); rutas de imagen; URLs de Loom; `filename` |
| Tablas  | `transacciones.descripcion` (MAYÚSCULAS sin tildes, celulares, "A. Marulanda"); atributos `cedula` y `agente` en pqrs; `reporta_a` |

## PII disfrazada

- La cédula "de ejemplo" del manual es real (CL-13198). El teléfono "de talento" es el celular de una empleada.
- En el memo a inversionistas y el contrato con Veloz están el celular y la cédula del CEO.
- La minuta de ingeniería trae la dirección de una clienta que no se nombra (CL-15468).
- Hay 2 fechas en formato US (`DOB 01/08/1993` = 8 de enero) y 18 fechas de nacimiento en palabras.
- Las respuestas de solo dígitos solo se clasifican mirando la pregunta anterior del hilo.
- Unos 54 mensajes tienen letras acentuadas borradas ("cdula"): rompen las regex basadas en palabras clave.

## Falsos positivos y trampas de apellido

- Hay que conservar: Bancolombia, Davivienda, Colpatria, Nequi, Daviplata, PSE, DataCrédito, Superfinanciera/SFC,
  SIC, DIAN, Veloz, Carulla, Éxito, Juan Valdez, Avianca, Nutresa, Rappi, SURA, EAFIT, Shakira, Nairo Quintana,
  Egan Bernal, García Márquez, Jorge Isaacs, Plaza Botero, Parque Lleras, Estación Restrepo, El Poblado, y los ids
  CL-/KP-/PQR-/INC-/S-/TX-.
- Apellidos que también son lugar o marca y nombran personas: Santander, Bolívar, Botero, Carulla, Castilla, Robledo,
  Restrepo, Galán, Córdoba. A veces el lugar y la persona están en la misma frase.
- Palabras comunes que coinciden con nombres de pila: "Claro" (39 veces), "clara", "Celeste".
- Un regex de cédula sin contexto atrapa montos (1.480.000), el NIT 901.456.789-3 y números de crédito.

## Decisiones técnicas que salen de aquí

1. PDF: **pdfplumber** para el cuerpo (con tamaño de letra para encabezados y `extract_tables`), pypdf para la metadata.
2. Ocupación → 13 sectores, aplicada a todas las filas; barrio → otro barrio de la misma ciudad.
3. `reporta_a` → seudónimo corto del jefe, el mismo que se usa en el organigrama.
4. `title` del documento: el título de la metadata, con el H1 si falta, anonimizado. Lo de quitar el año es solo una
   hipótesis.
5. Si un nombre de pila suelto no está precedido de un nombre o rol, solo se reemplaza cuando el contexto del hilo
   o del documento lo ata a una persona conocida.
