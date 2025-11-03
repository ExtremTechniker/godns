package model

type Record struct {
	Domain string `json:"domain"`
	QType  string `json:"qtype"`
	TTL    int    `json:"ttl"`
	Value  string `json:"value"`
}
