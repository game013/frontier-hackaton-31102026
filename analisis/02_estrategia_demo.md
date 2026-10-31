# Estrategia de demo: vende los datos sin vender a la gente

## 1. Posicionamiento y estado

**Vendemos un corpus aprovechable para entrenar atención al cliente, con identidades sustituidas y relaciones conservadas.** El comprador necesita aprender cómo se resuelve un cobro, no conocer a quien lo reclamó.

> «Kipu Pay tiene tres años de experiencia atendiendo clientes. Hacemos que esa experiencia pueda aprovecharse sin entregar las identidades originales al comprador».

- **Cliente inicial:** la fintech que prepara el corpus para un comprador de datos de entrenamiento.
- **Usuario:** el equipo de datos, con revisión de privacidad y seguridad.
- **Valor:** reducir preparación y revisión sin destruir conversaciones ni relaciones útiles. Es una hipótesis comercial: no afirmar ahorros ni mejoras de entrenamiento sin medirlos.
- **Diferenciador:** una persona ficticia coherente entre fuentes y entre corridas; personas distintas no se fusionan por compartir nombre.
- **Alcance:** los formatos y condiciones del reto. No es una plataforma universal ni una certificación de anonimato o cumplimiento legal.

**Estado de esta revisión:** se revisaron `DISEÑO.md` v2, sus decisiones D1–D34, `CASOS_ESQUINA.md` y `PROGRESS.md`. El tablero aún sitúa la implementación al comienzo; no se verificó un servicio funcional ni resultados propios. Todo comportamiento del guion está condicionado a evidencia antes de presentarlo. La salida de `sample/expected_output/` pertenece al organizador, no es un resultado nuestro.

## 2. Prioridades según la rúbrica

| Etapa | Peso global | Implicación |
| --- | --- | --- |
| Systems design | 30 % | Decisiones, alternativas y medición explícitas en el documento y diagrama exigidos. |
| Ejecución | 40 % | Pipeline, contrato HTTP, estabilidad y entregables antes que presentación ornamental. |
| Demo day | 15 % | Una historia comprensible, transformación visible, prueba nueva y evidencia. |
| Profundidad técnica | 15 % | Cada integrante prepara el debrief individual; el pitch deja una muestra de las decisiones. |

La evaluación automática del corpus tiene otra distribución: **fuga 45, utilidad 35, consistencia 20**. No confundirla con los pesos globales ni inventar una fórmula de evaluación del pitch. Falta el detalle de criterios de Demo day en `RUBRICA.md`.

**Regla de inversión:** no sacrificar el 70 % de diseño y ejecución por embellecer el 15 % del pitch. La demo usa resultados y servicio reales; no un motor distinto en el navegador.

La última corrida anterior al crecimiento vale 30 % del componente automático y la última posterior, 70 %. No equivale al 70 % de toda la hackathon. Las tres corridas oficiales las lanza el usuario; no son ensayos de presentación.

## 3. Una historia y un caso difícil

Seguir **un reclamo por una cuota ya pagada** en chat, tabla y documento. Después de sustituir identidades, el comprador todavía entiende qué se reclamó, que el pago llegó por PSE desde Bancolombia, por qué hubo retraso y cómo se resolvió.

El caso de muestra asociado a `CL-88627` permite ensayar esta narrativa. Su documento es Markdown, no PDF. Los seudónimos definitivos deben salir de nuestro motor. Si se muestra la salida ideal antes de implementar, rotular «ejemplo del organizador» y no presentarla como prueba propia.

En el pitch añadir **un solo caso difícil, con un recuadro preparado**, sin abrir otra historia:

- **Preferido: homónimos.** Dos personas con el mismo nombre, evidencia distinta y dos perfiles. Seleccionar un caso probado de CE-01 o CE-02; mostrar la referencia que realmente resolvió el motor. No inventar que un radicado hizo el desempate si no fue así.
- **Alternativa si es más clara y está implementada: riesgo indirecto.** Mostrar la combinación de ocupación, barrio y año antes/después, junto a la fila vinculada; comprobar CE-12/CE-13 con J-IND. Cambiar solo el chat no basta. La política distingue texto y tablas: barrio a ciudad en texto, barrio sintético en la tabla; no afirmar que ambos se transforman igual.

