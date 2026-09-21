package utm

import (
	"math"
	"testing"
)

// Reference values produced by PROJ: cs2cs EPSG:4326 EPSG:25832.
func TestForwardMatchesPROJ(t *testing.T) {
	tests := []struct {
		lon, lat, e, n float64
	}{
		{6.0, 50.0, 285015.763263098, 5542944.018526014},
		{6.9578, 51.2277, 357408.665180433, 5677128.082989279},
		{7.6261, 51.9607, 405600.612897732, 5757558.641196058},
		{8.5, 50.5, 464539.554187468, 5594344.786006397},
		{9.0, 52.0, 500000.000000000, 5761038.212466577},
		{9.8, 53.5, 553065.505375901, 5928191.564741489},
		{5.9, 51.0, 282496.247325031, 5654399.656371777},
	}
	for _, tt := range tests {
		e, n := Forward(tt.lon, tt.lat)
		if math.Abs(e-tt.e) > 1e-6 || math.Abs(n-tt.n) > 1e-6 {
			t.Errorf("Forward(%v, %v) = (%.9f, %.9f), want (%.9f, %.9f)", tt.lon, tt.lat, e, n, tt.e, tt.n)
		}
	}
}
