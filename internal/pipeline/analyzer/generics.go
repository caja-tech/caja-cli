package analyzer

import (
	"caja-cli/internal/pipeline/analyzer/symbol"
	"fmt"
)

// inferTypes recursively walks the expected type and the actual type to resolve
// any GenericSymbols. It populates the inferred map.
func inferTypes(expected symbol.Symbol, actual symbol.Symbol, inferred map[string]symbol.Symbol) error {
	// If expected is a GenericSymbol, try to bind or verify it
	if gen, ok := expected.(*symbol.GenericSymbol); ok {
		if existing, exists := inferred[gen.Name]; exists {
			// If we already inferred this type, the new actual type must match it.
			// e.g. fn(T, T) called with (Number, String) would fail here.
			if !existing.Equals(actual) {
				return fmt.Errorf("conflicting types for %s: %s and %s", gen.Name, existing.String(), actual.String())
			}
			return nil
		}
		// Bind new type
		inferred[gen.Name] = actual
		return nil
	}

	// If expected is an ArraySymbol
	if expArr, ok := expected.(*symbol.ArraySymbol); ok {
		if actArr, ok := actual.(*symbol.ArraySymbol); ok {
			return inferTypes(expArr.ElementSymbol(), actArr.ElementSymbol(), inferred)
		}
	}

	// If expected is a MapSymbol
	if expMap, ok := expected.(*symbol.MapSymbol); ok {
		if actMap, ok := actual.(*symbol.MapSymbol); ok {
			if err := inferTypes(expMap.Key, actMap.Key, inferred); err != nil {
				return err
			}
			return inferTypes(expMap.Value, actMap.Value, inferred)
		}
	}

	// If expected is a FunctionSymbol
	if expFn, ok := expected.(*symbol.FunctionSymbol); ok {
		if actFn, ok := actual.(*symbol.FunctionSymbol); ok {
			if expFn.Arity() != actFn.Arity() {
				return fmt.Errorf("arity mismatch")
			}
			for i, pType := range expFn.ParamTypes() {
				if err := inferTypes(pType, actFn.ParamTypes()[i], inferred); err != nil {
					return err
				}
			}
			return inferTypes(expFn.ReturnType(), actFn.ReturnType(), inferred)
		}
	}

	// Basic types, Nullable, etc... just pass as they are if no generics inside.
	// For nullable, we unwrap if both are nullable
	if expNull, ok := expected.(*symbol.NullableSymbol); ok {
		if actNull, ok := actual.(*symbol.NullableSymbol); ok {
			return inferTypes(expNull.Underlying, actNull.Underlying, inferred)
		}
	}

	return nil
}

// substituteTypes replaces all GenericSymbols in the target with their bound concrete types.
func substituteTypes(target symbol.Symbol, inferred map[string]symbol.Symbol) symbol.Symbol {
	if target == nil {
		return nil
	}

	if gen, ok := target.(*symbol.GenericSymbol); ok {
		if concrete, exists := inferred[gen.Name]; exists {
			return concrete
		}
		// If not found, leave it as Any or as is
		return symbol.AnySymbol()
	}

	if arr, ok := target.(*symbol.ArraySymbol); ok {
		return symbol.NewArraySymbol(substituteTypes(arr.ElementSymbol(), inferred))
	}

	if m, ok := target.(*symbol.MapSymbol); ok {
		return symbol.NewMapSymbol(substituteTypes(m.Key, inferred), substituteTypes(m.Value, inferred))
	}

	if fn, ok := target.(*symbol.FunctionSymbol); ok {
		var newParams []symbol.Symbol
		for _, p := range fn.ParamTypes() {
			newParams = append(newParams, substituteTypes(p, inferred))
		}
		return symbol.NewFunctionSymbol(fn.TypeParameters, fn.Arity(), newParams, substituteTypes(fn.ReturnType(), inferred))
	}

	if nullType, ok := target.(*symbol.NullableSymbol); ok {
		return &symbol.NullableSymbol{Underlying: substituteTypes(nullType.Underlying, inferred)}
	}

	if structDef, ok := target.(*symbol.StructDefSymbol); ok {
		newFields := make(map[string]symbol.StructFieldSymbol)
		for name, field := range structDef.Fields {
			newFields[name] = symbol.StructFieldSymbol{
				Type:       substituteTypes(field.Type, inferred),
				IsConstant: field.IsConstant,
			}
		}
		return symbol.NewStructDefSymbol(structDef.Name, nil, newFields)
	}

	return target
}
