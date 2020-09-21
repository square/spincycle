// Copyright 2020, Square, Inc.

package spec

type CheckResult struct {
	Errors   []error
	Warnings []error
}

type CheckResults struct {
	Results    map[string]*CheckResult
	AnyError   bool
	AnyWarning bool
}

func NewCheckResults() *CheckResults {
	return &CheckResults{
		Results: map[string]*CheckResult{},
	}
}

func (c *CheckResults) AddResult(key string, result *CheckResult) {
	current, ok := c.Results[key]
	if ok {
		current.Errors = append(current.Errors, result.Errors...)
		current.Warnings = append(current.Warnings, result.Warnings...)
	} else {
		c.Results[key] = result
	}
	c.AnyError = c.AnyError || len(result.Errors) > 0
	c.AnyWarning = c.AnyWarning || len(result.Warnings) > 0
}

func (c *CheckResults) AddError(key string, err error) {
	if _, ok := c.Results[key]; !ok {
		c.Results[key] = &CheckResult{}
	}
	c.Results[key].Errors = append(c.Results[key].Errors, err)
	c.AnyError = true
}

func (c *CheckResults) AddWarning(key string, err error) {
	if _, ok := c.Results[key]; !ok {
		c.Results[key] = &CheckResult{}
	}
	c.Results[key].Warnings = append(c.Results[key].Warnings, err)
	c.AnyWarning = true
}

func (c *CheckResults) Union(other *CheckResults) {
	for key, result := range other.Results {
		if _, ok := c.Results[key]; !ok {
			c.Results[key] = &CheckResult{}
		}
		c.Results[key].Errors = append(c.Results[key].Errors, result.Errors...)
		c.Results[key].Warnings = append(c.Results[key].Warnings, result.Warnings...)
	}
	c.AnyError = c.AnyError || other.AnyError
	c.AnyWarning = c.AnyWarning || other.AnyWarning
}

func (c *CheckResults) Get(key string) (*CheckResult, bool) {
	result, ok := c.Results[key]
	return result, ok
}
