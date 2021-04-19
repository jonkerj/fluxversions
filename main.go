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
	"time"

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

func (ri *ReleaseInspector) inspectRelease(release *helmv2.HelmRelease) (error){
	fmt.Printf("Found release: %s/%s (chart %s %s)\n", release.Namespace, release.Name, release.Spec.Chart.Spec.Chart, release.Spec.Chart.Spec.Version)
	sr := release.Spec.Chart.Spec.SourceRef

	if sr.Kind != "HelmRepository" {
		return errors.New("Release does not originate from HelmRepository")
	}
	nameKey := types.NamespacedName{
		Name: sr.Name,
		Namespace: sr.Namespace,
	}
	var hr sourcev1.HelmRepository
	if err := ri.kubeClient.Get(context.Background(), nameKey, &hr); err != nil {
		return err
	}

	url := hr.GetArtifact().URL

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("Received a non 200 response code")
	}

	tmpFile, err := ioutil.TempFile("/", "index.*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return err
	}
	err = tmpFile.Close()
	if err != nil {
		return err
	}

	idx, err := repo.LoadIndexFile(tmpFile.Name())
	if err != nil {
		return err
	}
	currentVer := release.Spec.Chart.Spec.Version
	if ! strings.HasPrefix(currentVer, "v") {
		currentVer = "v" + currentVer
	}
	for _, entries := range idx.Entries {
		for _, entry := range entries {
			if entry.Name == release.Spec.Chart.Spec.Chart {
				repoVer := entry.Version
				if ! strings.HasPrefix(repoVer, "v") {
					repoVer = "v" + repoVer
				}

				if semver.Compare(currentVer, repoVer) < 0 {
					fmt.Printf("found newer: %s\n", repoVer)
				}
			}
		}
	}

	return nil
}

func (ri *ReleaseInspector) releases() <-chan *helmv2.HelmRelease {
	ch := make(chan *helmv2.HelmRelease);
	go func () {
		var list helmv2.HelmReleaseList
		if err := ri.kubeClient.List(context.Background(), &list); err == nil {
			for _, release := range list.Items {
				ch <- &release
			}
		} // TODO: log errors!
		close(ch)
	} ();
	return ch
}

func main() {
	var kubeconfigPath *string

	// ok, let's find out if there is a kubeconfig file. Default to "", which causes BuildConfigFromFlags to use in-cluster config
	defaultKubeconfigPath := ""
	if home := homedir.HomeDir(); home != "" {
		defaultKubeconfigPath = filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(defaultKubeconfigPath); err != nil {
			if os.IsNotExist(err) {
				defaultKubeconfigPath = ""
			}
		}
	}

	kubeconfigPath = flag.String("kubeconfig", defaultKubeconfigPath, "(optional) absolute path to the kubeconfig file")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}

	ri, err := NewReleaseInspector(config)
	if err != nil {
		panic(err.Error())
	}

	for {
		for release := range ri.releases() {
			err := ri.inspectRelease(release)
			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
			}
		}
		time.Sleep(time.Second * 10)
	}
}
