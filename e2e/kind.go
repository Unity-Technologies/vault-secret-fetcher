package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/fs"
)

type KindCluster struct {
	Name     string
	Provider *cluster.Provider
}

func NewKindCluster(nodeImage string) (*KindCluster, error) {
	clusterName := fmt.Sprintf("cluster-%s", strings.ToLower(random.UniqueId()))
	f, err := ioutil.TempFile("", "kubecfg-")
	if err != nil {
		return nil, err
	}
	os.Setenv("KUBECONFIG", f.Name())
	provider := cluster.NewProvider()
	err = provider.Create(
		clusterName,
		cluster.CreateWithNodeImage(nodeImage),
		cluster.CreateWithWaitForReady(120*time.Second),
	)
	if err != nil {
		return nil, err
	}
	s, err := provider.KubeConfig(clusterName, true)
	f.Write([]byte(s))
	return &KindCluster{Provider: provider, Name: clusterName}, nil
}

func (c *KindCluster) GetK8SOptions() *k8s.KubectlOptions {
	return k8s.NewKubectlOptions("kind-"+c.Name, "", "default")
}

func (c *KindCluster) Delete() error {
	return c.Provider.Delete(c.Name, "")
}

func (c *KindCluster) LoadImage(image string) error {
	dir, err := fs.TempDir("", "image-tar")
	defer os.RemoveAll(dir)
	imageTarName := filepath.Join(dir, "image.tar")
	err = c.saveImage(image, imageTarName)
	if err != nil {
		return err
	}
	f, err := os.Open(imageTarName)
	if err != nil {
		return fmt.Errorf("failed to open image %s", image)
	}
	defer f.Close()
	nodes, err := c.Provider.ListNodes(c.Name)
	if err != nil {
		return err
	}
	err = nil
	for _, node := range nodes {
		err = nodeutils.LoadImageArchive(node, f)
	}
	return err
}

func (c *KindCluster) saveImage(image, dest string) error {
	return exec.Command("docker", "save", "-o", dest, image).Run()
}

func (c *KindCluster) LoadImages(images ...string) error {
	var err error
	for _, image := range images {
		err = c.LoadImage(image)
	}
	return err
}
