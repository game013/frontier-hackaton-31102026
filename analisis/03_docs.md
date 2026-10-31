# 03 · Meta-análisis de documentos (fuentes/docs, 26 docs)

Base: `docs/ENUNCIADO.md`, `sample/` (sdoc_01), `fuentes/docs/*`, cruzado con `fuentes/tables/{clientes.csv,empleados.xml,pqrs.xml,transacciones.csv}` y `fuentes/chats/users.json`. Scripts en el scratchpad (`pdfx.py`, `lookup.py`, `q*.py`).

Ojo: `index.json` no viene ordenado (`doc_13` va antes de `doc_12`). Hay que recorrerlo por `id` y no suponer el orden.

---

## 1. Extracción

### PDF (11): todos salen del mismo generador (fpdf, "Kipu Docs 1.4", PDF 1.3)

- Todo es Helvetica/Helvetica-Bold y WinAnsi, con un único stream Flate por página. No hay XMP, `/Producer`, anotaciones, enlaces, OCG/capas, adjuntos embebidos, TOC, texto blanco, fuentes diminutas ni texto fuera de página. Tampoco aparece el operador `Tr` (texto invisible). Revisé los spans de PyMuPDF: el color siempre es 0 y los tamaños son solo 16, 13, 10,5 y 9.
- Jerarquía por tamaño de fuente: **16 bold = H1**, **13 bold = H2**, **10,5 = cuerpo**, **9 bold = cabecera de tabla**, **9 = celdas**. Las cartas (mora, paz y salvo, cobro jurídico) solo tienen cuerpo de 10,5.
- Solo `informe_riesgo_Q3_2026.pdf` tiene varias páginas (4). Los demás tienen 1.
- Hay tablas con rejilla en 9 de los 11 PDF (todos salvo carta_mora y paz_y_salvo).

| Librería | Texto corrido | Tablas | Veredicto |
|---|---|---|---|
| pypdf 6.x | Bien | Une las filas en una línea, pero **pega las celdas partidas sin espacio**: "menosefectivo", "de356.000", "previoaviso", "noreconocida" | Sirve para metadata (`reader.metadata`) |
| **pdfplumber 0.11** | Igual que pypdf | `extract_tables()` devuelve las celdas exactas (los saltos de línea quedan dentro de la celda como `\n`). En KP-022, `extract_text` desordena las celdas partidas | **Recomendado**: `extract_tables` para tablas, más el texto de las palabras fuera del bbox de las tablas, más `chars[].size/fontname` para H1/H2 |
| PyMuPDF 1.28 | Correcto, pero en tablas da una celda por línea y en texto justificado (vendor) una palabra por línea | `find_tables()` disponible | Buena alternativa (`get_text('dict')` trae el tamaño de fuente). Licencia AGPL |

**Recomendación:** pdfplumber (más pypdf para el diccionario Info). Hay que reconstruir el markdown así: `# ` para 16 pt, `## ` para 13 pt, tablas → `| a | b |` uniendo los `\n` de cada celda con un espacio, y el resto como párrafos. Los nombres pueden quedar partidos entre líneas o celdas, así que la detección debe correr **sobre el markdown final**, no sobre el texto crudo de pypdf.

#### Metadata de PDF (Info dict; no hay XMP)

| Archivo | /Title | /Author | /Subject | /Keywords | /CreationDate |
|---|---|---|---|---|---|
| politica_cobranza_v3 | Política de cobranza v3 | **Amalia Aristizábal** | Política interna - uso restringido | – | 2026-08-13 |
| vendor_agreement_veloz_2026 | Courier Services Agreement | Legal Team | Courier agreement Kipu Pay / Mensajeria Veloz - contact **amalia.aristizabal@kipupay.co** | – | 2026-08-25 |
| informe_pqr_septiembre_2026 | Informe de PQR, septiembre 2026 | **Fernanda Londoño** | PQR mensual | – | 2026-09-12 |
| carta_mora_luis_santander | Carta de mora - **Luis Santander** | **Milton Rojas** | – | – | 2026-09-24 |
| informe_riesgo_Q3_2026 | Informe trimestral de riesgo Q3 2026 | **Pamela Castaño** | Informe de riesgo para comité y junta | – | 2026-10-18 |
| evaluacion_desempeno_KP-009_2026 | Evaluación de desempeño 2026 | **Fernanda Londoño** | – | **Carlos Robledo**, soporte, evaluación 2026 | 2026-10-12 |
| evaluacion_desempeno_KP-022_2026 | Evaluación de desempeño 2026 - Soporte | **Fernanda Londoño** | Evaluación de desempeño anual | **Brayan Zapata**, soporte, Cali, evaluación 2026 | 2026-10-30 |
| paz_y_salvo_martin_jimenez | Paz y salvo crédito 4471-0211 | **Emilia Arango** | Paz y salvo crédito 4471-0211 | – | 2026-11-17 |
| informe_pqr_agosto_2026 | Informe de PQR, agosto 2026 | **Fernanda Londoño** | PQR mensual | PQR, soporte, agosto 2026, SFC | 2026-12-05 |
| security_report_Q3_2026 | Q3 2026 Security Report | Information Security | Quarterly review - owner **julian.henao@kipupay.co** | security, incidents, Q3 2026 | 2026-12-23 |
| carta_cobro_juridico_esteban_dario_mejia | Carta de cobro jurídico | Oficina jurídica externa | Cobro jurídico obligación 4471-0150 | cobro jurídico, mora, acuerdo de pago | 2027-01-04 |

