package compiler

import (
	"bytes"
	"caja-cli/internal/pipeline/ast"
	"fmt"
	"strings"
)

var builtinModules = map[string]bool{
	"array":  true,
	"date":   true,
	"string": true,
	"math":   true,
	"log":    true,
	"map":    true,
	"cast":   true,
}

func isOwned(n ast.Expression) bool {
	if prefix, ok := n.(*ast.PrefixExpression); ok && prefix.Operator == "move" {
		return true
	}
	switch n.(type) {
	case *ast.CallExpression, *ast.SafePipeExpression:
		return true
	}
	return false
}

func transpileBuiltinCall(module string, fn string, args []ast.Expression, ctx *transpileContext) (string, error) {
	ctx.usedModules[module] = true

	var argStrs []string
	for _, arg := range args {
		s, err := transpileExpression(arg, ctx, "")
		if err != nil {
			return "", err
		}
		argStrs = append(argStrs, s)
	}

	switch module {
	case "math":
		if fn == "log" {
			return fmt.Sprintf("(math.Log(%s) / math.Log(%s))", argStrs[0], argStrs[1]), nil
		}
		if fn == "rand" {
			ctx.usedModules["math_rand"] = true
			return "rand.Float64()", nil
		}
		return fmt.Sprintf("math.%s(%s)", strings.Title(fn), strings.Join(argStrs, ", ")), nil

	case "string":
		ctx.usedModules["strings"] = true
		switch fn {
		case "len":
			ctx.usedModules["utf8"] = true
			return fmt.Sprintf("float64(utf8.RuneCountInString(%s))", argStrs[0]), nil
		case "charAt":
			return fmt.Sprintf("string([]rune(%s)[int(%s)])", argStrs[0], argStrs[1]), nil
		case "substring":
			return fmt.Sprintf("string([]rune(%s)[int(%s):int(%s)])", argStrs[0], argStrs[1], argStrs[2]), nil
		case "concat":
			return fmt.Sprintf("(%s + %s)", argStrs[0], argStrs[1]), nil
		case "split":
			return fmt.Sprintf("strings.Split(%s, %s)", argStrs[0], argStrs[1]), nil
		case "contains":
			return fmt.Sprintf("strings.Contains(%s, %s)", argStrs[0], argStrs[1]), nil
		case "startsWith":
			return fmt.Sprintf("strings.HasPrefix(%s, %s)", argStrs[0], argStrs[1]), nil
		case "endsWith":
			return fmt.Sprintf("strings.HasSuffix(%s, %s)", argStrs[0], argStrs[1]), nil
		case "replace":
			return fmt.Sprintf("strings.ReplaceAll(%s, %s, %s)", argStrs[0], argStrs[1], argStrs[2]), nil
		case "toUpper":
			return fmt.Sprintf("strings.ToUpper(%s)", argStrs[0]), nil
		case "toLower":
			return fmt.Sprintf("strings.ToLower(%s)", argStrs[0]), nil
		case "trim":
			return fmt.Sprintf("strings.TrimSpace(%s)", argStrs[0]), nil
		case "join":
			return fmt.Sprintf("strings.Join(%s, %s)", argStrs[0], argStrs[1]), nil
		}

	case "array":
		switch fn {
		case "len":
			return fmt.Sprintf("float64(len(%s))", argStrs[0]), nil
		case "head":
			return fmt.Sprintf("%s[0]", argStrs[0]), nil
		case "last":
			return fmt.Sprintf("%s[len(%s)-1]", argStrs[0], argStrs[0]), nil
		case "push":
			if isOwned(args[0]) {
				return fmt.Sprintf("append(%s, %s)", argStrs[0], argStrs[1]), nil
			}
			return fmt.Sprintf("caja_array_push(%s, %s)", argStrs[0], argStrs[1]), nil
		case "pop":
			if isOwned(args[0]) {
				return fmt.Sprintf("%s[:len(%s)-1]", argStrs[0], argStrs[0]), nil
			}
			return fmt.Sprintf("caja_array_pop(%s)", argStrs[0]), nil
		default:
			return fmt.Sprintf("caja_array_%s(%s)", fn, strings.Join(argStrs, ", ")), nil
		}

	case "date":
		ctx.usedModules["time"] = true
		switch fn {
		case "today":
			return "caja_date_today()", nil
		case "new":
			return fmt.Sprintf("time.Date(int(%s), time.Month(%s), int(%s), 0, 0, 0, 0, time.UTC)", argStrs[0], argStrs[1], argStrs[2]), nil
		case "addDays":
			return fmt.Sprintf("%s.AddDate(0, 0, int(%s))", argStrs[0], argStrs[1]), nil
		case "diffDays":
			return fmt.Sprintf("float64(%s.Sub(%s).Hours() / 24)", argStrs[1], argStrs[0]), nil
		case "parse":
			return fmt.Sprintf("parseDate(%s)", argStrs[0]), nil
		case "year":
			return fmt.Sprintf("float64(%s.Year())", argStrs[0]), nil
		case "month":
			return fmt.Sprintf("float64(%s.Month())", argStrs[0]), nil
		case "day":
			return fmt.Sprintf("float64(%s.Day())", argStrs[0]), nil
		case "weekday":
			return fmt.Sprintf("float64(%s.Weekday())", argStrs[0]), nil
		}

	case "map":
		switch fn {
		case "containsKey":
			return fmt.Sprintf("caja_map_containsKey(%s, %s)", argStrs[0], argStrs[1]), nil
		case "delete":
			return fmt.Sprintf("caja_map_delete(%s, %s)", argStrs[0], argStrs[1]), nil
		}

	case "log":
		switch fn {
		case "export":
			ctx.usedModules["log_export"] = true
			ctx.usedModules["json"] = true
			ctx.usedModules["fmt"] = true
			return fmt.Sprintf("caja_log_export(%s)", argStrs[0]), nil
		case "info", "warn", "error":
			ctx.usedModules["log_" + fn] = true
			ctx.usedModules["fmt"] = true
			return fmt.Sprintf("caja_log_%s(%s, %s)", fn, argStrs[0], argStrs[1]), nil
		}
		
	case "cast":
		if fn == "to" {
			ctx.usedModules["fmt"] = true
			
			fallbackType := ""
			switch args[1].(type) {
			case *ast.StringLiteral:
				fallbackType = "String"
			case *ast.NumberLiteral:
				fallbackType = "Number"
			case *ast.BooleanLiteral:
				fallbackType = "Boolean"
			default:
				if fallbackSym, _ := ctx.analyzer.GetSymbol(args[1]); fallbackSym != nil {
					fallbackType = string(fallbackSym.Type())
				}
			}
			
			inputType := ""
			if inputSym, _ := ctx.analyzer.GetSymbol(args[0]); inputSym != nil {
				inputType = string(inputSym.Type())
			}

			// Generate the value string representation based on input type
			valStr := fmt.Sprintf("fmt.Sprintf(\"%%v\", %s)", argStrs[0])
			if inputType == "Date" {
				valStr = fmt.Sprintf("%s.Format(\"2006-01-02\")", argStrs[0])
			} else if inputType == "String" {
				valStr = argStrs[0]
			}
			
			switch fallbackType {
			case "String":
				return valStr, nil
			case "Number":
				if inputType == "Number" { return argStrs[0], nil }
				ctx.usedModules["strconv"] = true
				return fmt.Sprintf("func() float64 { v, err := strconv.ParseFloat(%s, 64); if err != nil { return %s }; return v }()", valStr, argStrs[1]), nil
			case "Boolean":
				if inputType == "Boolean" { return argStrs[0], nil }
				ctx.usedModules["strconv"] = true
				return fmt.Sprintf("func() bool { v, err := strconv.ParseBool(%s); if err != nil { return %s }; return v }()", valStr, argStrs[1]), nil
			case "Date":
				if inputType == "Date" { return argStrs[0], nil }
				ctx.usedModules["time"] = true
				return fmt.Sprintf("func() time.Time { v, err := time.Parse(\"2006-01-02\", %s); if err != nil { return %s }; return v }()", valStr, argStrs[1]), nil
			}
			
			return fmt.Sprintf("%s", argStrs[0]), nil
		}
	}

	return "", fmt.Errorf("unsupported builtin %s.%s", module, fn)
}

