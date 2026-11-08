package launch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/pcfpcfpcf/steth/internal/library"
)

const (
	// modelPlaceholder in a command's argument is replaced by the chosen model.
	modelPlaceholder = "{model}"
	// placePlaceholder makes an argument repeat once for each place where
	// model files are, with the flag before it: "--models-dir", "{place}".
	placePlaceholder = "{place}"
	// portPlaceholder is replaced by the first port from the profile's Port
	// up that nothing answers on when it starts.
	portPlaceholder = "{port}"
	// namedChoices is how many choices a refusal names.
	namedChoices = 5
)

// Config is steth's config file.
type Config struct {
	Launch []Profile `json:"launch"`
	// Models are directories and .gguf files, the places that are always
	// there; the page adds more. The model files under the places are the
	// choices of every profile with {model} that lists none.
	Models []string `json:"models"`
}

// Profile is one way to start an engine, from the config file.
type Profile struct {
	Name    string            `json:"name"`
	Command []string          `json:"command"` // argv; "{model}" in any argument is replaced by the chosen model
	Models  []string          `json:"models"`  // the choices for {model}; empty when the command has none, or takes the config's model files
	Dir     string            `json:"dir"`
	Env     map[string]string `json:"env"` // added to steth's own environment
	// Port is where the engine listens: with {port} in the command, the
	// first port tried; without, a port that must not answer before a start.
	Port int `json:"port"`
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
			return Config{}, fmt.Errorf(`%s: %w; the file holds {"launch": [...], "models": [...]}, and a profile's fields are name, command, models, dir, env and port`, path, err)
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
	for _, place := range c.Models {
		if err := library.Check(place); err != nil {
			return fmt.Errorf("models: %w; list absolute paths of directories and .gguf files that are there", err)
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
		case strings.Contains(p.Command[0], placePlaceholder):
			return fmt.Errorf(`profile %q starts its command with %s; the first argument is the program`, p.Name, placePlaceholder)
		case p.Port < 0 || p.Port > maxPort:
			return fmt.Errorf(`profile %q has port %d; give a number from 1 to %d`, p.Name, p.Port, maxPort)
		case p.takesPort() && p.Port == 0:
			return fmt.Errorf(`profile %q has %s but no "port"; give the first port to try, such as "port": 8080`, p.Name, portPlaceholder)
		case !p.takesModel() && len(p.Models) > 0:
			return fmt.Errorf(`profile %q lists models but its command has no %s; put %s where the model goes, or drop "models"`, p.Name, modelPlaceholder, modelPlaceholder)
		}
		named[p.Name] = true
	}
	return nil
}

func (p Profile) takesModel() bool { return p.has(modelPlaceholder) }

// takesPort says whether p's port is picked when it starts.
func (p Profile) takesPort() bool { return p.has(portPlaceholder) }

func (p Profile) has(placeholder string) bool {
	return slices.ContainsFunc(p.Command, func(arg string) bool { return strings.Contains(arg, placeholder) })
}

// takesFiles says whether p's choices are the model files under the places.
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
		return fmt.Errorf(`%q has no model %q; no model files are under the places now`, p.Name, model)
	}
	if len(choices) > namedChoices {
		return fmt.Errorf("%q has no model %q; choose one of %s and %d more", p.Name, model, quoteAll(choices[:namedChoices]), len(choices)-namedChoices)
	}
	return fmt.Errorf("%q has no model %q; choose one of %s", p.Name, model, quoteAll(choices))
}

// argv is the command with model in place of every {model}, port in place
// of every {port}, and each argument with {place} once for each of places,
// after the flag before it.
func (p Profile) argv(model string, places []string, port int) []string {
	var argv []string
	fill := func(arg string) string {
		arg = strings.ReplaceAll(arg, portPlaceholder, strconv.Itoa(port))
		return strings.ReplaceAll(arg, modelPlaceholder, model)
	}
	for i, arg := range p.Command {
		if !strings.Contains(arg, placePlaceholder) {
			if i+1 < len(p.Command) && isFlag(arg) && strings.Contains(p.Command[i+1], placePlaceholder) {
				continue // goes before each place
			}
			argv = append(argv, fill(arg))
			continue
		}
		for _, place := range places {
			if i > 0 && isFlag(p.Command[i-1]) {
				argv = append(argv, p.Command[i-1])
			}
			argv = append(argv, fill(strings.ReplaceAll(arg, placePlaceholder, place)))
		}
	}
	return argv
}

// isFlag is an argument such as --models-dir that names what follows it.
func isFlag(arg string) bool {
	return strings.HasPrefix(arg, "-") && !strings.ContainsAny(arg, "={")
}

func quoteAll(words []string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = fmt.Sprintf("%q", w)
	}
	return strings.Join(quoted, ", ")
}
