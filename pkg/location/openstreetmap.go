package location

import (
	"github.com/codingsince1985/geo-golang"
	"github.com/codingsince1985/geo-golang/openstreetmap"
)

func NewOpenstreetmapClient() *oc {
	geocoder := openstreetmap.Geocoder()
	return &oc{geocoder: geocoder}
}

type oc struct {
	geocoder geo.Geocoder
}

func (c *oc) FindLocation(query string) (*Location, error) {
	location, err := c.geocoder.Geocode(query)
	if err != nil {
		return nil, err
	}

	address, err := c.geocoder.ReverseGeocode(location.Lat, location.Lng)
	if err != nil {
		return nil, err
	}

	return &Location{
		Latitude:    location.Lat,
		Longitude:   location.Lng,
		Name:        query,
		Country:     address.Country,
		CountryCode: address.CountryCode,
	}, nil
}

func (c *oc) ReverseFindLocation(lat, lon float64) (*Location, error) {
	address, err := c.geocoder.ReverseGeocode(lat, lon)
	if err != nil {
		return nil, err
	}

	return &Location{
		Latitude:    lat,
		Longitude:   lon,
		Name:        address.FormattedAddress,
		Country:     address.Country,
		CountryCode: address.CountryCode,
	}, nil
}
