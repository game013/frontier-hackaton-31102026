# Calibración de los umbrales de Jev (D31)

Informe generado por `scripts/calibrar_jev.py` desde la caché de llamadas; no se edita a mano.

| Qué | Valor |
| --- | --- |
| Modelo (snapshot D32) | `typesafe/jev-1.14-20261014` |
| Endpoint | `POST https://openrouter.ai/api/alpha/decisions` |
| Proveedor | `data_collection: deny` (aceptado, no verificable) |
| Comando | `python3 scripts/calibrar_jev.py --workers 6` |
| Fecha de la corrida | 2026-10-31T18:56:34+00:00 |
| Muestra | `sample/` (`input/` y `expected_output/entities.json`) |
| Llamadas usadas | 344 |
| Costo reportado por la API | US$ 0,006788 |
| Método | umbral sobre `probabilities`; `confidence` solo como diagnóstico |

## 0. Hallazgos

- Dataset: 290 positivos de persona, 39 negativos y 15 casos de desempate, desde `sample/`.
- La pregunta de tipo separa limpio en esta muestra: la probabilidad máxima de persona en un negativo es 0,030 y la mediana en positivos es 0,870; por eso la precisión es 1,000 en toda la rejilla y el umbral no se puede identificar dentro de ella.
- El valor vigente de persona (0,50) conserva 1,000 de precisión con cobertura 0,893; bajar a 0,30 sube la cobertura a 0,997 sin falsos positivos en la muestra, pero 29 de esos aciertos pasan solo porque el umbral sobrescribe la opción que declaró el modelo (`usuario` y `slug`).
- Desempate: 12 pares A/B aceptados con 12 aciertos y 3/3 casos «ninguna» correctos; con el 0,70 vigente la precisión sigue en 1,000 y la cobertura en 1,000.
- Género: la exactitud es 1,000 en toda la rejilla; con el 0,60 vigente solo 1 de 3 nombres unisex quedan sin decidir, así que la rama unisex de D31 no se activa por confianza (CE-32).


## 1. Dataset etiquetado

Positivos de persona: formas de nombre, apodo, usuario, slug y parte local de correo de las personas de `users.json` y `clientes.csv`, más las `originals` con forma de nombre de `entities.json`. Los identificadores (cédula, celular, fecha, dirección, correo completo) quedan fuera: los resuelven los patrones. Negativos: allowlist del dominio, `kept` de la muestra y casos duros (apellido-lugar, «Claro», montos y NIT). Deduplicado por (mención normalizada, state normalizado); cada muestra lleva id opaco.

| Clase | N |
| --- | --- |
| desempate | 15 |
| no_persona | 39 |
| persona | 290 |

Grupos del dataset:

| Grupo | N |
| --- | --- |
| apellido_lugar | 6 |
| apodo | 20 |
| banco | 3 |
| barrio | 1 |
| billetera | 2 |
| ciudad | 2 |
| claro | 2 |
| kept | 2 |
| lugar | 4 |
| marca | 7 |
| ninguna | 3 |
| nombre_completo | 59 |
| nombre_corto | 50 |
| nombre_pila | 51 |
| numero | 2 |
| organizacion | 3 |
| palabra | 1 |
| par_apellido | 4 |
| par_nombre | 8 |
| regulador | 3 |
| slug | 40 |
| unisex | 3 |
| universidad | 1 |
| usuario | 67 |

Ventanas de contexto de los candidatos de tipo:

| Ventana | N |
| --- | --- |
| chat | 10 |
| chat_adjunto | 1 |
| curado | 35 |
| doc | 4 |
| ficha_usuario | 264 |
| tabla | 15 |

Verdad de género de las menciones con pregunta de género:

| Verdad | N |
| --- | --- |
| F | 80 |
| M | 80 |
| indeterminado | 3 |

## 2. Persona: curva por umbral

Regla medida: aceptar como persona si `probabilities.persona > t` (D31). Precisión = aceptados correctos / aceptados; cobertura = personas aceptadas / total de personas.