`/Creator` = "Kipu Docs 1.4" en los 11. `/Producer` y `/ModDate` no existen (vacíos). Algunas `CreationDate` son futuras respecto a la fecha de la carta, pero no son PII. Las claves presentes varían por PDF: Subject y Keywords son opcionales.

### HTML (6)

Todos usan la misma plantilla: `<title>`, `<meta charset>`, `<meta name="author|description|generator">` (generator = "Kipu Docs 1.4") y un `<style>` inline. Ninguno tiene `alt`, `title=`, `data-*` ni imágenes.

| Archivo | `<title>` | meta author | meta description | Ocultos / comentarios / atributos |
|---|---|---|---|---|
| organigrama_2026 | Organigrama Kipu Pay 2026 | **Adrián Ospina** | Estructura organizacional vigente | 4 `mailto:` (href y texto) · `<!-- generado ... por adrian.ospina@kipupay.co el 2026-10-01 -->` |
| investor_memo_q3_2026 (en inglés, aunque `lang="es"`) | Q3 2026 Investor Memo | **David Mejía** | Quarterly update for Andes Capital | `<!-- draft reviewed by Pamela Agudelo (pagudelo@gmail.com) -->` |
| faq_clientes | Preguntas frecuentes | **Clara Muñoz** | Centro de ayuda Kipu Pay | `<div style="display:none">Borrador revisado por Clara Muñoz el 2026-09-30…</div>` · `<a href="mailto:isabel.diaz@kipupay.co">soporte</a>`: **el correo solo está en el href** |
| calibracion_calidad_2026-09 | Calibración de calidad, septiembre 2026 | **Nicolás Andrés Correa** | Calibración mensual de tickets | `<!-- exportado por nicolas.correa@kipupay.co desde kipu-qa el 2026-10-02 -->` |
| comunicado_validacion_chat | Comunicado a clientes: validación en el chat | Equipo legal | Comunicado a clientes | `mailto:emilio.torres@kipupay.co` (href y texto) · `<p>*Bogotá, 14 de octubre de 2026*</p>`: markdown literal dentro del HTML |
| plan_marketing_q4_2026 | Plan de marketing Q4 2026 | Marketing Kipu Pay | Plan trimestral | `<div style="display:none" class="aprobacion">Presupuesto aprobado por Norberto Gutiérrez</div>` |

### MD (9): front matter YAML; las claves cambian por documento

| Archivo | Claves del front matter | Comentarios HTML / imágenes / tablas |
|---|---|---|
| acta_comite_credito_2026-08 | title, date, author (`gerencia de riesgo`), revisado_por (`@emilia.arango`), status | 1 tabla |
| acta_comite_credito_2026-09 | title, date, author (`pamela.castano@kipupay.co`), status | 1 tabla |
| acta_comite_fraude_2026-10-09 | incidente, fecha, severidad, asistentes (`@luis.santander, @adela.sanchez, @julian.henao`). **Sin title** | 1 tabla |
| incidente_fraude_INC-2026-031 | incidente, severidad, reportado_por (`@luis.santander`), fecha. **Sin title** | 1 tabla |
| informe_auditoria_datos_2026-07 | title, date, auditora (`kmarulanda@hotmail.com`), alcance | 2 tablas |
| manual_onboarding_agentes | title, version (2.3), owner | `![Captura…](img/onboarding_camila_restrepo.png)`: nombre en la ruta de la imagen |
| minuta_ingenieria_2026-09-30 | title, date, author (`Bibiana Giraldo`), sprint (42), grabacion (`https://www.loom.com/share/ing-bibiana-giraldo-sprint42`) | 1 tabla · nombre dentro de la URL |
| minuta_producto_2026-10-07 | title, asistentes (`@clara.munoz, @adrian.ospina, @farid.diaz, @fernanda.londono`) | URL `loom.com/share/demo-celeste-moreno-onboarding-v2` |
| politica_teletrabajo_2026 | title, version (1.0), vigencia, owner | – |

Ningún MD tiene comentarios `<!-- -->`. Tampoco hay caracteres invisibles ni confusables en MD/HTML (lo verifiqué con unicodedata).

---

## 2. Derivación de los campos de salida

