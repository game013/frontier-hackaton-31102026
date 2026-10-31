# Estrategia de demo: vende los datos sin vender a la gente

## 1. Decisión de posicionamiento

**Vendemos un corpus aprovechable para entrenar atención al cliente, con identidades sustituidas y relaciones conservadas.** El comprador necesita aprender cómo se resuelve un cobro, no conocer a quien lo reclamó.

Frase central del pitch:

> «Kipu Pay tiene tres años de experiencia atendiendo clientes. Hacemos que esa experiencia pueda aprovecharse sin entregar las identidades de sus clientes».

La audiencia debe recordar tres pruebas: **la historia sigue sirviendo; cada persona conserva su identidad ficticia; un dato nuevo recibe el mismo tratamiento**.

- Cliente inicial: la fintech que prepara el corpus para un comprador de datos de entrenamiento.
- Usuario: el equipo de datos, con revisión de privacidad y seguridad.
- Valor: reducir el trabajo de preparación y revisión, preservando conversaciones y relaciones útiles. Es una hipótesis de valor; no afirmar ahorros sin medirlos.
- Diferenciador a demostrar: resolver identidad entre fuentes, incluso con homónimos y nuevas incorporaciones. Detectar un correo es solo una parte del problema.
- Alcance: este reto y sus formatos. No presentar todavía una plataforma universal ni prometer cumplimiento legal automático.

**Estado al redactar:** el repo contiene el enunciado, fuentes, muestra, rúbrica y análisis; no se ha verificado un servicio funcional ni resultados propios. Este documento es un plan de producto y demostración, no evidencia de capacidades implementadas. Los ejemplos de `sample/expected_output/` son del organizador.

## 2. Cómo competir según la rúbrica

| Etapa | Peso | Cómo contribuye esta estrategia |
| --- | --- | --- |
| Systems design | 30 % | Decidir desde ahora qué pruebas construiremos, cómo se medirán y qué trade-offs explicar. No sustituye el documento de diseño ni el diagrama exigidos. |
| Ejecución | 40 % | Priorizar pipeline, contrato HTTP, validación y estabilidad. La demo consume el servicio real; no una transformación distinta en el navegador. |
| Demo day | 15 % | Una historia concreta, prueba entre fuentes, interacción en vivo y cierre con evidencia. |
| Profundidad técnica | 15 % | Reservar decisiones, límites y casos difíciles para el debrief individual. |

El puntaje automático del corpus tiene otra distribución: **fuga 45, utilidad 35, consistencia 20**. No confundirla con los pesos globales. La demo debe hacer visibles esas tres dimensiones, no inventar una fórmula de calificación del pitch.

`RUBRICA.md` advierte que no se capturó el detalle de criterios de Demo day. Esta estrategia se apoya en los pesos conocidos y el enunciado; ajustar si aparecen criterios adicionales.

**Regla de inversión:** no sacrificar el 70 % de diseño y ejecución para embellecer el 15 % del pitch. Una vista comparativa fiable vale más que un dashboard amplio sin pipeline detrás.

## 3. La historia que vamos a mostrar

Seguir **un reclamo por una cuota ya pagada** desde el chat hasta su referencia en una tabla y un documento.

Antes de sustituir identidades, la conversación contiene datos personales. Después, el comprador todavía entiende:

- Qué reclamó la persona y cómo respondió soporte.
- Que el pago llegó por PSE desde Bancolombia y hubo un retraso.
- Qué caso corresponde a qué persona a través de las fuentes.

El nombre, el correo y la cédula cambian; la explicación del cobro permanece. PSE y Bancolombia no desaparecen.

El caso de la muestra de Manuela Castaño y `CL-88627` permite preparar esta narrativa. **Los seudónimos de nuestra demo deben salir de nuestro motor**, no copiarse de la salida ideal. Si usamos la muestra antes de tener implementación, rotularla como «ejemplo del organizador».

Preparar además dos casos de respaldo para preguntas, fuera del guion de tres minutos, seleccionados por evidencia real:

1. Un nombre escondido en un adjunto o metadato: cambiar solo el cuerpo no alcanza.
2. Dos personas con el mismo nombre: necesitan identidades ficticias distintas, desempatadas con evidencia entre fuentes, no por adivinar a partir del nombre.

