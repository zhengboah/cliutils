// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"fmt"
	"time"
)

type MultiStepRetry struct {
	Retry    int `json:"retry"`    // retry times
	Interval int `json:"interval"` // ms
}

type MultiExtractedVar struct {
	Name   string `json:"name"`
	Field  string `json:"field"`
	Secure bool   `json:"secure"`
	Value  string `json:"value,omitempty"`
}

type MultiStep struct {
	Type          string              `json:"type"` // http or wait
	AllowFailure  bool                `json:"allow_failure"`
	Retry         *MultiStepRetry     `json:"retry"`
	TaskString    string              `json:"task,omitempty"`
	Value         int                 `json:"value,omitempty"` // wait seconds for wait task
	ExtractedVars []MultiExtractedVar `json:"extracted_vars,omitempty"`

	result map[string]interface{}
}

type MultiTask struct {
	Task
	Steps []*MultiStep `json:"steps"`

	duration      time.Duration
	extractedVars []MultiExtractedVar
	lastStep      int
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
	fields = map[string]interface{}{
		"success": -1,
	}

	tags = map[string]string{
		"status": "FAIL",
	}
	for k, v := range t.Tags {
		tags[k] = v
	}

	if t.lastStep > -1 {
		step := t.Steps[t.lastStep]
		if step.result != nil {
			if step.result["status"] == "OK" {
				tags["status"] = "OK"
				fields["success"] = 1
			}
			fields["message"] = step.result["message"]
		}
	}

	steps := []map[string]interface{}{}

	for _, s := range t.Steps {
		// extraced vars
		extractedVars := []MultiExtractedVar{}
		for _, v := range s.ExtractedVars {
			ev := MultiExtractedVar{
				Name:   v.Name,
				Field:  v.Field,
				Secure: v.Secure,
			}

			if !v.Secure {
				ev.Value = v.Value
			}

			extractedVars = append(extractedVars, ev)
		}
		result := map[string]interface{}{}
		if s.result != nil {
			for k, v := range s.result {
				result[k] = v
			}
		}
		result["extracted_vars"] = extractedVars
		steps = append(steps, result)
	}

	bytes, _ := json.Marshal(steps)
	fields["steps"] = string(bytes)

	return tags, fields
}

func (t *MultiTask) runHTTPStep(step *MultiStep) (map[string]interface{}, error) {
	var err error
	var task ITask
	runCount := 0
	maxCount := 1
	interval := time.Millisecond

	result := map[string]interface{}{}
	if step == nil {
		return nil, fmt.Errorf("step should not be nil")
	}

	if step.Retry != nil {
		if step.Retry.Retry > 0 {
			maxCount = step.Retry.Retry + 1
		}
		interval = time.Duration(step.Retry.Interval) * time.Millisecond
	}

	for runCount < maxCount {
		httpTask := HTTPTask{}
		if err = json.Unmarshal([]byte(step.TaskString), &httpTask); err != nil {
			return nil, fmt.Errorf("unmarshal http step task failed: %w", err)
		}

		task, err = NewTask(&httpTask)
		if err != nil {
			return nil, fmt.Errorf("new task failed: %w", err)
		}

		for _, v := range t.extractedVars {
			task.AddExtractedVar(&ConfigVar{
				Name:   v.Name,
				Secure: v.Secure,
				Value:  v.Value,
			})
		}

		err = task.RenderTemplateAndInit(t.globalVars)
		if err != nil {
			err = fmt.Errorf("init http step task failed: %w", err)
		} else {
			err = task.Run()
			if err != nil {
				err = fmt.Errorf("run http step task failed: %w", err)
			}
			tags, fields := task.GetResults()
			for k, v := range tags {
				result[k] = v
			}
			for k, v := range fields {
				result[k] = v
			}
		}

		if httpTask.postScriptResult != nil { // set extracted vars
			for i, v := range step.ExtractedVars {
				value, ok := httpTask.postScriptResult.Vars[v.Name]
				if ok {
					step.ExtractedVars[i].Value = fmt.Sprintf("%v", value)
				}

				// set extracted vars, which can be used in next step
				t.extractedVars = append(t.extractedVars, v)
			}

		}

		runCount++
		if runCount < maxCount {
			time.Sleep(interval)
		}
	}

	if len(result) > 0 && result["status"] != "OK" && step.AllowFailure {
		err = nil
	}

	return result, err
}

func (t *MultiTask) run() error {
	now := time.Now()
	lastStep := -1 // last step which is not wait
	for i, step := range t.Steps {
		switch step.Type {
		case "http":
			if i > lastStep {
				lastStep = i
			}
			if result, err := t.runHTTPStep(step); err != nil {
				return fmt.Errorf("run http step task failed: %w", err)
			} else {
				step.result = result
			}
		case "wait":
			time.Sleep(time.Duration(step.Value) * time.Second)

		default:
			return fmt.Errorf("step type should be wait or http")
		}
	}

	t.duration = time.Since(now)
	t.lastStep = lastStep

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
func (t *MultiTask) getHostName() ([]string, error) {
	hostNames := []string{}
	for _, step := range t.Steps {
		if step.Type == "http" {
			ct := &HTTPTask{}
			if err := json.Unmarshal([]byte(step.TaskString), ct); err != nil {
				return nil, fmt.Errorf("unmarshal http step task failed: %w", err)
			}

			task, err := NewTask(ct)
			if err != nil {
				return nil, fmt.Errorf("new task failed: %w", err)
			}

			if v, err := task.GetHostName(); err != nil {
				return nil, fmt.Errorf("get host name failed: %w", err)
			} else {
				hostNames = append(hostNames, v...)
			}
		}
	}

	return hostNames, nil
}

// TODO
func (t *MultiTask) getVariableValue(variable Variable) (string, error) {
	return "", fmt.Errorf("not support")
}

func (t *MultiTask) beforeFirstRender() {
}