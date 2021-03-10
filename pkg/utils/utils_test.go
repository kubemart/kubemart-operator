package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAppManifest(t *testing.T) {
	_, err := GetAppManifest("rabbitmq")
	if err != nil {
		t.Errorf("Expected nil error but got %s instead", err)
	}
}

func TestGetAppVersion(t *testing.T) {
	_, err := GetAppVersion("rabbitmq")
	if err != nil {
		t.Errorf("Expected nil error but got %s instead", err)
	}
}

func TestGetAppConfigurations1(t *testing.T) {
	parsed, _ := GetAppConfigurations("minio")
	expected := []ParsedConfiguration{
		{
			Key:      "MINIO_ACCESS_KEY",
			Template: "KUBEMART:ALPHANUMERIC(10)",
		},
		{
			Key:      "MINIO_SECRET_KEY",
			Template: "KUBEMART:ALPHANUMERIC(30)",
		},
	}

	if !elementsMatch(expected, parsed) {
		t.Errorf("Expected %s but got %s", expected, parsed)
	}

}

func TestGetAppConfigurations2(t *testing.T) {
	parsed, _ := GetAppConfigurations("permission-manager")
	expected := []ParsedConfiguration{
		{
			Key:      "CLUSTER_NAME",
			Template: "KUBEMART:CLUSTER_NAME",
		},
		{
			Key:      "CONTROL_PLANE_ADDRESS",
			Template: "https://KUBEMART:MASTER_IP:6443",
		},
		{
			Key:      "BASIC_AUTH_PASSWORD",
			Template: "KUBEMART:ALPHANUMERIC(10)",
		},
	}

	if !elementsMatch(expected, parsed) {
		t.Errorf("Expected %s but got %s", expected, parsed)
	}
}

