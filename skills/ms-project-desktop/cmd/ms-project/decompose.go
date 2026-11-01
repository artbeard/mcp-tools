package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
)

type decompositionAssignment struct {
	ResourceUniqueID int     `json:"resource_unique_id"`
	WorkHours        float64 `json:"work_hours"`
	Units            float64 `json:"units,omitempty"`
}

type decompositionChild struct {
	Name        string                    `json:"name"`
	WorkHours   *float64                  `json:"work_hours,omitempty"`
	Assignments []decompositionAssignment `json:"assignments,omitempty"`
	TextFields  textFieldValues           `json:"text_fields,omitempty"`
}

func validDecompositionHours(h float64) bool {
	return !math.IsNaN(h) && !math.IsInf(h, 0) && h >= 0 && h <= 1e7
}

func validateDecomposition(r changeRequest) error {
	if r.Kind != "task" || r.TaskUniqueID <= 0 || r.Name == "" || r.ChildName == "" || r.ExpectedWorkHours == nil || !validDecompositionHours(*r.ExpectedWorkHours) {
		return errors.New("task:decompose требует task_unique_id, исходное name, child_name и expected_work_hours")
	}
	rest := r
	rest.Kind, rest.Operation, rest.Name, rest.ChildName = "", "", "", ""
	rest.TaskUniqueID = 0
	rest.TaskChildren, rest.SourceAssignmentWork, rest.TextFields = nil, nil, nil
	rest.SourceWorkHours, rest.ExpectedWorkHours = nil, nil
	b, _ := json.Marshal(rest)
	if string(b) != `{"kind":"","operation":""}` {
		return errors.New("task:decompose не принимает обычные поля изменения расписания")
	}
	if err := validateChange(changeRequest{Kind: "task", Operation: "update", TaskUniqueID: r.TaskUniqueID, TextFields: r.TextFields}); err != nil {
		return err
	}
	if r.SourceWorkHours != nil && (!validDecompositionHours(*r.SourceWorkHours) || len(r.SourceAssignmentWork) > 0) {
		return errors.New("source_work_hours и source_assignment_work взаимоисключающие")
	}
	names := map[string]bool{r.ChildName: true}
	for _, c := range r.TaskChildren {
		if c.Name == "" || names[c.Name] {
			return errors.New("названия дочерних задач должны быть непустыми и уникальными")
		}
		names[c.Name] = true
		if c.WorkHours != nil && !validDecompositionHours(*c.WorkHours) {
			return errors.New("недопустимые часы подзадачи")
		}
		if c.TextFields["Text1"] != "" {
			return errors.New("дочерние задачи не индексируются")
		}
		if err := validateChange(changeRequest{Kind: "task", Operation: "update", TaskUniqueID: 1, TextFields: c.TextFields}); err != nil {
			return err
		}
		seen := map[int]bool{}
		total := 0.0
		for _, a := range c.Assignments {
			if a.ResourceUniqueID <= 0 || seen[a.ResourceUniqueID] || !validDecompositionHours(a.WorkHours) || math.IsNaN(a.Units) || math.IsInf(a.Units, 0) || a.Units <= 0 || a.Units > 100 {
				return errors.New("недопустимое назначение подзадачи")
			}
			seen[a.ResourceUniqueID] = true
			total += a.WorkHours
		}
		if len(c.Assignments) > 0 && (c.WorkHours == nil || math.Abs(total-*c.WorkHours) > 1e-6) {
			return errors.New("сумма назначений должна совпадать с часами подзадачи")
		}
	}
	seen := map[int]bool{}
	for _, a := range r.SourceAssignmentWork {
		if a.ResourceUniqueID <= 0 || seen[a.ResourceUniqueID] || !validDecompositionHours(a.WorkHours) || a.Units != 0 {
			return errors.New("изменение исходного назначения задаёт только ресурс и часы")
		}
		seen[a.ResourceUniqueID] = true
	}
	return nil
}

