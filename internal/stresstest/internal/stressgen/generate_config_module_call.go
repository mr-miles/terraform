package stressgen

import (
	"math/rand"

	"github.com/hashicorp/terraform/addrs"
)

// GenerateConfigModuleCall uses the given random number generator to generate
// a random ConfigModuleCall object.
func GenerateConfigModuleCall(rnd *rand.Rand, parentNS *Namespace) *ConfigModuleCall {
	addr := addrs.ModuleCall{Name: parentNS.GenerateShortName(rnd)}
	ret := &ConfigModuleCall{
		Addr:      addr,
		Arguments: make(map[addrs.InputVariable]ConfigExpr),
	}
	childNS := parentNS.ChildNamespace(addr.Name)

	// We support all three of the repetition modes for modules here: for_each
	// over a map, count with a number, and single-instance mode. However,
	// the rest of our generation strategy here works only with strings and
	// so we need to do some trickery here to produce suitable inputs for
	// the repetition arguments while still having them generate references
	// sometimes, because the repetition arguments play an important role in
	// constructing the dependency graph.
	// We achieve this as follows:
	// - for for_each, we generate a map with a random number of
	//   randomly-generated keys where each of the values is an expression
	//   randomly generated in our usual way.
	// - for count, we generate a random expression in the usual way, assume
	//   that the result will be convertable to a string (because that's our
	//   current standard) and apply some predictable string functions to it
	//   in order to deterministically derive a number.
	// Both cases therefore allow for the meta-argument to potentially depend
	// on other objects in the configuration, even though our current model
	// only allows for string dependencies directly.

	objCount := rnd.Intn(25)
	objs := make([]ConfigObject, 0, objCount+1) // +1 for the boilerplate object

	var instanceKeys []addrs.InstanceKey
	instanceKeys = []addrs.InstanceKey{addrs.NoKey}

	// We always need a boilerplate object.
	boilerplate := &ConfigBoilerplate{
		ModuleAddr: childNS.ModuleAddr,
	}
	objs = append(objs, boilerplate)

	for i := 0; i < objCount; i++ {
		obj := GenerateConfigObject(rnd, childNS)
		objs = append(objs, obj)

		if cv, ok := obj.(*ConfigVariable); ok && cv.CallerWillSet {
			// The expression comes from parentNS here because the arguments
			// are defined in the calling module, not the called module.
			chosenExpr := parentNS.GenerateExpression(rnd)
			ret.Arguments[cv.Addr] = chosenExpr
		}
	}

	ret.InstanceKeys = instanceKeys
	ret.Objects = objs

	// TODO: Also generate the nested module, with its own separate namespace
	// and then a separate registry per instance.
	return ret
}
