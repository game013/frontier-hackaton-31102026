// Command servidor es el único binario desplegado: levanta el servicio HTTP del
// contrato, con la captura de requests por fuera y apagado limpio en SIGTERM.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/captura"
	"github.com/game013/frontier-hackaton-31102026/internal/servidor"
)

const (
	// WriteTimeout cubre el lote de 50 registros con 60 s de contrato y las
	// descargas grandes de /output (>= 70 s, ver analisis/05_reuso_10102026.md).
	tiempoEscritura   = 90 * time.Second
	tiempoLectura     = 60 * time.Second
	tiempoCabeceras   = 10 * time.Second
	tiempoInactividad = 120 * time.Second
	// graciaApagado es lo que se espera al job en curso tras cancelarlo.
	graciaApagado = 20 * time.Second
)

func main() {
	log.SetOutput(os.Stderr)
	if err := corre(); err != nil {
		log.Fatal(err)
	}
}

func corre() (errFinal error) {
	cfg, err := servidor.Carga(os.Getenv)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DatosDir, 0o700); err != nil {
		return err
	}
	// La captura va al volumen con una línea por request; a stdout solo el
	// resumen (ids, códigos y conteos), porque los cuerpos traen PII.
	archivo, err := captura.Archivo(filepath.Join(cfg.DatosDir, "captura.jsonl"))
	if err != nil {
		return err
	}
	defer func() { errFinal = errors.Join(errFinal, archivo.Close()) }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// D33: la comprobación del ayudante de PDF solo va al log, nunca al
	// healthcheck ni al arranque.
	if detalle, err := servidor.CompruebaPDF(ctx, cfg.PythonBin, cfg.AyudantePDF); err != nil {
		log.Printf("pdf: aviso: %v", err)
	} else {
		log.Printf("pdf: %s", detalle)
	}

	svc, err := servidor.Nuevo(cfg, servidor.Deps{})
	if err != nil {
		return err
	}
	defer func() { errFinal = errors.Join(errFinal, svc.Cerrar()) }()

	srv := &http.Server{
		Addr:              ":" + cfg.Puerto,
		Handler:           captura.Middleware(svc.Handler(), captura.Opciones{Destinos: []io.Writer{archivo, os.Stdout}}),
		ReadHeaderTimeout: tiempoCabeceras,
		ReadTimeout:       tiempoLectura,
		WriteTimeout:      tiempoEscritura,
		IdleTimeout:       tiempoInactividad,
	}
	oyente, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	log.Printf("servidor escuchando en %s equipo=%s version=%s datos=%s",
		oyente.Addr(), cfg.Equipo, cfg.Version, cfg.DatosDir)

	terminado := make(chan error, 1)
	go func() { terminado <- srv.Serve(oyente) }()

	select {
	case err := <-terminado:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Printf("servidor: apagando")
		svc.Cerrar()
		apagado, cancelar := context.WithTimeout(context.Background(), graciaApagado)
		defer cancelar()
		if err := srv.Shutdown(apagado); err != nil {
			srv.Close()
			return err
		}
		if err := <-terminado; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
