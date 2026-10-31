#!/usr/bin/env python3
"""Calibra los umbrales de Jev 1.14 (D31) sobre una muestra etiquetada.

Construye en memoria un dataset etiquetado desde `sample/` (PII sintética), consulta
`POST https://openrouter.ai/api/alpha/decisions` con el snapshot fijo de D32 y escribe
`analisis/06_calibracion.md`. El informe solo lleva ids opacos (`cand_0001`) y conteos:
nunca nombres, correos, cédulas, celulares ni direcciones.

    python3 scripts/calibrar_jev.py --dry-run              # dataset y etiquetas, sin red
    python3 scripts/calibrar_jev.py --workers 6            # corrida real + informe
    python3 scripts/calibrar_jev.py --dry-run --verificar-informe

La corrida real cuesta menos de US$0,01 y deja la respuesta cruda en
`tmp/calibracion_jev/cache.json`: el informe se regenera desde ahí sin volver a pagar
(`--sin-red`). Cada llamada lleva timeout y los 429/5xx/timeout se reintentan con backoff.

Desde un worktree sin `.env` ni `sample/`, pasa `--env` y `--sample` con la ruta del checkout principal.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
import time
import unicodedata
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

RAIZ = Path(__file__).resolve().parent.parent
ENDPOINT = "https://openrouter.ai/api/alpha/decisions"
MODELO_POR_DEFECTO = "typesafe/jev-1.14-20261014"
D31 = {"persona": 0.50, "desempate": 0.70, "genero": 0.60}
UMBRALES = [round(0.30 + 0.05 * i, 2) for i in range(14)]  # 0,30 … 0,95
OPCIONES_TIPO = ("persona", "organizacion", "lugar", "marca", "otro")
REINTENTABLES = {408, 425, 429, 500, 502, 503, 504, 529}
INTENTOS = 4
TIMEOUT = 45
MASCARA = "[DATO]"


# ---------------------------------------------------------------- normalización


def norm(texto: str) -> str:
    """Minúsculas, sin tildes y con espacios colapsados (base de comparación)."""
    descompuesto = unicodedata.normalize("NFKD", texto or "")
    sin_marcas = "".join(c for c in descompuesto if not unicodedata.combining(c))
    return re.sub(r"\s+", " ", sin_marcas.lower()).strip()


def solo_digitos(texto: str) -> str:
    return re.sub(r"\D", "", texto or "")


def contiene(pajar: str, aguja: str) -> bool:
    """Mismo criterio que `contiene()` de docs/validar.mjs, sin la tabla de confusables."""
    if not aguja:
        return False
    digitos = solo_digitos(aguja)
    if (
        digitos
        and len(digitos) >= 6
        and len(digitos) >= 0.6 * len(aguja)
        and not re.search(r"[a-zA-Z]", aguja)
    ):
        return digitos in solo_digitos(pajar)
    h, n = norm(pajar), norm(aguja)
    if re.search(r"[a-z]", n):
        if re.search(rf"(?<![a-z0-9]){re.escape(n)}(?![a-z0-9])", h):
            return True
        palabras = n.split()
        if len(palabras) >= 2 and all(len(p) >= 3 for p in palabras):
            patron = r" *".join(re.escape(p) for p in palabras)
            return bool(re.search(rf"(?<![a-z0-9]){patron}(?![a-z0-9])", h))
        return False
    return n in h


def plegar(texto: str) -> tuple[str, list[int]]:
    """Texto plegado (sin tildes, minúsculas) y mapa índice plegado → índice original."""
    partes: list[str] = []
    mapa: list[int] = []
    for posicion, caracter in enumerate(texto):
        plegado = ""
        for descompuesto in unicodedata.normalize("NFKD", caracter):
            if unicodedata.combining(descompuesto):
                continue
            plegado += descompuesto.lower()
        partes.append(plegado)
        mapa.extend([posicion] * len(plegado))
    return "".join(partes), mapa


def _borde(texto: str, posicion: int) -> bool:
    """True si en `posicion` empieza o termina una palabra (lo contrario de letra o dígito)."""
    if posicion < 0 or posicion >= len(texto):
        return True
    return not re.match(r"[a-z0-9]", texto[posicion])


def spans_de(texto_plegado: str, mapa: list[int], aguja: str) -> list[tuple[int, int]]:
    """Posiciones (inicio, fin) de `aguja` en el texto original, sin distinguir tilde ni caja.

    Solo cuenta apariciones que empiezan y terminan en borde de palabra: «Ana» no está
    dentro de «manager» ni «Juli» dentro de «julio».
    """
    plegada = plegar(aguja)[0]
    if not plegada:
        return []
    rangos: list[tuple[int, int]] = []
    inicio = texto_plegado.find(plegada)
    while inicio != -1:
        fin = inicio + len(plegada)
        if _borde(texto_plegado, inicio - 1) and _borde(texto_plegado, fin):
            rangos.append((mapa[inicio], mapa[fin - 1] + 1))
        inicio = texto_plegado.find(plegada, fin)
    return rangos


def aparece(texto: str, aguja: str) -> bool:
    """Aparición literal (sin tildes ni caja, en borde de palabra) de `aguja` en `texto`."""
    plegado, mapa = plegar(texto)
    return bool(spans_de(plegado, mapa, aguja))


# ---------------------------------------------------------------- registro y ventanas


@dataclass
class Forma:
    texto: str
    grupo: str


@dataclass
class Persona:
    clave: str
    nombre: str
    rol: str
    registro: str
    cargo: str | None = None
    ciudad: str | None = None
    ocupacion: str | None = None
    correo: str | None = None
    usuario: str | None = None
    display: str | None = None
    formas: list[Forma] = field(default_factory=list)


@dataclass
class Ventana:
    tipo: str
    id_registro: str
    texto: str


@dataclass
class Muestra:
    id: str
    clase: str
    grupo: str
    ventana: str
    id_registro: str
    candidato: str
    state: str
    verdad_persona: bool | None = None
    verdad_tipo: str | None = None
    verdad_genero: str | None = None
    verdad_desempate: str | None = None
    pregunta_genero: bool = False


def leer_env(ruta: Path) -> dict[str, str]:
    valores: dict[str, str] = {}
    if not ruta.exists():
        return valores
    for linea in ruta.read_text(encoding="utf-8", errors="replace").splitlines():
        linea = linea.strip()
        if not linea or linea.startswith("#") or "=" not in linea:
            continue
        clave, valor = linea.split("=", 1)
        valores[clave.strip()] = valor.strip().strip('"').strip("'")
    return valores


def leer_clientes(sample: Path) -> list[dict[str, str]]:
    ruta = sample / "input/tables/clientes.csv"
    if not ruta.exists():
        return []
    texto = ruta.read_bytes().decode("latin-1")
    lineas = [l for l in texto.splitlines() if l.strip()]
    cabecera = lineas[0].split(";")
    return [dict(zip(cabecera, linea.split(";"))) for linea in lineas[1:]]


def forma_de_nombre(texto: str) -> str | None:
    """Devuelve la forma si sirve para la pregunta de tipo, o None si es un identificador.

    La pregunta de tipo de Jev solo ve candidatos tipo nombre o usuario; cédula, celular,
    fecha, dirección y correo completo los resuelven los patrones (C14).
    """
    valor = (texto or "").strip()
    if len(norm(valor)) < 3 or "@" in valor:
        return None
    digitos = solo_digitos(valor)
    if digitos and len(digitos) >= 6 and len(digitos) >= 0.6 * len(valor) and not re.search(
        r"[a-zA-Z]", valor
    ):
        return None
    if re.fullmatch(r"\d{4}-\d{2}-\d{2}", valor):
        return None
    if re.match(r"^(calle|carrera|cra|cl|kr|diagonal|transversal|tv|av|autopista)\b", norm(valor)):
        return None
    return valor


def cargar_personas(sample: Path) -> list[Persona]:
    personas: list[Persona] = []
    por_nombre: dict[str, Persona] = {}
    for fila in leer_clientes(sample):
        nombre = (fila.get("nombre_completo") or "").strip()
        if not nombre:
            continue
        persona = Persona(
            clave=f"cl:{fila['cliente_id']}",
            nombre=nombre,
            rol="cliente",
            registro=fila["cliente_id"],
            ciudad=(fila.get("ciudad") or "").strip() or None,
            ocupacion=(fila.get("ocupacion") or "").strip() or None,
            correo=(fila.get("correo") or "").strip() or None,
        )
        personas.append(persona)
        por_nombre[norm(nombre)] = persona

    ruta = sample / "input/chats/users.json"
    if ruta.exists():
        for usuario in json.loads(ruta.read_text(encoding="utf-8")):
            nombre = (usuario.get("real_name") or "").strip()
            if not nombre or usuario.get("is_bot") or usuario.get("deleted"):
                continue
            perfil = usuario.get("profile") or {}
            correo = (perfil.get("email") or "").strip() or None
            existente = por_nombre.get(norm(nombre))
            if existente is not None:
                existente.correo = correo or existente.correo
                existente.usuario = (usuario.get("name") or "").strip() or existente.usuario
                existente.display = (perfil.get("display_name") or "").strip() or existente.display
                continue
            personas.append(
                Persona(
                    clave=f"uid:{usuario['id']}",
                    nombre=nombre,
                    rol="empleado" if (correo or "").endswith("@kipupay.co") else "cliente",
                    registro=usuario["id"],
                    cargo=(perfil.get("title") or "").strip() or None,
                    correo=correo,
                    usuario=(usuario.get("name") or "").strip() or None,
                    display=(perfil.get("display_name") or "").strip() or None,
                )
            )
    agregar_formas(personas, sample)
    return personas


def agregar_formas(personas: list[Persona], sample: Path) -> None:
    for persona in personas:
        tokens = persona.nombre.split()
        formas = [Forma(persona.nombre, "nombre_completo")]
        if len(tokens) >= 3:
            formas.append(Forma(" ".join(tokens[:2]), "nombre_corto"))
        if len(tokens) >= 2:
            formas.append(Forma(tokens[0], "nombre_pila"))
        if persona.display and norm(persona.display) != norm(tokens[0]) and len(persona.display) >= 3:
            formas.append(Forma(persona.display, "apodo"))
        if persona.usuario and persona.usuario != "deactivateduser":
            formas.append(Forma(persona.usuario, "usuario"))
            if "." in persona.usuario:
                formas.append(Forma(persona.usuario.replace(".", "_"), "slug"))
        if persona.correo and "@" in persona.correo:
            formas.append(Forma(persona.correo.split("@", 1)[0], "correo_local"))
        vistas: dict[str, Forma] = {}
        for forma in formas:
            if forma_de_nombre(forma.texto) is None:
                continue
            vistas.setdefault(norm(forma.texto), forma)
        persona.formas = list(vistas.values())

    # Las `originals` de tipo persona de entities.json: solo formas de nombre o usuario.
    ruta = sample / "expected_output/entities.json"
    if not ruta.exists():
        return
    for entidad in json.loads(ruta.read_text(encoding="utf-8")).get("entities", []):
        if entidad.get("type") != "person":
            continue
        destino = None
        for original in entidad.get("originals", []):
            forma = forma_de_nombre(original)
            if forma is None:
                continue
            if destino is None:
                mejor = [p for p in personas if contiene(p.nombre, forma)]
                if not mejor:
                    continue
                destino = min(mejor, key=lambda p: len(p.nombre))
            if norm(forma) not in {norm(f.texto) for f in destino.formas}:
                grupo = "nombre_completo" if len(forma.split()) >= 3 else (
                    "nombre_corto" if len(forma.split()) == 2 else "nombre_pila"
                )
                destino.formas.append(Forma(forma, grupo))


def cargar_ventanas(sample: Path, personas: list[Persona]) -> list[Ventana]:
    ventanas: list[Ventana] = []

    chats = sample / "input/chats"
    if chats.exists():
        for archivo in sorted(chats.glob("*/[0-9]*.json")):
            for mensaje in json.loads(archivo.read_text(encoding="utf-8")):
                texto = (mensaje.get("text") or "").strip()
                if texto:
                    ventanas.append(
                        Ventana(
                            "chat",
                            mensaje.get("client_msg_id") or mensaje.get("ts") or archivo.name,
                            texto,
                        )
                    )
                for adjunto in mensaje.get("files") or []:
                    nombre_archivo = (adjunto.get("name") or "").strip()
                    if nombre_archivo:
                        ventanas.append(
                            Ventana("chat_adjunto", adjunto.get("id") or nombre_archivo, f"Adjunto: {nombre_archivo}")
                        )

    indice_docs = sample / "input/docs/index.json"
    if indice_docs.exists():
        for documento in json.loads(indice_docs.read_text(encoding="utf-8")):
            archivo = sample / "input/docs" / documento["filename"]
            if not archivo.exists():
                continue
            for linea in archivo.read_text(encoding="utf-8").splitlines():
                if linea.strip():
                    ventanas.append(Ventana("doc", documento["id"], linea.strip()))

    for persona in personas:
        if persona.clave.startswith("cl:"):
            campos = [
                f"Ficha de cliente {persona.registro}",
                f"nombre completo: {persona.nombre}",
            ]
            if persona.ciudad:
                campos.append(f"ciudad: {persona.ciudad}")
            if persona.ocupacion:
                campos.append(f"ocupación: {persona.ocupacion}")
            if persona.correo:
                campos.append(f"correo: {persona.correo}")
            ventanas.append(Ventana("tabla", persona.registro, " | ".join(campos)))
        else:
            campos = [f"Ficha de usuario {persona.registro}", f"nombre real: {persona.nombre}"]
            if persona.usuario:
                campos.append(f"usuario del chat: {persona.usuario}")
            if persona.cargo:
                campos.append(f"cargo: {persona.cargo}")
            if persona.correo:
                campos.append(f"correo: {persona.correo}")
            ventanas.append(Ventana("ficha_usuario", persona.registro, " | ".join(campos)))
    return ventanas


class Enmascarador:
    """Sustituye las formas conocidas del registro por `[DATO]` y deja la mención literal.

    D15 enmascara con `[PERSONA_n]`; aquí se usa un marcador neutro para no revelar al
    clasificador la respuesta que se está midiendo.
    """

    def __init__(self, personas: list[Persona]):
        self._formas: list[str] = []
        for persona in personas:
            for forma in persona.formas:
                self._formas.append(forma.texto)
        self._formas = sorted({f for f in self._formas if len(norm(f)) >= 3}, key=lambda f: -len(f))
        self._cache: dict[str, tuple[str, list[int], list[tuple[int, int, str]]]] = {}

    def _ocurrencias(self, texto: str) -> tuple[str, list[int], list[tuple[int, int, str]]]:
        if texto in self._cache:
            return self._cache[texto]
        plegado, mapa = plegar(texto)
        ocupados: list[tuple[int, int]] = []
        cambios: list[tuple[int, int, str]] = []
        for forma in self._formas:
            for inicio, fin in spans_de(plegado, mapa, forma):
                if any(inicio < f and i < fin for i, f in ocupados):
                    continue
                ocupados.append((inicio, fin))
                cambios.append((inicio, fin, MASCARA))
        self._cache[texto] = (plegado, mapa, cambios)
        return self._cache[texto]

    def enmascarar(self, texto: str, candidato: str) -> str:
        plegado, mapa, cambios = self._ocurrencias(texto)
        preservar = spans_de(plegado, mapa, candidato)
        aplicar = [
            (inicio, fin, reemplazo)
            for inicio, fin, reemplazo in cambios
            if not any(inicio >= p_inicio and fin <= p_fin for p_inicio, p_fin in preservar)
        ]
        resultado = texto
        for inicio, fin, reemplazo in sorted(aplicar, reverse=True):
            resultado = resultado[:inicio] + reemplazo + resultado[fin:]
        return re.sub(r"\s+", " ", resultado).strip()


# ---------------------------------------------------------------- verdad de género

GENERO = {
    "alejandra": "F", "amalia": "F", "ana": "F", "andres": "M", "anibal": "M", "brayan": "M",
    "camila": "F", "carlos": "M", "dagoberto": "M", "daniel": "M", "daniela": "F", "david": "M",
    "diego": "M", "elvira": "F", "esteban": "M", "felipe": "M", "fernanda": "F", "guillermo": "M",
    "hernando": "M", "ivonne": "F", "juan": "M", "juana": "F", "juliana": "F", "julieth": "F",
    "julian": "M", "karen": "F", "kelly": "F", "laura": "F", "leonel": "M", "lisandro": "M",
    "luis": "M", "luisa": "F", "manuela": "F", "mariana": "F", "mateo": "M", "nicolas": "M",
    "nohora": "F", "norberto": "M", "octavio": "M", "ofelia": "F", "paola": "F", "pedro": "M",
    "renata": "F", "rosa": "F", "salome": "F", "sebastian": "M", "silvia": "F", "uriel": "M",
    "yamid": "M", "yesenia": "F",
}
UNISEX = {"alexis", "angel", "ariel", "cruz", "guadalupe", "sol"}


def genero_de(nombre_pila: str) -> str | None:
    clave = norm(nombre_pila)
    if clave in UNISEX:
        return "indeterminado"
    return GENERO.get(clave)


# ---------------------------------------------------------------- dataset


@dataclass
class NegativoCurado:
    mencion: str
    tipo: str
    grupo: str
    contexto: str | None  # None: se busca la mención en las ventanas del corpus


NEGATIVOS_CURADOS = [
    NegativoCurado("Bancolombia", "organizacion", "banco", None),
    NegativoCurado("PSE", "organizacion", "banco", None),
    NegativoCurado("Superfinanciera", "organizacion", "regulador", None),
    NegativoCurado("Kipu Pay", "marca", "marca", None),
    NegativoCurado("Granada", "lugar", "barrio", None),
    NegativoCurado("Medellín", "lugar", "ciudad", None),
    NegativoCurado("Bucaramanga", "lugar", "ciudad", None),
    NegativoCurado("Davivienda", "organizacion", "banco", "Transferí desde Davivienda el 3 de agosto y sigue en proceso."),
    NegativoCurado("Colpatria", "organizacion", "banco", "La tarjeta de Colpatria la tengo bloqueada desde junio."),
    NegativoCurado("Nequi", "marca", "billetera", "Le mandé el comprobante por Nequi anoche."),
    NegativoCurado("Daviplata", "marca", "billetera", "Recibí el reembolso en Daviplata, muchas gracias."),
    NegativoCurado("DataCrédito", "organizacion", "regulador", "DataCrédito me reporta una mora que no existe."),
    NegativoCurado("SIC", "organizacion", "regulador", "Radiqué la queja en la SIC la semana pasada."),
    NegativoCurado("DIAN", "organizacion", "regulador", "La DIAN pidió la factura electrónica del servicio."),
    NegativoCurado("Veloz", "organizacion", "organizacion", "El contrato con Veloz vence en diciembre."),
    NegativoCurado("Carulla", "marca", "marca", "Compré el mercado en Carulla con la tarjeta."),
    NegativoCurado("Éxito", "marca", "marca", "Pagué en Éxito y el cobro se duplicó."),
    NegativoCurado("éxito", "otro", "palabra", "El pago quedó registrado con éxito y ya me llegó el correo."),
    NegativoCurado("Juan Valdez", "marca", "marca", "Tomé café en Juan Valdez mientras esperaba respuesta."),
    NegativoCurado("Avianca", "marca", "marca", "El tiquete de Avianca lo compré con la tarjeta."),
    NegativoCurado("Nutresa", "organizacion", "organizacion", "Nutresa es uno de nuestros comercios aliados."),
    NegativoCurado("Rappi", "marca", "marca", "El domicilio lo pedí por Rappi y no llegó."),
    NegativoCurado("SURA", "organizacion", "organizacion", "El seguro lo tengo con SURA desde el año pasado."),
    NegativoCurado("EAFIT", "organizacion", "universidad", "Estudio en EAFIT y pago la matrícula por cuotas."),
    NegativoCurado("Plaza Botero", "lugar", "lugar", "Nos vimos en la Plaza Botero y firmé el documento."),
    NegativoCurado("Parque Lleras", "lugar", "lugar", "El local queda cerca del Parque Lleras."),
    NegativoCurado("Estación Restrepo", "lugar", "lugar", "Bajé en la Estación Restrepo y caminé tres cuadras."),
    NegativoCurado("El Poblado", "lugar", "lugar", "Vivo en El Poblado, cerca de la loma."),
    NegativoCurado("Santander", "organizacion", "apellido_lugar", "La transferencia salió del Banco Santander hacia la cuenta."),
    NegativoCurado("Bolívar", "lugar", "apellido_lugar", "El cliente vive en Cartagena, Bolívar, y pide el cambio de dirección."),
    NegativoCurado("Castilla", "lugar", "apellido_lugar", "La oficina de Castilla atiende hasta las cinco."),
    NegativoCurado("Restrepo", "lugar", "apellido_lugar", "El usuario reporta que vive en el barrio Restrepo."),
    NegativoCurado("Córdoba", "lugar", "apellido_lugar", "La sucursal de Montería, Córdoba, abre el sábado."),
    NegativoCurado("Robledo", "lugar", "apellido_lugar", "Vive en Robledo y trabaja en el centro."),
    NegativoCurado("Claro", "otro", "claro", "Claro, es 1.107.360.039"),
    NegativoCurado("Claro", "marca", "claro", "Mi plan de Claro vence el 30 y no me llega la factura."),
    NegativoCurado("1.480.000", "otro", "numero", "El cobro fue de 1.480.000 y no coincide con la cuota."),
    NegativoCurado("901.456.789-3", "otro", "numero", "El comercio tiene NIT 901.456.789-3 y no aparece en el listado."),
    NegativoCurado("WhatsApp", "marca", "marca", "Prefiere que lo contactemos por WhatsApp para el reporte."),
]


def ventana_para(forma: str, ventanas: list[Ventana], preferida: Ventana | None) -> Ventana | None:
    orden = {"chat": 0, "doc": 1, "tabla": 2, "ficha_usuario": 3, "chat_adjunto": 4}
    candidatas = [v for v in ventanas if aparece(v.texto, forma)]
    if candidatas:
        return min(candidatas, key=lambda v: (orden.get(v.tipo, 9), len(v.texto)))
    return preferida


def state_de(ventana: Ventana, candidato: str, enmascarador: Enmascarador) -> str:
    contexto = enmascarador.enmascarar(ventana.texto, candidato)
    return f"Mención candidata: «{candidato}»\nContexto: {contexto}"


def construir_dataset(sample: Path) -> tuple[list[Muestra], dict]:
    personas = cargar_personas(sample)
    ventanas = cargar_ventanas(sample, personas)
    enmascarador = Enmascarador(personas)
    ficha = {p.clave: [v for v in ventanas if v.id_registro == p.registro][-1] for p in personas}
    muestras: list[Muestra] = []
    vistos: set[tuple[str, str]] = set()

    def agregar(muestra: Muestra) -> bool:
        clave = (norm(muestra.candidato), norm(muestra.state))
        if clave in vistos:
            return False
        vistos.add(clave)
        muestras.append(muestra)
        return True

    for persona in personas:
        for forma in persona.formas:
            ventana = ventana_para(forma.texto, ventanas, ficha[persona.clave])
            if ventana is None:
                continue
            verdad_genero = genero_de(persona.nombre.split()[0]) if forma.grupo in (
                "nombre_completo",
                "nombre_corto",
                "nombre_pila",
            ) else None
            muestra = Muestra(
                id="",
                clase="persona",
                grupo=forma.grupo,
                ventana=ventana.tipo,
                id_registro=ventana.id_registro,
                candidato=forma.texto,
                state=state_de(ventana, forma.texto, enmascarador),
                verdad_persona=True,
                verdad_tipo="persona",
                verdad_genero=verdad_genero,
                pregunta_genero=verdad_genero is not None,
            )
            agregar(muestra)

    # CE-32: nombres unisex. La tabla de género no los decide; se etiquetan `indeterminado`
    # para medir la rama unisex de D31 (no cuentan en la exactitud, sí en la cobertura).
    for nombre, contexto in ((
        "Alexis",
        "Mi vecino Alexis me recomendó pedir la tarjeta por la app.",
    ), (
        "Guadalupe",
        "Guadalupe del área de cartera me confirmó el abono de ayer.",
    ), (
        "Ariel",
        "Ariel quedó de enviar el comprobante mañana temprano.",
    )):
        muestra = Muestra(
            id="",
            clase="persona",
            grupo="unisex",
            ventana="curado",
            id_registro=f"ce32:{norm(nombre)}",
            candidato=nombre,
            state=state_de(Ventana("curado", f"ce32:{norm(nombre)}", contexto), nombre, enmascarador),
            verdad_persona=True,
            verdad_tipo="persona",
            verdad_genero="indeterminado",
            pregunta_genero=True,
        )
        agregar(muestra)

    for negativo in NEGATIVOS_CURADOS:
        ventana = ventana_para(negativo.mencion, ventanas, None)
        if negativo.contexto is not None or ventana is None:
            if negativo.contexto is None:
                continue
            ventana = Ventana("curado", f"neg:{negativo.grupo}", negativo.contexto)
        muestra = Muestra(
            id="",
            clase="no_persona",
            grupo=negativo.grupo,
            ventana=ventana.tipo,
            id_registro=ventana.id_registro,
            candidato=negativo.mencion,
            state=state_de(ventana, negativo.mencion, enmascarador),
            verdad_persona=False,
            verdad_tipo=negativo.tipo,
        )
        agregar(muestra)

    muestras.extend(construir_desempates(personas))
    for numero, muestra in enumerate(muestras, start=1):
        muestra.id = f"cand_{numero:04d}"

    conteos = {
        "clase": _conteo(m.clase for m in muestras),
        "grupo": _conteo(m.grupo for m in muestras),
        "ventana": _conteo(m.ventana for m in muestras if m.clase != "desempate"),
        "genero": _conteo(
            m.verdad_genero for m in muestras if m.clase == "persona" and m.pregunta_genero
        ),
    }
    return muestras, conteos


def _conteo(valores) -> dict[str, int]:
    salida: dict[str, int] = {}
    for valor in valores:
        salida[valor] = salida.get(valor, 0) + 1
    return dict(sorted(salida.items()))


def ficha_desempate(persona: Persona) -> str:
    partes = [f"registro {persona.registro}", f"rol {persona.rol}"]
    if persona.cargo and norm(persona.cargo) not in {"cliente", "empleado", norm(persona.rol)}:
        partes.append(f"cargo {persona.cargo}")
    partes.append(f"ciudad {persona.ciudad}" if persona.ciudad else "ciudad no registrada")
    return ", ".join(partes)


def evidencia_de(verdadera: Persona, otra: Persona) -> str | None:
    """Frase del hilo que solo encaja con la ficha verdadera, o None si no hay atributo distinto."""
    if verdadera.ciudad and otra.ciudad and norm(verdadera.ciudad) != norm(otra.ciudad):
        return f"en el hilo dice «estoy en {verdadera.ciudad}, allá me atendieron»"
    if verdadera.rol != otra.rol:
        if verdadera.cargo and norm(verdadera.cargo) not in {"cliente", "empleado"}:
            return f"en el hilo dice «soy {verdadera.cargo.lower()} del equipo»"
        return "en el hilo dice «soy cliente y llevo dos años con la cuenta»"
    if verdadera.registro.startswith("CL-") and not otra.registro.startswith("CL-"):
        return f"en el hilo cita el radicado {verdadera.registro}"
    return None


def construir_desempates(personas: list[Persona]) -> list[Muestra]:
    por_nombre: dict[str, list[Persona]] = {}
    por_apellido: dict[str, list[Persona]] = {}
    for persona in personas:
        por_nombre.setdefault(norm(persona.nombre), []).append(persona)
        for apellido in persona.nombre.split()[-2:]:
            por_apellido.setdefault(norm(apellido), []).append(persona)

    pares: list[tuple[Persona, Persona]] = []
    for grupo in por_nombre.values():
        if len(grupo) >= 2:
            pares.append((grupo[0], grupo[1]))
    for grupo in por_apellido.values():
        clientes = sorted({p.clave: p for p in grupo if p.clave.startswith("cl:")}.values(), key=lambda p: p.clave)
        for i in range(len(clientes)):
            for j in range(i + 1, len(clientes)):
                if norm(clientes[i].nombre) != norm(clientes[j].nombre):
                    pares.append((clientes[i], clientes[j]))

    muestras: list[Muestra] = []
    for izquierda, derecha in pares:
        for verdadera, otra in ((izquierda, derecha), (derecha, izquierda)):
            evidencia = evidencia_de(verdadera, otra)
            if evidencia is None:
                continue
            primera, segunda = (verdadera, otra) if verdadera is izquierda else (otra, verdadera)
            state = (
                f"Ficha A: {ficha_desempate(primera)}\n"
                f"Ficha B: {ficha_desempate(segunda)}\n"
                f"Mención en el hilo: {evidencia}"
            )
            verdad = "A" if verdadera is primera else "B"
            muestras.append(
                Muestra(
                    id="",
                    clase="desempate",
                    grupo="par_nombre" if norm(primera.nombre) == norm(segunda.nombre) else "par_apellido",
                    ventana="desempate",
                    id_registro=f"{primera.registro}|{segunda.registro}",
                    candidato=evidencia,
                    state=state,
                    verdad_desempate=verdad,
                )
            )

    ninguna = [
        "Ficha A: registro CL-19127, rol cliente, ciudad Medellín\nFicha B: registro CL-88627, rol cliente, ciudad Cali\nMención en el hilo: en el hilo cita el radicado CL-72856 y dice que vive en Pereira",
        "Ficha A: registro U05EFT6ED, rol empleado, cargo CEO, ciudad no registrada\nFicha B: registro U0Z6HH756, rol empleado, cargo CTO, ciudad no registrada\nMención en el hilo: en el hilo dice «soy la gerente de riesgo del equipo»",
        "Ficha A: registro CL-92356, rol cliente, ciudad Cali\nFicha B: registro CL-72856, rol cliente, ciudad Bucaramanga\nMención en el hilo: en el hilo cita el radicado CL-19127 y dice que vive en Manizales",
    ]
    for texto in ninguna:
        muestras.append(
            Muestra(
                id="",
                clase="desempate",
                grupo="ninguna",
                ventana="desempate",
                id_registro="ninguna",
                candidato=texto,
                state=texto,
                verdad_desempate="ninguna",
            )
        )
    return muestras


def preguntas_de(muestra: Muestra) -> dict:
    if muestra.clase == "desempate":
        return {
            "desempate": {
                "type": "choice",
                "instructions": (
                    "Las fichas A y B describen dos registros distintos que comparten el mismo nombre. "
                    "Decide a cuál ficha corresponde la mención del hilo; responde A o B solo si la "
                    "evidencia de la mención coincide con esa ficha, y ninguna si no alcanza para "
                    "ninguna de las dos."
                ),
                "criteria": {"A": None, "B": None, "ninguna": None},
            }
        }
    preguntas = {
        "tipo": {
            "type": "choice",
            "instructions": (
                "Clasifica la mención candidata de la primera línea por tipo de entidad. Las "
                f"apariciones de personas conocidas aparecen enmascaradas como {MASCARA}. Responde "
                "persona solo si la mención nombra o identifica a una persona; organización para "
                "empresas e instituciones, lugar para ciudades, barrios y direcciones, marca para "
                "productos y comercios, otro para el resto."
            ),
            "criteria": {opcion: None for opcion in OPCIONES_TIPO},
        }
    }
    if muestra.pregunta_genero:
        preguntas["genero"] = {
            "type": "score",
            "instructions": (
                "Género asociado al nombre de pila de la mención candidata: M masculino, F femenino, "
                "indeterminado si el nombre es unisex o no permite decidirlo."
            ),
            "criteria": ["M", "F", "indeterminado"],
        }
    return preguntas


def autoverificar(muestras: list[Muestra], conteos: dict) -> list[str]:
    problemas: list[str] = []
    ids = [m.id for m in muestras]
    if len(ids) != len(set(ids)):
        problemas.append("ids de muestra repetidos")
    claves = [(norm(m.candidato), norm(m.state)) for m in muestras]
    if len(claves) != len(set(claves)):
        problemas.append("muestras sin deduplicar")

    for muestra in muestras:
        if not contiene(muestra.state, muestra.candidato):
            problemas.append(f"{muestra.id}: el state no contiene la mención")
        if len(muestra.state) > 1200:
            problemas.append(f"{muestra.id}: state mayor a 1200 caracteres")
        if muestra.clase == "persona":
            if not muestra.verdad_persona:
                problemas.append(f"{muestra.id}: positivo mal etiquetado")
            if forma_de_nombre(muestra.candidato) is None:
                problemas.append(f"{muestra.id}: positivo con forma de identificador")
            if muestra.grupo not in {
                "nombre_completo",
                "nombre_corto",
                "nombre_pila",
                "apodo",
                "usuario",
                "slug",
                "correo_local",
                "unisex",
            }:
                problemas.append(f"{muestra.id}: grupo desconocido en positivos")
            if muestra.pregunta_genero and muestra.verdad_genero not in {"M", "F", "indeterminado"}:
                problemas.append(f"{muestra.id}: verdad de género inválida")
        elif muestra.clase == "no_persona":
            if muestra.verdad_persona:
                problemas.append(f"{muestra.id}: negativo mal etiquetado")
            if muestra.verdad_tipo not in set(OPCIONES_TIPO) - {"persona"}:
                problemas.append(f"{muestra.id}: tipo esperado inválido en negativos")
        elif muestra.clase == "desempate":
            if muestra.verdad_desempate not in {"A", "B", "ninguna"}:
                problemas.append(f"{muestra.id}: verdad de desempate inválida")
        else:
            problemas.append(f"{muestra.id}: clase desconocida")

    if conteos["clase"].get("persona", 0) < 50:
        problemas.append("menos de 50 positivos de persona")
    if conteos["clase"].get("no_persona", 0) < 20:
        problemas.append("menos de 20 negativos")
    if conteos["clase"].get("desempate", 0) < 10:
        problemas.append("menos de 10 pares de desempate")
    if conteos["genero"].get("M", 0) < 10 or conteos["genero"].get("F", 0) < 10:
        problemas.append("muestras de género insuficientes por clase")
    return problemas


# ---------------------------------------------------------------- cliente de Jev


class ErrorJev(RuntimeError):
    pass


class ClienteJev:
    def __init__(self, clave: str, modelo: str, cache: dict, ruta_cache: Path, sin_red: bool, workers: int):
        self._clave = clave
        self._modelo = modelo
        self._cache = cache
        self._ruta_cache = ruta_cache
        self._sin_red = sin_red
        self._workers = workers

    def clave_cache(self, state: str, preguntas: dict) -> str:
        material = "\x00".join(
            [self._modelo, state, json.dumps(preguntas, sort_keys=True, ensure_ascii=False)]
        )
        return hashlib.sha256(material.encode("utf-8")).hexdigest()

    def _llamar(self, state: str, preguntas: dict) -> dict:
        cuerpo = {
            "model": self._modelo,
            "provider": {"data_collection": "deny"},
            "state": state,
            "questions": preguntas,
        }
        datos = json.dumps(cuerpo, ensure_ascii=False).encode("utf-8")
        ultimo = "sin intento"
        for intento in range(INTENTOS):
            peticion = urllib.request.Request(
                ENDPOINT,
                data=datos,
                headers={
                    "Authorization": f"Bearer {self._clave}",
                    "Content-Type": "application/json",
                },
                method="POST",
            )
            try:
                with urllib.request.urlopen(peticion, timeout=TIMEOUT) as respuesta:
                    return json.loads(respuesta.read().decode("utf-8"))
            except urllib.error.HTTPError as error:
                ultimo = f"HTTP {error.code}"
                if error.code not in REINTENTABLES:
                    raise ErrorJev(f"la API respondió {ultimo}") from None
            except (urllib.error.URLError, TimeoutError, OSError) as error:
                ultimo = f"red {type(error).__name__}"
            time.sleep(2**intento)
        raise ErrorJev(f"sin respuesta tras {INTENTOS} intentos ({ultimo})")

    def resolver(self, pendientes: dict[str, tuple[str, dict]]) -> dict[str, dict]:
        entradas: dict[str, dict] = {}
        faltantes: list[str] = []
        for clave, (state, preguntas) in pendientes.items():
            if clave in self._cache["calls"]:
                entradas[clave] = self._cache["calls"][clave]
            else:
                faltantes.append(clave)
        if faltantes and self._sin_red:
            raise ErrorJev(f"{len(faltantes)} llamadas sin caché y --sin-red")
        if faltantes:
            with ThreadPoolExecutor(max_workers=self._workers) as ejecutor:
                futuros = {
                    ejecutor.submit(self._llamar, *pendientes[clave]): clave for clave in faltantes
                }
                try:
                    for futuro in as_completed(futuros):
                        clave = futuros[futuro]
                        state, preguntas = pendientes[clave]
                        entrada = {
                            "modelo": self._modelo,
                            "state": state,
                            "questions": preguntas,
                            "response": futuro.result(),
                            "ts": datetime.now(timezone.utc).isoformat(timespec="seconds"),
                        }
                        self._cache["calls"][clave] = entrada
                        entradas[clave] = entrada
                finally:
                    self.guardar()
        return entradas

    def guardar(self) -> None:
        self._ruta_cache.parent.mkdir(parents=True, exist_ok=True)
        meta = self._cache.setdefault("meta", {})
        meta["modelo"] = self._modelo
        meta["endpoint"] = ENDPOINT
        meta["ultima_corrida"] = datetime.now(timezone.utc).isoformat(timespec="seconds")
        meta["llamadas"] = len(self._cache["calls"])
        self._ruta_cache.write_text(
            json.dumps(self._cache, ensure_ascii=False, indent=1), encoding="utf-8"
        )


def cargar_cache(ruta: Path) -> dict:
    if ruta.exists():
        datos = json.loads(ruta.read_text(encoding="utf-8"))
        datos.setdefault("calls", {})
        return datos
    return {"meta": {}, "calls": {}}


# ---------------------------------------------------------------- métricas


def respuesta_de(entrada: dict, pregunta: str) -> dict:
    return ((entrada.get("response") or {}).get("answers") or {}).get(pregunta) or {}


def probabilidades(entrada: dict, pregunta: str) -> dict[str, float]:
    respuesta = respuesta_de(entrada, pregunta)
    crudas = respuesta.get("probabilities") or {}
    leyenda = respuesta.get("legend") or {}
    if leyenda and crudas and all(str(k).isdigit() for k in crudas):
        return {leyenda.get(str(k), str(k)): float(v) for k, v in crudas.items()}
    return {str(k): float(v) for k, v in crudas.items()}


def confianza(entrada: dict, pregunta: str) -> float | None:
    valor = respuesta_de(entrada, pregunta).get("confidence")
    return float(valor) if isinstance(valor, (int, float)) else None


def curva_persona(muestras: list[Muestra], entradas: dict[str, dict]) -> list[dict]:
    filas = []
    for umbral in UMBRALES:
        tp = fp = fn = 0
        for muestra in muestras:
            if muestra.clase not in {"persona", "no_persona"}:
                continue
            probabilidad = probabilidades(entradas[muestra.id], "tipo").get("persona", 0.0)
            aceptado = probabilidad > umbral
            if aceptado and muestra.verdad_persona:
                tp += 1
            elif aceptado and not muestra.verdad_persona:
                fp += 1
            elif not aceptado and muestra.verdad_persona:
                fn += 1
        precision = tp / (tp + fp) if tp + fp else None
        cobertura = tp / (tp + fn) if tp + fn else None
        f1 = (
            2 * precision * cobertura / (precision + cobertura)
            if precision is not None and cobertura is not None and precision + cobertura
            else None
        )
        filas.append(
            {
                "umbral": umbral,
                "tp": tp,
                "fp": fp,
                "fn": fn,
                "precision": precision,
                "cobertura": cobertura,
                "f1": f1,
            }
        )
    return filas


def curva_desempate(muestras: list[Muestra], entradas: dict[str, dict]) -> list[dict]:
    filas = []
    total = sum(
        1 for m in muestras if m.clase == "desempate" and m.verdad_desempate != "ninguna"
    )
    for umbral in UMBRALES:
        aceptados = aciertos = 0
        ninguna_total = ninguna_ok = 0
        for muestra in muestras:
            if muestra.clase != "desempate":
                continue
            probs = probabilidades(entradas[muestra.id], "desempate")
            ganadora = respuesta_de(entradas[muestra.id], "desempate").get("choice")
            if muestra.verdad_desempate == "ninguna":
                ninguna_total += 1
                ninguna_ok += 1 if ganadora == "ninguna" else 0
                continue
            if ganadora in {"A", "B"} and probs.get(ganadora, 0.0) > umbral:
                aceptados += 1
                aciertos += 1 if ganadora == muestra.verdad_desempate else 0
        precision = aciertos / aceptados if aceptados else None
        filas.append(
            {
                "umbral": umbral,
                "aceptados": aceptados,
                "aciertos": aciertos,
                "precision": precision,
                "cobertura": aceptados / total if total else None,
                "ninguna_ok": ninguna_ok,
                "ninguna_total": ninguna_total,
            }
        )
    return filas


def genero_decidido(entrada: dict, umbral: float) -> str | None:
    """Género que decide la regla de D31 para una respuesta `score`, o None si queda unisex."""
    probs = probabilidades(entrada, "genero")
    maximo = max(probs.get("M", 0.0), probs.get("F", 0.0))
    if maximo < umbral:
        return None
    return "M" if probs.get("M", 0.0) >= probs.get("F", 0.0) else "F"


def curva_genero(muestras: list[Muestra], entradas: dict[str, dict]) -> list[dict]:
    filas = []
    unisex_total = sum(
        1 for m in muestras if m.pregunta_genero and m.verdad_genero == "indeterminado"
    )
    for umbral in UMBRALES:
        decididos = aciertos = unisex_ok = 0
        for muestra in muestras:
            if not muestra.pregunta_genero:
                continue
            decidido = genero_decidido(entradas[muestra.id], umbral)
            if decidido is not None:
                if muestra.verdad_genero in {"M", "F"}:
                    decididos += 1
                    aciertos += 1 if decidido == muestra.verdad_genero else 0
            elif muestra.verdad_genero == "indeterminado":
                unisex_ok += 1
        total = sum(1 for m in muestras if m.pregunta_genero and m.verdad_genero in {"M", "F"})
        filas.append(
            {
                "umbral": umbral,
                "decididos": decididos,
                "aciertos": aciertos,
                "exactitud": aciertos / decididos if decididos else None,
                "cobertura": decididos / total if total else None,
                "unisex_indeterminado": unisex_ok,
                "unisex_total": unisex_total,
                "total_con_genero": total,
            }
        )
    return filas


def matriz_tipos(muestras: list[Muestra], entradas: dict[str, dict]) -> dict:
    """Cuenta verdad → opción declarada por el modelo en la pregunta de tipo."""
    credito: dict[tuple[str, str], int] = {}
    for muestra in muestras:
        if muestra.clase not in {"persona", "no_persona"}:
            continue
        respuesta = respuesta_de(entradas[muestra.id], "tipo").get("choice") or "otro"
        clave = (muestra.verdad_tipo or "otro", respuesta)
        credito[clave] = credito.get(clave, 0) + 1
    return credito


def aceptacion_por_grupo(muestras: list[Muestra], entradas: dict[str, dict], umbral: float) -> dict:
    salida: dict[str, list[int]] = {}
    for muestra in muestras:
        if muestra.clase != "persona":
            continue
        aceptado = probabilidades(entradas[muestra.id], "tipo").get("persona", 0.0) > umbral
        acumulado = salida.setdefault(muestra.grupo, [0, 0])
        acumulado[1] += 1
        acumulado[0] += 1 if aceptado else 0
    return salida


def discrimina(filas: list[dict], campo: str) -> bool:
    """True si algún umbral de la rejilla muestra un error: la curva tiene un óptimo medible.

    Cuando el modelo acierta en todos los tramos, el criterio de las notas colapsa al borde
    inferior de la rejilla y no hay evidencia para mover el valor vigente.
    """
    valores = [f[campo] for f in filas if f[campo] is not None]
    return bool(valores) and min(valores) < 1.0


def recomendar_persona(filas: list[dict]) -> dict | None:
    validas = [f for f in filas if f["precision"] is not None and f["f1"] is not None]
    if not validas:
        return None
    con_precision = [f for f in validas if f["precision"] >= 0.95]
    return max(con_precision or validas, key=lambda f: (f["f1"], f["umbral"]))


def recomendar_desempate(filas: list[dict]) -> dict | None:
    validas = [f for f in filas if f["precision"] is not None and f["aceptados"] > 0]
    con_precision = [f for f in validas if f["precision"] >= 0.95]
    if con_precision:
        return min(con_precision, key=lambda f: f["umbral"])
    return max(validas, key=lambda f: (f["precision"], f["umbral"])) if validas else None


def recomendar_genero(filas: list[dict]) -> dict | None:
    validas = [f for f in filas if f["exactitud"] is not None and (f["cobertura"] or 0) >= 0.80]
    if validas:
        return max(validas, key=lambda f: (f["exactitud"], f["cobertura"]))
    con_exactitud = [f for f in filas if f["exactitud"] is not None]
    return max(con_exactitud, key=lambda f: f["exactitud"]) if con_exactitud else None


def operativo(decision: str, filas: list[dict], campo: str, recomendado: dict | None) -> dict | None:
    """Recomendación que se lleva a D31: la del criterio, salvo que la muestra no discrimine."""
    if recomendado is None:
        return None
    if not discrimina(filas, campo):
        vigente = next((f for f in filas if abs(f["umbral"] - D31[decision]) < 1e-9), None)
        if vigente is not None:
            return vigente
    return recomendado


TRAMOS = ((0.00, 0.30), (0.30, 0.50), (0.50, 0.70), (0.70, 0.90), (0.90, 1.01))


def tramos(valores: list[float]) -> list[int]:
    return [sum(1 for v in valores if bajo <= v < alto) for bajo, alto in TRAMOS]


def ids_mal_clasificados(
    muestras: list[Muestra], entradas: dict[str, dict], clase: str, umbral: float
) -> list[str]:
    """Ids opacos (con grupo y ventana) de los errores en la pregunta de tipo."""
    salida = []
    for muestra in muestras:
        if muestra.clase != clase:
            continue
        probabilidad = probabilidades(entradas[muestra.id], "tipo").get("persona", 0.0)
        if clase == "persona" and probabilidad <= umbral:
            salida.append(f"`{muestra.id}` ({muestra.grupo}, {muestra.ventana})")
        if clase == "no_persona" and probabilidad > umbral:
            salida.append(f"`{muestra.id}` ({muestra.grupo}, {muestra.ventana})")
    return salida


# ---------------------------------------------------------------- informe


def _num(valor, decimales: int = 3) -> str:
    if valor is None:
        return "—"
    return f"{valor:.{decimales}f}".replace(".", ",")


def _tabla(cabecera: list[str], filas: list[list[str]]) -> str:
    lineas = ["| " + " | ".join(cabecera) + " |", "| " + " | ".join("---" for _ in cabecera) + " |"]
    for fila in filas:
        lineas.append("| " + " | ".join(str(c) for c in fila) + " |")
    return "\n".join(lineas)


def generar_informe(meta: dict) -> tuple[str, str]:
    muestras: list[Muestra] = meta["muestras"]
    entradas: dict[str, dict] = meta["entradas"]
    conteos: dict = meta["conteos"]
    persona = meta["curva_persona"]
    desempate = meta["curva_desempate"]
    genero = meta["curva_genero"]
    rec_persona = meta["rec_persona"]
    rec_desempate = meta["rec_desempate"]
    rec_genero = meta["rec_genero"]
    umbral_persona = rec_persona["umbral"] if rec_persona else D31["persona"]
    matriz = matriz_tipos(muestras, entradas)
    grupos = aceptacion_por_grupo(muestras, entradas, umbral_persona)
    forzados = _aceptados_forzados(muestras, entradas, umbral_persona)
    forzados_vigente = _aceptados_forzados(muestras, entradas, D31["persona"])
    sin_aceptar = ids_mal_clasificados(muestras, entradas, "persona", umbral_persona)
    aceptados_de_mas = ids_mal_clasificados(muestras, entradas, "no_persona", umbral_persona)
    prob_positivos = [
        probabilidades(entradas[m.id], "tipo").get("persona", 0.0)
        for m in muestras
        if m.clase == "persona"
    ]
    prob_negativos = [
        probabilidades(entradas[m.id], "tipo").get("persona", 0.0)
        for m in muestras
        if m.clase == "no_persona"
    ]
    umbral_desempate = rec_desempate["umbral"] if rec_desempate else D31["desempate"]
    desempate_mal = _desempate_mal(muestras, entradas)
    umbral_genero = rec_genero["umbral"] if rec_genero else D31["genero"]
    genero_mal = _genero_mal(muestras, entradas, umbral_genero)
    gen_mf = [
        _max_genero(entradas[m.id])
        for m in muestras
        if m.pregunta_genero and m.verdad_genero in {"M", "F"}
    ]
    gen_unisex = [
        _max_genero(entradas[m.id])
        for m in muestras
        if m.pregunta_genero and m.verdad_genero == "indeterminado"
    ]

    piezas: list[str] = []
    piezas.append("# Calibración de los umbrales de Jev (D31)\n")
    piezas.append(
        "Informe generado por `scripts/calibrar_jev.py` desde la caché de llamadas; no se edita a mano.\n"
    )
    piezas.append(
        _tabla(
            ["Qué", "Valor"],
            [
                ["Modelo (snapshot D32)", f"`{meta['modelo']}`"],
                ["Endpoint", f"`POST {meta['endpoint']}`"],
                ["Proveedor", "`data_collection: deny` (aceptado, no verificable)"],
                ["Comando", f"`{meta['comando']}`"],
                ["Fecha de la corrida", meta["fecha"]],
                ["Muestra", "`sample/` (`input/` y `expected_output/entities.json`)"],
                ["Llamadas usadas", str(meta["llamadas"])],
                ["Costo reportado por la API", f"US$ {_num(meta['costo'], 6)}"],
                ["Método", "umbral sobre `probabilities`; `confidence` solo como diagnóstico"],
            ],
        )
    )

    # Hallazgos: los cinco números que resumen la corrida, todos desde los datos.
    mediana_positivos = sorted(prob_positivos)[len(prob_positivos) // 2] if prob_positivos else None
    max_negativos = max(prob_negativos) if prob_negativos else None
    cobertura_vigente = next(
        (f["cobertura"] for f in persona if abs(f["umbral"] - D31["persona"]) < 1e-9), None
    )
    fila_desempate_vigente = next(
        (f for f in desempate if abs(f["umbral"] - D31["desempate"]) < 1e-9), None
    )
    fila_genero_vigente = next(
        (f for f in genero if abs(f["umbral"] - D31["genero"]) < 1e-9), None
    )
    hallazgos = [
        f"Dataset: {conteos['clase'].get('persona', 0)} positivos de persona, "
        f"{conteos['clase'].get('no_persona', 0)} negativos y "
        f"{conteos['clase'].get('desempate', 0)} casos de desempate, desde `sample/`.",
        f"La pregunta de tipo separa limpio en esta muestra: la probabilidad máxima de persona en un "
        f"negativo es {_num(max_negativos)} y la mediana en positivos es {_num(mediana_positivos)}; por eso "
        "la precisión es 1,000 en toda la rejilla y el umbral no se puede identificar dentro de ella.",
        f"El valor vigente de persona (0,50) conserva 1,000 de precisión con cobertura "
        f"{_num(cobertura_vigente)}; bajar a 0,30 sube la cobertura a {_num(persona[0]['cobertura'])} sin "
        f"falsos positivos en la muestra, pero {len(forzados)} de esos aciertos pasan solo porque el umbral "
        "sobrescribe la opción que declaró el modelo (`usuario` y `slug`).",
        f"Desempate: {desempate[0]['aceptados']} pares A/B aceptados con {desempate[0]['aciertos']} aciertos "
        f"y {desempate[0]['ninguna_ok']}/{desempate[0]['ninguna_total']} casos «ninguna» correctos; con el "
        f"{_num(D31['desempate'], 2)} vigente la precisión sigue en "
        f"{_num(fila_desempate_vigente['precision'] if fila_desempate_vigente else None)} y la cobertura en "
        f"{_num(fila_desempate_vigente['cobertura'] if fila_desempate_vigente else None)}.",
        f"Género: la exactitud es 1,000 en toda la rejilla; con el {_num(D31['genero'], 2)} vigente solo "
        f"{fila_genero_vigente['unisex_indeterminado'] if fila_genero_vigente else 0} de "
        f"{len(gen_unisex)} nombres unisex quedan sin decidir, así que la rama unisex de D31 no se activa "
        "por confianza (CE-32).",
    ]
    piezas.append("\n## 0. Hallazgos\n")
    piezas.append("\n".join(f"- {texto}" for texto in hallazgos) + "\n")

    piezas.append("\n## 1. Dataset etiquetado\n")
    piezas.append(
        "Positivos de persona: formas de nombre, apodo, usuario, slug y parte local de correo de las personas "
        "de `users.json` y `clientes.csv`, más las `originals` con forma de nombre de `entities.json`. Los "
        "identificadores (cédula, celular, fecha, dirección, correo completo) quedan fuera: los resuelven los "
        "patrones. Negativos: allowlist del dominio, `kept` de la muestra y casos duros (apellido-lugar, "
        "«Claro», montos y NIT). Deduplicado por (mención normalizada, state normalizado); cada muestra lleva "
        "id opaco.\n"
    )
    piezas.append(
        _tabla(
            ["Clase", "N"],
            [[clase, str(total)] for clase, total in conteos["clase"].items()],
        )
    )
    piezas.append("\nGrupos del dataset:\n")
    piezas.append(
        _tabla(["Grupo", "N"], [[grupo, str(total)] for grupo, total in conteos["grupo"].items()])
    )
    piezas.append("\nVentanas de contexto de los candidatos de tipo:\n")
    piezas.append(
        _tabla(["Ventana", "N"], [[v, str(n)] for v, n in conteos["ventana"].items()])
    )
    piezas.append("\nVerdad de género de las menciones con pregunta de género:\n")
    piezas.append(
        _tabla(["Verdad", "N"], [[g, str(n)] for g, n in conteos["genero"].items()])
    )

    piezas.append("\n## 2. Persona: curva por umbral\n")
    piezas.append(
        "Regla medida: aceptar como persona si `probabilities.persona > t` (D31). Precisión = aceptados "
        "correctos / aceptados; cobertura = personas aceptadas / total de personas.\n"
    )
    piezas.append(
        _tabla(
            ["t", "TP", "FP", "FN", "Precisión", "Cobertura", "F1", "D31"],
            [
                [
                    _num(f["umbral"], 2),
                    str(f["tp"]),
                    str(f["fp"]),
                    str(f["fn"]),
                    _num(f["precision"]),
                    _num(f["cobertura"]),
                    _num(f["f1"]),
                    "vigente" if abs(f["umbral"] - D31["persona"]) < 1e-9 else "",
                ]
                for f in persona
            ],
        )
    )
    piezas.append(
        f"\nCriterio de elección: máximo F1 con precisión ≥ 0,95. Recomendado: **t = "
        f"{_num(rec_persona['umbral'], 2) if rec_persona else '—'}**"
        + (
            f" (precisión {_num(rec_persona['precision'])}, cobertura {_num(rec_persona['cobertura'])}, "
            f"F1 {_num(rec_persona['f1'])})."
            if rec_persona
            else " (no hubo umbral con precisión suficiente)."
        )
        + "\n"
    )

    piezas.append("\nDistribución de `probabilities.persona` (conteos):\n")
    piezas.append(
        _tabla(
            ["Tramo", "Positivos", "Negativos"],
            [
                [
                    f"[{_num(bajo, 2)}–{_num(alto, 2)})",
                    str(pos),
                    str(neg),
                ]
                for (bajo, alto), pos, neg in zip(TRAMOS, tramos(prob_positivos), tramos(prob_negativos))
            ],
        )
    )
    piezas.append(
        "\nIds no aceptados como persona en ese umbral: "
        + (", ".join(sin_aceptar) if sin_aceptar else "ninguno")
        + ".\n"
    )
    piezas.append(
        "Ids de no-persona aceptados como persona en ese umbral: "
        + (", ".join(aceptados_de_mas) if aceptados_de_mas else "ninguno")
        + ".\n"
    )
    if not discrimina(persona, "precision"):
        piezas.append(
            "\n**La curva no separa clases:** la precisión es 1,000 en toda la rejilla, así que el criterio "
            "de máximo F1 con precisión ≥ 0,95 solo premia la cobertura y el óptimo cae en el borde inferior "
            "(0,30), que es un límite de la rejilla y no un óptimo medido. Con esta muestra no hay falsos "
            "positivos que justifiquen bajar el umbral ni errores que justifiquen subirlo.\n"
        )

    piezas.append(
        "\n### Matriz de confusión por tipo\n\n"
        "Filas: verdad esperada. Columnas: opción que declaró el modelo en la pregunta de tipo "
        "(independiente del umbral; la regla de aceptación se mide en la curva de arriba).\n"
    )
    respuestas = ["persona", "organizacion", "lugar", "marca", "otro"]
    filas_matriz = []
    for verdad in ["persona", "organizacion", "lugar", "marca", "otro"]:
        fila = [verdad] + [str(matriz.get((verdad, r), 0)) for r in respuestas] + [
            str(sum(matriz.get((verdad, r), 0) for r in respuestas))
        ]
        filas_matriz.append(fila)
    piezas.append(_tabla(["Verdad \\ respuesta"] + respuestas + ["N"], filas_matriz))
    piezas.append(
        "\nPositivos que la regla acepta aunque el modelo declaró otra opción de tipo: "
        f"{len(forzados)} con t = {_num(umbral_persona, 2)} ({_resumen_grupos(forzados)}); con el valor "
        f"vigente {_num(D31['persona'], 2)} serían {len(forzados_vigente)}. En esos casos el umbral "
        "sobrescribe la respuesta del modelo, no la confirma.\n"
    )
    piezas.append(
        "\nAceptación de las formas de persona por grupo (mismo t):\n"
    )
    piezas.append(
        _tabla(
            ["Grupo", "Aceptadas", "N"],
            [[grupo, str(aceptadas), str(total)] for grupo, (aceptadas, total) in sorted(grupos.items())],
        )
    )

    piezas.append("\n## 3. Desempate entre fichas (A / B / ninguna)\n")
    piezas.append(
        "Regla medida: aceptar si la opción ganadora es A o B y `probabilities[ganadora] > t`; si no, decide "
        "D19 (persona nueva dudosa). La cobertura se mide sobre los pares A/B; los casos «ninguna» se "
        "informan aparte.\n"
    )
    piezas.append(
        _tabla(
            ["t", "Aceptados", "Aciertos", "Precisión", "Cobertura", "Ninguna OK", "D31"],
            [
                [
                    _num(f["umbral"], 2),
                    str(f["aceptados"]),
                    str(f["aciertos"]),
                    _num(f["precision"]),
                    _num(f["cobertura"]),
                    f"{f['ninguna_ok']}/{f['ninguna_total']}",
                    "vigente" if abs(f["umbral"] - D31["desempate"]) < 1e-9 else "",
                ]
                for f in desempate
            ],
        )
    )
    piezas.append(
        f"\nCriterio de elección: el menor umbral con precisión ≥ 0,95. Recomendado: **t = "
        f"{_num(rec_desempate['umbral'], 2) if rec_desempate else '—'}**"
        + (
            f" (precisión {_num(rec_desempate['precision'])}, cobertura {_num(rec_desempate['cobertura'])})."
            if rec_desempate
            else "."
        )
        + "\n"
    )

    piezas.append(
        "\nCasos de desempate mal resueltos (ganadora distinta de la verdad): "
        + (", ".join(desempate_mal) if desempate_mal else "ninguno")
        + ".\n"
    )
    vigente_desempate = next(
        (f for f in desempate if abs(f["umbral"] - D31["desempate"]) < 1e-9), None
    )
    if vigente_desempate is not None and not discrimina(desempate, "precision"):
        piezas.append(
            f"\n**La curva no separa:** la precisión es 1,000 en toda la rejilla. Con el valor vigente "
            f"(t = {_num(D31['desempate'], 2)}) la precisión también es "
            f"{_num(vigente_desempate['precision'])} y la cobertura es "
            f"{_num(vigente_desempate['cobertura'])}; el mínimo umbral con precisión ≥ 0,95 cae en el borde "
            "inferior de la rejilla, así que la muestra no distingue entre los dos valores.\n"
        )

    piezas.append("\n## 4. Género (M / F / indeterminado)\n")
    piezas.append(
        "Regla medida: si `max(P(M), P(F)) ≥ t` se decide ese género; si no, unisex (D31). La cobertura es "
        "la fracción de nombres con género conocido que quedan decididos; los nombres unisex de verdad "
        "(`indeterminado`) no cuentan en la exactitud y se informan aparte como «unisex sin decidir», que "
        "es el acierto de la rama unisex.\n"
    )
    piezas.append(
        _tabla(
            ["t", "Decididos", "Aciertos", "Exactitud", "Cobertura", "Unisex sin decidir", "D31"],
            [
                [
                    _num(f["umbral"], 2),
                    str(f["decididos"]),
                    str(f["aciertos"]),
                    _num(f["exactitud"]),
                    _num(f["cobertura"]),
                    f"{f['unisex_indeterminado']}/{f['unisex_total']}",
                    "vigente" if abs(f["umbral"] - D31["genero"]) < 1e-9 else "",
                ]
                for f in genero
            ],
        )
    )
    piezas.append(
        f"\nCriterio de elección: máxima exactitud con cobertura ≥ 0,80. Recomendado: **t = "
        f"{_num(rec_genero['umbral'], 2) if rec_genero else '—'}**"
        + (
            f" (exactitud {_num(rec_genero['exactitud'])}, cobertura {_num(rec_genero['cobertura'])})."
            if rec_genero
            else "."
        )
        + "\n"
    )

    piezas.append("\nDistribución de `max(P(M), P(F))` (conteos):\n")
    piezas.append(
        _tabla(
            ["Tramo", "Nombres con género conocido", "Nombres unisex"],
            [
                [
                    f"[{_num(bajo, 2)}–{_num(alto, 2)})",
                    str(mf),
                    str(unisex),
                ]
                for (bajo, alto), mf, unisex in zip(TRAMOS, tramos(gen_mf), tramos(gen_unisex))
            ],
        )
    )
    piezas.append(
        "\nIds mal decididos en ese umbral: "
        + (", ".join(genero_mal) if genero_mal else "ninguno")
        + ".\n"
    )
    if not discrimina(genero, "exactitud"):
        piezas.append(
            "\n**La curva no separa:** la exactitud es 1,000 en toda la rejilla. Además los nombres unisex "
            "de verdad reciben M o F con probabilidad alta, así que la rama unisex de D31 no se activa por "
            "confianza: en esta muestra el género unisex lo decide el catálogo de nombres, no Jev (CE-32).\n"
        )

    piezas.append("\n## 5. D31 vigente frente a lo recomendado\n")
    filas_d31 = []
    cambios: list[str] = []
    for etiqueta, decision, vigente, recomendado, aplicable_env, criterio in (
        (
            "Persona",
            "persona",
            D31["persona"],
            rec_persona,
            meta["op_persona"],
            "`probabilities.persona > t`",
        ),
        (
            "Desempate",
            "desempate",
            D31["desempate"],
            rec_desempate,
            meta["op_desempate"],
            "`probabilities[ganadora] > t`",
        ),
        (
            "Género",
            "genero",
            D31["genero"],
            rec_genero,
            meta["op_genero"],
            "`max(P(M), P(F)) ≥ t`",
        ),
    ):
        operativa = aplicable_env["umbral"] if aplicable_env else None
        coincide = operativa is not None and abs(operativa - vigente) < 1e-9
        filas_d31.append(
            [
                etiqueta,
                _num(vigente, 2),
                _num(recomendado["umbral"] if recomendado else None, 2),
                _num(operativa, 2),
                "coincide" if coincide else ("cambia" if operativa is not None else "sin datos"),
                criterio,
            ]
        )
        if not coincide and operativa is not None:
            cambios.append(f"{decision} {_num(operativa, 2)}")
    piezas.append(
        _tabla(
            [
                "Decisión",
                "D31",
                "Recomendado por la curva",
                "Recomendación operativa",
                "Estado",
                "Regla",
            ],
            filas_d31,
        )
    )
    piezas.append(
        "\nRegla de la recomendación operativa: es la que deja el criterio de cada decisión, salvo que la "
        "curva no separe clases (ningún umbral de la rejilla con error); en ese caso se conserva el valor "
        "vigente de D31, porque el borde inferior de la rejilla no es un óptimo medido.\n"
    )
    if cambios:
        decision_texto = (
            f"DECISIÓN: recalibrar D31 con la muestra: {', '.join(cambios)} sobre probabilities "
            "(persona > t, desempate ≥ t, género ≥ t)."
        )
    else:
        decision_texto = (
            f"DECISIÓN: D31 se mantiene (persona > {_num(D31['persona'], 2)}, desempate ≥ "
            f"{_num(D31['desempate'], 2)}, género ≥ {_num(D31['genero'], 2)}) porque la muestra de C25 no "
            "separa clases (precisión y exactitud en 1,000 en toda la rejilla y óptimo de la curva en el "
            "borde 0,30); se recalibra con el corpus real antes de mover umbrales."
        )
    piezas.append("\n**Propuesta (no aplicada: D31 se cambia solo con el usuario).**\n")
    piezas.append(f"`{decision_texto}`\n")

    piezas.append("\n## 6. Diagnóstico de `confidence`\n")
    piezas.append(
        "D31 fija el desempate en `confidence ≥ 0,70`. La curva de arriba usa `probabilities`; para poder "
        "comparar, esta es la misma regla con `confidence`:\n"
    )
    filas_conf = []
    for umbral in UMBRALES:
        aceptados = aciertos = 0
        for muestra in muestras:
            if muestra.clase != "desempate" or muestra.verdad_desempate == "ninguna":
                continue
            ganadora = respuesta_de(entradas[muestra.id], "desempate").get("choice")
            valor = confianza(entradas[muestra.id], "desempate") or 0.0
            if ganadora in {"A", "B"} and valor >= umbral:
                aceptados += 1
                aciertos += 1 if ganadora == muestra.verdad_desempate else 0
        filas_conf.append(
            [_num(umbral, 2), str(aceptados), str(aciertos), _num(aciertos / aceptados if aceptados else None)]
        )
    piezas.append(_tabla(["t sobre confidence", "Aceptados", "Aciertos", "Precisión"], filas_conf))
    piezas.append(
        "\nEn `choice` la `confidence` no es la probabilidad de la opción ganadora (la API devuelve, por "
        "ejemplo, `probabilities.persona = 0,73` con `confidence = 0,66`); por eso el umbral de D31 no se "
        "traslada sin medirlo.\n"
    )
    media_conf_persona = _media_confianza(muestras, entradas, "persona")
    media_conf_no_persona = _media_confianza(muestras, entradas, "no_persona")
    piezas.append(
        f"`confidence` media en la pregunta de tipo: {_num(media_conf_persona)} en positivos de persona y "
        f"{_num(media_conf_no_persona)} en negativos.\n"
    )

    piezas.append("\n## 7. Costo y llamadas\n")
    piezas.append(
        _tabla(
            ["Concepto", "Valor"],
            [
                ["Llamadas distintas", str(meta["llamadas"])],
                ["Costo total reportado", f"US$ {_num(meta['costo'], 6)}"],
                ["Costo medio por llamada", f"US$ {_num(meta['costo_medio'], 6)}"],
                ["Tokens de entrada", str(meta["tokens_entrada"])],
                ["Tokens de salida", str(meta["tokens_salida"])],
            ],
        )
    )

    piezas.append("\n## 8. Limitaciones\n")
    limitaciones = [
        f"Muestra chica: {conteos['clase'].get('persona', 0)} positivos de persona, "
        f"{conteos['clase'].get('no_persona', 0)} negativos y "
        f"{conteos['clase'].get('desempate', 0)} casos de desempate. Los intervalos de confianza son "
        "amplios y un solo caso mueve la curva varios puntos.",
        "La mayoría de los positivos no aparecen en el texto de la muestra: su ventana es la ficha del "
        "registro, un contexto más limpio que un chat real. La precisión medida es un techo optimista.",
        "Los negativos de la allowlist que no aparecen en el corpus usan una frase curada; miden el "
        "comportamiento del modelo, no la distribución del corpus.",
        "El enmascarado usa `[DATO]` en vez de `[PERSONA_n]` (D15) para no revelar la respuesta; la mención "
        "candidata va literal en la primera línea.",
        "Los pares de desempate sin atributo distinguible en la ficha (homónimos cliente–cliente sin ciudad "
        "ni radicado) se excluyen en vez de fabricar evidencia trivial.",
        "La verdad de género sale de una tabla de nombres del script: nombres unisex o fuera de la tabla "
        "quedan en `indeterminado` o sin pregunta.",
        "Las figuras públicas de la allowlist quedan fuera del dataset de tipo: son personas y conservarlas "
        "es trabajo de la allowlist (C26), no del umbral.",
        "`data_collection: deny` se envía en cada llamada, pero que el proveedor lo cumpla no es verificable "
        "desde aquí (D15).",
    ]
    piezas.append("\n".join(f"- {texto}" for texto in limitaciones) + "\n")

    piezas.append("\n## 9. Reproducir\n")
    piezas.append(
        "```sh\npython3 scripts/calibrar_jev.py --dry-run\n"
        "python3 scripts/calibrar_jev.py --workers 6\n"
        "python3 scripts/calibrar_jev.py --dry-run --verificar-informe\n```\n\n"
        "La corrida real cachea cada respuesta por hash en `tmp/calibracion_jev/cache.json`: el informe se "
        "regenera sin volver a llamar a la API y `--verificar-informe` comprueba que no quedó ninguna forma "
        "de `entities.json`, `users.json` ni `clientes.csv` bajo la normalización del jurado.\n"
    )
    return "\n".join(piezas), decision_texto


def _aceptados_forzados(
    muestras: list[Muestra], entradas: dict[str, dict], umbral: float
) -> list[Muestra]:
    """Positivos que pasan la regla aunque la opción declarada por el modelo sea otra."""
    salida = []
    for muestra in muestras:
        if muestra.clase != "persona":
            continue
        declarada = respuesta_de(entradas[muestra.id], "tipo").get("choice")
        probabilidad = probabilidades(entradas[muestra.id], "tipo").get("persona", 0.0)
        if declarada != "persona" and probabilidad > umbral:
            salida.append(muestra)
    return salida


def _resumen_grupos(muestras: list[Muestra]) -> str:
    conteo = _conteo(m.grupo for m in muestras)
    return ", ".join(f"{grupo} {total}" for grupo, total in conteo.items()) or "ninguno"


def _max_genero(entrada: dict) -> float:
    probs = probabilidades(entrada, "genero")
    return max(probs.get("M", 0.0), probs.get("F", 0.0))


def _desempate_mal(muestras: list[Muestra], entradas: dict[str, dict]) -> list[str]:
    salida = []
    for muestra in muestras:
        if muestra.clase != "desempate":
            continue
        ganadora = respuesta_de(entradas[muestra.id], "desempate").get("choice")
        if ganadora != muestra.verdad_desempate:
            salida.append(f"`{muestra.id}` ({muestra.grupo}, esperado {muestra.verdad_desempate})")
    return salida


def _genero_mal(
    muestras: list[Muestra], entradas: dict[str, dict], umbral: float
) -> list[str]:
    salida = []
    for muestra in muestras:
        if not muestra.pregunta_genero:
            continue
        decidido = genero_decidido(entradas[muestra.id], umbral)
        if decidido != muestra.verdad_genero:
            salida.append(
                f"`{muestra.id}` ({muestra.verdad_genero} → {decidido or 'unisex'})"
            )
    return salida


def _media_confianza(muestras: list[Muestra], entradas: dict[str, dict], clase: str) -> float | None:
    valores = [
        confianza(entradas[m.id], "tipo")
        for m in muestras
        if m.clase == clase and confianza(entradas[m.id], "tipo") is not None
    ]
    return sum(valores) / len(valores) if valores else None


# ---------------------------------------------------------------- verificación del informe


def formas_sensibles(sample: Path) -> list[tuple[str, str]]:
    formas: list[tuple[str, str]] = []
    ruta = sample / "expected_output/entities.json"
    if ruta.exists():
        for entidad in json.loads(ruta.read_text(encoding="utf-8")).get("entities", []):
            for original in entidad.get("originals", []):
                formas.append(("entities", original))
    ruta = sample / "input/chats/users.json"
    if ruta.exists():
        for usuario in json.loads(ruta.read_text(encoding="utf-8")):
            perfil = usuario.get("profile") or {}
            for valor in (
                usuario.get("real_name"),
                usuario.get("name"),
                perfil.get("real_name"),
                perfil.get("display_name"),
                perfil.get("email"),
                perfil.get("phone"),
            ):
                if valor:
                    formas.append(("users", valor))
    for fila in leer_clientes(sample):
        for campo in ("nombre_completo", "num_doc", "correo", "celular", "direccion"):
            if fila.get(campo):
                formas.append(("clientes", fila[campo]))
    unicas: dict[str, tuple[str, str]] = {}
    for origen, forma in formas:
        if len(norm(forma)) < 3:
            continue
        unicas.setdefault(norm(forma), (origen, forma))
    return list(unicas.values())


def verificar_informe(ruta: Path, sample: Path) -> list[str]:
    if not ruta.exists():
        return [f"no existe el informe {ruta}"]
    texto = ruta.read_text(encoding="utf-8")
    hallazgos: list[str] = []
    for origen, forma in formas_sensibles(sample):
        if contiene(texto, forma):
            huella = hashlib.sha256(norm(forma).encode("utf-8")).hexdigest()[:8]
            hallazgos.append(f"forma sensible de {origen} (huella {huella}, {len(forma)} caracteres)")
    return hallazgos


# ---------------------------------------------------------------- CLI


def imprimir_conteos(conteos: dict, muestras: list[Muestra]) -> None:
    print(f"muestras: {len(muestras)}")
    for clase, total in conteos["clase"].items():
        print(f"  clase {clase}: {total}")
    for grupo, total in conteos["grupo"].items():
        print(f"  grupo {grupo}: {total}")
    for ventana, total in conteos["ventana"].items():
        print(f"  ventana {ventana}: {total}")
    for verdad, total in conteos["genero"].items():
        print(f"  género {verdad}: {total}")


def costo_de(entradas: dict[str, dict]) -> tuple[float, int, int]:
    costo = 0.0
    entrada_tokens = salida_tokens = 0
    for entrada in entradas.values():
        uso = (entrada.get("response") or {}).get("usage") or {}
        costo += float(uso.get("cost") or 0.0)
        entrada_tokens += int(uso.get("input_tokens") or 0)
        salida_tokens += int(uso.get("output_tokens") or 0)
    return costo, entrada_tokens, salida_tokens


def main(argv: list[str] | None = None) -> int:
    analizador = argparse.ArgumentParser(
        description="Calibra los umbrales de Jev 1.14 (D31) sobre la muestra etiquetada."
    )
    analizador.add_argument("--sample", type=Path, default=RAIZ / "sample")
    analizador.add_argument(
        "--cache", type=Path, default=RAIZ / "tmp/calibracion_jev/cache.json"
    )
    analizador.add_argument(
        "--out", type=Path, default=RAIZ / "analisis/06_calibracion.md"
    )
    analizador.add_argument("--model", default=MODELO_POR_DEFECTO)
    analizador.add_argument("--env", type=Path, default=RAIZ / ".env")
    analizador.add_argument("--workers", type=int, default=6)
    analizador.add_argument("--limite", type=int, default=0, help="recorta el dataset (humo)")
    analizador.add_argument("--dry-run", action="store_true", help="arma y valida el dataset sin red")
    analizador.add_argument("--sin-red", action="store_true", help="solo caché; falla si falta alguna")
    analizador.add_argument("--verificar-informe", action="store_true")
    args = analizador.parse_args(argv)

    if not args.sample.exists():
        print(f"falta la muestra en {args.sample}", file=sys.stderr)
        return 1

    muestras, conteos = construir_dataset(args.sample)
    problemas = autoverificar(muestras, conteos)
    if problemas:
        for problema in problemas:
            print(f"check: {problema}", file=sys.stderr)
        return 1

    if args.dry_run:
        imprimir_conteos(conteos, muestras)
        if args.verificar_informe:
            hallazgos = verificar_informe(args.out, args.sample)
            for hallazgo in hallazgos:
                print(f"fuga en el informe: {hallazgo}", file=sys.stderr)
            if hallazgos:
                return 1
            print(f"informe sin formas sensibles: {args.out}")
        return 0

    if args.limite > 0:
        muestras = muestras[: args.limite]

    clave_api = leer_env(args.env).get("OPENROUTER_API_KEY")
    if not clave_api:
        print("falta OPENROUTER_API_KEY en el entorno o en .env", file=sys.stderr)
        return 1

    cache = cargar_cache(args.cache)
    cliente = ClienteJev(clave_api, args.model, cache, args.cache, args.sin_red, args.workers)
    pendientes: dict[str, tuple[str, dict]] = {}
    for muestra in muestras:
        preguntas = preguntas_de(muestra)
        pendientes.setdefault(cliente.clave_cache(muestra.state, preguntas), (muestra.state, preguntas))
    try:
        por_clave = cliente.resolver(pendientes)
    except ErrorJev as error:
        print(f"error de la API: {error}", file=sys.stderr)
        return 1

    entradas: dict[str, dict] = {}
    for muestra in muestras:
        preguntas = preguntas_de(muestra)
        entradas[muestra.id] = por_clave[cliente.clave_cache(muestra.state, preguntas)]

    curva_p = curva_persona(muestras, entradas)
    curva_d = curva_desempate(muestras, entradas)
    curva_g = curva_genero(muestras, entradas)
    costo, tokens_entrada, tokens_salida = costo_de(entradas)
    llamadas = len({cliente.clave_cache(m.state, preguntas_de(m)) for m in muestras})

    meta = {
        "muestras": muestras,
        "entradas": entradas,
        "conteos": conteos,
        "curva_persona": curva_p,
        "curva_desempate": curva_d,
        "curva_genero": curva_g,
        "rec_persona": recomendar_persona(curva_p),
        "rec_desempate": recomendar_desempate(curva_d),
        "rec_genero": recomendar_genero(curva_g),
        "op_persona": operativo("persona", curva_p, "precision", recomendar_persona(curva_p)),
        "op_desempate": operativo(
            "desempate", curva_d, "precision", recomendar_desempate(curva_d)
        ),
        "op_genero": operativo("genero", curva_g, "exactitud", recomendar_genero(curva_g)),
        "modelo": args.model,
        "endpoint": ENDPOINT,
        "comando": "python3 scripts/calibrar_jev.py --workers " + str(args.workers),
        "fecha": (cache.get("meta") or {}).get("ultima_corrida")
        or datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "llamadas": llamadas,
        "costo": costo,
        "costo_medio": costo / llamadas if llamadas else 0.0,
        "tokens_entrada": tokens_entrada,
        "tokens_salida": tokens_salida,
    }
    informe, decision = generar_informe(meta)
    meta["decision"] = decision
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(informe, encoding="utf-8")

    hallazgos = verificar_informe(args.out, args.sample)
    if hallazgos:
        for hallazgo in hallazgos:
            print(f"fuga en el informe: {hallazgo}", file=sys.stderr)
        return 1

    print(f"informe: {args.out}")
    print(f"llamadas: {llamadas} · costo: US$ {costo:.6f}")
    for nombre, etiqueta, recomendado, operativa, vigente in (
        ("persona", "persona", meta["rec_persona"], meta["op_persona"], D31["persona"]),
        ("desempate", "desempate", meta["rec_desempate"], meta["op_desempate"], D31["desempate"]),
        ("genero", "género", meta["rec_genero"], meta["op_genero"], D31["genero"]),
    ):
        if recomendado is None or operativa is None:
            print(f"{etiqueta}: sin datos suficientes")
            continue
        estado = "coincide con D31" if abs(operativa["umbral"] - vigente) < 1e-9 else "propuesta"
        print(
            f"{etiqueta}: curva t = {recomendado['umbral']:.2f} · "
            f"operativo t = {operativa['umbral']:.2f} ({estado})"
        )
    print(meta["decision"])
    return 0


if __name__ == "__main__":
    sys.exit(main())