func TestGetSeed(t *testing.T) {
	actual := GetSeed("rabbitmq", "5b8ef190-387f-470b-a174-2eb229d76cfa")
	expected := int64(-830595325964329316)
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestGenerateRandomAlphanumeric(t *testing.T) {
	for i := 0; i < 10; i++ {
		s, _ := GenerateRandomAlphanumeric(10, "test-app", "2d4a24cb-ec08-4838-bde8-9467555e6d42")
		expected := "EKGLWrDivJ"
		if expected != s {
			t.Errorf("Expected %s but got %s", expected, s)
		}
	}
}

func TestGenerateRandomWords(t *testing.T) {
	actual := GenerateRandomWords(5, "test-app", "2d4a24cb-ec08-4838-bde8-9467555e6d42")
	expected := "tilde meh lomo umami craft beer"
	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestExtractNumFromKubemartConfigTemplate1(t *testing.T) {
	r, _ := ExtractNumFromKubemartConfigTemplate("KUBEMART:ALPHANUMERIC(30)")
	expected := 30
	if expected != r {
		t.Errorf("Expected %d but got %d", expected, r)
	}
}

func TestExtractNumFromKubemartConfigTemplate2(t *testing.T) {
	r, _ := ExtractNumFromKubemartConfigTemplate("xxxKUBEMART:WORDS(20)xxx")
	expected := 20
	if expected != r {
		t.Errorf("Expected %d but got %d", expected, r)
	}
}

func TestExtractKubemartConfigTemplate1(t *testing.T) {
	actual, _ := ExtractKubemartConfigTemplate("KUBEMART:MASTER_IP")
	expected := "KUBEMART:MASTER_IP"

	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestExtractKubemartConfigTemplate2(t *testing.T) {
	actual, _ := ExtractKubemartConfigTemplate("https://KUBEMART:MASTER_IP:6443")
	expected := "KUBEMART:MASTER_IP"

	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestGetBase64String(t *testing.T) {
	encoded := GetBase64String("hello")
	expected := "aGVsbG8="
	if expected != encoded {
		t.Errorf("Expected %s but got %s", expected, encoded)
	}
}

func TestGetAppPlanVariableName1(t *testing.T) {
	actual, _ := GetAppPlanVariableName("mariadb")
	expected := "VOLUME_SIZE"
	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestGetAppPlanVariableName2(t *testing.T) {
	actual, _ := GetAppPlanVariableName("minio")
	expected := "PV_SIZE_GB"
	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestSanitizeDependencyName1(t *testing.T) {
	input := "mariadb"
	actual, _ := SanitizeDependencyName(input)
	expected := input
	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestSanitizeDependencyName2(t *testing.T) {
	input := "the-app-3"
	actual, _ := SanitizeDependencyName(input)
	expected := input
	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestSanitizeDependencyName3(t *testing.T) {
	input := "the-app3"
	actual, _ := SanitizeDependencyName(input)
	expected := input
	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestGetDepthDependenciesToInstall1(t *testing.T) {
	toInstall := &[]string{}
	joomlaDependencies := []string{"longhorn", "mariadb", "cert-manager"}
	installedApps := make(map[string]bool)

	_ = GetDepthDependenciesToInstall(toInstall, joomlaDependencies, installedApps)
	expected := []string{"longhorn", "mariadb", "cert-manager"}
	if !elementsMatch(expected, *toInstall) {
		t.Errorf("Expected %s but got %s", expected, *toInstall)
	}
}

func TestGetDepthDependenciesToInstall2(t *testing.T) {
	toInstall := &[]string{}
	joomlaDependencies := []string{"longhorn", "mariadb", "cert-manager"}
	installed := make(map[string]bool)
	installed["longhorn"] = true

	_ = GetDepthDependenciesToInstall(toInstall, joomlaDependencies, installed)
	expected := []string{"mariadb", "cert-manager"}
	if !elementsMatch(expected, *toInstall) {
		t.Errorf("Expected %s but got %s", expected, *toInstall)
	}
}

func TestGetDepthDependenciesToInstall3(t *testing.T) {
	toInstall := &[]string{}
	joomlaDependencies := []string{"longhorn", "mariadb", "cert-manager"}
	installed := make(map[string]bool)
	installed["longhorn"] = true
	installed["mariadb"] = true

	_ = GetDepthDependenciesToInstall(toInstall, joomlaDependencies, installed)
	expected := []string{"cert-manager"}
	if !elementsMatch(expected, *toInstall) {
		t.Errorf("Expected %s but got %s", expected, *toInstall)
	}
}

func TestGetDepthDependenciesToInstall4(t *testing.T) {
	toInstall := &[]string{}
	joomlaDependencies := []string{"longhorn", "mariadb", "cert-manager"}
	installed := make(map[string]bool)
	installed["longhorn"] = true
	installed["mariadb"] = true
	installed["cert-manager"] = true

	_ = GetDepthDependenciesToInstall(toInstall, joomlaDependencies, installed)
	expected := []string{}
	if !elementsMatch(expected, *toInstall) {
		t.Errorf("Expected %s but got %s", expected, *toInstall)
	}
}

func TestGetDepthDependenciesToInstall5(t *testing.T) {
	toInstall := &[]string{}
	appDependencies := []string{"z-app-2"}
	installed := make(map[string]bool)

	_ = GetDepthDependenciesToInstall(toInstall, appDependencies, installed)
	expected := []string{"z-app-2", "z-app-3", "z-app-4"}
	if !elementsMatch(expected, *toInstall) {
		t.Errorf("Expected %s but got %s", expected, *toInstall)
	}
}

func TestGetDepthDependenciesToInstall6(t *testing.T) {
	toInstall := &[]string{}
	appDependencies := []string{"z-app-2"}
	installed := make(map[string]bool)
	installed["z-app-2"] = true

	_ = GetDepthDependenciesToInstall(toInstall, appDependencies, installed)
	expected := []string{"z-app-3", "z-app-4"}
	if !elementsMatch(expected, *toInstall) {
		t.Errorf("Expected %s but got %s", expected, *toInstall)
	}
}

func TestGetDepthDependenciesToInstall7(t *testing.T) {
	toInstall := &[]string{}
	appDependencies := []string{"z-app-2"}
	installed := make(map[string]bool)
	installed["z-app-2"] = true
	installed["z-app-3"] = true

	_ = GetDepthDependenciesToInstall(toInstall, appDependencies, installed)
	expected := []string{"z-app-4"}
	if !elementsMatch(expected, *toInstall) {
		t.Errorf("Expected %s but got %s", expected, *toInstall)
	}
}

func TestGetDepthDependenciesToInstall8(t *testing.T) {
	toInstall := &[]string{}
	appDependencies := []string{"z-app-2"}
	installed := make(map[string]bool)
	installed["z-app-2"] = true
	installed["z-app-3"] = true
	installed["z-app-4"] = true

	_ = GetDepthDependenciesToInstall(toInstall, appDependencies, installed)
	expected := []string{}
	if !elementsMatch(expected, *toInstall) {
		t.Errorf("Expected %s but got %s", expected, *toInstall)
	}
}

func TestIsStrSliceContains(t *testing.T) {
	staffs := []string{"Zulh", "Saiyam"}
	actual := IsStrSliceContains(&staffs, "Zulh")
	expected := true
	if expected != actual {
		t.Errorf("Expected %t but actual is %t", expected, actual)
	}
}

func TestGetAppDependencies1(t *testing.T) {
	actual, _ := GetAppDependencies("mariadb")
	expected := []string{"longhorn"}
	if !elementsMatch(expected, actual) {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestGetAppDependencies2(t *testing.T) {
	actual, _ := GetAppDependencies("longhorn")
	expected := []string{}
	if !elementsMatch(expected, actual) {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestExtractPlanIntFromPlanStr1(t *testing.T) {
	actual := ExtractPlanIntFromPlanStr("5Gi")
	expected := 5
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestExtractPlanIntFromPlanStr2(t *testing.T) {
	actual := ExtractPlanIntFromPlanStr("5Gib")
	expected := 5
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestExtractPlanIntFromPlanStr3(t *testing.T) {
	actual := ExtractPlanIntFromPlanStr("5GB")
	expected := 5
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestExtractPlanIntFromPlanStr4(t *testing.T) {
	actual := ExtractPlanIntFromPlanStr("5 Gi")
	expected := 5
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestExtractPlanIntFromPlanStr5(t *testing.T) {
	actual := ExtractPlanIntFromPlanStr("5 Gib")
	expected := 5
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestExtractPlanIntFromPlanStr6(t *testing.T) {
	actual := ExtractPlanIntFromPlanStr("5 GB")
	expected := 5
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestExtractPlanIntFromPlanStr7(t *testing.T) {
	actual := ExtractPlanIntFromPlanStr("five")
	expected := -1
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestExtractPlanIntFromPlanStr8(t *testing.T) {
	actual := ExtractPlanIntFromPlanStr("fiveGi")
	expected := -1
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestExtractPlanIntFromPlanStr9(t *testing.T) {
	actual := ExtractPlanIntFromPlanStr("five Gi")
	expected := -1
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestGetAppPlans1(t *testing.T) {
	actual, _ := GetAppPlans("mariadb")
	expected := []int{5, 10, 20}
	if !elementsMatch(expected, actual) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestGetAppPlans2(t *testing.T) {
	actual, _ := GetAppPlans("cert-manager")
	expected := []int{}
	if !elementsMatch(expected, actual) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

func TestGetSmallestAppPlan(t *testing.T) {
	plans := []int{5, 10, 20}
	actual := GetSmallestAppPlan(plans)
	expected := 5
	if expected != actual {
		t.Errorf("Expected %d but actual is %d", expected, actual)
	}
}

func TestGetNamespaceFromAppManifest(t *testing.T) {
	actual, _ := GetNamespaceFromAppManifest("rabbitmq")
	expected := "rabbitmq"
	if expected != actual {
		t.Errorf("Expected %s but actual is %s", expected, actual)
	}
}

func TestContainsString(t *testing.T) {
	finalizers := []string{"finalizers.kubemart.civo.com", "finalizers.something.else.com"}
	actual := ContainsString(finalizers, "finalizers.kubemart.civo.com")
	expected := true
	if expected != actual {
		t.Errorf("Expected %t but actual is %t", expected, actual)
	}
}

func TestRemoveString(t *testing.T) {
	finalizers := []string{"finalizers.kubemart.civo.com", "finalizers.something.else.com"}
	actual := RemoveString(finalizers, "finalizers.kubemart.civo.com")
	expected := []string{"finalizers.something.else.com"}
	if !elementsMatch(expected, actual) {
		t.Errorf("Expected %v but got %v", expected, actual)
	}
}

// --------------------
// Test helpers
// --------------------

type dummyt struct{}

func (t dummyt) Errorf(string, ...interface{}) {}

func elementsMatch(listA, listB interface{}) bool {
	return assert.ElementsMatch(dummyt{}, listA, listB)
}
