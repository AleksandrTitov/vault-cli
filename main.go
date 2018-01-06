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
	"text/template"
)

/*
TODO:
 + - Unseal using stdin
 + - Save to file
 - Separate function
 - Function for the connection
 - Connection using proxy
*/

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
	configFile = "vault-cli.conf"
)

const unsealKeyTmpl = `{{ block "list" .}}
{{- range $index, $element := .KeysBase64 }}
Unseal Key {{ inc $index }}: {{ $element -}}
{{end}}
{{"\n"}}Initial Root Token: {{ .RootToken }}
{{ end }}{{"\n"}}`

/*
func getNodeOfService(svcName, fld string) (map[string]string) {

	var Service [] map[string]interface{}

	resp, err := http.Get("http://127.0.0.1:8500/v1/catalog/service/" + svcName)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	respCode := resp.StatusCode
	if respCode != 200 {
		fmt.Println("Status code: " + strconv.Itoa(resp.StatusCode))
	}

	body, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(body, &Service)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	if len(Service) == 0 {
		fmt.Println("Service " + svcName + " not found!")
		return nil
	}

	fmt.Println(Service[0][fld])

	var valueSvcFld map[string] string
	for _, srv := range Service{
		fmt.Println(srv[fld])
		//valueSvcFld[srv["Node"]] = srv["ID"]
	}

	fmt.Println(valueSvcFld)
	return valueSvcFld//Service
}
*/

func getNodeOfService(consulScheme, consulAddr, svcName string) map[string][]string {

	var Service []сonsulServiceResp

	apiUrlService := fmt.Sprintf("%s://%s:/v1/catalog/service/%s", consulScheme, consulAddr, svcName)

	resp, err := http.Get(apiUrlService)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(body, &Service)
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

func getKVValue(consulScheme, consulAddr, keyKV string) string {

	var Value []сonsulKVResp
	var valueKV string

	apiUrlKV := fmt.Sprintf("%s://%s:/v1/kv/%s", consulScheme, consulAddr, keyKV)
	resp, err := http.Get(apiUrlKV)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(body, &Value)
	if err != nil {
		fmt.Println(err)
		//return ""
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

	resp, err := http.Get(apiUrlHealth)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(body, &healthResponse)
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
	apiUrlInit := fmt.Sprintf("%s/v1/sys/init", vaultAddr)

	data := []byte(fmt.Sprintf(`{"secret_shares": %s,"secret_threshold": %s}`, secretShares, secretThreshold))
	payload := bytes.NewReader(data)
	resp, err := http.Post(apiUrlInit, "application/json", payload)
	if err != nil {
		fmt.Println(err)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(body, &vaultInit)
	if err != nil {
		fmt.Println(err)
	}

	return vaultInit
}

func vaultUnsealNode(nodeAddr, unsealKey string) {

	apiUrlUnseal := fmt.Sprintf("%s/v1/sys/unseal", nodeAddr)

	data := []byte(fmt.Sprintf(`{"key": "%s"}`, unsealKey))
	payload := bytes.NewReader(data)
	resp, err := http.Post(apiUrlUnseal, "application/json", payload)
	if err != nil {
		fmt.Println(err)
	}
	ioutil.ReadAll(resp.Body)
}

func vaultBootstrap() {

	var (
		keys       []string
		initStatus bool
		tmplToBuf  bytes.Buffer
	)

	config := readConfig(configFile)
	vaultScheme := getKVValue(config.Consul.Scheme, config.Consul.Addr, config.Vault.Scheme)
	service := getNodeOfService(config.Consul.Scheme, config.Consul.Addr, config.Vault.Name)

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
		for nodeName, urlNode := range service {
			nodeAddr := fmt.Sprintf("%s://%s:%s", vaultScheme, urlNode[0], urlNode[1])

			fmt.Printf("* Unseal node %s: %s\n\n", nodeName, nodeAddr)
			for num, key := range keys {
				sealStatus := getVaultHealth("Sealed", nodeAddr)
				if sealStatus == false {
					fmt.Println("\n* Node unsealed\n")
					break
				} else {
					fmt.Printf("Use key %d: %s\n", num+1, key)
					vaultUnsealNode(nodeAddr, key)
				}
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

	in := bufio.NewReader(os.Stdin)
	keyString, err := in.ReadString('\n')
	keyString = strings.Replace(keyString, "\n", "", -1)
	if err == nil {
		keys = strings.Split(string(keyString), " ")
	}

	config := readConfig(configFile)
	vaultScheme := getKVValue(config.Consul.Scheme, config.Consul.Addr, config.Vault.Scheme)
	service := getNodeOfService(config.Consul.Scheme, config.Consul.Addr, config.Vault.Name)

	for nodeName, urlNode := range service {
		nodeAddr = fmt.Sprintf("%s://%s:%s", vaultScheme, urlNode[0], urlNode[1])
		fmt.Printf("* Unseal node %s: %s\n\n", nodeName, nodeAddr)
		sealStatus = getVaultHealth("Sealed", nodeAddr)
		if sealStatus == false {
			fmt.Printf("* Node %s already unsealed\n\n", nodeName)
		} else {
			for num, key := range keys {
				fmt.Printf("Use key %d: %s\n", num+1, key)
				vaultUnsealNode(nodeAddr, key)
			}
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

	vaultBootstrap()

	//vaultUnsealCluster()

}