- **id / format / filename**: tal cual vienen de `index.json`. **filename se anonimiza como texto** si lleva nombre de persona (`carta_mora_luis_santander.pdf` → `carta_mora_<nombre_seud>.pdf`). Se conservan el formato (minúsculas, sin tildes, `_`) y la extensión. `KP-009`/`KP-022` y `veloz` no se tocan.
- **title**: en la muestra, el front matter y el H1 dicen "Memo cierre de julio 2026" y la salida dice "Memo cierre de julio". No sale del filename (`memo_cierre_julio` daría "Memo cierre julio", sin "de"). La explicación más probable es que el generador tiene un `title` interno y lo renderiza en el front matter con el año pegado, así que la verdad no se puede reconstruir de forma fiable. En el corpus real, `<title>`/`/Title`/front-matter **no** coinciden con el H1 (p. ej. "Informe de PQR, agosto 2026" frente a "Informe mensual de PQR - agosto de 2026"). Propuesta:
  1. Usar el título de metadata (MD `title` → HTML `<title>` → PDF `/Title`). Si no hay (acta_comite_fraude, incidente_fraude), usar el primer H1.
  2. Heurística opcional que replica la muestra: quitar un año final ` 20\d\d`. Solo afecta a "Organigrama Kipu Pay 2026" y "Evaluación de desempeño 2026" (en esos títulos el año va al final). Riesgo bajo, porque el título casi seguro solo pesa en el parecido de texto.
  3. **Anonimizar** el título: "Carta de mora - Luis Santander".
- **body** (markdown):
  - MD: el archivo **íntegro y verbatim** (front matter incluido), con sustituciones. Es exactamente lo que hace la muestra.
  - HTML: conversión a markdown (h1/h2 → `#`/`##`, `p`, `ul`/`ol`, tablas → pipe tables, `<a href="mailto:x">t</a>` → `[t](mailto:x)`, `<strong>` → `**`). **Conservar los comentarios `<!-- -->` literales** y **el texto de los `display:none`** (como HTML crudo `<div style="display:none">…</div>` o como párrafo), anonimizados. No escapar el `*…*` que ya trae el comunicado. Descartar `<style>` y `<head>`.
  - PDF: markdown reconstruido con pdfplumber (H1/H2 por tamaño, tablas en pipe, párrafos uniendo líneas). En el informe de riesgo de 4 páginas basta con concatenar las páginas.
- **metadata** (el validador compara el conjunto de claves con la "entrada normalizada" del evaluador, que no conocemos; si falla, solo sale en el detalle y la corrida se puntúa igual):
  - MD: **todas las claves del front matter**, con valores en texto crudo (no tipar `2026-08-28` como fecha ni `2.3` como float). Es exactamente lo que hace la muestra.
  - HTML: `title`, `author`, `description`, `generator` (los `<meta name>` más `<title>`). Ninguno tiene `keywords`.
  - PDF: claves en minúscula sin `/`, solo las presentes: `title`, `author`, `subject`, `keywords`, `creator`, `creationdate`. Duda: ¿`creation_date`? ¿Excluir fechas y creator? El enunciado menciona "author, subject, keywords", así que lo mínimo seguro es `title, author, subject, keywords, creator`. Conviene decidirlo con un test contra `/anonymize` o el validador si dan pista.
  - En todos los casos, los valores con PII se seudonimizan con el **mismo** seudónimo que en el body (author por nombre, emails por email del seudónimo y mismo dominio: `kelly.castano@kipupay.co` → `doris.lozano@kipupay.co`). También van seudonimizados los handles `@nombre.apellido` y la URL de Loom.
- El validador mira **todos** los campos (filename, title, metadata, claves incluidas) para la fuga, normaliza tildes y confusables, y compacta separadores entre dígitos (`+57 322 973 0911` cuenta como `3229730911`).

---

## 3. Inventario de PII por documento

Leyenda: E = empleado (KP-xxx en empleados.xml), C = cliente (CL-xxxxx), X = externo (no está en ninguna tabla), U = solo en chats/users.json.

### doc_01 organigrama_2026.html
| PII | Tipo | Dónde |
|---|---|---|
| Adrián Ospina / adrian.ospina@kipupay.co | E KP-013 | meta author, tabla, comentario HTML |
| David Mejía, Farid Díaz, Pamela Castaño, Fernanda Londoño + 4 correos | E | tabla (texto y `mailto:` href) |
| 31 nombres más en "Persona" y "Reporta a" (Luis Santander, Clara Montoya, Juan Esteban Torres, Milton Rojas, Celeste Moreno, Carlos Robledo, Isabel Díaz, Andrés Gómez, Fernanda Vargas, Farid Rodríguez, Clara Muñoz, Amalia Aristizábal, Emilio Torres, Norberto Gutiérrez, Nicolás López, Juana Rojas, David López, Lizeth Pérez, Yuliana Mejía, Laura Ospina, Brayan Zapata, Samuel Henao, Ivonne Cardona, Beatriz Osorio, Adela Sánchez, Emilia Arango, Sebastián Giraldo, Jerónimo Torres, Bibiana Giraldo, Nicolás Andrés Correa, Julián Henao) | E | tabla. **La relación "reporta a" debe sobrevivir**: mismo seudónimo en las columnas Persona y Reporta a |
| Homónimos de empleado con cliente: Luis Santander, Laura Ospina, Andrés Gómez, Sebastián Giraldo | E ≠ C | aquí siempre son los **empleados** |

