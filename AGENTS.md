# Vende los datos sin vender a la gente

Hackathon Frontier, 2026-10-31. Construimos en **Go** un servicio HTTP (Railway) que anonimiza el corpus de soporte
de Kipu Pay (chats, documentos y tablas) para venderlo: sin **fuga** de ninguna persona real, útil para entrenar, y
con cada persona convertida en **una sola persona falsa** en todas las fuentes. Un programa puntúa fuga 45,
utilidad 35 y consistencia 20. Enunciado: `docs/ENUNCIADO.md`. Diseño y decisiones: `DISEÑO.md`.

## Ruta

1. **Esqueleto:** servicio con `/v1/health`, `/run`, `/run/{id}`, `/output/{archivo}`, `/anonymize`, `/entities`
   (contrato: `docs/api/participante.openapi.yaml`), SQLite en `/data`, desplegado en Railway.
2. **Ingesta:** las tres fuentes del API → registros canónicos con el esquema de `output/`.
3. **Pipeline por reglas:** registro de personas → detección → resolución → bóveda → reemplazo →
   cuasi-identificadores → compuerta de fuga.
4. **Corrida 1 pronto**, antes de que crezcan las fuentes (vale el 30 %).
5. **Capa LLM** para el residuo; corridas 2 y 3 después del crecimiento.

El porqué de cada paso y las alternativas descartadas están en `DISEÑO.md` §7. Antes de cambiar una decisión de esa
tabla, actualízala ahí en el mismo commit.

## Vocabulario

- **Persona:** la unidad de seudonimización, con llave estable (`p_cl_<num>`, `p_kp_<num>`, `p_t_<hash>`). Dos
  personas con el mismo nombre son dos personas; la identidad sale de evidencia (cédula, correo, `CL-`, `PQR-`,
  transacción, uid, rol en el hilo), nunca del nombre solo.
- **Forma:** cada manera en que una persona aparece (nombre completo, corto, de pila, MAYÚSCULAS sin tildes, slug,
  correo, correo deletreado, cédula con y sin puntos, celular, dirección, fecha). Todas van a `originals`.
- **Perfil:** la identidad falsa completa y coherente de una persona. Cada forma se reemplaza por su equivalente
  del perfil, en el mismo estilo.
- **Bóveda:** la tabla SQLite persona → perfil. Solo agrega: un perfil asignado es para siempre. Junto con el
  registro de personas, es el **único lugar donde se aceptan datos reales**: tiene que contenerlos para resolver
  identidades. Vive en `/data` (Railway) y en su copia local; nunca entra al repo. La otra excepción es
  `entities.json`, que el jurado exige y nunca viaja con el dataset.
- **Fuga:** una forma real que sigue en la salida bajo la normalización de `contiene()` en `docs/validar.mjs`.

## Reglas del dominio

- Reemplazo **literal** de fragmentos; el resto del texto queda byte a byte igual. El LLM solo señala fragmentos.
- Se conservan: ids internos (`CL-`, `TX-`, `PQR-`, `KP-`, `INC-`), organizaciones, lugares, marcas, figuras
  públicas, fechas de negocio, montos y ciudades.
- La referencia de estilo es `sample/expected_output/`; las convenciones deducidas están en `analisis/01_sample.md`.
- `/run` y `/anonymize` llaman el mismo núcleo, así una persona recibe el mismo perfil en ambos.

## Directorios

| Ruta          | Qué hay                                                                                  |
| ------------- | ---------------------------------------------------------------------------------------- |
| `docs/`       | Material del reto: enunciado, contratos OpenAPI, `validar.mjs`. Solo lectura.            |
| `analisis/`   | Meta-análisis: `00_resumen.md` primero; chats, docs, tablas e identidad, reuso de 10102026. |
| `DISEÑO.md`   | Diseño, supuestos, decisiones, plan para medir. Entregable de Systems design.            |
| `CASOS_ESQUINA.md` | Catálogo vivo de casos esquina, cada uno con su juez. Todo caso descubierto se agrega aquí. |
| `RUBRICA.md`  | Rúbrica de la hackathon y pesos.                                                         |
| `scripts/`    | Utilidades; `descargar_fuentes.py` baja el API completo a `fuentes/`.                     |
| `fuentes/`    | Corpus real descargado. Ignorado por git.                                                |
| `sample/`     | Dataset de muestra con su salida ideal y 13 tests. Ignorado por git.                     |
| `tmp/`        | Scratch local. Ignorado por git.                                                         |

Al crear un directorio de código (`cmd/`, `internal/`…), agrégalo a esta tabla en el mismo commit.

## Herramientas

- Go 1.24+, solo `net/http` y `modernc.org/sqlite` (sin cgo). Base reusable en `../10102026` (servidor, SQLite con
  migraciones, captura de requests): ver `analisis/05_reuso_10102026.md`.
- PDF: ayudante en Python con pdfplumber (cuerpo) y pypdf (metadata), invocado como subproceso.
- LLM: Claude Haiku vía OpenRouter (`OPENROUTER_API_KEY` en `.env`).
- API de fuentes: `TOKEN` en `.env`; 600 peticiones por minuto.
- Validación: `node docs/validar.mjs output/ --public fuentes/` y `node docs/validar.mjs sample/expected_output --public sample`.
- Railway: una réplica, Serverless apagado, volumen en `/data`, healthcheck en `/v1/health`.

## Antes de dar algo por hecho

Un cambio al pipeline está listo cuando el validador da OK, el barrido de fuga da 0, los 13 tests de la muestra
pasan y el juez propio deja en verde todos los casos de `CASOS_ESQUINA.md`. Cada test compara contra un resultado esperado y cada paso de red lleva timeout.

Las corridas oficiales son tres y las lanza el usuario con VALIDAR ENTRYPOINT: propónselas, no las supongas.

## Commits

Cada paso completado termina en un commit **atómico, transaccional y convencional**:

- **Atómico:** un solo cambio lógico por commit.
- **Transaccional:** el repo compila y pasa sus tests tras cada commit.
- **Convencional:** Conventional Commits en español (`feat:`, `fix:`, `docs:`, `chore:`, `test:`, `refactor:`),
  asunto corto en imperativo.
- Agrega archivos por ruta explícita y revisa `git diff --cached --name-status` antes de commitear.
- El repo lleva código y documentación. `.env`, tokens, `fuentes/`, `sample/`, `tmp/`, `output/` y las bases SQLite
  se quedan en disco local; nunca uses `git add -f` sobre ellos. Si `git ls-files` muestra alguno, quítalo con
  `git rm --cached`, conservando el archivo.
- Los logs y capturas muestran ids de registro y conteos; tokens y datos personales van redactados.