Elegir antes del ensayo, no narrar ambas alternativas. Si ninguna tiene evidencia suficiente, retirar el recuadro y la afirmación. Metadatos y el segundo caso difícil quedan para el video o preguntas.

## 4. Pitch de tres minutos

Duración confirmada por el equipo: **3 minutos**. Ensayar **2:45 de contenido + 15 segundos de margen**. Una persona conduce; otra opera si hay equipo. Una pantalla comparativa y una tarjeta de evidencia; sin recorrido por endpoints o arquitectura.

Los textos siguientes son afirmaciones a habilitar al verificar el comportamiento, no resultados ya obtenidos.

| Tiempo | Pantalla y acción | Guion hablado |
| --- | --- | --- |
| 0:00–0:20 | Problema, una diapositiva | «Kipu Pay tiene tres años de conversaciones y un comprador que quiere usarlas para entrenar atención al cliente. El comprador necesita aprender cómo se resuelve un reclamo, no conocer a quien lo hizo». |
| 0:20–0:50 | Antes/después: chat y referencias a tabla y documento | «Aquí una clienta reclama una cuota que ya pagó. Sustituimos nombre, cédula y correo. El pago por PSE desde Bancolombia y la solución siguen intactos. Y la misma persona conserva su identidad ficticia en las tres fuentes». |
| 0:50–1:15 | Recuadro del caso difícil y decisión técnica | «Un mismo nombre puede pertenecer a dos personas: aquí, [evidencia verificada] nos permite distinguirlas y darles perfiles distintos. Sustituimos los fragmentos personales, sin reescribir la conversación». |
| 1:15–1:55 | Un clic: enviar dos registros nuevos; mostrar ambas respuestas | «Ahora enviamos dos mensajes que no estaban en el corpus: uno sobre una persona conocida y otro sobre una persona ficticia nueva». Al responder: «La primera conserva su seudónimo; la segunda recibe un perfil nuevo. El detalle de negocio que acabamos de editar también permanece». |
| 1:55–2:30 | Tarjeta de evidencia con versión y alcance | «En esta corrida conservamos [X de X registros]. Tras incorporar nuevos datos, [N de N personas anteriores] conservaron su seudónimo: guardamos los perfiles, no los sorteamos de nuevo. El barrido final encontró [F coincidencias de formas originales conocidas]». Usar solo resultados medidos y aprobados para presentar. |
| 2:30–2:45 | Resultado final, sin navegación | «El comprador puede aprender de cómo resolvemos un problema sin necesitar la identidad original del cliente. Conservamos la experiencia de soporte y sustituimos las identidades». |
| 2:45–3:00 | Margen | Absorber latencia, pausas y transiciones; no añadir otra funcionalidad. |

**Alternativa de diez segundos si se elige riesgo indirecto:** «Aunque quitemos el nombre, barrio, profesión y año pueden señalar a alguien. Aquí rompemos esa combinación también al cruzar las fuentes». Reemplaza la frase de homónimos, no se añade a ella.

La profundidad técnica cabe en dos decisiones y una prueba: **reemplazar sin reescribir; guardar un perfil por persona; distinguir personas con evidencia**. El crecimiento se demuestra en la tarjeta, no con otra ejecución pesada durante el pitch.

No llenar cada intervalo con discurso: incluye clics y lectura de pantalla. Si falta una medición, quitar la afirmación o sustituirla por otra verificada; nunca leer marcadores ni presentar objetivos como resultados. No afirmar «cero fugas» sin el calificativo «conocidas en estas comprobaciones». Si el barrido detecta fugas conocidas, corregir antes de presentar la salida como lista.

### Prueba en vivo: dos registros, una petición

