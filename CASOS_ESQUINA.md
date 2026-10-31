# Casos esquina

Catálogo vivo. Cada caso es una regresión que mide el juez propio (`cmd/juez`, `DISEÑO.md` §8.1). Un caso en rojo
bloquea la corrida oficial.

## Cómo agregar un caso

1. Llega de la cola de dudosos, de una captura de `/anonymize`, de una corrida oficial o de leer el corpus.
2. Agrega una fila con el siguiente id libre, el ejemplo por **id de registro** (sin la forma real en este archivo)
   y el resultado esperado.
3. Asígnale un juez de la tabla de jueces. Si ninguno lo mide, crea un juez nuevo antes de corregir el pipeline.
4. Corrige el pipeline y deja el caso en verde en el mismo commit.

## Jueces

| Juez | Familia            | Qué comprueba                                                                                      | Peso jurado |
| ---- | ------------------ | -------------------------------------------------------------------------------------------------- | ----------- |
| J-DIR | Fuga directa      | El dato no aparece en su registro ni en otro, ni como hash sin sal, bajo `contiene()`              | 3 / 35      |
| J-OCU | Fuga oculta       | El dato no aparece en metadata, filename, adjunto, front matter, comentario HTML o clave           | 2 / 35      |
| J-IND | Cuasi-identificador | Ocupación, barrio y año no co-ocurren en el registro ni en su fila del maestro (`join_records`)  | 1 / 35      |
| J-GLO | Barrido global    | Ninguna forma de `originals` aparece en ninguna parte de la salida                                 | 10          |
| J-FP  | Falso positivo    | La entidad conservada sigue intacta en su registro                                                 | 9           |
| J-CAT | Categoría         | Nombre del mismo género, correo válido, cédula de formato válido y lejos del original              | 8           |
| J-EST | Estructura        | Los joins unen las mismas filas; conteo de registros por persona; quién habla con quién por hilo   | 8           |
| J-PAR | Parecido          | Texto no personal igual al ideal (muestra) o byte a byte fuera de los fragmentos (corpus)          | 10          |
| J-CON | Consistencia      | Con `lookup_forms`, un solo seudónimo presente en todos los registros de la persona                | 8           |
| J-HOM | Homónimos         | Dos personas con el mismo nombre reciben dos seudónimos distintos                                  | 5           |
| J-REP | Reporta a         | `reporta_a` resuelve al seudónimo del jefe correcto                                                | 4           |
| J-DIC | Diccionario       | `entities.json` declara todas las formas reales de cada persona                                    | 3           |
| J-CRE | Crecimiento       | Con 80 % y luego 100 % de las fechas, ningún seudónimo viejo cambia                                | multiplica  |
| J-REC | Cobertura         | Toda persona de la verdad (muestra) es detectada y seudonimizada; meta 100 % (D34)                 | evita ~24   |
| J-VIV | En vivo           | `/anonymize` da el mismo perfil que `output/`, persona nueva única, homónimo nuevo separado, < 60 s | contrato   |

## Casos

