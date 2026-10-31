#!/usr/bin/env bash
# prueba_respaldo.sh — prueba de ida y vuelta del respaldo sobre una bóveda
# temporal, con el corpus de muestra (C24).
#
# Qué hace:
#   1. corre el pipeline sobre sample/input con --datos <tmp>/boveda-a;
#   2. respalda esa bóveda con scripts/boveda.sh a <tmp>/respaldos;
#   3. restaura el respaldo en <tmp>/boveda-b;
#   4. vuelve a correr el pipeline con --datos <tmp>/boveda-b y el mismo secreto;
#   5. exige que entities.json salga byte a byte igual al de boveda-a;
#   6. exige que un respaldo con destino dentro del repo se rechace.
#
# No imprime formas ni nombres: solo rutas temporales y el resultado de cada paso.
#
# Uso: scripts/prueba_respaldo.sh
set -euo pipefail

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
corpus="$raiz/sample/input"

if [[ ! -d "$corpus" ]]; then
	echo "prueba_respaldo: SALTADA, falta $corpus (está ignorado por git; baja la muestra)"
	exit 0
fi

trabajo="$(mktemp -d "${TMPDIR:-/tmp}/prueba-respaldo.XXXXXX")"
trap 'rm -rf "$trabajo"' EXIT
cd "$raiz"

echo "prueba_respaldo: compilando"
go build -o "$trabajo/respaldo" ./cmd/respaldo

echo "prueba_respaldo: corrida 1 sobre boveda-a"
go run ./cmd/pipeline --fuentes "$corpus" --datos "$trabajo/boveda-a" \
	--salida "$trabajo/salida-a" --sin-llm >/dev/null

echo "prueba_respaldo: respaldo"
BOVEDA_BINARIO="$trabajo/respaldo" scripts/boveda.sh respaldo \
	--datos "$trabajo/boveda-a" --destino "$trabajo/respaldos"

respaldo="$(find "$trabajo/respaldos" -maxdepth 1 -name 'boveda-*.db' -print)"
if [[ "$(printf '%s\n' "$respaldo" | wc -l | tr -d ' ')" != "1" ]]; then
	echo "prueba_respaldo: FALLA, el respaldo no dejó un único archivo: $respaldo"
	exit 1
fi

echo "prueba_respaldo: restauración en boveda-b"
BOVEDA_BINARIO="$trabajo/respaldo" scripts/boveda.sh restaura \
	--respaldo "$respaldo" --datos "$trabajo/boveda-b"

echo "prueba_respaldo: corrida 2 sobre boveda-b"
go run ./cmd/pipeline --fuentes "$corpus" --datos "$trabajo/boveda-b" \
	--salida "$trabajo/salida-b" --sin-llm >/dev/null

if ! cmp -s "$trabajo/salida-a/entities.json" "$trabajo/salida-b/entities.json"; then
	echo "prueba_respaldo: FALLA, entities.json no coincide tras restaurar el respaldo"
	exit 1
fi
echo "prueba_respaldo: OK, entities.json idéntico tras restaurar ($(wc -c <"$trabajo/salida-b/entities.json" | tr -d ' ') bytes)"

echo "prueba_respaldo: el destino dentro del repo tiene que rechazarse"
dentro="$raiz/tmp/respaldos-prueba-respaldo"
if BOVEDA_BINARIO="$trabajo/respaldo" scripts/boveda.sh respaldo \
	--datos "$trabajo/boveda-a" --destino "$dentro" 2>"$trabajo/error.txt"; then
	echo "prueba_respaldo: FALLA, aceptó un destino dentro del repo"
	exit 1
fi
if [[ -e "$dentro" ]]; then
	echo "prueba_respaldo: FALLA, escribió $dentro dentro del repo"
	exit 1
fi
echo "prueba_respaldo: OK, rechazado: $(cat "$trabajo/error.txt")"

echo "prueba_respaldo: todo en verde"
