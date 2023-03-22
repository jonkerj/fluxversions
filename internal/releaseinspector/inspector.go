package releaseinspector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"golang.org/x/mod/semver"

	"helm.sh/helm/v3/pkg/repo"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	helmv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
)

var ignorePrerelease = []string{"rc", "alpha", "beta", "snapshot"}

func New(kubeConfig *restclient.Config) (*ReleaseInspector, error) {
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
		Name:      sr.Name,
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

func (ri *ReleaseInspector) Inspect(release helmv2.HelmRelease) error {
	currentVer := release.Spec.Chart.Spec.Version
	if !strings.HasPrefix(currentVer, "v") {
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
	ENTRIES:
		for _, entry := range entries {
			if entry.Name == release.Spec.Chart.Spec.Chart {
				repoVer := entry.Version
				if !strings.HasPrefix(repoVer, "v") {
					repoVer = "v" + repoVer
				}
				for _, keyword := range ignorePrerelease {
					if strings.Contains(semver.Prerelease(repoVer), keyword) {
						continue ENTRIES
					}
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
	ch := make(chan helmv2.HelmRelease)
	go func() {
		var list helmv2.HelmReleaseList
		if err := ri.kubeClient.List(context.Background(), &list); err == nil {
			for _, release := range list.Items {
				ch <- release
			}
		} // TODO: log errors!
		close(ch)
	}()
	return ch
}
