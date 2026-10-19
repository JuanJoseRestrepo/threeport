package v0

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// natsContainerNames are the four containers this installer writes for NATS:
// the client pod's nats-box, and the StatefulSet's server, reloader and
// metrics exporter. CockroachDB's containers live in the same file and are
// deliberately not covered here - that is a separate issue's scope.
var natsContainerNames = map[string]bool{
	"nats-box": true,
	"nats":     true,
	"reloader": true,
	"metrics":  true,
}

// TestEveryNatsContainerHasResources covers the four containers that used to
// carry a nil or empty resources placeholder.
//
// A pod with no resources.requests has nothing stopping it from starving its
// node of CPU or memory under load, and nothing giving the scheduler a number
// to bin-pack against. All four NATS containers had one of two forms of this
// gap - "resources": nil on nats-box and the reloader, "resources":
// map[string]interface{}{} (present but empty) on nats and the metrics
// exporter - both of which pass a check that only asks whether the key
// exists.
func TestEveryNatsContainerHasResources(t *testing.T) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "dependencies.go", nil, 0)
	require.NoError(t, err)

	checked := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		name, isContainer := containerName(literal)
		if !isContainer || !natsContainerNames[name] {
			return true
		}
		checked[name] = true

		resources := keyedValue(literal, `"resources"`)
		if !assert.NotNil(t, resources, "%s: container %q has no resources field at all", fileSet.Position(literal.Pos()), name) {
			return true
		}

		resourcesLit, ok := resources.(*ast.CompositeLit)
		if !assert.True(
			t, ok, "%s: container %q's resources is not a map literal - nil would leave the pod unbounded",
			fileSet.Position(literal.Pos()), name,
		) {
			return true
		}

		keys := keyedStrings(resourcesLit)
		assert.True(
			t, keys[`"requests"`], "%s: container %q sets no resource requests",
			fileSet.Position(literal.Pos()), name,
		)
		assert.True(
			t, keys[`"limits"`], "%s: container %q sets no resource limits",
			fileSet.Position(literal.Pos()), name,
		)

		return true
	})

	for name := range natsContainerNames {
		assert.True(t, checked[name], "container %q was never found in dependencies.go - this test checked nothing for it", name)
	}
}

// TestTheNatsServerHasAnActiveLivenessProbe covers the one probe that was
// left commented out.
//
// readinessProbe only ever pulls a pod from Service endpoints; it never asks
// the kubelet to restart anything. Without a liveness probe, a NATS process
// that hangs without crashing is marked NotReady forever and nothing brings
// it back.
func TestTheNatsServerHasAnActiveLivenessProbe(t *testing.T) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "dependencies.go", nil, 0)
	require.NoError(t, err)

	var found *ast.CompositeLit
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if name, isContainer := containerName(literal); isContainer && name == "nats" {
			found = literal
		}

		return true
	})
	require.NotNil(t, found, "the nats server container was not found in dependencies.go")

	assert.NotNil(
		t, keyedValue(found, `"livenessProbe"`),
		"the nats server container has no active livenessProbe - a hung-but-not-crashed "+
			"process is marked NotReady forever with nothing to restart it",
	)
}

// containerName reports the string value of a composite literal's "name"
// field, and whether it looks like a container literal - one that also
// carries an "image".
func containerName(literal *ast.CompositeLit) (string, bool) {
	if keyedValue(literal, `"image"`) == nil {
		return "", false
	}

	nameExpr := keyedValue(literal, `"name"`)
	basic, ok := nameExpr.(*ast.BasicLit)
	if !ok {
		return "", false
	}

	name, err := strconv.Unquote(basic.Value)
	if err != nil {
		return "", false
	}

	return name, true
}

// keyedValue returns the value a composite literal gives one string key.
func keyedValue(literal *ast.CompositeLit, key string) ast.Expr {
	for _, element := range literal.Elts {
		keyed, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if basic, ok := keyed.Key.(*ast.BasicLit); ok && basic.Value == key {
			return keyed.Value
		}
	}

	return nil
}

// keyedStrings returns the string keys of a composite literal, as written.
func keyedStrings(literal *ast.CompositeLit) map[string]bool {
	keys := map[string]bool{}
	for _, element := range literal.Elts {
		keyed, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if basic, ok := keyed.Key.(*ast.BasicLit); ok {
			keys[basic.Value] = true
		}
	}

	return keys
}
