package logging

import (
	logging "github.com/knative/pkg/test/logging"
)

var loggerName = "knative-pipeline-runner"

// Use this
var Log *logging.BaseLogger

func init() {
	logging.InitializeLogger(false)
	Log = logging.GetContextLogger(loggerName)
}
