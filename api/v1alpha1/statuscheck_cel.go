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

package v1alpha1

import (
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/pkg/errors"
)

// HTTPCriteriaResponseVariable is the name of the variable that holds the
// JSON-decoded response body in an HTTP criteria expression.
const HTTPCriteriaResponseVariable = "response"

// httpCriteriaCostLimit caps the runtime cost of a single evaluation, so an
// expression can't spin on a large response body.
const httpCriteriaCostLimit = 1000000

var httpCriteriaEnv = sync.OnceValues(func() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable(HTTPCriteriaResponseVariable, cel.DynType),
		cel.CrossTypeNumericComparisons(true),
	)
})

// CompileHTTPCriteriaExpression compiles an HTTP criteria expression and checks
// that it evaluates to a bool.
func CompileHTTPCriteriaExpression(expression string) (cel.Program, error) {
	env, err := httpCriteriaEnv()
	if err != nil {
		return nil, errors.Wrap(err, "create CEL environment")
	}
	ast, issues := env.Compile(expression)
	if issues.Err() != nil {
		return nil, issues.Err()
	}
	if outputType := ast.OutputType(); !outputType.IsExactType(cel.BoolType) && !outputType.IsExactType(cel.DynType) {
		return nil, errors.Errorf("expression must evaluate to a bool, got %s", outputType)
	}
	return env.Program(ast, cel.CostLimit(httpCriteriaCostLimit))
}
