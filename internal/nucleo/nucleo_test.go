package nucleo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/almacen"
	"github.com/game013/frontier-hackaton-31102026/internal/campos"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/chats"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/tablas"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/norm"
	"github.com/game013/frontier-hackaton-31102026/internal/registro"
)

const secretoPrueba = "secreto-de-prueba"

// muestra arma la entrada de sample/input; la prueba se salta si no está
// descargado (el directorio está ignorado por git).
func muestra(t *testing.T) Entrada {
	t.Helper()
	raiz := filepath.Join("..", "..", "sample", "input")
	if _, err := os.Stat(raiz); err != nil {
		t.Skipf("sin muestra: %v", err)
	}
	ctx := context.Background()
	tablasReales, err := tablas.Leer(filepath.Join(raiz, "tables"))
	if err != nil {
		t.Fatalf("tablas: %v", err)
	}
	corpus, err := chats.Cargar(filepath.Join(raiz, "chats"))
	if err != nil {
		t.Fatalf("chats: %v", err)
	}
	docsReales, err := docs.NuevoLector().Directorio(ctx, filepath.Join(raiz, "docs"))
	if err != nil {
		t.Fatalf("docs: %v", err)
	}
	entrada := Entrada{Usuarios: corpus.Usuarios, Autores: corpus.Autores, Canales: corpus.Canales}
	for _, r := range corpus.Registros {
		entrada.Registros = append(entrada.Registros, r)
	}
	for _, r := range docsReales {
		entrada.Registros = append(entrada.Registros, r)
	}
	for _, r := range tablasReales {
		entrada.Registros = append(entrada.Registros, r)
	}
	return entrada
}

// corre corre el núcleo sobre la muestra con una bóveda temporal.
func corre(t *testing.T, entrada Entrada, vivo bool) (modelo.Resultado, *almacen.Almacen) {
	t.Helper()
	base, err := almacen.Open(filepath.Join(t.TempDir(), "almacen.db"))
	if err != nil {
		t.Fatalf("almacen: %v", err)
	}
	t.Cleanup(func() { base.Cerrar() })
	n, err := Nuevo(Opciones{Almacen: base, Secreto: []byte(secretoPrueba), Vivo: vivo, Plazo: 2 * time.Minute})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	res, err := n.Corrida(context.Background(), entrada)
	if err != nil {
		t.Fatalf("Corrida: %v", err)
	}
	return res, base
}

// La corrida de la muestra termina sin fugas conocidas: es la meta de C17/C18
// (validador OK, J-GLO 0, J-REC 100 %).
func TestMuestraCierraSinFugas(t *testing.T) {
	entrada := muestra(t)
	res, _ := corre(t, entrada, false)
	if len(res.Registros) != len(entrada.Registros) {
		t.Fatalf("registros = %d, quiero %d", len(res.Registros), len(entrada.Registros))
	}
	if len(res.Entities) == 0 {
		t.Fatal("entities.json vacío")
	}
	// Todas las formas reales del corpus tienen que haber desaparecido, con la
	// misma vara del jurado.
	for _, r := range res.Registros {
		for _, campo := range campos.Lista(r) {
			for _, forma := range formasRealesDe(entrada) {
				if norm.Contiene(campo.Valor, forma) {
					t.Errorf("%s %s conserva una forma real", r.IDRegistro(), campo.Ruta)
				}
			}
		}
	}
}

// Las personas con nombre y cédula de la muestra terminan declaradas.
func TestMuestraDeclaraPersonas(t *testing.T) {
	entrada := muestra(t)
	res, _ := corre(t, entrada, false)
	personas := map[string]bool{}
	for _, e := range res.Entities {
		if e.Type == modelo.EntidadPersona {
			personas[e.EntityID] = true
		}
	}
	for _, llave := range []string{"p_cl_88627", "p_cl_19127", "p_cl_72856", "p_cl_92356"} {
		if !personas[llave] {
			t.Errorf("falta la persona %s en entities.json", llave)
		}
	}
	if len(personas) < 5 {
		t.Errorf("personas declaradas = %d, quiero al menos 5", len(personas))
	}
}

