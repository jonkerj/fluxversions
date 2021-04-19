package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"

	"helm.sh/helm/v3/pkg/repo"

	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2beta1"
)

type ReleaseInspector struct {
	kubeClient client.Client
}

func NewReleaseInspector(kubeConfig *restclient.Config) (*ReleaseInspector, error) {
	scheme := apiruntime.NewScheme()
	_ = sourcev1.AddToScheme(scheme)
	_ = helmv2.AddToScheme(scheme)


	kc, err := client.New(kubeConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}
	ri := new(ReleaseInspector)
	ri.kubeClient = kc
	return ri, nil
}

func (ri *ReleaseInspector) getIndex(release helmv2.HelmRelease) (*repo.IndexFile, error) {
	sr := release.Spec.Chart.Spec.SourceRef
	if sr.Kind != "HelmRepository" {
		return nil, nil
	}
	nameKey := types.NamespacedName{
		Name: sr.Name,
		Namespace: sr.Namespace,
	}
	var hr sourcev1.HelmRepository
	if err := ri.kubeClient.Get(context.Background(), nameKey, &hr); err != nil {
		return nil, fmt.Errorf("Error fetching HelmRelease: %s", err.Error())
	}

	url := hr.GetArtifact().URL

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Error fetching artifact: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("Received a non 200 response code")
	}

	tmpFile, err := ioutil.TempFile("/", "index.*.yaml")
	if err != nil {
		return nil, fmt.Errorf("Error creating temp file: %s", err.Error())
	}
	defer os.Remove(tmpFile.Name())

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error saving artifact to temp file: %s", err.Error())
	}
	err = tmpFile.Close()
	if err != nil {
		return nil, fmt.Errorf("Error closing temp file: %s", err.Error())
	}

	idx, err := repo.LoadIndexFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("Error creating helm repo object: %s", err.Error())
	}

	return idx, nil
}

func (ri *ReleaseInspector) Inspect(release helmv2.HelmRelease) (error){
	currentVer := release.Spec.Chart.Spec.Version
	if ! strings.HasPrefix(currentVer, "v") {
		currentVer = "v" + currentVer
	}

	idx, err := ri.getIndex(release)
	if err != nil {
		return fmt.Errorf("Error loading helm index: %s", err.Error())
	}
	if idx == nil {
		return nil
	}

	newest := ""
	for _, entries := range idx.Entries {
		for _, entry := range entries {
			if entry.Name == release.Spec.Chart.Spec.Chart {
				repoVer := entry.Version
				if ! strings.HasPrefix(repoVer, "v") {
					repoVer = "v" + repoVer
				}

				if semver.Compare(currentVer, repoVer) < 0 && (newest == "" || semver.Compare(newest, repoVer) < 0) {
					newest = repoVer
				}
			}
		}
	}
	if newest != "" {
		fmt.Printf("Release: %s/%s (chart %s) could be upgraded from %s to %s\n", release.Namespace, release.Name, release.Spec.Chart.Spec.Chart, currentVer, newest)
	}

	return nil
}

func (ri *ReleaseInspector) Releases() <-chan helmv2.HelmRelease {
	ch := make(chan helmv2.HelmRelease);
	go func () {
		var list helmv2.HelmReleaseList
		if err := ri.kubeClient.List(context.Background(), &list); err == nil {
			for _, release := range list.Items {
				ch <- release
			}
		} // TODO: log errors!
		close(ch)
	} ();
	return ch
}

func getKubeConfig() (*restclient.Config, error) {
	var kubeconfigPath *string

	defaultKubeconfigPath := ""
	if home := homedir.HomeDir(); home != "" {
		defaultKubeconfigPath = filepath.Join(home, ".kube", "config")
	}
	kubeconfigPath = flag.String("kubeconfig", defaultKubeconfigPath, "(optional) absolute path to the kubeconfig file")
	flag.Parse()

	// test if kubeconfig actually exists
	if _, err := os.Stat(*kubeconfigPath); err != nil {
		if os.IsNotExist(err) {
			// it does not exist. Attempt to load in-cluster credentials
			kubeconfig, err := restclient.InClusterConfig()
			if err != nil {
				return nil, fmt.Errorf("Error creating in-cluster config: %s", err.Error())
			}
			return kubeconfig, nil
		}
	}
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("Error creating config from kubeconfig file: %s", err.Error())
	}
	return kubeconfig, nil
}

func main() {
	config, err := getKubeConfig()
	if err != nil {
		panic(err.Error())
	}

	ri, err := NewReleaseInspector(config)
	if err != nil {
		panic(err.Error())
	}
	for release := range ri.Releases() {
		err := ri.Inspect(release)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
		}
	}
}
