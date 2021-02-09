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

// GetAppConfigurations ...
func GetAppConfigurations(appName string) ([]ParsedConfiguration, error) {
	parsedConfigs := []ParsedConfiguration{}
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/kubernetes-marketplace/%s/%s/manifest.yaml", marketplaceAccount, marketplaceBranch, appName)
	res, err := http.Get(url)
	if err != nil {
		return parsedConfigs, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return parsedConfigs, err
	}

	manifest := &AppManifest{}
	err = yaml.Unmarshal(body, &manifest)
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
