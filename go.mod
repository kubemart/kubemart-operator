module github.com/kubemart/kubemart-operator

go 1.16

require (
	github.com/brianvoe/gofakeit v3.18.0+incompatible
	github.com/dlclark/regexp2 v1.4.0
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/stretchr/testify v1.7.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.3
)

// replace github.com/kubemart/kubemart-operator/pkg/utils => /Users/zulh/civo/bizaar-operator/pkg/utils
