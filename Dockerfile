# syntax=docker/dockerfile:1
# --- etapa 1: compila el servicio en Go sin cgo -------------------------------
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o bin/servidor ./cmd/servidor

# --- etapa 2: runtime con Python para el ayudante de PDF (D7, D33) ------------
FROM python:3.12-slim

# Versiones fijas: pdfplumber para cuerpo y tablas, pypdf para metadata y el
# respaldo de texto plano (D33).
RUN pip install --no-cache-dir pdfplumber==0.11.7 pypdf==5.6.1

WORKDIR /app
# Todo el árbol de trabajo: run.sh, el binario y tools/pdf/extraer.py cuando C7
# lo agregue. El context respeta .dockerignore (sin .env, fuentes, sample).
COPY --from=build /src /app
RUN chmod +x /app/run.sh

# Railway monta el volumen en /data: bóveda, jobs, salidas y captura.
ENV PORT=8080 \
    DATA_DIR=/data \
    PYTHON_BIN=python3 \
    PDF_HELPER=/app/tools/pdf/extraer.py

EXPOSE 8080
CMD ["./run.sh"]
