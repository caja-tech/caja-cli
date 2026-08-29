package ast

import (
	"caja-cli/internal/pipeline/lexer"
	"strings"
)

// Node is the base interface for every element in the abstract syntax tree.
// Every node can report the literal text of its originating token and produce
// a human-readable string representation of itself.
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement represents an AST node that performs an action (e.g. an
// assignment). It embeds Node and adds the unexported statementNode marker
// method to distinguish statements from expressions at compile time.
type Statement interface {
	Node
	statementNode()
}

// Expression represents an AST node that produces a value (e.g. a number
// literal or an infix operation). It embeds Node and adds the unexported
// expressionNode marker method to distinguish expressions from statements.
type Expression interface {
	Node
	expressionNode()
}

// Program is the root node of every parsed AST. It holds the ordered list of
// top-level statements that make up the source program.
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}

	return ""
}

func (p *Program) String() string {
	output := ""
	for _, statement := range p.Statements {
		output += statement.String()
	}
	return output
}

// NilLiteral is an expression node that represents a nil constant.
// Token carries the original tokenizer.Token.
type NilLiteral struct {
	Token lexer.Token
}

func (n *NilLiteral) expressionNode()      {}
func (n *NilLiteral) TokenLiteral() string { return n.Token.Literal }
func (n *NilLiteral) String() string       { return n.Token.Literal }

// NumberLiteral is an expression node that represents a numeric constant.
// Token carries the original tokenizer.Token, and Value holds the parsed
// float64 representation of the literal.
type NumberLiteral struct {
	Token lexer.Token
	Value float64
}

func (n *NumberLiteral) expressionNode() {}
func (n *NumberLiteral) TokenLiteral() string {
	return n.Token.Literal
}
func (n *NumberLiteral) String() string { return n.Token.Literal }

// StringLiteral is an expression node that represents a string constant.
// Token carries the original tokenizer.Token, and Value holds the string content.
type StringLiteral struct {
	Token lexer.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return `"` + sl.Value + `"` }

// BooleanLiteral is an expression node that represents a boolean constant.
// Token carries the original tokenizer.Token, and Value holds the boolean value.
type BooleanLiteral struct {
	Token lexer.Token
	Value bool
}

func (b *BooleanLiteral) expressionNode()      {}
func (b *BooleanLiteral) TokenLiteral() string { return b.Token.Literal }
func (b *BooleanLiteral) String() string       { return b.Token.Literal }

// DateLiteral is an expression node that represents a date constant.
// Token carries the original tokenizer.Token, and Value holds the date content.
type DateLiteral struct {
	Token lexer.Token
	Value string
}

func (dl *DateLiteral) expressionNode()      {}
func (dl *DateLiteral) TokenLiteral() string { return dl.Token.Literal }
func (dl *DateLiteral) String() string       { return "'" + dl.Value + "'" }

// ArrayLiteral is an expression node that represents a list of values.
// Token carries the original tokenizer.Token (typically '['), and Elements
// holds the ordered slice of expressions that make up the array's contents.
type ArrayLiteral struct {
	Token    lexer.Token
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	out := "["
	for i, el := range al.Elements {
		out += el.String()
		if i != len(al.Elements)-1 {
			out += ", "
		}
	}
	out += "]"
	return out
}

// Parameter represents a single parameter in a function declaration, containing
// its Name and expected Type.
type Parameter struct {
	Name string
	Type string
}

func (p *Parameter) String() string {
	return p.Name + ": " + p.Type
}

// FunctionLiteral is an expression node that represents a function definition.
// It includes the parameters, optional return type, and the function body.
type FunctionLiteral struct {
	Token          lexer.Token
	TypeParameters []string
	Parameters     []*Parameter
	ReturnType     string
	Body           *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	out := fl.Token.Literal

	if len(fl.TypeParameters) > 0 {
		out += "<" + strings.Join(fl.TypeParameters, ", ") + ">"
	}

	out += "("

	if len(fl.Parameters) > 0 {
		for i, param := range fl.Parameters {
			out += param.String()
			if i != len(fl.Parameters)-1 {
				out += ", "
			}
		}
	}

	out += ")"
	if fl.ReturnType != "" {
		out += " -> " + fl.ReturnType
	}
	out += " { ... }"
	return out
}

// StructLiteral is an expression node representing a struct instantiation.
// It holds the struct name identifier and a map of provided field values.
type StructLiteral struct {
	Token         lexer.Token // The '{' token
	StructName    string
	TypeArguments []string
	Fields        map[string]Expression
}

