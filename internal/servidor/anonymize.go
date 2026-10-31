package servidor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/game013/frontier-hackaton-31102026/internal/captura"
	"github.com/game013/frontier-hackaton-31102026/internal/modelo"
)

const (
	// limiteRegistros y plazoAnonimiza son el contrato: 50 registros en 60 s.
	limiteRegistros  = 50
	plazoAnonimiza   = 50 * time.Second
	limiteCuerpoAnon = 32 << 20
)

type peticionAnonimiza struct {
	Source  string             `json:"source"`
	Records *[]json.RawMessage `json:"records"`
}

// anonimiza es la prueba en vivo del jurado (D8): mismo núcleo que /run, mismo
// esquema de registro a la entrada y a la salida.
func (s *Servidor) anonimiza(w http.ResponseWriter, r *http.Request) {
	metrica := captura.Anota(r)
	var peticion peticionAnonimiza
	decodificador := json.NewDecoder(http.MaxBytesReader(w, r.Body, limiteCuerpoAnon))
	if err := decodificador.Decode(&peticion); err != nil {
		metrica.Error = "cuerpo"
		respondeError(w, http.StatusUnprocessableEntity, "cuerpo inválido: se espera {source, records}")
		return
	}
	fuente, err := fuenteDe(peticion.Source)
	if err != nil {
		metrica.Error = "fuente"
		respondeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	metrica.Fuente = string(fuente)
	if peticion.Records == nil {
		metrica.Error = "records"
		respondeError(w, http.StatusUnprocessableEntity, "falta records")
		return
	}
	if len(*peticion.Records) > limiteRegistros {
		metrica.Error = "records"
		respondeError(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("el lote trae %d registros y el máximo es %d", len(*peticion.Records), limiteRegistros))
		return
	}

	registros, err := decodificaRegistros(fuente, *peticion.Records)
	if err != nil {
		metrica.Error = "esquema"
		respondeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	metrica.Registros = len(registros)

	ctx, cancel := context.WithTimeout(r.Context(), plazoAnonimiza)
	defer cancel()
	resultado, err := s.nucleo.Anonymize(ctx, fuente, registros)
	if err != nil {
		// El error del núcleo puede citar texto del registro: al cliente va un
		// código, nunca el mensaje.
		metrica.Error = "nucleo"
		respondeError(w, http.StatusInternalServerError, "no se pudo anonimizar el lote")
		return
	}
	entidades := resultado.Entities
	if entidades == nil {
		entidades = []modelo.Entity{}
	}
	salida := resultado.Registros
	if salida == nil {
		salida = []modelo.Registro{}
	}
	metrica.Entidades = len(entidades)
	respondeJSON(w, http.StatusOK, struct {
		Records  []modelo.Registro `json:"records"`
		Entities []modelo.Entity   `json:"entities"`
	}{Records: salida, Entities: entidades})
}

func fuenteDe(valor string) (modelo.Fuente, error) {
	switch fuente := modelo.Fuente(valor); fuente {
	case modelo.FuenteChats, modelo.FuenteDocs, modelo.FuenteTablas:
		return fuente, nil
	}
	return "", fmt.Errorf("source %q no es chats, docs ni tables", valor)
}

// clavesRegistro lista los campos obligatorios del contrato por fuente: un
// registro al que le falta uno está fuera de esquema (422).
var clavesRegistro = map[modelo.Fuente][]string{
	modelo.FuenteChats:  {"id", "thread_id", "author", "ts", "text", "attachments"},
	modelo.FuenteDocs:   {"id", "format", "filename", "title", "body", "metadata"},
	modelo.FuenteTablas: {"id", "table", "columns"},
}

var (
	formatosDoc  = map[string]bool{"pdf": true, "html": true, "md": true}
	tablasFuente = map[string]bool{"clientes": true, "transacciones": true, "pqrs": true, "empleados": true}
)

// decodificaRegistros valida y arma los registros canónicos. Los mensajes de
// error citan índice y campo, nunca valores: un 422 no puede llevar PII.
func decodificaRegistros(fuente modelo.Fuente, crudos []json.RawMessage) ([]modelo.Registro, error) {
	salida := make([]modelo.Registro, 0, len(crudos))
	for i, crudo := range crudos {
		var claves map[string]json.RawMessage
		if err := json.Unmarshal(crudo, &claves); err != nil {
			return nil, fmt.Errorf("registro %d: no es un objeto con los campos del contrato", i)
		}
		for _, obligatoria := range clavesRegistro[fuente] {
			if _, ok := claves[obligatoria]; !ok {
				return nil, fmt.Errorf("registro %d: falta el campo %s", i, obligatoria)
			}
		}
		var registro modelo.Registro
		switch fuente {
		case modelo.FuenteChats:
			var chat modelo.ChatRecord
			if err := estricto(crudo, &chat); err != nil {
				return nil, fmt.Errorf("registro %d: %v", i, err)
			}
			registro = chat
		case modelo.FuenteDocs:
			var doc modelo.DocRecord
			if err := estricto(crudo, &doc); err != nil {
				return nil, fmt.Errorf("registro %d: %v", i, err)
			}
			if !formatosDoc[doc.Format] {
				return nil, fmt.Errorf("registro %d: format %q no es pdf, html ni md", i, doc.Format)
			}
			registro = doc
		case modelo.FuenteTablas:
			var tabla modelo.TableRecord
			if err := estricto(crudo, &tabla); err != nil {
				return nil, fmt.Errorf("registro %d: %v", i, err)
			}
			if !tablasFuente[tabla.Table] {
				return nil, fmt.Errorf("registro %d: table %q no es clientes, transacciones, pqrs ni empleados", i, tabla.Table)
			}
			registro = tabla
		}
		if registro.IDRegistro() == "" {
			return nil, fmt.Errorf("registro %d: id vacío", i)
		}
		salida = append(salida, registro)
	}
	return salida, nil
}

// estricto decodifica un registro con los tipos del contrato. Los campos que el
// esquema no declara se ignoran: el OpenAPI no los prohíbe y el jurado puede
// mandar campos de más.
func estricto(crudo json.RawMessage, destino any) error {
	decodificador := json.NewDecoder(bytes.NewReader(crudo))
	if err := decodificador.Decode(destino); err != nil {
		return errors.New("campos o tipos fuera del esquema del registro")
	}
	return nil
}
