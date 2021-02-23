package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAppConfigurations1(t *testing.T) {
	parsed, _ := GetAppConfigurations("minio")
	expected := []ParsedConfiguration{
		{
			Key:      "MINIO_ACCESS_KEY",
			Template: "BIZAAR:ALPHANUMERIC(10)",
		},
		{
			Key:      "MINIO_SECRET_KEY",
			Template: "BIZAAR:ALPHANUMERIC(30)",
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
			Template: "BIZAAR:CLUSTER_NAME",
		},
		{
			Key:      "CONTROL_PLANE_ADDRESS",
			Template: "https://BIZAAR:MASTER_IP:6443",
		},
		{
			Key:      "BASIC_AUTH_PASSWORD",
			Template: "BIZAAR:ALPHANUMERIC(10)",
		},
	}

	if !elementsMatch(expected, parsed) {
		t.Errorf("Expected %s but got %s", expected, parsed)
	}
}

func TestExtractBizaarConfigTemplate1(t *testing.T) {
	actual, _ := ExtractBizaarConfigTemplate("BIZAAR:MASTER_IP")
	expected := "BIZAAR:MASTER_IP"

	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestExtractBizaarConfigTemplate2(t *testing.T) {
	actual, _ := ExtractBizaarConfigTemplate("https://BIZAAR:MASTER_IP:6443")
	expected := "BIZAAR:MASTER_IP"

	if expected != actual {
		t.Errorf("Expected %s but got %s", expected, actual)
	}
}

func TestExtractNumFromBizaarConfigTemplate1(t *testing.T) {
	r, _ := ExtractNumFromBizaarConfigTemplate("BIZAAR:ALPHANUMERIC(30)")
	expected := 30
	if expected != r {
		t.Errorf("Expected %d but got %d", expected, r)
	}
}

func TestExtractNumFromBizaarConfigTemplate2(t *testing.T) {
	r, _ := ExtractNumFromBizaarConfigTemplate("xxxBIZAAR:WORDS(20)xxx")
	expected := 20
	if expected != r {
		t.Errorf("Expected %d but got %d", expected, r)
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

func TestGetDepthDependenciesToInstall6(t *testing.T) {
	toInstall := &[]string{}
	appDependencies := []string{"z-app-2"}
	installed := make(map[string]bool)

	_ = GetDepthDependenciesToInstall(toInstall, appDependencies, installed)
	expected := []string{"z-app-2", "z-app-3", "z-app-4"}
	if !elementsMatch(expected, *toInstall) {
		t.Errorf("Expected %s but got %s", expected, *toInstall)
	}
}

func TestGetDepthDependenciesToInstall7(t *testing.T) {
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

func TestGetDepthDependenciesToInstall8(t *testing.T) {
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

func TestGetDepthDependenciesToInstall9(t *testing.T) {
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

// --------------------
// Test helpers
// --------------------

type dummyt struct{}

func (t dummyt) Errorf(string, ...interface{}) {}

func elementsMatch(listA, listB interface{}) bool {
	return assert.ElementsMatch(dummyt{}, listA, listB)
}
