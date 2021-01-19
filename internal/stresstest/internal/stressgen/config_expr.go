package stressgen

import (
	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/zclconf/go-cty/cty"
)

// ConfigExpr is an interface implemented by types that represent various
// kinds of expression that are relevant to our testing.
//
// Since stresstest is focused mainly on testing graph building and graph
// traversal behaviors, and not on expression evaluation details, we don't
// aim to cover every possible kind of expression here but should aim to model
// all kinds of expression that can contribute in some way to the graph shape.
type ConfigExpr interface {
	// BuildExpr builds the hclwrite representation of the recieving expression,
	// for inclusion in the generated configuration files.
	BuildExpr() *hclwrite.Expression

	// ExpectedValue returns the value this expression ought to return if
	// Terraform behaves correctly. This must be the specific, fully-known
	// value we expect to find in the final state, not any placeholder value
	// that might show up during planning if we were faking a computed resource
	// argument.
	ExpectedValue(reg *Registry) cty.Value
}

// ConfigExprConst is an implementation of ConfigExpr representing static,
// constant values.
type ConfigExprConst struct {
	Value cty.Value
}

var _ ConfigExpr = (*ConfigExprConst)(nil)

// BuildExpr implements ConfigExpr.BuildExpr
func (e *ConfigExprConst) BuildExpr() *hclwrite.Expression {
	return hclwrite.NewExpressionLiteral(e.Value)
}

// ExpectedValue implements ConfigExpr.ExpectedValue
func (e *ConfigExprConst) ExpectedValue(reg *Registry) cty.Value {
	return e.Value
}

// ConfigExprRef is an implementation of ConfigExpr representing a reference
// to some referencable object elsewhere in the configuration.
type ConfigExprRef struct {
	// Target is the object being referenced.
	Target addrs.Referenceable

	// Path is an optional extra set of path traversal steps into the object,
	// allowing for e.g. referring to an attribute of an object.
	Path cty.Path
}

// NewConfigExprRef constructs a new ConfigExprRef with the given base address
// and path.
func NewConfigExprRef(objAddr addrs.Referenceable, path cty.Path) *ConfigExprRef {
	return &ConfigExprRef{
		Target: objAddr,
		Path:   path,
	}
}

var _ ConfigExpr = (*ConfigExprRef)(nil)

// BuildExpr implements ConfigExpr.BuildExpr.
func (e *ConfigExprRef) BuildExpr() *hclwrite.Expression {
	// Walking backwards from an already-parsed traversal to the traversal it
	// came from is not something we typically do in normal Terraform use,
	// and so this is a pretty hacky implementation of it just mushing
	// together some utilities we have elsewhere. Perhaps we can improve on
	// this in future if we find other use-cases for doing stuff like this.
	str := e.Target.String()
	if len(e.Path) > 0 {
		// CAUTION! tfdiags.FormatCtyPath is intended for display to users
		// and doesn't guarantee to produce exactly-valid traversal source
		// code. However, it's currently good enough for our purposes here
		// because we're only using a subset of valid paths:
		// - we're not generating attribute names that require special quoting
		// - we're not trying to traverse through sets
		// - we're not trying to use unknown values in these paths
		// If any of these assumptions change in future then we might need
		// to seek a different approach here.
		pathStr := tfdiags.FormatCtyPath(e.Path)
		str = str + pathStr
	}
	traversal, diags := hclsyntax.ParseTraversalAbs([]byte(str), "", hcl.InitialPos)
	if diags.HasErrors() {
		panic("we generated an invalid traversal and thus can't parse it")
	}
	return hclwrite.NewExpressionAbsTraversal(traversal)
}

// ExpectedValue implements ConfigExpr.ExpectedValue by wrapping
// Registry.RefValue.
func (e *ConfigExprRef) ExpectedValue(reg *Registry) cty.Value {
	return reg.RefValue(e.Target, e.Path)
}
