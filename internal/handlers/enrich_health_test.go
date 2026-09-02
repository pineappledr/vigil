package handlers

import (
	"encoding/json"
	"testing"
)

// TestEnrichDrivesWithHealth usa la forma REAL del payload de smartctl, que es
// donde falló la primera versión: deserializar el disco directo a
// DriveSmartData compilaba y no daba error, pero la lista de atributos quedaba
// vacía (el agente manda ata_smart_attributes.table, el struct espera
// `attributes`) y TODOS los discos salían HEALTHY. El campo llegaba a la UI
// siempre con el mismo valor, que es peor que no mandarlo.
func TestEnrichDrivesWithHealth(t *testing.T) {
	// Un disco con 770 sectores reasignados: el Samsung real de la flota.
	raw := `{"drives":[{
	  "serial_number":"S6PWNS0T803253J",
	  "model_name":"Samsung SSD 870 EVO 1TB",
	  "smart_status":{"passed":true},
	  "ata_smart_attributes":{"table":[
	    {"id":5,"name":"Reallocated_Sector_Ct","value":98,"thresh":10,"raw":{"value":770}}
	  ]}
	},{
	  "serial_number":"SANO123",
	  "model_name":"Disco sano",
	  "smart_status":{"passed":true},
	  "ata_smart_attributes":{"table":[
	    {"id":5,"name":"Reallocated_Sector_Ct","value":100,"thresh":10,"raw":{"value":0}}
	  ]}
	}]}`

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}

	enrichDrivesWithHealth(data, "friday")

	drives := data["drives"].([]interface{})
	malo := drives[0].(map[string]interface{})
	bueno := drives[1].(map[string]interface{})

	if malo["_health"] != "CRITICAL" {
		t.Errorf("disco con 770 sectores reasignados: _health = %v, quiero CRITICAL", malo["_health"])
	}
	if bueno["_health"] != "HEALTHY" {
		t.Errorf("disco sin defectos: _health = %v, quiero HEALTHY", bueno["_health"])
	}
	// El bug original daba el MISMO veredicto a los dos.
	if malo["_health"] == bueno["_health"] {
		t.Error("ambos discos con el mismo veredicto: los atributos no se están leyendo")
	}
}