func checkDecomposition(s outlineSnapshot, r changeRequest) (*taskSnapshot, error) {
	var t *taskSnapshot
	for i := range s.Tasks {
		if s.Tasks[i].UniqueID == r.TaskUniqueID {
			t = &s.Tasks[i]
			break
		}
	}
	if t == nil || t.Summary || t.Milestone {
		return nil, errors.New("нужна существующая несводная задача, не веха")
	}
	if t.Name != r.Name || math.Abs(t.WorkHours-*r.ExpectedWorkHours) > 0.00001 {
		return nil, errors.New("задача изменена после чтения: перечитайте план")
	}
	if code, ok := r.TextFields["Text1"]; ok && code != t.TextFields["Text1"] {
		return nil, errors.New("индекс исходной задачи не соответствует запросу")
	}
	source := t.WorkHours
	if len(r.SourceAssignmentWork) > 0 || r.SourceWorkHours != nil {
		if t.Manual || t.TaskType != 0 || t.EffortDriven || t.ActualWorkHours > 0 {
			return nil, errors.New("перераспределение поддерживается только для автоматической задачи с фиксированными единицами без факта")
		}
	}
	if r.SourceWorkHours != nil {
		if len(t.Assignments) != 0 {
			return nil, errors.New("source_work_hours допустим только без назначений")
		}
		source = *r.SourceWorkHours
	}
	for _, update := range r.SourceAssignmentWork {
		found := false
		for _, a := range t.Assignments {
			if a.ResourceUniqueID == update.ResourceUniqueID {
				source += update.WorkHours - a.WorkHours
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("исходное назначение не найдено")
		}
	}
	if source < t.ActualWorkHours-1e-6 {
		return nil, errors.New("исходные часы меньше факта")
	}
	total := source
	resources := map[int]bool{}
	for _, a := range s.Resources {
		resources[a.UniqueID] = true
	}
	for _, c := range r.TaskChildren {
		if c.WorkHours != nil {
			total += *c.WorkHours
		}
		for _, a := range c.Assignments {
			if !resources[a.ResourceUniqueID] {
				return nil, errors.New("ресурс подзадачи не найден")
			}
		}
	}
	if math.Abs(total-t.WorkHours) > 0.00001 {
		return nil, fmt.Errorf("декомпозиция должна сохранить бюджет: было %.6f ч, станет %.6f ч", t.WorkHours, total)
	}
	return t, nil
}

func previewDecomposition(out *os.File, r changeRequest) error {
	raw, err := runPowerShell(outlineScript)
	if err != nil {
		return err
	}
	var s outlineSnapshot
	if err = json.Unmarshal(raw, &s); err != nil {
		return err
	}
	t, err := checkDecomposition(s, r)
	if err != nil {
		return err
	}
	b, err := json.Marshal(map[string]any{"plan": s.Path, "request": r, "current": t, "warnings": []string{"Одна отменяемая операция; исходный UniqueID, факт и связи сохраняются. Назначения переносятся без увеличения суммарных часов. Даты связанных задач могут пересчитаться. Без оценки дочерняя задача остаётся ручной; с часами — автоматической. Перед записью проверяется актуальность исходной задачи."}, "confirmation_required": true})
	if err != nil {
		return err
	}
	return writeJSONBytes(out, b)
}

func applyDecomposition(out *os.File, r changeRequest, skipBackup bool) error {
	// Сверяем актуальную задачу перед записью; остальные разделы пользователь может редактировать.
	raw, err := runPowerShell(outlineScript)
	if err != nil {
		return err
	}
	var s outlineSnapshot
	if err = json.Unmarshal(raw, &s); err != nil {
		return err
	}
	t, err := checkDecomposition(s, r)
	if err != nil {
		return err
	}
	b, err := json.Marshal(map[string]any{"request": r, "expected": t})
	if err != nil {
		return err
	}
	return writeJSON(out, decompositionScript(base64.StdEncoding.EncodeToString(b), skipBackup))
}

func decompositionScript(encoded string, skipBackup bool) string {
	backup := "$backup = $null"
	if !skipBackup {
		backup = `$source = $project.FullName
$directory = [System.IO.Path]::GetDirectoryName($source)
$baseName = [System.IO.Path]::GetFileNameWithoutExtension($source)
$extension = [System.IO.Path]::GetExtension($source)
$backup = Join-Path $directory ($baseName + '-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + $extension)
if (Test-Path -LiteralPath $backup) { throw 'Копия уже существует.' }
Copy-Item -LiteralPath $source -Destination $backup -ErrorAction Stop`
	}
	return projectPreamble() + `
$data = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('` + encoded + `')) | ConvertFrom-Json
$p = $data.request
$taskIndex = @{}
foreach ($item in $project.Tasks) { if ($null -ne $item) { $taskIndex[[int]$item.UniqueID] = $item } }
function Find-Task([int]$id) {
    if ($taskIndex.ContainsKey($id)) { return $taskIndex[$id] }
    $items = @($project.Tasks | Where-Object { $null -ne $_ -and $_.UniqueID -eq $id })
    if ($items.Count -ne 1) { throw 'Задача не найдена.' }
    $taskIndex[$id] = $items[0]
    return $items[0]
}
function Put-Texts($task,$fields) {
    foreach ($f in $fields.PSObject.Properties) {
        [void]$task.GetType().InvokeMember($f.Name,[System.Reflection.BindingFlags]::SetProperty,$null,$task,@([string]$f.Value))
        if ([string]$task.($f.Name) -cne [string]$f.Value) { throw 'Текстовое поле не сохранено.' }
    }
}
$original = Find-Task([int]$p.task_unique_id)
if ($original.Summary -or $original.Milestone -or [string]$original.Name -cne [string]$p.name -or [math]::Abs([double]$original.Work/60 - [double]$p.expected_work_hours) -gt 0.00001) { throw 'Исходная задача изменилась; запись отменена.' }
$e = $data.expected
if ([double]$original.ActualWork/60 -ne [double]$e.actual_work_hours -or [string]$original.ActualStart -ne [string]$e.actual_start -or [string]$original.ActualFinish -ne [string]$e.actual_finish -or [string]$original.Text1 -cne [string]$e.text_fields.Text1 -or [bool]$original.Manual -ne [bool]$e.manual -or [int]$original.Type -ne [int]$e.task_type) { throw 'Исходные параметры изменились; запись отменена.' }
if ($original.Assignments.Count -ne @($e.assignments).Count) { throw 'Назначения изменились.' }
foreach ($ea in $e.assignments) {
    $aa = @($original.Assignments | Where-Object { $_.UniqueID -eq $ea.unique_id })
    if ($aa.Count -ne 1 -or [math]::Abs([double]$aa[0].Work/60-[double]$ea.work_hours) -gt 0.00001 -or [math]::Abs([double]$aa[0].ActualWork/60-[double]$ea.actual_work_hours) -gt 0.00001 -or [double]$aa[0].Units -ne [double]$ea.units) { throw 'Назначения изменились.' }
}
$guard = @{}
foreach ($t in $project.Tasks) {
    if ($null -ne $t -and -not $t.Summary) {
        $a = @{}
        foreach ($x in $t.Assignments) { $a[[int]$x.UniqueID] = @{ work=[double]$x.Work; actual=[double]$x.ActualWork; units=[double]$x.Units } }
        $guard[[int]$t.UniqueID] = @{ work=[double]$t.Work; actual=[double]$t.ActualWork; actual_start=[string]$t.ActualStart; actual_finish=[string]$t.ActualFinish; assignments=$a; predecessors=[string]$t.Predecessors; successors=[string]$t.Successors }
    }
}
$oldStart = $original.Start
$oldFinish = $original.Finish
$oldWork = [double]$original.Work
$oldActual = [double]$original.ActualWork
$oldLevel = [int]$original.OutlineLevel
` + backup + `
$undoOpen = $false
$mutated = $false
try {
    $projectApplication.OpenUndoTransaction('Декомпозиция задачи по видам работ')
    $undoOpen = $true
    $mutated = $true
    $parent = $project.Tasks.Add([string]$p.name,[int]$original.ID)
    $parent.OutlineLevel = $oldLevel
    $original.OutlineLevel = $oldLevel+1
    $parent.Text1 = [string]$original.Text1
    $original.Text1 = ''
    $original.Name = [string]$p.child_name
    Put-Texts $parent $p.text_fields
    if ($null -ne $p.source_work_hours) { $original.Work = [double]$p.source_work_hours*60; $guard[[int]$original.UniqueID].work = [double]$p.source_work_hours*60 }
    foreach ($update in $p.source_assignment_work) {
        $a = @($original.Assignments | Where-Object { $_.Resource.UniqueID -eq $update.resource_unique_id })[0]
        $original.EffortDriven = $false
        $a.Work = [double]$update.work_hours*60
        $guard[[int]$original.UniqueID].assignments[[int]$a.UniqueID].work = [double]$update.work_hours*60
    }
    if ($null -ne $p.source_assignment_work -and @($p.source_assignment_work).Count -gt 0) {
        $sum = 0.0
        foreach ($a in $guard[[int]$original.UniqueID].assignments.Values) { $sum += $a.work }
        $guard[[int]$original.UniqueID].work = $sum
    }
    $children = @()
    foreach ($c in $p.task_children) {
        # Вставляем после последней дочерней строки, сохраняя порядок BE/FE/тесты.
        $at = [int]$original.ID + 1 + $children.Count
        $new = $project.Tasks.Add([string]$c.name,$at)
        $taskIndex[[int]$new.UniqueID] = $new
        $new.OutlineLevel = $oldLevel+1
        $new.Manual = $true
        $new.Start = $oldStart
        $new.Finish = $oldFinish
        $new.Text1 = ''
        Put-Texts $new $c.text_fields
        if ($null -ne $c.work_hours) {
            $new.Manual = $false
            $new.Type = 0
            $new.EffortDriven = $false
            if ($null -ne $c.assignments -and @($c.assignments).Count -gt 0) {
                foreach ($a in $c.assignments) {
                    $resource = @($project.Resources | Where-Object { $null -ne $_ -and $_.UniqueID -eq $a.resource_unique_id })[0]
                    $assignment = $new.Assignments.Add($new.ID,$resource.ID,[double]$a.units)
                    $assignment.Work = [double]$a.work_hours*60
                }
            } else { $new.Work = [double]$c.work_hours*60 }
            # После переключения из ручного режима Project сбрасывает начало.
            $new.Start = $oldStart
        }
        $children += [PSCustomObject]@{ unique_id=[int]$new.UniqueID; name=[string]$new.Name; expected_work=$c.work_hours; expected_assignments=$c.assignments; estimated=($null -ne $c.work_hours) }
    }
    [void]$projectApplication.CalculateProject()
    if ($original.OutlineParent.UniqueID -ne $parent.UniqueID -or [math]::Abs([double]$parent.Work-$oldWork) -gt 0.5 -or [math]::Abs([double]$parent.ActualWork-$oldActual) -gt 0.01) { throw 'Декомпозиция изменила бюджет, факт или иерархию.' }
    foreach ($entry in $guard.GetEnumerator()) {
        $t = Find-Task([int]$entry.Key)
        $g = $entry.Value
        if ($t.Summary -or [math]::Abs([double]$t.Work-[double]$g.work) -gt 0.5 -or [math]::Abs([double]$t.ActualWork-[double]$g.actual) -gt 0.01 -or [string]$t.ActualStart -ne $g.actual_start -or [string]$t.ActualFinish -ne $g.actual_finish -or $t.Assignments.Count -ne $g.assignments.Count) { throw ('Изменились исходные часы или факт задачи '+$t.UniqueID) }
        foreach ($a in $t.Assignments) {
            $ag = $g.assignments[[int]$a.UniqueID]
            if ($null -eq $ag -or [math]::Abs([double]$a.Work-[double]$ag.work) -gt 0.5 -or [math]::Abs([double]$a.ActualWork-[double]$ag.actual) -gt 0.01 -or [math]::Abs([double]$a.Units-[double]$ag.units) -gt 0.000001) { throw ('Изменилось исходное назначение задачи '+$t.UniqueID) }
        }
    }
    foreach ($c in $children) {
        $t = Find-Task($c.unique_id)
        $expected = 0.0
        if ($c.estimated) { $expected = [double]$c.expected_work*60 }
        if ($t.OutlineParent.UniqueID -ne $parent.UniqueID -or [string]$t.Text1 -ne '' -or [math]::Abs([double]$t.Work-$expected) -gt 0.5 -or [double]$t.ActualWork -ne 0) { throw 'Не сохранены параметры подзадачи.' }
        foreach ($ea in $c.expected_assignments) {
            $a = @($t.Assignments | Where-Object { $_.Resource.UniqueID -eq $ea.resource_unique_id })
            if ($a.Count -ne 1 -or [math]::Abs([double]$a[0].Work-[double]$ea.work_hours*60) -gt 0.5 -or [math]::Abs([double]$a[0].Units-[double]$ea.units) -gt 0.000001) { throw 'Не сохранено назначение подзадачи.' }
        }
    }
    $projectApplication.CloseUndoTransaction()
    $undoOpen = $false
} catch {
    $failure = $_
    if ($undoOpen) { $projectApplication.CloseUndoTransaction() }
    if ($mutated -and -not $projectApplication.Undo(1)) { throw "Ошибка: $failure. Отмена не выполнена; план не сохранён." }
    throw $failure
}
$saved = $false
$saveFailure = $null
for ($attempt = 0; $attempt -lt 5 -and -not $saved; $attempt++) {
    try { $saved = [bool]$projectApplication.FileSave() } catch { $saveFailure = $_ }
    if (-not $saved) { Start-Sleep -Milliseconds 250 }
}
$rows = @($original)+@($children | ForEach-Object { Find-Task $_.unique_id })
$resultRows = @($rows | ForEach-Object { [PSCustomObject]@{ unique_id=$_.UniqueID; name=$_.Name; work_hours=[double]$_.Work/60; actual_work_hours=[double]$_.ActualWork/60; manual=$_.Manual; start=$_.Start.ToString('yyyy-MM-dd'); finish=$_.Finish.ToString('yyyy-MM-dd') } })
[PSCustomObject]@{ saved=$saved; save_error=[string]$saveFailure; source=$project.FullName; backup=$backup; operation='decompose'; parent_unique_id=$parent.UniqueID; source_unique_id=$original.UniqueID; work_hours=[double]$parent.Work/60; children=$resultRows } | ConvertTo-Json -Depth 8 -Compress | ForEach-Object { [Console]::Out.Write($_) }
`
}