### doc_02 politica_cobranza_v3.pdf
| PII | Tipo | Dónde |
|---|---|---|
| Amalia Aristizábal | E KP-014 | /Author |
| Bernardo Salazar (CL-61348) | C | §4 "Ejemplo real" (el CL- se conserva) |

### doc_03 acta_comite_credito_2026-09.md
| PII | Tipo | Dónde |
|---|---|---|
| pamela.castano@kipupay.co | E KP-003 | front matter author |
| David Mejía, Pamela Castaño, Norberto Gutiérrez, Milton Rojas | E | "Asistentes" |
| Juanita Hernández (CL-31549), auxiliar contable, ingresos 9,8 M | C | tabla S-2609-01 |
| Inés Pérez, "trabaja en Avianca" | C CL-79080 / U iperez | tabla S-2609-03 (Avianca se conserva) |
| Felipe Castilla, electricista independiente | U fcastilla (no está en clientes.csv) | tabla S-2609-05. **Castilla también es barrio** |
| "Cliente de Bucaramanga, arquitecto, vinculado desde 2021" | cuasi-id → Kevin Díaz Hoyos CL-25026 (único) | tabla S-2609-02 |
| CL-37777 (= Julieth Salazar, que sale nombrada con CC en doc_13) | C | tabla S-2609-04 |

### doc_04 vendor_agreement_veloz_2026.pdf (inglés)
| PII | Tipo | Dónde |
|---|---|---|
| David Mejía, ID No. **71.170.753** (= CC de KP-001) | E | cuerpo ×2 (partes y firma) |
| Jesús Sánchez, ID No. **71.772.071** | X (representante de Veloz) | cuerpo ×2 |
| amalia.aristizabal@kipupay.co | E KP-014 | /Subject, y en el cuerpo ×2 (texto y "(mailto:…)" impreso) |
| Se conserva: Kipu Pay S.A.S., **NIT 901.456.789-3** (tiene formato de cédula con puntos), dirección corporativa Cra 43A # 1-50 Torre 2 piso 9 El Poblado, Mensajería Veloz Ltda. | org | cuerpo |

### doc_05 investor_memo_q3_2026.html (inglés)
| PII | Tipo | Dónde |
|---|---|---|
| David Mejía | E KP-001 | meta author |
| david.mejia@kipupay.co, **+57 322 973 0911** (= celular de KP-001) | E | §Ask |
| Pamela Agudelo, pagudelo@gmail.com | X | comentario HTML |
| "Head of Risk, joined in 2021 from Bancolombia" | cuasi-id de empleado → Pamela Castaño (KP-003, ingreso 2021) | §Risk |
| "Customer A, ride-share driver in Bogotá… pre-approved after three months" | cuasi-id débil → CL-78497 Andrés Gómez Giraldo (vinculado en 2026-09, "crédito vehicular a los 3 meses"); hay 2 conductores en Bogotá | §Customers |
| "Customer B, a chef in the capital… payroll" | ambiguo (2 chefs en Bogotá; Lucía Giraldo, chef de Medellín, recibe nómina) | §Customers |

### doc_06 manual_onboarding_agentes.md
| PII | Tipo | Dónde |
|---|---|---|
| **1.036.371.441** | cédula real de C CL-13198 Brayan Gómez Vargas | "Ejemplo de conversación correcta" |
| `onboarding_camila_restrepo.png` | C CL-72216 Camila Restrepo Torres (`camila_restrepo` es el payload del test público pub_01) | ruta de la imagen |

### doc_07 informe_pqr_septiembre_2026.pdf
| PII | Tipo | Dónde |
|---|---|---|
| Fernanda Londoño | E KP-004 | /Author |
| **Daniela Ordóñez** ×2: PQR-0451 → **CL-56640** (Cali, enfermera); PQR-0467 → **CL-50124** (Barranquilla, contadora) | **homónimos C/C** | tabla. El desempate es el radicado → `pqrs.xml@cliente_id`. **Dos seudónimos distintos** en dos filas de la misma tabla |
| Mariana Carulla (PQR-0472) | C CL-65777 | tabla. En la misma fila, "Compra en **Carulla**" (marca) se conserva |
| Emilia Galán (PQR-0480) | C CL-98510 | tabla ("Estación Restrepo" es un lugar) |
| CL-34834 (PQR-0488) | C (Óscar Ramírez) | tabla, solo el id |

