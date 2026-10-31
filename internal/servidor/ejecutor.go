package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/fuentes"
	"github.com/game013/frontier-hackaton-31102026/internal/ingesta/docs"
	"github.com/game013/frontier-hackaton-31102026/internal/llm"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
	"github.com/game013/frontier-hackaton-31102026/internal/nucleo"
	"github.com/game013/frontier-hackaton-31102026/internal/salida"
)

const (
	// plazoPeticion acota cada petición al API de fuentes. Es por petición, no
	// por corrida: el plazo de la corrida lo pone el job (plazoCorrida).
	plazoPeticion = 60 * time.Second
	// carpetaFuentes es donde queda el corpus materializado dentro del
	// directorio de la corrida: los lectores de ingesta leen de disco.
	carpetaFuentes = "fuentes"
	// permisosPrivados es el modo de todo lo que baja del API: es material real
	// y vive en el volumen, nunca en el repo.
	permisosPrivados = 0o600
)

// EjecutorFuentes es el ejecutor real de /run: descarga el API de fuentes,
// materializa chats/, docs/ y tables/ bajo dirSalida, ingesta las tres fuentes,
// pasa el lote por el núcleo y escribe output/ en dirSalida.
//
// El núcleo que usa es el de corrida (Vivo:false, D24): las personas que
// aparecen en /run van al registro, no a personas_vivo. La bóveda es la misma
// que la de /anonymize, para que una persona reciba el mismo perfil en los dos
// caminos (D8).
type EjecutorFuentes struct {
	corredor *nucleo.Nucleo
	lector   *docs.Lector
	uso      func() llm.Uso
}

// NuevoEjecutor arma el ejecutor de /run. Un lector nil toma el del entorno
// (PDF_PYTHON y tools/pdf/extraer.py), que es lo que hace el servicio.
//
// uso es la contabilidad del modelo (lo que el cliente de Jev devuelve en Uso);
// nil deja la corrida reportando cero tokens, que es lo correcto sin modelo. Se
// lee dos veces, al empezar y al terminar, y lo que la corrida reporta es la
// diferencia: el cliente acumula y el mismo sirve a /anonymize.
func NuevoEjecutor(corredor *nucleo.Nucleo, lector *docs.Lector, uso func() llm.Uso) (*EjecutorFuentes, error) {
	if corredor == nil {
		return nil, errors.New("ejecutor: falta el núcleo de /run")
	}
	if lector == nil {
		lector = docs.NuevoLector()
	}
	return &EjecutorFuentes{corredor: corredor, lector: lector, uso: uso}, nil
}

// Ejecuta cumple Ejecutor. Devuelve los conteos de la salida y, si algo falla
// después de escribirla, los conteos junto con el error: /run/{id} los reporta
// igual.
func (e *EjecutorFuentes) Ejecuta(ctx context.Context, sourcesURL, token, dirSalida string) (Conteos, error) {
	if strings.TrimSpace(dirSalida) == "" {
		return Conteos{}, errors.New("ejecutor: falta el directorio de la corrida")
	}
	// La contabilidad se fotografía antes de la primera llamada: lo que la
	// corrida costó es la diferencia con el acumulado del cliente al terminar.
	var usoAntes llm.Uso
	if e.uso != nil {
		usoAntes = e.uso()
	}
	cliente, err := fuentes.Nuevo(fuentes.Opciones{
		Base: sourcesURL, Token: token, Timeout: plazoPeticion,
	})
	if err != nil {
		return Conteos{}, fmt.Errorf("ejecutor: %w", err)
	}

	raiz := filepath.Join(dirSalida, carpetaFuentes)
	if err := os.MkdirAll(raiz, 0o700); err != nil {
		return Conteos{}, fmt.Errorf("ejecutor: prepara %s: %w", raiz, err)
	}
	indice, err := e.descarga(ctx, cliente, raiz)
	if err != nil {
		return Conteos{}, err
	}

	entrada, err := nucleo.CargaCorpus(ctx, raiz, e.lector)
	if err != nil {
		return Conteos{}, fmt.Errorf("ejecutor: ingesta: %w", err)
	}
	resultado, err := e.corredor.Corrida(ctx, entrada)
	if err != nil {
		return Conteos{}, err
	}
	if err := salida.Escribe(dirSalida, resultado.Registros, resultado.Entities); err != nil {
		return Conteos{}, fmt.Errorf("ejecutor: %w", err)
	}

	conteos := cuentaPorFuente(resultado.Registros)
	// La contabilidad se lee después de que el núcleo terminó y se le resta la
	// de antes: es el costo de esta corrida y va en /run/{id} (D13).
	if e.uso != nil {
		conteos.Modelo = consumoDeLaCorrida(usoAntes, e.uso())
	}
	// El índice va primero y se compara al final: declara todos los ids que la
	// salida debe contener, así que un id declarado y ausente es una fuente
	// ingerida a medias y la corrida falla a la vista (D11).
	return conteos, verificaCobertura(indice, resultado.Registros)
}

