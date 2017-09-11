package azure_test

import (
	"io/ioutil"

	"github.com/cloudfoundry/bosh-bootloader/storage"
	"github.com/cloudfoundry/bosh-bootloader/terraform/azure"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("TemplateGenerator", func() {
	var (
		templateGenerator azure.TemplateGenerator
	)

	BeforeEach(func() {
		templateGenerator = azure.NewTemplateGenerator()
	})

	Describe("Generate", func() {
		DescribeTable("generates a terraform template for azure", func(fixtureFilename, lbType string) {
			expectedTemplate, err := ioutil.ReadFile(fixtureFilename)
			Expect(err).NotTo(HaveOccurred())

			template := templateGenerator.Generate(storage.State{
				EnvID: "azure-environment",
				Azure: storage.Azure{
					SubscriptionID: "subscription-id",
					TenantID:       "tenant-id",
					ClientID:       "client-id",
					ClientSecret:   "client-secret",
				},
				LB: storage.LB{
					Type: lbType,
				},
			})
			Expect(template).To(Equal(string(expectedTemplate)))
		},
			Entry("when no lb type is provided", "fixtures/azure_template.tf", ""),
			Entry("when a cf lb type is provided", "fixtures/azure_template_cf_lb.tf", "cf"),
		)
	})
})
