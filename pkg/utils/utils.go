package utils

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/brianvoe/gofakeit"
	"github.com/dlclark/regexp2"
	"gopkg.in/yaml.v2"
)

const (
	marketplaceAccount = "zulh-civo"
	marketplaceBranch  = "b"
)

// AppManifest is the original structure of app's manifest.yaml file
type AppManifest struct {
	Version      string   `yaml:"version"`
	Namespace    string   `yaml:"namespace"`
	Dependencies []string `yaml:"dependencies"`
	Plans        []struct {
		Label         string `yaml:"label"`
		Configuration map[string]struct {
			Value string `yaml:"value"`
		} `yaml:"configuration"`
	} `yaml:"plans"`
	Configuration map[string]struct {
		Label string `yaml:"label"`
		Value string `yaml:"value"`
	} `yaml:"configuration"`
}

// ParsedConfiguration is the structure of app's configuration.
// In AppManifest, the configuration key e.g. "MINIO_ACCESS_KEY" is the YAML key.
// That makes our live difficult because Go can't really work with dynamic keys in YAML, JSON & etc.
// So, we need to put the key inside a struct like below to make our work easier.
type ParsedConfiguration struct {
	Key      string // e.g. "CONTROL_PLANE_ADDRESS"
	Template string // e.g. "https://BIZAAR:MASTER_IP:6443"
}

// GetAppManifest ...
func GetAppManifest(appName string) (*AppManifest, error) {
	manifest := &AppManifest{}
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/kubernetes-marketplace/%s/%s/manifest.yaml", marketplaceAccount, marketplaceBranch, appName)
	res, err := http.Get(url)
	if err != nil {
		return manifest, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return manifest, err
	}

	err = yaml.Unmarshal(body, &manifest)
	if err != nil {
		return manifest, err
	}

	return manifest, nil
}

// GetAppVersion ...
func GetAppVersion(appName string) (string, error) {
	manifest, err := GetAppManifest(appName)
	if err != nil {
		return "", err
	}

	version := manifest.Version
	if version == "" {
		return "", fmt.Errorf("Version is empty")
	}

	return version, nil
}

// GetAppConfigurations ...
func GetAppConfigurations(appName string) ([]ParsedConfiguration, error) {
	parsedConfigs := []ParsedConfiguration{}
	manifest, err := GetAppManifest(appName)
	if err != nil {
		return parsedConfigs, err
	}

	conf := manifest.Configuration
	keys := reflect.ValueOf(conf).MapKeys()
	strKeys := make([]string, len(keys))
	for i := 0; i < len(keys); i++ {
		strKeys[i] = keys[i].String()
	}

	for _, key := range strKeys {
		parsedConfigs = append(parsedConfigs, ParsedConfiguration{
			Key:      key,
			Template: conf[key].Value,
		})
	}

	return parsedConfigs, nil
}

// GetSeed takes app and uid, hash it and returns the hash in int64 format
func GetSeed(app, uid string) int64 {
	h := md5.New()
	sd := fmt.Sprintf("%s-%s", app, uid)
	io.WriteString(h, sd)
	seed := binary.BigEndian.Uint64(h.Sum(nil))
	return int64(seed)
}

// GenerateRandomAlphanumeric ...
func GenerateRandomAlphanumeric(n int, app, uid string) (string, error) {
	letters := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]byte, n)

	seed := GetSeed(app, uid)
	rand.Seed(int64(seed))

	for i := range b {
		max := len(letters)
		index := rand.Intn(max)
		b[i] = letters[index]
	}

	return string(b), nil

}

// GenerateRandomWords ...
func GenerateRandomWords(n int, app, uid string) string {
	seed := GetSeed(app, uid)
	gofakeit.Seed(seed)
	words := []string{}

	for i := 0; i < n; i++ {
		w := gofakeit.HipsterWord()
		words = append(words, w)
	}

	return strings.Join(words, " ")
}

// ExtractNumFromBizaarConfigTemplate takes Bizaar config template e.g. BIZAAR:ALPHANUMERIC(30)
// or BIZAAR:WORDS(30) and return 30 (int)
func ExtractNumFromBizaarConfigTemplate(template string) (int, error) {
	r, err := regexp2.Compile(`(?<=\()\d+(?=\))`, 0)
	if err != nil {
		return 0, err
	}

	match, err := r.FindStringMatch(template)
	if err != nil {
		return 0, err
	}

	numStr := match.String()
	numInt, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}

	return numInt, nil

}

// ExtractBizaarConfigTemplate takes Bizaar config template e.g. "https://BIZAAR:MASTER_IP:6443"
// and returns just "BIZAAR:MASTER_IP". Examples: https://rubular.com/r/egFowm7E0S4Esg.
func ExtractBizaarConfigTemplate(template string) (string, error) {
	r, err := regexp.Compile(`BIZAAR:[A-Z_0-9()]+`)
	if err != nil {
		return "", err
	}

	matched := r.FindString(template)
	if matched == "" {
		return "", fmt.Errorf("Matched template is empty")
	}

	return matched, nil
}

