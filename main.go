package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"gopkg.in/gcfg.v1"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
)

type сonsulServiceResp struct {
	ID              string
	Node            string
	Address         string
	Datacenter      string
	TaggedAddresses struct {
		Lan string
		Wan string
	}
	NodeMeta struct {
	}
	ServiceID                string
	ServiceName              string
	ServiceTags              []string
	ServiceAddress           string
	ServicePort              int
	ServiceEnableTagOverride bool
	CreateIndex              int
	ModifyIndex              int
}

type сonsulKVResp struct {
	LockIndex   int
	Key         string
	Flags       int
	Value       string
	CreateIndex int
	ModifyIndex int
}

type vaultHealthResp struct {
	ClusterID     string
	ClusterName   string
	Version       string
	ServerTimeUtc int
	Standby       bool
	Sealed        bool
	Initialized   bool
}

type vaultInitResp struct {
	Keys       []string `json:"keys"`
	KeysBase64 []string `json:"keys_base64"`
	RootToken  string   `json:"root_token"`
}

type Config struct {
	Vault struct {
		Scheme string
		Name   string
	}
	Init struct {
		Save      bool
		Shares    string
		Threshold string
	}
	Consul struct {
		Addr   string
		Scheme string
	}
}

const (
	configFile     = "vault-cli.conf"
	consulTokenEnv = "CONSUL_HTTP_TOKEN"
)

func respHTTP(url, methodReq string, metadataHTTP map[string]string, dataHTTP []byte) []byte {

	client := &http.Client{}
	httpReq, _ := http.NewRequest(methodReq, url, bytes.NewBuffer(dataHTTP))
	for key, value := range metadataHTTP {
		httpReq.Header.Add(key, value)
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		fmt.Println(err)
	}

	body, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		fmt.Println(err)
	}
	defer httpResp.Body.Close()

	return body
}

func getNodeOfService(consulScheme, consulAddr, consulToken, svcName string) map[string][]string {

	var Service []сonsulServiceResp
	var metadataHTTP = map[string]string{
		"X-Consul-Token": consulToken,
	}

	apiUrlService := fmt.Sprintf("%s://%s:/v1/catalog/service/%s", consulScheme, consulAddr, svcName)

	body := respHTTP(apiUrlService, "GET", metadataHTTP, nil)
	err := json.Unmarshal(body, &Service)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	var valueSvc = make(map[string][]string)
	for _, srv := range Service {
		valueSvc[string(srv.CreateIndex)] = []string{srv.Address, strconv.Itoa(srv.ServicePort)}
		//valueSvc[string(srv.Node)] = []string{ srv.Address, strconv.Itoa(srv.ServicePort) }
	}

	return valueSvc
}

func getKVValue(consulScheme, consulAddr, consulToken, keyKV string) string {

	var Value []сonsulKVResp
	var valueKV string
	var metadataHTTP = map[string]string{
		"X-Consul-Token": consulToken,
	}

	apiUrlKV := fmt.Sprintf("%s://%s:/v1/kv/%s", consulScheme, consulAddr, keyKV)
	body := respHTTP(apiUrlKV, "GET", metadataHTTP, nil)

	err := json.Unmarshal(body, &Value)
	if err != nil {
		fmt.Println(err)
	}

	for _, srv := range Value {
		valueKVByte, _ := base64.StdEncoding.DecodeString(srv.Value)
		valueKV = string(valueKVByte)
	}

	return valueKV
}

func getVaultHealth(healthKey, vaultAddr string) bool {

	var healthResponse vaultHealthResp

	apiUrlHealth := fmt.Sprintf("%s/v1/sys/health", vaultAddr)
	body := respHTTP(apiUrlHealth, "GET", nil, nil)

	err := json.Unmarshal(body, &healthResponse)
	if err != nil {
		fmt.Println(err)
	}

	switch healthKey {
	case "Sealed":
		status := healthResponse.Sealed
		return status
	case "Initialized":
		status := healthResponse.Initialized
		return status
	default:
		return false
	}
}

func vaultInit(vaultAddr, secretShares, secretThreshold string) vaultInitResp {

	var vaultInit vaultInitResp
	var metadataHTTP = map[string]string{
		"Content-Type": "application/json",
	}

	apiUrlInit := fmt.Sprintf("%s/v1/sys/init", vaultAddr)
	data := []byte(fmt.Sprintf(`{"secret_shares": %s,"secret_threshold": %s}`, secretShares, secretThreshold))

	body := respHTTP(apiUrlInit, "POST", metadataHTTP, data)
	err := json.Unmarshal(body, &vaultInit)
	if err != nil {
		fmt.Println(err)
	}

	return vaultInit
}

func vaultUnsealNode(nodeAddr, unsealKey string, wg *sync.WaitGroup) {

	var metadataHTTP = map[string]string{
		"Content-Type": "application/json",
	}
	defer wg.Done()

	data := []byte(fmt.Sprintf(`{"key": "%s"}`, unsealKey))
	apiUrlUnseal := fmt.Sprintf("%s/v1/sys/unseal", nodeAddr)

	respHTTP(apiUrlUnseal, "POST", metadataHTTP, data)
}

