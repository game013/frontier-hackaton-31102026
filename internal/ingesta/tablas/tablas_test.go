package tablas

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

// La muestra y el corpus viven fuera de git (AGENTS.md), así que las pruebas
// que dependen de ellos se saltan cuando no están descargados.
const (
	rutaMuestraTablas   = "../../../sample/input/tables"
	rutaMuestraEsperada = "../../../sample/expected_output/tables.jsonl"
	rutaFuentesTablas   = "../../../fuentes/tables"
	rutaIndiceFuentes   = "../../../fuentes/record_index.json"
)

func exigirRuta(t *testing.T, ruta string) {
	t.Helper()
	if _, err := os.Stat(ruta); err != nil {
		t.Skipf("%s no está descargado: %v", ruta, err)
	}
}

// TestLeerMuestra reproduce los ids y las claves de columns de la salida ideal.
func TestLeerMuestra(t *testing.T) {
	exigirRuta(t, rutaMuestraTablas)
	exigirRuta(t, rutaMuestraEsperada)

	registros, err := Leer(rutaMuestraTablas)
	if err != nil {
		t.Fatalf("Leer(%q) devolvió error: %v", rutaMuestraTablas, err)
	}
	esperados := leerEsperados(t, rutaMuestraEsperada)

	if len(registros) != len(esperados) {
		t.Fatalf("llegaron %d registros y la salida ideal trae %d", len(registros), len(esperados))
	}
	for i, esperado := range esperados {
		llegó := registros[i]
		if llegó.ID != esperado.ID {
			t.Errorf("registro %d: id %q, se esperaba %q", i, llegó.ID, esperado.ID)
		}
		if llegó.Table != esperado.Table {
			t.Errorf("%s: table %q, se esperaba %q", esperado.ID, llegó.Table, esperado.Table)
		}
		if obtenidas, queridas := claves(llegó.Columns), claves(esperado.Columns); !slices.Equal(obtenidas, queridas) {
			t.Errorf("%s: columns %v, se esperaba %v", esperado.ID, obtenidas, queridas)
		}
	}
}

func leerEsperados(t *testing.T, ruta string) []modelo.TableRecord {
	t.Helper()
	archivo, err := os.Open(ruta)
	if err != nil {
		t.Fatalf("abrir %s: %v", ruta, err)
	}
	t.Cleanup(func() { archivo.Close() })

	var registros []modelo.TableRecord
	lineas := bufio.NewScanner(archivo)
	lineas.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for lineas.Scan() {
		if len(lineas.Bytes()) == 0 {
			continue
		}
		var registro modelo.TableRecord
		if err := json.Unmarshal(lineas.Bytes(), &registro); err != nil {
			t.Fatalf("%s: %v", ruta, err)
		}
		registros = append(registros, registro)
	}
	if err := lineas.Err(); err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	return registros
}

// claves devuelve los nombres de columna ordenados, para comparar sin depender
// del orden del mapa.
func claves(columnas map[string]string) []string {
	nombres := make([]string, 0, len(columnas))
	for nombre := range columnas {
		nombres = append(nombres, nombre)
	}
	slices.Sort(nombres)
	return nombres
}

// TestLeerTransacciones lee un CSV UTF-8 con BOM, comas dentro de comillas y
// CRLF sin dejar el retorno de carro pegado al último campo.
func TestLeerTransacciones(t *testing.T) {
	registros, err := Leer("testdata/transacciones")
	if err != nil {
		t.Fatalf("Leer devolvió error: %v", err)
	}
	esperados := []modelo.TableRecord{
		{
			ID:    "transacciones:TX-900001",
			Table: "transacciones",
			Columns: map[string]string{
				"tx_id":       "TX-900001",
				"fecha":       "2026-07-01",
				"cliente_id":  "CL-90001",
				"tipo":        "pago_credito",
				"monto":       "520000",
				"descripcion": "Cuota crédito, sede Norte",
				"canal":       "pse",
				"estado":      "aprobada",
			},
		},
		{
			ID:    "transacciones:TX-900002",
			Table: "transacciones",
			Columns: map[string]string{
				"tx_id":       "TX-900002",
				"fecha":       "2026-07-02",
				"cliente_id":  "CL-90002",
				"tipo":        "pago_servicio",
				"monto":       "1370000",
				"descripcion": "Retiro cajero Bancolombia Chapinero",
				"canal":       "debito_automatico",
				"estado":      "aprobada",
			},
		},
	}
	compararRegistros(t, registros, esperados)
}