func (sl *StructLiteral) expressionNode()      {}
func (sl *StructLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StructLiteral) String() string {
	var out string
	structName := sl.StructName
	if len(sl.TypeArguments) > 0 {
		structName += "::<" + strings.Join(sl.TypeArguments, ", ") + ">"
	}
	out += structName + " {"
	var fields []string
	for name, val := range sl.Fields {
		fields = append(fields, name+": "+val.String())
	}
	out += strings.Join(fields, ", ")
	out += "}"
	return out
}

// LetStatement is a statement node that represents a variable declaration
// and initialization (e.g. "let rate = 100"). Token holds the "let" keyword token,
// Name is the target identifier, and Value is the right-hand-side expression.
type LetStatement struct {
	Token     lexer.Token
	Name      *Identifier
	Value     Expression
	IsPrivate bool
	ValueType string
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) String() string {
	var out string
	if ls.IsPrivate {
		out += "private "
	}
	out += ls.TokenLiteral() + " " + ls.Name.String()

	if ls.ValueType != "" {
		out += ": " + ls.ValueType
	}

	out += " = "

	if ls.Value != nil {
		out += ls.Value.String()
	}
	return out
}

// ConstStatement is a statement node that represents an immutable variable declaration
// and initialization (e.g. "const rate = 100"). Token holds the "const" keyword token,
// Name is the target identifier, and Value is the right-hand-side expression.
type ConstStatement struct {
	Token     lexer.Token
	Name      *Identifier
	Value     Expression
	IsPrivate bool
	ValueType string
}

func (cs *ConstStatement) statementNode()       {}
func (cs *ConstStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ConstStatement) String() string {
	var out string
	if cs.IsPrivate {
		out += "private "
	}
	out += cs.TokenLiteral() + " " + cs.Name.String()

	if cs.ValueType != "" {
		out += ": " + cs.ValueType
	}

	out += " = "

	if cs.Value != nil {
		out += cs.Value.String()
	}
	return out
}

// ImportStatement represents an import declaration (e.g. "import second").
// Token holds the "import" keyword token, and Name is the module identifier.
type ImportStatement struct {
	Token lexer.Token
	Name  *Identifier
	Path  string
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) String() string {
	if is.Path != "" && is.Path != is.Name.Value {
		// If path is a string literal and basename is not the bound name, we might have an alias or just standard string behavior.
		// A reliable way to format: if alias is used, it should print `import "path" as alias`
		basename := is.Path
		for i := len(basename) - 1; i >= 0; i-- {
			if basename[i] == '/' {
				basename = basename[i+1:]
				break
			}
		}
		if is.Name.Value != basename {
			return is.TokenLiteral() + " \"" + is.Path + "\" as " + is.Name.Value
		}
		return is.TokenLiteral() + " \"" + is.Path + "\""
	}
	if is.Path != is.Name.Value {
		// Ident alias: import mod as alias
		return is.TokenLiteral() + " " + is.Path + " as " + is.Name.Value
	}
	return is.TokenLiteral() + " " + is.Name.String()
}

// ReturnStatement is a statement node that represents an explicit return from
// the script (e.g. "return rate * 2"). Token holds the "return" keyword token
// and ReturnValue is the expression whose result becomes the script's output.
type ReturnStatement struct {
	Token       lexer.Token
	ReturnValue Expression
}

func (r *ReturnStatement) statementNode()       {}
func (r *ReturnStatement) TokenLiteral() string { return r.Token.Literal }
func (r *ReturnStatement) String() string {
	if r.ReturnValue != nil {
		return r.TokenLiteral() + " " + r.ReturnValue.String()
	}
	return r.TokenLiteral()
}

// AssignStatement is a statement node that binds the result of an expression
// to an identifier (e.g. "rate = 100 / 2"). Token holds the '=' token, Name
// is the target identifier, and Value is the right-hand-side expression.
type AssignStatement struct {
	Token lexer.Token
	Name  *Identifier
	Value Expression
}

func (a *AssignStatement) statementNode() {}
func (a *AssignStatement) TokenLiteral() string {
	return a.Token.Literal
}
func (a *AssignStatement) String() string {
	return a.Name.String() + " = " + a.Value.String()
}

// BlockStatement is a statement node that represents a sequence of statements
// grouped together, typically enclosed in curly braces. Token holds the '{' token,
// and Statements contains the ordered list of statements within the block.
type BlockStatement struct {
	Token      lexer.Token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out string
	for _, s := range bs.Statements {
		out += s.String()
	}
	return out
}

// FunctionSignature represents the expected type signature of a function,
// including its parameter types and optional return type.
type FunctionSignature struct {
	ParamTypes []string
	ReturnType string
}

