package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const (
	namespacePath                     = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	secretFetcherVersionName          = "FETCHER_FORMAT_VERSION"
	secretFetcherDebugMode = "FETCHER_DEBUG"
	PinnedCert = `-----BEGIN CERTIFICATE-----.-----END CERTIFICATE-----`
)

var (
	build string
	debugMode = os.Getenv(secretFetcherDebugMode) == "true"
)

type vaultSecretFetcherError struct {
	msg string
}

func (e vaultSecretFetcherError) Error() string {
	return fmt.Sprintf("%s", e.msg)
}

func SysExec() {
	flag.Parse()

	cmd, err := exec.LookPath(os.Args[1])
	if err != nil {
		log.Fatalf("Fatal error: SysExec() failed to locate the entrypoint '%s' - %s", os.Args[1], err)
	}

	if err := syscall.Exec(cmd, flag.Args(), os.Environ()); err != nil {
		log.Fatalf("Fatal error: SysExec() failed to perform execv syscall - %s", err)
	}
}

func GetNamespace() string {
	namespaceFile, err := ioutil.ReadFile(namespacePath)
	if err != nil {
		log.Fatalf("ERROR: GetNamespace() failed to read namespace from %s", namespacePath)
	}

	namespace := strings.TrimSpace(string(namespaceFile))
	if len(namespace) == 0 {
		log.Fatalf("ERROR: Namespace value in %s is empty", namespacePath)
	}

	return namespace
}

func GetenvSafe(key string, strict bool) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		if strict {
			log.Fatalf("ERROR: GetenvSafe() failed to load '%s'. It's either empty or not set", key)
		}
		if debugMode {
			log.Printf("INFO: GetenvSafe() failed to load '%s'. It's either empty or not set", key)
		}
		return ""
	}
	return value
}

func GetFormatVersion() string {
	var value string
	var ok bool

	if value, ok = os.LookupEnv(secretFetcherVersionName); ok {
		switch value {
		case SecretFormatV1, SecretFormatV2:
			return value
		default:
			return value
		}
	}
	return SecretFormatV1
}

func Auth() {
	client := NewVaultClient()
	log.Printf("INFO: authenticating with endpoint '%s' using role %s", client.authURL, client.role)

	resp, err := client.auth()
	if err != nil {
		log.Printf("%s", err)
		log.Print(client.LogString())
		os.Exit(1)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("WARN: error closing auth request response body: %s\n", err.Error())
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("ERROR: Auth() failed reading auth request response body: %s\n", err.Error())
	}
	if resp.StatusCode != 200 {
		log.Fatalf("ERROR: Auth() failed to authenticate with Vault - Response code: %d - %s", resp.StatusCode, body)
	}

	var responseInterface map[string]interface{}
	err = json.Unmarshal(body, &responseInterface)
	if err != nil {
		log.Fatalf("ERROR: Auth() failed to unmarshal JSON response: %s\n", err.Error())
	}
	token := responseInterface["auth"].(map[string]interface{})["client_token"]
	if tokenStr, ok := token.(string); ok {
		client.SetToken(tokenStr)
		return
	}

	log.Fatal("Fatal error: Auth() failed to read token from authentication JSON response")
}

func FetchSecret(secret Secret) error {
	var resp VaultReadResponse
	var secretStr string
	var err error

	client := NewVaultClient()

	if resp, err = client.readSecret(secret.GetPath()); err != nil {
		return err
	}

	if secretStr, err = resp.GetSecret(secret.GetKey()); err != nil {
		message := fmt.Sprintf("error extracting secret [%s] from response: %s", secret.GetKey(), err.Error())
		return errors.New(message)
	}
	secret.SetValue(secretStr)
	return nil
}

func SetSecretToEnvVar(varName, value string) error {
	return os.Setenv(varName, value)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s /path/to/entrypoint\n\n", os.Args[0])
		flag.PrintDefaults()
	}

	versionFlagPointer := flag.Bool("v", false, "Print version number")
	flag.Parse()

	if *versionFlagPointer {
		fmt.Printf("Vault Secret Fetcher - revision: %s \n", build)
		os.Exit(0)
	}

	log.Printf("INFO: Vault Secret Fetcher started (revision: %s)", build)
	secretsFetched := 0

	if len(os.Args) == 1 {
		log.Fatal("ERROR: Missing entrypoint argument")
	}

	token := os.Getenv("VAULT_TOKEN")
	if len(token) == 0 {
		Auth()
	} else {
		log.Println("INFO: A VAULT_TOKEN has been provided. Will skip authentication and use the provided token")
	}

	secretFormat := GetFormatVersion()
	matcher := NewMatcher(secretFormat)
	for _, e := range os.Environ() {
		var secret Secret
		var err error

		if secret, err = matcher.Match(e); err != nil {
			switch err.(type) {
			case NoMatchError:
				if debugMode {
					log.Printf("DEBUG: %s", err.Error())
				}
				continue
			default:
				log.Fatalf("ERROR: %s", err.Error())
			}
		}
		if err := FetchSecret(secret); err != nil {
			log.Fatalf("ERROR: Failed to retrieve secret from %s::%s: %s", secret.GetPath(), secret.GetKey(), err.Error())
		}
		secretsFetched = secretsFetched + 1
		if err := SetSecretToEnvVar(secret.VarName(), secret.GetValue()); err != nil {
			message := fmt.Sprintf("ERROR: Failed to set %s in environment: %s", secret.VarName(), err.Error())
			log.Fatalf(message)
		}
	}

	log.Printf("INFO: Secrets fetched: %d/%d", secretsFetched, matcher.ToFetch())

	if secretsFetched != matcher.ToFetch() {
		log.Fatal("ERROR: Was not able to successfully fetch/set all secrets. Failing deployment")
	}

	// Equivalent to 'exec $@'.
	SysExec()
}
