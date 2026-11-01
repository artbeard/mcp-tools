package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"math"
	"os"
	"strings"
	"testing"
)

func TestWindowsReadOnlyWorkIntegration(t *testing.T) {
	if os.Getenv("MS_PROJECT_READONLY_TEST") != "1" {
		t.Skip("требуются Windows и открытый Project; проверка только чтения")
	}
	script := base64.StdEncoding.EncodeToString([]byte(changeScript("e30=", false)))
	parser := `$ErrorActionPreference = 'Stop'
$script = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('` + script + `'))
$tokens = $null
$parseErrors = $null
[void][System.Management.Automation.Language.Parser]::ParseInput($script, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count -gt 0) { throw ($parseErrors | Out-String) }
[Console]::Out.Write('{}')`
	if _, err := runPowerShell(parser); err != nil {
		t.Fatal(err)
	}
	raw, err := runPowerShell(outlineScript)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot outlineSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Path == "" || len(snapshot.Tasks) == 0 {
		t.Fatal("открытый план не прочитан")
	}
	for _, task := range snapshot.Tasks {
		if task.WorkHours < 0 || task.ActualWorkHours < 0 || task.RemainingWorkHours < 0 {
			t.Fatal("отрицательные часы в снимке")
		}
		if len(task.Assignments) > 0 && !task.Summary {
			var total float64
			for _, assignment := range task.Assignments {
				total += assignment.WorkHours
			}
			if math.Abs(total-task.WorkHours) > 0.001 {
				t.Fatalf("часы задачи %d не совпадают с назначениями", task.UniqueID)
			}
		}
	}
}

func TestDecodeWorkChange(t *testing.T) {
	request, err := decodeChange(`{"kind":"schedule","operation":"update","task_unique_id":34,"work_hours":0,"task_type":0}`)
	if err != nil || request.WorkHours == nil || *request.WorkHours != 0 || request.TaskType == nil {
		t.Fatalf("zero work must remain explicit: %#v, %v", request, err)
	}
	for _, input := range []string{
		`{"kind":"schedule","operation":"update","task_unique_id":34,"work":160}`,
		`{"kind":"schedule","operation":"update","task_unique_id":34,"work_hours":160} {}`,
	} {
		if _, err := decodeChange(input); err == nil {
			t.Fatalf("ambiguous input accepted: %s", input)
		}
	}
}

func TestValidateWorkChange(t *testing.T) {
	for _, hours := range []float64{0, 0.5, 160, 160.25} {
		if err := validateChange(changeRequest{Kind: "schedule", Operation: "update", TaskUniqueID: 1, WorkHours: &hours}); err != nil {
			t.Fatal(err)
		}
	}
	for _, hours := range []float64{-1, math.NaN(), math.Inf(1), 1e8, 0.001} {
		if err := validateChange(changeRequest{Kind: "schedule", Operation: "update", TaskUniqueID: 1, WorkHours: &hours}); err == nil {
			t.Fatalf("invalid hours accepted: %v", hours)
		}
	}
	hours := 160.0
	if err := validateChange(changeRequest{Kind: "assignment", Operation: "update", TaskUniqueID: 1, ResourceUniqueID: 2, WorkHours: &hours}); err != nil {
		t.Fatal(err)
	}
	for _, request := range []changeRequest{
		{Kind: "assignment", Operation: "update", TaskUniqueID: 1, ResourceUniqueID: 2},
		{Kind: "assignment", Operation: "add", TaskUniqueID: 1, ResourceUniqueID: 2, WorkHours: &hours},
		{Kind: "schedule", Operation: "link", TaskUniqueID: 1, WorkHours: &hours},
		{Kind: "schedule", Operation: "update", TaskUniqueID: 1, WorkHours: &hours, Duration: "5d"},
		{Kind: "task", Operation: "update", TaskUniqueID: 1, WorkHours: &hours, Start: "2026-10-08"},
	} {
		if err := validateChange(request); err == nil {
			t.Fatalf("invalid request accepted: %#v", request)
		}
	}
}

