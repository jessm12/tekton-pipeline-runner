# knative-pipeline-runner

Example code to create Knative pipelines and resources (currently just a git and image resource with unique names) dynamically, for example in response to receiving a Github webhook event such as a push.

## Usage

### Defining the Knative resources
- `kubectl apply -f config`: this creates a new pipeline and associated steps that will be run

### Building the event handler
- `docker build -t <docker username>/knative-pipeline-runner:latest` .
- `docker push <docker username>/knative-pipeline-runner:latest`

### Using the event handler
- Replace `event-handler/github-event-handler.yml` contents to point to your image

### Run the event handler
- `kubectl apply -f event-handler/github-event-handler.yml`

### Setting up a webhook
- Modify `github_source_templates/git-repo.yaml`, referencing your own secrets and repository to create a webhook for
- `kubectl apply -f github_source_templates/git-repo.yaml`
- Check the webhook was created successfully

### Watch it all run
- Push code to your repository
- Observe as a PipelineRun is created along with associated PipelineResources
- Observe as your application is built, run and deployed

