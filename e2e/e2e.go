package e2e

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
)

type Secret struct {
	Path    string
	Key     string
	Value   string
	Env     string
	Version int
}

type K8SCluster interface {
	GetK8SOptions() *k8s.KubectlOptions
	LoadImages(images ...string) error
	Delete() error
}

type TestContext struct {
	cluster        K8SCluster
	KubectlOptions *k8s.KubectlOptions
	Secrets        []Secret
	VSFImage       string
	KindImage      string
	VaultImage     string
}

func (c *TestContext) setup(vsfImage, vaultImage, kindImage string, secrets []Secret) (*TestContext, error) {
	c.Secrets = secrets
	c.VSFImage = vsfImage
	c.VaultImage = vaultImage
	log.Println("Preparing local k8s cluster for tests...")
	k8sCluster, err := NewKindCluster(kindImage)
	if err != nil {
		return nil, err
	}
	log.Println("Local k8s cluster is ready")
	c.cluster = k8sCluster
	c.KubectlOptions = c.cluster.GetK8SOptions()
	log.Printf("Uploading %s image to k8s...\n", c.VSFImage)
	err = c.cluster.LoadImages(c.VSFImage, c.VaultImage)
	if err != nil {
		return nil, err
	}
	log.Printf("The image %s was uploaded...\n", c.VSFImage)
	return c, nil
}

func (c *TestContext) teardown() error {
	log.Println("Deleting local k8s cluster...")
	return c.cluster.Delete()
}

func (c *TestContext) withDependencies(t *testing.T, dependencies ...*Dependency) {
	for _, dependency := range dependencies {
		if dependency.isApplied {
			continue
		}
		dependency.mu.Lock()
		defer dependency.mu.Unlock()
		log.Printf("Applying dependency %s for tests...\n", dependency.name)
		dependency.apply(t, dependency, c)
	}
}

type testContextApplyFunc func(t *testing.T, d *Dependency, c *TestContext)

type Dependency struct {
	mu sync.Mutex

	name      string
	apply     testContextApplyFunc
	isApplied bool
}

var Vault = &Dependency{
	name: "vault",
	apply: func(t *testing.T, d *Dependency, c *TestContext) {
		templatePath := "./resources/vault.yml.tmpl"
		values := struct {
			VaultImage string
			Secrets    []Secret
		}{
			VaultImage: c.VaultImage,
			Secrets:    c.Secrets,
		}
		kubectlApplyWithTemplate(t, c.KubectlOptions, templatePath, values)
		k8s.WaitUntilPodAvailable(t, c.KubectlOptions, "vault", 10, 5*time.Second)
		_, err := k8s.RunKubectlAndGetOutputE(t, c.KubectlOptions, "exec", "vault", "--", "/bin/sh", "-c", "/vault-config/init.sh")
		if err != nil {
			t.Error(err)
			return
		}
		d.isApplied = true
	},
}

var AppWithV1Matcher = &Dependency{
	name: "app-with-matcher-v1",
	apply: func(t *testing.T, d *Dependency, c *TestContext) {
		templatePath := "./resources/app-with-matcher-v1.yml.tmpl"
		values := struct {
			PodImage string
			Secrets  []Secret
		}{
			PodImage: c.VSFImage,
			Secrets:  c.Secrets,
		}
		kubectlApplyWithTemplate(t, c.KubectlOptions, templatePath, values)
		k8s.WaitUntilPodAvailable(t, c.KubectlOptions, "app-with-matcher-v1", 10, 5*time.Second)
		d.isApplied = true
	},
}

var AppWithV2Matcher = &Dependency{
	name: "app-with-matcher-v2",
	apply: func(t *testing.T, d *Dependency, c *TestContext) {
		templatePath := "./resources/app-with-matcher-v2.yml.tmpl"
		values := struct {
			PodImage string
			Secrets  []Secret
		}{
			PodImage: c.VSFImage,
			Secrets:  c.Secrets,
		}
		kubectlApplyWithTemplate(t, c.KubectlOptions, templatePath, values)
		k8s.WaitUntilPodAvailable(t, c.KubectlOptions, "app-with-matcher-v2", 10, 5*time.Second)
		d.isApplied = true
	},
}
