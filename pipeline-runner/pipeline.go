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

	"os"

	logging "github.com/a-roberts/knative-pipeline-runner/logging"
	restful "github.com/emicklei/go-restful"
	v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	clientset "github.com/knative/build-pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
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
}

// RegisterWebhook ...
func (r Resource) RegisterWebhook(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path("/").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.POST("/").To(r.HandleWebhook))
	container.Add(ws)
}

/* Get all pipelines in a given namespace: the caller needs to handle any errors */
func (r Resource) getPipeline(name, namespace string) (v1alpha1.Pipeline, error) {
	logging.Log.Debugf("in getPipeline, name %s, namespace %s \n", name, namespace)

	pipelines := r.PipelineClient.Pipelines(namespace)
	pipeline, err := pipelines.Get(name, metav1.GetOptions{})
	if err != nil {
		logging.Log.Errorf("could not retrieve the pipeline called %s in namespace %s", name, namespace)
		return *pipeline, err
	} else {
		logging.Log.Debugf("Found the pipeline definition OK")
	}
	return *pipeline, nil
}

/* Create a new PipelineResource: this should be of type git or image */
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

/* Create a new PipelineResource: this should be of type git or image */
func definePipelineRun(pipelineRunName, namespace, saName string,
	pipeline v1alpha1.Pipeline,
	triggerType v1alpha1.PipelineTriggerType,
	resourceBinding []v1alpha1.PipelineResourceBinding) *v1alpha1.PipelineRun {

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
	}
	pipelineRunPointer := &pipelineRunData
	return pipelineRunPointer
}

func getDateTimeAsString() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

/* This is the main flow that handles building and deploying: given everything we need to kick off a build, do so */
func createPipelineRunFromWebhookData(buildInformation BuildInformation, r Resource) {
	logging.Log.Debugf("In createPipelineRunFromWebhookData, build information: %s", buildInformation)
	// Todo should these be parameterised? Non-default namespace support and then RBAC/roles/bindings etc
	namespaceToUse := "default"
	saName := "default"

	startTime := getDateTimeAsString()

	// Assumes you've already applied the yaml: so the pipeline definition and its tasks exist
	generatedPipelineRunName := fmt.Sprintf("devops-pipeline-run-%s", startTime)

	pipelineTemplateName := "simple-pipeline"
	pipelineNs := "default"

	// Unique names
	imageResourceName := fmt.Sprintf("docker-image-%s", startTime)
	gitResourceName := fmt.Sprintf("git-source-%s", startTime)

	pipeline, err := r.getPipeline(pipelineTemplateName, pipelineNs)
	if err != nil {
		logging.Log.Errorf("could not find the pipeline template %s in namespace %s", pipelineTemplateName, pipelineNs)
		return
	} else {
		logging.Log.Debugf("Found the pipeline template %s OK", pipelineTemplateName)
	}

	logging.Log.Debug("Creating PipelineResources next...")

	// This is building and pushing to the local registry only - this would be useful to expose as a parameter
	// so we can push to a different image registry. Should it be an env var we pass through in the pod definition?

	registryURL := os.Getenv("DOCKER_REGISTRY_LOCATION")
	urlToUse := fmt.Sprintf("%s/%s:%s", registryURL, buildInformation.REPONAME, buildInformation.SHORTID)
	logging.Log.Infof("Pushing the image to %s", urlToUse)

	paramsForImageResource := []v1alpha1.Param{{Name: "url", Value: urlToUse}}
	pipelineImageResource := definePipelineResource(imageResourceName, pipelineNs, paramsForImageResource, "image")
	createdPipelineImageResource, err := r.PipelineClient.PipelineResources(pipelineNs).Create(pipelineImageResource)
	if err != nil {
		logging.Log.Errorf("could not create pipeline image resource to be used in the pipeline, error: %s", err)
	} else {
		logging.Log.Infof("Created pipeline image resource %s successfully", createdPipelineImageResource.Name)
	}

	paramsForGitResource := []v1alpha1.Param{{Name: "revision", Value: buildInformation.COMMITID}, {Name: "url", Value: buildInformation.REPOURL}}
	pipelineGitResource := definePipelineResource(gitResourceName, pipelineNs, paramsForGitResource, "git")
	createdPipelineGitResource, err := r.PipelineClient.PipelineResources(pipelineNs).Create(pipelineGitResource)

	if err != nil {
		logging.Log.Errorf("could not create pipeline git resource to be used in the pipeline, error: %s", err)
	} else {
		logging.Log.Infof("Created pipeline git resource %s successfully", createdPipelineGitResource.Name)
	}

	gitResourceRef := v1alpha1.PipelineResourceRef{Name: gitResourceName}
	imageResourceRef := v1alpha1.PipelineResourceRef{Name: imageResourceName}

	resources := []v1alpha1.PipelineResourceBinding{{Name: "docker-image", ResourceRef: imageResourceRef}, {Name: "git-source", ResourceRef: gitResourceRef}}

	// PipelineRun yaml defines references to resources
	pipelineRunData := definePipelineRun(generatedPipelineRunName, namespaceToUse, saName, pipeline, v1alpha1.PipelineTriggerTypeManual, resources)

	logging.Log.Infof("Creating a new PipelineRun named %s", generatedPipelineRunName)

	pipelineRun, err := r.PipelineClient.PipelineRuns(pipelineNs).Create(pipelineRunData)
	if err != nil {
		logging.Log.Errorf("error creating the PipelineRun: %s", err)
	} else {
		logging.Log.Infof("PipelineRun created: %s", pipelineRun)
	}
}