func vaultBootstrap() {

	var (
		keys       []string
		initStatus bool
		tmplToBuf  bytes.Buffer
	)

	const unsealKeyTmpl = `{{ block "list" .}}` +
		`{{- range $index, $element := .KeysBase64 }}` +
		`Unseal Key {{ inc $index }}: {{ $element -}}{{"\n"}}` +
		`{{ end }}` +
		`{{"\n"}}Initial Root Token: {{ .RootToken }}{{"\n\n"}}` +
		`{{ end }}`

	consulToken := os.Getenv(consulTokenEnv)
	config := readConfig(configFile)
	vaultScheme := getKVValue(config.Consul.Scheme, config.Consul.Addr, consulToken, config.Vault.Scheme)
	service := getNodeOfService(config.Consul.Scheme, config.Consul.Addr, consulToken, config.Vault.Name)

	for _, urlNode := range service {
		nodeAddr := fmt.Sprintf("%s://%s:%s", vaultScheme, urlNode[0], urlNode[1])
		initStatus = getVaultHealth("Initialized", nodeAddr)
		if initStatus == true {
			fmt.Println("* Vault is already initialized")
		} else {
			resp := vaultInit(nodeAddr, config.Init.Shares, config.Init.Threshold)
			keys = resp.KeysBase64

			funcMap := template.FuncMap{
				"inc": func(i int) int {
					return i + 1
				},
			}

			tpl := template.Must(template.New("tmpl").Funcs(funcMap).Parse(unsealKeyTmpl))
			tpl.Execute(os.Stdout, resp)

			if config.Init.Save == true {
				tpl.Execute(&tmplToBuf, resp)
				ioutil.WriteFile("vault-keys", []byte(tmplToBuf.String()), 0600)
			}
		}
		break
	}

	if initStatus == false {

		threshold, _ := strconv.Atoi(config.Init.Threshold)

		for nodeName, urlNode := range service {
			nodeAddr := fmt.Sprintf("%s://%s:%s", vaultScheme, urlNode[0], urlNode[1])

			fmt.Printf("* Unseal node %s: %s\n\n", nodeName, nodeAddr)

			wg := &sync.WaitGroup{}

			for num, key := range keys[:threshold] {
				fmt.Printf("Use key %d: %s\n", num+1, key)
				wg.Add(1)
				go vaultUnsealNode(nodeAddr, key, wg)
			}
			wg.Wait()
			sealStatus := getVaultHealth("Sealed", nodeAddr)
			if sealStatus == false {
				fmt.Println("\n* Node successful unsealed\n")
			} else {
				fmt.Println("\n* Node didn't unsealed\n")
			}
		}
	}
}

func vaultUnsealCluster() {

	var (
		keys       []string
		sealStatus bool
		nodeAddr   string
	)

	consulToken := os.Getenv(consulTokenEnv)
	config := readConfig(configFile)

	in := bufio.NewReader(os.Stdin)
	keyString, err := in.ReadString('\n')
	keyString = strings.Replace(keyString, "\n", "", -1)
	if err == nil {
		keys = strings.Split(string(keyString), " ")
	}

	if strconv.Itoa(len(keys)) != config.Init.Threshold {
		panic(fmt.Sprintf("The number of 'unseal key' should be equal %s, try again.", config.Init.Threshold))
	}
	vaultScheme := getKVValue(config.Consul.Scheme, config.Consul.Addr, consulToken, config.Vault.Scheme)
	service := getNodeOfService(config.Consul.Scheme, config.Consul.Addr, consulToken, config.Vault.Name)

	for nodeName, urlNode := range service {
		nodeAddr = fmt.Sprintf("%s://%s:%s", vaultScheme, urlNode[0], urlNode[1])
		fmt.Printf("* Unseal node %s: %s\n\n", nodeName, nodeAddr)
		sealStatus = getVaultHealth("Sealed", nodeAddr)
		if sealStatus == false {
			fmt.Printf("* Node %s already unsealed\n\n", nodeName)
		} else {
			wg := &sync.WaitGroup{}

			for num, key := range keys {
				fmt.Printf("Use key %d: %s\n", num+1, key)
				wg.Add(1)
				go vaultUnsealNode(nodeAddr, key, wg)
			}
			wg.Wait()
			sealStatus = getVaultHealth("Sealed", nodeAddr)
			if sealStatus == false {
				fmt.Println("\n* Node successful unsealed\n")
			} else {
				fmt.Println("\n* Node didn't unsealed\n")
			}
		}
	}
}

func readConfig(configFile string) (cfg Config) {

	err := gcfg.ReadFileInto(&cfg, configFile)
	if err != nil {
		fmt.Println(err)
	}
	if cfg.Vault.Scheme == "default" {
		cfg.Vault.Scheme = "http"
	}
	if cfg.Consul.Scheme == "default" {
		cfg.Consul.Scheme = "http"
	}
	if cfg.Consul.Addr == "default" {
		cfg.Consul.Addr = "127.0.0.1:8500"
	}

	return cfg
}

func main() {

	helpMessage := "Usage: vault-cli <command>\n\nCommon commands:\n" +
		"* bootstrap\t Bootstrap Vault cluster\n" +
		"* unseal\t Unseal vault cluster"
	consulTokenErrMessage := fmt.Sprintf("* Variable '%s' is not set.", consulTokenEnv)

	if len(os.Args) == 2 {
		if os.Args[1] == "bootstrap" || os.Args[1] == "b" {
			if os.Getenv(consulTokenEnv) == "" {
				fmt.Println(consulTokenErrMessage)
			} else {
				vaultBootstrap()
			}
		} else if os.Args[1] == "unseal" || os.Args[1] == "u" {
			if os.Getenv(consulTokenEnv) == "" {
				fmt.Println(consulTokenErrMessage)
			} else {
				vaultUnsealCluster()
			}
		} else {
			fmt.Println(helpMessage)
		}
	} else {
		fmt.Println(helpMessage)
	}
}
