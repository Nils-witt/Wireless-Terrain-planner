// Package utm projects WGS84/ETRS89 geographic coordinates to ETRS89 / UTM
// zone 32N (EPSG:25832), the CRS of the DGM1 elevation tiles.
//
// It uses the Krüger n-series (Karney 2011), which is accurate to well below a
// millimetre inside a UTM zone. WGS84 and ETRS89 are treated as identical, as
// PROJ does for EPSG:4326 -> EPSG:25832.
package utm

import "math"

// GRS80 ellipsoid and UTM zone 32N parameters.
const (
	semiMajor = 6378137.0
	flatten   = 1 / 298.257222101
	k0        = 0.9996
	lon0      = 9.0 // central meridian in degrees
	falseE    = 500000.0
)

var (
	ecc   = math.Sqrt(flatten * (2 - flatten))
	n     = flatten / (2 - flatten)
	rectA = semiMajor / (1 + n) * (1 + n*n/4 + math.Pow(n, 4)/64 + math.Pow(n, 6)/256)
	alpha = [6]float64{
		n/2 - 2*n*n/3 + 5*math.Pow(n, 3)/16 + 41*math.Pow(n, 4)/180 - 127*math.Pow(n, 5)/288 + 7891*math.Pow(n, 6)/37800,
		13*n*n/48 - 3*math.Pow(n, 3)/5 + 557*math.Pow(n, 4)/1440 + 281*math.Pow(n, 5)/630 - 1983433*math.Pow(n, 6)/1935360,
		61*math.Pow(n, 3)/240 - 103*math.Pow(n, 4)/140 + 15061*math.Pow(n, 5)/26880 + 167603*math.Pow(n, 6)/181440,
		49561*math.Pow(n, 4)/161280 - 179*math.Pow(n, 5)/168 + 6601661*math.Pow(n, 6)/7257600,
		34729*math.Pow(n, 5)/80640 - 3418889*math.Pow(n, 6)/1995840,
		212378941 * math.Pow(n, 6) / 319334400,
	}
)

// Forward converts longitude/latitude in degrees to easting/northing in
// metres (EPSG:25832).
func Forward(lonDeg, latDeg float64) (easting, northing float64) {
	phi := latDeg * math.Pi / 180
	lam := (lonDeg - lon0) * math.Pi / 180

	sinPhi := math.Sin(phi)
	t := math.Sinh(math.Atanh(sinPhi) - ecc*math.Atanh(ecc*sinPhi))
	xiP := math.Atan2(t, math.Cos(lam))
	etaP := math.Atanh(math.Sin(lam) / math.Hypot(1, t))

	xi, eta := xiP, etaP
	for j, a := range alpha {
		k := float64(2 * (j + 1))
		xi += a * math.Sin(k*xiP) * math.Cosh(k*etaP)
		eta += a * math.Cos(k*xiP) * math.Sinh(k*etaP)
	}
	return falseE + k0*rectA*eta, k0 * rectA * xi
}
