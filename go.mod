module github.com/jonkerj/fluxversion

go 1.15

require (
	golang.org/x/mod v0.4.2
	github.com/fluxcd/helm-controller/api v0.8.0
	github.com/fluxcd/source-controller/api v0.9.0
	helm.sh/helm/v3 v3.5.2
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
	sigs.k8s.io/controller-runtime v0.8.3
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309
)
