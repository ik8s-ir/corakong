package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/Kong/go-pdk"
	"github.com/Kong/go-pdk/server"
	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const PluginName = "corakong"

var Version = "0.0.1"
var Priority = 1

type Config struct {
	Namespace string `json:"namespace"`
	Gateway   string `json:"gateway"`
}

func New() interface{} {
	return &Config{}
}

var (
	dynamicClient *dynamic.DynamicClient
	once          sync.Once
	gvr           = schema.GroupVersionResource{
		Group:    "waf.ik8s.ir",
		Version:  "v1alpha1",
		Resource: "wafrules",
	}
)

func main() {
	once.Do(func() {
		config, err := createConfig()
		if err != nil {
			log.Fatalf("Error creating config: %v", err)
		}

		dynamicClient, err = dynamic.NewForConfig(config)
		if err != nil {
			log.Fatalf("Error creating dynamic client: %v\n", err)
		}
	})

	err := server.StartServer(New, Version, Priority)
	if err != nil {
		log.Fatalf("Failed start %s plugin", PluginName)
	}
}

func (conf Config) Access(kong *pdk.PDK) {
	options := v1.ListOptions{
		LabelSelector: "enabled=true,gateway=" + conf.Gateway,
	}

	items, err := dynamicClient.Resource(gvr).Namespace(conf.Namespace).List(context.Background(), options)
	if err != nil {
		kong.Log.Debug(`Error getting CRD:`, err)
	}

	directives := ""
	for _, item := range items.Items {
		obj := item.Object
		spec, ok := obj["spec"].(map[string]interface{})
		if !ok {
			continue
		}
		rule, ok := spec["rule"].(string)
		if !ok {
			continue
		}
		directives += rule + "\n"
	}

	waf := createWaf(directives)
	tx := waf.NewTransaction()
	defer func() {
		tx.ProcessLogging()
		tx.Close()
	}()
	p, err1 := processRequest(tx, kong)
	if err1 != nil {
		kong.Response.Exit(403, []byte("Error in WAF, check your security rules."), nil)
	}
	if p != nil {
		kong.Response.Exit(403, []byte("Restricted by WAF"), nil)
	}
}

func createWaf(rules string) coraza.WAF {
	waf, err := coraza.NewWAF(coraza.NewWAFConfig().WithDirectives(rules))
	if err != nil {
		log.Fatal(err)
	}
	return waf
}

func processRequest(tx types.Transaction, kong *pdk.PDK) (*types.Interruption, error) {

	client, _ := kong.Client.GetIp()
	cport, _ := kong.Client.GetPort()

	tx.ProcessConnection(client, cport, "", 0)

	scheme, _ := kong.Request.GetScheme()
	host, _ := kong.Request.GetHeader("host")
	path, _ := kong.Request.GetPathWithQuery()
	url := fmt.Sprintf("%s://%s%s", scheme, host, path)
	method, _ := kong.Request.GetMethod()
	httpver, _ := kong.Request.GetHttpVersion()
	tx.ProcessURI(url, method, fmt.Sprintf("%f", httpver))
	headers, _ := kong.Request.GetHeaders(-1)

	for k, vr := range headers {
		for _, v := range vr {
			tx.AddRequestHeader(k, v)
		}
	}

	if host != "" {
		tx.AddRequestHeader("Host", host)
		tx.SetServerName(host)
	}

	in := tx.ProcessRequestHeaders()
	if in != nil {
		fmt.Printf("Transaction was interrupted with status %d\n", in.Status)
		return in, nil
	}

	return tx.ProcessRequestBody()
}

func createConfig() (*rest.Config, error) {
	configFile := "/home/kong/.kube/config"
	_, err := os.Stat(configFile)
	if err != nil {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", configFile)
}