### doc_08 incidente_fraude_INC-2026-031.md
| PII | Tipo | Dónde |
|---|---|---|
| @luis.santander | E KP-005 (no el cliente) | front matter reportado_por |
| Juan Pablo Bolívar Henao, CC **1.020.316.175**, CL-60315, "Arquitecto, cliente desde 2022" | C | "Cliente afectado" |
| "chef de Medellín, cliente nuestro desde 2021" | cuasi-id → CL-13198 Brayan Gómez Vargas (único) | "Cuenta destino" |
| IP 190.24.118.77 | identificador técnico (opcional) | tabla |
| Se conserva: Santander (banco y departamento, ×2), Davivienda, Superfinanciera, Kipu | org | |

### doc_09 carta_mora_luis_santander.pdf
| PII | Tipo | Dónde |
|---|---|---|
| **luis_santander** | C CL-21823 | **filename** |
| "Carta de mora - Luis Santander" | C | /Title → title |
| Luis Santander Moreno, Cl 126B # 39-65, Bucaramanga, CC 91.338.232 | C | encabezado |
| "Estimado señor **Santander**" | apellido suelto de la persona | saludo |
| Milton Rojas, Analista de Cobranza | E KP-017 | /Author y firma |
| **Homónimo**: Luis Santander cliente (CL-21823, U lsantander) ≠ Luis Santander empleado (KP-005) | | |

### doc_10 faq_clientes.html
| PII | Tipo | Dónde |
|---|---|---|
| Clara Muñoz | E KP-018 | meta author y `div display:none` |
| isabel.diaz@kipupay.co | E KP-010 | solo en el `href="mailto:"` (el texto visible es "soporte") |
| Se conserva: Nequi, Daviplata, Bancolombia, **Juan Valdez** (marca con forma de nombre), Avianca, El Poblado, dirección de oficina, WeWork, Calle 100 | | |

### doc_11 minuta_producto_2026-10-07.md
| PII | Tipo | Dónde |
|---|---|---|
| @clara.munoz, @adrian.ospina, @farid.diaz, @fernanda.londono | E | front matter asistentes y cuerpo (handles) |
| "Adrián" ×2 (nombre suelto) | E KP-013 | Temas y Acuerdos |
| `demo-celeste-moreno-onboarding-v2` | E KP-008 | URL de Loom |
| "clienta en Miami" / "piloto comercial de Barranquilla" (ambiguo: programa piloto u ocupación; hay un piloto comercial en Barranquilla, CL-44101) | cuasi-id débil | Temas 1 y 3 |
| Se conserva: Rappi, Miami | | |

### doc_12 evaluacion_desempeno_KP-009_2026.pdf
| PII | Tipo | Dónde |
|---|---|---|
| Fernanda Londoño | E KP-004 | /Author y "Evaluadora" |
| Carlos Robledo | E KP-009 | /Keywords |
| Carlos Robledo Salazar (KP-009), **fecha de nacimiento 1995-12-14**, ingreso 2021 | E | "Evaluado" |
| "El agente **Robledo**" (apellido suelto, es la persona) vs "el stand que pusimos en **Robledo**" (barrio, se conserva) | trampa | Comentarios |
| "Vive en Itagüí… ingreso 2021… agente" | cuasi-id | Comentarios |

### doc_13 informe_riesgo_Q3_2026.pdf (4 páginas)
| PII | Tipo | Dónde |
|---|---|---|
| Pamela Castaño | E KP-003 | /Author |
| Julieth Salazar, CC **43.663.646** (= CL-37777) | C | §8, página 4 |
| El resto son cifras agregadas por ciudad: se conservan | | |

### doc_14 acta_comite_credito_2026-08.md
| PII | Tipo | Dónde |
|---|---|---|
| @emilia.arango | E KP-031 | front matter revisado_por |
| David Mejía, Pamela Castaño (×4), Norberto Gutiérrez (×2), Emilia Arango | E | Asistentes y Discusión |
| **Mateo Henao** (S-2608-01, 45 M, cuota inicial 1.200.000 del 25-ago) | **homónimo C/C** → CL-37620 (Bogotá, ing. civil) y no CL-35202. El desempate está en TX-000006 de transacciones.csv | tabla |
| Martín Jiménez (S-2608-03, crédito 4471-0211) | C CL-98782 | tabla |
| "contadora de Pereira, vinculada en 2023" (S-2608-02) | cuasi-id → CL-78641 Alicia Mejía Jiménez | Discusión |
| CL-37214 | C | tabla |
| Se conserva: Nutresa, Fondo Nacional de Garantías, sala Guatapé | | |

### doc_15 evaluacion_desempeno_KP-022_2026.pdf
| PII | Tipo | Dónde |
|---|---|---|
| Fernanda Londoño | E | /Author y Evaluadora |
| Brayan Zapata | E KP-022 | /Keywords |
| Brayan Zapata Gutiérrez (KP-022), Documento **1.107.444.694**, ingreso 2025 | E | encabezado |
| "El agente **Zapata**" | apellido suelto | Comentarios |
| "vive en **Granada**, a diez minutos del coworking de Cali" + 2025 + agente | cuasi-id (barrio, ciudad, año) | Comentarios |
| "agente de Barranquilla en su proceso de inducción" | refuerza el cuasi-id de Yuliana Mejía | Plan 2027 |
| Se conserva: Éxito de San Fernando, PSE | | |

