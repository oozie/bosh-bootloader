package storage

import "reflect"

type Jumpbox struct {
	Enabled   bool                   `json:"enabled"`
	URL       string                 `json:"url"`
	Variables string                 `json:"variables"`
	Manifest  string                 `json:"manifest"`
	State     map[string]interface{} `json:"state"`
}

func (j Jumpbox) IsEmpty() bool {
	return reflect.DeepEqual(j, Jumpbox{})
}
