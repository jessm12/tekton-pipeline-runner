# knative-pipeline-runner

Example code to create Knative PipelineRuns and resources (currently just git and image PipelineResources with unique names) dynamically, for example in response to receiving a Github webhook event such as a push.

## Prerequisites

- Docker for Mac - switch to edge version
    - Under advanced set CPU 6, Memory 10, Swap 2.5
    - Under Daemon add insecure registry - `host.docker.internal:5000`
    - Enable Kubernetes
    - A properly set up `GOPATH` (it is advised to use `$HOME/go`). The directory structure is important so that `ko` commands function as expected. Images should be built and made available at your `localhost:5000` Docker registry when `ko` is used. See [Gopath docs](https://github.com/golang/go/wiki/GOPATH) for details.

- An image registry to push your built application image and a registry to push images from `ko apply` commands in setting up Tekton Pipelines and Knative eventing-sources (setting up a local environment is outlined below with a local registry being used but remote registries are supported)

## Install Knative and Istio

`kubectl apply --filename https://github.com/knative/serving/releases/download/v0.4.0/istio-crds.yaml && kubectl apply --filename https://github.com/knative/serving/releases/download/v0.4.0/istio.yaml`

`kubectl label namespace default istio-injection=enabled`

`kubectl get pods --namespace istio-system`: ensure all pods are running and healthy.

```
kubectl apply --filename https://github.com/knative/serving/releases/download/v0.4.0/serving.yaml \
--filename https://github.com/knative/build/releases/download/v0.4.0/build.yaml \
--filename https://github.com/knative/eventing/releases/download/v0.4.0/release.yaml \
--filename https://github.com/knative/eventing-sources/releases/download/v0.4.0/release.yaml \
--filename https://github.com/knative/serving/releases/download/v0.4.0/monitoring.yaml \
--filename https://raw.githubusercontent.com/knative/serving/v0.4.0/third_party/config/build/clusterrole.yaml
```

If an error occurs, run this again (it takes a short time to ensure all CRDs exist).

## Set up your environment if you wish to run this locally

- Set up a local docker registry and optionally a registry viewer:

  ```
  docker run -d --rm -p 5000:5000 --name registry-srv -e REGISTRY_STORAGE_DELETE_ENABLED=true registry:2

  docker run -d --rm -p 8080:8080 --name registry-web --link registry-srv -e REGISTRY_URL=http://registry-srv:5000/v2 -e REGISTRY_NAME=localhost:5000 hyper/docker-registry-web
  ```

`export KO_DOCKER_REPO=localhost:5000/knative`

At your `$GOPATH/src/github.com/knative directory`: `git clone https://github.com/knative/eventing-sources.git`
`cd eventing-sources`
`ko apply -f config`

This has been tested with `master` of the 6th of March 2019.

At your `$GOPATH/src/github.com/knative` directory: `git clone https://github.com/knative/build-pipeline.git`
`cd build-pipeline`
`git checkout 218af43e5aba4f0cee4a934297cfd68c2b797c33`
`ko apply -f config`

## Custom domain setup

`kubectl edit cm config-domain --namespace knative-serving`

Above `kind: ConfigMap`, add the following with only *two* spaces (so NOT in line with what's under `example`):

`YOURIP.nip.io: |`

`YOURIP` is determined from `ifconfig | grep "inet 9." | cut -d ' ' -f2`

## Build and push the event handler

`docker build -t <your public Dockerhub username>/github-event-handler .`
`docker push <your public Dockerhub username>/github-event-handler`

## Apply the Knative Tasks and Pipeline

`kubectl apply -f config/`

## Run it:

- Modify `event-handler/github-event-handler.yml` to refer to your event handler image on Dockerhub.
- Additionally, modify the `DOCKER_REGISTRY_LOCATION` value in this file to specify your remote registry that the Docker image for your application will be pushed to.
- Modify `github_source_templates/git-repo.yml` replacing your owner and repository and access tokens. Ensure you also specify the correct git provider. 

- `kubectl apply -f event-handler/github-event-handler.yml`
- `kubectl apply -f github_source_templates/git-repo.yml`
- Verify a webhook was created successfully on your repository, then push code to your repository. 

A PipelineRun will be created for you, as will the PipelineResources that reference the Git commit ID of your newly committed code and image coordinates. 

Observe as your code is checked out, built and pushed to your remote registry.

## Pushing to a remote registry

Create Docker secrets for both pushing and pulling and patch the default service account to use them.

To create a push secret, modify `push-secret.yaml` to include your own Docker registry credentials, then run `kubectl apply -f` on the `secrets/` directory

To create a pull secret, modify and run the below command, again including your own Docker registry credentials:

`kubectl create secret docker-registry docker-pull --docker-server=<docker-registry> --docker-username=<username> --docker-password=<password>`

Patch the service account to use these secrets by running the below commands: 

`kubectl patch sa default --type=json -p="[{\"op\":\"add\",\"path\":\"/imagePullSecrets/0\",\"value\":{\"name\": \"docker-pull\"}}]"`

`kubectl patch sa default --type=json -p="[{\"op\":\"add\",\"path\":\"/secrets/0\",\"value\":{\"name\": \"docker-push\"}}]"`