package servidor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Limites de la corrida: el contrato da 2 horas; se cortan 10 minutos antes
// para responder a tiempo con failed en vez de quedar colgada.
const plazoCorrida = 110 * time.Minute

// Estado es el ciclo de vida de una corrida, tal como lo enumera el contrato.
type Estado string

const (
	EstadoEnCola    Estado = "queued"
	EstadoCorriendo Estado = "running"
	EstadoHecha     Estado = "done"
	EstadoFallida   Estado = "failed"
)

// Conteos es cuántos registros por fuente produjo una corrida.
type Conteos struct {
	Chats  int
	Docs   int
	Tablas int
}

// Ejecutor corre el pipeline de /run: descarga las fuentes, ingesta, pasa todo
// por el núcleo y deja los cuatro archivos en dirSalida. C18 cablea el real.
type Ejecutor interface {
	Ejecuta(ctx context.Context, sourcesURL, token, dirSalida string) (Conteos, error)
}

// EjecutorPendiente mantiene /run honesto mientras el pipeline no está
// cableado: el job termina failed con la causa a la vista.
type EjecutorPendiente struct{}

func (EjecutorPendiente) Ejecuta(context.Context, string, string, string) (Conteos, error) {
	return Conteos{}, errors.New("pipeline de /run sin cablear: llega con C4, C7 y C18")
}

type corrida struct {
	id        string
	estado    Estado
	encolada  time.Time
	iniciada  time.Time
	terminada time.Time
	conteos   Conteos
	segundos  float64
	err       error
	dir       string
}

// corridas guarda los jobs en memoria: una réplica, un proceso (D12). C2 los
// lleva a SQLite para que un reinicio no los pierda.
type corridas struct {
	mu       sync.Mutex
	ctx      context.Context
	raiz     string
	ejecutor Ejecutor
	porID    map[string]*corrida
	activa   *corrida
	ultima   *corrida
	wg       sync.WaitGroup
	ahora    func() time.Time
}

func nuevoCorridas(ctx context.Context, datosDir string, ejecutor Ejecutor) *corridas {
	return &corridas{
		ctx:      ctx,
		raiz:     datosDir,
		ejecutor: ejecutor,
		porID:    map[string]*corrida{},
		ahora:    time.Now,
	}
}

// enCola crea el job y lo corre en su goroutine. Con una corrida activa
// devuelve esa misma: dos /run simultáneos no compiten por la bóveda.
func (c *corridas) enCola(sourcesURL, token string) *corrida {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.activa != nil {
		return c.activa
	}
	id := nuevoID()
	cor := &corrida{
		id:       id,
		estado:   EstadoEnCola,
		encolada: c.ahora(),
		dir:      filepath.Join(c.raiz, "salidas", id),
	}
	c.porID[id] = cor
	c.activa = cor
	c.wg.Add(1)
	go c.ejecuta(cor, sourcesURL, token)
	return cor
}

func (c *corridas) ejecuta(cor *corrida, sourcesURL, token string) {
	defer c.wg.Done()
	c.mu.Lock()
	cor.estado = EstadoCorriendo
	cor.iniciada = c.ahora()
	c.mu.Unlock()

	if err := os.MkdirAll(cor.dir, 0o700); err != nil {
		c.termina(cor, Conteos{}, fmt.Errorf("prepara la carpeta de salida: %w", err))
		return
	}
	ctx, cancel := context.WithTimeout(c.ctx, plazoCorrida)
	defer cancel()
	conteos, err := c.ejecutor.Ejecuta(ctx, sourcesURL, token, cor.dir)
	c.termina(cor, conteos, err)
}

func (c *corridas) termina(cor *corrida, conteos Conteos, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cor.terminada = c.ahora()
	cor.conteos = conteos
	if !cor.iniciada.IsZero() {
		cor.segundos = cor.terminada.Sub(cor.iniciada).Seconds()
	}
	if err != nil {
		cor.estado = EstadoFallida
		cor.err = err
	} else {
		cor.estado = EstadoHecha
		c.ultima = cor
	}
	if c.activa == cor {
		c.activa = nil
	}
}

// ultimaSalida devuelve la carpeta de la última corrida que terminó bien.
func (c *corridas) ultimaSalida() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ultima == nil {
		return "", false
	}
	return c.ultima.dir, true
}

// vistaCorrida es la respuesta de GET /run/{id}.
type vistaCorrida struct {
	Status     string      `json:"status"`
	StartedAt  string      `json:"started_at,omitempty"`
	FinishedAt string      `json:"finished_at,omitempty"`
	Records    vistaCuenta `json:"records"`
	Cost       *vistaCoste `json:"cost,omitempty"`
	Error      string      `json:"error,omitempty"`
}

type vistaCuenta struct {
	Chats  int `json:"chats"`
	Docs   int `json:"docs"`
	Tables int `json:"tables"`
}

type vistaCoste struct {
	TokensIn  int     `json:"tokens_in"`
	TokensOut int     `json:"tokens_out"`
	Seconds   float64 `json:"seconds"`
}

func (c *corridas) vista(id string) (vistaCorrida, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cor, ok := c.porID[id]
	if !ok {
		return vistaCorrida{}, false
	}
	v := vistaCorrida{
		Status:  string(cor.estado),
		Records: vistaCuenta{Chats: cor.conteos.Chats, Docs: cor.conteos.Docs, Tables: cor.conteos.Tablas},
	}
	if !cor.iniciada.IsZero() {
		v.StartedAt = cor.iniciada.UTC().Format(time.RFC3339Nano)
	}
	if !cor.terminada.IsZero() {
		v.FinishedAt = cor.terminada.UTC().Format(time.RFC3339Nano)
	}
	if cor.estado != EstadoEnCola {
		v.Cost = &vistaCoste{Seconds: cor.segundos}
	}
	if cor.err != nil {
		v.Error = cor.err.Error()
	}
	return v, true
}

// espera bloquea hasta que no queden corridas en vuelo. El ejecutor respeta el
// context, así que un SIGTERM cancela y vuelve.
func (c *corridas) espera() { c.wg.Wait() }

func nuevoID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "job_" + hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}
