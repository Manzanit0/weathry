package geocode

import (
	"fmt"

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

var _ Client = (*oc)(nil)

func (c *oc) Geocode(query string) (*Location, error) {
	location, err := c.geocoder.Geocode(query)
	if err != nil {
		return nil, err
	}

	if location == nil {
		return nil, fmt.Errorf("unable to geocode address")
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

func (c *oc) ReverseGeocode(lat, lon float64) (*Location, error) {
	address, err := c.geocoder.ReverseGeocode(lat, lon)
	if err != nil {
		return nil, err
	}

	if address == nil {
		return nil, fmt.Errorf("unable to reverse geocode location")
	}

	return &Location{
		Latitude:    lat,
		Longitude:   lon,
		Name:        fmt.Sprintf("%s, %s", address.City, address.Country),
		Country:     address.Country,
		CountryCode: address.CountryCode,
	}, nil
}
