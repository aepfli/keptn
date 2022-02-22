package go_tests

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/keptn/go-utils/pkg/api/models"
	"github.com/keptn/go-utils/pkg/common/strutils"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	"github.com/stretchr/testify/require"
)

const commitIDShipyard = `--- 
apiVersion: spec.keptn.sh/0.2.0
kind: Shipyard
metadata: 
  name: shipyard-quality-gates
spec: 
  stages: 
    - 
      name: hardening`

const gitCommitIDSLO = `---
spec_version: "0.1.1"
comparison:
  aggregate_function: "avg"
  compare_with: "single_result"
  include_result_with_score: "pass"
  number_of_comparison_results: 1
filter:
objectives:
  - sli: "response_time_p95"
    key_sli: false
    pass:             # pass if (relative change <= 75% AND absolute value is < 75ms)
      - criteria:
          - "<=+75%"  # relative values require a prefixed sign (plus or minus)
          - "<800"     # absolute values only require a logical operator
    warning:          # if the response time is below 200ms, the result should be a warning
      - criteria:
          - "<=1000"
          - "<=+100%"
    weight: 1
  - sli: "throughput"
    pass:
      - criteria:
          - "<=+100%"
          - ">=-80%"
  - sli: "error_rate"
total_score:
  pass: "90%"
  warning: "75%"`

func Test_EvaluationGitCommitID(t *testing.T) {
	projectName := "commit-id-evaluation"
	serviceName := "my-service"
	shipyardFilePath, err := CreateTmpShipyardFile(commitIDShipyard)
	require.Nil(t, err)
	defer os.Remove(shipyardFilePath)

	t.Log("deleting lighthouse configmap from previous test run")
	_, _ = ExecuteCommand(fmt.Sprintf("kubectl delete configmap -n %s lighthouse-config-keptn-%s", GetKeptnNameSpaceFromEnv(), projectName))

	t.Logf("creating project %s", projectName)
	projectName, err = CreateProject(projectName, shipyardFilePath, true)
	require.Nil(t, err)

	t.Logf("creating service %s", serviceName)
	output, err := ExecuteCommand(fmt.Sprintf("keptn create service %s --project=%s", serviceName, projectName))

	t.Log("Testing the evaluation...")

	require.Nil(t, err)
	require.Contains(t, output, "created successfully")

	t.Logf("adding an SLI provider")
	_, err = ExecuteCommand(fmt.Sprintf("kubectl create configmap -n %s lighthouse-config-%s --from-literal=sli-provider=my-sli-provider", GetKeptnNameSpaceFromEnv(), projectName))
	require.Nil(t, err)

	var evaluationFinishedEvent *models.KeptnContextExtendedCE
	evaluationFinishedPayload := &keptnv2.EvaluationFinishedEventData{}

	//first part

	t.Log("storing SLO file")
	commitID := storeWithCommit(t, projectName, "hardening", serviceName, gitCommitIDSLO, "slo.yaml")
	t.Logf("commitID is %s", commitID)

	t.Log("triggering the evaluation")
	_, evaluationFinishedEvent = performResourceServiceEvaluationTest(t, projectName, serviceName, commitID)

	t.Log("checking the finished event")
	err = keptnv2.Decode(evaluationFinishedEvent.Data, evaluationFinishedPayload)
	require.Nil(t, err)

	require.Equal(t, evaluationFinishedEvent.GitCommitID, commitID)
	t.Log("commitID is present and correct")

	//second part

	t.Log("storing second invalid SLO file")
	commitID1 := storeWithCommit(t, projectName, "hardening", serviceName, "gitCommitIDSLO", "slo.yaml")
	t.Logf("new commitID is %s", commitID1)

	t.Log("triggering the evaluation again with the old commitID")
	_, evaluationFinishedEvent = performResourceServiceEvaluationTest(t, projectName, serviceName, commitID)

	t.Log("checking the finished event again with the old commitID")
	err = keptnv2.Decode(evaluationFinishedEvent.Data, evaluationFinishedPayload)
	require.Nil(t, err)

	require.Equal(t, evaluationFinishedEvent.GitCommitID, commitID)
	t.Log("commitID is present and correct")
}

