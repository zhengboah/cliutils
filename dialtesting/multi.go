// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"fmt"
)

type MultiStepRetry struct {
	Retry    int `json:"retry"`    // retry times
	Interval int `json:"interval"` // ms
}

type MultiExtractedVar struct {
	Name   string `json:"name"`
	Field  string `json:"field"`
	Secure bool   `json:"secure"`
}

type MultiStep struct {
	Type          string              `json:"type"` // http or wait
	AllowFailure  bool                `json:"allow_failure"`
	Retry         *MultiStepRetry     `json:"retry"`
	TaskString    string              `json:"task,omitempty"`
	Value         int                 `json:"value,omitempty"` // wait seconds for wait task
	ExtractedVars []MultiExtractedVar `json:"extracted_vars,omitempty"`

	task ITask
}

type MultiTask struct {
	Task
	Steps []*MultiStep `json:"steps"`

	reqError    error
	stepResults []map[string]interface{}
}

func (t *MultiTask) clear() {
}

func (t *MultiTask) stop() error {
	return nil
}

func (t *MultiTask) class() string {
	return ClassMulti
}

func (t *MultiTask) metricName() string {
	return `multi_dial_testing`
}

func (t *MultiTask) getResults() (tags map[string]string, fields map[string]interface{}) {
	bytes, _ := json.Marshal(t.Task.ConfigVars)
	fields[`config_vars`] = string(bytes)

	return tags, fields
}

func (t *MultiTask) runHTTPStep(step *MultiStep) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	if step == nil {
		return nil, fmt.Errorf("step should not be nil")
	}
	task := HTTPTask{}
	if err := json.Unmarshal([]byte(step.TaskString), &task); err != nil {
		return nil, fmt.Errorf("unmarshal http step task failed: %w", err)
	}

	err := task.RenderTemplate(t.globalVars)
	if err != nil {
		err = fmt.Errorf("init http step task failed: %w", err)
	} else {
		err = task.Run()
		if err != nil {
			err = fmt.Errorf("run http step task failed: %w", err)
		}
	}

	if err != nil {
		if step.AllowFailure {
			err = nil
			tags, fields := task.GetResults()
			for k, v := range tags {
				result[k] = v
			}
			for k, v := range fields {
				result[k] = v
			}
		}
	}

	return result, err
}

func (t *MultiTask) run() error {

	results := []map[string]interface{}{}

	for _, step := range t.Steps {
		if step.Type == "http" {
			if result, err := t.runHTTPStep(step); err != nil {
				return fmt.Errorf("run http step task failed: %w", err)
			} else {
				results = append(results, result)
			}
		}
	}

	t.stepResults = results

	return nil
}

func (t *MultiTask) check() error {
	if len(t.Steps) == 0 {
		return fmt.Errorf("steps should not be empty")
	}

	for _, step := range t.Steps {
		switch step.Type {
		case "wait":
			if step.Value == 0 {
				return fmt.Errorf("wait step value should not be 0")
			}

		case "http":
			if step.TaskString == "" {
				return fmt.Errorf("http step task should not be empty")
			}

			task := HTTPTask{}
			if err := json.Unmarshal([]byte(step.TaskString), &task); err != nil {
				return fmt.Errorf("unmarshal http step task failed: %w", err)
			}

			if err := task.Check(); err != nil {
				return fmt.Errorf("check http step task failed: %w", err)
			}
		default:
			return fmt.Errorf("step type should be wait or http")
		}
	}

	return nil
}

func (t *MultiTask) checkResult() (reasons []string, succFlag bool) {
	return nil, false
}

func (t *MultiTask) init() error {
	return nil
}

// TODO
func (t *MultiTask) getHostName() (string, error) {
	return "", fmt.Errorf("not support")
}

// TODO
func (t *MultiTask) getVariableValue(variable Variable) (string, error) {
	return "", fmt.Errorf("not support")
}

func (t *MultiTask) beforeFirstRender() {
}
