// Genera el fixture de paridad con el jurado: internal/norm/testdata/casos.json.
//
//   node internal/norm/gen_fixture.mjs > internal/norm/testdata/casos.json
//
// El bloque de funciones de abajo es copia literal de docs/validar.mjs (solo
// lectura), líneas 5–139. El port de Go (norm.go) tiene que dar exactamente los
// mismos resultados. Los casos son datos inventados: ninguno sale del corpus.

// juez/texto.ts
var ESPACIO = "\\t\\n\\v\\f\\r \\x1c-\\x1f\\x85\\xa0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000";
var PALABRA = "\\p{L}\\p{N}_";
var CONFUSABLES = {
  \u0430: "a",
  \u0435: "e",
  \u043E: "o",
  \u0440: "p",
  \u0441: "c",
  \u0445: "x",
  \u0443: "y",
  \u0456: "i",
  \u0458: "j",
  \u0455: "s",
  \u0410: "A",
  \u0415: "E",
  \u041E: "O",
  \u0420: "P",
  \u0421: "C",
  \u0425: "X",
  \u0423: "Y",
  \u0406: "I",
  \u0408: "J",
  \u0405: "S",
  \u0412: "B",
  \u041D: "H",
  \u041A: "K",
  \u041C: "M",
  \u0422: "T",
  \u03BF: "o",
  \u03B1: "a",
  \u03C1: "p",
  \u03BD: "v",
  \u03B9: "i",
  \u2113: "l",
  \u0131: "i",
  \u0251: "a"
};
var RX_CONFUSABLE = new RegExp(`[${Object.keys(CONFUSABLES).join("")}]`, "gu");
var RX_FORMATO = new RegExp("\\p{Cf}", "u");
var RX_ESPACIO_RARO = /[\p{Zs}\t\r\n\v\f]/u;
var RX_ESPACIOS = / {2,}/g;
var RX_MARCA = new RegExp("\\p{Mn}", "gu");
var RX_SEPARADOR_DIGITOS = new RegExp(`(?<=\\p{Nd})[${ESPACIO}.,'\u2019\`\xB4\xB7/\\-\u2013\u2014_](?=\\p{Nd})`, "gu");
var RX_NO_DIGITO = new RegExp("\\P{Nd}", "gu");
var RX_TEXTO = new RegExp(`^[${PALABRA} .'_-]+$`, "u");
var RX_TOKENS = new RegExp(`[${PALABRA}]+|[^${PALABRA}${ESPACIO}]`, "gu");
var RX_BLANCOS = new RegExp(`[${ESPACIO}]+`, "u");
function escaparRegex(s) {
  return s.replace(/[.*+?^${}()|[\]\\/]/g, "\\$&");
}
function partir(s) {
  return s.split(RX_BLANCOS).filter((x) => x !== "");
}
function largo(s) {
  let n = 0;
  for (const _ of s) n++;
  return n;
}
function plegar(s) {
  return s.toLowerCase().replace(/ß/g, "ss").replace(/ς/g, "\u03C3").replace(/ſ/g, "s");
}
function quitarTildes(s) {
  return s.normalize("NFD").replace(RX_MARCA, "");
}
function clean(s) {
  const base = s.normalize("NFKC").replace(RX_CONFUSABLE, (c) => CONFUSABLES[c] ?? c);
  let out = "";
  for (const c of base) {
    if (RX_FORMATO.test(c)) continue;
    out += RX_ESPACIO_RARO.test(c) ? " " : c;
  }
  return out.replace(RX_ESPACIOS, " ");
}
var cacheNorm = /* @__PURE__ */ new Map();
function norm(s) {
  let r = cacheNorm.get(s);
  if (r === void 0) {
    r = plegar(quitarTildes(clean(s)));
    if (cacheNorm.size > 2e5) cacheNorm.clear();
    cacheNorm.set(s, r);
  }
  return r;
}
var cacheCompacto = /* @__PURE__ */ new Map();
function digitosCompactos(s) {
  let r = cacheCompacto.get(s);
  if (r === void 0) {
    r = clean(s).replace(RX_SEPARADOR_DIGITOS, "");
    if (cacheCompacto.size > 2e5) cacheCompacto.clear();
    cacheCompacto.set(s, r);
  }
  return r;
}
function soloDigitos(s) {
  return s.replace(RX_NO_DIGITO, "");
}
var cacheRx = /* @__PURE__ */ new Map();
function rx(fuente, flags = "u") {
  const clave = `${flags}/${fuente}`;
  let r = cacheRx.get(clave);
  if (!r) {
    r = new RegExp(fuente, flags);
    if (cacheRx.size > 5e4) cacheRx.clear();
    cacheRx.set(clave, r);
  }
  return r;
}
function digitosEn(haystack, digitos) {
  const compacto = digitosCompactos(haystack);
  const candidatos = /* @__PURE__ */ new Set([digitos]);
  if (digitos.length === 12 && digitos.startsWith("57")) candidatos.add(digitos.slice(2));
  for (const d of candidatos)
    if (rx(`(?<!\\p{Nd})(?:57)?${escaparRegex(d)}(?!\\p{Nd})`).test(compacto)) return true;
  return false;
}
function contiene(haystack, needle, opciones = {}) {
  if (!needle) return false;
  if (opciones.exacto) return haystack.includes(needle);
  const digitos = soloDigitos(needle);
  if (digitos && largo(digitos) >= 6 && largo(digitos) >= 0.6 * largo(needle) && !/[a-zA-Z]/.test(needle))
    return digitosEn(haystack, digitos);
  const h = norm(haystack);
  const n = norm(needle);
  if (RX_TEXTO.test(n) && /[a-z]/.test(n)) {
    if (rx(`(?<![a-z0-9])${escaparRegex(n)}(?![a-z0-9])`).test(h)) return true;
    const palabras = partir(n);
    if (palabras.length >= 2 && palabras.every((w) => largo(w) >= 3))
      return rx(
        `(?<![a-z0-9])${palabras.map(escaparRegex).join(`[${ESPACIO}]*`)}(?![a-z0-9])`
      ).test(h);
    return false;
  }
  return h.includes(n);
}

