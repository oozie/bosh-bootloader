package fakes

import (
	"github.com/cloudfoundry/bosh-bootloader/commands"
	"github.com/cloudfoundry/bosh-bootloader/storage"
)

type UpCmd struct {
	ExecuteCall struct {
		CallCount int
		Receives  struct {
			UpConfig commands.UpConfig
			State    storage.State
		}
		Returns struct {
			Error error
		}
	}
}

func (u *UpCmd) Execute(upConfig commands.UpConfig, state storage.State) error {
	u.ExecuteCall.CallCount++
	u.ExecuteCall.Receives.UpConfig = upConfig
	u.ExecuteCall.Receives.State = state
	return u.ExecuteCall.Returns.Error
}