// compararRegistros exige los mismos registros, en el mismo orden y con los
// mismos valores: id, table y columns.
func compararRegistros(t *testing.T, obtenidos, esperados []modelo.TableRecord) {
	t.Helper()
	if len(obtenidos) != len(esperados) {
		t.Fatalf("llegaron %d registros y se esperaban %d", len(obtenidos), len(esperados))
	}
	for i, esperado := range esperados {
		obtenido := obtenidos[i]
		if obtenido.ID != esperado.ID {
			t.Errorf("registro %d: id %q, se esperaba %q", i, obtenido.ID, esperado.ID)
		}
		if obtenido.Table != esperado.Table {
			t.Errorf("%s: table %q, se esperaba %q", esperado.ID, obtenido.Table, esperado.Table)
		}
		if obtenidas, queridas := claves(obtenido.Columns), claves(esperado.Columns); !slices.Equal(obtenidas, queridas) {
			t.Errorf("%s: columns %v, se esperaba %v", esperado.ID, obtenidas, queridas)
			continue
		}
		for columna, valor := range esperado.Columns {
			if obtenido.Columns[columna] != valor {
				t.Errorf("%s: %s = %q, se esperaba %q", esperado.ID, columna, obtenido.Columns[columna], valor)
			}
		}
	}
}

// TestLeerClientesLatin1 comprueba lo que el legado trae raro: Latin-1 con
// tildes y ñ, campo entre comillas con `;` y salto de línea, CRLF y una fila
// corta que igual debe salir con todas las columnas.
func TestLeerClientesLatin1(t *testing.T) {
	registros, err := Leer("testdata/clientes")
	if err != nil {
		t.Fatalf("Leer devolvió error: %v", err)
	}
	esperados := []modelo.TableRecord{
		{
			ID:    "clientes:CL-90001",
			Table: "clientes",
			Columns: map[string]string{
				"cliente_id":        "CL-90001",
				"nombre_completo":   "Muñoz Peña",
				"tipo_doc":          "CC",
				"num_doc":           "12345678",
				"correo":            "munoz@example.com",
				"celular":           "3001234567",
				"ciudad":            "Bogotá",
				"barrio":            "Chapinero",
				"direccion":         "Cl 1 # 2-3",
				"fecha_nacimiento":  "1990-01-02",
				"fecha_vinculacion": "2024-03-04",
				"ocupacion":         "abogada",
				"notas":             "Nota con ; punto y coma;\nsegunda línea",
			},
		},
		{
			ID:    "clientes:CL-90002",
			Table: "clientes",
			Columns: map[string]string{
				"cliente_id":        "CL-90002",
				"nombre_completo":   "Ana Belén Ñáñez",
				"tipo_doc":          "CC",
				"num_doc":           "87654321",
				"correo":            "nanez@example.com",
				"celular":           "3109876543",
				"ciudad":            "Cali",
				"barrio":            "Granada",
				"direccion":         "Cra 4 # 5-6",
				"fecha_nacimiento":  "1985-06-07",
				"fecha_vinculacion": "2023-08-09",
				"ocupacion":         "contadora",
				"notas":             "",
			},
		},
		{
			ID:    "clientes:CL-90003",
			Table: "clientes",
			Columns: map[string]string{
				"cliente_id":        "CL-90003",
				"nombre_completo":   "Julián Ospina Rojas",
				"tipo_doc":          "CC",
				"num_doc":           "10203040",
				"correo":            "jospina@example.com",
				"celular":           "3201112233",
				"ciudad":            "Medellín",
				"barrio":            "Robledo",
				"direccion":         "Tv 7 # 8-9",
				"fecha_nacimiento":  "",
				"fecha_vinculacion": "",
				"ocupacion":         "",
				"notas":             "",
			},
		},
	}
	compararRegistros(t, registros, esperados)
}