// --- Casos -----------------------------------------------------------------
// [haystack, needle, nota]

const CASOS_CONTENIDO = [
  ["González Pérez", "gonzalez perez", "mayúsculas sin tildes"],
  ["GONZÁLEZ PÉREZ", "González Pérez", "needle con tildes"],
  ["González Pérez", "Gonza\u0301lez Pe\u0301rez", "needle descompuesto"],
  ["Muñoz Ríos", "munoz rios", "eñe y tildes"],
  ["Ana   María", "ana maria", "espacios repetidos en el haystack"],
  ["Ana\tMaría", "ana maria", "tabulador"],
  ["Ana\nMaría", "ana maria", "salto de línea"],
  ["Ana\u00a0María", "ana maria", "espacio duro"],
  ["Ana\u2003María", "ana maria", "espacio em"],
  ["Ana\u200bMaría", "anamaría", "formato Cf eliminado"],
  ["Anastasia", "ana", "palabra dentro de otra palabra"],
  ["Ana", "ana", "igual"],
  ["ANA_PEREZ", "ana_perez", "guion bajo es carácter de palabra"],
  ["anaperez", "ana_perez", "guion bajo ausente"],
  ["juan.perez@kipupay.co", "JUAN.PEREZ@KIPUPAY.CO", "correo completo"],
  ["Juan.Pérez", "juan.perez", "punto entre nombre y apellido"],
  ["juan.perezr", "juan.perez", "el punto es prefijo de otra palabra"],
  ["escribió: juan.perez@kipupay.co", "juan.perez", "el correo no cierra la palabra"],
  ["s.a.s. kipu", "S.A.S.", "siglas con puntos"],
  ["folio #12345", "#12345", "símbolo fuera de RX_TEXTO"],
  ["WEIß", "weiss", "eszett a dos eses"],
  ["Straße", "strasse", "eszett con mayúscula inicial"],
  ["ΟΔΥΣΣΕΥΣ", "οδυσσευσ", "sigma final del griego"],
  ["ΣΟΦΙΑ", "σοφια", "griego con mayúsculas"],
  ["Vasyl", "Vаsyl", "confusable cirílico а en el needle"],
  ["Ѕmith", "smith", "confusable cirílico Ѕ en el haystack"],
  ["Íñigo", "inigo", "í y ñ"],
  ["İstanbul", "istanbul", "i con punto"],
  ["Ǎna Lucía", "ana lucia", "caron"],
  ["François", "francois", "cedilla"],
  ["ＡＮＡ", "ana", "ancho completo en el haystack"],
  ["Ana", "ＡＮＡ", "ancho completo en el needle"],
  ["ana maria", "ANA MARIA", "mayúsculas en el needle"],
  ["anamaria", "ana maria", "el jurado acepta palabras pegadas"],
  ["Ana-María", "ana maria", "guion no es espacio"],
  ["ana maria", "ana ma", "palabra de dos letras"],
  ["ana maría gómez", "maria gomez", "dos palabras contiguas"],
  ["Ana Maria", "Maria Gómez", "falta una palabra"],
  ["Angélica", "angelica", "eñe no, tilde sí"],
  ["JOSÉ  ÁNGEL", "jose angel", "dos nombres con tildes"],
  [" ana ", " ana ", "espacios en los bordes"],
  ["ana", " ana", "espacio inicial que el haystack no tiene"],
  ["", "ana", "haystack vacío"],
  ["ana", "", "needle vacío"],
  ["su cédula 1.020.304.050 quedó registrada", "1020304050", "cédula con puntos"],
  ["su cédula 1020304050 quedó registrada", "1.020.304.050", "needle con puntos"],
  ["su cédula 1-020-304-050", "1020304050", "separador guion"],
  ["su cédula 1 020 304 050", "1020304050", "separador espacio"],
  ["su cédula 1_020_304_050", "1020304050", "separador guion bajo"],
  ["su cédula 1/020/304/050", "1020304050", "separador barra"],
  ["su cédula 1–020–304–050", "1020304050", "separador raya"],
  ["su cédula 1´020´304´050", "1020304050", "separador acento agudo"],
  ["su cédula 1·020·304·050", "1020304050", "separador punto medio"],
  ["su cédula 1,020,304,050", "1020304050", "separador coma"],
  ["su cédula 10203040501", "1020304050", "un dígito de más al final"],
  ["su cédula 11020304050", "1020304050", "un dígito de más al inicio"],
  ["tel +57 300 123 4567", "3001234567", "celular con prefijo país"],
  ["tel +573001234567", "3001234567", "celular E164"],
  ["tel 3001234567", "573001234567", "needle con 57 y haystack sin él"],
  ["tel +57 300 123 4567", "573001234567", "needle de doce dígitos"],
  ["celular +57 (300) 123-4567", "3001234567", "paréntesis no son separadores"],
  ["valor 1.234.567", "1234567", "siete dígitos"],
  ["valor 1234567", "1.234.567", "needle de siete dígitos con puntos"],
  ["factura 12345", "1234", "menos de seis dígitos"],
  ["CC-1020304050", "CC1020304050", "letras y dígitos: no compacta"],
  ["valor ١٢٣٤٥٦", "١٢٣٤٥٦", "dígitos arábigo-índicos"],
  ["cédula 1 . 0 2 0 3 0 4", "1 . 0 2 0 3 0 4", "dígitos espaciados: cae a la rama de texto"],
  ["cédula 1020304", "1 . 0 2 0 3 0 4", "la proporción de dígitos no alcanza"],
  ["cédula 1.2.3.4.5.6", "123456.....", "la proporción de dígitos manda sobre el compactado"],
  ["cédula 1.2.3.4.5.6", "123456    ", "relleno con espacios"],
  ["folio 123456", "123456", "seis dígitos justos"],
  ["folio 1234567", "123456", "seis dígitos con otro pegado"],
  ["valor 000123456", "123456", "ceros a la izquierda no estorban"],
  ["Pedido 4500123 de Ana", "123", "tres dígitos dentro de un número"],
];

