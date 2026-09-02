package smart

import "testing"

// TestTempFromRaw usa los valores REALES que la flota reportó el 2026-09-02,
// cuando 13 de 17 discos aparecían CRITICAL por este atributo mientras
// smart_passed era true y ninguno tenía un sector reasignado.
func TestTempFromRaw(t *testing.T) {
	cases := []struct {
		name string
		raw  int64
		want int
	}{
		// Firmwares que ya devuelven los grados limpios.
		{"limpio 34C", 34, 34},
		{"limpio 46C", 46, 46},

		// Los de producción: el raw empaqueta actual|min<<16|max<<32.
		// 236224315425 = 0x37_00230021 -> byte bajo 0x21 = 33 °C, que es
		// exactamente lo que la UI mostraba para ese disco.
		{"Brain HDWG180 (UI decía 33C)", 236224315425, 33},
		{"Brain HDWG180 (UI decía 34C)", 236224380962, 34},

		// Basura pura: mejor "sin dato" que una alarma inventada.
		{"cero", 0, 0},
		{"negativo", -1, 0},

		// Un disco de verdad caliente sigue detectándose.
		{"70C real", 70, 70},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := tempFromRaw(c.raw); got != c.want {
				t.Errorf("tempFromRaw(%d) = %d, quiero %d", c.raw, got, c.want)
			}
		})
	}
}

// TestTemperatureSeverityNoFalseCritical es la regresión concreta: esos raws
// cruzaban el umbral de 65 °C y marcaban CRITICAL discos sanos.
func TestTemperatureSeverityNoFalseCritical(t *testing.T) {
	for _, raw := range []int64{236224315425, 236224380962, 257699217439, 270584053791} {
		if sev := GetAttributeSeverity(194, raw, 0, 0); sev == SeverityCritical {
			t.Errorf("raw %d marcado CRITICAL; es un disco sano con el raw empaquetado", raw)
		}
	}
	// Y un disco realmente caliente NO debe pasar desapercibido.
	if sev := GetAttributeSeverity(194, 70, 0, 0); sev != SeverityCritical {
		t.Errorf("70 °C debería ser CRITICAL, dio %q", sev)
	}
}