// indiceDeRegistros es /record_index: los ids que la salida debe contener, por
// fuente.
type indiceDeRegistros struct {
	Chats  []string `json:"chats"`
	Docs   []string `json:"docs"`
	Tablas []string `json:"tables"`
}

// descarga materializa el árbol completo del API bajo raiz y devuelve el índice
// de registros para comprobar la cobertura al final.
func (e *EjecutorFuentes) descarga(ctx context.Context, cliente *fuentes.Cliente, raiz string) (indiceDeRegistros, error) {
	var indice indiceDeRegistros
	crudo, err := e.trae(ctx, cliente, "/record_index", filepath.Join(raiz, "record_index.json"))
	if err != nil {
		return indice, err
	}
	if err := json.Unmarshal(crudo, &indice); err != nil {
		return indice, fmt.Errorf("ejecutor: record_index ilegible: %w", err)
	}
	if err := e.descargaChats(ctx, cliente, raiz); err != nil {
		return indice, err
	}
	if err := e.descargaDocs(ctx, cliente, raiz); err != nil {
		return indice, err
	}
	if err := e.descargaTablas(ctx, cliente, raiz); err != nil {
		return indice, err
	}
	return indice, nil
}

// descargaChats baja la metadata del canal, su lista de fechas y el archivo de
// cada fecha: la ingesta espera un directorio por canal con un <fecha>.json.
func (e *EjecutorFuentes) descargaChats(ctx context.Context, cliente *fuentes.Cliente, raiz string) error {
	dir := filepath.Join(raiz, "chats")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("ejecutor: prepara %s: %w", dir, err)
	}
	if _, err := e.trae(ctx, cliente, "/chats/users", filepath.Join(dir, "users.json")); err != nil {
		return err
	}
	crudo, err := e.trae(ctx, cliente, "/chats/channels", filepath.Join(dir, "channels.json"))
	if err != nil {
		return err
	}
	var canales []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(crudo, &canales); err != nil {
		return fmt.Errorf("ejecutor: channels.json ilegible: %w", err)
	}
	for _, canal := range canales {
		nombre, err := nombreSeguro(canal.Name)
		if err != nil {
			return err
		}
		fechas, err := e.pide(ctx, cliente, "/chats/"+url.PathEscape(nombre))
		if err != nil {
			return err
		}
		var dias []string
		if err := json.Unmarshal(fechas, &dias); err != nil {
			return fmt.Errorf("ejecutor: las fechas del canal no son una lista: %w", err)
		}
		for _, dia := range dias {
			fecha, err := nombreSeguro(dia)
			if err != nil {
				return err
			}
			ruta := "/chats/" + url.PathEscape(nombre) + "/" + url.PathEscape(fecha)
			destino := filepath.Join(dir, nombre, fecha+".json")
			if _, err := e.trae(ctx, cliente, ruta, destino); err != nil {
				return err
			}
		}
	}
	return nil
}

// descargaDocs baja el índice y el original de cada documento con el nombre de
// archivo que declara el índice: la ingesta arma el registro desde ese archivo.
func (e *EjecutorFuentes) descargaDocs(ctx context.Context, cliente *fuentes.Cliente, raiz string) error {
	dir := filepath.Join(raiz, "docs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("ejecutor: prepara %s: %w", dir, err)
	}
	crudo, err := e.trae(ctx, cliente, "/docs", filepath.Join(dir, "index.json"))
	if err != nil {
		return err
	}
	var entradas []struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(crudo, &entradas); err != nil {
		return fmt.Errorf("ejecutor: index.json de docs ilegible: %w", err)
	}
	for _, entrada := range entradas {
		nombre, err := nombreSeguro(entrada.Filename)
		if err != nil {
			return err
		}
		ruta := "/docs/" + url.PathEscape(entrada.ID)
		if _, err := e.trae(ctx, cliente, ruta, filepath.Join(dir, nombre)); err != nil {
			return err
		}
	}
	return nil
}

// descargaTablas baja el índice y los bytes originales de cada tabla, con el
// nombre que declara el índice.
func (e *EjecutorFuentes) descargaTablas(ctx context.Context, cliente *fuentes.Cliente, raiz string) error {
	dir := filepath.Join(raiz, "tables")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("ejecutor: prepara %s: %w", dir, err)
	}
	crudo, err := e.trae(ctx, cliente, "/tables", filepath.Join(dir, "index.json"))
	if err != nil {
		return err
	}
	var tablas []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(crudo, &tablas); err != nil {
		return fmt.Errorf("ejecutor: index.json de tables ilegible: %w", err)
	}
	for _, tabla := range tablas {
		nombre, err := nombreSeguro(tabla.Name)
		if err != nil {
			return err
		}
		ruta := "/tables/" + url.PathEscape(nombre)
		if _, err := e.trae(ctx, cliente, ruta, filepath.Join(dir, nombre)); err != nil {
			return err
		}
	}
	return nil
}

