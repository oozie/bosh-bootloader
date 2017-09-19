package bosh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type Client interface {
	UpdateCloudConfig(yaml []byte) error
	Info() (Info, error)
}

type Info struct {
	Name    string `json:"name"`
	UUID    string `json:"uuid"`
	Version string `json:"version"`
}

var (
	MAX_RETRIES = 5
	RETRY_DELAY = 10 * time.Second
)

type client struct {
	directorAddress string
	username        string
	password        string
	caCert          string
	httpClient      *http.Client
}

func NewClient(httpClient *http.Client, directorAddress, username, password, caCert string) Client {
	return client{
		directorAddress: directorAddress,
		username:        username,
		password:        password,
		caCert:          caCert,
		httpClient:      httpClient,
	}
}

func (c client) Info() (Info, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/info", c.directorAddress), strings.NewReader(""))
	if err != nil {
		return Info{}, err
	}

	response, err := makeRequests(c.httpClient, request)
	if err != nil {
		return Info{}, err
	}

	if response.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("unexpected http response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	var info Info
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		return Info{}, err
	}

	return info, nil
}

func (c client) UpdateCloudConfig(yaml []byte) error {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/cloud_configs", c.directorAddress), bytes.NewBuffer(yaml))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "text/yaml")

	urlParts, err := url.Parse(c.directorAddress)
	if err != nil {
		return err //not tested
	}

	boshHost, _, err := net.SplitHostPort(urlParts.Host)
	if err != nil {
		return err //not tested
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)

	conf := &clientcredentials.Config{
		ClientID:     c.username,
		ClientSecret: c.password,
		TokenURL:     fmt.Sprintf("https://%s:8443/oauth/token", boshHost),
	}

	httpClient := conf.Client(ctx)
	response, err := makeRequests(httpClient, request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected http response %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}

	return nil
}

func makeRequests(httpClient *http.Client, request *http.Request) (*http.Response, error) {
	var (
		response *http.Response
		err      error
	)

	for i := 0; i < MAX_RETRIES; i++ {
		response, err = httpClient.Do(request)
		if err == nil {
			break
		}
		time.Sleep(RETRY_DELAY)
	}
	if err != nil {
		return &http.Response{}, fmt.Errorf("made %d attempts, last error: %s", MAX_RETRIES, err)
	}

	return response, nil
}
