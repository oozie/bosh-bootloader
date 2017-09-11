package azure

import (
	"strings"

	"github.com/cloudfoundry/bosh-bootloader/storage"
)

type TemplateGenerator struct{}

func NewTemplateGenerator() TemplateGenerator {
	return TemplateGenerator{}
}

func (t TemplateGenerator) Generate(state storage.State) string {
	baseTemplate := strings.Join([]string{VarsTemplate, ResourceGroupTemplate, NetworkTemplate, StorageTemplate, NetworkSecurityGroupTemplate, OutputTemplate}, "\n")

	if state.LB.Type == "cf" {
		baseTemplate = strings.Join([]string{baseTemplate, CFLBTemplate}, "\n")
	}

	return baseTemplate
}