- Un lote de la misma fuente, válido según `docs/api/participante.openapi.yaml`. La interfaz construye el sobre; no hace falta un editor OpenAPI.
- **A, persona conocida:** referencia suficiente para que el motor la vincule al corpus, mediante identificador o hilo conocido. El nombre aislado no garantiza identidad (D29).
- **B, persona ficticia nueva:** nombre y datos ficticios comprobados como ajenos al corpus y a los perfiles existentes. Verificar detección, formato del perfil y ausencia de colisiones en el mapa; ver dos nombres distintos no basta.
- Editar una frase de negocio ficticia antes de enviar para que sea visible que el payload puede variar. No pedir PII real ni abrir una ronda de propuestas del público.
- Mostrar el texto enviado y los campos relevantes de la respuesta real. Para A, comparación con el perfil del corpus; para B, perfil nuevo y comprobación de unicidad. No mostrar el diccionario completo.
- Esta petición demuestra continuidad y creación de perfil, **no** estabilidad tras regenerar el corpus ni los dos homónimos. Esas pruebas son separadas.
- D24 mantiene las personas creadas en vivo en almacenamiento separado: reservan seudónimo, pero no se agregan al `entities.json` de la última corrida. No prometer que B ya forma parte del corpus entregable ni que se integrará automáticamente al crecer las fuentes.

### Latencia y transición al respaldo

Medir el servicio desplegado con el lote exacto, incluyendo estado frío/caliente, casos nuevos y dependencia del modelo. No prometer `<300 ms`: D28 permite decisiones externas por mención; no todo es un lookup.

Al enviar, explicar qué se compara, sin silencio. **A los 3 segundos sin respuesta**, continuar con una captura o grabación de una ejecución real, rotulada con versión y «respaldo grabado». El umbral es de presentación: no implica cancelar necesariamente la petición ni modifica los timeouts del servicio. Nunca usar un mock como prueba de ejecución.

## 5. Superficie mínima y límite de inversión

### P0 — servicio y evidencia

1. Servicio público con el contrato, `run.sh`, regeneración completa y salidas válidas.
2. Resolución entre fuentes, perfiles persistidos y comprobación de estabilidad al crecer el corpus.
3. Tratamiento de texto, tablas, autores, metadatos y nombres de archivo sin pérdida de esquema.
4. Mismo núcleo para `/run` y `/anonymize`; persistencia de personas nuevas según D24.
5. Validador, jueces por mecanismo, cobertura de personas en la muestra y mediciones de tiempo/tokens.
6. Un caso difícil verificado, una prueba en vivo reproducible y respaldo de la misma versión.

### P1 — una sola pantalla, sin navegación

- **Izquierda:** fragmentos de entrada del chat, la fila y el documento; no archivos completos.
- **Derecha:** salida real de esos fragmentos, con etiquetas de identidad sustituida, marca conservada y continuidad entre fuentes. Resaltados preparados: no construir un diff genérico.
- **Recuadro:** un único caso difícil verificado.
- **Panel inferior:** dos registros preparados, un campo editable y **un botón** que envía el lote al backend configurado. Respuesta legible, estado y procedencia visibles.

Sin SPA, editor de esquemas, visualizador de grafos ni dashboard administrativo. Elegir la herramienta más barata para el equipo, sin imponer un framework nuevo. No asumir localhost: ensayar contra la URL que realmente se usará.

**Presupuesto P1: máximo 60 minutos una vez estable P0.** Si no está lista o legible al cerrar ese bloque, usar diapositivas de salidas reales y un cliente HTTP ya disponible para la petición, con texto grande y sin secretos. No consumir tiempo de correcciones del pipeline para acabar la interfaz. El respaldo grabado se prepara en cualquier caso.

## 6. Qué tomar del diseño actualizado