| t | TP | FP | FN | Precisión | Cobertura | F1 | D31 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 0,30 | 289 | 0 | 1 | 1,000 | 0,997 | 0,998 |  |
| 0,35 | 288 | 0 | 2 | 1,000 | 0,993 | 0,997 |  |
| 0,40 | 281 | 0 | 9 | 1,000 | 0,969 | 0,984 |  |
| 0,45 | 276 | 0 | 14 | 1,000 | 0,952 | 0,975 |  |
| 0,50 | 259 | 0 | 31 | 1,000 | 0,893 | 0,944 | vigente |
| 0,55 | 241 | 0 | 49 | 1,000 | 0,831 | 0,908 |  |
| 0,60 | 223 | 0 | 67 | 1,000 | 0,769 | 0,869 |  |
| 0,65 | 209 | 0 | 81 | 1,000 | 0,721 | 0,838 |  |
| 0,70 | 195 | 0 | 95 | 1,000 | 0,672 | 0,804 |  |
| 0,75 | 181 | 0 | 109 | 1,000 | 0,624 | 0,769 |  |
| 0,80 | 165 | 0 | 125 | 1,000 | 0,569 | 0,725 |  |
| 0,85 | 150 | 0 | 140 | 1,000 | 0,517 | 0,682 |  |
| 0,90 | 125 | 0 | 165 | 1,000 | 0,431 | 0,602 |  |
| 0,95 | 81 | 0 | 209 | 1,000 | 0,279 | 0,437 |  |

Criterio de elección: máximo F1 con precisión ≥ 0,95. Recomendado: **t = 0,30** (precisión 1,000, cobertura 0,997, F1 0,998).


Distribución de `probabilities.persona` (conteos):

| Tramo | Positivos | Negativos |
| --- | --- | --- |
| [0,00–0,30) | 1 | 39 |
| [0,30–0,50) | 29 | 0 |
| [0,50–0,70) | 64 | 0 |
| [0,70–0,90) | 65 | 0 |
| [0,90–1,01) | 131 | 0 |

Ids no aceptados como persona en ese umbral: `cand_0215` (usuario, ficha_usuario).

Ids de no-persona aceptados como persona en ese umbral: ninguno.


**La curva no separa clases:** la precisión es 1,000 en toda la rejilla, así que el criterio de máximo F1 con precisión ≥ 0,95 solo premia la cobertura y el óptimo cae en el borde inferior (0,30), que es un límite de la rejilla y no un óptimo medido. Con esta muestra no hay falsos positivos que justifiquen bajar el umbral ni errores que justifiquen subirlo.


### Matriz de confusión por tipo

Filas: verdad esperada. Columnas: opción que declaró el modelo en la pregunta de tipo (independiente del umbral; la regla de aceptación se mide en la curva de arriba).

| Verdad \ respuesta | persona | organizacion | lugar | marca | otro | N |
| --- | --- | --- | --- | --- | --- | --- |
| persona | 260 | 1 | 0 | 0 | 29 | 290 |
| organizacion | 0 | 11 | 0 | 2 | 0 | 13 |
| lugar | 0 | 1 | 11 | 0 | 0 | 12 |
| marca | 0 | 1 | 0 | 9 | 0 | 10 |
| otro | 0 | 1 | 0 | 0 | 3 | 4 |

Positivos que la regla acepta aunque el modelo declaró otra opción de tipo: 29 con t = 0,30 (apodo 1, slug 12, usuario 16); con el valor vigente 0,50 serían 0. En esos casos el umbral sobrescribe la respuesta del modelo, no la confirma.


Aceptación de las formas de persona por grupo (mismo t):

| Grupo | Aceptadas | N |
| --- | --- | --- |
| apodo | 20 | 20 |
| nombre_completo | 59 | 59 |
| nombre_corto | 50 | 50 |
| nombre_pila | 51 | 51 |
| slug | 40 | 40 |
| unisex | 3 | 3 |
| usuario | 66 | 67 |

## 3. Desempate entre fichas (A / B / ninguna)

Regla medida: aceptar si la opción ganadora es A o B y `probabilities[ganadora] > t`; si no, decide D19 (persona nueva dudosa). La cobertura se mide sobre los pares A/B; los casos «ninguna» se informan aparte.

| t | Aceptados | Aciertos | Precisión | Cobertura | Ninguna OK | D31 |
| --- | --- | --- | --- | --- | --- | --- |
| 0,30 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,35 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,40 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,45 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,50 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,55 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,60 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,65 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,70 | 12 | 12 | 1,000 | 1,000 | 3/3 | vigente |
| 0,75 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,80 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,85 | 12 | 12 | 1,000 | 1,000 | 3/3 |  |
| 0,90 | 9 | 9 | 1,000 | 0,750 | 3/3 |  |
| 0,95 | 1 | 1 | 1,000 | 0,083 | 3/3 |  |

Criterio de elección: el menor umbral con precisión ≥ 0,95. Recomendado: **t = 0,30** (precisión 1,000, cobertura 1,000).