| Id    | Caso                                              | Ejemplo (registro)                               | Esperado                                                   | Juez         |
| ----- | ------------------------------------------------- | ------------------------------------------------ | ---------------------------------------------------------- | ------------ |
| CE-01 | Homónimos cliente–cliente                          | 2 pares en `clientes` (ver `analisis/04`)        | Dos seudónimos; desempate por PQR, fecha + monto o adjunto | J-HOM, J-CON |
| CE-02 | Agente con el nombre de un cliente en el mismo hilo | KP-021 y CL-69452                               | Autor por uid; menciones por rol, "soy X" o handoff        | J-HOM, J-EST |
| CE-03 | MAYÚSCULAS sin tildes                              | `transacciones.descripcion`                      | Reemplazo en MAYÚSCULAS sin tildes                         | J-DIR, J-GLO |
| CE-04 | Correo deletreado ("arroba", "punto")              | 8 mensajes en chats                              | Correo falso deletreado en el mismo estilo                 | J-DIR, J-CAT |
| CE-05 | Metadata de PDF (Author, Keywords)                 | PDFs de `docs/`                                  | Clave presente con seudónimo                               | J-OCU        |
| CE-06 | Comentario HTML y bloque `display:none`            | HTML de `docs/`                                  | Comentario conservado y anonimizado                        | J-OCU        |
| CE-07 | Front matter con correo o handle                   | MD de `docs/`                                    | Clave presente con seudónimo                               | J-OCU        |
| CE-08 | Nombre o `CC<cédula>` en adjunto                   | `files[].name` en chats                          | Filename anonimizado como texto                            | J-OCU        |
| CE-09 | Cédula "de ejemplo" que es real                    | Manual (CL-13198)                                | Reemplazada aunque no haya palabra clave                   | J-DIR        |
| CE-10 | Celular o cédula del CEO en memo y contrato        | Memo a inversionistas, contrato Veloz            | Reemplazados                                               | J-DIR        |
| CE-11 | Dirección de clienta no nombrada                   | Minuta de ingeniería (CL-15468)                  | Dirección falsa de esa persona                             | J-DIR, J-CON |
| CE-12 | Cuasi-identificador sin nombre                     | 80e0b943 → CL-50124                              | Sector, ciudad y "hace unos años" en el chat; sector y barrio alterno en la fila del maestro | J-IND        |
| CE-13 | Plantilla "soy X, vivo en Y, desde Z"              | af9ccd6e, 8e61cf6b, 9cb80452; variante «llevo con ustees desde» (así en el corpus) | Generalizado en texto y fila; «ustees» es interlocutor igual que «ustedes» | J-IND        |
| CE-14 | Apellido que es lugar o marca                      | d40d8c0a (Santander), 594351d5 (Bolívar)          | Persona reemplazada; lugar o marca intactos                | J-DIR, J-FP  |
| CE-15 | Nombre común que es palabra ("Claro")              | 39 apariciones                                   | "Claro" intacto; la empleada sí se reemplaza               | J-FP         |
| CE-16 | Respuesta de solo dígitos                          | Tras "¿me confirmas tu cédula?"                  | Clasificada por la pregunta anterior y reemplazada         | J-DIR        |
| CE-17 | Usuario de chat borrado                            | U07HF5JVK → CL-58402                             | Identificado por la cédula del texto                       | J-CON, J-DIC |
| CE-18 | Persona fuera del maestro                          | CL-55481                                         | Entidad con llave `p_cl_55481`                             | J-CON, J-DIC |
| CE-19 | Tercero sin fila                                   | Notas y chats (ver `analisis/02`)                | Entidad `p_t_`, seudónimo único y persistido               | J-DIR, J-CON |
| CE-20 | Documento en inglés                                | Security report, contrato Veloz                  | Mismos detectores; nombres colombianos falsos              | J-DIR, J-CAT |
| CE-21 | Fecha de nacimiento en formato US o en palabras    | `DOB 01/08/1993`; 18 en palabras                 | Fecha falsa en el mismo formato                            | J-DIR        |
| CE-22 | Letras acentuadas borradas ("cdula")               | ~54 mensajes                                     | Detección sin depender de la palabra clave                 | J-DIR        |
| CE-23 | Montos, NIT y números de crédito parecidos a cédula | 1.480.000; NIT de Kipu Pay                      | Intactos                                                   | J-FP, J-PAR  |
| CE-24 | Comentario XML con correo de empleado              | Cabecera de `pqrs.xml`                           | No se reemite o se anonimiza                               | J-OCU        |
| CE-25 | "Reporta a" con nombre corto                       | `empleados.reporta_a`                            | Seudónimo corto del jefe                                   | J-REP        |
| CE-26 | Persona vieja con un identificador nuevo           | Tras el crecimiento                              | Mismo seudónimo                                            | J-CRE, J-VIV |
| CE-27 | Persona nueva con el nombre de una vieja           | Tras el crecimiento y en `/anonymize`            | Seudónimo nuevo                                            | J-HOM, J-VIV |
| CE-28 | Empate sin LLM disponible                          | `/anonymize` con timeout                         | Persona nueva marcada dudosa, sin fusión                   | J-HOM, J-VIV |
| CE-29 | Nombre en minúsculas en un chat                    | "juan perez" sin mayúsculas                      | Candidato por diccionario (D30a); reemplazado en minúsculas | J-REC, J-DIR |
| CE-30 | Nombre tras disparador sin estar en el catálogo    | "la señora …", "Prepared by …"                   | Candidato por disparador (D30c); Jev lo clasifica persona  | J-REC, J-DIR |
| CE-31 | Nombre extranjero en documento en inglés           | Security report                                  | Candidato por mayúsculas (D30b); seudónimo de origen extranjero (D27) | J-REC, J-CAT |
| CE-32 | Nombre unisex con confianza baja de género         | "Alexis" (M 0,35–0,38)                           | Seudónimo unisex (D31)                                     | J-CAT        |
| CE-33 | Nombre de pila compartido que sobrevive            | 4 nombres de pila en `doc_22` (cliente y empleado) | Aceptado: D25 no barre nombres de pila sueltos; 8 formas declaradas siguen en la salida | J-GLO        |
| CE-35 | Fecha de negocio tomada como nacimiento            | Tabla `transacciones` (fecha 2026-09-08) y chats | La fecha de negocio se conserva; sin dueño no se crea persona ni se barre (289 fechas) | J-FP, J-PAR  |
| CE-36 | Placa que parte una palabra                       | Nombre de archivo con `sep2026`                  | `sep202` dentro de `sep2026` no es placa: se rechaza por bordes y por minúsculas | J-FP         |
| CE-37 | Persona nueva con formas sin indexar              | Persona añadida a mitad de corrida               | Sus formas entran al índice y se reemplazan (`AgregaForma`) | J-DIR, J-CON |
| CE-38 | Candidatos solapados de dos generadores           | Chat `1975b3e1-…` ("Soy " + nombre): disparador y mayúsculas | Un solo candidato por tramo, el del generador más preciso; con Jev la corrida no cae por ediciones solapadas | J-EST, J-DIR |
| CE-39 | Fecha de negocio detrás de un "desde" suelto            | «Kipu Pay opera desde 2020», «la empresa abrió desde 2019», «la resolución entró en vigencia en 2024» | El año se conserva; solo se generaliza con ancla de relación ("llevo con ustedes desde", "cliente desde") | J-PAR, J-FP  |
| CE-40 | Oficio y año sin verbo, en el acta de un comité            | «Antecedentes de la afectada: ingeniera de sistemas, vinculada en 2023» (`acta_comite_credito_2026-08`, `acta_comite_fraude_2026-10-09`) | Oficio → sector y año → «hace unos años»: el acta ya presenta a la persona («la afectada») y «vinculada en» ancla la relación | J-IND        |
| CE-41 | Barrio de una cosa tomado como barrio de la persona        | «el consultorio queda en Castilla», «la oficina en Buenos Aires», «la sede queda en Envigado» junto al oficio de alguien | El barrio se conserva (D1): solo se generaliza detrás de un verbo de vivienda («vivo en») o de trabajo en primera persona («trabajo cerca, en Granada», `sample_08`); un locativo de cosa no ata el lugar a nadie | J-FP, J-PAR  |
| CE-42 | Año de negocio tras un verbo de entrada y otra frase en medio | «La resolución entró en vigencia en 2024», «el radicado entró de nuevo en 2024» | El año se conserva: si entre el verbo de entrada y el año hay otro «en» o un «de» que une otra frase, el año es de negocio. Se acepta que «entró en la empresa en 2021» también se conserve | J-FP, J-PAR  |
| CE-43 | Año de la empresa tomado por un ancla en tercera persona | «Kipu Pay trabaja desde 2019», «la función está desde 2024», «el equipo de soporte trabaja desde 2020» | El año se conserva: un ancla en tercera persona solo vale con un sustantivo de persona en la ventana («la clienta está desde», «el arquitecto trabaja desde») o el puente «con nosotros/ustedes» | J-FP, J-PAR  |
| CE-44 | Año de la persona con artículo o con el orden invertido | «Soy cliente desde el 2021», «desde el año 2021», «Desde 2021 soy cliente» | El año va a «hace unos años»: el artículo («el», «del», «el año») entre «desde» y el año no lo vuelve fecha de negocio, y con el orden invertido la relación se busca hasta cuatro palabras detrás del año | J-IND, J-FP   |
| CE-34 | Test público `pub_04` pide dos entidades con una forma completa que solo tiene una fila | `fuentes/tests_public.json` `pub_04` | No se cumple declarando una forma que no es de la persona; el criterio propio es dos personas, dos seudónimos, separadas por uid | J-HOM        |