| Decisión | Valor para el comprador | Uso en la demo |
| --- | --- | --- |
| D2, D3, D10, D29: persona y perfil completo; identidad por evidencia | Conserva relaciones sin fusionar personas por nombre. | Comparativa y caso difícil. En vivo, referencias suficientes y no solo `author`. |
| D4, D22: bóveda persistente de solo agregar y llaves estables | Una entrega ampliada mantiene la continuidad con la anterior. | «Guardamos los perfiles, no los sorteamos de nuevo», respaldado por J-CRE. La semilla sola no sustituye la bóveda ni su respaldo. |
| D8: mismo núcleo para corpus y mensajes nuevos | El comportamiento mostrado en vivo es el que transforma los datos entregados. | Petición al servicio real; sin motor alternativo en el frontend. |
| D6: reemplazo literal | Se conserva la explicación del reclamo sin inventar montos o soluciones. | Una frase en el pitch; detalles en video. |
| D11, D17, D34: compuerta, jueces por mecanismo y cobertura de personas | Se revisan fallos distintos; buscar solo lo ya detectado dejaría puntos ciegos. | Barrido conocido en la tarjeta; J-REC y casos de regresión en video. Ninguno prueba ausencia universal de reidentificación. |
| D13, D15, D26, D28, D30–D32: reglas primero, modelo para decisiones cerradas y residuo | Evita delegar la redacción del corpus; limita el contexto enviado y conserva decisiones. | Preguntas/video, no una lista de proveedores en el pitch. No vender como «todo local» ni «ningún dato sale»: hay riesgo residual de exposición al proveedor declarado en D15. |
| D19, D20: ambigüedad registrada en vez de fusión silenciosa | Hace visibles los casos que necesitan revisión. | Debrief: explicar el costo en consistencia de crear una persona dudosa. No prometer desambiguación infalible. |

No llevar al pitch nombres de librerías, número de goroutines, umbrales de confianza ni snapshot del modelo. Son decisiones de implementación, no el argumento comercial principal. Este documento consume el diseño; no modifica sus decisiones.

## 7. Evidencia y límites de las afirmaciones

| Afirmación | Prueba necesaria |
| --- | --- |
| Conservamos X de X registros | Comparación por fuente con el `record_index` de la corrida, ids únicos y validador. |
| Conservamos campos y contenido | Comparación de claves/columnas con la entrada y controles de texto y estructura; el OK del validador no sustituye todo esto. D33 puede producir un cuerpo PDF vacío tras fallar los extractores: conservar el id no significa conservar su utilidad. Contar y declarar esos fallos. |
| No encontramos fugas conocidas | Barrido global con alcance, resultado y versión; además pruebas de cobertura J-REC sobre la muestra para no medir solo personas ya detectadas. No extrapolar su cobertura a todo el corpus. |
| Los seudónimos anteriores no cambiaron | J-CRE: corrida con subconjunto, nueva corrida ampliada y comparación por persona. Declarar si es simulación local o crecimiento real del API. |
| Dos homónimos son distintos | Evidencia de identidad y perfiles distintos; J-HOM y J-CON. |
| La persona nueva tiene perfil único | J-VIV y comprobación contra los perfiles reservados, no solo inspección visual. |
| El procesamiento es operativo | Tiempo y tokens reales; costo monetario solo con base de cálculo explícita. |

La tarjeta del pitch prioriza **cobertura de registros, estabilidad entre corridas y barrido conocido**. Máximo tres cifras con versión y alcance. Puntaje oficial o duración pueden sustituir una, no sumarse. Mantener disponible el resto para preguntas. Las puntuaciones del juez propio no son puntajes oficiales ni porcentajes de privacidad.

El `entities.json` de una corrida ampliada **debe poder crecer**: lo que no cambia es el perfil de las personas anteriores, no el archivo completo. La prueba entre corridas no puede reemplazarse por una sola llamada a `/anonymize`.

No afirmar resultados de entrenamiento, ahorros económicos, anonimato absoluto, cumplimiento legal automático o que jamás se exponen datos a terceros. `entities.json` contiene originales y nunca viaja con el corpus comercial. El reto exige endpoints sin autenticación; comercializar requeriría separación de accesos, protección del mapa y política de entrega, no implementadas por el mero contrato del reto.

## 8. Video de cinco minutos: misma narrativa, versión extendida

El pitch de tres minutos está confirmado; el enunciado pide una demo de cinco y la rúbrica menciona video. Planear ambos y confirmar el formato final de subida. Comparten caso, versión y evidencia, con dos guiones propios. El video extendido es la referencia narrativa completa; el pitch destila sus pruebas esenciales, no añade una historia distinta.