Casos de desempate mal resueltos (ganadora distinta de la verdad): ninguno.


**La curva no separa:** la precisión es 1,000 en toda la rejilla. Con el valor vigente (t = 0,70) la precisión también es 1,000 y la cobertura es 1,000; el mínimo umbral con precisión ≥ 0,95 cae en el borde inferior de la rejilla, así que la muestra no distingue entre los dos valores.


## 4. Género (M / F / indeterminado)

Regla medida: si `max(P(M), P(F)) ≥ t` se decide ese género; si no, unisex (D31). La cobertura tiene en el denominador todas las menciones con pregunta de género, unisex incluidos, y en el numerador todas las que la regla decide (también unisex decididos); por eso se informa el conteo de decididos. La exactitud se mide solo sobre los M/F decididos: la verdad unisex no elige lado y se informa aparte como «unisex sin decidir», que es el acierto de la rama unisex.

| t | Decididos | M/F decididos | Aciertos | Exactitud | Cobertura | Unisex sin decidir | D31 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 0,30 | 163 | 160 | 160 | 1,000 | 1,000 | 0/3 |  |
| 0,35 | 163 | 160 | 160 | 1,000 | 1,000 | 0/3 |  |
| 0,40 | 162 | 160 | 160 | 1,000 | 0,994 | 1/3 |  |
| 0,45 | 161 | 159 | 159 | 1,000 | 0,988 | 1/3 |  |
| 0,50 | 161 | 159 | 159 | 1,000 | 0,988 | 1/3 |  |
| 0,55 | 161 | 159 | 159 | 1,000 | 0,988 | 1/3 |  |
| 0,60 | 161 | 159 | 159 | 1,000 | 0,988 | 1/3 | vigente |
| 0,65 | 160 | 158 | 158 | 1,000 | 0,982 | 1/3 |  |
| 0,70 | 160 | 158 | 158 | 1,000 | 0,982 | 1/3 |  |
| 0,75 | 160 | 158 | 158 | 1,000 | 0,982 | 1/3 |  |
| 0,80 | 159 | 157 | 157 | 1,000 | 0,975 | 1/3 |  |
| 0,85 | 158 | 156 | 156 | 1,000 | 0,969 | 1/3 |  |
| 0,90 | 156 | 155 | 155 | 1,000 | 0,957 | 2/3 |  |
| 0,95 | 153 | 152 | 152 | 1,000 | 0,939 | 2/3 |  |

Criterio de elección: máxima exactitud con cobertura ≥ 0,80. Recomendado: **t = 0,30** (exactitud 1,000, cobertura 1,000).


Distribución de `max(P(M), P(F))` (conteos):

| Tramo | Nombres con género conocido | Nombres unisex |
| --- | --- | --- |
| [0,00–0,30) | 0 | 0 |
| [0,30–0,50) | 1 | 1 |
| [0,50–0,70) | 1 | 0 |
| [0,70–0,90) | 3 | 1 |
| [0,90–1,01) | 155 | 1 |

Ids mal decididos en ese umbral: `cand_0288` (indeterminado → M), `cand_0289` (indeterminado → F), `cand_0290` (indeterminado → F).


**La curva no separa:** la exactitud es 1,000 en toda la rejilla. Además los nombres unisex de verdad reciben M o F con probabilidad alta, así que la rama unisex de D31 no se activa por confianza: en esta muestra el género unisex lo decide el catálogo de nombres, no Jev (CE-32).


## 5. D31 vigente frente a lo recomendado

| Decisión | D31 | Recomendado por la curva | Recomendación operativa | Estado | Regla |
| --- | --- | --- | --- | --- | --- |
| Persona | 0,50 | 0,30 | 0,50 | coincide | `probabilities.persona > t` |
| Desempate | 0,70 | 0,30 | 0,70 | coincide | `probabilities[ganadora] > t` |
| Género | 0,60 | 0,30 | 0,60 | coincide | `max(P(M), P(F)) ≥ t` |

Regla de la recomendación operativa: es la que deja el criterio de cada decisión, salvo que la curva no separe clases (ningún umbral de la rejilla con error); en ese caso se conserva el valor vigente de D31, porque el borde inferior de la rejilla no es un óptimo medido.

Qué mide esta tabla: D31 redacta el desempate con `confidence ≥ 0,70` y la rama unisex del género con `confianza < 0,60`, mientras que la calibración mide `probabilities` (la probabilidad de la opción ganadora y `max(P(M), P(F))`). «Coincide» significa que el umbral recomendado es numéricamente el vigente, no que la regla medida sea una traducción literal del texto de D31; el §6 mide la misma regla con `confidence` para que la diferencia se vea.


