// Package llm es el cliente de Jev 1.14 (snapshot typesafe/jev-1.14-20261014)
// sobre el endpoint de decisiones de OpenRouter (D13, D32). Solo hace preguntas
// cerradas: clasificar un fragmento, desempatar entre candidatos, decidir el
// género de un nombre. Nunca genera ni reescribe texto.
//
// Una llamada por pregunta con su propio state (D28): el state de un registro
// sesga las respuestas sobre otro. El paralelismo se limita con un semáforo de
// 16 llamadas simultáneas dentro del cliente, así que quien llama puede lanzar
// una goroutine por mención.
//
// JSON que se envía, POST <URLBase>/api/alpha/decisions:
//
//	{
//	  "model": "typesafe/jev-1.14-20261014",
//	  "provider": {"data_collection": "deny"},
//	  "state": "[PERSONA_1] pide una reposición de su saldo",
//	  "questions": {
//	    "q1": {
//	      "type": "choice",
//	      "instructions": "¿A qué tipo de entidad corresponde el fragmento?",
//	      "criteria": {"person": null, "organization_or_place": null, "other": null}
//	    }
//	  }
//	}
//
// El state es string (el esquema acepta string, objeto o arreglo; aquí siempre
// llega la ventana mínima ya enmascarada por Enmascarador, D15). Las claves de
// questions son el ID de modelo.Pregunta. Sin descripciones por opción en el
// contrato de C0, criteria viaja con las opciones y null como descripción; un
// choice rico en matices pediría un CONTRATO nuevo.
//
// JSON que se espera:
//
//	{
//	  "id": "gen-dec-...",
//	  "model": "typesafe/jev-1.14-20261014",
//	  "provider": "TypeSafe",
//	  "answers": {
//	    "q1": {"type": "choice", "choice": "person", "confidence": 0.92,
//	           "probabilities": {"person": 0.92, "other": 0.08}}
//	  },
//	  "usage": {"cost": 0.00002, "input_tokens": 476, "output_tokens": 70}
//	}
//
// answer por tipo:
//   - choice: choice (la opción ganadora) y probabilities por opción.
//   - noul: noul es P(verdadero); se derivan {"true": noul, "false": 1-noul}.
//   - score: score (número) y, cuando viene, legend de índice a criterio; las
//     probabilidades se reetiquetan con el texto del legend para que la opción
//     elegida sea el criterio y no el índice.
//
// Los umbrales de D31 se aplican sobre probabilities: la respuesta solo se
// acepta si la probabilidad de la opción elegida supera el umbral. Si no, vuelve
// ErrBajaConfianza junto con la respuesta, para que el pipeline mande la mención
// a la cola de dudosos (D19) sin perder la evidencia.
//
// La caché es por hash de (snapshot, state enmascarado y pregunta), guardada en
// la tabla cache_modelo de SQLite (D32). El snapshot entra en la llave: mover el
// alias no reusa decisiones viejas.
package llm