func injectBuiltinDependencies(ctx *transpileContext, buf *bytes.Buffer) {
	if ctx.usedModules["array"] {
		buf.WriteString(`
func caja_array_push[T any](arr []T, item T) []T { 
	res := append([]T{}, arr...)
	return append(res, item) 
}
func caja_array_pop[T any](arr []T) []T {
	if len(arr) == 0 { return arr }
	res := append([]T{}, arr...)
	return res[:len(res)-1]
}
func caja_array_tail[T any](arr []T) []T { 
	if len(arr) <= 1 { return []T{} }
	res := append([]T{}, arr...)
	return res[1:]
}
func caja_array_copy[T any](arr []T) []T {
	res := make([]T, len(arr))
	copy(res, arr)
	return res
}
func caja_array_slice[T any](arr []T, start float64, end float64) []T {
	if int(start) < 0 || int(end) > len(arr) || int(start) > int(end) {
		return []T{}
	}
	res := append([]T{}, arr...)
	return res[int(start):int(end)]
}
func caja_array_join[T any](arr []T, other []T) []T {
	res := append([]T{}, arr...)
	return append(res, other...)
}
`)
	}
	
	if ctx.usedModules["map"] {
		buf.WriteString(`
func caja_map_containsKey[K comparable, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}
func caja_map_delete[K comparable, V any](m map[K]V, key K) map[K]V {
	res := make(map[K]V)
	for k, v := range m {
		res[k] = v
	}
	delete(res, key)
	return res
}
`)
	}
	
	if ctx.usedModules["time"] {
		buf.WriteString(`
func caja_date_today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
`)
	}
	
	if ctx.usedModules["log_export"] {
		buf.WriteString(`
func caja_log_export(v any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}
`)
	}
	if ctx.usedModules["log_info"] {
		buf.WriteString(`
func caja_log_info(msg string, args any) {
	fmt.Printf("[INFO] %s %+v\n", msg, args)
}
`)
	}
	if ctx.usedModules["log_warn"] {
		buf.WriteString(`
func caja_log_warn(msg string, args any) {
	fmt.Printf("[WARN] %s %+v\n", msg, args)
}
`)
	}
	if ctx.usedModules["log_error"] {
		buf.WriteString(`
func caja_log_error(msg string, args any) {
	fmt.Printf("[ERROR] %s %+v\n", msg, args)
}
`)
	}
}

func transpileBuiltinProperty(module string, prop string, ctx *transpileContext) (string, error) {
	ctx.usedModules[module] = true
	
	if module == "math" {
		switch prop {
		case "PI": return "math.Pi", nil
		case "E": return "math.E", nil
		case "SQRT2": return "math.Sqrt2", nil
		case "LN2": return "math.Ln2", nil
		case "LN10": return "math.Ln10", nil
		case "LOG2E": return "math.Log2E", nil
		case "LOG10E": return "math.Log10E", nil
		}
	}
	
	// Fallback to capitalizing first letter
	return fmt.Sprintf("%s.%s", module, strings.Title(prop)), nil
}