func performResourceServiceEvaluationTest(t *testing.T, projectName string, serviceName string, commitID string) (string, *models.KeptnContextExtendedCE) {
	keptnContext := ""
	source := "golang-test"

	t.Log("sent evaluation.hardening.triggered with commitid: ", commitID)
	_, err := ExecuteCommand(fmt.Sprintf("keptn trigger evaluation --project=%s --stage=hardening --service=%s --start=2022-01-26T10:05:53.931Z --end=2022-01-26T10:10:53.931Z --git-commit-id=%s", projectName, serviceName, commitID))
	require.Nil(t, err)

	var getSLITriggeredEvent *models.KeptnContextExtendedCE
	require.Eventually(t, func() bool {
		t.Log("checking if ", keptnv2.GetTriggeredEventType(keptnv2.GetSLITaskName), "for context ", keptnContext, " event is available")
		event, err := GetLatestEventOfType(keptnContext, projectName, "hardening", keptnv2.GetTriggeredEventType(keptnv2.GetSLITaskName))
		if err != nil || event == nil {
			return false
		}
		getSLITriggeredEvent = event
		return true
	}, 1*time.Minute, 10*time.Second)

	t.Log("got SLI triggered event, checking commitid")
	require.Equal(t, commitID, getSLITriggeredEvent.GitCommitID)
	t.Log("commitID is present and correct")

	getSLIPayload := &keptnv2.GetSLITriggeredEventData{}
	err = keptnv2.Decode(getSLITriggeredEvent.Data, getSLIPayload)
	require.Nil(t, err)
	require.Equal(t, "my-sli-provider", getSLIPayload.GetSLI.SLIProvider)
	require.NotEmpty(t, getSLIPayload.GetSLI.Start)
	require.NotEmpty(t, getSLIPayload.GetSLI.End)
	require.Contains(t, getSLIPayload.GetSLI.Indicators, "response_time_p95")
	require.Contains(t, getSLIPayload.GetSLI.Indicators, "throughput")
	require.Contains(t, getSLIPayload.GetSLI.Indicators, "error_rate")

	//SLI uses a different commitID
	_, err = ApiPOSTRequest("/v1/event", models.KeptnContextExtendedCE{
		Contenttype: "application/json",
		Data: &keptnv2.GetSLIStartedEventData{
			EventData: keptnv2.EventData{
				Project: projectName,
				Stage:   "hardening",
				Service: serviceName,
				Status:  keptnv2.StatusSucceeded,
				Result:  keptnv2.ResultPass,
				Message: "",
			},
		},
		ID:                 uuid.NewString(),
		Shkeptncontext:     keptnContext,
		Shkeptnspecversion: KeptnSpecVersion,
		Source:             &source,
		Specversion:        "1.0",
		Time:               time.Now(),
		Triggeredid:        getSLITriggeredEvent.ID,
		GitCommitID:        commitID,
		Type:               strutils.Stringp(keptnv2.GetStartedEventType(keptnv2.GetSLITaskName)),
	}, 3)

	require.Nil(t, err)
	_, err = ApiPOSTRequest("/v1/event", models.KeptnContextExtendedCE{
		Contenttype: "application/json",
		Data: &keptnv2.GetSLIFinishedEventData{
			EventData: keptnv2.EventData{
				Project: projectName,
				Stage:   "hardening",
				Service: serviceName,
				Labels:  nil,
				Status:  keptnv2.StatusSucceeded,
				Result:  keptnv2.ResultPass,
				Message: "",
			},
			GetSLI: keptnv2.GetSLIFinished{
				Start: getSLIPayload.GetSLI.Start,
				End:   getSLIPayload.GetSLI.End,
				IndicatorValues: []*keptnv2.SLIResult{
					{
						Metric:        "response_time_p95",
						Value:         200,
						ComparedValue: 0,
						Success:       true,
						Message:       "",
					},
					{
						Metric:        "throughput",
						Value:         200,
						Success:       true,
						ComparedValue: 0,
						Message:       "",
					},
					{
						Metric:        "error_rate",
						Value:         0,
						ComparedValue: 0,
						Success:       true,
						Message:       "",
					},
				},
			},
		},
		Extensions:         nil,
		ID:                 uuid.NewString(),
		Shkeptncontext:     keptnContext,
		Shkeptnspecversion: KeptnSpecVersion,
		Source:             &source,
		Specversion:        "1.0",
		Time:               time.Now(),
		Triggeredid:        getSLITriggeredEvent.ID,
		GitCommitID:        commitID,
		Type:               strutils.Stringp(keptnv2.GetFinishedEventType(keptnv2.GetSLITaskName)),
	}, 3)
	require.Nil(t, err)

	// wait for the evaluation.finished event to be available and evaluate it
	var evaluationFinishedEvent *models.KeptnContextExtendedCE
	require.Eventually(t, func() bool {
		t.Log("checking if evaluation.finished event is available")
		event, err := GetLatestEventOfType(keptnContext, projectName, "hardening", keptnv2.GetFinishedEventType(keptnv2.EvaluationTaskName))
		if err != nil || event == nil {
			return false
		}
		evaluationFinishedEvent = event
		return true
	}, 1*time.Minute, 10*time.Second)

	return keptnContext, evaluationFinishedEvent
}
