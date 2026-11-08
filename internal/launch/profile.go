package launch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// modelPlaceholder in a command's argument is replaced by the chosen model.
const modelPlaceholder = "{model}"

// Profile is one way to start an engine, from the config file.
type Profile struct {
	Name    string            `json:"name"`
	Command []string          `json:"command"` // argv; "{model}" in any argument is replaced by the chosen model
	Models  []string          `json:"models"`  // the choices for {model}; empty when the command has none
	Dir     string            `json:"dir"`
	Env     map[string]string `json:"env"` // added to steth's own environment
}

// Load reads the config file: {"launch": [Profile, ...]}.
func Load(path string) ([]Profile, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file struct {
		Launch []Profile `json:"launch"`
	}
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		// encoding/json has no typed error for an unknown field.
		if strings.HasPrefix(err.Error(), "json: unknown field") {
			return nil, fmt.Errorf(`%s: %w; the file holds {"launch": [...]}, and a profile's fields are name, command, models, dir and env`, path, err)
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf(`%s: text after the first JSON object; the file holds one object, {"launch": [...]}`, path)
	}
	if err := validate(file.Launch); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return file.Launch, nil
}

func validate(profiles []Profile) error {
	named := map[string]bool{}
	for i, p := range profiles {
		switch {
		case p.Name == "":
			return fmt.Errorf(`profile %d has no name; give it one, such as "name": "geode"`, i+1)
		case named[p.Name]:
			return fmt.Errorf("two profiles are named %q; give each its own name", p.Name)
		case len(p.Command) == 0 || p.Command[0] == "":
			return fmt.Errorf(`profile %q has an empty command; give its argv, such as "command": ["llama", "serve", "--port", "9931"]`, p.Name)
		case p.takesModel() && len(p.Models) == 0:
			return fmt.Errorf(`profile %q has %s in its command but no models; list the choices under "models"`, p.Name, modelPlaceholder)
		case !p.takesModel() && len(p.Models) > 0:
			return fmt.Errorf(`profile %q lists models but its command has no %s; put %s where the model goes, or drop "models"`, p.Name, modelPlaceholder, modelPlaceholder)
		}
		named[p.Name] = true
	}
	return nil
}

func (p Profile) takesModel() bool {
	return slices.ContainsFunc(p.Command, func(arg string) bool { return strings.Contains(arg, modelPlaceholder) })
}

// checkModel says whether model is one this profile can start with.
func (p Profile) checkModel(model string) error {
	if len(p.Models) == 0 {
		if model != "" {
			return fmt.Errorf("%q takes no model; start it with none", p.Name)
		}
		return nil
	}
	if slices.Contains(p.Models, model) {
		return nil
	}
	return fmt.Errorf("%q has no model %q; choose one of %s", p.Name, model, quoteAll(p.Models))
}

// argv is the command with model in place of every {model}.
func (p Profile) argv(model string) []string {
	argv := make([]string, len(p.Command))
	for i, arg := range p.Command {
		argv[i] = strings.ReplaceAll(arg, modelPlaceholder, model)
	}
	return argv
}

func quoteAll(words []string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = fmt.Sprintf("%q", w)
	}
	return strings.Join(quoted, ", ")
}