**Propuesta (no aplicada: D31 se cambia solo con el usuario).**

`DECISIÓN: D31 se mantiene (persona > 0,50, desempate ≥ 0,70, género ≥ 0,60) porque la muestra de C25 no separa clases (precisión y exactitud en 1,000 en toda la rejilla y óptimo de la curva en el borde 0,30); se recalibra con el corpus real antes de mover umbrales.`


## 6. Diagnóstico de `confidence`

D31 fija el desempate en `confidence ≥ 0,70`. La curva de arriba usa `probabilities`; para poder comparar, esta es la misma regla con `confidence`:

| t sobre confidence | Aceptados | Aciertos | Precisión |
| --- | --- | --- | --- |
| 0,30 | 12 | 12 | 1,000 |
| 0,35 | 12 | 12 | 1,000 |
| 0,40 | 12 | 12 | 1,000 |
| 0,45 | 12 | 12 | 1,000 |
| 0,50 | 12 | 12 | 1,000 |
| 0,55 | 12 | 12 | 1,000 |
| 0,60 | 12 | 12 | 1,000 |
| 0,65 | 12 | 12 | 1,000 |
| 0,70 | 12 | 12 | 1,000 |
| 0,75 | 12 | 12 | 1,000 |
| 0,80 | 12 | 12 | 1,000 |
| 0,85 | 12 | 12 | 1,000 |
| 0,90 | 3 | 3 | 1,000 |
| 0,95 | 0 | 0 | — |

En `choice` la `confidence` no es la probabilidad de la opción ganadora (la API devuelve, por ejemplo, `probabilities.persona = 0,73` con `confidence = 0,66`); por eso el umbral de D31 no se traslada sin medirlo.

`confidence` media en la pregunta de tipo: 0,752 en positivos de persona y 0,834 en negativos.


## 7. Costo y llamadas

| Concepto | Valor |
| --- | --- |
| Llamadas distintas | 344 |
| Costo total reportado | US$ 0,006788 |
| Costo medio por llamada | US$ 0,000020 |
| Tokens de entrada | 161612 |
| Tokens de salida | 21543 |

## 8. Limitaciones

- Muestra chica: 290 positivos de persona, 39 negativos y 15 casos de desempate. Los intervalos de confianza son amplios y un solo caso mueve la curva varios puntos.
- La mayoría de los positivos no aparecen en el texto de la muestra: su ventana es la ficha del registro, un contexto más limpio que un chat real. La precisión medida es un techo optimista.
- Los negativos de la allowlist que no aparecen en el corpus usan una frase curada; miden el comportamiento del modelo, no la distribución del corpus.
- El enmascarado usa `[DATO]` en vez de `[PERSONA_n]` (D15) para no revelar la respuesta; la mención candidata va literal en la primera línea.
- Los pares de desempate sin atributo distinguible en la ficha (homónimos cliente–cliente sin ciudad ni radicado) se excluyen en vez de fabricar evidencia trivial.
- La verdad de género sale de una tabla de nombres del script: nombres unisex o fuera de la tabla quedan en `indeterminado` o sin pregunta.
- Las figuras públicas de la allowlist quedan fuera del dataset de tipo: son personas y conservarlas es trabajo de la allowlist (C26), no del umbral.
- `data_collection: deny` se envía en cada llamada, pero que el proveedor lo cumpla no es verificable desde aquí (D15).


## 9. Reproducir

```sh
# 1. dataset y etiquetas, sin red
python3 scripts/calibrar_jev.py --dry-run

# 2. corrida real: solo llama a la API por las claves que falten en la caché
python3 scripts/calibrar_jev.py --workers 6

# 3. regenerar el informe desde la caché: no necesita OPENROUTER_API_KEY
python3 scripts/calibrar_jev.py --sin-red --env /ruta/inexistente

# 4. comprobar que el informe no filtró ninguna forma real
python3 scripts/calibrar_jev.py --dry-run --verificar-informe
```

La corrida real cachea cada respuesta por su clave en `tmp/calibracion_jev/cache.json` y solo pide la clave de la API si hay llamadas sin caché; con la caché completa, `--sin-red` regenera el informe igual y falla si falta alguna respuesta. `--verificar-informe` busca las formas de `entities.json`, `users.json` y `clientes.csv` con el mismo `contiene()` de `docs/validar.mjs` (`CONFUSABLES`, NFKC, invisibles, ß/ς/ſ, dígitos con separadores y prefijo 57).