## Aceptados con motivo

Casos que el juez mide en rojo y que se aceptan a sabiendas, con el número y la razón. No bloquean la corrida oficial.

- **CE-33 · J-GLO 99,8 % sobre `fuentes/` (3883 de 3891 formas tras la ola 3; eran 3909 de 3917 antes
  de C19).** Quedan 8 formas de tipo `nombre_pila`
  declaradas en `entities.json` que siguen en el texto. Son 4 nombres de pila, cada uno compartido por dos personas:
  dos pares coinciden en `doc_22` (un cliente y un empleado con el mismo nombre de pila) y dos pares en 20 y 7
  mensajes de chats (dos empleados con el mismo nombre de pila). D25 dice que un nombre de pila suelto no se barre:
  la sustitución del nombre completo es la que hace irreversible la salida, y borrar cada aparición de un nombre de
  pila rompería el texto sin ganar nada (el jurado además exime esas formas cuando van en mayúscula y sin dígitos).
  El detector no las ve en la salida limpia porque ahí no son menciones con dueño. Criterio medible:
  `go run ./cmd/juez --salida … --publico fuentes → J-GLO 3883/3891`; lo que sí está en verde son las fugas duras
  (J-DIR y J-OCU 100 %) y J-DIC 100 %.
- **CE-34 · `pub_04` no es satisfacible sin mentir.** El test pide dos entidades que declaren exactamente la misma
  forma completa (nombre y dos apellidos). En el corpus esa forma completa la tiene una sola fila del maestro; la
  otra persona homónima es un segundo apellido distinto, y los dos usuarios de chat con el nombre corto sí se
  separan por uid. `evaluaHomonimos` exige la forma exacta, así que cumplirlo obligaría a declarar en `entities.json`
  una forma que no es de la persona —justo lo que el validador y J-DIC castigan—. No se toca el pipeline para
  complacerlo. Criterio medible del caso real: dos personas, dos seudónimos distintos, separadas por uid (J-HOM).
