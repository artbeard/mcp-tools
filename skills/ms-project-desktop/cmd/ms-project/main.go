// ms-project выводит сведения об открытых планах Microsoft Project Desktop.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

type project struct {
	PID   int    `json:"pid"`
	Path  string `json:"path"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

type taskSnapshot struct {
	ID                 int                  `json:"id"`
	UniqueID           int                  `json:"unique_id"`
	Outline            string               `json:"outline"`
	Level              int                  `json:"level"`
	Summary            bool                 `json:"summary"`
	Name               string               `json:"name"`
	Manual             bool                 `json:"manual"`
	Start              string               `json:"start"`
	Finish             string               `json:"finish"`
	Predecessors       string               `json:"predecessors"`
	Successors         string               `json:"successors"`
	ResourceNames      string               `json:"resource_names"`
	Milestone          bool                 `json:"milestone"`
	TaskType           int                  `json:"task_type"`
	EffortDriven       bool                 `json:"effort_driven"`
	DurationMinutes    float64              `json:"duration_minutes"`
	WorkHours          float64              `json:"work_hours"`
	ActualWorkHours    float64              `json:"actual_work_hours"`
	RemainingWorkHours float64              `json:"remaining_work_hours"`
	Assignments        []assignmentSnapshot `json:"assignments"`
	ActualStart        string               `json:"actual_start"`
	ActualFinish       string               `json:"actual_finish"`
	Stop               string               `json:"stop"`
	Resume             string               `json:"resume"`
	RemainingDuration  float64              `json:"remaining_duration_minutes"`
	Dependencies       []dependencySnapshot `json:"dependencies"`
	TextFields         map[string]string    `json:"text_fields"`
}

type dependencySnapshot struct {
	PredecessorUniqueID int     `json:"predecessor_unique_id"`
	LinkType            int     `json:"link_type"`
	LagMinutes          float64 `json:"lag_minutes"`
}

type assignmentSnapshot struct {
	UniqueID           int     `json:"unique_id"`
	ResourceUniqueID   int     `json:"resource_unique_id"`
	ResourceName       string  `json:"resource_name"`
	Units              float64 `json:"units"`
	WorkHours          float64 `json:"work_hours"`
	ActualWorkHours    float64 `json:"actual_work_hours"`
	RemainingWorkHours float64 `json:"remaining_work_hours"`
}

type outlineSnapshot struct {
	Name      string             `json:"name"`
	Path      string             `json:"path"`
	Tasks     []taskSnapshot     `json:"tasks"`
	Resources []resourceSnapshot `json:"resources"`
}

type resourceSnapshot struct {
	UniqueID int     `json:"unique_id"`
	Name     string  `json:"name"`
	MaxUnits float64 `json:"max_units"`
}

// changeRequest intentionally exposes a small, typed command surface. Existing
// Project objects are always addressed by their stable UniqueID, never by name.
// textFieldValues не допускает null, чтобы не очищать поля неявно.
type textFieldValues map[string]string

func (values *textFieldValues) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == nil {
		return errors.New("text_fields должен быть объектом со строковыми значениями")
	}
	result := textFieldValues{}
	for name, value := range raw {
		trimmed := bytes.TrimSpace(value)
		if len(trimmed) == 0 || trimmed[0] != '"' {
			return errors.New("значения text_fields должны быть строками")
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return err
		}
		result[name] = text
	}
	*values = result
	return nil
}

type taskTextUpdate struct {
	TaskUniqueID int             `json:"task_unique_id"`
	Name         string          `json:"name,omitempty"`
	TextFields   textFieldValues `json:"text_fields,omitempty"`
}

type changeRequest struct {
	TaskChildren         []decompositionChild      `json:"task_children,omitempty"`
	SourceAssignmentWork []decompositionAssignment `json:"source_assignment_work,omitempty"`
	SourceWorkHours      *float64                  `json:"source_work_hours,omitempty"`
	ExpectedWorkHours    *float64                  `json:"expected_work_hours,omitempty"`
	ChildName            string                    `json:"child_name,omitempty"`
	TaskUpdates          []taskTextUpdate          `json:"task_updates,omitempty"`
	TextFields           textFieldValues           `json:"text_fields,omitempty"`
	Kind                 string                    `json:"kind"`
	Operation            string                    `json:"operation"`
	TaskUniqueID         int                       `json:"task_unique_id,omitempty"`
	ParentUniqueID       int                       `json:"parent_unique_id,omitempty"`
	ResourceUniqueID     int                       `json:"resource_unique_id,omitempty"`
	PredecessorUniqueID  int                       `json:"predecessor_unique_id,omitempty"`
	LinkType             *int                      `json:"link_type,omitempty"`
	Name                 string                    `json:"name,omitempty"`
	Start                string                    `json:"start,omitempty"`
	Finish               string                    `json:"finish,omitempty"`
	Duration             string                    `json:"duration,omitempty"`
	Manual               *bool                     `json:"manual,omitempty"`
	Milestone            *bool                     `json:"milestone,omitempty"`
	ConstraintType       *int                      `json:"constraint_type,omitempty"`
	ConstraintDate       string                    `json:"constraint_date,omitempty"`
	MaxUnits             *float64                  `json:"max_units,omitempty"`
	Units                *float64                  `json:"units,omitempty"`
	WorkHours            *float64                  `json:"work_hours,omitempty"`
	ActualWorkHours      *float64                  `json:"actual_work_hours,omitempty"`
	TaskType             *int                      `json:"task_type,omitempty"`
	EffortDriven         *bool                     `json:"effort_driven,omitempty"`
}

func decodeChange(input string) (changeRequest, error) {
	var request changeRequest
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return request, errors.New("ожидается один JSON-запрос")
	}
	return request, validateChange(request)
}

func main() {
	status := flag.Bool("status", false, "список открытых планов")
	outline := flag.Bool("outline", false, "структура задач единственного открытого плана")
	backup := flag.Bool("backup", false, "создать резервную копию единственного открытого плана")
	previewID := flag.Int("preview-task-unique-id", 0, "UniqueID задачи для предварительного просмотра")
	name := flag.String("name", "", "новое название")
	start := flag.String("start", "", "новая дата начала")
	finish := flag.String("finish", "", "новая дата окончания")
	duration := flag.String("duration", "", "новая длительность")
	previewChange := flag.String("preview-change", "", "JSON предварительно проверяемого изменения")
	applyChange := flag.String("apply-change", "", "JSON применяемого изменения")
	skipBackup := flag.Bool("skip-backup", false, "резервная копия уже создана для текущей сессии")
	flag.Parse()

	selected := 0
	if *status {
		selected++
	}
	if *outline {
		selected++
	}
	if *backup {
		selected++
	}
	if *previewID > 0 {
		selected++
	}
	if *previewChange != "" {
		selected++
	}
	if *applyChange != "" {
		selected++
	}
	if selected != 1 {
		fmt.Fprintln(os.Stderr, "ERROR: specify exactly one action")
		os.Exit(1)
	}
	if *previewID > 0 {
		if err := previewTaskUpdate(os.Stdout, *previewID, *name, *start, *finish, *duration); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *previewChange != "" || *applyChange != "" {
		input := *previewChange
		if input == "" {
			input = *applyChange
		}
		request, err := decodeChange(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: invalid change request: %v\n", err)
			os.Exit(1)
		}
		if *previewChange != "" {
			err = previewChangeRequest(os.Stdout, request)
		} else {
			err = applyChangeRequest(os.Stdout, request, *skipBackup)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *backup {
		if err := backupOpenPlan(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := run(os.Stdout, *outline); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func backupOpenPlan(output *os.File) error {
	script := projectPreamble() + `$ProgressPreference = 'SilentlyContinue'
$source = $project.FullName
if ([string]::IsNullOrWhiteSpace($source)) { throw 'Не удалось определить путь к открытому плану.' }
$directory = [System.IO.Path]::GetDirectoryName($source)
$name = [System.IO.Path]::GetFileNameWithoutExtension($source)
$extension = [System.IO.Path]::GetExtension($source)
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$target = Join-Path $directory ($name + '-' + $stamp + $extension)
if (Test-Path -LiteralPath $target) { throw 'Резервная копия с таким именем уже существует.' }
Copy-Item -LiteralPath $source -Destination $target -ErrorAction Stop
[PSCustomObject]@{ source = $source; backup = $target } | ConvertTo-Json -Compress`
	return writeJSON(output, script)
}

func previewTaskUpdate(output *os.File, uniqueID int, name, start, finish, duration string) error {
	raw, err := runPowerShell(outlineScript)
	if err != nil {
		return err
	}
	var snapshot outlineSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return err
	}
	var task *taskSnapshot
	for index := range snapshot.Tasks {
		if snapshot.Tasks[index].UniqueID == uniqueID {
			task = &snapshot.Tasks[index]
			break
		}
	}
	if task == nil {
		return fmt.Errorf("task UniqueID=%d not found", uniqueID)
	}
	for _, date := range []string{start, finish} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return fmt.Errorf("invalid date %q", date)
			}
		}
	}
	if start != "" && finish != "" && start > finish {
		return errors.New("start must not be after finish")
	}
	proposed := map[string]string{"name": task.Name, "start": task.Start, "finish": task.Finish}
	if name != "" {
		proposed["name"] = name
	}
	if start != "" {
		proposed["start"] = start
	}
	if finish != "" {
		proposed["finish"] = finish
	}
	if duration != "" {
		proposed["duration"] = duration
	}
	warnings := []string{}
	if !task.Manual && (start != "" || finish != "" || duration != "") {
		warnings = append(warnings, "Задача автоматически планируется: Project может пересчитать даты по связям и календарю.")
	}
	if task.Predecessors != "" {
		warnings = append(warnings, "У задачи есть предшественники: "+task.Predecessors)
	}
	result := map[string]any{"plan": map[string]string{"name": snapshot.Name, "path": snapshot.Path}, "task": task, "proposed": proposed, "warnings": warnings, "confirmation_required": true}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return writeJSONBytes(output, encoded)
}

func validateChange(request changeRequest) error {
	if request.Operation == "decompose" {
		return validateDecomposition(request)
	}
	if request.TaskChildren != nil || request.SourceAssignmentWork != nil || request.SourceWorkHours != nil || request.ExpectedWorkHours != nil {
		return errors.New("поля декомпозиции допустимы только для task:decompose")
	}
	if request.ChildName != "" && !(request.Kind == "task" && request.Operation == "wrap") {
		return errors.New("child_name допустим только для task:wrap")
	}
	if request.Kind == "task" && request.Operation == "wrap" {
		if request.Name == "" || request.ChildName == "" || request.TaskUniqueID <= 0 {
			return errors.New("task:wrap требует task_unique_id, name и child_name")
		}
		other := request
		other.Kind, other.Operation, other.Name, other.ChildName = "", "", "", ""
		other.TaskUniqueID = 0
		other.TextFields = nil
		data, _ := json.Marshal(other)
		if string(data) != `{"kind":"","operation":""}` {
			return errors.New("task:wrap принимает только task_unique_id, name, child_name и text_fields")
		}
	}

	if request.Operation == "update_many" || request.TaskUpdates != nil {
		if request.Kind != "task" || request.Operation != "update_many" || len(request.TaskUpdates) == 0 {
			return errors.New("task_updates требует task:update_many с непустым списком")
		}
		// Пакетная запись ограничена названиями и текстовыми полями.
		other := request
		other.TaskUpdates = nil
		other.Kind = ""
		other.Operation = ""
		data, _ := json.Marshal(other)
		if string(data) != `{"kind":"","operation":""}` {
			return errors.New("task:update_many принимает только task_updates")
		}
		seen := map[int]bool{}
		for _, update := range request.TaskUpdates {
			if update.TaskUniqueID <= 0 || seen[update.TaskUniqueID] || (update.Name == "" && len(update.TextFields) == 0) {
				return errors.New("task_updates: нужны уникальные положительные UniqueID и изменяемые поля")
			}
			seen[update.TaskUniqueID] = true
			if err := validateChange(changeRequest{Kind: "task", Operation: "update", TaskUniqueID: update.TaskUniqueID, Name: update.Name, TextFields: update.TextFields}); err != nil {
				return err
			}
		}
		return nil
	}
	if request.TextFields != nil {
		if request.Kind != "task" || (request.Operation != "create" && request.Operation != "update" && request.Operation != "wrap") {
			return errors.New("text_fields поддерживается для task:create, task:update и task:wrap")
		}
		for name := range request.TextFields {
			number, err := strconv.Atoi(strings.TrimPrefix(name, "Text"))
			if err != nil || number < 1 || number > 30 || name != "Text"+strconv.Itoa(number) {
				return errors.New("text_fields: допустимы только поля Text1–Text30")
			}
		}
	}
	allowed := map[string]map[string]bool{
		"task":       {"create": true, "update": true, "delete": true, "reparent": true, "outdent": true, "wrap": true},
		"schedule":   {"update": true, "link": true, "unlink": true, "timeline_add": true, "timeline_remove": true},
		"resource":   {"create": true, "update": true, "delete": true},
		"assignment": {"add": true, "remove": true, "update": true},
	}
	if !allowed[request.Kind][request.Operation] {
		return errors.New("допустимы kind: task|schedule|resource|assignment и соответствующая operation")
	}
	if request.LinkType != nil && (request.Kind != "schedule" || request.Operation != "link" || *request.LinkType < 0 || *request.LinkType > 3) {
		return errors.New("link_type допустим только для schedule:link: 0 — окончание–окончание, 1 — окончание–начало, 2 — начало–окончание, 3 — начало–начало")
	}
	if request.Kind == "schedule" && (request.Operation == "link" || request.Operation == "unlink") && request.TaskUniqueID == request.PredecessorUniqueID {
		return errors.New("задача не может зависеть от самой себя")
	}
	if request.WorkHours != nil || request.ActualWorkHours != nil || request.TaskType != nil || request.EffortDriven != nil {
		isTaskUpdate := (request.Kind == "task" || request.Kind == "schedule") && request.Operation == "update"
		isAssignmentUpdate := request.Kind == "assignment" && request.Operation == "update"
		isAssignmentAdd := request.Kind == "assignment" && request.Operation == "add"
		if !isTaskUpdate && !isAssignmentUpdate && !isAssignmentAdd {
			return errors.New("поля трудозатрат поддерживаются только для update задачи или назначения")
		}
		if (isAssignmentUpdate || isAssignmentAdd) && (request.WorkHours == nil || request.TaskType != nil || request.EffortDriven != nil) {
			return errors.New("назначение с часами требует work_hours; тип и effort_driven меняются отдельно у задачи")
		}
		if request.ActualWorkHours != nil && (!isAssignmentUpdate || request.WorkHours == nil) {
			return errors.New("actual_work_hours задаётся только вместе с work_hours для существующего назначения")
		}
		if request.WorkHours != nil {
			hours := *request.WorkHours
			if math.IsNaN(hours) || math.IsInf(hours, 0) || hours < 0 || hours > 1e7 {
				return errors.New("work_hours должен быть конечным числом от 0 до 10000000")
			}
			if math.Abs(hours*60-math.Round(hours*60)) > 1e-6 {
				return errors.New("work_hours должен задавать целое количество минут")
			}
			if request.Duration != "" || request.Start != "" || request.Finish != "" || request.ConstraintType != nil || request.ConstraintDate != "" || request.Milestone != nil {
				return errors.New("трудозатраты и даты, длительность, ограничения или веха изменяются отдельными запросами")
			}
		}
		if request.ActualWorkHours != nil {
			hours := *request.ActualWorkHours
			if math.IsNaN(hours) || math.IsInf(hours, 0) || hours < 0 || hours > 1e7 || math.Abs(hours*60-math.Round(hours*60)) > 1e-6 {
				return errors.New("actual_work_hours должен быть конечным неотрицательным числом с точностью до целой минуты")
			}
			if hours > *request.WorkHours+1e-6 {
				return errors.New("фактические трудозатраты не могут превышать общий объём")
			}
		}
		if request.TaskType != nil && (*request.TaskType < 0 || *request.TaskType > 2) {
			return errors.New("task_type: 0 — единицы, 1 — длительность, 2 — трудозатраты")
		}
		if request.TaskType != nil && *request.TaskType == 2 && request.EffortDriven != nil && !*request.EffortDriven {
			return errors.New("фиксированные трудозатраты требуют effort_driven=true")
		}
	}
	if request.Kind == "assignment" && request.Operation == "update" && request.WorkHours == nil && request.Units == nil {
		return errors.New("для assignment:update нужен work_hours или units с сохранением часов")
	}
	if unitsOnlyChange(request) && (request.Name != "" || request.Start != "" || request.Finish != "" ||
		request.Duration != "" || request.Manual != nil || request.Milestone != nil || request.ConstraintType != nil ||
		request.ConstraintDate != "" || request.ParentUniqueID != 0 || request.PredecessorUniqueID != 0 || request.MaxUnits != nil) {
		return errors.New("изменение только units нельзя совмещать с другими полями")
	}
	if request.Start != "" {
		if _, err := time.Parse("2006-01-02", request.Start); err != nil {
			return fmt.Errorf("invalid start %q", request.Start)
		}
	}
	if request.Finish != "" {
		if _, err := time.Parse("2006-01-02", request.Finish); err != nil {
			return fmt.Errorf("invalid finish %q", request.Finish)
		}
	}
	if request.Start != "" && request.Finish != "" && request.Start > request.Finish {
		return errors.New("start must not be after finish")
	}
	if (request.Kind == "task" && request.Operation != "create") || request.Kind == "schedule" || request.Kind == "assignment" {
		if request.TaskUniqueID < 1 {
			return errors.New("task_unique_id должен быть положительным")
		}
	}
	if (request.Kind == "resource" && request.Operation != "create") || request.Kind == "assignment" {
		if request.ResourceUniqueID < 1 {
			return errors.New("resource_unique_id должен быть положительным")
		}
	}
	if request.Kind == "task" && request.Operation == "create" && strings.TrimSpace(request.Name) == "" {
		return errors.New("для создания задачи нужен name")
	}
	if request.Kind == "task" && request.Operation == "reparent" && request.ParentUniqueID < 1 {
		return errors.New("для изменения родительской задачи нужен parent_unique_id")
	}
	if request.Kind == "task" && request.Operation == "reparent" && request.ParentUniqueID == request.TaskUniqueID {
		return errors.New("задача не может быть собственным родителем")
	}
	if request.Kind == "resource" && request.Operation == "create" && strings.TrimSpace(request.Name) == "" {
		return errors.New("для создания ресурса нужен name")
	}
	if (request.Kind == "schedule" && (request.Operation == "link" || request.Operation == "unlink")) && request.PredecessorUniqueID < 1 {
		return errors.New("для связи нужен predecessor_unique_id")
	}
	if request.Kind == "assignment" && request.Operation == "add" && (request.Units == nil || *request.Units <= 0) {
		return errors.New("для назначения нужен положительный units, например 1 или 0.5")
	}
	if request.Units != nil && (request.Kind != "assignment" || (request.Operation != "add" && request.Operation != "update") || math.IsNaN(*request.Units) || math.IsInf(*request.Units, 0) || *request.Units <= 0 || *request.Units > 100) {
		return errors.New("units допустим только для add/update назначения и должен быть конечным числом в (0, 100]")
	}
	if request.MaxUnits != nil && (*request.MaxUnits <= 0 || *request.MaxUnits > 100) {
		return errors.New("max_units должен быть в диапазоне (0, 100]")
	}
	return nil
}

func unitsOnlyChange(request changeRequest) bool {
	return request.Kind == "assignment" && request.Operation == "update" && request.Units != nil &&
		request.WorkHours == nil && request.ActualWorkHours == nil
}

func workTarget(snapshot outlineSnapshot, request changeRequest) (*taskSnapshot, *assignmentSnapshot, error) {
	for index := range snapshot.Tasks {
		task := &snapshot.Tasks[index]
		if task.UniqueID != request.TaskUniqueID {
			continue
		}
		if task.Summary {
			return nil, nil, errors.New("трудозатраты сводной задачи рассчитываются по дочерним задачам")
		}
		taskType := task.TaskType
		if request.TaskType != nil {
			taskType = *request.TaskType
		}
		if taskType == 2 && request.EffortDriven != nil && !*request.EffortDriven {
			return nil, nil, errors.New("фиксированные трудозатраты требуют effort_driven=true")
		}
		actual := task.ActualWorkHours
		var assignment *assignmentSnapshot
		if request.Kind == "assignment" {
			for i := range task.Assignments {
				if task.Assignments[i].ResourceUniqueID == request.ResourceUniqueID {
					if assignment != nil {
						return nil, nil, errors.New("для ресурса найдено несколько назначений")
					}
					assignment = &task.Assignments[i]
				}
			}
			if request.Operation == "add" {
				if assignment != nil {
					return nil, nil, errors.New("ресурс уже назначен; используйте update")
				}
				if taskType != 0 || task.EffortDriven {
					return nil, nil, errors.New("назначение сразу с часами требует фиксированных единиц и effort_driven=false")
				}
				actual = 0
			} else if assignment == nil {
				return nil, nil, errors.New("назначение ресурса в задаче не найдено")
			} else {
				actual = assignment.ActualWorkHours
			}
			if request.Operation == "update" && request.Units != nil && (taskType != 0 ||
				(task.ActualWorkHours > 0 && !unitsOnlyChange(request))) {
				return nil, nil, errors.New("изменение units требует фиксированных единиц; при наличии факта меняйте только units с сохранением часов")
			}
		}
		// Project округляет длительность до минут; доли занятости дают дробные минуты работы.
		if request.ActualWorkHours != nil && *request.ActualWorkHours+1.0/120 < actual {
			return nil, nil, errors.New("уменьшение ранее записанных фактических трудозатрат не поддерживается")
		}
		if request.WorkHours != nil {
			if *request.WorkHours+1e-6 < actual {
				return nil, nil, errors.New("трудозатраты не могут быть меньше уже выполненной работы")
			}
			if task.Milestone && *request.WorkHours > 0 {
				return nil, nil, errors.New("положительные трудозатраты вехи не поддерживаются")
			}
		}
		return task, assignment, nil
	}
	return nil, nil, errors.New("задача не найдена")
}

func previewChangeRequest(output *os.File, request changeRequest) error {
	if request.Operation == "decompose" {
		return previewDecomposition(output, request)
	}
	raw, err := runPowerShell(outlineScript)
	if err != nil {
		return err
	}
	var snapshot outlineSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return err
	}
	findTask := func(uniqueID int) *taskSnapshot {
		for index := range snapshot.Tasks {
			if snapshot.Tasks[index].UniqueID == uniqueID {
				return &snapshot.Tasks[index]
			}
		}
		return nil
	}
	findResource := func(uniqueID int) *resourceSnapshot {
		for index := range snapshot.Resources {
			if snapshot.Resources[index].UniqueID == uniqueID {
				return &snapshot.Resources[index]
			}
		}
		return nil
	}
	current := map[string]any{}
	if request.Operation == "update_many" {
		targets := []taskSnapshot{}
		for _, update := range request.TaskUpdates {
			task := findTask(update.TaskUniqueID)
			if task == nil {
				return fmt.Errorf("task UniqueID=%d not found", update.TaskUniqueID)
			}
			targets = append(targets, *task)
		}
		current["tasks"] = targets
	}
	if request.WorkHours != nil || request.ActualWorkHours != nil || request.TaskType != nil || request.EffortDriven != nil || unitsOnlyChange(request) {
		_, assignment, err := workTarget(snapshot, request)
		if err != nil {
			return err
		}
		if assignment != nil {
			current["assignment"] = assignment
		}
	}
	if request.Kind == "task" && request.Operation == "reparent" {
		parent, err := reparentTarget(snapshot, request)
		if err != nil {
			return err
		}
		current["parent"] = parent
	}
	if request.TaskUniqueID > 0 {
		task := findTask(request.TaskUniqueID)
		if task == nil {
			return fmt.Errorf("task UniqueID=%d not found", request.TaskUniqueID)
		}
		if request.Operation == "wrap" && task.Summary {
			return errors.New("task:wrap поддерживает только несводные задачи")
		}
		current["task"] = task
	}
	if request.ParentUniqueID > 0 && findTask(request.ParentUniqueID) == nil {
		return fmt.Errorf("parent task UniqueID=%d not found", request.ParentUniqueID)
	}
	if request.PredecessorUniqueID > 0 && findTask(request.PredecessorUniqueID) == nil {
		return fmt.Errorf("predecessor task UniqueID=%d not found", request.PredecessorUniqueID)
	}
	if request.ResourceUniqueID > 0 {
		resource := findResource(request.ResourceUniqueID)
		if resource == nil {
			return fmt.Errorf("resource UniqueID=%d not found", request.ResourceUniqueID)
		}
		current["resource"] = resource
	}
	warnings := []string{"Изменение ещё не внесено. Для записи нужен отдельный вызов с confirmed=true после явного подтверждения."}
	if unitsOnlyChange(request) {
		warnings = append(warnings, "Меняются только проценты назначения. Общие и фактические часы, фактические даты и остальные назначения сохраняются; нарушение проверки вызывает отмену без сохранения. Предварительный просмотр не моделирует новые даты.")
	}
	if request.ActualWorkHours != nil {
		warnings = append(warnings, "Фактические часы назначения будут явно изменены; остаток равен work_hours минус actual_work_hours. Даты и распределение факта по дням рассчитывает Project, они не являются подтверждённым табелем.")
	}
	if request.WorkHours != nil {
		task, _, _ := workTarget(snapshot, request)
		taskType := task.TaskType
		if request.TaskType != nil {
			taskType = *request.TaskType
		}
		if task.Manual && (request.Manual == nil || *request.Manual) {
			warnings = append(warnings, "Ручное планирование: ввод часов не гарантирует пересчёт длительности.")
		}
		if taskType == 1 {
			warnings = append(warnings, "Фиксированная длительность: Project пересчитает единицы назначений вместо длительности.")
		}
		if len(task.Assignments) == 0 {
			warnings = append(warnings, "Исполнители не назначены: часы сохраняются, длительность нельзя определить по загрузке.")
		}
		warnings = append(warnings, "Предварительный просмотр не моделирует пересчёт Project; фактические даты и часы возвращаются после применения.")
	}
	if request.Kind == "schedule" || request.Kind == "assignment" {
		warnings = append(warnings, "Microsoft Project пересчитает связанные даты и загрузку ресурсов.")
	}
	if request.Kind == "task" && request.Operation == "reparent" {
		warnings = append(warnings, "Задача будет перенесена первой дочерней строкой выбранного родителя. Project может назначить новый UniqueID, который возвращается в результате. Сводные даты этапов пересчитываются. Поддерживаются только несводные задачи без связей и назначений ресурсов.")
	}
	if request.Operation == "delete" || request.Operation == "remove" || request.Operation == "unlink" {
		warnings = append(warnings, "Операция удаляет данные и требует отдельного подтверждения.")
	}
	result := map[string]any{"plan": map[string]string{"name": snapshot.Name, "path": snapshot.Path}, "request": request, "current": current, "warnings": warnings, "confirmation_required": true}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return writeJSONBytes(output, encoded)
}

func applyChangeRequest(output *os.File, request changeRequest, skipBackup bool) error {
	if request.Operation == "decompose" {
		return applyDecomposition(output, request, skipBackup)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(payload)
	return writeJSON(output, changeScript(encoded, skipBackup))
}

func reparentTarget(snapshot outlineSnapshot, request changeRequest) (*taskSnapshot, error) {
	var task, parent *taskSnapshot
	for index := range snapshot.Tasks {
		item := &snapshot.Tasks[index]
		if item.UniqueID == request.TaskUniqueID {
			task = item
		}
		if item.UniqueID == request.ParentUniqueID {
			parent = item
		}
	}
	if task == nil || parent == nil {
		return nil, errors.New("задача или целевой родитель не найдены")
	}
	if task.UniqueID == parent.UniqueID {
		return nil, errors.New("задача не может быть собственным родителем")
	}
	if task.Summary {
		return nil, errors.New("перенос сводной задачи пока не поддерживается")
	}
	if task.Predecessors != "" || task.Successors != "" || task.ResourceNames != "" {
		return nil, errors.New("перенос задачи со связями или назначениями ресурсов пока не поддерживается")
	}
	if strings.HasPrefix(parent.Outline, task.Outline+".") {
		return nil, errors.New("нельзя перенести задачу внутрь её потомка")
	}
	return parent, nil
}

func run(output *os.File, outline bool) error {
	if outline {
		return writeJSON(output, outlineScript)
	}

	raw, err := runPowerShell(statusScript)
	if err != nil {
		return err
	}
	projects, err := parseProjects(raw)
	if err != nil {
		return err
	}
	// Process command lines retain the old path after Save As. Resolve the live
	// document via COM before the server chooses a backup/session identity.
	if len(projects) == 1 {
		raw, err := runPowerShell(projectPreamble() + `[PSCustomObject]@{ path = $project.FullName; name = $project.Name } | ConvertTo-Json -Compress`)
		if err != nil {
			return err
		}
		var live outlineSnapshot
		if err := json.Unmarshal(raw, &live); err != nil {
			return err
		}
		if strings.TrimSpace(live.Path) == "" {
			return errors.New("Microsoft Project returned an empty live file path")
		}
		projects[0].Path = live.Path
		projects[0].Name = live.Name
	}
	encoded, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	return writeJSONBytes(output, encoded)
}

func writeJSON(output *os.File, script string) error {
	raw, err := runPowerShell(script)
	if err != nil {
		return err
	}
	return writeJSONBytes(output, raw)
}

func writeJSONBytes(output *os.File, raw []byte) error {
	if !json.Valid(raw) {
		return errors.New("PowerShell returned invalid JSON")
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return err
	}
	_, err := fmt.Fprintln(output, pretty.String())
	return err
}

func runPowerShell(script string) ([]byte, error) {
	command := powerShellCommand(script)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Windows PowerShell: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func powerShellCommand(script string) *exec.Cmd {
	// WSL interop limits command-line size; pass script data through UTF-8 stdin.
	bootstrap := "[Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false); $script = [Console]::In.ReadToEnd(); & ([scriptblock]::Create($script))"
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePowerShell(bootstrap))
	command.Stdin = strings.NewReader(script)
	return command
}

func encodePowerShell(script string) string {
	codeUnits := utf16.Encode([]rune(script))
	bytes := make([]byte, len(codeUnits)*2)
	for index, codeUnit := range codeUnits {
		bytes[index*2] = byte(codeUnit)
		bytes[index*2+1] = byte(codeUnit >> 8)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func parseProjects(raw []byte) ([]project, error) {
	var projects []project
	if err := json.Unmarshal(raw, &projects); err != nil {
		var single project
		if singleErr := json.Unmarshal(raw, &single); singleErr != nil {
			return nil, fmt.Errorf("PowerShell returned invalid JSON: %w", err)
		}
		projects = []project{single}
	}
	for index := range projects {
		if projects[index].Path == "" {
			return nil, errors.New("Microsoft Project returned an empty file path")
		}
		projects[index].Name = windowsBaseName(projects[index].Path)
	}
	return projects, nil
}

func windowsBaseName(path string) string {
	if index := strings.LastIndex(path, "\\"); index >= 0 {
		return path[index+1:]
	}
	return path
}

const statusScript = `$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [Console]::OutputEncoding
$projects = @(
    Get-CimInstance Win32_Process |
        Where-Object { $_.Name -eq 'WINPROJ.EXE' } |
        ForEach-Object {
            $path = ''
            if ($_.CommandLine -match '"([^"]+\.mpp)"') {
                $path = $Matches[1]
            }
            if ($path -ne '') {
                [PSCustomObject]@{
                    pid = [int]$_.ProcessId
                    path = $path
                    title = (Get-Process -Id $_.ProcessId).MainWindowTitle
                }
            }
        }
)
if ($projects.Count -eq 0) {
    [Console]::Out.Write('[]')
} else {
    [Console]::Out.Write(($projects | ConvertTo-Json -Compress))
}`

const outlineScript = `$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [Console]::OutputEncoding
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Runtime.InteropServices.ComTypes;

public static class ProjectRot
{
    [DllImport("ole32.dll")]
    private static extern int GetRunningObjectTable(int reserved, out IRunningObjectTable runningObjectTable);

    [DllImport("ole32.dll")]
    private static extern int CreateBindCtx(int reserved, out IBindCtx bindContext);

    public static object FindOpenProject()
    {
        IRunningObjectTable rot;
        IBindCtx bindContext;
        GetRunningObjectTable(0, out rot);
        CreateBindCtx(0, out bindContext);
        IEnumMoniker enumerator;
        rot.EnumRunning(out enumerator);
        var monikers = new IMoniker[1];
        object project = null;
        var count = 0;
        while (enumerator.Next(1, monikers, IntPtr.Zero) == 0)
        {
            string name;
            monikers[0].GetDisplayName(bindContext, null, out name);
            object candidate;
            rot.GetObject(monikers[0], out candidate);
            try
            {
                var fullName = candidate.GetType().InvokeMember("FullName",
                    System.Reflection.BindingFlags.GetProperty, null, candidate, null) as string;
                if (!String.IsNullOrEmpty(fullName) && fullName.EndsWith(".mpp", StringComparison.OrdinalIgnoreCase))
                {
                    project = candidate;
                    count++;
                }
            }
            catch { }
        }
        if (count != 1)
        {
            throw new InvalidOperationException("Для чтения outline должен быть открыт ровно один .mpp.");
        }
        return project;
    }
}
'@
$project = [ProjectRot]::FindOpenProject()
$projectApplication = $project.Application
if ($null -eq $projectApplication) { throw 'MS Project временно недоступен для автоматизации: завершите активный ввод или закройте диалог.' }
function Format-ProjectDate($value) {
    if ($null -eq $value) { return '' }
    if ($value -is [datetime]) { return $value.ToString('yyyy-MM-dd') }
    return [string]$value
}
$tasks = @(
    $project.Tasks |
        Where-Object { $null -ne $_ } |
        ForEach-Object {
            $task = $_
            $textFields = @{}
            for ($fieldNumber = 1; $fieldNumber -le 30; $fieldNumber++) {
                $fieldName = 'Text' + $fieldNumber
                $value = [string]$task.$fieldName
                if ($value -ne '') { $textFields[$fieldName] = $value }
            }
            [PSCustomObject]@{
                text_fields = $textFields
				id = $_.ID
				unique_id = $_.UniqueID
                outline = $_.OutlineNumber
                level = $_.OutlineLevel
                summary = $_.Summary
                milestone = $_.Milestone
                task_type = [int]$_.Type
                effort_driven = $_.EffortDriven
                duration_minutes = [double]$_.Duration
                work_hours = [double]$_.Work / 60.0
                actual_work_hours = [double]$_.ActualWork / 60.0
                remaining_work_hours = [double]$_.RemainingWork / 60.0
                actual_start = [string]$task.ActualStart
                actual_finish = [string]$task.ActualFinish
                stop = [string]$task.Stop
                resume = [string]$task.Resume
                remaining_duration_minutes = [double]$task.RemainingDuration
                dependencies = @($task.TaskDependencies | Where-Object { $_.To.UniqueID -eq $task.UniqueID } | ForEach-Object {
                    [PSCustomObject]@{
                        predecessor_unique_id = $_.From.UniqueID
                        link_type = [int]$_.Type
                        lag_minutes = [double]$_.Lag
                    }
                })
                assignments = @($task.Assignments | Where-Object { $null -ne $_ } | ForEach-Object {
                    [PSCustomObject]@{
                        unique_id = $_.UniqueID
                        resource_unique_id = $_.Resource.UniqueID
                        resource_name = $_.Resource.Name
                        units = [double]$_.Units
                        work_hours = [double]$_.Work / 60.0
                        actual_work_hours = [double]$_.ActualWork / 60.0
                        remaining_work_hours = [double]$_.RemainingWork / 60.0
                    }
                })
                name = $_.Name
				manual = $_.Manual
				constraint_type = $_.ConstraintType
				constraint_date = [string]$_.ConstraintDate
				predecessors = $_.Predecessors
				successors = $_.Successors
				resource_names = $_.ResourceNames
                start = Format-ProjectDate $task.Start
                finish = Format-ProjectDate $task.Finish
            }
        }
)
$resources = @(
    $project.Resources |
        Where-Object { $null -ne $_ } |
        ForEach-Object {
            [PSCustomObject]@{
                id = $_.ID
                unique_id = $_.UniqueID
                name = $_.Name
                type = $_.Type
                max_units = $_.MaxUnits
                standard_rate = [string]$_.StandardRate
                overallocated = $_.Overallocated
            }
        }
)
[PSCustomObject]@{
    name = $project.Name
    path = $project.FullName
    task_count = $tasks.Count
    tasks = $tasks
    resource_count = $resources.Count
    resources = $resources
} | ConvertTo-Json -Depth 6 -Compress`

func projectPreamble() string {
	return strings.Split(outlineScript, "$tasks = @(")[0]
}

func changeScript(encodedRequest string, skipBackup bool) string {
	preamble := projectPreamble()
	backupScript := `$backup = $null`
	if !skipBackup {
		backupScript = `$ProgressPreference = 'SilentlyContinue'
$directory = [System.IO.Path]::GetDirectoryName($source)
$baseName = [System.IO.Path]::GetFileNameWithoutExtension($source)
$extension = [System.IO.Path]::GetExtension($source)
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$backup = Join-Path $directory ($baseName + '-' + $stamp + $extension)
if (Test-Path -LiteralPath $backup) { throw 'Резервная копия с таким именем уже существует.' }
Copy-Item -LiteralPath $source -Destination $backup -ErrorAction Stop`
	}
	return preamble + `$payloadJson = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('` + encodedRequest + `'))
