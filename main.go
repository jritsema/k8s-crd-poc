package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

//SchemeGroupVersion Create a Rest client with the new CRD Schema
var SchemeGroupVersion = schema.GroupVersion{Group: CRDGroup, Version: CRDVersion}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ShipmentEnvironment{},
		&ShipmentEnvironmentList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

func newClient(cfg *rest.Config) (*rest.RESTClient, *runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	SchemeBuilder := runtime.NewSchemeBuilder(addKnownTypes)
	if err := SchemeBuilder.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}
	config := *cfg
	config.GroupVersion = &SchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{
		CodecFactory: serializer.NewCodecFactory(scheme)}

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, nil, err
	}
	return client, scheme, nil
}

var client *kubernetes.Clientset
var customClient *ShipmentEnvironmentClient
var shipmentEnvironmentScheme *runtime.Scheme
var config *rest.Config

//GetClientConfig returns a rest config, if path not specified assume in cluster config
func GetClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func main() {

	kubeconf := flag.String("kubeconf", "", "Path to a kube config. Only required if out-of-cluster.")
	flag.Parse()

	config, err := GetClientConfig(*kubeconf)
	if err != nil {
		panic(err.Error())
	}

	//create a new clientset which include our CRD schema
	restClient, scheme, err := newClient(config)
	if err != nil {
		panic(err)
	}
	shipmentEnvironmentScheme = scheme

	//default namespace
	ns := os.Getenv("NAMESPACE")
	if len(ns) == 0 {
		ns = "default"
	}

	//create a custom client
	customClient = getShipmentEnvironmentClient(restClient, ns)

	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	//list all ShipmentEnvironment objects
	items, err := customClient.List()
	if err != nil {
		panic(err)
	}
	fmt.Printf("found %v Shipment Environments\n", len(items.Items))

	// Watch for changes in ShipmentEnvironment objects and fire Add, Delete, Update callbacks
	_, controller := cache.NewInformer(
		customClient.NewListWatch(),
		&ShipmentEnvironment{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    addShipmentEnv,
			UpdateFunc: updateShipmentEnv,
			DeleteFunc: deleteShipmentEnv,
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)

	// Wait forever
	select {}
}

func int32Ptr(i int32) *int32 { return &i }
