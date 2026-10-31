# Triage labels

Los skills hablan de cinco roles canónicos de triage. `PROGRESS.md` no tiene etiquetas: cada rol se traduce a la
columna Estado (o a la sección donde vive la fila).

| Rol en mattpocock/skills | En `PROGRESS.md`                                                     | Significado                         |
| ------------------------ | -------------------------------------------------------------------- | ----------------------------------- |
| `needs-triage`           | Entrada `Pn` en "Pendientes de decisión", Estado `abierta`           | El usuario tiene que evaluarla      |
| `needs-info`             | Estado `bloqueada` + línea `BLOQUEO:` en la Bitácora con lo que falta | Esperando información               |
| `ready-for-agent`        | Estado `lista`                                                       | Especificada, lista para un agente  |
| `ready-for-human`        | Estado `lista`, Responsable `usuario`                                | La hace una persona                 |
| `wontfix`                | Estado `descartada` + motivo en la Bitácora                          | No se hará                          |

Cuando un skill mencione un rol (p. ej. "aplica la etiqueta lista para agente"), aplica la traducción de esta tabla.
