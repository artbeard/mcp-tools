// CLI: локальный таймер + списание worklog в Yandex Tracker (бывший tracker_time.py).
//
// Модель state-файла:
//   active   - один текущий таймер (или null)
//   sessions - остановленные куски времени, ещё не отправленные в Tracker
//
// Команды: start / status / stop / write / clear.
// write по умолчанию dry-run; реально: --send + --confirm-send.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/artbeard/mcp-tools/internal/envfile"
	"github.com/artbeard/mcp-tools/internal/repopath"
	"github.com/artbeard/mcp-tools/internal/timerstate"
	"github.com/artbeard/mcp-tools/internal/tracker"
)

func main() {
	// Глобальные флаги - до имени команды: tracker-time --state-file X start IT-1
	stateFile := flag.String("state-file", "", "JSON state (по умолчанию ~/.local/state/mcp-tools/tracker-time.json)")
	envDir := flag.String("env-dir", "", "каталог с .env / .env.local (нужен только для write --send)")
	flag.Parse()
	args := flag.Args() // всё после глобальных флагов: [команда, ...]
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: tracker-time [--state-file PATH] [--env-dir DIR] <start|status|stop|write|clear> ...\n")
		os.Exit(2)
	}

	path := *stateFile
	if path == "" {
		var err error
		path, err = timerstate.DefaultPath()
		if err != nil {
			fatal(err)
		}
	}

	var err error
	switch args[0] {
	case "start":
		err = cmdStart(path, args[1:])
	case "status":
		err = cmdStatus(path)
	case "stop":
		err = cmdStop(path, args[1:])
	case "write":
		err = cmdWrite(path, *envDir, args[1:])
	case "clear":
		err = cmdClear(path, args[1:])
	default:
		err = fmt.Errorf("unknown command %q (want start|status|stop|write|clear)", args[0])
	}
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}

// start ISSUE_KEY [--comment ...] - завести active-таймер.
// Одновременно может быть только один active.
func cmdStart(statePath string, args []string) error {
	// stdlib flag ломается на "start IT-1 --comment x" (флаг после позиционного),
	// поэтому --comment выдираем вручную из любого места.
	comment, positional, err := pullStringFlag(args, "comment")
	if err != nil {
		return err
	}
	if len(positional) < 1 {
		return fmt.Errorf("start requires issue key")
	}
	issueKey := strings.ToUpper(strings.TrimSpace(positional[0]))
	if !tracker.ValidIssueKey(issueKey) {
		return fmt.Errorf("unsupported issue key format")
	}

	st, err := timerstate.Load(statePath)
	if err != nil {
		return err
	}
	if st.Active != nil {
		started, _ := timerstate.ParseTime(st.Active.StartedAt)
		return fmt.Errorf(
			"timer already active for %s since %s",
			st.Active.IssueKey,
			timerstate.FormatLocal(started),
		)
	}
	st.Active = &timerstate.Active{
		IssueKey:  issueKey,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Comment:   strings.TrimSpace(comment),
	}
	if err := timerstate.Save(statePath, st); err != nil {
		return err
	}
	return printJSON(map[string]any{"status": "STARTED", "issue_key": issueKey})
}

// status - что сейчас тикает + сколько pending sessions ждут write.
func cmdStatus(statePath string) error {
	st, err := timerstate.Load(statePath)
	if err != nil {
		return err
	}
	pendingSeconds := 0
	for _, s := range st.Sessions {
		pendingSeconds += s.DurationSeconds
	}
	result := map[string]any{
		"active":           nil,
		"pending_sessions": len(st.Sessions),
		"pending_seconds":  pendingSeconds,
		"pending":          tracker.SecondsToHuman(pendingSeconds),
	}
	if st.Active != nil {
		started, err := timerstate.ParseTime(st.Active.StartedAt)
		if err != nil {
			return err
		}
		elapsed := int(time.Now().UTC().Sub(started.UTC()).Seconds())
		result["active"] = map[string]any{
			"issue_key":        st.Active.IssueKey,
			"started_at":       st.Active.StartedAt,
			"started_at_local": timerstate.FormatLocal(started),
			"elapsed_seconds":  elapsed,
			"elapsed":          tracker.SecondsToHuman(elapsed),
			"comment":          st.Active.Comment,
		}
	}
	return printJSON(result)
}

