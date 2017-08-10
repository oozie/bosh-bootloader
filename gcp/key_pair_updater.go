package gcp

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	compute "google.golang.org/api/compute/v1"

	"github.com/cloudfoundry/bosh-bootloader/storage"

	"golang.org/x/crypto/ssh"
)

type KeyPairUpdater struct {
	retryCount            int
	random                io.Reader
	rsaKeyGenerator       rsaKeyGenerator
	sshPublicKeyGenerator sshPublicKeyGenerator
	client                metadataSetter
	logger                logger
}

type rsaKeyGenerator func(io.Reader, int) (*rsa.PrivateKey, error)
type sshPublicKeyGenerator func(interface{}) (ssh.PublicKey, error)

func NewKeyPairUpdater(random io.Reader, generateRSAKey rsaKeyGenerator, generateSSHPublicKey sshPublicKeyGenerator, client metadataSetter, logger logger) KeyPairUpdater {
	rand.Seed(time.Now().UnixNano())
	return KeyPairUpdater{
		random:                random,
		rsaKeyGenerator:       generateRSAKey,
		sshPublicKeyGenerator: generateSSHPublicKey,
		client:                client,
		logger:                logger,
	}
}

func (k KeyPairUpdater) Update() (storage.KeyPair, error) {
	privateKey, publicKey, err := k.createKeyPair()
	if err != nil {
		return storage.KeyPair{}, fmt.Errorf("create key pair: %s", err)
	}

	project, err := k.client.GetProject()
	if err != nil {
		return storage.KeyPair{}, fmt.Errorf("get project: %s", err)
	}

	sshKeyItemValue := fmt.Sprintf("vcap:%s vcap", strings.TrimSpace(publicKey))

	var updated bool
	for i, item := range project.CommonInstanceMetadata.Items {
		if item.Key == "sshKeys" {
			sshKeys := strings.Split(*item.Value, "\n")
			sshKeys = append(sshKeys, sshKeyItemValue)

			newValue := strings.Join(sshKeys, "\n")
			project.CommonInstanceMetadata.Items[i].Value = &newValue
			updated = true
			k.logger.Step("appending new ssh-keys for the project %q", project.Name)
			break
		}
	}

	if !updated {
		k.logger.Step("Creating new ssh-keys for the project %q", project.Name)
		sshKeyItem := &compute.MetadataItems{
			Key:   "sshKeys",
			Value: &sshKeyItemValue,
		}

		project.CommonInstanceMetadata.Items = append(project.CommonInstanceMetadata.Items, sshKeyItem)
	}

	_, err = k.client.SetCommonInstanceMetadata(project.CommonInstanceMetadata)
	if err != nil {
		k.retryCount++
		if k.retryCount < 3 {
			time.Sleep(time.Duration((500 + rand.Intn(1500))) * time.Millisecond)
			return k.Update()
		}
		return storage.KeyPair{}, fmt.Errorf("set common instance metadata: %s", err)
	}

	return storage.KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

func (keyPairUpdater KeyPairUpdater) createKeyPair() (string, string, error) {
	rsaKey, err := keyPairUpdater.rsaKeyGenerator(keyPairUpdater.random, 2048)
	if err != nil {
		return "", "", fmt.Errorf("rsa key generator: %s", err)
	}

	publicKey, err := keyPairUpdater.sshPublicKeyGenerator(rsaKey.Public())
	if err != nil {
		return "", "", fmt.Errorf("ssh public key generator: %s", err)
	}

	rawPublicKey := string(ssh.MarshalAuthorizedKey(publicKey))
	rawPublicKey = strings.TrimSuffix(rawPublicKey, "\n")

	privateKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
		},
	)

	return string(privateKey), rawPublicKey, nil
}