// GetBase64String takes input and returns base64 encoded version
func GetBase64String(input string) string {
	return base64.URLEncoding.EncodeToString([]byte(input))
}

// GetAppPlanVariableName ...
func GetAppPlanVariableName(appName string) (string, error) {
	manifest, err := GetAppManifest(appName)
	if err != nil {
		return "", err
	}

	planVariableNames := []string{}
	for _, plan := range manifest.Plans {
		conf := plan.Configuration
		keys := reflect.ValueOf(conf).MapKeys()
		for i := 0; i < len(keys); i++ {
			planVariableNames = append(planVariableNames, keys[i].String())
		}
	}

	// fmt.Printf("Plan variable names for %s: %+v\n", appName, planVariableNames)
	return planVariableNames[0], nil
}

// SanitizeDependencyName ...
// https://rubular.com/r/5ibwrOnew3vKpf
func SanitizeDependencyName(lowerCasedInput string) (string, error) {
	emptyStr := ""
	r, err := regexp.Compile(`^[a-z-0-9]*`)
	if err != nil {
		return emptyStr, err
	}

	cleaned := r.FindString(lowerCasedInput)
	if cleaned == emptyStr {
		return emptyStr, fmt.Errorf("Dependency name is empty")
	}

	return cleaned, nil
}

// GetDepthDependenciesToInstall will modify toInstall will ALL dependencies needed
// to install an app. For example, let's say we are install Joomla which depends on Longhorn, MariaDB and Cert Manager.
// MariaDB depends on Longhorn. Cert Manager depends on Helm.
// If we have already installed Longhorn in the cluster, then toInstall will give us MariaDB, Cert Manager and Helm.
func GetDepthDependenciesToInstall(toInstall *[]string, dependencies []string, installed map[string]bool) error {
	for _, dependency := range dependencies {
		deps, err := GetAppDependencies(dependency)
		if err != nil {
			return err
		}

		_, isInstalled := installed[dependency]
		if !isInstalled && !IsStrSliceContains(toInstall, dependency) {
			*toInstall = append(*toInstall, dependency)
		}

		_ = GetDepthDependenciesToInstall(toInstall, deps, installed)
	}

	return nil
}

// IsStrSliceContains will return true if element is found in the slc
func IsStrSliceContains(slc *[]string, element string) bool {
	for _, s := range *slc {
		if s == element {
			return true
		}
	}

	return false
}

// GetAppDependencies ...
func GetAppDependencies(appName string) ([]string, error) {
	deps := []string{}
	manifest, err := GetAppManifest(appName)
	if err != nil {
		return deps, err
	}

	for _, dep := range manifest.Dependencies {
		s := strings.ToLower(dep)
		d, err := SanitizeDependencyName(s)
		if err != nil {
			fmt.Printf("Skipping %s from being added to dependencies list due to an error - %v\n", dep, err)
			continue
		}
		deps = append(deps, d)
	}

	return deps, nil
}

// ExtractPlanIntFromPlanStr takes plan string i.e. "5Gi" and return 5 (int).
// If something goes wrong, it will return -1 (int).
func ExtractPlanIntFromPlanStr(input string) (output int) {
	r, err := regexp.Compile(`[0-9]+`)
	if err != nil {
		return -1
	}

	str := r.FindString(input)
	if str == "" {
		return -1
	}

	output, err = strconv.Atoi(str)
	if err != nil {
		return -1
	}

	return output
}

// GetAppPlans returns sorted app plans e.g. [5,10,20]
func GetAppPlans(appName string) ([]int, error) {
	plans := []int{}
	manifest, err := GetAppManifest(appName)
	if err != nil {
		return plans, err
	}

	for _, plan := range manifest.Plans {
		conf := plan.Configuration
		keys := reflect.ValueOf(conf).MapKeys()
		strKeys := make([]string, len(keys))
		for i := 0; i < len(keys); i++ {
			strKeys[i] = keys[i].String()
		}

		for _, key := range strKeys {
			p := ExtractPlanIntFromPlanStr(conf[key].Value)
			if p > 0 {
				plans = append(plans, p)
			}
		}
	}

	sort.Ints(plans)
	return plans, nil
}

// GetSmallestAppPlan take plans slice e.g. [20,5,10] and return 5 (int)
func GetSmallestAppPlan(sortedPlans []int) int {
	return sortedPlans[0]
}

// GetNamespaceFromAppManifest ...
func GetNamespaceFromAppManifest(appName string) (string, error) {
	manifest, err := GetAppManifest(appName)
	if err != nil {
		return "", err
	}

	ns := manifest.Namespace
	return ns, nil
}

// ContainsString ...
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// RemoveString ...
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
