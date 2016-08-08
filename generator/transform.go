// Copyright 2015 ThoughtWorks, Inc.

// This file is part of getgauge/html-report.

// getgauge/html-report is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// getgauge/html-report is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with getgauge/html-report.  If not, see <http://www.gnu.org/licenses/>.

package generator

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"time"

	gm "github.com/getgauge/html-report/gauge_messages"
)

const (
	execTimeFormat = "15:04:05"
	dothtml        = ".html"
)

func toOverview(res *gm.ProtoSuiteResult) *overview {
	totalSpecs := 0
	if res.GetSpecResults() != nil {
		totalSpecs = len(res.GetSpecResults())
	}
	passed := totalSpecs - int(res.GetSpecsFailedCount()) - int(res.GetSpecsSkippedCount())
	return &overview{
		ProjectName: res.GetProjectName(),
		Env:         res.GetEnvironment(),
		Tags:        res.GetTags(),
		SuccRate:    res.GetSuccessRate(),
		ExecTime:    formatTime(res.GetExecutionTime()),
		Timestamp:   res.GetTimestamp(),
		TotalSpecs:  totalSpecs,
		Failed:      int(res.GetSpecsFailedCount()),
		Passed:      passed,
		Skipped:     int(res.GetSpecsSkippedCount()),
	}
}

func toHookFailure(failure *gm.ProtoHookFailure, hookName string) *hookFailure {
	if failure == nil {
		return nil
	}

	return &hookFailure{
		ErrMsg:     failure.GetErrorMessage(),
		HookName:   hookName,
		Screenshot: base64.StdEncoding.EncodeToString(failure.GetScreenShot()),
		StackTrace: failure.GetStackTrace(),
	}
}

func toHTMLFileName(specName, projectRoot string) string {
	specPath, err := filepath.Rel(projectRoot, specName)
	if err != nil {
		specPath = filepath.Join(projectRoot, filepath.Base(specName))
	}
	specPath = strings.Replace(specPath, string(filepath.Separator), "_", -1)
	ext := filepath.Ext(specPath)
	return strings.TrimSuffix(specPath, ext) + dothtml
}

func toSidebar(res *gm.ProtoSuiteResult) *sidebar {
	specsMetaList := make([]*specsMeta, 0)
	for _, specRes := range res.SpecResults {
		sm := &specsMeta{
			SpecName:   specRes.ProtoSpec.GetSpecHeading(),
			ExecTime:   formatTime(specRes.GetExecutionTime()),
			Failed:     specRes.GetFailed(),
			Skipped:    specRes.GetSkipped(),
			Tags:       specRes.ProtoSpec.GetTags(),
			ReportFile: toHTMLFileName(specRes.ProtoSpec.GetFileName(), ProjectRoot),
		}
		specsMetaList = append(specsMetaList, sm)
	}

	return &sidebar{
		IsBeforeHookFailure: res.PreHookFailure != nil,
		Specs:               specsMetaList,
	}
}

func toSpecHeader(res *gm.ProtoSpecResult) *specHeader {
	return &specHeader{
		SpecName: res.ProtoSpec.GetSpecHeading(),
		ExecTime: formatTime(res.GetExecutionTime()),
		FileName: res.ProtoSpec.GetFileName(),
		Tags:     res.ProtoSpec.GetTags(),
	}
}

func toSpec(res *gm.ProtoSpecResult) *spec {
	spec := &spec{
		CommentsBeforeTable: make([]string, 0),
		CommentsAfterTable:  make([]string, 0),
		Scenarios:           make([]*scenario, 0),
		BeforeHookFailure:   toHookFailure(res.GetProtoSpec().GetPreHookFailure(), "Before Spec"),
		AfterHookFailure:    toHookFailure(res.GetProtoSpec().GetPostHookFailure(), "After Spec"),
	}
	isTableScanned := false
	for _, item := range res.GetProtoSpec().GetItems() {
		switch item.GetItemType() {
		case gm.ProtoItem_Comment:
			if isTableScanned {
				spec.CommentsAfterTable = append(spec.CommentsAfterTable, item.GetComment().GetText())
			} else {
				spec.CommentsBeforeTable = append(spec.CommentsBeforeTable, item.GetComment().GetText())
			}
		case gm.ProtoItem_Table:
			spec.Table = toTable(item.GetTable())
			isTableScanned = true
		case gm.ProtoItem_Scenario:
			spec.Scenarios = append(spec.Scenarios, toScenario(item.GetScenario(), -1))
		case gm.ProtoItem_TableDrivenScenario:
			for i, sce := range item.GetTableDrivenScenario().GetScenarios() {
				spec.Scenarios = append(spec.Scenarios, toScenario(sce, i))
			}
		}
	}
	return spec
}