// stop [--comment ...] - погасить active, положить кусок в sessions (ещё не в Tracker).
func cmdStop(statePath string, args []string) error {
	comment, _, err := pullStringFlag(args, "comment")
	if err != nil {
		return err
	}

	st, err := timerstate.Load(statePath)
	if err != nil {
		return err
	}
	if st.Active == nil {
		return fmt.Errorf("no active timer")
	}
	started, err := timerstate.ParseTime(st.Active.StartedAt)
	if err != nil {
		return err
	}
	stopped := time.Now().UTC()
	duration := int(stopped.Sub(started.UTC()).Seconds())
	// Tracker worklog - минутами; меньше минуты округляем вверх до 60с
	if duration < 60 {
		duration = 60
	}
	cmt := strings.TrimSpace(comment)
	if cmt == "" {
		cmt = st.Active.Comment // берём комментарий со start, если stop без --comment
	}
	session := timerstate.Session{
		IssueKey:        st.Active.IssueKey,
		StartedAt:       st.Active.StartedAt,
		StoppedAt:       stopped.Format(time.RFC3339Nano),
		DurationSeconds: duration,
		Comment:         cmt,
	}
	st.Active = nil
	st.Sessions = append(st.Sessions, session)
	if err := timerstate.Save(statePath, st); err != nil {
		return err
	}
	return printJSON(map[string]any{
		"status":           "STOPPED",
		"issue_key":        session.IssueKey,
		"duration_seconds": duration,
		"duration":         tracker.SecondsToHuman(duration),
		"comment":          cmt,
	})
}

// write - собрать pending sessions в один worklog payload и (опционально) отправить.
// Без --send только показывает DRY_RUN. С --send нужен ещё --confirm-send.
func cmdWrite(statePath, envDir string, args []string) error {
	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	issueKey := fs.String("issue-key", "", "списать только эту задачу")
	comment := fs.String("comment", "", "комментарий worklog")
	roundTo := fs.Int("round-to", 1, "округление длительности до N минут")
	ignoreActive := fs.Bool("ignore-active", false, "писать, даже если таймер ещё бежит")
	send := fs.Bool("send", false, "реально отправить worklog")
	confirmSend := fs.Bool("confirm-send", false, "подтверждение вместе с --send")
	// true в map = у флага есть значение (--issue-key IT-1), false = boolean-флаг
	flagArgs, positional := splitFlags(args, map[string]bool{
		"issue-key": true, "comment": true, "round-to": true,
		"ignore-active": false, "send": false, "confirm-send": false,
	})
	if len(positional) > 0 {
		return fmt.Errorf("write: unexpected args %v", positional)
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	st, err := timerstate.Load(statePath)
	if err != nil {
		return err
	}
	// По умолчанию нельзя списывать, пока таймер ещё тикает - сначала stop
	if st.Active != nil && !*ignoreActive {
		return fmt.Errorf(
			"timer is active for %s; stop it first or pass --ignore-active",
			st.Active.IssueKey,
		)
	}

	sessions, err := selectSessions(st.Sessions, *issueKey)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return fmt.Errorf("no stopped sessions to write")
	}
	// В один POST уходит одна задача; если в pending несколько ключей - требуем --issue-key
	keys := map[string]struct{}{}
	for _, s := range sessions {
		keys[s.IssueKey] = struct{}{}
	}
	if len(keys) != 1 {
		return fmt.Errorf("sessions contain multiple issues; pass --issue-key")
	}
	var onlyKey string
	for k := range keys {
		onlyKey = k
	}

	// start worklog = самое раннее started_at; duration = сумма кусков (с округлением)
	startedMin, err := timerstate.ParseTime(sessions[0].StartedAt)
	if err != nil {
		return err
	}
	rawSeconds := 0
	for _, s := range sessions {
		rawSeconds += s.DurationSeconds
		t, err := timerstate.ParseTime(s.StartedAt)
		if err != nil {
			return err
		}
		if t.Before(startedMin) {
			startedMin = t
		}
	}
	durationSeconds := tracker.RoundSeconds(rawSeconds, *roundTo)

	// Комментарий: явный --comment, иначе склейка уникальных из sessions, иначе дефолт
	cmt := strings.TrimSpace(*comment)
	if cmt == "" {
		seen := map[string]struct{}{}
		var parts []string
		for _, s := range sessions {
			c := strings.TrimSpace(s.Comment)
			if c == "" {
				continue
			}
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			parts = append(parts, c)
		}
		cmt = strings.Join(parts, "; ")
		if cmt == "" {
			cmt = "Работа по задаче."
		}
	}

	payload := map[string]any{
		"start":    timerstate.FormatWorklogStart(startedMin),
		"duration": tracker.SecondsToISODuration(durationSeconds), // PT1H30M
		"comment":  cmt,
	}
	base := map[string]any{
		"issue_key":        onlyKey,
		"sessions":         len(sessions),
		"raw_seconds":      rawSeconds,
		"raw_duration":     tracker.SecondsToHuman(rawSeconds),
		"duration_seconds": durationSeconds,
		"duration":         tracker.SecondsToHuman(durationSeconds),
		"payload":          payload,
	}

	if !*send {
		out := map[string]any{"status": "DRY_RUN"}
		for k, v := range base {
			out[k] = v
		}
		return printJSON(out)
	}
	if !*confirmSend && os.Getenv("CONFIRM_SEND") != "YES" {
		return fmt.Errorf("--send requires --confirm-send or CONFIRM_SEND=YES")
	}

	// Env/токен нужны только на реальной отправке
	dir := envDir
	if dir == "" {
		dir, err = repopath.EnvDir()
		if err != nil {
			return err
		}
	}
	env, err := envfile.LoadDir(dir)
	if err != nil {
		return err
	}
	client, err := tracker.NewClient(env)
	if err != nil {
		return err
	}
	result, err := client.PostWorklog(onlyKey, payload)
	if err != nil {
		return err
	}

	// После успешного POST выкидываем списанные sessions из state
	remove := map[string]struct{}{}
	for _, s := range sessions {
		remove[sessionID(s)] = struct{}{}
	}
	kept := make([]timerstate.Session, 0, len(st.Sessions))
	for _, s := range st.Sessions {
		if _, ok := remove[sessionID(s)]; ok {
			continue
		}
		kept = append(kept, s)
	}
	st.Sessions = kept
	if err := timerstate.Save(statePath, st); err != nil {
		return err
	}

	out := map[string]any{"status": "POSTED", "result": result}
	for k, v := range base {
		out[k] = v
	}
	return printJSON(out)
}

