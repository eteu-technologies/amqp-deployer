package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

var (
	varPattern     = regexp.MustCompile(`\(\(([a-z]+):([a-zA-Z0-9_\-]+)\)\)`)
	allowedEnvvars = map[string]bool{
		"HOME":            true,
		"LANG":            true,
		"PATH":            true,
		"SHELL":           true,
		"USER":            true,
		"XDG_RUNTIME_DIR": true,
	}
)

// https://stackoverflow.com/a/35247204
func replaceAllGroupFunc(re *regexp.Regexp, str string, repl func([]string) string) string {
	result := ""
	lastIndex := 0

	for _, v := range re.FindAllSubmatchIndex([]byte(str), -1) {
		groups := []string{}
		for i := 0; i < len(v); i += 2 {
			groups = append(groups, str[v[i]:v[i+1]])
		}

		result += str[lastIndex:v[0]] + repl(groups)
		lastIndex = v[1]
	}

	return result + str[lastIndex:]
}

func replaceVars(raw string, data map[string]string) string {
	return replaceAllGroupFunc(varPattern, raw, func(groups []string) string {
		vtype := strings.ToLower(groups[1])
		key := groups[2]

		switch vtype {
		case "data":
			if v, ok := data[key]; ok {
				return v
			}
			return ""
		case "env":
			if _, ok := allowedEnvvars[key]; ok {
				return os.Getenv(key)
			}
		}
		return ""
	})
}

func processDeployable(deploy Deployable, data map[string]string) (err error) {
	for idx, action := range deploy.Actions {
		idx := idx
		if len(action.Command) == 0 {
			err = fmt.Errorf("action %d has empty command array", idx)
			return
		}

		workdir := replaceVars(action.WorkDir, data)
		argv := []string{}
		for _, arg := range action.Command {
			argv = append(argv, replaceVars(arg, data))
		}

		cmd := exec.Command(argv[0], argv...)
		cmd.Dir = workdir
		cmd.Env = os.Environ()
		for key, value := range action.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, replaceVars(value, data)))
		}

		// TODO: wire to zap or journald
		cmd.Stderr = os.Stdout
		cmd.Stdout = os.Stdout

		zap.L().Debug("processing action", zap.Int("idx", idx), zap.Strings("argv", argv))
		if err = cmd.Start(); err != nil {
			err = fmt.Errorf("action %d command failed to execute: %w", idx, err)
			return
		}

		go func() {
			err := cmd.Wait()
			zap.L().Info("action command exited", zap.String("tag", deploy.Tag), zap.Int("idx", idx), zap.Error(err))
		}()
	}

	return
}
