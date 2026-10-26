// Package prom reads the Prometheus text format engines serve on /metrics.
package prom

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Sample is one line of a metrics body: a name, its labels and its value.
type Sample struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// Parse reads every sample of a metrics body, skipping comments.
func Parse(body []byte) ([]Sample, error) {
	var samples []Sample
	lines := bufio.NewScanner(bytes.NewReader(body))
	for n := 1; lines.Scan(); n++ {
		line := strings.TrimSpace(lines.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		s, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("metrics line %d: %w", n, err)
		}
		samples = append(samples, s)
	}
	return samples, lines.Err()
}

func parseLine(line string) (Sample, error) {
	s := Sample{Labels: map[string]string{}}
	end := strings.IndexAny(line, "{ ")
	if end < 0 {
		return s, fmt.Errorf("expected a name and a value, got %q", line)
	}
	s.Name, line = line[:end], line[end:]
	if strings.HasPrefix(line, "{") {
		rest, err := parseLabels(line[1:], s.Labels)
		if err != nil {
			return s, err
		}
		line = rest
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return s, fmt.Errorf("%s: expected a value", s.Name)
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return s, fmt.Errorf("%s: expected a number, got %q", s.Name, fields[0])
	}
	s.Value = v
	return s, nil
}

// parseLabels reads `a="x",b="y"}` into `labels` and returns what follows.
func parseLabels(text string, labels map[string]string) (string, error) {
	for {
		text = strings.TrimLeft(text, " ,")
		if strings.HasPrefix(text, "}") {
			return text[1:], nil
		}
		name, rest, found := strings.Cut(text, "=\"")
		if !found {
			return "", fmt.Errorf("expected name=\"value\" in labels, got %q", text)
		}
		var value strings.Builder
		i := 0
		for ; i < len(rest) && rest[i] != '"'; i++ {
			if rest[i] == '\\' && i+1 < len(rest) {
				i++
				switch rest[i] {
				case 'n':
					value.WriteByte('\n')
				default:
					value.WriteByte(rest[i])
				}
				continue
			}
			value.WriteByte(rest[i])
		}
		if i == len(rest) {
			return "", fmt.Errorf("label %s: unterminated value", name)
		}
		labels[strings.TrimSpace(name)] = value.String()
		text = rest[i+1:]
	}
}

// Sum adds the samples named `name` whose labels include all of `match`; it
// is false when there are none.
func Sum(samples []Sample, name string, match map[string]string) (float64, bool) {
	var total float64
	found := false
	for _, s := range samples {
		if s.Name != name || !matches(s.Labels, match) {
			continue
		}
		total += s.Value
		found = true
	}
	return total, found
}

func matches(labels, match map[string]string) bool {
	for k, v := range match {
		if labels[k] != v {
			return false
		}
	}
	return true
}
