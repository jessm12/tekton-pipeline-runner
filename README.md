# knative-pipeline-runner

Example code to create Knative pipelines and resources (currently just a git and image resource with unique names) dynamically, for example in response to receiving a Github webhook event such as a push.

## Usage

### Defining the Knative resources
- `kubectl apply -f config`: this creates a new pipeline and associated steps that will be run

### Building the event handler
- `docker build -t <docker username>/knative-pipeline-runner:latest` .
- `docker push <docker username>/knative-pipeline-runner:latest`

### Using the event handler
- Replace `github-event-handler.yml` contents to point to your image

### Setting up a webhook
- Modify `git-repo.yml`, referencing your own secrets and repository to create a webhook for
- `kubectl apply -f git-repo.yml`
- Check the webhook was created successfully

### Run the event handler
- `kubectl apply -f github-event-handler.yml`
- Push code to your repository

### Watch it all run
- Observe as a PipelineRun is created along with associated PipelineResources
- Observe as your application is built, run and deployed