// pide descarga una ruta del API y devuelve los bytes tal como llegaron.
func (e *EjecutorFuentes) pide(ctx context.Context, cliente *fuentes.Cliente, ruta string) ([]byte, error) {
	respuesta, err := cliente.Obtener(ctx, ruta)
	if err != nil {
		return nil, fmt.Errorf("ejecutor: %w", err)
	}
	if respuesta.SinCambios {
		// El cliente de una corrida nace sin ETag: un 304 aquí es un servidor
		// que no está sirviendo lo que declara.
		return nil, fmt.Errorf("ejecutor: %s respondió 304 en la primera visita", ruta)
	}
	return respuesta.Cuerpo, nil
}

// trae descarga una ruta y la deja en destino. Los bytes se escriben sin tocar,
// porque el charset declarado lo interpreta la ingesta y recodificar rompería
// la salida byte a byte.
func (e *EjecutorFuentes) trae(ctx context.Context, cliente *fuentes.Cliente, ruta, destino string) ([]byte, error) {
	crudo, err := e.pide(ctx, cliente, ruta)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o700); err != nil {
		return nil, fmt.Errorf("ejecutor: prepara %s: %w", filepath.Dir(destino), err)
	}
	if err := os.WriteFile(destino, crudo, permisosPrivados); err != nil {
		return nil, fmt.Errorf("ejecutor: escribe %s: %w", destino, err)
	}
	return crudo, nil
}

// nombreSeguro rechaza un nombre que la fuente pudiera usar para salir de la
// carpeta de la corrida. El mensaje no repite el valor: los nombres del API
// pueden traer datos personales.
func nombreSeguro(valor string) (string, error) {
	if valor == "" || valor == "." || valor == ".." || strings.ContainsAny(valor, `/\`) {
		return "", errors.New("ejecutor: la fuente trae un nombre de archivo fuera de la carpeta de la corrida")
	}
	return valor, nil
}

// consumoDeLaCorrida resta a la contabilidad del final la del principio del
// proceso. El cliente de Jev es uno solo para /run y /anonymize, así que su
// acumulado no es el de la corrida: publicarlo tal cual le sumaría a cada job
// el costo de los anteriores.
func consumoDeLaCorrida(antes, despues llm.Uso) llm.Uso {
	return llm.Uso{
		Llamadas:      despues.Llamadas - antes.Llamadas,
		Aciertos:      despues.Aciertos - antes.Aciertos,
		Fallos:        despues.Fallos - antes.Fallos,
		TokensEntrada: despues.TokensEntrada - antes.TokensEntrada,
		TokensSalida:  despues.TokensSalida - antes.TokensSalida,
		CostoUSD:      despues.CostoUSD - antes.CostoUSD,
	}
}

// cuentaPorFuente cuenta los registros de la salida por fuente: es lo que
// /run/{id} reporta en records.
func cuentaPorFuente(registros []modelo.Registro) Conteos {
	var conteos Conteos
	for _, r := range registros {
		switch r.FuenteRegistro() {
		case modelo.FuenteChats:
			conteos.Chats++
		case modelo.FuenteDocs:
			conteos.Docs++
		case modelo.FuenteTablas:
			conteos.Tablas++
		}
	}
	return conteos
}

// verificaCobertura compara la salida con record_index. Un id declarado que no
// está en la salida es una fuente ingerida a medias y falla la corrida; un id de
// más no falla, porque el corpus crece entre que se pide el índice y que se
// piden los mensajes.
func verificaCobertura(indice indiceDeRegistros, registros []modelo.Registro) error {
	presentes := map[modelo.Fuente]map[string]bool{
		modelo.FuenteChats:  {},
		modelo.FuenteDocs:   {},
		modelo.FuenteTablas: {},
	}
	for _, r := range registros {
		if ids, ok := presentes[r.FuenteRegistro()]; ok {
			ids[r.IDRegistro()] = true
		}
	}
	var faltantes []string
	for _, grupo := range []struct {
		fuente modelo.Fuente
		ids    []string
	}{
		{modelo.FuenteChats, indice.Chats},
		{modelo.FuenteDocs, indice.Docs},
		{modelo.FuenteTablas, indice.Tablas},
	} {
		faltan := 0
		ejemplos := []string{}
		for _, id := range grupo.ids {
			if presentes[grupo.fuente][id] {
				continue
			}
			faltan++
			if len(ejemplos) < 3 {
				ejemplos = append(ejemplos, id)
			}
		}
		if faltan > 0 {
			faltantes = append(faltantes, fmt.Sprintf("%s: faltan %d de %d (%s)",
				grupo.fuente, faltan, len(grupo.ids), strings.Join(ejemplos, ", ")))
		}
	}
	if len(faltantes) > 0 {
		return fmt.Errorf("ejecutor: la salida no cubre record_index — %s", strings.Join(faltantes, "; "))
	}
	return nil
}
