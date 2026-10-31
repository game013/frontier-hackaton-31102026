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
| CE-12 | Cuasi-identificador sin nombre                     | 80e0b943 → CL-50124                              | Sector, ciudad, "hace unos años" en chat **y** maestro     | J-IND        |
| CE-13 | Plantilla "soy X, vivo en Y, desde Z"              | af9ccd6e, 8e61cf6b, 9cb80452                     | Generalizado en texto y fila                               | J-IND        |
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
