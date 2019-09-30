package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/sethgrid/pester"
)

func newPesterClient(httpClient *http.Client) *pester.Client {
	pesterClient := pester.NewExtendedClient(httpClient)
	pesterClient.Concurrency = 1
	pesterClient.MaxRetries = 5
	pesterClient.Backoff = pester.ExponentialBackoff
	pesterClient.KeepLog = true
	return pesterClient
}

func PrepareHTTPSClient() *pester.Client {
	sysCertStoreFailed := false
	embeddedCertFailed := false

	certPool, err := x509.SystemCertPool()
	if err != nil || certPool == nil {
		sysCertStoreFailed = true
		certPool = x509.NewCertPool()
		log.Println("INFO: PrepareHTTPClient() failed to get the system's cert store; using embedded cert")
	}
	ok := certPool.AppendCertsFromPEM([]byte(PinnedCert))
	if !ok {
		embeddedCertFailed = false
		log.Println("INFO: PrepareHTTPClient() failed to parse the embedded cert; using system cert store")
	}

	if sysCertStoreFailed && embeddedCertFailed {
		log.Fatal("ERROR: Both embedded cert and system cert store failed.")
	}

	tlsConfig := &tls.Config{
		RootCAs: certPool,
	}
	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	return newPesterClient(client)
}

type VaultV1Data map[string]string

type VaultV2Data struct {
	Data VaultV1Data `json:"data"`
}

type VaultReadResponse interface {
	GetSecret(string) (string, error)
}

type VaultBaseResponse struct {
	Warnings      []string `json:"warnings" yaml:"warnings" `
	Errors        []string `json:"errors" yaml:"errors" `
	RequestID     string   `json:"request_id" yaml:"request_id" `
	LeaseID       string   `json:"lease_id" yaml:"lease_id" `
	Renewable     bool     `json:"renewable" yaml:"renewable" `
	LeaseDuration int      `json:"lease_duration" yaml:"lease_duration" `
}

type VaultV1Response struct {
	VaultBaseResponse
	Data VaultV1Data `json:"data" yaml:"data" `
}

func NewVaultV1Response(data []byte) (VaultReadResponse, error) {
	var resp VaultV1Response

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (r VaultV1Response) GetSecret(key string) (string, error) {
	var value string
	var ok bool

	if value, ok = r.Data[key]; !ok {
		return "", errors.New(fmt.Sprintf("no value for key: %s", key))
	}
	return value, nil
}

type VaultV2Response struct {
	VaultBaseResponse
	Data VaultV2Data `json:"data" yaml:"data" `
}

func NewVaultV2Response(data []byte) (VaultReadResponse, error) {
	var resp VaultV2Response

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (r VaultV2Response) GetSecret(key string) (string, error) {
	var value string
	var ok bool

	if value, ok = r.Data.Data[key]; !ok {
		return "", errors.New(fmt.Sprintf("no value for key: %s", key))
	}
	return value, nil
}

type VaultClient struct {
	vaultAddress   string
	cluster        string
	jwt            string
	authURL        string
	namespace      string
	role           string
	client         *pester.Client
	token          string
	BackendVersion string
}

var vaultClient *VaultClient

func NewVaultClient() *VaultClient {
	if vaultClient == nil {
		vaultAddress := GetenvSafe("VAULT_ADDR", true)
		cluster := GetenvSafe("KUBERNETES_CLUSTER", true)
		jwt := GetenvSafe("VAULT_SERVICE_ACCOUNT_JWT", true)
		authURL := fmt.Sprintf("%s/v1/auth/kubernetes-%s/login", vaultAddress, cluster)
		namespace := GetNamespace()
		role := fmt.Sprintf("%s-vault-sa", namespace)
		client := PrepareHTTPSClient()
		vaultBackendVersion := GetenvSafe("VAULT_KV_VERSION", false)
		token := GetenvSafe("VAULT_TOKEN", false)

		vaultClient = &VaultClient{
			vaultAddress:   vaultAddress,
			cluster:        cluster,
			jwt:            jwt,
			authURL:        authURL,
			namespace:      namespace,
			role:           role,
			client:         client,
			BackendVersion: vaultBackendVersion,
			token:          token,
		}
	}
	return vaultClient
}

func (vc VaultClient) payloadJSON() []byte {
	if payload, err := json.Marshal(map[string]string{"jwt": vc.jwt, "role": vc.role}); err != nil {
		log.Fatalf("error creating vault JSON payload: %s", err.Error())
	} else {
		return payload
	}
	return nil
}

func (vc VaultClient) newAuthRequest() *http.Request {
	var req *http.Request
	var err error

	if req, err = http.NewRequest(http.MethodPost, vc.authURL, bytes.NewBuffer(vc.payloadJSON())); err != nil {
		log.Fatalf("error creating vault auth request: %s", err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (vc VaultClient) newReadSecretRequest(secretPath string) *http.Request {
	var req *http.Request
	var err error

	secretURL := fmt.Sprintf("%s/v1/%s", vc.vaultAddress, secretPath)
	if req, err = http.NewRequest("GET", secretURL, nil); err != nil {
		log.Fatalf("error creating vault read request: %s", err.Error())
	}
	req.Header.Set("X-Vault-Token", vc.token)
	return req
}

func (vc VaultClient) auth() (*http.Response, error) {
	req := vc.newAuthRequest()
	return vc.client.Do(req)
}

func (vc VaultClient) readSecret(secretPath string) (VaultReadResponse, error) {
	req := vc.newReadSecretRequest(secretPath)
	if resp, err := vc.client.Do(req); err != nil {
		return nil, err
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("WARN: error closing read request response body: %s\n", err.Error())
			}
		}()
		if err != nil {
			return nil, errors.New(fmt.Sprintf("FetchSecret() failed reading auth request response body: %s\n", err.Error()))
		}
		if resp.StatusCode != 200 {
			return nil, vaultSecretFetcherError{fmt.Sprintf("FetchSecret() failed to fetch '%s' - Response code: %d", secretPath, resp.StatusCode)}
		}
		if vc.BackendVersion == "" || vc.BackendVersion == "1" {
			return NewVaultV1Response([]byte(body))
		} else if vc.BackendVersion == "2" {
			return NewVaultV2Response([]byte(body))
		} else {
			return nil, errors.New(fmt.Sprintf("received invalid kv version of: %s", vc.BackendVersion))
		}
	}
}

func (vc VaultClient) LogString() string {
	return vc.client.LogString()
}

func (vc *VaultClient) SetToken(token string) {
	vc.token = token
}