const CASOS_NORMALIZACION = [
  ["González  Pérez", "tildes y espacios repetidos"],
  ["ＡＮＡ", "ancho completo"],
  ["İstanbul", "i con punto arriba"],
  ["Weiß", "eszett"],
  ["STRAẞE", "eszett mayúscula"],
  ["ΟΔΥΣΣΕΥΣ", "sigma final"],
  ["Vаsyl", "confusable cirílico"],
  ["Ana\u200bMaría", "formato Cf"],
  ["Ana\u00a0María", "espacio duro"],
  ["Ana\tMaría\r\n", "controles de espacio"],
  ["Muñoz", "eñe"],
  ["Ǎna", "caron"],
  ["e\u0301", "e + acento combinado"],
  ["1.020.304", "dígitos con puntos"],
  ["ﬁn", "ligadura fi"],
  ["ℓ", "ele de script"],
  ["½", "fracción vulgar"],
  ["①", "dígito en círculo"],
  ["㎏", "símbolo de kilogramo"],
  ["ｶ", "katakana de ancho medio"],
  ["№", "número abreviado"],
  ["Ⅻ", "numeral romano"],
  ["ẛ", "ese larga con punto"],
  ["Å", "signo de angstrom"],
  ["ǅ", "dígrafo con título"],
  ["  doble   espacio  ", "espacios en los bordes"],
  ["Ὰ", "alfa con acento y ypogegrammeni"],
  ["α\u0301", "alfa + acento combinado"],
  ["ッ", "katakana pequeño"],
  ["2²", "superíndice"],
];