| Tiempo del video | Contenido |
| --- | --- |
| 0:00–2:30 | Problema, antes/después, caso difícil, lote conocido/nuevo y evidencia esencial. Misma narrativa del pitch, sin duplicar luego las mismas métricas. |
| 2:30–3:35 | Diagrama breve: fuentes → registro de personas → detección/resolución → perfil persistido → reemplazo → validación. Una alternativa descartada: reescritura total. Ampliar el caso difícil con la evidencia real de desempate, o generalización sobre la unión si fue el caso elegido. |
| 3:35–4:20 | Prueba de crecimiento: comparación de dos corridas, perfiles anteriores estables y registros nuevos. Explicar bóveda, llaves estables y respaldo; no afirmar que `entities.json` queda idéntico. |
| 4:20–4:45 | Límites y control: barrido de formas conocidas, cobertura J-REC en muestra, cola de dudosos y uso acotado del modelo con riesgo residual declarado. Solo mostrar controles ya implementados. |
| 4:45–5:00 | Cierre con la experiencia de soporte preservada. |

El video refuerza diseño y profundidad, pero no sustituye documento, diagrama ni debrief individual. No usar cifras de costo o latencia del modelo con datos inventados como si fueran el costo o latencia del pipeline completo.

## 9. Debrief y preparación operativa

Cada integrante prepara un ejemplo, una decisión y una limitación para: identidad frente a cadena; homónimos y rol en el hilo; cuasi-identificadores sobre la unión; metadatos y adjuntos; bóveda y recuperación; D24 en vivo; modelo y exposición residual; casos dudosos; cobertura de personas frente a barrido conocido.

### Antes de presentar

- Verificar los resultados y habilitar solo las frases respaldadas por esa versión. No marcar C23 como completada por tener esta estrategia: aún requiere `DECISIONS.md` y cifras de ejecución.
- Congelar el caso, datos ficticios nuevos y perfiles del corpus; comprobar los dos registros contra el servicio desplegado.
- Capturar entrada, salida, tarjeta y grabación de respaldo de la misma versión. No mostrar `.env`, tokens, logs sensibles, diccionario completo ni PII real del corpus en la proyección pública.
- Si el caso difícil viene del corpus real, representarlo con un fixture ficticio equivalente y validarlo por separado; distinguir esa demostración de los resultados agregados del corpus. No alterar evidencia y seguir llamándola ejecución original.
- Probar latencia fría/caliente, un caso nuevo y fallo de red. Ensayar tres veces a 2:45, incluida transición al respaldo en tres segundos.
- No ejecutar `/run` en escena ni consumir corridas oficiales para ensayar. Comprobar versión y salud del servicio antes del turno.
- Asegurar legibilidad desde lejos: fragmentos cortos, sin JSON extenso ni tablas diminutas.

### Si algo falla

- **Red o espera:** «La petición sigue pendiente; esta es una ejecución grabada de la versión X». Mostrar respaldo rotulado, no un mock.
- **Resultado incorrecto:** reconocer el fallo y continuar con evidencia de alcance real; no cambiar silenciosamente de entrada.
- **Sin crecimiento probado:** retirar la cifra de estabilidad; mencionar el mecanismo solo como diseño, sin prometer resultado.
- **Sin puntaje oficial:** métricas locales identificadas, sin equivalencias inventadas.
- **Tiempo reducido:** quitar primero el recuadro difícil y sus frases; conservar valor → transformación → prueba/evidencia → cierre.

## 10. Criterio de demo lista

En menos de tres minutos una persona no técnica entiende **qué obtiene el comprador, qué cambia para proteger identidades y qué evidencia respalda el resultado**. Una persona técnica puede rastrear cada afirmación hasta una corrida, una decisión y una limitación. El ensayo incluye el fallo de red sin exceder el tiempo.

Fuentes: `docs/ENUNCIADO.md`, `RUBRICA.md`, `DISEÑO.md` §6–9 y D1–D34, `CASOS_ESQUINA.md`, `PROGRESS.md`, `analisis/01_sample.md`, tests públicos y muestra del organizador.
