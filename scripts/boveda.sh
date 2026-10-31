#!/usr/bin/env bash
# boveda.sh — respaldo y restauración de la bóveda a un disco local, fuera de git.
#
# Qué datos toca:
#   respaldo  lee <datos>/almacen.db (registro de personas y perfiles: el único
#             lugar con datos reales, junto con entities.json) y escribe
#             <destino>/boveda-<UTC>.db con la copia coherente que deja
#             `cmd/respaldo`. Nada se versiona: el destino tiene que estar fuera
#             del repo.
#   restaura  lee el archivo del respaldo y deja <datos>/almacen.db en una carpeta
#             nueva; una bóveda en uso no se pisa.
#
# No imprime formas ni nombres: solo rutas, tamaños, conteos y hashes.
#
# Enganchar tras una corrida, a mano o por cron:
#   15 16 * * * cd /srv/frontier && scripts/boveda.sh respaldo --datos /data --destino ~/respaldos
#
# Para no compilar en cada llamada, apunta BOVEDA_BINARIO al binario ya compilado
# (go build -o /usr/local/bin/respaldo ./cmd/respaldo).
set -euo pipefail

uso() {
	cat <<'FIN'
boveda: respaldo y restauración de la bóveda (personas + perfiles) fuera de git.

Uso:
  scripts/boveda.sh respaldo --datos <carpeta> --destino <carpeta>
      Deja <destino>/boveda-<UTC>.db. Falla si el destino está dentro del repo.

  scripts/boveda.sh restaura --respaldo <archivo> --datos <carpeta>
      Deja <carpeta>/almacen.db. La carpeta tiene que ser nueva.

Variables:
  BOVEDA_BINARIO  binario de cmd/respaldo ya compilado (por defecto: go run ./cmd/respaldo)
FIN
}

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ $# -eq 0 ]]; then
	uso >&2
	exit 2
fi

verbo="$1"
case "$verbo" in
respaldo | restaura)
	shift
	;;
-h | --help | ayuda)
	uso
	exit 0
	;;
*)
	echo "boveda: verbo desconocido: $verbo" >&2
	uso >&2
	exit 2
	;;
esac

if [[ -n "${BOVEDA_BINARIO:-}" ]]; then
	exec "$BOVEDA_BINARIO" "$verbo" "$@"
fi
cd "$raiz"
exec go run ./cmd/respaldo "$verbo" "$@"
