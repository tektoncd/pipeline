package e2e

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func Details(pre v1alpha1.PipelineResource) string {
	var key = "url"
	if pre.Spec.Type == v1alpha1.PipelineResourceTypeStorage {
		key = "location"
	}
	if pre.Spec.Type == v1alpha1.PipelineResourceTypeCloudEvent {
		key = "targeturi"
	}

	for _, p := range pre.Spec.Params {
		if strings.ToLower(p.Name) == key {
			return p.Name + ": " + p.Value
		}
	}

	return "---"
}

func TaskRunHasFailed(tr *v1alpha1.TaskRun) string {
	if len(tr.Status.Conditions) == 0 {
		return ""
	}

	if tr.Status.Conditions[0].Status == corev1.ConditionFalse {
		return tr.Status.Conditions[0].Message
	}
	return ""
}

// this will sort the Resource by Type and then by Name
func SortResourcesByTypeAndName(pres []v1alpha1.PipelineDeclaredResource) []v1alpha1.PipelineDeclaredResource {
	sort.Slice(pres, func(i, j int) bool {
		if pres[j].Type < pres[i].Type {
			return false
		}

		if pres[j].Type > pres[i].Type {
			return true
		}

		return pres[j].Name > pres[i].Name
	})

	return pres
}

// Pipeline Run Describe command

func PipelineRunHasFailed(pr *v1alpha1.PipelineRun) string {
	if len(pr.Status.Conditions) == 0 {
		return ""
	}

	if pr.Status.Conditions[0].Status == corev1.ConditionFalse {
		for _, taskrunStatus := range pr.Status.TaskRuns {
			if len(taskrunStatus.Status.Conditions) == 0 {
				continue
			}
			if taskrunStatus.Status.Conditions[0].Status == corev1.ConditionFalse {
				return fmt.Sprintf("%s (%s)", pr.Status.Conditions[0].Message,
					taskrunStatus.Status.Conditions[0].Message)
			}
		}
		return pr.Status.Conditions[0].Message
	}
	return ""
}

type taskrunList []tkr

type tkr struct {
	TaskrunName string
	*v1alpha1.PipelineRunTaskRunStatus
}

func (s taskrunList) Len() int      { return len(s) }
func (s taskrunList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s taskrunList) Less(i, j int) bool {
	return s[j].Status.StartTime.Before(s[i].Status.StartTime)
}

func NewTaskrunListFromMapWithTestData(t *testing.T, statusMap map[string]*v1alpha1.PipelineRunTaskRunStatus, td map[int]interface{}) taskrunList {
	t.Helper()
	var trl taskrunList

	for _, tr := range td {
		switch tr := tr.(type) {
		case *PipelineRunDescribeData:
			if len(tr.TaskRuns) == len(statusMap) {

				taskRefData := []*TaskRunRefData{}

				for _, ref := range tr.TaskRuns {
					switch ref := ref.(type) {
					case *TaskRunRefData:
						taskRefData = append(taskRefData, ref)
					default:
						t.Error("TaskRunRef Test Data Format Didn't Match please do check Test Data which you passing")
					}
				}
				for _, tref := range taskRefData {
					statusMap[tref.TaskRunName].Status.Conditions[0].Reason = tref.Status
					trl = append(trl, tkr{
						tref.TaskRunName,
						statusMap[tref.TaskRunName],
					})

				}
			} else {
				t.Error("Test data length didnt match with Real data")
			}

		default:
			t.Error("Test Data Format Didn't Match please do recheck Test Data")
		}
	}

	return trl
}