El caso indirecto de barrio + ocupación + año queda como respaldo técnico o reemplazo del caso de adjunto. Mostrarlo solo si podemos probar que la combinación también se rompió al unir las fuentes.

## 4. Guion de tres minutos

Duración del pitch confirmada por el equipo: **3 minutos**. Objetivo de ensayo: **2:45 de contenido y 15 segundos de margen**. El video de cinco minutos mencionado en el enunciado es un entregable separado, salvo aclaración de la organización.

Una persona conduce; otra opera si hay equipo. Una historia, una petición en vivo y una tarjeta de evidencia. Sacar del escenario el diagrama, el segundo caso y las explicaciones de endpoints; reservarlos para preguntas.

| Tiempo | Pantalla y acción | Guion hablado propuesto |
| --- | --- | --- |
| 0:00–0:20 | Problema, una sola diapositiva | «Kipu Pay tiene tres años de conversaciones y un comprador que quiere usarlas para entrenar atención al cliente. Pero esas conversaciones contienen identidades. Queremos conservar lo que sirve para aprender, sin entregar quién vivió cada problema». |
| 0:20–0:55 | Antes/después del reclamo; señalar nombre, correo y frase de negocio | «Aquí una clienta reclama una cuota que ya pagó. Cambiamos su nombre y sus datos de contacto. Pero conservamos lo importante: pagó por PSE desde Bancolombia, hubo un retraso y soporte retiró el cobro. El comprador todavía puede aprender cómo resolver este caso». |
| 0:55–1:20 | Chat, tabla y documento en una misma vista | «La dificultad es que esa persona aparece en varias fuentes. Aquí conserva la misma identidad ficticia en el chat, la tabla y el documento. Sustituimos fragmentos personales, sin reescribir la conversación ni romper sus relaciones». |
| 1:20–2:00 | Editar una frase ficticia en un registro nuevo y enviarlo al servicio | «Probemos un mensaje nuevo sobre esta misma persona». Editar y enviar. Al recibir: «El mensaje no estaba en el corpus. Recibe el mismo seudónimo y conserva el detalle de negocio que acabamos de agregar». Dejar tiempo para que se lea la respuesta. |
| 2:00–2:30 | Tarjeta de evidencia, máximo tres cifras | «Esto es lo que medimos en la corrida que están viendo: conservamos [X de X registros], aprobamos [Y de Z pruebas locales] y procesamos el corpus en [T]. Son resultados de esta versión, no una garantía absoluta contra reidentificación». Usar solo cifras verificadas; si hay puntaje oficial, sustituir una de las métricas e identificarlo. |
| 2:30–2:45 | Resultado final, sin nueva navegación | «El comprador necesita aprender cómo se resolvió el problema, no conocer a quien lo tuvo. Nuestra solución conserva esa experiencia y sustituye las identidades». |
| 2:45–3:00 | Margen | No abrir otra funcionalidad. Absorber latencia o pausas. |

Los espacios de cada bloque incluyen acciones y lectura de pantalla: no llenarlos con más discurso. Reemplazar los marcadores de métricas antes del ensayo final; si falta evidencia, quitar la afirmación.

**Recorte editorial:** homónimos, metadatos, cuasi-identificadores, costo en tokens y arquitectura van al debrief. La profundidad técnica del pitch queda concentrada en una decisión —reemplazar sin reescribir— y una prueba —consistencia entre fuentes y un registro nuevo—.

### Interacción en vivo acotada, pero auténtica

- Pedir **datos ficticios**, nunca nombres, cédulas o teléfonos reales del público.
- Usar un registro normalizado válido según `docs/api/participante.openapi.yaml`; la interfaz construye el sobre y deja editar el texto. El editor libre no debe romper el contrato.
- Llevar preparado un registro nuevo sobre una persona ya procesada y editar en escena una frase de negocio ficticia. No abrir una ronda de propuestas del público en el guion de tres minutos. La persona ficticia nueva queda como respaldo o para preguntas.
- La interfaz debe mostrar qué se envió y qué devolvió el servicio. No precalcular la respuesta del texto editado ni presentar una grabación como petición en vivo.
- No prometer resolver homónimos con un nombre aislado. El caso preparado debe contener las referencias que el motor realmente necesita.
- Si la respuesta tarda más de 10 segundos, continuar explicando la evidencia anterior; a los 15, usar el respaldo. Son umbrales de presentación, no límites del contrato: el reto permite 60 segundos por lote de hasta 50 registros.