### doc_16 calibracion_calidad_2026-09.html
| PII | Tipo | Dónde |
|---|---|---|
| Nicolás Andrés Correa / nicolas.correa@kipupay.co | E KP-029 | meta author, cuerpo y comentario HTML |
| Farid Díaz | E | Aprueba |
| Agentes: Laura Ospina (fila 1), Ivonne Cardona, Yuliana Mejía, Brayan Zapata, Samuel Henao | E | columna Agente |
| Clientes: Gabriela Londoño (CL-45343), Ricardo Martínez (CL-37920), Lucía Giraldo (CL-93152), **Laura Ospina (fila 5, = CL-69452)** | C | columna Cliente |
| **Agente y clienta con el mismo nombre en la misma tabla**: Laura Ospina KP-021 (fila 1, agente) ≠ Laura Ospina CL-69452 (fila 5, cliente; desempate por PQR-0546 del 2026-08-13, agente samuel.henao) | homónimo E/C | tabla |

### doc_17 acta_comite_fraude_2026-10-09.md
| PII | Tipo | Dónde |
|---|---|---|
| @luis.santander, @adela.sanchez, @julian.henao | E | front matter asistentes |
| Luis Santander (coordinador, **E**), Adela Sánchez, Julián Henao, Pamela Castaño | E | Asistentes |
| Ignacio Zapata Ospina, CC **91715263** (sin puntos) | C CL-74990 | Caso |
| CL-30787 / PQR-2026-0548 + "ingeniera de sistemas, vinculada en 2023… Bucaramanga" | cuasi-id (Nathalia Quintero) | Antecedentes |
| Se conserva: Colpatria, Pereira, Bucaramanga | | |

### doc_18 paz_y_salvo_martin_jimenez.pdf
| PII | Tipo | Dónde |
|---|---|---|
| **martin_jimenez** | C CL-98782 | **filename** |
| Martín Jiménez Moreno, CC 70.699.295, Medellín | C | encabezado |
| Emilia Arango | E KP-031 | /Author y firma |
| crédito 4471-0211, S-2608-03 (enlaza con doc_14 y PQR-0547) | ids de negocio | /Title, /Subject, cuerpo |

### doc_19 informe_auditoria_datos_2026-07.md
| PII | Tipo | Dónde |
|---|---|---|
| kmarulanda@hotmail.com | X | front matter auditora |
| Kelly Marulanda, socia | X (la misma persona que el correo) | cuerpo |
| Amalia Aristizábal, abogada senior | E | cuerpo |
| CL-37214 | C | Hallazgo 3 |
| Se conserva: Auditores Andinos, Ley 1581, Kipi | | |

### doc_20 comunicado_validacion_chat.html
| PII | Tipo | Dónde |
|---|---|---|
| emilio.torres@kipupay.co | E KP-032 | `mailto:` href y texto |
| meta author "Equipo legal" | no es persona | |

### doc_21 informe_pqr_agosto_2026.pdf
| PII | Tipo | Dónde |
|---|---|---|
| Fernanda Londoño | E | /Author y "Elaboró" |
| **Juliana Mejía** ×2: PQR-0537 → **CL-67191** (Medellín, periodista); PQR-0541 → **CL-50577** (Cali, psicóloga) | **homónimos C/C** | tabla. El desempate es el radicado |
| Rodolfo Torres (PQR-0540) | C CL-64103 | tabla |
| CL-19162 (PQR-0539), "Cliente web, Bogotá" (PQR-0546 = Laura Ospina, cliente) | C | tabla |
| Se conserva: Éxito, Defensoría del Consumidor Financiero, PSE | | |

### doc_22 minuta_ingenieria_2026-09-30.md
| PII | Tipo | Dónde |
|---|---|---|
| author: Bibiana Giraldo; grabacion `…/ing-bibiana-giraldo-sprint42` | E KP-028 | front matter (y por tanto metadata) |
| Bibiana Giraldo, Jerónimo Torres, Farid Díaz, Nicolás Andrés Correa | E | Asistentes |
| Nombres sueltos: Jerónimo ×2, Sebastián ×2 (= KP-026, no el cliente CL-66397), Nicolás Andrés ×2, Bibiana, Farid | E | cuerpo y tabla Acuerdos |
| Elvira Hoyos | C CL-71712 | §3 |
| **Diagonal 139A # 41-38 of 1006**: "la dirección de otra clienta" | dirección de C CL-15468 Alejandra Flórez Montoya, que no se nombra | §3 |
| Se conserva: Nequi | | |

### doc_23 plan_marketing_q4_2026.html
| PII | Tipo | Dónde |
|---|---|---|
| Nicolás López, Norberto Gutiérrez | E | cuerpo y `div display:none` |
| Alejandra Rojas, "@arojas", periodista independiente | C CL-83915 / U arojas | §Aliada de contenido (el handle = usuario de Slack) |
| "clienta de Cali desde 2021… community manager" | cuasi-id → CL-79476 Paola Pérez Ospina (única) | §Testimonio |
| Se conserva: Nairo Quintana, Boyacá, "María" (la novela) | | |