// TestLeerPQRS arma cada registro con los atributos del <pqr> y los hijos
// <descripcion> (CDATA con `<` y `&`) y <respuesta>, sin el comentario XML que
// parece un registro y sin los atributos de la raíz.
func TestLeerPQRS(t *testing.T) {
	registros, err := Leer("testdata/pqrs")
	if err != nil {
		t.Fatalf("Leer devolvió error: %v", err)
	}
	esperados := []modelo.TableRecord{
		{
			ID:    "pqrs:PQR-2026-9001",
			Table: "pqrs",
			Columns: map[string]string{
				"radicado":    "PQR-2026-9001",
				"cliente_id":  "CL-90001",
				"cedula":      "12345678",
				"ciudad":      "Bogotá",
				"fecha":       "2026-09-19",
				"agente":      "julian.ospina",
				"tipo":        "reclamo",
				"estado":      "cerrado",
				"descripcion": `Me cobraron 356.000 < 400.000 y la etiqueta decía "A & B".`,
				"respuesta":   "Se revirtió el cobro & se corrigió la asociación.",
			},
		},
		{
			ID:    "pqrs:PQR-2026-9002",
			Table: "pqrs",
			Columns: map[string]string{
				"radicado":    "PQR-2026-9002",
				"cliente_id":  "CL-90002",
				"cedula":      "87654321",
				"ciudad":      "Cali",
				"fecha":       "2026-09-28",
				"agente":      "",
				"tipo":        "queja",
				"estado":      "abierto",
				"descripcion": "Sin respuesta todavía.",
				"respuesta":   "",
			},
		},
	}
	compararRegistros(t, registros, esperados)
}

// TestLeerEmpleados arma cada registro con los atributos del <empleado> y sus
// hijos, incluido el atributo tipo de <documento> y el reporta_a vacío del CEO.
func TestLeerEmpleados(t *testing.T) {
	registros, err := Leer("testdata/empleados")
	if err != nil {
		t.Fatalf("Leer devolvió error: %v", err)
	}
	esperados := []modelo.TableRecord{
		{
			ID:    "empleados:KP-901",
			Table: "empleados",
			Columns: map[string]string{
				"id":               "KP-901",
				"cargo":            "CEO",
				"ciudad":           "Medellín",
				"ingreso":          "2019",
				"reporta_a":        "",
				"nombre":           "Nicolás Arriaga Pardo",
				"correo":           "nicolas.arriaga@kipupay.co",
				"celular":          "3001110001",
				"documento":        "19000001",
				"documento_tipo":   "CC",
				"fecha_nacimiento": "1975-06-17",
			},
		},
		{
			ID:    "empleados:KP-902",
			Table: "empleados",
			Columns: map[string]string{
				"id":               "KP-902",
				"cargo":            "CTO",
				"ciudad":           "Medellín",
				"ingreso":          "2019",
				"reporta_a":        "Nicolás Arriaga",
				"nombre":           "Farid Ospina Gómez",
				"correo":           "farid.ospina@kipupay.co",
				"celular":          "3001110002",
				"documento":        "1099000002",
				"documento_tipo":   "CE",
				"fecha_nacimiento": "1999-06-07",
			},
		},
	}
	compararRegistros(t, registros, esperados)
}

// TestLeerSinTablas: un directorio sin ninguna de las cuatro tablas es error, no
// un corpus vacío.
func TestLeerSinTablas(t *testing.T) {
	if _, err := Leer(t.TempDir()); err == nil {
		t.Fatal("Leer de un directorio vacío devolvió nil; se esperaba error")
	}
}

// TestLeerHumoFuentes solo compara conteos contra fuentes/record_index.json: el
// corpus es real, así que la prueba no mira valores.
func TestLeerHumoFuentes(t *testing.T) {
	exigirRuta(t, rutaFuentesTablas)
	exigirRuta(t, rutaIndiceFuentes)

	registros, err := Leer(rutaFuentesTablas)
	if err != nil {
		t.Fatalf("Leer(%q) devolvió error: %v", rutaFuentesTablas, err)
	}
	conteos := map[string]int{}
	for _, registro := range registros {
		llave, hayPrefijo := strings.CutPrefix(registro.ID, registro.Table+":")
		if !hayPrefijo || llave == "" {
			t.Fatalf("%s: el id no es <tabla>:<llave>", registro.ID)
		}
		if len(registro.Columns) == 0 {
			t.Fatalf("%s: sin columnas", registro.ID)
		}
		conteos[registro.Table]++
	}

	for tabla, esperados := range conteosDelIndice(t, rutaIndiceFuentes) {
		if conteos[tabla] != esperados {
			t.Errorf("%s: llegaron %d registros y el índice trae %d", tabla, conteos[tabla], esperados)
		}
		delete(conteos, tabla)
	}
	for tabla, llegaron := range conteos {
		t.Errorf("%s: %d registros que no están en el índice", tabla, llegaron)
	}
}

