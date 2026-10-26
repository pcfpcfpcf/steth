package prom

import "testing"

const body = `# HELP vllm:num_requests_running Number of requests in model execution batches.
# TYPE vllm:num_requests_running gauge
vllm:num_requests_running{engine="0",model_name="qwen"} 2.0
vllm:num_requests_running{engine="1",model_name="qwen"} 1.0
vllm:num_requests_running{engine="0",model_name="other"} 5.0
vllm:cache_config_info{block_size="16",note="a \"quoted\", odd} label\\n"} 1.0
llamacpp:requests_deferred 3
vllm:kv_cache_usage_perc{model_name="qwen"} NaN
`

func TestParse(t *testing.T) {
	samples, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 6 {
		t.Fatalf("%d samples, expected 6", len(samples))
	}
	if got := samples[3].Labels["note"]; got != `a "quoted", odd} label\n` {
		t.Errorf("label %q", got)
	}
	if samples[4].Name != "llamacpp:requests_deferred" || samples[4].Value != 3 {
		t.Errorf("unlabelled sample %+v", samples[4])
	}
	if running, ok := Sum(samples, "vllm:num_requests_running", map[string]string{"model_name": "qwen"}); !ok || running != 3 {
		t.Errorf("running %v, %v; expected 3 over both engines", running, ok)
	}
	if _, ok := Sum(samples, "vllm:num_requests_waiting", nil); ok {
		t.Error("a metric not there was found")
	}
}

func TestParseRejectsBadValue(t *testing.T) {
	if _, err := Parse([]byte("vllm:x{a=\"b\"} lots\n")); err == nil {
		t.Error("expected an error for a value that is not a number")
	}
}