$payload = $payloadJson | ConvertFrom-Json
function Has-Field([string]$name) { return $null -ne $payload.PSObject.Properties[$name] }
function Get-ComProperty($item, [string]$name) {
    return ,$item.GetType().InvokeMember($name, [System.Reflection.BindingFlags]::GetProperty, $null, $item, $null)
}
function Get-TaskByUniqueId([int]$uniqueId) {
    $item = @($project.Tasks | Where-Object { $null -ne $_ -and $_.UniqueID -eq $uniqueId })
    if ($item.Count -ne 1) { throw "Задача UniqueID=$uniqueId не найдена." }
    return $item[0]
}
function Get-ResourceByUniqueId([int]$uniqueId) {
    $item = @($project.Resources | Where-Object { $null -ne $_ -and $_.UniqueID -eq $uniqueId })
    if ($item.Count -ne 1) { throw "Ресурс UniqueID=$uniqueId не найден." }
    return $item[0]
}
function Set-TaskFields($task) {
    if (Has-Field 'text_fields') {
        foreach ($field in $payload.text_fields.PSObject.Properties) {
            [void]$task.GetType().InvokeMember($field.Name, [System.Reflection.BindingFlags]::SetProperty, $null, $task, @([string]$field.Value))
        }
    }
    if (Has-Field 'name') { $task.Name = [string]$payload.name }
    if (Has-Field 'manual') { $task.Manual = [bool]$payload.manual }
	if (Has-Field 'milestone') { $task.Milestone = [bool]$payload.milestone }
    if (Has-Field 'task_type') { $task.Type = [int]$payload.task_type }
    if (Has-Field 'effort_driven') { $task.EffortDriven = [bool]$payload.effort_driven }
    if (Has-Field 'work_hours') { $task.Work = [double]$payload.work_hours * 60.0 }
    if (Has-Field 'duration') { $task.Duration = [string]$payload.duration }
    if (Has-Field 'start') { $task.Start = [datetime]::ParseExact([string]$payload.start, 'yyyy-MM-dd', $null) }
    if (Has-Field 'finish') { $task.Finish = [datetime]::ParseExact([string]$payload.finish, 'yyyy-MM-dd', $null) }
    if (Has-Field 'constraint_type') { $task.ConstraintType = [int]$payload.constraint_type }
    if (Has-Field 'constraint_date') { $task.ConstraintDate = [datetime]::ParseExact([string]$payload.constraint_date, 'yyyy-MM-dd', $null) }
}
function Get-Assignment($task, [int]$resourceUniqueId) {
    $items = @($task.Assignments | Where-Object { $null -ne $_ -and $_.Resource.UniqueID -eq $resourceUniqueId })
    if ($items.Count -ne 1) { throw 'Для задачи и ресурса не найдено ровно одно назначение.' }
    return $items[0]
}
$unitsOnlyEdit = $payload.kind -eq 'assignment' -and $payload.operation -eq 'update' -and (Has-Field 'units') -and -not (Has-Field 'work_hours') -and -not (Has-Field 'actual_work_hours')
$workEdit = (Has-Field 'work_hours') -or (Has-Field 'actual_work_hours') -or (Has-Field 'task_type') -or (Has-Field 'effort_driven') -or $unitsOnlyEdit
$wrapEdit = $payload.kind -eq 'task' -and $payload.operation -eq 'wrap'
$batchTextEdit = $payload.kind -eq 'task' -and $payload.operation -eq 'update_many'
$structureEdit = $wrapEdit -or ($payload.kind -eq 'task' -and $payload.operation -eq 'create')
$textEdit = (Has-Field 'text_fields') -or $batchTextEdit -or $structureEdit
$dependencyEdit = $payload.kind -eq 'schedule' -and $payload.operation -in @('link', 'unlink')
$dependencyProgress = @{}
if ($dependencyEdit -or $unitsOnlyEdit -or $batchTextEdit -or $wrapEdit) {
    foreach ($item in $project.Tasks) {
        if ($null -ne $item -and -not $item.Summary) {
            $assignmentProgress = @{}
            foreach ($a in $item.Assignments) {
                $assignmentProgress[[int]$a.UniqueID] = @{
                    work = [double]$a.Work; actual = [double]$a.ActualWork; units = [double]$a.Units
                }
            }
            $dependencyProgress[[int]$item.UniqueID] = @{
                work = [double]$item.Work; actual = [double]$item.ActualWork
                actual_start = [string]$item.ActualStart; actual_finish = [string]$item.ActualFinish
                assignments = $assignmentProgress
            }
        }
    }
}
$workTask = $null
$originalUnits = @{}
$originalActualWork = @{}
$originalAssignmentWork = @{}
$originalActualStart = ''
$originalActualFinish = ''
$originalTaskActualWork = 0
$expectedTaskActualWork = 0
if ($workEdit) {
    $workTask = Get-TaskByUniqueId([int]$payload.task_unique_id)
    if ($workTask.Summary) { throw 'Трудозатраты сводной задачи рассчитываются по дочерним задачам.' }
    $workItem = $workTask
    if ($payload.kind -eq 'assignment' -and $payload.operation -eq 'add') {
        if ([int]$workTask.Type -ne 0 -or $workTask.EffortDriven) { throw 'Назначение с часами требует фиксированных единиц и effort_driven=false.' }
        $duplicates = @($workTask.Assignments | Where-Object { $_.Resource.UniqueID -eq [int]$payload.resource_unique_id })
        if ($duplicates.Count -ne 0) { throw 'Ресурс уже назначен.' }
        $workItem = $null
    } elseif ($payload.kind -eq 'assignment') { $workItem = Get-Assignment $workTask ([int]$payload.resource_unique_id) }
    if (Has-Field 'work_hours') {
        $minutes = [double]$payload.work_hours * 60.0
        if ($null -ne $workItem -and $minutes + 0.00001 -lt [double]$workItem.ActualWork) { throw 'Трудозатраты меньше уже выполненной работы.' }
        if ($workTask.Milestone -and $minutes -gt 0) { throw 'Положительные трудозатраты вехи не поддерживаются.' }
    }
    if (Has-Field 'actual_work_hours') {
        if ($payload.kind -ne 'assignment' -or -not (Has-Field 'work_hours')) { throw 'Факт поддерживается только вместе с общим объёмом существующего назначения.' }
        $actualMinutes = [double]$payload.actual_work_hours * 60.0
        if ($actualMinutes + 0.5 -lt [double]$workItem.ActualWork) { throw 'Уменьшение ранее записанного факта не поддерживается.' }
        if ($actualMinutes -gt [double]$payload.work_hours * 60.0 + 0.00001) { throw 'Факт превышает общий объём.' }
    }
    $effectiveType = [int]$workTask.Type
    if (Has-Field 'task_type') { $effectiveType = [int]$payload.task_type }
    if ($effectiveType -eq 2 -and (Has-Field 'effort_driven') -and -not $payload.effort_driven) {
        throw 'Фиксированные трудозатраты требуют effort_driven=true.'
    }
    $originalTaskActualWork = [double]$workTask.ActualWork
    $originalActualStart = [string]$workTask.ActualStart
    $originalActualFinish = [string]$workTask.ActualFinish
    if ($payload.kind -eq 'assignment' -and $payload.operation -eq 'update' -and (Has-Field 'units') -and
        ([int]$workTask.Type -ne 0 -or ($originalTaskActualWork -gt 0 -and -not $unitsOnlyEdit))) {
        throw 'Изменение units требует фиксированных единиц; при наличии факта меняйте только units.'
    }
    $expectedTaskActualWork = $originalTaskActualWork
    if (Has-Field 'actual_work_hours') {
        $expectedTaskActualWork += [double]$payload.actual_work_hours * 60.0 - [double]$workItem.ActualWork
    }
    foreach ($assignment in $workTask.Assignments) {
        $originalUnits[[int]$assignment.UniqueID] = [double]$assignment.Units
        $originalActualWork[[int]$assignment.UniqueID] = [double]$assignment.ActualWork
        $originalAssignmentWork[[int]$assignment.UniqueID] = [double]$assignment.Work
    }
}
$createParent = $null
if ($payload.kind -eq 'task' -and $payload.operation -eq 'create' -and (Has-Field 'parent_unique_id')) {
    $createParent = Get-TaskByUniqueId([int]$payload.parent_unique_id)
    $createParentWork = [double]$createParent.Work
    $createParentActual = [double]$createParent.ActualWork
}
$source = $project.FullName
if ([string]::IsNullOrWhiteSpace($source)) { throw 'Не удалось определить путь к открытому плану.' }
` + backupScript + `