func (fs *FunctionSignature) String() string {
	out := "fn("

	if len(fs.ParamTypes) > 0 {
		for i, param := range fs.ParamTypes {
			out += param
			if i != len(fs.ParamTypes)-1 {
				out += ", "
			}
		}
	}

	out += ")"
	if fs.ReturnType != "" {
		out += " -> " + fs.ReturnType
	}

	return out
}

// StructField represents a single field definition within a struct type alias,
// holding the field's name, expected type string, and constant modifier.
type StructField struct {
	Name       *Identifier
	Type       string
	IsConstant bool
}

func (sf *StructField) String() string {
	if sf.IsConstant {
		return "const " + sf.Name.String() + " " + sf.Type
	}
	return sf.Name.String() + " " + sf.Type
}

// StructDefinition represents the body of a struct type declaration,
// containing the struct keyword token and a slice of field definitions.
type StructDefinition struct {
	Token  lexer.Token // The 'struct' token
	Fields []StructField
}

func (sd *StructDefinition) String() string {
	var out string
	out += "struct {\n"
	for _, f := range sd.Fields {
		out += "  " + f.String() + "\n"
	}
	out += "}"
	return out
}

// TypeAliasStatement represents a top-level type alias declaration
// (e.g. "type BinaryOp fn(Number, Number): Number"). It holds the "type" token,
// Name is the new alias identifier, and Signature is the function signature.
type TypeAliasStatement struct {
	Token            lexer.Token
	Name             *Identifier
	TypeParameters   []string           // Used for generic aliases like type Map<K, V> ...
	Signature        *FunctionSignature // Used for fn(...) types
	TargetType       string             // Used for simple alias types like Number, [String], etc.
	StructDefinition *StructDefinition  // Used for struct types
	IsPrivate        bool
}

func (ts *TypeAliasStatement) statementNode()       {}
func (ts *TypeAliasStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *TypeAliasStatement) String() string {
	var out string
	if ts.IsPrivate {
		out += "private "
	}

	namePart := ts.Name.String()
	if len(ts.TypeParameters) > 0 {
		namePart += "<" + strings.Join(ts.TypeParameters, ", ") + ">"
	}

	if ts.Signature != nil {
		out += ts.TokenLiteral() + " " + namePart + " " + ts.Signature.String()
	} else if ts.StructDefinition != nil {
		out += ts.TokenLiteral() + " " + namePart + " " + ts.StructDefinition.String()
	} else {
		out += ts.TokenLiteral() + " " + namePart + " " + ts.TargetType
	}

	return out
}

// ExpressionStatement wraps a standalone expression that appears as a
// top-level statement (e.g. "rate + 50"). Token is the first token of the
// expression, and Expression holds the parsed expression tree.
type ExpressionStatement struct {
	Token      lexer.Token
	Expression Expression
}

func (e *ExpressionStatement) statementNode() {}
func (e *ExpressionStatement) TokenLiteral() string {
	return e.Token.Literal
}
func (e *ExpressionStatement) String() string {
	return e.Expression.String()
}

