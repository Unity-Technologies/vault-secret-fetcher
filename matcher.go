package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	SecretFormatV1       = "1"
	SecretFormatV2       = "2"
	v1CheckPattern       = "{{[\\s]*vault-secret ([a-zA-Z0-9_-]+\\/[a-zA-Z0-9\\/_-]+)[\\s]*}}$"
	v1VaultSecretPattern = "{{[\\s]*vault-secret "
	v1Key                = "secret"
	v2SecretPattern      = `(?P<prefix>VAULTSECRET::)(?P<info>\{.*\})`
)

// SecretFormatError is a custom error type.
type SecretFormatError struct {
	Message string
}

func (e SecretFormatError) Error() string {
	return e.Message
}

// NewSecretFormatError creates a new `SecretFormatError`.
func NewSecretFormatError(message string) SecretFormatError {
	return SecretFormatError{Message: message}
}

// NoMatchError is a custom error type.
type NoMatchError struct {
	Message string
}

func (e NoMatchError) Error() string {
	return e.Message
}

// NewNoMatchError creates a new `NoMatchError`.
func NewNoMatchError(varName string, matcherVersion string) NoMatchError {
	message := fmt.Sprintf("'%s' does not match a v%s SecretMatcher secret", varName, matcherVersion)
	return NoMatchError{Message: message}
}

func SecretPrinter(s Secret) string {
	return fmt.Sprintf("<v%sSecret: %s:%s>", s.Version(), s.GetPath(), s.GetKey())
}

type Secret interface {
	GetPath() string
	GetKey() string
	VarName() string
	SetValue(string)
	GetValue() string
	Version() string
	// Including Stringer interface so developers MUST implement a method for
	// printing objects that *DOES NOT* print the secret value.
	fmt.Stringer
}

type SecretMatcher interface {
	Match(string) (Secret, error)
	ToFetch() int
	Version() string
}

type v1Secret struct {
	path    string
	key     string
	varName string
	value   string
	version string
}

func (s v1Secret) GetPath() string {
	return s.path
}

func (s v1Secret) GetKey() string {
	return s.key
}

func (s v1Secret) VarName() string {
	return s.varName
}

func (s *v1Secret) SetValue(value string) {
	s.value = value
}

func (s v1Secret) GetValue() string {
	return s.value
}

func (s v1Secret) Version() string {
	return s.version
}

func (s *v1Secret) String() string {
	return SecretPrinter(s)
}

func newV1Secret(varName, path string) *v1Secret {
	return &v1Secret{path: path, key: v1Key, varName: varName, version: SecretFormatV1}
}

type V1Matcher struct {
	checkRegex       *regexp.Regexp
	vaultSecretRegex *regexp.Regexp
	version          string
	toFetch          int
}

func NewV1Matcher() *V1Matcher {
	return &V1Matcher{
		version:          SecretFormatV1,
		checkRegex:       regexp.MustCompile(v1CheckPattern),
		vaultSecretRegex: regexp.MustCompile(v1VaultSecretPattern),
	}
}

func (m *V1Matcher) Match(str string) (Secret, error) {
	envVarLine := strings.Split(str, "=")
	if m.vaultSecretRegex.MatchString(envVarLine[1]) {
		m.toFetch++
		if m.checkRegex.MatchString(envVarLine[1]) {
			extracted := m.checkRegex.FindStringSubmatch(envVarLine[1])
			return newV1Secret(envVarLine[0], extracted[1]), nil
		}
		message := fmt.Sprintf(
			"'%s' does not follow the correct path definition format.",
			envVarLine[1],
		)
		return nil, NewSecretFormatError(message)
	}
	return nil, NewNoMatchError(envVarLine[0], m.version)
}

func (m *V1Matcher) ToFetch() int {
	return m.toFetch
}

func (m V1Matcher) Version() string {
	return m.version
}

type v2Secret struct {
	Path    string `json:"path"`
	Key     string `json:"key"`
	varName string
	value   string
	version string
}

func newV2Secret(varName string, data []byte) (*v2Secret, error) {
	secret := v2Secret{varName: varName, version: SecretFormatV2}
	if err := json.Unmarshal(data, &secret); err != nil {
		message := fmt.Sprintf(
			"failed to parse SecretFormatV2 secret from '%s': %s",
			string(data),
			err.Error(),
		)
		return nil, NewSecretFormatError(message)
	}
	return &secret, nil
}

func (s v2Secret) GetPath() string {
	return s.Path
}

func (s v2Secret) GetKey() string {
	return s.Key
}

func (s v2Secret) VarName() string {
	return s.varName
}

func (s *v2Secret) SetValue(value string) {
	s.value = value
}

func (s v2Secret) GetValue() string {
	return s.value
}

func (s v2Secret) Version() string {
	return s.version
}

func (s *v2Secret) String() string {
	return SecretPrinter(s)
}

type V2Matcher struct {
	secretRegex *regexp.Regexp
	version     string
	toFetch     int
}

func NewV2Matcher() *V2Matcher {
	return &V2Matcher{
		version:     SecretFormatV2,
		secretRegex: regexp.MustCompile(v2SecretPattern),
	}
}

func (m *V2Matcher) Match(str string) (Secret, error) {
	envVarLine := strings.Split(str, "=")
	if m.secretRegex.MatchString(envVarLine[1]) {
		m.toFetch++
		js := strings.Split(envVarLine[1], "::")[1]
		return newV2Secret(envVarLine[0], []byte(js))
	}
	return nil, NewNoMatchError(envVarLine[0], m.version)
}

func (m V2Matcher) ToFetch() int {
	return m.toFetch
}

func (m V2Matcher) Version() string {
	return m.version
}

func NewMatcher(version string) SecretMatcher {
	switch version {
	case SecretFormatV1:
		return NewV1Matcher()
	case SecretFormatV2:
		return NewV2Matcher()
	default:
		return NewV1Matcher()
	}
}