### doc_24 security_report_Q3_2026.pdf (inglés)
| PII | Tipo | Dónde |
|---|---|---|
| julian.henao@kipupay.co | E KP-030 | /Subject |
| Julián Henao (Security Lead), Farid Díaz (CTO) | E | "Prepared/Approved by" |
| **nicolas.correa@kipupay.co** ("employee's real corporate email") | E KP-029 | Internal Finding |
| Se conserva: SIC, Superfinanciera, INC-2026-031/038 | | |

### doc_25 politica_teletrabajo_2026.md
| PII | Tipo | Dónde |
|---|---|---|
| Lizeth Pérez, David Mejía | E | cuerpo |
| **+57 302 721 6839** (se presenta como teléfono "de talento", pero es el celular de KP-033 Lizeth Pérez, 3027216839) | E | §Dudas |
| "agente de soporte en Barranquilla, con nosotros desde 2024… su casa en El Prado" | cuasi-id → KP-020 Yuliana Mejía | §Casos especiales |
| Se conserva: SURA, ARL | | |

### doc_26 carta_cobro_juridico_esteban_dario_mejia.pdf
| PII | Tipo | Dónde |
|---|---|---|
| **esteban_dario_mejia** | C CL-98225 | **filename** |
| Esteban Darío Mejía Castaño, Av 87 # 36-81, Bogotá, CC 79.183.480 | C | encabezado |
| Hernando Gómez, hgomez@gmail.com, abogado externo | X | firma |
| Se conserva: DataCrédito, "Oficina jurídica externa" (/Author) | | |

---

## 4. Cuasi-identificadores y homónimos

**Cuasi-identificadores** (generalizar: barrio → ciudad, ocupación → sector, año → "hace unos años"; no borrar). El evaluador cruza contra la fila de clientes.csv, así que la tabla también debe quedar generalizada y coherente.

| Doc | Combinación | Apunta a |
|---|---|---|
| doc_14 | contadora + Pereira + 2023 | CL-78641 |
| doc_03 | arquitecto + Bucaramanga + 2021 | CL-25026 |
| doc_08 | chef + Medellín + 2021 | CL-13198 |
| doc_08 | arquitecto + 2022 (ya nombrado) | CL-60315 |
| doc_17 | ingeniera de sistemas + 2023 + Bucaramanga | CL-30787 |
| doc_23 | community manager + Cali + 2021 | CL-79476 |
| doc_05 | ride-share driver + Bogotá (+3 meses); chef + capital | CL-78497? / ambiguo |
| doc_05 | Head of Risk + 2021 + ex-Bancolombia | KP-003 |
| doc_25 | agente + Barranquilla + 2024 + El Prado | KP-020 |
| doc_15 | agente + Granada/Cali + 2025 | KP-022 |
| doc_12 | agente + Itagüí + 2021 + fecha de nacimiento | KP-009 |
| doc_11 | piloto comercial + Barranquilla (ambiguo) | CL-44101? |

**Homónimos que tocan documentos** (el desempate vive en otra fuente):
- Daniela Ordóñez (doc_07): por radicado → pqrs.xml.
- Juliana Mejía (doc_21): por radicado → pqrs.xml.
- Mateo Henao (doc_14): por solicitud + monto + fecha → transacciones.csv.
- Laura Ospina (doc_16, agente y clienta en la misma tabla; doc_01 = agente).
- Luis Santander (doc_09 = cliente; doc_01/08/17 = empleado).
- Andrés Gómez y Sebastián Giraldo (doc_01/22 = empleados; también existen como clientes).

**Casi-homónimos con nombre de pila compartido**: hay dos Fernanda, dos Farid, dos David y dos Nicolás entre los empleados, y Brayan Zapata (empleado) frente a Brayan Gómez (cliente). Un nombre suelto se resuelve por contexto (p. ej. "Farid… líder del área" = Díaz).

## 5. Falsos positivos que hay que conservar

- **Reguladores / sector**: Superfinanciera / Superintendencia Financiera / SFC, SIC, DataCrédito, Defensoría del Consumidor Financiero, Fondo Nacional de Garantías, Ley 1266/1581, ARL.
- **Bancos y pagos**: Bancolombia, Davivienda, Colpatria, Santander (banco), Nequi, Daviplata, PSE.
- **Marcas y empresas**: Kipu Pay, Kipi (bot), Kipu Core/Docs, **Veloz / Mensajería Veloz Ltda.** (proveedor), Nutresa, Avianca, Éxito, **Carulla**, **Juan Valdez**, Rappi, WeWork, SURA, Loom, Andes Capital, Auditores Andinos, "Oficina jurídica externa", "Equipo legal", "Legal Team", "Information Security".
- **Lugares**: Medellín, Bogotá, Cali, Barranquilla, Bucaramanga, Pereira, Itagüí, Boyacá, Santander (departamento), Miami, Portugal, El Poblado, San Fernando, Calle 100, **Estación Restrepo**, **Robledo** (barrio, en "stand en Robledo"), Granada, sala Guatapé, sala Botero.
- **Figuras públicas y obras**: Shakira, Nairo Quintana, "María" (novela).
- **Datos de la empresa**: NIT 901.456.789-3 y la dirección Cra 43A # 1-50 Torre 2 piso 9.
- **Ids de negocio** (se conservan como en la muestra): CL-/KP-/PQR-/INC-/S-26xx-/TX-, y créditos 4471-xxxx (unen con transacciones).

