package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/k8s"
)

var testContext = &TestContext{}

var (
	defaultVaultImage = "vault:1.6.0"
	defaultKindImage  = "kindest/node:v1.20.0"
)

var secrets = []Secret{
	{Path: "secrets/a", Key: "secret", Value: "valueA", Env: "SECRET_A", Version: 1},
	{Path: "secrets/b", Key: "secret", Value: "valueB", Env: "SECRET_B", Version: 1},
	{Path: "secrets/c", Key: "secret", Value: "valueC", Env: "SECRET_C", Version: 1},
	{Path: "secrets/d", Key: "secret", Value: "valueD", Env: "SECRET_D", Version: 2},
	{Path: "secrets/e", Key: "key1", Value: "valueE", Env: "SECRET_E", Version: 2},
	{Path: "secrets/f", Key: "key2", Value: "valueF", Env: "SECRET_F", Version: 2},
}

type Config struct {
	VSFImage   string
	VaultImage string
	KindImage  string
}

func (c *Config) String() string {
	w := bytes.NewBufferString("")
	json.NewEncoder(w).Encode(c)
	return w.String()
}

func initConfig() (*Config, error) {
	vsfImage := readEnvWithDefault("E2E_VAULT_SECRET_FETCHER_IMAGE", "")
	if vsfImage == "" {
		return nil, errors.New("environment variable VAULT_SECRET_FETCHER_IMAGE is empty")
	}

	vaultImage := readEnvWithDefault("E2E_VAULT_IMAGE", defaultVaultImage)
	kindImage := readEnvWithDefault("E2E_K8S_KIND_IMAGE", defaultKindImage)

	return &Config{
		VSFImage: vsfImage, VaultImage: vaultImage, KindImage: kindImage,
	}, nil
}

func TestMain(m *testing.M) {
	config, err := initConfig()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Running e2e test for vault-secret-fetcher with config: %s", config)
	tContext, err := testContext.setup(config.VSFImage, config.VaultImage, config.KindImage, secrets)
	if err != nil {
		log.Fatal(err)
	}
	testContext = tContext
	exitCode := m.Run()
	err = testContext.teardown()
	if err != nil {
		log.Println(err)
	}
	os.Exit(exitCode)
}

func TestVSFMatcherV1(t *testing.T) {
	testContext.withDependencies(t, Vault, AppWithV1Matcher)
	stdout, err := k8s.RunKubectlAndGetOutputE(t, testContext.KubectlOptions, "exec", "app-with-matcher-v1", "--", "/bin/sh", "-c", "cat /tmp/env")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range testContext.Secrets {
		if tt.Version != 1 {
			continue
		}
		t.Run(tt.Path, func(t *testing.T) {
			want := fmt.Sprintf("%s=%s", tt.Env, tt.Value)
			if !strings.Contains(stdout, want) {
				t.Errorf("MatcherV1 = %v, want %v", stdout, want)
			}
		})
	}
}

func TestVSFMatcherV2(t *testing.T) {
	testContext.withDependencies(t, Vault, AppWithV2Matcher)
	stdout, err := k8s.RunKubectlAndGetOutputE(t, testContext.KubectlOptions, "exec", "app-with-matcher-v2", "--", "/bin/sh", "-c", "cat /tmp/env")
	if err != nil {
		t.Error(err)
	}
	for _, tt := range testContext.Secrets {
		if tt.Version != 2 {
			continue
		}
		t.Run(tt.Path, func(t *testing.T) {
			want := fmt.Sprintf("%s=%s", tt.Env, tt.Value)
			if !strings.Contains(stdout, want) {
				t.Errorf("MatcherV2 = %v, want %v", stdout, want)
			}
		})
	}
}
