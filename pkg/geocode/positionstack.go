package geocode

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	// the free subscription doesn't support HTTPS.
	host = "http://api.positionstack.com"
)

func NewPositionStackClient(h *http.Client, apiKey string) *psc {
	return &psc{h: h, apiKey: apiKey}
}

type psc struct {
	h      *http.Client
	apiKey string
}

var _ Client = (*psc)(nil)

func (c *psc) queryWithDefaults() url.Values {
	v := url.Values{}
	v.Set("access_key", c.apiKey)
	return v
}

func (c *psc) Geocode(query string) (*Location, error) {
	q := c.queryWithDefaults()
	q.Set("query", query)
	q.Set("limit", "1")
	url := fmt.Sprintf("%s/v1/forward?%s", host, q.Encode())

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	res, err := c.h.Do(req)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 {
		var d ErrorResponse
		err = json.Unmarshal(data, &d)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf(d.Error.Message)
	}

	var d PositionStackLocationResponse
	err = json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	if len(d.Data) == 0 {
		return nil, fmt.Errorf("no location data returned by API")
	}
	return &Location{
		Latitude:  d.Data[0].Latitude,
		Longitude: d.Data[0].Longitude,
		Name:      d.Data[0].Name,
		Country:   d.Data[0].Country,
	}, nil
}

func (c *psc) ReverseGeocode(lat, lon float64) (*Location, error) {
	q := c.queryWithDefaults()
	q.Set("query", fmt.Sprintf("%f,%f", lat, lon))
	url := fmt.Sprintf("http://api.positionstack.com/v1/reverse?%s", q.Encode())

	res, err := c.h.Get(url)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 {
		var d ErrorResponse
		err = json.Unmarshal(data, &d)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf(d.Error.Message)
	}

	var d PositionStackLocationResponse
	err = json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	if len(d.Data) == 0 {
		return nil, fmt.Errorf("no location data returned by API")
	}
	return &Location{
		Latitude:  d.Data[0].Latitude,
		Longitude: d.Data[0].Longitude,
		Name:      d.Data[0].Name,
		Country:   d.Data[0].Country,
	}, nil
}

type PositionStackLocationResponse struct {
	Data []PositionStackLocation `json:"data"`
}

// FIXME: some properties are commented out because the API returns Javascript
// nulls... which is invalid JSON :-/
type PositionStackLocation struct {
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Label     string  `json:"label,omitempty"`
	Name      string  `json:"name,omitempty"`
	Type      string  `json:"type,omitempty"`
	Distance  float64 `json:"distance,omitempty"`
	// Number        string  `json:"number,omitempty"`
	// Street        string  `json:"street,omitempty"`
	// PostalCode    string  `json:"postal_code,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	// Region        string  `json:"region,omitempty"`
	// RegionCode    string  `json:"region_code,omitempty"`
	// Neighbourhood string  `json:"neighbourhood,omitempty"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	Continent   string `json:"continent,omitempty"`
	MapURL      string `json:"map_url,omitempty"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Context struct {
			Query []struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"query"`
		} `json:"context"`
	} `json:"error"`
}
