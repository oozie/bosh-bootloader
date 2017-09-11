package fakes

import (
	"github.com/cloudfoundry/bosh-bootloader/commands"
	"github.com/cloudfoundry/bosh-bootloader/storage"
)

type AzureCreateLBs struct {
	Name        string
	ExecuteCall struct {
		CallCount int
		Receives  struct {
			Config commands.AzureCreateLBsConfig
			State  storage.State
		}
		Returns struct {
			Error error
		}
	}
}

func (u *AzureCreateLBs) Execute(config commands.AzureCreateLBsConfig, state storage.State) error {
	u.ExecuteCall.CallCount++
	u.ExecuteCall.Receives.Config = config
	u.ExecuteCall.Receives.State = state
	return u.ExecuteCall.Returns.Error
}