// conteosDelIndice cuenta los ids de record_index.json por tabla.
func conteosDelIndice(t *testing.T, ruta string) map[string]int {
	t.Helper()
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	var indice map[string][]string
	if err := json.Unmarshal(datos, &indice); err != nil {
		t.Fatalf("%s: %v", ruta, err)
	}
	conteos := map[string]int{}
	for _, id := range indice["tables"] {
		tabla, _, hayLlave := strings.Cut(id, ":")
		if !hayLlave {
			t.Fatalf("id sin tabla en el índice: %q", id)
		}
		conteos[tabla]++
	}
	return conteos
}

// TestLeerFilasDescuadradas fija el trato de las filas que no cuadran con la
// cabecera: un campo de más con contenido es error (no se pierde en silencio),
// los campos extra vacíos se ignoran y una cabecera sin llave es error.
func TestLeerFilasDescuadradas(t *testing.T) {
	casos := []struct {
		nombre      string
		contenido   string
		quiereError bool
	}{
		{
			nombre:      "campo de más con contenido",
			contenido:   "cliente_id;nombre_completo\r\nCL-90001;Ana Belén;sobra\r\n",
			quiereError: true,
		},
		{
			nombre:    "campo de más vacío",
			contenido: "cliente_id;nombre_completo\r\nCL-90001;Ana Belén;\r\n",
		},
		{
			nombre:      "cabecera sin la llave",
			contenido:   "id;nombre_completo\r\nCL-90001;Ana Belén\r\n",
			quiereError: true,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			directorio := t.TempDir()
			if err := os.WriteFile(filepath.Join(directorio, "clientes.csv"), []byte(caso.contenido), 0o600); err != nil {
				t.Fatalf("escribir la tabla de prueba: %v", err)
			}
			_, err := Leer(directorio)
			if caso.quiereError && err == nil {
				t.Fatal("Leer devolvió nil; se esperaba error")
			}
			if !caso.quiereError && err != nil {
				t.Fatalf("Leer devolvió error: %v", err)
			}
		})
	}
}

// TestLeerOrdenDeTablas: con las cuatro tablas presentes, los registros salen en
// el orden del índice de fuentes (clientes, transacciones, pqrs, empleados).
func TestLeerOrdenDeTablas(t *testing.T) {
	directorio := t.TempDir()
	archivos := map[string]string{
		"clientes.csv":      "testdata/clientes/clientes.csv",
		"transacciones.csv": "testdata/transacciones/transacciones.csv",
		"pqrs.xml":          "testdata/pqrs/pqrs.xml",
		"empleados.xml":     "testdata/empleados/empleados.xml",
	}
	for destino, origen := range archivos {
		datos, err := os.ReadFile(origen)
		if err != nil {
			t.Fatalf("leer %s: %v", origen, err)
		}
		if err := os.WriteFile(filepath.Join(directorio, destino), datos, 0o600); err != nil {
			t.Fatalf("escribir %s: %v", destino, err)
		}
	}

	registros, err := Leer(directorio)
	if err != nil {
		t.Fatalf("Leer devolvió error: %v", err)
	}
	var tablas []string
	for _, registro := range registros {
		if len(tablas) == 0 || tablas[len(tablas)-1] != registro.Table {
			tablas = append(tablas, registro.Table)
		}
	}
	esperado := []string{"clientes", "transacciones", "pqrs", "empleados"}
	if !slices.Equal(tablas, esperado) {
		t.Fatalf("las tablas salieron en el orden %v, se esperaba %v", tablas, esperado)
	}
	if len(registros) != 3+2+2+2 {
		t.Errorf("llegaron %d registros y se esperaban 9", len(registros))
	}
}
