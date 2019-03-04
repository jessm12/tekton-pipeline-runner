#*******************************************************************************
# Licensed Materials - Property of IBM
# "Restricted Materials of IBM"
# 
# Copyright IBM Corp. 2018 All Rights Reserved
#
# US Government Users Restricted Rights - Use, duplication or disclosure
# restricted by GSA ADP Schedule Contract with IBM Corp.
#*******************************************************************************

FROM golang:1.10 as builder
USER root
RUN curl -fsSL -o /usr/local/bin/dep https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 && chmod +x /usr/local/bin/dep
WORKDIR /go/src/github.ibm.com/swiss-cloud/devops-back-end/
COPY . .
#RUN dep ensure  
RUN dep ensure -vendor-only 
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine@sha256:7df6db5aa61ae9480f52f0b3a06a140ab98d427f86d8d5de0bedab9b8df6b1c0
RUN addgroup -g 1000 mcgroup && \
  adduser -G mcgroup -u 1000 -D -S mcuser
USER 1000

WORKDIR /go/src/github.ibm.com/swiss-cloud/devops-back-end/
COPY --from=builder /go/src/github.ibm.com/swiss-cloud/devops-back-end/ .

ENTRYPOINT ["./app"]


