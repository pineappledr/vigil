package smart

import "testing"

// TestSanearRawDesempaqueta usa el valor REAL del SanDisk SD9SB8W de la flota:
// Program_Fail_Cnt_Total = 204475917, que serían ~204 millones de fallos de
// escritura en un disco que responde y pasa el self-test.
func TestSanearRawDesempaqueta(t *testing.T) {
	casos := []struct {
		nombre string
		id     int
		raw    int64
		quiero int64
	}{
		{"SanDisk 181 de producción", 181, 204475917, 13},
		{"770 sectores reasignados es real", 5, 770, 770},
		{"2335 errores CRC son reales", 199, 2335, 2335},
		{"un timeout es un timeout", 188, 1, 1},
		{"cero", 5, 0, 0},
		{"atributo sin regla se respeta", 9, 999999999, 999999999},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := sanearRaw(c.id, c.raw); got != c.quiero {
				t.Errorf("sanearRaw(%d, %d) = %d, quiero %d", c.id, c.raw, got, c.quiero)
			}
		})
	}
}

// TestCriterioGraduado fija la decisión de diseño: el daño del plato es crítico
// al primer sector, pero un error de firmware aislado es warning, no crítico.
// Marcar crítico al primer timeout hizo que el SanDisk (1 timeout) apareciera
// junto a un disco con 770 sectores muertos, y eso entierra la señal real.
func TestCriterioGraduado(t *testing.T) {
	casos := []struct {
		nombre string
		id     int
		raw    int64
		quiero string
	}{
		// Daño del plato: sin gradación posible.
		{"1 sector reasignado", 5, 1, SeverityCritical},
		{"1 sector pendiente", 197, 1, SeverityCritical},
		{"770 reasignados (Samsung de Friday)", 5, 770, SeverityCritical},

		// Firmware/enlace: graduado.
		{"1 command timeout (SanDisk)", 188, 1, SeverityWarning},
		{"13 program fails (SanDisk saneado)", 181, 204475917, SeverityWarning},
		{"muchos program fails sí es crítico", 181, 500, SeverityCritical},

		// Sanos.
		{"sin sectores reasignados", 5, 0, SeverityHealthy},
		{"sin timeouts", 188, 0, SeverityHealthy},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := GetAttributeSeverity(c.id, c.raw, 0, 0); got != c.quiero {
				t.Errorf("attr %d raw %d -> %q, quiero %q", c.id, c.raw, got, c.quiero)
			}
		})
	}
}