// clear --active / --sessions [--issue-key] - локальная чистка state, без Tracker API.
func cmdClear(statePath string, args []string) error {
	fs := flag.NewFlagSet("clear", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	clearActive := fs.Bool("active", false, "сбросить активный таймер")
	clearSessions := fs.Bool("sessions", false, "сбросить pending sessions")
	issueKey := fs.String("issue-key", "", "только sessions этой задачи")
	flagArgs, positional := splitFlags(args, map[string]bool{
		"active": false, "sessions": false, "issue-key": true,
	})
	if len(positional) > 0 {
		return fmt.Errorf("clear: unexpected args %v", positional)
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	st, err := timerstate.Load(statePath)
	if err != nil {
		return err
	}
	if *clearActive {
		st.Active = nil
	}
	if *clearSessions {
		if strings.TrimSpace(*issueKey) != "" {
			key := strings.ToUpper(strings.TrimSpace(*issueKey))
			if !tracker.ValidIssueKey(key) {
				return fmt.Errorf("unsupported issue key format")
			}
			kept := make([]timerstate.Session, 0)
			for _, s := range st.Sessions {
				if s.IssueKey == key {
					continue
				}
				kept = append(kept, s)
			}
			st.Sessions = kept
		} else {
			st.Sessions = []timerstate.Session{}
		}
	}
	if err := timerstate.Save(statePath, st); err != nil {
		return err
	}
	return printJSON(map[string]any{"status": "CLEARED"})
}

// selectSessions - все pending либо только по --issue-key.
func selectSessions(sessions []timerstate.Session, issueKey string) ([]timerstate.Session, error) {
	if strings.TrimSpace(issueKey) == "" {
		out := make([]timerstate.Session, len(sessions))
		copy(out, sessions)
		return out, nil
	}
	key := strings.ToUpper(strings.TrimSpace(issueKey))
	if !tracker.ValidIssueKey(key) {
		return nil, fmt.Errorf("unsupported issue key format")
	}
	var out []timerstate.Session
	for _, s := range sessions {
		if s.IssueKey == key {
			out = append(out, s)
		}
	}
	return out, nil
}

// sessionID - ключ, чтобы после POST выкинуть именно списанные куски.
func sessionID(s timerstate.Session) string {
	return s.IssueKey + "|" + s.StartedAt + "|" + s.StoppedAt + "|" + fmt.Sprint(s.DurationSeconds)
}

// pullStringFlag достаёт --name VALUE / --name=VALUE из args в любом месте
// (нужно, потому что MCP зовёт: start IT-1 --comment "...").
func pullStringFlag(args []string, name string) (value string, rest []string, err error) {
	long := "--" + name
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == long {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("flag %s needs a value", long)
			}
			return args[i+1], append(append([]string{}, args[:i]...), args[i+2:]...), nil
		}
		if strings.HasPrefix(a, long+"=") {
			return strings.TrimPrefix(a, long+"="), append(append([]string{}, args[:i]...), args[i+1:]...), nil
		}
	}
	return "", args, nil
}

// splitFlags раскладывает args на флаги и позиционные.
// takesValue[name]=true - у флага есть значение (--issue-key IT-1).
func splitFlags(args []string, takesValue map[string]bool) (flagArgs, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "--") {
			positional = append(positional, a)
			continue
		}
		name := strings.TrimPrefix(a, "--")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			flagArgs = append(flagArgs, a)
			continue
		}
		flagArgs = append(flagArgs, a)
		if takesValue[name] {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		}
	}
	return flagArgs, positional
}

func printJSON(payload any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
