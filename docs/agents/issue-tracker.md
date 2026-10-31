# Issue tracker: PROGRESS.md

Las tareas de este repo viven como filas de `PROGRESS.md`. No se usan GitHub Issues: el repo es público y el
dominio maneja datos personales reales.

## Convenciones

- Un ticket es una fila de la tabla de su ola en `PROGRESS.md`:
  `| Paso | Qué | Toca | Depende de | Hecha cuando | Responsable | Estado | Commit |`.
- Se numera `C<n>`, siguiendo la última fila existente.
- El estado de triage va en la columna Estado (ver `triage-labels.md`).
- Un spec que no cabe en una celda va en `docs/specs/<Cn>-<slug>.md` y la fila lo enlaza en *Qué*.
- Comentarios e historia: una línea en la **Bitácora**, la más nueva arriba: `HH:MM · Cn · agente · qué pasó`.
  Prefijos: `CONTRATO:`, `DECISIÓN:`, `BLOQUEO:`, `CASO:`.
- Nunca nombres, cédulas, correos ni celulares reales; solo ids de registro y conteos (regla 8 de `PROGRESS.md`).

## Cuando un skill dice "publica en el issue tracker"

Agrega la fila en la ola que corresponda, con Estado `espera` (si tiene dependencias abiertas) o `lista`, y haz un
commit que solo toque `PROGRESS.md`: `chore: agrega Cn`.

## Cuando un skill dice "trae el ticket"

Lee la fila `C<n>` de `PROGRESS.md`, sus líneas de Bitácora y el spec enlazado, si lo hay.

## Tomar, cerrar, bloquear

Sigue las reglas de coordinación de `PROGRESS.md`: tomar (1), terminar (5), abrir dependientes (6), bloqueos (7).

## Operaciones de wayfinding

Usadas por `/wayfinder`.

- **Mapa:** las secciones "Estado ahora" y "Pendientes de decisión" de `PROGRESS.md`.
- **Hijo:** una fila `C<n>`; la pregunta o tarea va en *Qué*.
- **Bloqueo:** la columna *Depende de*. Una fila se desbloquea cuando todas sus dependencias están `hecha`.
- **Frontera:** filas `lista` sin Responsable; gana el número más bajo.
- **Tomar:** regla 1 (Estado `en curso`, Responsable, hora, commit `chore: toma Cn`) antes de trabajar.
- **Resolver:** regla 5; la respuesta o el resumen va a la Bitácora, y si cambia una decisión, a `DISEÑO.md` §7.