// Dos corridas con bóvedas nuevas dan la misma salida byte a byte: el perfil
// sale del HMAC y no del orden (prueba 12 de DISEÑO §8.2).
func TestCorridaEsDeterminista(t *testing.T) {
	entrada := muestra(t)
	primera, _ := corre(t, entrada, false)
	segunda, _ := corre(t, entrada, false)
	if len(primera.Registros) != len(segunda.Registros) {
		t.Fatalf("registros %d y %d", len(primera.Registros), len(segunda.Registros))
	}
	for i := range primera.Registros {
		a, err := jsonDe(primera.Registros[i])
		if err != nil {
			t.Fatalf("json: %v", err)
		}
		b, err := jsonDe(segunda.Registros[i])
		if err != nil {
			t.Fatalf("json: %v", err)
		}
		if a != b {
			t.Fatalf("el registro %s cambió entre corridas", primera.Registros[i].IDRegistro())
		}
	}
	if len(primera.Entities) != len(segunda.Entities) {
		t.Fatalf("entidades %d y %d", len(primera.Entities), len(segunda.Entities))
	}
}

// Una bóveda con perfiles asignados no los cambia en la siguiente corrida (D4).
func TestBovedaConservaLosPerfiles(t *testing.T) {
	entrada := muestra(t)
	primera, base := corre(t, entrada, false)
	antes := map[string]modelo.Perfil{}
	for _, e := range primera.Entities {
		if e.Type == modelo.EntidadPersona && e.Pseudonyms["name"] != "" {
			antes[e.EntityID] = modelo.Perfil{Nombre: e.Pseudonyms["name"]}
		}
	}
	n, err := Nuevo(Opciones{Almacen: base, Secreto: []byte(secretoPrueba), Plazo: 2 * time.Minute})
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	segunda, err := n.Corrida(context.Background(), entrada)
	if err != nil {
		t.Fatalf("Corrida 2: %v", err)
	}
	for _, e := range segunda.Entities {
		if esperado, ok := antes[e.EntityID]; ok && e.Pseudonyms["name"] != esperado.Nombre {
			t.Errorf("%s cambió de perfil guardado", e.EntityID)
		}
	}
}

// Sin secreto no hay HMAC y el núcleo no arranca.
func TestSinSecretoFalla(t *testing.T) {
	if _, err := Nuevo(Opciones{}); err == nil {
		t.Fatal("quería error sin secreto")
	}
}

// Las personas nacidas en /anonymize van a personas_vivo y no al registro (D24).
func TestVivoNoEntraAlRegistro(t *testing.T) {
	entrada := Entrada{Registros: []modelo.Registro{
		modelo.ChatRecord{ID: "m1", ThreadID: "h", Author: "Ana Pérez Gómez", Text: "Hola"},
	}}
	res, base := corre(t, entrada, true)
	if len(res.Registros) != 1 {
		t.Fatalf("registros = %d", len(res.Registros))
	}
	if guardadas, _ := base.Personas(context.Background()); len(guardadas) != 0 {
		t.Fatalf("personas del corpus = %d, quiero 0", len(guardadas))
	}
}

// formasRealesDe junta los valores del corpus de entrada para el barrido.
func formasRealesDe(entrada Entrada) []string {
	indice, err := registro.Construye(entrada.Registros, registro.Opciones{
		Usuarios: entrada.Usuarios, Autores: entrada.Autores, Canales: entrada.Canales,
	})
	if err != nil {
		return nil
	}
	var out []string
	for _, f := range indice.Formas() {
		out = append(out, f.Valor)
	}
	return out
}

func jsonDe(r modelo.Registro) (string, error) {
	datos, err := json.Marshal(r)
	return string(datos), err
}
