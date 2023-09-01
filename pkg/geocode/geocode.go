package geocode

type Client interface {
	Geocode(query string) (*Location, error)
	ReverseGeocode(lat, long float64) (*Location, error)
}

type Location struct {
	Latitude    float64
	Longitude   float64
	Name        string
	Country     string
	CountryCode string
}
