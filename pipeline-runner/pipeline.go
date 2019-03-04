/*******************************************************************************
 * Licensed Materials - Property of IBM
 * "Restricted Materials of IBM"
 *
 * Copyright IBM Corp. 2018 All Rights Reserved
 *
 * US Government Users Restricted Rights - Use, duplication or disclosure
 * restricted by GSA ADP Schedule Contract with IBM Corp.
 *******************************************************************************/

package endpoints

import (
	"fmt"
	"strconv"
	"time"

	restful "github.com/emicklei/go-restful"
	v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclientset "k8s.io/client-go/kubernetes"

	"strings"

	gh "gopkg.in/go-playground/webhooks.v3/github"
)

//Resource - cannot extract to utils as would be non local and couldnt add methods
type Resource struct {
	PipelineClient *clientset.TektonV1alpha1Client
	K8sClient      *k8sclientset.Clientset
}

//BuildInformation - information required to build a particular commit from a Git repository.
type BuildInformation struct {
	REPOURL   string
	SHORTID   string
	COMMITID  string
	REPONAME  string
	TIMESTAMP string
}

//BuildRequest - a manual submission data struct
type BuildRequest struct {
	/* Example payload
	{
	  "repourl": "https://github.ibm.com/your-org/test-project",
	  "commitid": "7d84981c66718ee2dda1af280f915cc2feb6ffow",
	  "reponame": "test-project"
	}
	*/
	REPOURL  string `json:"repourl"`
	COMMITID string `json:"commitid"`
	REPONAME string `json:"reponame"`
	BRANCH   string `json:"branch"`
	// todo do we need PATH for this as well now?
}

func (r Resource) RegisterWebhook(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path("/").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.POST("/").To(r.HandleWebhook))
	container.Add(ws)
}

func (r Resource) handleManualBuildRequest(request *restful.Request, response *restful.Response) {
	requestData := BuildRequest{}

	if err := request.ReadEntity(&requestData); err != nil {
		fmt.Printf("An error occurred decoding the manual build request body: %s", err)
		return
	}

	id := ""
	shortid := ""
	if requestData.COMMITID != "" {
		id = requestData.COMMITID
		shortid = requestData.COMMITID[0:7]
	} else {
		id = requestData.BRANCH
		shortid = "latest"
	}

	timestamp := getDateTimeAsString()
	buildInformation := BuildInformation{}
	buildInformation.REPOURL = requestData.REPOURL
	buildInformation.SHORTID = shortid
	buildInformation.COMMITID = id
	buildInformation.REPONAME = requestData.REPONAME
	buildInformation.TIMESTAMP = timestamp

	fmt.Printf("Handling manual build request, build information: \n %s", buildInformation)
	submitBuild(buildInformation, r)
}

func definePipelineResource(name, namespace string, params []v1alpha1.Param, resourceType v1alpha1.PipelineResourceType) *v1alpha1.PipelineResource {
	pipelineResource := v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.PipelineResourceSpec{
			Type:   resourceType,
			Params: params,
		},
	}
	resourcePointer := &pipelineResource
	return resourcePointer
}

// Todo add endpoint for creating pipelineruns: give it some json: name of the pipeline, timeout, perhaps even pipeline params ;)

// Todo make the timeout a param and do *desired timeout in seconds* * time.Second
// Todo pass in a big ol' struct instead
func definePipelineRun(pipelineRunName, namespace, saName string,
	pipeline v1alpha1.Pipeline,
	triggerType v1alpha1.PipelineTriggerType,
	resourceBinding []v1alpha1.PipelineResourceBinding) *v1alpha1.PipelineRun {

	// Todo test this is deployed ok, accept more params (resources) and use
	startTime := time.Now()

	pipelineRunData := v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineRunName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "devops-knative",
			},
		},

		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{Name: pipeline.Name},
			// E.g. v1alpha1.PipelineTriggerTypeManual
			Trigger:        v1alpha1.PipelineTrigger{Type: triggerType},
			ServiceAccount: saName,
			Timeout:        &metav1.Duration{Duration: 1 * time.Hour},
			Resources:      resourceBinding,
		},

		// We think the flow here is: Ready -> Running -> can be cancelled by sending an update: PipelineRunCancelled
		Status: v1alpha1.PipelineRunStatus{
			Conditions: []duckv1alpha1.Condition{{Type: duckv1alpha1.ConditionReady}},
			StartTime:  &metav1.Time{Time: startTime},
		},
	}
	pipelineRunPointer := &pipelineRunData
	return pipelineRunPointer
}

func getDateTimeAsString() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// Caller needs to handle error and not pipeline ref being valid
func (r Resource) getPipelineImpl(name, namespace string) (v1alpha1.Pipeline, error) {
	fmt.Printf("In getPipelineImpl, name %s, namespace %s \n", name, namespace)

	pipelines := r.PipelineClient.Pipelines(namespace)
	pipeline, err := pipelines.Get(name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Couldn't retrieve the pipeline called %s in namespace %s", name, namespace)
		return *pipeline, err
	} else {
		fmt.Println("Found the pipeline definition OK")
	}
	return *pipeline, nil
}

