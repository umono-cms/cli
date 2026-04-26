package confed

import (
	"bufio"
	"os"
	"strings"
)

type EnvEditor struct {
	keys     []string
	keyValue map[string]string
}

func NewEnvEditor() *EnvEditor {
	return &EnvEditor{
		keys:     make([]string, 0),
		keyValue: make(map[string]string),
	}
}

func (e *EnvEditor) Read(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	e.keys = make([]string, 0)
	e.keyValue = make(map[string]string)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		e.keys = append(e.keys, key)
		e.keyValue[key] = value
	}

	return scanner.Err()
}

func (e *EnvEditor) SetValue(key, value string) *EnvEditor {
	e.removeKey(key)
	e.keys = append(e.keys, key)
	e.keyValue[key] = value
	return e
}

func (e *EnvEditor) PrependValue(key, value string) *EnvEditor {
	e.removeKey(key)
	e.keys = append([]string{key}, e.keys...)
	e.keyValue[key] = value
	return e
}

func (e *EnvEditor) removeKey(key string) {
	for i, k := range e.keys {
		if k == key {
			e.keys = append(e.keys[:i], e.keys[i+1:]...)
			return
		}
	}
}

func (e *EnvEditor) AddBlankLine() *EnvEditor {
	e.keys = append(e.keys, "")
	return e
}

func (e *EnvEditor) Write(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	for _, key := range e.keys {
		if key == "" {
			writer.WriteString("\n")
			continue
		}
		writer.WriteString(key + "=" + e.keyValue[key] + "\n")
	}

	return writer.Flush()
}
