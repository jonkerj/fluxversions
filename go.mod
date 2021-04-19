module github.com/jonkerj/fluxversion

go 1.15

require (
	github.com/fluxcd/helm-controller/api v0.8.0
	github.com/fluxcd/source-controller/api v0.9.0
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/mod v0.4.2
	golang.org/x/tools v0.1.0 // indirect
	helm.sh/helm/v3 v3.5.2
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
	sigs.k8s.io/controller-runtime v0.8.3
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309
)