// HandleWebhook should be called when we hit the / endpoint with webhook data. Todo provide proper responses e.g. 503, server errors, 200 if good
func (r Resource) HandleWebhook(request *restful.Request, response *restful.Response) {
	logging.Log.Debugf("In HandleWebhook code with error handling for a GitHub event")
	buildInformation := BuildInformation{}

	githubEvent := "Ce-Github-Event"
	gitHubEventType := request.HeaderParameter(githubEvent)

	if len(gitHubEventType) < 1 {
		logging.Log.Errorf("found header (%s) exists but has no value! \n Request is: %s", githubEvent, request)
		return
	}

	gitHubEventTypeString := strings.Replace(gitHubEventType, "\"", "", -1)

	logging.Log.Debugf("GitHub event type is %s \n", gitHubEventTypeString)

	timestamp := getDateTimeAsString()

	if gitHubEventTypeString == "push" {
		logging.Log.Debugf("Handling a push event...")

		webhookData := gh.PushPayload{}
		if err := request.ReadEntity(&webhookData); err != nil {
			logging.Log.Errorf("an error occurred decoding webhook data: %s", err)
			return
		}

		buildInformation.REPOURL = webhookData.Repository.URL
		buildInformation.SHORTID = webhookData.HeadCommit.ID[0:7]
		buildInformation.COMMITID = webhookData.HeadCommit.ID
		buildInformation.REPONAME = webhookData.Repository.Name
		buildInformation.TIMESTAMP = timestamp

		createPipelineRunFromWebhookData(buildInformation, r)
		logging.Log.Debugf("Build information for repository %s:%s \n %s", buildInformation.REPOURL, buildInformation.SHORTID, buildInformation)

	} else if gitHubEventTypeString == "pull_request" {
		logging.Log.Debugf("Handling a pull request event...")

		webhookData := gh.PullRequestPayload{}

		if err := request.ReadEntity(&webhookData); err != nil {
			logging.Log.Errorf("an error occurred decoding webhook data: %s", err)
			return
		}

		buildInformation.REPOURL = webhookData.Repository.HTMLURL
		buildInformation.SHORTID = webhookData.PullRequest.Head.Sha[0:7]
		buildInformation.COMMITID = webhookData.PullRequest.Head.Sha
		buildInformation.REPONAME = webhookData.Repository.Name
		buildInformation.TIMESTAMP = timestamp

		createPipelineRunFromWebhookData(buildInformation, r)
		logging.Log.Debugf("Build information for repository %s:%s \n %s \n", buildInformation.REPOURL, buildInformation.SHORTID, buildInformation)

	} else {
		logging.Log.Errorf("event wasn't a push or pull event, no action will be taken")
	}

}
