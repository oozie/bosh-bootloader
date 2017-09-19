package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	uuid "github.com/nu7hatch/gouuid"
)

var (
	marshalIndent = json.MarshalIndent
	uuidNewV4     = uuid.NewV4
)

const (
	STATE_VERSION = 10

	OS_READ_WRITE_MODE = os.FileMode(0644)
	StateFileName      = "bbl-state.json"
)

type logger interface {
	Println(message string)
}

type AWS struct {
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	Region          string `json:"region"`
}

type Azure struct {
	SubscriptionID string `json:"subscriptionId"`
	TenantID       string `json:"tenantId"`
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
}

type GCP struct {
	ServiceAccountKey string   `json:"serviceAccountKey,omitempty"`
	ProjectID         string   `json:"projectID,omitempty"`
	Zone              string   `json:"zone"`
	Region            string   `json:"region"`
	Zones             []string `json:"zones"`
}

type Stack struct {
	Name            string `json:"name"`
	LBType          string `json:"lbType"`
	CertificateName string `json:"certificateName"`
	BOSHAZ          string `json:"boshAZ"`
}

type LB struct {
	Type   string `json:"type"`
	Cert   string `json:"cert"`
	Key    string `json:"key"`
	Chain  string `json:"chain"`
	Domain string `json:"domain,omitempty"`
}

type State struct {
	Version                    int     `json:"version"`
	IAAS                       string  `json:"iaas"`
	ID                         string  `json:"id"`
	NoDirector                 bool    `json:"noDirector"`
	MigratedFromCloudFormation bool    `json:"migratedFromCloudFormation"`
	AWS                        AWS     `json:"aws,omitempty"`
	Azure                      Azure   `json:"azure,omitempty"`
	GCP                        GCP     `json:"gcp,omitempty"`
	KeyPair                    KeyPair `json:"keyPair,omitempty"`
	Jumpbox                    Jumpbox `json:"jumpbox,omitempty"`
	BOSH                       BOSH    `json:"bosh,omitempty"`
	Stack                      Stack   `json:"stack"`
	EnvID                      string  `json:"envID"`
	TFState                    string  `json:"tfState"`
	LB                         LB      `json:"lb"`
	LatestTFOutput             string  `json:"latestTFOutput"`
}

type Store struct {
	version   int
	stateFile string
}

func NewStore(dir string) Store {
	return Store{
		version:   STATE_VERSION,
		stateFile: filepath.Join(dir, StateFileName),
	}
}

func (s Store) Set(state State) error {
	_, err := os.Stat(filepath.Dir(s.stateFile))
	if err != nil {
		return err
	}

	if reflect.DeepEqual(state, State{}) {
		err := os.Remove(s.stateFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		return nil
	}

	state.Version = s.version

	if state.ID == "" {
		uuid, err := uuidNewV4()
		if err != nil {
			return fmt.Errorf("Create state ID: %s", err)
		}
		state.ID = uuid.String()
	}


	state.AWS.AccessKeyID = ""
	state.AWS.SecretAccessKey = ""
	state.GCP.ServiceAccountKey = ""
	state.GCP.ProjectID = ""

	jsonData, err := marshalIndent(state, "", "\t")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(s.stateFile, jsonData, os.FileMode(0644))
	if err != nil {
		return err
	}

	return nil
}

func (g GCP) Empty() bool {
	return g.ServiceAccountKey == "" && g.ProjectID == "" && g.Region == "" && g.Zone == ""
}

var GetStateLogger logger

func GetState(dir string) (State, error) {
	state := State{}

	_, err := os.Stat(dir)
	if err != nil {
		return state, err
	}

	file, err := os.Open(filepath.Join(dir, StateFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}

	err = json.NewDecoder(file).Decode(&state)
	if err != nil {
		return state, err
	}

	emptyState := State{}
	if reflect.DeepEqual(state, emptyState) {
		state = State{
			Version: STATE_VERSION,
		}
	}

	if state.Version < 3 {
		return state, errors.New("Existing bbl environment is incompatible with bbl v3. Create a new environment with v3 to continue.")
	}

	if state.Version > STATE_VERSION {
		return state, fmt.Errorf("Existing bbl environment was created with a newer version of bbl. Please upgrade to a version of bbl compatible with schema version %d.\n", state.Version)
	}

	return state, nil
}

func stateAndBBLStateExist(dir string) (bool, error) {
	stateFile := filepath.Join(dir, "state.json")
	_, err := os.Stat(stateFile)
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return false, err
	}

	bblStateFile := filepath.Join(dir, StateFileName)
	_, err = os.Stat(bblStateFile)
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}
