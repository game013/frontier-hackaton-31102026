package residuo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/detecta"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// falso es un modelo de decisiones para pruebas: responde lo que se le diga y
// guarda los estados que recibió.
type falso struct {
	respuesta modelo.Respuesta
	err       error
	estados   []string
	demora    time.Duration
}

func (f *falso) Nombre() string { return "falso" }

func (f *falso) Decide(ctx context.Context, estado string, _ modelo.Pregunta) (modelo.Respuesta, error) {
	f.estados = append(f.estados, estado)
	if f.demora > 0 {
		select {
		case <-time.After(f.demora):
		case <-ctx.Done():
			return modelo.Respuesta{}, ctx.Err()
		}
	}
	return f.respuesta, f.err
}

func candidato(valor, generador string) detecta.Candidato {
	return detecta.Candidato{
		Registro: "m1", Fuente: modelo.FuenteChats, Campo: "text",
		Inicio: 0, Fin: len(valor), Valor: valor, Generador: generador,
		Contexto: "la señora " + valor + " me contó",
	}
}

// Una persona con probabilidad alta se convierte en mención; la resolución
// decide el dueño, no el modelo (D26).
func TestClasificaPersonaConUmbral(t *testing.T) {
	modeloFalso := &falso{respuesta: modelo.Respuesta{
		Pregunta: "origen", Opcion: "persona",
		Probabilidades: map[string]float64{"persona": 0.93, "lugar": 0.05},
	}}
	c := Nuevo(Opciones{Modelo: modeloFalso})
	menciones := c.Clasifica(context.Background(), []detecta.Candidato{candidato("Gloria", "disparador")})
	if len(menciones) != 1 {
		t.Fatalf("menciones = %d, quiero 1", len(menciones))
	}
	m := menciones[0]
	if m.Tipo != modelo.MencionPersona || m.Persona != "" || m.Regla != "residuo:disparador" {
		t.Fatalf("mención = %+v", m)
	}
	if m.Confianza < 0.9 {
		t.Errorf("confianza = %v", m.Confianza)
	}
}

// Por debajo de 0,5 la respuesta no es confiable y el texto se conserva (D31).
func TestUmbralBajoConservaElTexto(t *testing.T) {
	modeloFalso := &falso{respuesta: modelo.Respuesta{
		Pregunta: "origen", Opcion: "persona",
		Probabilidades: map[string]float64{"persona": 0.38, "otro": 0.4},
	}}
	c := Nuevo(Opciones{Modelo: modeloFalso})
	if menciones := c.Clasifica(context.Background(), []detecta.Candidato{candidato("Gloria", "disparador")}); len(menciones) != 0 {
		t.Fatalf("clasifiqué con confianza baja: %+v", menciones)
	}
}

// Una organización no se toca: utilidad 9.
func TestOrganizacionSeConserva(t *testing.T) {
	modeloFalso := &falso{respuesta: modelo.Respuesta{
		Pregunta: "origen", Opcion: "organizacion",
		Probabilidades: map[string]float64{"persona": 0.1, "organizacion": 0.85},
	}}
	c := Nuevo(Opciones{Modelo: modeloFalso})
	if menciones := c.Clasifica(context.Background(), []detecta.Candidato{candidato("Banco Davivienda", "mayusculas")}); len(menciones) != 0 {
		t.Fatalf("toqué una organización: %+v", menciones)
	}
}

// Si el modelo falla o se pasa del tiempo, cae a las reglas: el disparador sigue
// siendo persona, la mayúscula sola no.
func TestTimeoutCaeAReglas(t *testing.T) {
	lento := &falso{demora: 50 * time.Millisecond}
	c := Nuevo(Opciones{Modelo: lento, Timeout: 5 * time.Millisecond})
	cands := []detecta.Candidato{
		candidato("Gloria", "disparador"),
		candidato("Doris Lozano", "mayusculas"),
	}
	menciones := c.Clasifica(context.Background(), cands)
	if len(menciones) != 1 || menciones[0].Valor != "Gloria" {
		t.Fatalf("respaldo con timeout = %+v", menciones)
	}
}

// El estado que viaja al proveedor lleva las formas conocidas enmascaradas y
// solo una ventana (D15).
func TestEstadoEnmascaradoYAcotado(t *testing.T) {
	modeloFalso := &falso{respuesta: modelo.Respuesta{Opcion: "otro"}}
	c := Nuevo(Opciones{
		Modelo: modeloFalso,
		Enmascara: func(s string) string {
			return strings.ReplaceAll(s, "Gloria", "[PERSONA_1]")
		},
	})
	c.Clasifica(context.Background(), []detecta.Candidato{candidato("Gloria", "disparador")})
	if len(modeloFalso.estados) != 1 {
		t.Fatalf("llamadas = %d", len(modeloFalso.estados))
	}
	estado := modeloFalso.estados[0]
	if !strings.Contains(estado, "[PERSONA_1]") {
		t.Errorf("el estado no enmascara: %q", estado)
	}
	if strings.Contains(estado, "la señora Gloria me contó") {
		t.Errorf("el estado lleva la forma real sin enmascarar: %q", estado)
	}
}

// Un error del modelo no puede tumbar el lote: es el respaldo de /anonymize.
func TestErrorDelModeloNoRompeElLote(t *testing.T) {
	c := Nuevo(Opciones{Modelo: &falso{err: errors.New("boom")}})
	menciones := c.Clasifica(context.Background(), []detecta.Candidato{candidato("Gloria", "disparador")})
	if len(menciones) != 1 {
		t.Fatalf("respaldo = %+v", menciones)
	}
}
