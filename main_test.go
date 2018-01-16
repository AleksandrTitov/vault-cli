package main

import (
	"testing"
	"encoding/json"
	"fmt"
	"net/http"
	"log"
	"reflect"
	"strconv"
)

func TestReadConfig(t *testing.T)  {

	var testConfig = Config {}

	testConfig.Vault.Scheme = "service/dc1/vault/config/scheme"
	testConfig.Vault.Name = "vault"
	testConfig.Init.Save = true
	testConfig.Init.Shares = "5"
	testConfig.Init.Threshold = "3"
	testConfig.Consul.Addr = "127.0.0.1:8500"
	testConfig.Consul.Scheme = "http"

	if readConfig(configFile) != testConfig {
		t.Error("Error of func 'readConfig'")
	}
}

const (
	// const of server for emulation
	httpServerScheme = "http"
	httpServerAddr = "127.0.0.1"
	httpServerPort = "8085"
)

const (
	//const for testing func getNodeOfService
	node = "vault-test-node"
	addr = "127.0.0.1"
	serviceAddr = "127.0.0.1"
	servicePort = 8666
	createIndex = 00000
)

func (resp *сonsulServiceResp) setConsulServiceResp() {
	resp.Node = node
	resp.Address = addr
	resp.ServiceAddress = serviceAddr
	resp.ServicePort = servicePort
	resp.CreateIndex = createIndex
}

func handlerConsulService(w http.ResponseWriter, r *http.Request) {

	var resp = сonsulServiceResp{}
	resp.setConsulServiceResp()
	respSl := []сonsulServiceResp{}
	respSl = append(respSl, resp)
	b,_ := json.Marshal(respSl)

	fmt.Fprint(w, string(b))
}

func startHttpServer(urlPath ...string) *http.Server {

	srv := &http.Server{Addr: ":" + httpServerPort}

	for _, path := range urlPath {
		http.HandleFunc(path, handlerConsulService)
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Http server: ListenAndServe() error: %s", err)
		}
	}()

	return srv
}

func TestGetNodeOfService(t *testing.T)  {

	var (
		nodeTest = map[string][]string{
			node:{serviceAddr, strconv.Itoa(servicePort)},
		}
		node map[string][]string
	)

	srv := startHttpServer("/v1/catalog/service/vault")
	node = getNodeOfService(httpServerScheme, httpServerAddr + ":" + httpServerPort, "", "vault")

	if reflect.DeepEqual(node,nodeTest) != true {
		t.Error("Error of func 'getNodeOfService'")
	}

	srv.Shutdown(nil)
}