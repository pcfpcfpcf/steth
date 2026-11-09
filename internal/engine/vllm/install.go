package vllm

import "github.com/pcfpcfpcf/steth/internal/installed"

// Binaries is the program that serves vLLM. It is not asked a version:
// `vllm --version` imports torch, which takes seconds. Its {model} is a
// model file from the places; vLLM reads GGUF only experimentally, so a
// start may need a config profile naming a Hugging Face model instead.
var Binaries = []installed.Binary{{
	Name:  "vllm",
	Serve: []string{"serve", "{model}", "--port", "{port}"},
	Port:  Ports[0],
}}