## 5. Qué construir para sostener la demo

### P0 — producto evaluable

1. Servicio público con el contrato requerido, `run.sh` y regeneración completa dentro del límite.
2. Resolución de identidad entre fuentes y mapa estable de seudónimos entre corridas.
3. Transformación de texto, tablas, autores, nombres de archivo y metadatos; conservar campos e ids.
4. `/anonymize` usando el mismo motor y mapa que el procesamiento del corpus.
5. Validación local, evaluación de las seis familias de casos y medición de tiempo/tokens.
6. Descubrimiento de fuentes nuevas y prueba de continuidad tras el crecimiento del dataset.

### P1 — superficie mínima de demostración

Una sola página con tres vistas, sin navegación compleja:

- **Comparar:** entrada y salida lado a lado, cambios resaltados y etiquetas «identidad sustituida» / «dato de negocio conservado».
- **Seguir una persona:** sus referencias en chat, tabla y documento. El original solo en la vista de revisión del entorno de demo.
- **Probar mensaje:** editor, botón de envío, estado y respuesta real de `/anonymize`.

Pie de pantalla con versión, identificador de corrida y procedencia de las métricas. El diagrama y los resultados pueden ser diapositivas; no hace falta construir un panel administrativo.

No incluir antes de estabilizar P0: login ornamental, conectores fuera del reto, chatbot sobre el corpus, entrenamiento de un modelo o gráficos sin función probatoria.

## 6. Evidencia que debemos capturar

| Afirmación | Evidencia mínima | Cómo decirlo |
| --- | --- | --- |
| Conservamos todos los registros | Conteos por fuente comparados con el `record_index` de la corrida y validador aprobado. | «Conservamos X de X registros de esta versión del corpus». |
| Protegemos las formas detectadas | Barrido global y pruebas con alcance documentado. | «No detectamos fugas en estas comprobaciones». No «es imposible reidentificar». |
| El contenido sigue sirviendo | Frase de negocio intacta, marcas preservadas y relaciones verificadas. | «Se conserva el contexto de este reclamo». No afirmar calidad de entrenamiento sin evaluarla. |
| Mantenemos identidades consistentes | Una persona en varias fuentes, homónimos distintos y comparación entre corridas. | «En estas pruebas, las personas conservaron su seudónimo». |
| Procesamos el corpus de forma operativa | Duración, tokens y, si existe, costo monetario medido con base explícita. | «Esta corrida tardó X y consumió Y tokens». |
| Funcionamos con registros nuevos | Petición en vivo contra el servicio desplegado. | «Este mensaje nuevo reutiliza el mismo seudónimo». |

En pantalla, máximo tres cifras: cobertura, pruebas locales aprobadas/ejecutadas y duración. Si existe puntaje oficial, sustituir una de las dos últimas métricas. Cada una con versión y alcance. No convertir el porcentaje de tests locales aprobados en un supuesto «porcentaje de privacidad».

Los 8.297 ids y aproximadamente 435 mil tokens del enunciado describen el corpus inicial, no un resultado nuestro ni necesariamente el corpus tras crecer. Los resultados orientativos del enunciado tampoco son nuestro benchmark.

## 7. Profundidad técnica: una decisión en escena, diez minutos preparados

Para preguntas y debrief, usar este esquema; no necesita una pantalla propia durante el pitch:

**Fuentes heterogéneas → identidad entre fuentes → sustitución consistente → validación y salida**.

Decisión candidata, sujeta a que sea la implementada:

> «Sustituimos fragmentos personales en lugar de reescribir conversaciones enteras. Así evitamos alterar montos, fechas de negocio y la solución del reclamo».

