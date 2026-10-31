#!/bin/sh
# Levanta el servicio (entregable del reto). Railway lo usa como CMD de la
# imagen: el binario viaja compilado en bin/servidor. En local, si falta, lo
# compila; para desarrollo conviene `go run ./cmd/servidor`, que deja
# DATA_DIR=var por defecto.
set -eu

# PORT lo inyecta Railway; DATA_DIR es el volumen montado (D12).
: "${PORT:=8080}"
: "${DATA_DIR:=/data}"
export PORT DATA_DIR

mkdir -p "$DATA_DIR"

if [ ! -x bin/servidor ]; then
	go build -trimpath -buildvcs=false -o bin/servidor ./cmd/servidor
fi

exec bin/servidor
