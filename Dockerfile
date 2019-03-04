FROM golang:1.10 as builder
USER root
RUN curl -fsSL -o /usr/local/bin/dep https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 && chmod +x /usr/local/bin/dep
WORKDIR /go/src/github.com/a-roberts/knative-pipeline-runner/
COPY . . 
RUN dep ensure -vendor-only 
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine@sha256:7df6db5aa61ae9480f52f0b3a06a140ab98d427f86d8d5de0bedab9b8df6b1c0
RUN addgroup -g 1000 kgroup && \
  adduser -G kgroup -u 1000 -D -S kuser
USER 1000

WORKDIR /go/src/github.com/a-roberts/knative-pipeline-runner/
COPY --from=builder /go/src/github.com/a-roberts/knative-pipeline-runner/ .

ENTRYPOINT ["./app"]


