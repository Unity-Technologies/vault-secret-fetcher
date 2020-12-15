package e2e

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/gruntwork-io/terratest/modules/k8s"
)

func kubectlApplyWithTemplate(t *testing.T, kubectlOptions *k8s.KubectlOptions, templatePath string, values interface{}) {
	config, err := renderTemplateToString(t, templatePath, values)
	if err != nil {
		t.Error(err)
	}
	k8s.KubectlApplyFromString(t, kubectlOptions, config)
}

func renderTemplateToString(t *testing.T, f string, data interface{}) (string, error) {
	tmpl, err := template.New("").ParseFiles(f)
	baseName := filepath.Base(f)
	if err != nil {
		t.Error(err)
	}
	config := bytes.NewBufferString("")
	err = tmpl.ExecuteTemplate(config, baseName, data)
	return config.String(), err
}

func readEnvWithDefault(name, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}
