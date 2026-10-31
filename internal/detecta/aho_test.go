package detecta

import "testing"

func TestACEncuentraAgujasQueCompartenPrefijo(t *testing.T) {
	agujas := [][]byte{
		[]byte("kelly"),
		[]byte("kelly castano"),
		[]byte("kelly_castano"),
		[]byte("kelly.castano@kipupay.co"),
		[]byte("castano"),
	}
	a := nuevoAC(agujas)
	texto := "preparado por kelly castano, contadora"
	var valores []string
	for _, c := range a.busca(texto) {
		valores = append(valores, texto[c.inicio:c.fin])
	}
	t.Logf("coincidencias: %v", valores)
	if len(valores) == 0 {
		t.Fatal("no encontró ninguna aguja")
	}
}
