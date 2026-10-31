# Domain docs

Cómo deben consumir los skills de ingeniería la documentación de dominio de este repo. Layout: **single-context**.

## Antes de explorar, lee esto

- **`AGENTS.md` §Vocabulario:** el glosario del proyecto (persona, forma, perfil, bóveda, fuga).
- **`DISEÑO.md` §7:** la tabla de decisiones (D1…Dn), con su porqué y las alternativas descartadas. Hace las veces
  de ADRs; no se crea `docs/adr/`. Cambiar una decisión exige actualizar esa tabla en el mismo commit.
- **`CASOS_ESQUINA.md`:** casos esquina, cada uno con su juez.
- **`analisis/00_resumen.md`:** el meta-análisis del corpus, si el tema toca los datos de entrada.

## Usa el vocabulario del glosario

Cuando tu salida nombre un concepto del dominio (título de una fila de `PROGRESS.md`, propuesta de refactor,
hipótesis, nombre de un test), usa el término tal como lo define `AGENTS.md` §Vocabulario. No derives a sinónimos.

Si el concepto que necesitas no está en el glosario, es una señal: o estás inventando lenguaje que el proyecto no
usa (reconsidéralo) o hay un hueco real (anótalo para `/domain-modeling`, que lo agrega a §Vocabulario).

## Señala conflictos con decisiones

Si tu salida contradice una decisión de `DISEÑO.md` §7, dilo explícitamente en lugar de pasarla por encima, y
anótalo en la Bitácora de `PROGRESS.md` con `DECISIÓN:` (regla 10):

> _Contradice D14 (detección en dos pasadas), pero vale la pena reabrirla porque…_