## 6. Documentos en inglés

El enunciado dice "2", pero el contenido en inglés aparece en **3** documentos:
- `security_report_Q3_2026.pdf`
- `vendor_agreement_veloz_2026.pdf`
- `investor_memo_q3_2026.html`, que declara `lang="es"`. Quizá por eso cuentan dos.

Qué cambia en ellos:
- Marcadores en inglés: "Prepared by", "Approved by", "represented by its CEO, David Mejía, identified with ID No. 71.170.753", "For the Client/Provider:", "Contact:", "owner", "contact", "draft reviewed by". Hay que tenerlos en los patrones de NER y de regex.
- Formato de fechas en inglés ("October 6, 2026") y de números con coma de miles (9,800).
- Los cuasi-identificadores también vienen en inglés ("ride-share driver in Bogotá", "chef in the capital", "Head of Risk… joined in 2021 from Bancolombia").
- Teléfono internacional con espacios: `+57 322 973 0911`.
- Los nombres siguen siendo colombianos. El seudónimo debe ser un nombre colombiano del mismo género, no anglosajón.

## 7. Trampas y casos raros

1. **Apellido que es también lugar o marca**: Santander (persona / banco / departamento en doc_08, doc_09); Robledo (agente / barrio en el mismo párrafo de doc_12); Carulla (clienta / supermercado en la misma fila de doc_07); Castilla (cliente Felipe / barrio); Restrepo (Camila / Estación); Galán (Emilia / barrio Galán en clientes.csv); Bolívar (Juan Pablo / Seguros Bolívar en tablas).
2. **Apellido o nombre suelto** que remite a una persona ya vista: "señor Santander", "agente Robledo", "agente Zapata", "Adrián", "Jerónimo", "Sebastián", "Nicolás Andrés", "Bibiana", "Farid". Hay que reemplazarlo por la parte correspondiente del seudónimo.
3. **Nombres en tokens no textuales**: filename (`luis_santander`, `martin_jimenez`, `esteban_dario_mejia`), ruta de imagen (`camila_restrepo`), slugs de URL con guiones (`bibiana-giraldo`, `celeste-moreno`), handles (`@luis.santander`, `@arojas`), correos `nombre.apellido@kipupay.co` e `inicial+apellido@gmail.com`.
4. **PII solo en markup**: `mailto:` con texto "soporte" (doc_10), comentarios HTML (doc_01/05/16), `display:none` (doc_10/23), front matter y metadata de PDF (Title, Keywords, Subject). Si un extractor descarta comentarios o divs ocultos pierde texto. Si los conserva sin anonimizar, fuga.
5. **Cédulas en formatos distintos**: con puntos (`1.020.316.175`, `43.663.646`), sin puntos (`91715263`), precedidas de "CC", "Documento:" o "ID No.". También aparecen dentro de un ejemplo didáctico (doc_06), y ese ejemplo es una cédula real.
6. **El regex de cédula da falsos positivos** con el NIT `901.456.789-3`, con montos (`3.184.600`, `1.480.000`) y con números de crédito `4471-0150`. Conviene exigir un contexto (CC/cédula/documento/ID) o cruzar contra el maestro.
7. **Dirección de un tercero no nombrado** (doc_22): pertenece a CL-15468 y hay que seudonimizarla como dirección de esa persona. La dirección corporativa se conserva.
8. **Teléfono de una persona presentado como teléfono de área** (doc_25) y teléfono del CEO en el memo (doc_05).
9. **Externos sin fila en ninguna tabla**: Hernando Gómez, Kelly Marulanda, Pamela Agudelo y Jesús Sánchez. Necesitan entidad y seudónimo propios. Felipe Castilla solo está en users.json.
10. **Celdas partidas en PDF**: con pypdf, un nombre partido entre líneas de una celda saldría pegado. Hay que usar las tablas de pdfplumber y detectar sobre el markdown final.
11. **Markdown literal dentro de HTML** (`*Bogotá…*`) y `lang="es"` en un documento en inglés.
12. **Front matter con valores no-string** (version 2.3, sprint 42, fechas): hay que serializarlos como texto crudo para que no cambien (p. ej. `1.0` → `1`).
13. `index.json` está desordenado (doc_13 antes que doc_12).
