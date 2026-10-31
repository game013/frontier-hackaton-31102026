"""Extrae el cuerpo en markdown y la metadata de un PDF.

Lo invoca internal/ingesta/docs como subproceso: una llamada por documento y,
si esa falla o se pasa del timeout, una segunda en modo texto plano (D33).

    extraer.py <archivo.pdf>            cuerpo en markdown con pdfplumber
    extraer.py --texto <archivo.pdf>    cuerpo en texto plano con pypdf

Escribe en stdout un único objeto JSON: {"body": "...", "metadata": {...}}.
La metadata sale de pypdf (los 11 PDF del corpus solo traen diccionario Info):
solo las claves title, author, subject y keywords, solo si están presentes.
Ante cualquier error sale con código distinto de cero y un mensaje en stderr.
"""

import json
import re
import sys

# Claves de metadata que viajan a docs.jsonl. Se comparan por nombre en
# minúscula y sin la barra inicial del diccionario Info.
CLAVES_METADATA = ("title", "author", "subject", "keywords")

# Tamaños de fuente del generador del corpus (analisis/03_docs.md §1):
# 16 = H1, 13 = H2, 10,5 = cuerpo, 9 = celdas de tabla.
TAMANO_H1 = 15.0
TAMANO_H2 = 12.0

# Un hueco vertical mayor que esta fracción del interlineado habitual de la
# página abre un párrafo nuevo.
FACTOR_PARRAFO = 1.25

# Si la página tiene líneas que llegan al margen derecho, una línea corta cierra
# su párrafo: así se separan los bloques de encabezado y las firmas, que llevan
# el mismo interlineado que el cuerpo. La tolerancia es una fracción del ancho
# de la columna de texto. En una página sin líneas al margen (texto a bandera)
# la regla no aplica, porque ahí todas las líneas son cortas.
TOLERANCIA_MARGEN = 0.05

# Marcas de lista. El número va de uno a dos dígitos: "2026. Confidencial."
# abre una línea de cuerpo, no una lista.
MARCA_LISTA = re.compile(r"^\s*(?:[-•*]|\d{1,2}[.)])\s+")


def texto_limpio(valor):
    """Colapsa espacios y saltos de línea en un valor de texto."""
    return re.sub(r"\s+", " ", str(valor)).strip()


def metadata_pypdf(ruta):
    from pypdf import PdfReader

    with open(ruta, "rb") as archivo:
        info = PdfReader(archivo).metadata or {}
    encontrada = {}
    for clave, valor in info.items():
        nombre = str(clave).lstrip("/").lower()
        if nombre not in CLAVES_METADATA:
            continue
        limpio = texto_limpio(valor)
        if limpio:
            encontrada[nombre] = limpio
    return encontrada


def celda_texto(celda):
    return texto_limpio(celda or "")


def fila_markdown(filas):
    """Convierte las celdas de una tabla extraída en una tabla de pipes."""
    filas = [[celda_texto(c) or " " for c in fila] for fila in filas]
    filas = [f for f in filas if any(c.strip() for c in f)]
    if not filas:
        return ""
    ancho = max(len(f) for f in filas)
    filas = [f + [" "] * (ancho - len(f)) for f in filas]
    lineas = ["| " + " | ".join(filas[0]) + " |"]
    lineas.append("| " + " | ".join(["---"] * ancho) + " |")
    for fila in filas[1:]:
        lineas.append("| " + " | ".join(fila) + " |")
    return "\n".join(lineas)


def dentro_de(bbox, cajas):
    for caja in cajas:
        if bbox[1] >= caja[1] - 2 and bbox[3] <= caja[3] + 2:
            return True
    return False


def tamano_dominante(linea):
    """Tamaño de fuente que más se repite entre los caracteres de la línea."""
    cuenta = {}
    for caracter in linea.get("chars", []):
        tamano = round(caracter.get("size", 0), 1)
        cuenta[tamano] = cuenta.get(tamano, 0) + 1
    if not cuenta:
        return 0.0
    return max(cuenta.items(), key=lambda par: par[1])[0]


def nivel_titulo(tamano):
    if tamano >= TAMANO_H1:
        return 1
    if tamano >= TAMANO_H2:
        return 2
    return 0


def interlineado_habitual(lineas):
    """Distancia entre líneas que más se repite en la página."""
    cuenta = {}
    for anterior, siguiente in zip(lineas, lineas[1:]):
        distancia = round(siguiente["top"] - anterior["top"], 1)
        if distancia > 0:
            cuenta[distancia] = cuenta.get(distancia, 0) + 1
    if not cuenta:
        return 0.0
    return max(cuenta.items(), key=lambda par: par[1])[0]


