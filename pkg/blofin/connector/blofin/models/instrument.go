package models

type Instrument struct {
	InstID        string `json:"instId"`
	BaseCurrency  string `json:"baseCurrency"`
	QuoteCurrency string `json:"quoteCurrency"`
	ContractValue string `json:"contractValue"`
	ContractType  string `json:"contractType"`
	Status        string `json:"state"`
}
