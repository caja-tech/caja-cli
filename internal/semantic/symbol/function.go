package symbol

import "caja-cli/internal/environment"

// FunctionSymbol represents a function type in the semantic analysis,
// tracking its expected number of arguments (arity), parameter types, and return type.
type FunctionSymbol struct {
	symbolType environment.ObjectType
	arity      int
	paramTypes []Symbol
	returnType Symbol
}

// NewFunctionSymbol creates and returns a new FunctionSymbol with the specified arity, parameter types, and return type.
func NewFunctionSymbol(arity int, paramTypes []Symbol, returnType Symbol) *FunctionSymbol {
	return &FunctionSymbol{
		symbolType: environment.FUNCTION_OBJ,
		arity:      arity,
		paramTypes: paramTypes,
		returnType: returnType,
	}
}

// Equals compares this FunctionSymbol with another Symbol to determine if they represent the same function signature.
// It returns true if both are FunctionSymbols with matching arities, parameter types, and return types, or if the other symbol is ANY_OBJ.
func (fs *FunctionSymbol) Equals(other Symbol) bool {
	otherSymbol, ok := other.(*FunctionSymbol)
	if !ok {
		return false
	}

	if otherSymbol.symbolType == environment.ANY_OBJ {
		return true
	}

	if fs.symbolType != otherSymbol.symbolType {
		return false
	}

	if fs.arity != otherSymbol.arity {
		return false
	}

	for i, pt := range fs.paramTypes {
		if !pt.Equals(otherSymbol.paramTypes[i]) {
			return false
		}
	}

	if fs.returnType != nil && otherSymbol.returnType != nil {
		return fs.returnType.Equals(otherSymbol.returnType)
	}

	return fs.returnType == otherSymbol.returnType
}

// Type returns the underlying environment.ObjectType of this symbol, which is always FUNCTION_OBJ.
func (fs *FunctionSymbol) Type() environment.ObjectType {
	return fs.symbolType
}

// Arity returns the expected number of arguments for the function.
func (fs *FunctionSymbol) Arity() int {
	return fs.arity
}

// ParamTypes returns the list of symbols representing the function's parameter types.
func (fs *FunctionSymbol) ParamTypes() []Symbol {
	return fs.paramTypes
}

// ReturnType returns the symbol representing the function's return type.
func (fs *FunctionSymbol) ReturnType() Symbol {
	return fs.returnType
}