def bloques_de_pagina(pagina):
    """Devuelve los bloques markdown de una página, en orden de lectura."""
    tablas = pagina.find_tables()
    cajas = [tabla.bbox for tabla in tablas]
    elementos = []
    for tabla in tablas:
        filas = tabla.extract()
        if filas:
            elementos.append((tabla.bbox[1], "tabla", filas))
    for linea in pagina.extract_text_lines():
        bbox = (linea["x0"], linea["top"], linea["x1"], linea["bottom"])
        if dentro_de(bbox, cajas):
            continue
        if not texto_limpio(linea.get("text", "")):
            continue
        elementos.append((linea["top"], "linea", linea))
    # sorted es estable: con el mismo top, la tabla va antes que sus líneas.
    elementos.sort(key=lambda elemento: elemento[0])

    lineas = [e[2] for e in elementos if e[1] == "linea"]
    interlineado = interlineado_habitual(lineas)
    margen_izquierdo = min((linea["x0"] for linea in lineas), default=0.0)
    margen_derecho = pagina.width - margen_izquierdo
    llega_al_margen = margen_derecho - TOLERANCIA_MARGEN * (margen_derecho - margen_izquierdo)
    hay_continuaciones = any(linea["x1"] >= llega_al_margen for linea in lineas)

    bloques = []
    actual = None  # {"nivel", "marca", "lineas", "top", "x1"}

    def cierra():
        if actual is None:
            return
        texto = " ".join(actual["lineas"])
        if actual["nivel"]:
            bloques.append("#" * actual["nivel"] + " " + texto)
        elif actual["marca"]:
            bloques.append(actual["marca"] + " " + texto)
        else:
            bloques.append(texto)

    for _, tipo, dato in elementos:
        if tipo == "tabla":
            cierra()
            actual = None
            tabla = fila_markdown(dato)
            if tabla:
                bloques.append(tabla)
            continue

        texto = texto_limpio(dato["text"])
        tamano = tamano_dominante(dato)
        nivel = nivel_titulo(tamano)
        marca = ""
        if not nivel:
            encontrada = MARCA_LISTA.match(texto)
            if encontrada:
                marca = encontrada.group(0).strip()
                texto = texto[encontrada.end():].strip()

        if actual is not None:
            salto = dato["top"] - actual["top"]
            nuevo = (
                nivel != actual["nivel"]
                or (marca and marca != actual["marca"])
                or (hay_continuaciones and actual["x1"] < llega_al_margen)
                or (interlineado > 0 and salto > FACTOR_PARRAFO * interlineado)
            )
            if nuevo:
                cierra()
                actual = None

        if actual is None:
            actual = {
                "nivel": nivel,
                "marca": marca,
                "lineas": [texto],
                "top": dato["top"],
                "x1": dato["x1"],
            }
        else:
            actual["lineas"].append(texto)
            actual["top"] = dato["top"]
            actual["x1"] = dato["x1"]

    cierra()
    return bloques


def cuerpo_pdfplumber(ruta):
    import pdfplumber

    bloques = []
    with pdfplumber.open(ruta) as pdf:
        for pagina in pdf.pages:
            bloques.extend(bloques_de_pagina(pagina))
    if not bloques:
        return ""
    return "\n\n".join(bloques) + "\n"


def cuerpo_pypdf(ruta):
    from pypdf import PdfReader

    with open(ruta, "rb") as archivo:
        paginas = PdfReader(archivo).pages
        partes = []
        for pagina in paginas:
            texto = pagina.extract_text() or ""
            if texto.strip():
                partes.append(texto.strip())
    if not partes:
        return ""
    return "\n\n".join(partes) + "\n"


def main(argv):
    solo_texto = "--texto" in argv
    rutas = [arg for arg in argv if not arg.startswith("--")]
    if len(rutas) != 1:
        print("uso: extraer.py [--texto] <archivo.pdf>", file=sys.stderr)
        return 2
    ruta = rutas[0]
    try:
        metadata = metadata_pypdf(ruta)
    except Exception as error:  # la metadata no puede tumbar el cuerpo
        print("metadata: %s" % error, file=sys.stderr)
        metadata = {}
    try:
        cuerpo = cuerpo_pypdf(ruta) if solo_texto else cuerpo_pdfplumber(ruta)
    except Exception as error:
        print("cuerpo: %s" % error, file=sys.stderr)
        return 1
    print(json.dumps({"body": cuerpo, "metadata": metadata}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