const CASOS_DIGITOS = [
  ["cédula 1.020.304.050", "1020304050", "separadores"],
  ["cédula 1020304050", "1020304050", "sin separadores"],
  ["cédula 10203040501", "1020304050", "dígito vecino al final"],
  ["cédula 11020304050", "1020304050", "dígito vecino al inicio"],
  ["tel +57 300 123 4567", "573001234567", "doce dígitos"],
  ["tel 3001234567", "573001234567", "candidato sin el 57"],
  [" CC-1020304050 ", "1020304050", "con texto alrededor"],
  ["no hay", "123456", "sin coincidencia"],
  ["valor 1 234 567", "1234567", "separado por espacios"],
  ["valor 1.234.567-8", "1234567", "dígito extra tras guion"],
];

const fixture = {
  fuente: "docs/validar.mjs",
  comando: "node internal/norm/gen_fixture.mjs > internal/norm/testdata/casos.json",
  contiene: CASOS_CONTENIDO.map(([haystack, needle, nota]) => ({
    haystack,
    needle,
    resultado: contiene(haystack, needle),
    nota,
  })),
  normalizacion: CASOS_NORMALIZACION.map(([entrada, nota]) => ({
    entrada,
    clean: clean(entrada),
    quitarTildes: quitarTildes(entrada),
    plegar: plegar(entrada),
    norm: norm(entrada),
    nota,
  })),
  digitosEn: CASOS_DIGITOS.map(([haystack, digitos, nota]) => ({
    haystack,
    digitos,
    resultado: digitosEn(haystack, digitos),
    nota,
  })),
};

process.stdout.write(`${JSON.stringify(fixture, null, 2)}\n`);