func submitBuild(buildInformation BuildInformation, r Resource) {
	fmt.Println("In submitBuild")
	namespaceToUse := "default"
	saName := "default"

	startTime := time.Now().Unix()

	// Assumes you've already applied the yaml and the pipeline
	generatedPipelineRunName := fmt.Sprintf("devops-pipeline-run-%d", startTime)

	pipelineTemplateName := "simple-pipeline"
	pipelineNs := "default"

	// As these are dynamically created and unique, we need to modify the pipeline resource spec names to use them
	// We can't create resources that already exist (even if the fields differ, we need unique names)
	imageResourceName := fmt.Sprintf("docker-image-%d", startTime)
	gitResourceName := fmt.Sprintf("git-source-%d", startTime)

	pipeline, err := r.getPipelineImpl(pipelineTemplateName, pipelineNs)
	if err != nil {
		fmt.Printf("Couldn't find the pipeline template %s in namespace %s \n", pipelineTemplateName, pipelineNs)
		return
	} else {
		fmt.Printf("Found the pipeline template %s OK \n", pipelineTemplateName)
	}

	// We don't want to modify an existing pipeline as this may be in use
	// We actually want to create a new pipeline that uses the new resources

	fmt.Println("Creating resources next...")

	urlToUse := fmt.Sprintf("host.docker.internal:5000/knative/%s:%s", buildInformation.REPONAME, buildInformation.SHORTID)

	paramsForImageResource := []v1alpha1.Param{{Name: "url", Value: urlToUse}}
	pipelineImageResource := definePipelineResource(imageResourceName, pipelineNs, paramsForImageResource, "image")
	createdPipelineImageResource, err := r.PipelineClient.PipelineResources(pipelineNs).Create(pipelineImageResource)
	if err != nil {
		fmt.Printf("Could not create pipeline image resource to be used in the pipeline, error: %s", err)
	} else {
		fmt.Printf("Created pipeline image resource %s successfully \n", createdPipelineImageResource.Name)
	}

	paramsForGitResource := []v1alpha1.Param{{Name: "revision", Value: buildInformation.COMMITID}, {Name: "url", Value: buildInformation.REPOURL}}
	pipelineGitResource := definePipelineResource(gitResourceName, pipelineNs, paramsForGitResource, "git")
	createdPipelineGitResource, err := r.PipelineClient.PipelineResources(pipelineNs).Create(pipelineGitResource)

	if err != nil {
		fmt.Printf("Could not create pipeline git resource to be used in the pipeline, error: %s \n", err)
	} else {
		fmt.Printf("Created pipeline git resource %s successfully \n", createdPipelineGitResource.Name)
	}

	gitResourceRef := v1alpha1.PipelineResourceRef{Name: gitResourceName}
	imageResourceRef := v1alpha1.PipelineResourceRef{Name: imageResourceName}

	//resources := []v1alpha1.PipelineResourceBinding{{Name: imageResourceName, ResourceRef: imageResourceRef}, {Name: gitResourceName, ResourceRef: gitResourceRef}}
	resources := []v1alpha1.PipelineResourceBinding{{Name: "docker-image", ResourceRef: imageResourceRef}, {Name: "git-source", ResourceRef: gitResourceRef}}

	// PipelineRun yaml defines references to resources
	pipelineRunData := definePipelineRun(generatedPipelineRunName, namespaceToUse, saName, pipeline, v1alpha1.PipelineTriggerTypeManual, resources)

	fmt.Printf("Creating a new PipelineRun named %s \n", generatedPipelineRunName)

	pipelineRun, err := r.PipelineClient.PipelineRuns(pipelineNs).Create(pipelineRunData)
	if err != nil {
		fmt.Printf("Error creating the PipelineRun: %s \n", err)
	} else {
		fmt.Printf("PipelineRun created: %s \n", pipelineRun)
	}
}

// HandleWebhook does some fun stuff when we get an event
// Todo provide proper responses e.g. 503, server errors, 200 if good
func (r Resource) HandleWebhook(request *restful.Request, response *restful.Response) {
	fmt.Println("In handleWebhook code with error handling for github event")
	buildInformation := BuildInformation{}

	gitHubEventType := request.HeaderParameter("Ce-X-Github-Event")

	if len(gitHubEventType) < 1 {
		fmt.Printf("Found header for the event type from Github is incompatible: we require it to at least contain slashes. It is: %s \n", gitHubEventType)
		return
	}

	gitHubEventTypeString := strings.Replace(gitHubEventType, "\"", "", -1)

	fmt.Printf("GitHub event type is %s \n", gitHubEventTypeString)

	timestamp := getDateTimeAsString()

	if gitHubEventTypeString == "push" {
		fmt.Println("Handling push event...")

		webhookData := gh.PushPayload{}
		if err := request.ReadEntity(&webhookData); err != nil {
			fmt.Printf("An error occurred decoding webhook data: %s", err)
			return
		}

		buildInformation.REPOURL = webhookData.Repository.URL
		buildInformation.SHORTID = webhookData.HeadCommit.ID[0:7]
		buildInformation.COMMITID = webhookData.HeadCommit.ID
		buildInformation.REPONAME = webhookData.Repository.Name
		buildInformation.TIMESTAMP = timestamp

		submitBuild(buildInformation, r)
		fmt.Printf("Build information for repository %s:%s \n %s \n", buildInformation.REPOURL, buildInformation.SHORTID, buildInformation)

	} else if gitHubEventTypeString == "pull_request" {
		fmt.Println("Handling pull request event...")

		webhookData := gh.PullRequestPayload{}

		if err := request.ReadEntity(&webhookData); err != nil {
			fmt.Printf("An error occurred decoding webhook data: %s", err)
			return
		}

		buildInformation.REPOURL = webhookData.Repository.HTMLURL
		buildInformation.SHORTID = webhookData.PullRequest.Head.Sha[0:7]
		buildInformation.COMMITID = webhookData.PullRequest.Head.Sha
		buildInformation.REPONAME = webhookData.Repository.Name
		buildInformation.TIMESTAMP = timestamp

		submitBuild(buildInformation, r)
		fmt.Printf("Build information for repository %s:%s \n %s \n", buildInformation.REPOURL, buildInformation.SHORTID, buildInformation)

	} else {
		fmt.Println("Event wasn't a push or pull event, no action will be taken")
	}
}
