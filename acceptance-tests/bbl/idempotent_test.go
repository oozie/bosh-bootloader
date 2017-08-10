package acceptance_test

import (
	"fmt"

	acceptance "github.com/cloudfoundry/bosh-bootloader/acceptance-tests"
	"github.com/cloudfoundry/bosh-bootloader/acceptance-tests/actors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("idempotent test", func() {
	var (
		bbl           actors.BBL
		configuration acceptance.Config
	)

	BeforeEach(func() {
		var err error
		configuration, err = acceptance.LoadConfig()
		Expect(err).NotTo(HaveOccurred())

		envIDSuffix := fmt.Sprintf("reentrant-%d", GinkgoParallelNode())
		bbl = actors.NewBBL(configuration.StateFileDir, pathToBBL, configuration, envIDSuffix)
	})

	AfterEach(func() {
		bbl.Destroy()
	})

	It("is able to bbl up idempotently with a director", func() {
		bbl.Up(configuration.IAAS, []string{"--name", bbl.PredefinedEnvID()})
		bbl.Up(configuration.IAAS, []string{})
	})

	It("is able to bbl up idempotently with no director", func() {
		bbl.Up(configuration.IAAS, []string{"--name", bbl.PredefinedEnvID(), "--no-director"})
		bbl.Up(configuration.IAAS, []string{})
	})
})
