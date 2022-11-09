package location

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Client interface {
	FindLocation(query string) (*Location, error)
}

type Location struct {
	Latitude  float64
	Longitude float64
	Name      string
	Region    string
	Country   string
}

func NewPositionStackClient(h *http.Client, apiKey string) *psc {
	return &psc{h: h, apiKey: apiKey}
}

type psc struct {
	h      *http.Client
	apiKey string
}

func (c *psc) FindLocation(query string) (*Location, error) {
	url := fmt.Sprintf("http://api.positionstack.com/v1/forward?access_key=%s&query=%s", c.apiKey, query)

	res, err := c.h.Get(url)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var d LocationRequestResponse
	err = json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	return &Location{
		Latitude:  d.Data[0].Latitude,
		Longitude: d.Data[0].Longitude,
		Name:      d.Data[0].Name,
		Region:    d.Data[0].Region,
		Country:   d.Data[0].Country,
	}, nil
}

type LocationRequestResponse struct {
	Data []LocationData `json:"data,omitempty"`
}

type LocationData struct {
	Latitude           float64 `json:"latitude,omitempty"`
	Longitude          float64 `json:"longitude,omitempty"`
	Label              string  `json:"label,omitempty"`
	Name               string  `json:"name,omitempty"`
	Type               string  `json:"type,omitempty"`
	Number             string  `json:"number,omitempty"`
	Street             string  `json:"street,omitempty"`
	PostalCode         string  `json:"postal_code,omitempty"`
	Confidence         float64 `json:"confidence,omitempty"`
	Region             string  `json:"region,omitempty"`
	RegionCode         string  `json:"region_code,omitempty"`
	AdministrativeArea string  `json:"administrative_area,omitempty"`
	Neighbourhood      string  `json:"neighbourhood,omitempty"`
	Country            string  `json:"country,omitempty"`
	Country_code       string  `json:"country_code,omitempty"`
	MapUrl             string  `json:"map_url,omitempty"`
}