func toScenario(scn *gm.ProtoScenario, tableRowIndex int) *scenario {
	return &scenario{
		Heading:           scn.GetScenarioHeading(),
		ExecTime:          formatTime(scn.GetExecutionTime()),
		Tags:              scn.GetTags(),
		ExecStatus:        getStatus(scn.GetFailed(), scn.GetSkipped()),
		Contexts:          getItems(scn.GetContexts()),
		Items:             getItems(scn.GetScenarioItems()),
		Teardown:          getItems(scn.GetTearDownSteps()),
		BeforeHookFailure: toHookFailure(scn.GetPreHookFailure(), "Before Scenario"),
		AfterHookFailure:  toHookFailure(scn.GetPostHookFailure(), "After Scenario"),
		TableRowIndex:     tableRowIndex,
	}
}

func toComment(protoComment *gm.ProtoComment) *comment {
	return &comment{Text: protoComment.GetText()}
}

func toStep(protoStep *gm.ProtoStep) *step {
	res := protoStep.GetStepExecutionResult().GetExecutionResult()
	result := &result{
		Status:     getStatus(res.GetFailed(), protoStep.GetStepExecutionResult().GetSkipped()),
		Screenshot: base64.StdEncoding.EncodeToString(res.GetScreenShot()),
		StackTrace: res.GetStackTrace(),
		Message:    res.GetErrorMessage(),
		ExecTime:   formatTime(res.GetExecutionTime()),
	}
	if protoStep.GetStepExecutionResult().GetSkipped() {
		result.Message = protoStep.GetStepExecutionResult().GetSkippedReason()
	}
	return &step{
		Fragments:       toFragments(protoStep.GetFragments()),
		Res:             result,
		PreHookFailure:  toHookFailure(protoStep.GetStepExecutionResult().GetPreHookFailure(), "Before Step"),
		PostHookFailure: toHookFailure(protoStep.GetStepExecutionResult().GetPostHookFailure(), "After Step"),
	}
}

func toConcept(protoConcept *gm.ProtoConcept) *concept {
	protoConcept.ConceptStep.StepExecutionResult = protoConcept.GetConceptExecutionResult()
	return &concept{
		CptStep: toStep(protoConcept.GetConceptStep()),
		Items:   getItems(protoConcept.GetSteps()),
	}
}

func toFragments(protoFragments []*gm.Fragment) []*fragment {
	fragments := make([]*fragment, 0)
	for _, f := range protoFragments {
		switch f.GetFragmentType() {
		case gm.Fragment_Text:
			fragments = append(fragments, &fragment{FragmentKind: textFragmentKind, Text: f.GetText()})
		case gm.Fragment_Parameter:
			switch f.GetParameter().GetParameterType() {
			case gm.Parameter_Static:
				fragments = append(fragments, &fragment{FragmentKind: staticFragmentKind, Text: f.GetParameter().GetValue()})
			case gm.Parameter_Dynamic:
				fragments = append(fragments, &fragment{FragmentKind: dynamicFragmentKind, Text: f.GetParameter().GetValue()})
			case gm.Parameter_Table:
				fragments = append(fragments, &fragment{FragmentKind: tableFragmentKind, Table: toTable(f.GetParameter().GetTable())})
			case gm.Parameter_Special_Table:
				fragments = append(fragments, &fragment{FragmentKind: specialTableFragmentKind, Name: f.GetParameter().GetName(), Table: toTable(f.GetParameter().GetTable())})
			case gm.Parameter_Special_String:
				fragments = append(fragments, &fragment{FragmentKind: specialStringFragmentKind, Name: f.GetParameter().GetName(), Text: f.GetParameter().GetValue()})
			}
		}
	}
	return fragments
}

func toTable(protoTable *gm.ProtoTable) *table {
	rows := make([]*row, len(protoTable.GetRows()))
	for i, r := range protoTable.GetRows() {
		rows[i] = &row{
			Cells: r.GetCells(),
			Res:   pass,
		}
	}
	return &table{Headers: protoTable.GetHeaders().GetCells(), Rows: rows}
}

func getItems(protoItems []*gm.ProtoItem) []item {
	items := make([]item, 0)
	for _, i := range protoItems {
		switch i.GetItemType() {
		case gm.ProtoItem_Step:
			items = append(items, toStep(i.GetStep()))
		case gm.ProtoItem_Comment:
			items = append(items, toComment(i.GetComment()))
		case gm.ProtoItem_Concept:
			items = append(items, toConcept(i.GetConcept()))
		}
	}
	return items
}

func getStatus(failed, skipped bool) status {
	if failed {
		return fail
	} else if skipped {
		return skip
	}
	return pass
}

func formatTime(ms int64) string {
	return time.Unix(0, ms*int64(time.Millisecond)).UTC().Format(execTimeFormat)
}
