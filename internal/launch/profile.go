package launch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	// modelPlaceholder in a command's argument is replaced by the chosen model.
	modelPlaceholder = "{model}"
	// namedChoices is how many choices a refusal names.
	namedChoices = 5
)

// Config is steth's config file.
type Config struct {
	Launch []Profile `json:"launch"`
	// Models are directories and .gguf files; the model files under them
	// are the choices of every profile with {model} that lists none.
	Models []string `json:"models"`
}

// Profile is one way to start an engine, from the config file.
type Profile struct {
	Name    string            `json:"name"`
	Command []string          `json:"command"` // argv; "{model}" in any argument is replaced by the chosen model
	Models  []string          `json:"models"`  // the choices for {model}; empty when the command has none, or takes the config's model files
	Dir     string            `json:"dir"`
	Env     map[string]string `json:"env"` // added to steth's own environment
}

// Load reads the config file: {"launch": [Profile, ...], "models": [...]}.
func Load(path string) (Config, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var file Config
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		// encoding/json has no typed error for an unknown field.
		if strings.HasPrefix(err.Error(), "json: unknown field") {
			return Config{}, fmt.Errorf(`%s: %w; the file holds {"launch": [...], "models": [...]}, and a profile's fields are name, command, models, dir and env`, path, err)
		}
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return Config{}, fmt.Errorf(`%s: text after the first JSON object; the file holds one object, {"launch": [...], "models": [...]}`, path)
	}
	if err := validate(file); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return file, nil
}

func validate(c Config) error {
	for _, root := range c.Models {
		info, err := os.Stat(root)
		switch {
		case err != nil:
			return fmt.Errorf(`models: %w; list directories and %s files that are there`, err, modelExt)
		case !info.IsDir() && !strings.EqualFold(filepath.Ext(root), modelExt):
			return fmt.Errorf(`models: %q is neither a directory nor a %s file`, root, modelExt)
		}
	}
	named := map[string]bool{}
	for i, p := range c.Launch {
		switch {
		case p.Name == "":
			return fmt.Errorf(`profile %d has no name; give it one, such as "name": "geode"`, i+1)
		case named[p.Name]:
			return fmt.Errorf("two profiles are named %q; give each its own name", p.Name)
		case len(p.Command) == 0 || p.Command[0] == "":
			return fmt.Errorf(`profile %q has an empty command; give its argv, such as "command": ["llama", "serve", "--port", "9931"]`, p.Name)
		case p.takesModel() && len(p.Models) == 0 && len(c.Models) == 0:
			return fmt.Errorf(`profile %q has %s in its command but no models; list the choices under its "models", or directories and files under the file's "models"`, p.Name, modelPlaceholder)
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

// takesFiles says whether p's choices are the config's model files.
func (p Profile) takesFiles() bool { return p.takesModel() && len(p.Models) == 0 }

// checkModel says whether model is one of choices, p's choices now.
func (p Profile) checkModel(model string, choices []string) error {
	if !p.takesModel() {
		if model != "" {
			return fmt.Errorf("%q takes no model; start it with none", p.Name)
		}
		return nil
	}
	if slices.Contains(choices, model) {
		return nil
	}
	if len(choices) == 0 {
		return fmt.Errorf(`%q has no model %q; no model files are under the config's "models" now`, p.Name, model)
	}
	if len(choices) > namedChoices {
		return fmt.Errorf("%q has no model %q; choose one of %s and %d more", p.Name, model, quoteAll(choices[:namedChoices]), len(choices)-namedChoices)
	}
	return fmt.Errorf("%q has no model %q; choose one of %s", p.Name, model, quoteAll(choices))
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
