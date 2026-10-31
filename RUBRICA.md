# Rúbrica de evaluación del reto

Fuente: pestaña RÚBRICA de `/evento/reto` en Frontier (captura del 2026-10-31).

## Pesos globales

| # | Etapa              | Peso | Se alimenta de                                              |
| - | ------------------ | ---- | ----------------------------------------------------------- |
| 1 | Systems design     | 30 % | Doc de diseño + diagrama de arquitectura (cierre **12:00**) |
| 2 | Ejecución          | 40 % | Repo, entrypoint, video y set de evaluaciones oculto        |
| 3 | Demo day           | 15 % | Pitch en tu turno + material opcional                       |
| 4 | Profundidad técnica| 15 % | Debrief individual de 10 minutos                            |

Escala de 1 a 4 por criterio.

## 1. Systems design (30 %) — cierre 12:00

Entregables: **doc de diseño** y **diagrama de arquitectura**.

| Criterio                      | Qué buscan                                                    | 1 se ve así                              | 4 se ve así                                                                 |
| ----------------------------- | ------------------------------------------------------------- | ---------------------------------------- | --------------------------------------------------------------------------- |
| Entendimiento del problema    | Qué tiene que resolver el sistema, sus límites y sus supuestos | Repite el brief sin tomar decisiones     | Nombra supuestos, casos difíciles y lo que deja fuera, con razones          |
| Arquitectura y trade-offs     | Componentes claros y decisiones justificadas                  | Un diagrama sin decisiones detrás        | Cada decisión dice qué alternativa descartó y por qué                       |
| Plan para medir               | Cómo van a saber si funciona antes de construir               | No hay forma de medir el resultado       | Casos de prueba y métricas concretas desde el diseño                        |

### Implicaciones para el doc de diseño

- **Supuestos explícitos** (p. ej. el maestro de clientes es la verdad de identidad; los ids `CL-/TX-/PQR-/KP-` no son PII; dominios públicos se conservan).
- **Casos difíciles con nombre propio**: homónimos que se desempatan en otra fuente, agente que se llama como una clienta, correos deletreados, MAYÚSCULAS sin tildes, metadatos de PDF, comentarios HTML, menciones de Slack, cuasi-identificadores sobre la unión de fuentes, crecimiento de fuentes sin aviso.
- **Qué dejamos fuera y por qué** (p. ej. OCR de imágenes, reescritura con LLM).
- **Cada decisión con su alternativa descartada**: tabla decisión / alternativa / por qué.
- **Plan de medición**: evaluador local que imita las seis familias de tests (directa, indirecta, oculta, falso_positivo, homonimos, consistencia), barrido de fuga con la misma normalización de `validar.mjs`, métricas objetivo por dimensión (fuga 45, utilidad 35, consistencia 20), y prueba de estabilidad de seudónimos entre corridas.

## 2. Ejecución (40 %)

Repo, entrypoint (`VALIDAR ENTRYPOINT`, 3 corridas), video y set de evaluaciones oculto. El puntaje automático del
jurado (0–100) vive aquí: última corrida antes del cambio de fuentes = 30 %, última después = 70 %.
Entregables: repo con pipeline, `run.sh`, `DECISIONS.md`, demo de 5 minutos.

## 3. Demo day (15 %)

Pitch en tu turno. Material opcional. `/anonymize` en vivo con lo que escriba el público es el gancho.

## 4. Profundidad técnica (15 %)

Debrief individual de 10 minutos: cada integrante debe poder explicar el pipeline y sus trade-offs.

> Nota: solo se vio el detalle de criterios de Systems design. Falta capturar los criterios de Ejecución, Demo day y
> Profundidad técnica si la página los muestra.