$changed = $null
$previousTaskUniqueId = $null
$workUndoOpen = $false
$workMutated = $false
try {
if ($workEdit -or $dependencyEdit -or $textEdit) {
    $projectApplication.OpenUndoTransaction('Изменение трудозатрат')
    $workUndoOpen = $true
    $workMutated = $true
}
switch ([string]$payload.kind + ':' + [string]$payload.operation) {
    'resource:create' {
        $changed = $project.Resources.Add([string]$payload.name)
        if (Has-Field 'max_units') { $changed.MaxUnits = [double]$payload.max_units }
    }
    'resource:update' {
        $changed = Get-ResourceByUniqueId([int]$payload.resource_unique_id)
        if (Has-Field 'name') { $changed.Name = [string]$payload.name }
        if (Has-Field 'max_units') { $changed.MaxUnits = [double]$payload.max_units }
    }
    'resource:delete' {
        $changed = Get-ResourceByUniqueId([int]$payload.resource_unique_id)
        [void]$changed.Delete()
    }
    'task:create' {
        if (Has-Field 'parent_unique_id') {
            $parent = Get-TaskByUniqueId([int]$payload.parent_unique_id)
            $changed = $project.Tasks.Add([string]$payload.name, [int]$parent.ID + 1)
            # Задаём уровень созданного объекта напрямую: выделение видимой строки
            # может затронуть соседнюю задачу в свёрнутом представлении.
            $changed.OutlineLevel = [int]$parent.OutlineLevel + 1
        } else { $changed = $project.Tasks.Add([string]$payload.name) }
        Set-TaskFields $changed
    }
    'task:wrap' {
        $original = Get-TaskByUniqueId([int]$payload.task_unique_id)
        if ($original.Summary) { throw 'Нельзя обернуть сводную задачу.' }
        $oldLevel = [int]$original.OutlineLevel
        $originalWork = [double]$original.Work
        $originalActual = [double]$original.ActualWork
        $changed = $project.Tasks.Add([string]$payload.name, [int]$original.ID)
        $changed.OutlineLevel = $oldLevel
        $original.OutlineLevel = $oldLevel + 1
        $original.Name = [string]$payload.child_name
        $changed.Text1 = [string]$original.Text1
        $original.Text1 = ''
        Set-TaskFields $changed
        if ($original.OutlineParent.UniqueID -ne $changed.UniqueID -or
            [math]::Abs([double]$changed.Work - $originalWork) -gt 0.01 -or
            [math]::Abs([double]$changed.ActualWork - $originalActual) -gt 0.01) { throw 'Обёртка изменила часы или иерархию.' }
    }
    'task:update_many' {
        $batchPayload = $payload
        $batchResults = @()
        foreach ($update in $batchPayload.task_updates) {
            $payload = $update
            $changed = Get-TaskByUniqueId([int]$update.task_unique_id)
            Set-TaskFields $changed
            foreach ($field in $update.text_fields.PSObject.Properties) {
                if ([string](Get-ComProperty $changed $field.Name) -cne [string]$field.Value) { throw ('Project не сохранил поле ' + $field.Name) }
            }
            if ((Has-Field 'name') -and [string]$changed.Name -cne [string]$update.name) { throw 'Project не сохранил название.' }
            $batchResults += [PSCustomObject]@{ task_unique_id = $changed.UniqueID; name = $changed.Name; text_fields = $update.text_fields }
        }
        $payload = $batchPayload
    }
    'task:update' { $changed = Get-TaskByUniqueId([int]$payload.task_unique_id); Set-TaskFields $changed }
    'task:delete' { $changed = Get-TaskByUniqueId([int]$payload.task_unique_id); [void]$changed.Delete() }
    'task:reparent' {
        $changed = Get-TaskByUniqueId([int]$payload.task_unique_id)
        $parent = Get-TaskByUniqueId([int]$payload.parent_unique_id)
        if ($changed.UniqueID -eq $parent.UniqueID) { throw 'Задача не может быть собственным родителем.' }
        if ($changed.Summary) { throw 'Перенос сводной задачи пока не поддерживается.' }
        if ($changed.Predecessors -ne '' -or $changed.Successors -ne '' -or $changed.Assignments.Count -gt 0) {
            throw 'Перенос задачи со связями или назначениями ресурсов пока не поддерживается.'
        }
        if ([string]$parent.OutlineNumber -like ([string]$changed.OutlineNumber + '.*')) {
            throw 'Нельзя перенести задачу внутрь её потомка.'
        }
        if ($changed.OutlineParent.UniqueID -ne $parent.UniqueID) {
            $beforeIds = @($project.Tasks | Where-Object { $null -ne $_ } | ForEach-Object { $_.UniqueID } | Sort-Object)
            $original = @{
                name = [string]$changed.Name
                manual = $changed.Manual
                start = [string]$changed.Start
                finish = [string]$changed.Finish
                duration = [string]$changed.Duration
                constraint_type = $changed.ConstraintType
                constraint_date = [string]$changed.ConstraintDate
            }
            # Verify the selected UniqueID before allowing a row operation.
            $pjViewGantt = 4
            if (-not $projectApplication.ViewApplyEx([Type]::Missing, $true, $false, 0, $pjViewGantt)) {
                throw 'Не удалось открыть таблицу задач для переноса.'
            }
            [void]$projectApplication.FilterClear()
            [void]$projectApplication.OutlineShowAllTasks()
            $pjTaskName = 188743694
            $nameField = $projectApplication.FieldConstantToFieldName($pjTaskName)
            [void]$projectApplication.SelectTaskField($changed.ID, $nameField, $false)
            if (-not $projectApplication.SelectRow($changed.ID, $false, 0, $false, $false)) {
                throw 'Не удалось выделить переносимую задачу.'
            }
            $selection = Get-ComProperty $projectApplication 'ActiveSelection'
            $selected = Get-ComProperty $selection 'Tasks'
            if ($selected.Count -ne 1 -or $selected.Item(1).UniqueID -ne $changed.UniqueID) {
                throw "Текущее представление не позволяет однозначно выделить задачу: выбрано $($selected.Count), активная задача=$($projectApplication.ActiveCell.Task.UniqueID), представление=$($project.CurrentView), ожидается UniqueID=$($changed.UniqueID)."
            }
            $undoOpen = $false
            $mutated = $false
            try {
                $projectApplication.OpenUndoTransaction('Перенос задачи к другому родителю')
                $undoOpen = $true
                if (-not $projectApplication.EditCut()) { throw 'Не удалось вырезать строку задачи.' }
                $mutated = $true
                # Resolve the parent again: removing a row renumbers subsequent task IDs.
                $parent = Get-TaskByUniqueId([int]$payload.parent_unique_id)
                $destination = [int]$parent.ID + 1
                [void]$projectApplication.SelectTaskField($destination, $nameField, $false)
                if (-not $projectApplication.SelectRow($destination, $false, 0, $false, $false)) {
                    throw 'Не удалось выделить место вставки.'
                }
                $nextTask = @($project.Tasks | Where-Object { $null -ne $_ -and $_.ID -eq $destination })
                if ($nextTask.Count -eq 1) {
                    $selection = Get-ComProperty $projectApplication 'ActiveSelection'
                    $selected = Get-ComProperty $selection 'Tasks'
                    if ($selected.Count -ne 1 -or $selected.Item(1).UniqueID -ne $nextTask[0].UniqueID) {
                        throw 'Текущее представление изменяет место вставки задачи.'
                    }
                }
                if (-not $projectApplication.EditPaste()) { throw 'Не удалось вставить строку задачи.' }
                # Project may allocate a new UniqueID when pasting the cut row.
                $inserted = @($project.Tasks | Where-Object { $null -ne $_ -and $_.ID -eq $destination })
                if ($inserted.Count -ne 1) { throw 'После вставки не найдена ровно одна задача.' }
                $changed = $inserted[0]
                $parent = Get-TaskByUniqueId([int]$payload.parent_unique_id)
                $changed.OutlineLevel = [int]$parent.OutlineLevel + 1
                if ($changed.OutlineParent.UniqueID -ne $parent.UniqueID) {
                    throw 'После переноса родитель задачи не соответствует запросу.'
                }
                $afterIds = @($project.Tasks | Where-Object { $null -ne $_ } | ForEach-Object { $_.UniqueID } | Sort-Object)
                $expectedIds = @(@($beforeIds | Where-Object { $_ -ne [int]$payload.task_unique_id }) + @($changed.UniqueID) | Sort-Object)
                if (($expectedIds -join ',') -ne ($afterIds -join ',') -or $beforeIds.Count -ne $afterIds.Count) {
                    throw 'При переносе изменились другие задачи или количество строк.'
                }
                $previousTaskUniqueId = [int]$payload.task_unique_id
                [void]$projectApplication.CalculateProject()
                if ([string]$changed.Name -ne $original.name -or $changed.Manual -ne $original.manual -or
                    [string]$changed.Start -ne $original.start -or [string]$changed.Finish -ne $original.finish -or
                    [string]$changed.Duration -ne $original.duration -or $changed.ConstraintType -ne $original.constraint_type -or
                    [string]$changed.ConstraintDate -ne $original.constraint_date) {
                    throw 'При переносе изменились параметры задачи; операция отменяется.'
                }
                $projectApplication.CloseUndoTransaction()
                $undoOpen = $false
            } catch {
                $failure = $_
                if ($undoOpen) { $projectApplication.CloseUndoTransaction() }
                if ($mutated) {
                    try {
                        $undone = $projectApplication.Undo(1)
                        if (-not $undone) { throw 'Microsoft Project не подтвердил отмену.' }
                    } catch {
                        throw "Ошибка переноса: $failure. Автоматическая отмена не выполнена: $_. План не сохранён; резервная копия: $backup."
                    }
                }
                throw $failure
            }
        }
    }
    'task:outdent' {
        $changed = Get-TaskByUniqueId([int]$payload.task_unique_id)
        [void]$projectApplication.SelectRow($changed.ID, $false, 1, $false, $false)
        [void]$projectApplication.OutlineOutdent(1)
    }
    'schedule:update' { $changed = Get-TaskByUniqueId([int]$payload.task_unique_id); Set-TaskFields $changed }
    'schedule:link' {
        $changed = Get-TaskByUniqueId([int]$payload.task_unique_id)
        $predecessor = Get-TaskByUniqueId([int]$payload.predecessor_unique_id)
        $linkType = 1
        if (Has-Field 'link_type') { $linkType = [int]$payload.link_type }
        $changed.TaskDependencies.Add($predecessor, $linkType, 0) | Out-Null
    }
    'schedule:unlink' {
        $changed = Get-TaskByUniqueId([int]$payload.task_unique_id)
        $predecessor = Get-TaskByUniqueId([int]$payload.predecessor_unique_id)
        @($changed.TaskDependencies | Where-Object { $_.From.UniqueID -eq $predecessor.UniqueID }) | ForEach-Object { [void]$_.Delete() }
    }
    'schedule:timeline_add' { $changed = Get-TaskByUniqueId([int]$payload.task_unique_id); $projectApplication.TaskOnTimeline($changed.ID, $false) | Out-Null }
    'schedule:timeline_remove' { $changed = Get-TaskByUniqueId([int]$payload.task_unique_id); $projectApplication.TaskOnTimeline($changed.ID, $true) | Out-Null }
    'assignment:add' {
        $task = Get-TaskByUniqueId([int]$payload.task_unique_id)
        $resource = Get-ResourceByUniqueId([int]$payload.resource_unique_id)
        $changed = $task.Assignments.Add($task.ID, $resource.ID, [double]$payload.units)
        if (Has-Field 'work_hours') {
            $changed.Work = [double]$payload.work_hours * 60.0
            $originalUnits[[int]$changed.UniqueID] = [double]$payload.units
            $originalActualWork[[int]$changed.UniqueID] = 0.0
        }
    }
    'assignment:remove' {
        $task = Get-TaskByUniqueId([int]$payload.task_unique_id)
        $resource = Get-ResourceByUniqueId([int]$payload.resource_unique_id)
        $changed = @($task.Assignments | Where-Object { $null -ne $_ -and $_.Resource.UniqueID -eq $resource.UniqueID })
        if ($changed.Count -ne 1) { throw 'Для задачи и ресурса не найдено ровно одно назначение.' }
        [void]$changed[0].Delete()
    }
    'assignment:update' {
        $changed = Get-Assignment $workTask ([int]$payload.resource_unique_id)
        if (Has-Field 'units') {
            $changed.Units = [double]$payload.units
            $originalUnits[[int]$changed.UniqueID] = [double]$payload.units
        }
        if ($unitsOnlyEdit) {
            $changed.RemainingWork = $originalAssignmentWork[[int]$changed.UniqueID] - $originalActualWork[[int]$changed.UniqueID]
        } else { $changed.Work = [double]$payload.work_hours * 60.0 }
        if (Has-Field 'actual_work_hours') { $changed.ActualWork = [double]$payload.actual_work_hours * 60.0 }
    }
    default { throw 'Неподдерживаемое изменение.' }
}
[void]$projectApplication.CalculateProject()
if ($dependencyEdit -or $unitsOnlyEdit -or $batchTextEdit -or $wrapEdit) {
    $preservedWorkTolerance = 0.01
    if ($unitsOnlyEdit) { $preservedWorkTolerance = 0.5 }
    foreach ($entry in $dependencyProgress.GetEnumerator()) {
        $item = Get-TaskByUniqueId([int]$entry.Key)
        $old = $entry.Value
        if ([math]::Abs([double]$item.Work - $old.work) -gt $preservedWorkTolerance -or
            [math]::Abs([double]$item.ActualWork - $old.actual) -gt 0.01 -or
            [string]$item.ActualStart -ne $old.actual_start -or [string]$item.ActualFinish -ne $old.actual_finish) {
            throw "При изменении связи или процентов изменились часы или фактические даты задачи UniqueID=$($item.UniqueID)."
        }
        if ($item.Assignments.Count -ne $old.assignments.Count) { throw 'При изменении связи изменился набор назначений.' }
        foreach ($a in $item.Assignments) {
            $prior = $old.assignments[[int]$a.UniqueID]
            $expectedUnits = $prior.units
            if ($unitsOnlyEdit -and $a.UniqueID -eq $changed.UniqueID) { $expectedUnits = [double]$payload.units }
            if ($null -eq $prior -or [math]::Abs([double]$a.Work - $prior.work) -gt $preservedWorkTolerance -or
                [math]::Abs([double]$a.ActualWork - $prior.actual) -gt 0.01 -or
                [math]::Abs([double]$a.Units - $expectedUnits) -gt 0.000001) {
                throw 'При изменении связи изменились часы или проценты назначения.'
            }
        }
    }
}
if ($dependencyEdit) {
    $links = @($changed.TaskDependencies | Where-Object {
        $_.From.UniqueID -eq [int]$payload.predecessor_unique_id -and $_.To.UniqueID -eq $changed.UniqueID
    })
    if ($payload.operation -eq 'link' -and ($links.Count -ne 1 -or [int]$links[0].Type -ne $linkType)) {
        throw 'Project не сохранил заданную связь или её тип.'
    }
    if ($payload.operation -eq 'unlink' -and $links.Count -ne 0) { throw 'Project не удалил заданную связь.' }
    $projectApplication.CloseUndoTransaction()
    $workUndoOpen = $false
}
if ($workEdit) {
    $verified = $workTask
    if ($payload.kind -eq 'assignment') { $verified = $changed }
    # Project округляет пересчитанный остаток; факт без явного изменения сохраняем строго.
    $workTolerance = 0.5
    $taskActualTolerance = 0.01
    if (Has-Field 'actual_work_hours') { $taskActualTolerance = 0.5 }
    if ((Has-Field 'work_hours') -and [math]::Abs([double]$verified.Work - [double]$payload.work_hours * 60.0) -gt $workTolerance) {
        throw ('Project не сохранил заданное количество человекочасов: задано {0} минут, получено {1} минут, допуск {2} минуты.' -f ([double]$payload.work_hours * 60.0), [double]$verified.Work, $workTolerance)
    }
    if ((Has-Field 'task_type') -and $workTask.Type -ne [int]$payload.task_type) { throw 'Project не сохранил тип задачи.' }
    if ((Has-Field 'effort_driven') -and $workTask.EffortDriven -ne [bool]$payload.effort_driven) { throw 'Project не сохранил effort_driven.' }
    if ((Has-Field 'actual_work_hours') -and (
        [math]::Abs([double]$verified.ActualWork - [double]$payload.actual_work_hours * 60.0) -gt $workTolerance -or
        [math]::Abs([double]$verified.RemainingWork - ([double]$payload.work_hours - [double]$payload.actual_work_hours) * 60.0) -gt $workTolerance)) {
        throw 'Project не сохранил заданный факт или остаток назначения.'
    }
    if ([math]::Abs([double]$workTask.ActualWork - $expectedTaskActualWork) -gt $taskActualTolerance) { throw 'Project неожиданно изменил фактические трудозатраты задачи.' }
    if (-not (Has-Field 'actual_work_hours') -and
        ([string]$workTask.ActualStart -ne $originalActualStart -or [string]$workTask.ActualFinish -ne $originalActualFinish)) {
        throw 'Project неожиданно изменил фактические даты задачи.'
    }
    foreach ($assignment in $workTask.Assignments) {
        if (-not $originalActualWork.ContainsKey([int]$assignment.UniqueID)) { throw 'Project изменил набор назначений.' }
        $expectedActual = $originalActualWork[[int]$assignment.UniqueID]
        if ((Has-Field 'actual_work_hours') -and $assignment.UniqueID -eq $changed.UniqueID) {
            $expectedActual = [double]$payload.actual_work_hours * 60.0
        }
        $actualTolerance = 0.01
        if ((Has-Field 'actual_work_hours') -and $assignment.UniqueID -eq $changed.UniqueID) { $actualTolerance = $workTolerance }
        if ([math]::Abs([double]$assignment.ActualWork - $expectedActual) -gt $actualTolerance) {
            throw 'Project изменил фактические трудозатраты назначения.'
        }
        if ($payload.kind -eq 'assignment' -and $assignment.UniqueID -ne $changed.UniqueID -and
            [math]::Abs([double]$assignment.Work - $originalAssignmentWork[[int]$assignment.UniqueID]) -gt 0.5) {
            throw 'Project изменил плановые часы другого назначения.'
        }
    }
    if ($workTask.Type -eq 0) {
        foreach ($assignment in $workTask.Assignments) {
            if (-not $originalUnits.ContainsKey([int]$assignment.UniqueID) -or
                [math]::Abs([double]$assignment.Units - $originalUnits[[int]$assignment.UniqueID]) -gt 0.000001) {
                throw 'Project изменил проценты назначения при фиксированных единицах.'
            }
        }
        if ($workTask.Assignments.Count -ne $originalUnits.Count) { throw 'Project изменил набор назначений.' }
    }
    $projectApplication.CloseUndoTransaction()
    $workUndoOpen = $false
}
if ($null -ne $createParent) {
    if ($changed.OutlineParent.UniqueID -ne $createParent.UniqueID -or
        [math]::Abs([double]$createParent.Work - $createParentWork) -gt 0.01 -or
        [math]::Abs([double]$createParent.ActualWork - $createParentActual) -gt 0.01) {
        throw ('Создание подзадачи изменило исходные часы или родителя: родитель {0}/{1}, часы {2}/{3}, факт {4}/{5}.' -f $changed.OutlineParent.UniqueID, $createParent.UniqueID, [double]$createParent.Work, $createParentWork, [double]$createParent.ActualWork, $createParentActual)
    }
}
if ($textEdit) {
    foreach ($field in $payload.text_fields.PSObject.Properties) {
        $value = [string](Get-ComProperty $changed $field.Name)
        if ($value -cne [string]$field.Value) { throw ('Project не сохранил поле ' + $field.Name) }
    }
    if ((Has-Field 'name') -and [string]$changed.Name -cne [string]$payload.name) { throw 'Project не сохранил название задачи.' }
    if ($workUndoOpen) {
        $projectApplication.CloseUndoTransaction()
        $workUndoOpen = $false
    }
}
} catch {
    $failure = $_
    if ($workUndoOpen) { $projectApplication.CloseUndoTransaction() }
    if ($workMutated) {
        try {
            $undone = $projectApplication.Undo(1)
            if (-not $undone) { throw 'Microsoft Project не подтвердил отмену.' }
        } catch {
            throw "Ошибка изменения часов: $failure. Автоматическая отмена не выполнена: $_. План не сохранён; резервная копия: $backup."
        }
    }
    throw $failure
}
[void]$projectApplication.FileSave()
$result = [ordered]@{ source = $source; backup = $backup; kind = $payload.kind; operation = $payload.operation; saved = $true }
if ($dependencyEdit) {
    $result.predecessor_unique_id = [int]$payload.predecessor_unique_id
    if ($payload.operation -eq 'link') { $result.link_type = $linkType }
}
if ($payload.kind -eq 'task' -and $payload.operation -eq 'reparent') {
    $result.parent_unique_id = $changed.OutlineParent.UniqueID
    $result.outline = $changed.OutlineNumber
    $result.previous_task_unique_id = $previousTaskUniqueId
}
if ($null -ne $changed -and $payload.operation -notin @('delete', 'remove', 'unlink')) {
    if ($payload.kind -eq 'resource') { $result.resource_unique_id = $changed.UniqueID; $result.name = $changed.Name }
    elseif ($payload.kind -eq 'assignment') { $result.task_unique_id = $payload.task_unique_id; $result.resource_unique_id = $payload.resource_unique_id; $result.units = $changed.Units; $result.work_hours = [double]$changed.Work / 60.0 }
    else { $result.task_unique_id = $changed.UniqueID; $result.name = $changed.Name; $result.start = $changed.Start.ToString('yyyy-MM-dd'); $result.finish = $changed.Finish.ToString('yyyy-MM-dd'); $result.work_hours = [double]$changed.Work / 60.0; $result.duration_minutes = [double]$changed.Duration; $result.task_type = [int]$changed.Type }
}
if ($null -ne $changed -and $payload.kind -in @('task', 'schedule', 'assignment') -and $payload.operation -eq 'update') {
    $result.actual_work_hours = [double]$changed.ActualWork / 60.0
    $result.remaining_work_hours = [double]$changed.RemainingWork / 60.0
}
if ($textEdit) {
    $result.text_fields = @{}
    foreach ($field in $payload.text_fields.PSObject.Properties) {
        $result.text_fields[$field.Name] = [string](Get-ComProperty $changed $field.Name)
    }
}
if ($batchTextEdit) { $result.tasks = $batchResults }
if ($wrapEdit) { $result.child_task_unique_id = $original.UniqueID; $result.parent_unique_id = $changed.UniqueID }
$json = [PSCustomObject]$result | ConvertTo-Json -Depth 6 -Compress
[Console]::Out.Write($json)`
}
