package commands_test

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/bosh-bootloader/bosh"
	"github.com/cloudfoundry/bosh-bootloader/commands"
	"github.com/cloudfoundry/bosh-bootloader/fakes"
	"github.com/cloudfoundry/bosh-bootloader/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	variablesYAML = `admin_password: some-admin-password
director_ssl:
  ca: some-ca
  certificate: some-certificate
  private_key: some-private-key
`
)

var _ = Describe("GCPUp", func() {
	var (
		gcpUp                 commands.GCPUp
		stateStore            *fakes.StateStore
		terraformManager      *fakes.TerraformManager
		boshManager           *fakes.BOSHManager
		cloudConfigManager    *fakes.CloudConfigManager
		envIDManager          *fakes.EnvIDManager
		terraformManagerError *fakes.TerraformManagerError
		gcpZones              *fakes.GCPClient

		bblState               storage.State
		expectedZonesState     storage.State
		expectedTerraformState storage.State
		expectedBOSHState      storage.State

		expectedTerraformTemplate string

		expectedAvailabilityZones []string
	)

	BeforeEach(func() {
		boshManager = &fakes.BOSHManager{}
		cloudConfigManager = &fakes.CloudConfigManager{}
		envIDManager = &fakes.EnvIDManager{}
		gcpZones = &fakes.GCPClient{}
		stateStore = &fakes.StateStore{}
		terraformManager = &fakes.TerraformManager{}
		terraformManagerError = &fakes.TerraformManagerError{}

		bblState = storage.State{
			EnvID: "some-env-id",
			GCP: storage.GCP{
				Region: "some-region",
			},
		}

		expectedZonesState = bblState
		expectedZonesState.GCP.Zones = []string{"some-zone", "some-other-zone"}

		expectedTerraformState = expectedZonesState
		expectedTerraformState.TFState = "some-tf-state"

		expectedBOSHState = expectedTerraformState
		expectedBOSHState.BOSH = storage.BOSH{
			DirectorName:           "bosh-some-env-id",
			DirectorUsername:       "admin",
			DirectorPassword:       "some-admin-password",
			DirectorAddress:        "some-director-address",
			DirectorSSLCA:          "some-ca",
			DirectorSSLCertificate: "some-certificate",
			DirectorSSLPrivateKey:  "some-private-key",
			State: map[string]interface{}{
				"new-key": "new-value",
			},
			Variables: variablesYAML,
			Manifest:  "some-bosh-manifest",
		}

		expectedAvailabilityZones = []string{"some-zone", "some-other-zone"}

		terraformManager.VersionCall.Returns.Version = "0.8.7"
		envIDManager.SyncCall.Returns.State = storage.State{EnvID: "some-env-id"}
		terraformManager.ApplyCall.Returns.BBLState = expectedTerraformState
		boshManager.CreateDirectorCall.Returns.State = expectedBOSHState
		boshManager.CreateJumpboxCall.Returns.State = expectedBOSHState
		gcpZones.GetZonesCall.Returns.Zones = expectedAvailabilityZones

		gcpUp = commands.NewGCPUp(
			stateStore,
			terraformManager,
			boshManager,
			cloudConfigManager,
			envIDManager,
			gcpZones,
		)

		body, err := ioutil.ReadFile("fixtures/terraform_template_no_lb.tf")
		Expect(err).NotTo(HaveOccurred())

		expectedTerraformTemplate = string(body)
	})

	AfterEach(func() {
		commands.ResetMarshal()
	})

	Describe("Execute", func() {
		It("creates the environment", func() {
			err := gcpUp.Execute(commands.UpConfig{}, bblState)
			Expect(err).NotTo(HaveOccurred())

			By("retrieves the env ID", func() {
				Expect(envIDManager.SyncCall.CallCount).To(Equal(1))
				Expect(envIDManager.SyncCall.Receives.State).To(Equal(bblState))
				Expect(envIDManager.SyncCall.Receives.Name).To(BeEmpty())
			})

			By("saving the resulting state with the env ID", func() {
				Expect(stateStore.SetCall.CallCount).To(BeNumerically(">=", 1))
				Expect(stateStore.SetCall.Receives[0].State).To(Equal(bblState))
			})

			By("getting gcp availability zones", func() {
				Expect(gcpZones.GetZonesCall.CallCount).To(Equal(1))
				Expect(gcpZones.GetZonesCall.Receives.Region).To(Equal("some-region"))
			})

			By("saving gcp zones to the state", func() {
				Expect(stateStore.SetCall.CallCount).To(BeNumerically(">=", 2))
				Expect(stateStore.SetCall.Receives[1].State).To(Equal(expectedZonesState))
			})

			By("creating gcp resources via terraform", func() {
				Expect(terraformManager.ApplyCall.CallCount).To(Equal(1))
				Expect(terraformManager.ApplyCall.Receives.BBLState).To(Equal(expectedZonesState))
			})

			By("saving the terraform state to the state", func() {
				Expect(stateStore.SetCall.CallCount).To(BeNumerically(">=", 3))
				Expect(stateStore.SetCall.Receives[2].State).To(Equal(expectedTerraformState))
			})

			By("getting the terraform outputs", func() {
				Expect(terraformManager.GetOutputsCall.CallCount).To(Equal(1))
				Expect(terraformManager.GetOutputsCall.Receives.BBLState).To(Equal(expectedTerraformState))
			})

			By("creating a jumpbox", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(boshManager.CreateJumpboxCall.Receives.State).To(Equal(expectedTerraformState))
			})

			By("creating a bosh", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(boshManager.CreateDirectorCall.Receives.State).To(Equal(expectedBOSHState))
			})

			By("saving the bosh state to the state", func() {
				Expect(stateStore.SetCall.CallCount).To(BeNumerically(">=", 4))
				Expect(stateStore.SetCall.Receives[3].State).To(Equal(expectedBOSHState))
			})

			By("updating the cloud config", func() {
				Expect(cloudConfigManager.UpdateCall.CallCount).To(Equal(1))
				Expect(cloudConfigManager.UpdateCall.Receives.State).To(Equal(expectedBOSHState))
			})
		})

		Context("when a name is passed in for env-id", func() {
			It("passes that name in for the env id manager to use", func() {
				err := gcpUp.Execute(commands.UpConfig{
					Name: "some-other-env-id",
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())

				Expect(envIDManager.SyncCall.CallCount).To(Equal(1))
				Expect(envIDManager.SyncCall.Receives.Name).To(Equal("some-other-env-id"))
			})
		})

		Context("when ops file are passed in", func() {
			It("passes the ops file contents to the bosh manager", func() {
				opsFile, err := ioutil.TempFile("", "ops-file")
				Expect(err).NotTo(HaveOccurred())

				opsFilePath := opsFile.Name()
				opsFileContents := "some-ops-file-contents"
				err = ioutil.WriteFile(opsFilePath, []byte(opsFileContents), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				err = gcpUp.Execute(commands.UpConfig{
					OpsFile: opsFilePath,
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())

				Expect(boshManager.CreateDirectorCall.Receives.State.BOSH.UserOpsFile).To(Equal("some-ops-file-contents"))
			})
		})

		Context("when the no-director flag is provided", func() {
			BeforeEach(func() {
				terraformManager.ApplyCall.Returns.BBLState.NoDirector = true
			})

			It("does not create a bosh or update cloud config", func() {
				err := gcpUp.Execute(commands.UpConfig{
					NoDirector: true,
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())

				Expect(terraformManager.ApplyCall.CallCount).To(Equal(1))
				Expect(boshManager.CreateJumpboxCall.CallCount).To(Equal(0))
				Expect(boshManager.CreateDirectorCall.CallCount).To(Equal(0))
				Expect(cloudConfigManager.UpdateCall.CallCount).To(Equal(0))
				Expect(stateStore.SetCall.CallCount).To(Equal(3))
				Expect(stateStore.SetCall.Receives[2].State.NoDirector).To(Equal(true))
			})

			Context("when re-bbling up an environment with no director", func() {
				It("does not create a bosh director", func() {
					err := gcpUp.Execute(commands.UpConfig{}, storage.State{
						NoDirector: true,
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(terraformManager.ApplyCall.CallCount).To(Equal(1))
					Expect(boshManager.CreateJumpboxCall.CallCount).To(Equal(0))
					Expect(boshManager.CreateDirectorCall.CallCount).To(Equal(0))
					Expect(cloudConfigManager.UpdateCall.CallCount).To(Equal(0))
					Expect(stateStore.SetCall.CallCount).To(Equal(3))
					Expect(stateStore.SetCall.Receives[2].State.NoDirector).To(Equal(true))
				})
			})
		})

		Context("reentrance", func() {
			It("calls terraform manager with previous state", func() {
				expectedZonesState.TFState = "existing-tf-state"
				err := gcpUp.Execute(commands.UpConfig{}, expectedZonesState)
				Expect(err).NotTo(HaveOccurred())

				Expect(terraformManager.ApplyCall.CallCount).To(Equal(1))
				Expect(terraformManager.ApplyCall.Receives.BBLState).To(Equal(expectedZonesState))
			})
		})

		Context("failure cases", func() {
			It("returns an error if terraform manager version validator fails", func() {
				terraformManager.ValidateVersionCall.Returns.Error = errors.New("cannot validate version")
				err := gcpUp.Execute(commands.UpConfig{}, storage.State{})

				Expect(err).To(MatchError("cannot validate version"))
			})

			It("returns an error when the ops file cannot be read", func() {
				err := gcpUp.Execute(commands.UpConfig{
					OpsFile: "some/fake/path",
				}, storage.State{})
				Expect(err).To(MatchError("error reading ops-file contents: open some/fake/path: no such file or directory"))
			})

			Context("when a bbl environment exists with a bosh director", func() {
				It("fast fails before creating any infrastructure", func() {
					err := gcpUp.Execute(commands.UpConfig{
						NoDirector: true,
					}, storage.State{
						BOSH: storage.BOSH{
							DirectorName: "some-director",
						},
					})
					Expect(err).To(MatchError(`Director already exists, you must re-create your environment to use "--no-director"`))

					Expect(envIDManager.SyncCall.CallCount).To(Equal(0))
					Expect(terraformManager.ApplyCall.CallCount).To(Equal(0))
					Expect(boshManager.CreateDirectorCall.CallCount).To(Equal(0))
				})
			})

			It("fast fails if a gcp environment with the same name already exists", func() {
				envIDManager.SyncCall.Returns.Error = errors.New("environment already exists")
				err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
				Expect(err).To(MatchError("environment already exists"))
			})

			It("returns an error when state store fails to set after syncing env id", func() {
				stateStore.SetCall.Returns = []fakes.SetCallReturn{{Error: errors.New("set call failed")}}
				err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
				Expect(err).To(MatchError("set call failed"))
			})

			It("returns an error when GCP AZs cannot be retrieved", func() {
				gcpZones.GetZonesCall.Returns.Error = errors.New("can't get gcp availability zones")
				err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
				Expect(err).To(MatchError("can't get gcp availability zones"))
			})

			It("returns an error when the state fails to be set after retrieving GCP zones", func() {
				stateStore.SetCall.Returns = []fakes.SetCallReturn{{}, {errors.New("state failed to be set")}}
				err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
				Expect(err).To(MatchError("state failed to be set"))
			})

			Context("terraform manager error handling", func() {
				BeforeEach(func() {
					terraformManagerError.ErrorCall.Returns = "failed to apply"
					terraformManagerError.BBLStateCall.Returns.BBLState = storage.State{
						TFState: "some-updated-tf-state",
					}
				})

				It("saves the tf state when the applier fails", func() {
					terraformManager.ApplyCall.Returns.Error = terraformManagerError
					err := gcpUp.Execute(commands.UpConfig{}, storage.State{})

					Expect(err).To(MatchError("failed to apply"))
					Expect(stateStore.SetCall.CallCount).To(Equal(3))
					Expect(stateStore.SetCall.Receives[2].State.TFState).To(Equal("some-updated-tf-state"))
				})

				It("returns an error when the applier fails and we cannot retrieve the updated bbl state", func() {
					terraformManagerError.BBLStateCall.Returns.Error = errors.New("some-bbl-state-error")
					terraformManager.ApplyCall.Returns.Error = terraformManagerError
					err := gcpUp.Execute(commands.UpConfig{}, storage.State{})

					Expect(err).To(MatchError("the following errors occurred:\nfailed to apply,\nsome-bbl-state-error"))
					Expect(stateStore.SetCall.CallCount).To(Equal(2))
				})

				It("returns an error if applier fails with non terraform manager apply error", func() {
					terraformManager.ApplyCall.Returns.Error = errors.New("failed to apply")
					err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
					Expect(err).To(MatchError("failed to apply"))
				})

				It("returns an error when the terraform manager fails, we can retrieve the updated bbl state, and state fails to be set", func() {
					updatedBBLState := storage.State{}
					updatedBBLState.TFState = "some-updated-tf-state"

					terraformManagerError.BBLStateCall.Returns.BBLState = updatedBBLState
					terraformManager.ApplyCall.Returns.Error = terraformManagerError

					stateStore.SetCall.Returns = []fakes.SetCallReturn{{}, {}, {errors.New("state failed to be set")}}
					err := gcpUp.Execute(commands.UpConfig{}, storage.State{})

					Expect(err).To(MatchError("the following errors occurred:\nfailed to apply,\nstate failed to be set"))
					Expect(stateStore.SetCall.CallCount).To(Equal(3))
					Expect(stateStore.SetCall.Receives[2].State.TFState).To(Equal("some-updated-tf-state"))
				})
			})

			It("returns an error when the state fails to be set after applying terraform", func() {
				stateStore.SetCall.Returns = []fakes.SetCallReturn{{}, {}, {}, {errors.New("state failed to be set")}}
				err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
				Expect(err).To(MatchError("state failed to be set"))
			})

			It("returns an error when ther terraform manager fails to get outputs", func() {
				terraformManager.GetOutputsCall.Returns.Error = errors.New("nope")
				err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
				Expect(err).To(MatchError("nope"))
			})

			Context("bosh manager error handling", func() {
				Context("when bosh manager fails with bosh manager create error", func() {
					var (
						expectedBOSHState map[string]interface{}
					)

					BeforeEach(func() {
						expectedBOSHState = map[string]interface{}{
							"partial": "bosh-state",
						}

						newState := storage.State{}
						newState.BOSH.State = expectedBOSHState

						expectedError := bosh.NewManagerCreateError(newState, errors.New("failed to create"))
						boshManager.CreateDirectorCall.Returns.Error = expectedError
					})

					It("returns the error and saves the state", func() {
						err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
						Expect(err).To(MatchError("failed to create"))
						Expect(stateStore.SetCall.CallCount).To(Equal(4))
						Expect(stateStore.SetCall.Receives[3].State.BOSH.State).To(Equal(expectedBOSHState))
					})

					It("returns a compound error when it fails to save the state", func() {
						stateStore.SetCall.Returns = []fakes.SetCallReturn{{}, {}, {}, {errors.New("state failed to be set")}}

						err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
						Expect(err).To(MatchError("the following errors occurred:\nfailed to create,\nstate failed to be set"))
						Expect(stateStore.SetCall.CallCount).To(Equal(4))
						Expect(stateStore.SetCall.Receives[3].State.BOSH.State).To(Equal(expectedBOSHState))
					})
				})

				It("returns an error when bosh manager fails to create a bosh with a non bosh manager create error", func() {
					boshManager.CreateDirectorCall.Returns.Error = errors.New("failed to create")
					err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
					Expect(err).To(MatchError("failed to create"))
				})
			})

			It("returns an error when the state fails to be set after deploying bosh", func() {
				stateStore.SetCall.Returns = []fakes.SetCallReturn{{}, {}, {}, {errors.New("state failed to be set")}}
				err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
				Expect(err).To(MatchError("state failed to be set"))
			})

			It("returns an error when the cloud config manager fails to update", func() {
				cloudConfigManager.UpdateCall.Returns.Error = errors.New("failed to update")
				err := gcpUp.Execute(commands.UpConfig{}, storage.State{})
				Expect(err).To(MatchError("failed to update"))
			})
		})
	})
})
