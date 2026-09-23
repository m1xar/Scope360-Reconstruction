package models

type Instrument struct {
	InstID   string `json:"instId"`
	InstType string `json:"instType"`
	BaseCcy  string `json:"baseCcy"`
	QuoteCcy string `json:"quoteCcy"`
	CtType   string `json:"ctType"`
	CtVal    string `json:"ctVal"`
	CtMult   string `json:"ctMult"`
	CtValCcy string `json:"ctValCcy"`
	State    string `json:"state"`
	ExpTime  string `json:"expTime"`
}

type Instrumentidentifier struct {
	InstID   string
	InstType string
}

var DefaultInstrument = Instrument{
	CtVal:  "1",
	CtMult: "1",
}
