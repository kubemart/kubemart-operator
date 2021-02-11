module github.com/civo/bizaar-operator

go 1.13

require (
	github.com/brianvoe/gofakeit v3.18.0+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/dlclark/regexp2 v1.4.0
	github.com/go-logr/logr v0.1.0
	github.com/go-loremipsum/loremipsum v1.1.0
	github.com/mitchellh/copystructure v1.1.1 // indirect
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/prometheus/common v0.4.1
	github.com/stretchr/testify v1.7.0
	gopkg.in/yaml.v2 v2.3.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.3
)

// TODO - remove this
replace github.com/civo/bizaar-operator/pkg/utils => /Users/zulh/civo/bizaar-operator/pkg/utils
