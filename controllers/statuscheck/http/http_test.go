// Copyright Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package http

import (
	"strings"
	"testing"

	"github.com/go-logr/logr"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
)

func Test_validateStatusCode(t *testing.T) {
	tcs := []struct {
		name     string
		criteria string
		result   int
		expect   bool
	}{
		{
			name:     "single code, correct result",
			criteria: "200",
			result:   200,
			expect:   true,
		}, {
			name:     "single code, wrong result",
			criteria: "200",
			result:   201,
			expect:   false,
		}, {
			name:     "code range, correct result",
			criteria: "200-200",
			result:   200,
			expect:   true,
		}, {
			name:     "code range, correct result",
			criteria: "200-400",
			result:   400,
			expect:   true,
		}, {
			name:     "code range, wrong result",
			criteria: "200-400",
			result:   500,
			expect:   false,
		}, {
			name:     "illegal criteria",
			criteria: "200.400",
			result:   500,
			expect:   false,
		}, {
			name:     "illegal criteria",
			criteria: "200-x",
			result:   500,
			expect:   false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ok := validateStatusCode(tc.criteria, response{statusCode: tc.result})
			if ok != tc.expect {
				t.Errorf("criteria: %s result: %d expect: %t", tc.criteria, tc.result, tc.expect)
			}
		})
	}
}

func Test_validateExpression(t *testing.T) {
	prometheusBody := `{"status":"success","data":{"result":[{"metric":{},"value":[1700000000,"0.12"]}]}}`
	tcs := []struct {
		name       string
		expression string
		body       string
		expect     bool
		reason     string
	}{
		{
			name:       "no expression",
			expression: "",
			body:       "not json",
			expect:     true,
		}, {
			name:       "prometheus value below threshold",
			expression: "double(response.data.result[0].value[1]) < 0.15",
			body:       prometheusBody,
			expect:     true,
		}, {
			name:       "prometheus value above threshold",
			expression: "double(response.data.result[0].value[1]) < 0.1",
			body:       prometheusBody,
			expect:     false,
			reason:     "expression evaluated to false",
		}, {
			name:       "json number compared with int literal",
			expression: "response.replicas >= 3",
			body:       `{"replicas":3}`,
			expect:     true,
		}, {
			name:       "string field",
			expression: `response.status == "ok"`,
			body:       `{"status":"ok"}`,
			expect:     true,
		}, {
			name:       "missing field",
			expression: `response.status == "ok"`,
			body:       `{}`,
			expect:     false,
			reason:     "evaluate expression",
		}, {
			name:       "non json body",
			expression: `response.status == "ok"`,
			body:       "ok",
			expect:     false,
			reason:     "decode response body as JSON",
		}, {
			name:       "non bool result",
			expression: "response.status",
			body:       `{"status":"ok"}`,
			expect:     false,
			reason:     "not a bool",
		}, {
			name:       "invalid expression",
			expression: "response.status ==",
			body:       `{"status":"ok"}`,
			expect:     false,
			reason:     "invalid expression",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			criteria := v1alpha1.HTTPCriteria{StatusCode: "200", Expression: tc.expression}
			ok, reason, err := validate(logr.Discard(), criteria, response{statusCode: 200, body: tc.body})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tc.expect {
				t.Errorf("expression: %s body: %s expect: %t, got reason: %s", tc.expression, tc.body, tc.expect, reason)
			}
			if !strings.Contains(reason, tc.reason) {
				t.Errorf("expected reason containing %q, got %q", tc.reason, reason)
			}
		})
	}
}

func Test_validateExpressionSkippedOnStatusCodeMismatch(t *testing.T) {
	criteria := v1alpha1.HTTPCriteria{StatusCode: "200", Expression: "true"}
	ok, reason, err := validate(logr.Discard(), criteria, response{statusCode: 500, body: "{}"})
	if err != nil || ok || reason != "unexpected status code: 500" {
		t.Errorf("got ok: %t reason: %q err: %v", ok, reason, err)
	}
}
