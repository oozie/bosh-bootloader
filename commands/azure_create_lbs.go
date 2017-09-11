package commands

import (
	"github.com/cloudfoundry/bosh-bootloader/storage"
)

type AzureCreateLBs struct {
}

type AzureCreateLBsConfig struct {
	LBType       string
	CertPath     string
	KeyPath      string
	ChainPath    string
	Domain       string
	SkipIfExists bool
}

func NewAzureCreateLBs() AzureCreateLBs {
	return AzureCreateLBs{}
}

func (c AzureCreateLBs) Execute(config AzureCreateLBsConfig, state storage.State) error {
	return nil
}