No explicar expresiones regulares, estructuras de datos o proveedores salvo pregunta. La profundidad está en demostrar consecuencias de las decisiones.

Cada integrante debe poder responder, con un ejemplo y una limitación:

| Pregunta probable | Qué debe contener la respuesta |
| --- | --- |
| ¿Por qué no basta detectar nombres y correos? | Variantes, metadatos, terceros mencionados y combinaciones indirectas. |
| ¿Cómo distinguen homónimos? | Evidencia concreta entre fuentes, regla de desempate y comportamiento ante ambigüedad. |
| ¿Cómo conservan los seudónimos con fuentes nuevas? | Mecanismo implementado de persistencia/estabilidad y prueba comparando dos corridas. |
| ¿Por qué no borrar o hashear todo? | Se pierden categoría, relaciones y utilidad; hashes simples de identificadores no garantizan protección. |
| ¿Cómo evitan destruir el texto? | Reemplazos acotados, preservación de estructura y tests de falsos positivos. |
| ¿Cómo tratan barrio, ocupación y año? | Riesgo por combinación y evaluación sobre la unión de fuentes; política de generalización y sus límites. |
| ¿Dónde usan modelos? | Uso real, costo medido, datos enviados y alternativa descartada. Si no se usan, decirlo sin disculparse. |
| ¿Esto garantiza anonimato legal? | No. Es seudonimización con controles evaluados en el reto, no certificación ni prueba universal contra reidentificación. |
| ¿Qué ocurre con el diccionario? | `entities.json` contiene originales: es un artefacto restringido para el jurado, nunca parte del corpus comercial. |

**Límite importante:** el reto exige endpoints sin autenticación y acceso del jurado al diccionario. Eso no es una arquitectura de producción segura. Para comercializar harían falta separación de accesos, protección del mapa y una política de entrega; no afirmar que esos controles ya existen.

## 8. Ensayo, contingencias y entrega

### Antes de consumir corridas oficiales

- Aprobar el validador local y comprobar el servicio desplegado sin disparar evaluaciones innecesarias.
- Planear las tres corridas: obtener una válida antes del cambio de fuentes y reservar capacidad para después. La última anterior vale 30 % del componente automático y la última posterior, 70 %. No usar las tres como ensayos de presentación.
- Separar la ejecución pesada del corpus de la petición interactiva; no lanzar `/run` durante el pitch.

### Antes de presentar

- Elegir los casos definitivos desde salidas propias verificadas y fijar su versión.
- Capturar video corto, entrada/salida y métricas de esa misma versión como respaldo.
- Ensayar tres veces con cronómetro, incluyendo una respuesta lenta y un fallo de red.
- Comprobar legibilidad desde lejos y evitar terminales con JSON extenso.
- Mantener separado el pitch de tres minutos del video de cinco minutos indicado en el enunciado. En el video se puede ampliar un caso difícil y la arquitectura, con la misma evidencia; confirmar los requisitos finales del entregable.
- No mostrar `.env`, tokens, logs sensibles ni el diccionario completo. Usar la muestra del organizador o datos ficticios para la interacción pública.

### Si algo falla

- **Servicio o red:** «La petición en vivo no respondió; este es el resultado grabado de la versión X». Mostrar el respaldo identificado como tal.
- **Respuesta incorrecta:** reconocer el caso que falló, mostrar el alcance real de las pruebas y continuar. No cambiar silenciosamente de entrada.
- **Sin puntaje oficial:** presentar métricas locales rotuladas. No inventar una equivalencia.
- **Tiempo reducido:** conservar problema → antes/después → continuidad entre fuentes → prueba o evidencia → cierre. Quitar primero el caso difícil secundario y el diagrama.

## 9. Criterio de demo lista

La demo está lista cuando, en menos de tres minutos, una persona no técnica puede explicar **qué obtiene el comprador, qué cambia para proteger a las personas y qué pruebas respaldan el resultado**; y una persona técnica puede rastrear cada afirmación hasta una corrida, una decisión y una limitación.

Fuentes: `docs/ENUNCIADO.md`, `RUBRICA.md`, `analisis/01_sample.md`, `fuentes/tests_public.json` y `sample/expected_output/`.