// Identifier is an expression node that represents a named variable reference.
// Token carries the original tokenizer.Token and Value holds the identifier's
// name as a plain string.
type Identifier struct {
	Token lexer.Token // The lexer.IDENT token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

// GenericIdentifier represents an identifier with generic type arguments,
// e.g. CustomStruct::<String>. Used temporarily during parsing to pass
// turbofish arguments to StructLiteral.
type GenericIdentifier struct {
	Token         lexer.Token // The '::' token
	Identifier    *Identifier
	TypeArguments []string
}

func (gi *GenericIdentifier) expressionNode()      {}
func (gi *GenericIdentifier) TokenLiteral() string { return gi.Token.Literal }
func (gi *GenericIdentifier) String() string {
	return gi.Identifier.Value + "::<" + strings.Join(gi.TypeArguments, ", ") + ">"
}

// PrefixExpression represents a prefix operator and its operand expression.
type PrefixExpression struct {
	Token    lexer.Token // The prefix token, e.g.: ! or -
	Operator string
	Right    Expression
}

func (p *PrefixExpression) expressionNode()      {}
func (p *PrefixExpression) TokenLiteral() string { return p.Token.Literal }
func (p *PrefixExpression) String() string {
	return "(" + p.Operator + p.Right.String() + ")"
}

// InfixExpression is an expression node for binary operations such as
// addition, subtraction, multiplication, and division. It stores the operator
// token, the operator symbol as a string, and the left and right operands.
type InfixExpression struct {
	Token    lexer.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (i *InfixExpression) expressionNode() {}
func (i *InfixExpression) TokenLiteral() string {
	return i.Token.Literal
}
func (i *InfixExpression) String() string {
	return "(" + i.Left.String() + " " + i.Operator + " " + i.Right.String() + ")"
}

// IfExpression is an expression node that represents a conditional branching
// construct. Token holds the 'if' token, Condition is the expression to be
// evaluated, Consequence is the block of statements to execute if the condition
// is true, and Alternative is the optional block of statements to execute if
// the condition is false.
type IfExpression struct {
	Token       lexer.Token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) String() string {
	out := "if " + ie.Condition.String() + " " + ie.Consequence.String()
	if ie.Alternative != nil {
		out += " else " + ie.Alternative.String()
	}
	return out
}

// CallExpression is an expression node that represents a function invocation.
// It contains the function being called and the list of argument expressions.
type CallExpression struct {
	Token         lexer.Token // The '(' token or the '::' token
	Function      Expression  // Identifier or FunctionLiteral
	TypeArguments []string    // Generic type arguments e.g., f::<Number>()
	Arguments     []Expression
	RParenToken   lexer.Token
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	out := ce.Function.String()
	
	if len(ce.TypeArguments) > 0 {
		out += "::<"
		for i, t := range ce.TypeArguments {
			out += t
			if i != len(ce.TypeArguments)-1 {
				out += ", "
			}
		}
		out += ">"
	}
	
	out += "("

	if len(ce.Arguments) > 0 {
		for i, arg := range ce.Arguments {
			out += arg.String()
			if i != len(ce.Arguments)-1 {
				out += ", "
			}
		}
	}

	out += ")"

	return out
}

// IndexExpression is an expression node that represents an array or bracket
// notation lookup. Token carries the original tokenizer.Token (typically '['),
// Left is the expression being indexed into, and Index is the expression evaluating to the index.
type IndexExpression struct {
	Token lexer.Token
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	return "(" + ie.Left.String() + "[" + ie.Index.String() + "])"
}

// IndexAssignmentStatement binds an expression result to an array index.
type IndexAssignmentStatement struct {
	Token lexer.Token // The '=' token
	Left  Expression  // The array being indexed
	Index Expression  // The index expression
	Value Expression  // The value being assigned
}

func (i *IndexAssignmentStatement) statementNode()       {}
func (i *IndexAssignmentStatement) TokenLiteral() string { return i.Token.Literal }
func (i *IndexAssignmentStatement) String() string {
	return i.Left.String() + "[" + i.Index.String() + "] = " + i.Value.String()
}

// PropertyExpression represents a property access via dot notation (e.g. "second.myFunc").
// Token holds the "." token, Object is the expression being accessed, and Property is the identifier.
type PropertyExpression struct {
	Token    lexer.Token
	Object   Expression
	Property *Identifier
	Safe     bool
}

func (pe *PropertyExpression) expressionNode()      {}
func (pe *PropertyExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PropertyExpression) String() string {
	if pe.Safe {
		return "(" + pe.Object.String() + "?." + pe.Property.String() + ")"
	}
	return "(" + pe.Object.String() + "." + pe.Property.String() + ")"
}

// PropertyAssignmentStatement binds an expression result to an object's property.
type PropertyAssignmentStatement struct {
	Token    lexer.Token // The '=' token
	Object   Expression  // The object being modified
	Property *Identifier // The specific property to reassign
	Value    Expression  // The value being assigned
	Safe     bool        // True if accessed with ?.
}

func (p *PropertyAssignmentStatement) statementNode()       {}
func (p *PropertyAssignmentStatement) TokenLiteral() string { return p.Token.Literal }
func (p *PropertyAssignmentStatement) String() string {
	if p.Safe {
		return p.Object.String() + "?." + p.Property.String() + " = " + p.Value.String()
	}
	return p.Object.String() + "." + p.Property.String() + " = " + p.Value.String()
}

// MapLiteral represents a dictionary/map expression.
type MapLiteral struct {
	Token lexer.Token // the '{' token
	Pairs map[Expression]Expression
}

func (ml *MapLiteral) expressionNode()      {}
func (ml *MapLiteral) TokenLiteral() string { return ml.Token.Literal }
func (ml *MapLiteral) String() string {
	var out string
	out += "{"
	var pairs []string
	for key, value := range ml.Pairs {
		pairs = append(pairs, key.String()+": "+value.String())
	}
	out += strings.Join(pairs, ", ")
	out += "}"
	return out
}
