/*******************************************************************************
 * Licensed Materials - Property of IBM
 * "Restricted Materials of IBM"
 *
 * Copyright IBM Corp. 2018 All Rights Reserved
 *
 * US Government Users Restricted Rights - Use, duplication or disclosure
 * restricted by GSA ADP Schedule Contract with IBM Corp.
 *******************************************************************************/

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	pipelinerunner "github.com/a-roberts/knative-pipeline-runner"
	restful "github.com/emicklei/go-restful"
	clientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Resource - cannot extract to utils as would be non local and couldn't add methods
type Resource struct {
	PipelineClient *clientset.TektonV1alpha1Client
	K8sClient      *k8sclientset.Clientset
}

func main() {

	flag.Parse()

	var cfg *rest.Config
	var err error
	kubeconfig := os.Getenv("KUBECONFIG")
	if len(kubeconfig) != 0 {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		fmt.Printf("Error building kubeconfig from %s: %s\n", kubeconfig, err.Error())
	}

	port := ":8080"
	portnumber := os.Getenv("PORT")
	if portnumber != "" {
		port = ":" + portnumber
		fmt.Printf("Port number from config: %s\n", portnumber)
	}

	pipelineClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("Error building pipeline clientset: %s\n", err.Error())
	} else {
		fmt.Println("Got a pipeline client")
	}

	k8sClient, err := k8sclientset.NewForConfig(cfg)
	if err != nil {
		fmt.Printf("Error building k8s clientset: %s\n", err.Error())
	} else {
		fmt.Println("Got a k8s client")
	}

	resource := pipelinerunner.Resource{
		PipelineClient: pipelineClient,
		K8sClient:      k8sClient,
	}

	wsContainer := restful.NewContainer()
	wsContainer.Router(restful.CurlyRouter{})

	resource.RegisterWebhook(wsContainer)

	fmt.Println("Creating server and entering wait loop")
	mux := http.NewServeMux()
	mux.Handle("/", wsContainer)

	server := &http.Server{Addr: port, Handler: mux}

	log.Fatal(server.ListenAndServe())
}
