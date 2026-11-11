package destination

type Candidate struct {
	AddressOnlyGoogle bool    `json:"-"`
	GoogleResult      bool    `json:"-"`
	GoogleName        string  `json:"-"`
	GoogleAddress     string  `json:"-"`
	City              string  `json:"city,omitempty"`
	State             string  `json:"state,omitempty"`
	Label             string  `json:"label"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
}