func TestDependencyTypesAndReferences(t *testing.T) {
	request, err := decodeChange(`{"kind":"schedule","operation":"link","task_unique_id":36,"predecessor_unique_id":77,"link_type":0}`)
	if err != nil || request.LinkType == nil || *request.LinkType != 0 {
		t.Fatalf("finish-to-finish must remain explicit: %#v, %v", request, err)
	}
	for _, kind := range []int{0, 1, 2, 3} {
		request.LinkType = &kind
		if err := validateChange(request); err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []int{-1, 4} {
		request.LinkType = &kind
		if err := validateChange(request); err == nil {
			t.Fatalf("invalid link type accepted: %d", kind)
		}
	}
	kind := 0
	request.LinkType = &kind
	request.Operation = "unlink"
	if err := validateChange(request); err == nil {
		t.Fatal("unlink must not silently ignore link_type")
	}
	request.LinkType = nil
	request.PredecessorUniqueID = request.TaskUniqueID
	if err := validateChange(request); err == nil {
		t.Fatal("self dependency accepted")
	}
}

func TestDecodeAndValidateActualWorkChange(t *testing.T) {
	request, err := decodeChange(`{"kind":"assignment","operation":"update","task_unique_id":17,"resource_unique_id":12,"work_hours":5,"actual_work_hours":1}`)
	if err != nil || request.ActualWorkHours == nil || *request.ActualWorkHours != 1 {
		t.Fatalf("explicit actual work not decoded: %#v, %v", request, err)
	}
	for _, actual := range []float64{0, 1, 5} {
		request.ActualWorkHours = &actual
		if err := validateChange(request); err != nil {
			t.Fatal(err)
		}
	}
	for _, actual := range []float64{-1, 6, math.NaN(), math.Inf(1), 0.001} {
		request.ActualWorkHours = &actual
		if err := validateChange(request); err == nil {
			t.Fatalf("invalid actual accepted: %v", actual)
		}
	}
	actual := 1.0
	request.ActualWorkHours = &actual
	request.Kind = "schedule"
	if err := validateChange(request); err == nil {
		t.Fatal("task-level actual work is ambiguous with multiple assignments")
	}
	request.Kind = "assignment"
	request.WorkHours = nil
	if err := validateChange(request); err == nil {
		t.Fatal("actual update requires explicit total work")
	}
}

func TestAssignmentAddWithWorkAndUnitsUpdate(t *testing.T) {
	for _, operation := range []string{"add", "update"} {
		input := `{"kind":"assignment","operation":"` + operation + `","task_unique_id":1,"resource_unique_id":2,"units":0.3,"work_hours":48}`
		request, err := decodeChange(input)
		if err != nil || validateChange(request) != nil {
			t.Fatalf("valid assignment request rejected: %s, %v", input, err)
		}
		for _, units := range []float64{0, -1, math.NaN(), math.Inf(1), 101} {
			request.Units = &units
			if validateChange(request) == nil {
				t.Fatalf("invalid units accepted: %v", units)
			}
		}
	}
	hours, units := 24.0, 1.0
	request := changeRequest{Kind: "assignment", Operation: "add", TaskUniqueID: 1, ResourceUniqueID: 2, Units: &units, WorkHours: &hours}
	snapshot := outlineSnapshot{Tasks: []taskSnapshot{{UniqueID: 1, TaskType: 0, ActualWorkHours: 100}}}
	if _, a, err := workTarget(snapshot, request); err != nil || a != nil {
		t.Fatalf("new assignment has no recorded actual: %v", err)
	}
	snapshot.Tasks[0].Assignments = []assignmentSnapshot{{ResourceUniqueID: 2}}
	if _, _, err := workTarget(snapshot, request); err == nil {
		t.Fatal("duplicate assignment accepted")
	}
	request.Operation = "update"
	if _, _, err := workTarget(snapshot, request); err == nil {
		t.Fatal("units update on task with recorded actual accepted")
	}
	snapshot.Tasks[0].ActualWorkHours = 0
	if _, _, err := workTarget(snapshot, request); err != nil {
		t.Fatal(err)
	}
	request.Operation = "add"
	snapshot.Tasks[0].Assignments = nil
	for _, taskType := range []int{1, 2} {
		snapshot.Tasks[0].TaskType = taskType
		if _, _, err := workTarget(snapshot, request); err == nil {
			t.Fatalf("add with work accepted for task type %d", taskType)
		}
	}
	snapshot.Tasks[0].TaskType = 0
	snapshot.Tasks[0].EffortDriven = true
	if _, _, err := workTarget(snapshot, request); err == nil {
		t.Fatal("effort-driven add accepted")
	}
}

func TestUnitsOnlyChangeWithRecordedHistory(t *testing.T) {
	request, err := decodeChange(`{"kind":"assignment","operation":"update","task_unique_id":1,"resource_unique_id":2,"units":1}`)
	if err != nil || !unitsOnlyChange(request) {
		t.Fatalf("units-only request rejected: %v", err)
	}
	for _, remaining := range []float64{0, 28.00116666666667} {
		snapshot := outlineSnapshot{Tasks: []taskSnapshot{{
			UniqueID: 1, TaskType: 0, ActualWorkHours: 134.925,
			Assignments: []assignmentSnapshot{{ResourceUniqueID: 2, Units: 0.7,
				WorkHours: 134.925 + remaining, ActualWorkHours: 134.925, RemainingWorkHours: remaining}},
		}}}
		task, assignment, err := workTarget(snapshot, request)
		if err != nil || assignment == nil || assignment.ActualWorkHours != 134.925 ||
			assignment.RemainingWorkHours != remaining || task.ActualWorkHours != 134.925 {
			t.Fatalf("valid history was changed or rejected: %#v, %v", assignment, err)
		}
		for _, taskType := range []int{1, 2} {
			snapshot.Tasks[0].TaskType = taskType
			if _, _, err := workTarget(snapshot, request); err == nil {
				t.Fatalf("units-only accepted for task type %d", taskType)
			}
		}
		snapshot.Tasks[0].TaskType = 0
		snapshot.Tasks[0].Summary = true
		if _, _, err := workTarget(snapshot, request); err == nil {
			t.Fatal("summary units edit accepted")
		}
		snapshot.Tasks[0].Summary = false
		snapshot.Tasks[0].Assignments = nil
		if _, _, err := workTarget(snapshot, request); err == nil {
			t.Fatal("missing assignment accepted")
		}
	}
	for _, input := range []string{
		`{"kind":"assignment","operation":"update","task_unique_id":1,"resource_unique_id":2,"units":1,"duration":"5d"}`,
		`{"kind":"assignment","operation":"update","task_unique_id":1,"resource_unique_id":2,"units":1,"actual_work_hours":1}`,
		`{"kind":"assignment","operation":"update","task_unique_id":1,"resource_unique_id":2,"units":1,"task_type":0}`,
		`{"kind":"assignment","operation":"update","task_unique_id":1,"resource_unique_id":2,"units":1,"manual":false}`,
	} {
		if _, err := decodeChange(input); err == nil {
			t.Fatalf("mixed units-only edit accepted: %s", input)
		}
	}
}

func TestActualWorkTargetPreservesRecordedHistory(t *testing.T) {
	work, actual := 10.0, 8.0
	request := changeRequest{Kind: "assignment", Operation: "update", TaskUniqueID: 1, ResourceUniqueID: 2, WorkHours: &work, ActualWorkHours: &actual}
	snapshot := outlineSnapshot{Tasks: []taskSnapshot{{UniqueID: 1, ActualWorkHours: 6, Assignments: []assignmentSnapshot{{ResourceUniqueID: 2, ActualWorkHours: 6}}}}}
	if _, _, err := workTarget(snapshot, request); err != nil {
		t.Fatal(err)
	}
	actual = 5
	if _, _, err := workTarget(snapshot, request); err == nil {
		t.Fatal("decreasing recorded actual work accepted")
	}
	actual = 6
	snapshot.Tasks[0].Assignments[0].ActualWorkHours = 6.000058333697919
	if _, _, err := workTarget(snapshot, request); err != nil {
		t.Fatalf("Project rounding must allow idempotent retries: %v", err)
	}
}

func TestWorkTargetRejectsDestructiveOrAmbiguousUpdates(t *testing.T) {
	hours := 10.0
	request := changeRequest{Kind: "schedule", Operation: "update", TaskUniqueID: 1, WorkHours: &hours}
	for _, task := range []taskSnapshot{
		{UniqueID: 1, Summary: true},
		{UniqueID: 1, Milestone: true},
		{UniqueID: 1, ActualWorkHours: 12},
	} {
		if _, _, err := workTarget(outlineSnapshot{Tasks: []taskSnapshot{task}}, request); err == nil {
			t.Fatalf("invalid target accepted: %#v", task)
		}
	}
	task := taskSnapshot{UniqueID: 1, ActualWorkHours: 4, Assignments: []assignmentSnapshot{{ResourceUniqueID: 2, ActualWorkHours: 12}}}
	request.Kind = "assignment"
	request.ResourceUniqueID = 2
	if _, _, err := workTarget(outlineSnapshot{Tasks: []taskSnapshot{task}}, request); err == nil {
		t.Fatal("assignment actual work must be checked independently")
	}
	request.ResourceUniqueID = 3
	if _, _, err := workTarget(outlineSnapshot{Tasks: []taskSnapshot{task}}, request); err == nil {
		t.Fatal("missing assignment accepted")
	}
	task.Assignments = append(task.Assignments, task.Assignments[0])
	request.ResourceUniqueID = 2
	if _, _, err := workTarget(outlineSnapshot{Tasks: []taskSnapshot{task}}, request); err == nil {
		t.Fatal("duplicate assignments accepted")
	}
}

func TestWorkTargetAllowsUnassignedTasksAndActualWorkBoundary(t *testing.T) {
	hours := 10.0
	request := changeRequest{Kind: "schedule", Operation: "update", TaskUniqueID: 1, WorkHours: &hours}
	snapshot := outlineSnapshot{Tasks: []taskSnapshot{{UniqueID: 1, ActualWorkHours: 10}}}
	if _, _, err := workTarget(snapshot, request); err != nil {
		t.Fatal(err)
	}
	request.TaskUniqueID = 2
	if _, _, err := workTarget(snapshot, request); err == nil {
		t.Fatal("missing task accepted")
	}
}

func TestValidateTaskTypeAndEffortDriven(t *testing.T) {
	for _, taskType := range []int{0, 1, 2} {
		if err := validateChange(changeRequest{Kind: "schedule", Operation: "update", TaskUniqueID: 1, TaskType: &taskType}); err != nil {
			t.Fatal(err)
		}
	}
	taskType := 3
	if err := validateChange(changeRequest{Kind: "schedule", Operation: "update", TaskUniqueID: 1, TaskType: &taskType}); err == nil {
		t.Fatal("invalid task type accepted")
	}
	taskType = 2
	effortDriven := false
	if err := validateChange(changeRequest{Kind: "schedule", Operation: "update", TaskUniqueID: 1, TaskType: &taskType, EffortDriven: &effortDriven}); err == nil {
		t.Fatal("fixed work must be effort driven")
	}
}

func TestPowerShellCommandPassesLargeUnicodeScriptThroughStdin(t *testing.T) {
	script := strings.Repeat("# Проверка длинного сценария\n", 3000)
	command := powerShellCommand(script)
	for _, arg := range command.Args {
		if len(arg) > 1024 {
			t.Fatal("script must not exceed the Windows command-line limit")
		}
	}
	input, err := io.ReadAll(command.Stdin)
	if err != nil || string(input) != script {
		t.Fatalf("script encoding was not preserved: %v", err)
	}
}

func TestEncodePowerShellUsesUTF16LE(t *testing.T) {
	encoded := encodePowerShell("'Пример'")
	if encoded == "" {
		t.Fatal("encoded command is empty")
	}
}

func TestOutlineScriptRequiresExactlyOneOpenPlan(t *testing.T) {
	if !strings.Contains(outlineScript, "count != 1") {
		t.Fatal("outline script must reject ambiguous open plans")
	}
}

func TestParseProjectsAddsWindowsBaseName(t *testing.T) {
	projects, err := parseProjects([]byte(`[{"pid":12,"path":"D:\\Проекты\\План.mpp","title":"План"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "План.mpp" {
		t.Fatalf("unexpected projects: %#v", projects)
	}
}

func TestParseProjectsAcceptsSingleProject(t *testing.T) {
	projects, err := parseProjects([]byte(`{"pid":12,"path":"D:\\Проекты\\План.mpp","title":"План"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "План.mpp" {
		t.Fatalf("unexpected projects: %#v", projects)
	}
}

func TestParseProjectsRejectsEmptyPath(t *testing.T) {
	_, err := parseProjects([]byte(`[{"pid":12,"path":"","title":"План"}]`))
	if err == nil || !strings.Contains(err.Error(), "empty file path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateChangeAcceptsSupportedAssignment(t *testing.T) {
	units := 0.4
	err := validateChange(changeRequest{Kind: "assignment", Operation: "add", TaskUniqueID: 1, ResourceUniqueID: 2, Units: &units})
	if err != nil {
		t.Fatal(err)
	}
}

func TestChangeRequestAcceptsMilestone(t *testing.T) {
	milestone := true
	err := validateChange(changeRequest{Kind: "task", Operation: "create", Name: "Веха", Milestone: &milestone})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateChangeRejectsUnstableTaskReference(t *testing.T) {
	err := validateChange(changeRequest{Kind: "schedule", Operation: "update"})
	if err == nil || !strings.Contains(err.Error(), "task_unique_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkRoundingDoesNotRelaxPreservedActualWork(t *testing.T) {
	script := changeScript("e30=", true)
	for _, expected := range []string{
		"$workTolerance = 0.5",
		"$taskActualTolerance = 0.01",
		"$expectedTaskActualWork) -gt $taskActualTolerance",
		"$actualTolerance = 0.01",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("missing rounding or actual-work guard: %s", expected)
		}
	}
}

func TestChangeScriptCreatesBackupBeforeMutation(t *testing.T) {
	script := changeScript("e30=", false)
	backup := strings.Index(script, "Copy-Item -LiteralPath $source -Destination $backup")
	mutation := strings.Index(script, "switch ([string]$payload.kind")
	if backup < 0 || mutation < 0 || backup > mutation {
		t.Fatal("backup must be created before the change switch")
	}
	if !strings.Contains(script, "CalculateProject") || !strings.Contains(script, "FileSave") {
		t.Fatal("change script must recalculate and save")
	}
	if !strings.Contains(script, "[void]$projectApplication.CalculateProject()") ||
		!strings.Contains(script, "[void]$projectApplication.FileSave()") ||
		!strings.Contains(script, "[Console]::Out.Write($json)") {
		t.Fatal("change script must suppress COM output and write exactly one JSON result")
	}
}

func TestChangeScriptSkipsBackupWhenSessionAlreadyHasOne(t *testing.T) {
	script := changeScript("e30=", true)
	if strings.Contains(script, "Copy-Item -LiteralPath $source -Destination $backup") {
		t.Fatal("change script must not create a second backup in one session")
	}
	if !strings.Contains(script, "$backup = $null") {
		t.Fatal("change script must report that a backup was not created")
	}
}

func TestChangeScriptSetsOnlyCreatedTaskOutline(t *testing.T) {
	script := changeScript("e30=", true)
	if !strings.Contains(script, "$changed.OutlineLevel = [int]$parent.OutlineLevel + 1") {
		t.Fatal("new child must use its own outline property")
	}
}

func TestReparentTargetSupportsDifferentParentsAtSameLevel(t *testing.T) {
	snapshot := outlineSnapshot{Tasks: []taskSnapshot{
		{UniqueID: 45, Outline: "4.3", Level: 2},
		{UniqueID: 6, Outline: "6", Level: 1, Summary: true},
	}}
	parent, err := reparentTarget(snapshot, changeRequest{TaskUniqueID: 45, ParentUniqueID: 6})
	if err != nil || parent.UniqueID != 6 {
		t.Fatalf("unexpected target: %v, %v", parent, err)
	}
}

func TestReparentTargetRejectsInvalidHierarchy(t *testing.T) {
	cases := []struct {
		name     string
		tasks    []taskSnapshot
		parentID int
	}{
		{"missing parent", []taskSnapshot{{UniqueID: 1, Outline: "1"}}, 2},
		{"self", []taskSnapshot{{UniqueID: 1, Outline: "1"}}, 1},
		{"summary", []taskSnapshot{{UniqueID: 1, Outline: "1", Summary: true}, {UniqueID: 2, Outline: "2"}}, 2},
		{"descendant", []taskSnapshot{{UniqueID: 1, Outline: "1"}, {UniqueID: 2, Outline: "1.1"}}, 2},
		{"predecessor", []taskSnapshot{{UniqueID: 1, Outline: "1", Predecessors: "3"}, {UniqueID: 2, Outline: "2"}}, 2},
		{"successor", []taskSnapshot{{UniqueID: 1, Outline: "1", Successors: "3"}, {UniqueID: 2, Outline: "2"}}, 2},
		{"assignment", []taskSnapshot{{UniqueID: 1, Outline: "1", ResourceNames: "Исполнитель"}, {UniqueID: 2, Outline: "2"}}, 2},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := reparentTarget(outlineSnapshot{Tasks: test.tasks}, changeRequest{TaskUniqueID: 1, ParentUniqueID: test.parentID})
			if err == nil {
				t.Fatal("invalid hierarchy must be rejected")
			}
		})
	}
}

func TestValidateChangeRejectsSelfParent(t *testing.T) {
	err := validateChange(changeRequest{Kind: "task", Operation: "reparent", TaskUniqueID: 1, ParentUniqueID: 1})
	if err == nil {
		t.Fatal("self-parenting must be rejected before invoking Project")
	}
}

func TestReparentScriptMovesExactlyOneRowAndVerifiesIdentity(t *testing.T) {
	script := changeScript("e30=", true)
	for _, required := range []string{
		"SelectRow($changed.ID, $false, 0, $false, $false)",
		"$projectApplication.EditCut()",
		"$projectApplication.EditPaste()",
		"$changed.OutlineParent.UniqueID -ne $parent.UniqueID",
		"($expectedIds -join ',') -ne ($afterIds -join ',')",
		"$beforeIds.Count -ne $afterIds.Count",
		"$result.previous_task_unique_id = $previousTaskUniqueId",
		"Get-ComProperty $projectApplication 'ActiveSelection'",
		"$projectApplication.OpenUndoTransaction(",
		"$projectApplication.Undo(1)",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("missing transfer safeguard: %s", required)
		}
	}
}

func TestTextFieldsRequest(t *testing.T) {
	for _, input := range []string{
		`{"kind":"task","operation":"update","task_unique_id":13,"name":"Фундамент API и SPA","text_fields":{"Text1":"PLT-002","Text2":"Русский текст","Text30":""}}`,
		`{"kind":"task","operation":"create","name":"Новая задача","text_fields":{"Text1":""}}`,
	} {
		request, err := decodeChange(input)
		if err != nil || request.TextFields == nil {
			t.Fatalf("valid text fields: %v", err)
		}
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		var decoded changeRequest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		for key, value := range request.TextFields {
			if got, ok := decoded.TextFields[key]; !ok || got != value {
				t.Fatalf("value lost: %s", key)
			}
		}
	}
	for _, input := range []string{
		`{"kind":"task","operation":"update","task_unique_id":13,"text_fields":{"Text0":"x"}}`,
		`{"kind":"task","operation":"update","task_unique_id":13,"text_fields":{"Text31":"x"}}`,
		`{"kind":"task","operation":"update","task_unique_id":13,"text_fields":{"Text01":"x"}}`,
		`{"kind":"task","operation":"update","task_unique_id":13,"text_fields":{"Name":"x"}}`,
		`{"kind":"task","operation":"update","task_unique_id":13,"text_fields":{"Text1":12}}`,
		`{"kind":"schedule","operation":"update","task_unique_id":13,"text_fields":{"Text1":"x"}}`,
		`{"kind":"task","operation":"delete","task_unique_id":13,"text_fields":{"Text1":"x"}}`,
	} {
		if _, err := decodeChange(input); err == nil {
			t.Fatalf("invalid text fields accepted: %s", input)
		}
	}
}
func TestTextFieldsSnapshot(t *testing.T) {
	var snapshot outlineSnapshot
	err := json.Unmarshal([]byte(`{"tasks":[{"unique_id":13,"text_fields":{"Text1":"PLT-002","Text2":"тест"}}]}`), &snapshot)
	if err != nil || snapshot.Tasks[0].TextFields["Text1"] != "PLT-002" {
		t.Fatalf("snapshot: %v", err)
	}
}

func TestTaskTextBatch(t *testing.T) {
	valid := `{"kind":"task","operation":"update_many","task_updates":[{"task_unique_id":13,"name":"Основа","text_fields":{"Text1":"PLT-002"}},{"task_unique_id":14,"text_fields":{"Text30":""}}]}`
	if _, err := decodeChange(valid); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`{"kind":"task","operation":"update_many","task_updates":[]}`,
		`{"kind":"task","operation":"update_many","task_updates":[{"task_unique_id":1,"name":"x"},{"task_unique_id":1,"name":"y"}]}`,
		`{"kind":"task","operation":"update_many","work_hours":12,"task_updates":[{"task_unique_id":1,"name":"x"}]}`,
		`{"kind":"task","operation":"update_many","task_updates":[{"task_unique_id":1,"work_hours":12}]}`,
		`{"kind":"task","operation":"update_many","task_updates":[{"task_unique_id":1,"text_fields":{"Text31":"x"}}]}`,
	} {
		if _, err := decodeChange(input); err == nil {
			t.Fatalf("invalid batch accepted: %s", input)
		}
	}
}

func TestCreateChildPreservesParentBudget(t *testing.T) {
	script := changeScript("e30=", true)
	if !strings.Contains(script, "$createParentWork = [double]$createParent.Work") || !strings.Contains(script, "Создание подзадачи изменило исходные часы") {
		t.Fatal("parent budget guard missing")
	}
}

func TestWrapRequest(t *testing.T) {
	if _, err := decodeChange(`{"kind":"task","operation":"wrap","task_unique_id":17,"name":"Стенд","child_name":"Существующая работа","text_fields":{"Text1":"INF-001"}}`); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`{"kind":"task","operation":"wrap","task_unique_id":17,"name":"Стенд"}`,
		`{"kind":"task","operation":"update","task_unique_id":17,"child_name":"x"}`,
		`{"kind":"task","operation":"wrap","task_unique_id":17,"name":"Стенд","child_name":"x","work_hours":0}`,
	} {
		if _, err := decodeChange(input); err == nil {
			t.Fatalf("invalid wrap accepted: %s", input)
		}
	}
}

func TestTextFieldsRejectNull(t *testing.T) {
	for _, input := range []string{
		`{"kind":"task","operation":"update","task_unique_id":13,"text_fields":{"Text1":null}}`,
		`{"kind":"task","operation":"update","task_unique_id":13,"text_fields":null}`,
	} {
		if _, err := decodeChange(input); err == nil {
			t.Fatal("null must not silently clear text fields")
		}
	}
}

func TestDecompositionValidationAndBudget(t *testing.T) {
	input := `{"kind":"task","operation":"decompose","task_unique_id":1,"name":"Задача","child_name":"Бэкенд","expected_work_hours":160,"source_assignment_work":[{"resource_unique_id":11,"work_hours":0}],"task_children":[{"name":"Фронтенд","work_hours":40,"assignments":[{"resource_unique_id":11,"units":1,"work_hours":40}]},{"name":"Тестирование","text_fields":{"Text2":"Оценка не задана"}}]}`
	r, err := decodeChange(input)
	if err != nil {
		t.Fatal(err)
	}
	if err = validateChange(r); err != nil {
		t.Fatal(err)
	}
	s := outlineSnapshot{Tasks: []taskSnapshot{{UniqueID: 1, Name: "Задача", TaskType: 0, WorkHours: 160, Assignments: []assignmentSnapshot{{ResourceUniqueID: 9, WorkHours: 120}, {ResourceUniqueID: 11, WorkHours: 40}}}}, Resources: []resourceSnapshot{{UniqueID: 11}}}
	if _, err = checkDecomposition(s, r); err != nil {
		t.Fatal(err)
	}
	larger := 41.0
	r.TaskChildren[0].WorkHours = &larger
	if _, err = checkDecomposition(s, r); err == nil {
		t.Fatal("увеличение бюджета разрешено")
	}
	r, _ = decodeChange(input)
	s.Tasks[0].ActualWorkHours = 1
	if _, err = checkDecomposition(s, r); err == nil {
		t.Fatal("перенос фактических часов разрешён")
	}
	r.TaskChildren[1].TextFields["Text1"] = "NEW-001"
	if err = validateChange(r); err == nil {
		t.Fatal("индекс дочерней задачи разрешён")
	}
	r, _ = decodeChange(input)
	r.TaskChildren[0].Assignments[0].Units = math.NaN()
	if err = validateChange(r); err == nil {
		t.Fatal("NaN назначение разрешено")
	}
}

func TestDecompositionPowerShellParse(t *testing.T) {
	if os.Getenv("MS_PROJECT_READONLY_TEST") != "1" {
		t.Skip("требуется Windows; только проверка синтаксиса")
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(decompositionScript("e30=", false)))
	script := `$script = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('` + encoded + `'))
$tokens = $null
$parseErrors = $null
[void][System.Management.Automation.Language.Parser]::ParseInput($script,[ref]$tokens,[ref]$parseErrors)
if ($parseErrors.Count -gt 0) { throw ($parseErrors | Out-String) }
[Console]::Out.Write('{}')`
	if _, err := runPowerShell(script); err != nil {
		t.Fatal(err)
	}
}
