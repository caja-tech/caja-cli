package environment

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

// newStringModule initializes and returns a builtin "string" module,
// populating it with standard string manipulation functions.
func (e *Environment) newStringModule() *Module {
	moduleName := "string"
	stringEnv := NewEnvironment(e.BaseDir, moduleName, true)

	stringEnv.Set("charAt", &Builtin{
		Name: "string.charAt",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'charAt'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'charAt' must be STRING, got %s", args[0].Type())
			}
			idxObj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'charAt' must be NUMBER, got %s", args[1].Type())
			}

			str := strObj.Value
			idx := int(idxObj.Value)

			if idx < 0 || idx >= len(str) {
				return nil, fmt.Errorf("runtime error: charAt index %d out of bounds for string of length %d", idx, len(str))
			}

			return &String{Value: string(str[idx])}, nil
		}})

	stringEnv.Set("substring", &Builtin{
		Name: "string.substring",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'substring'. got=%d, want=3", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'substring' must be STRING, got %s", args[0].Type())
			}
			startObj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'substring' must be NUMBER, got %s", args[1].Type())
			}
			endObj, ok := args[2].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: third argument to 'substring' must be NUMBER, got %s", args[2].Type())
			}

			str := strObj.Value
			start := int(startObj.Value)
			end := int(endObj.Value)

			if start < 0 || start > len(str) {
				return nil, fmt.Errorf("runtime error: substring start index %d out of bounds for string of length %d", start, len(str))
			}
			if end < 0 || end > len(str) {
				return nil, fmt.Errorf("runtime error: substring end index %d out of bounds for string of length %d", end, len(str))
			}
			if start > end {
				return nil, fmt.Errorf("runtime error: substring start index %d is greater than end index %d", start, end)
			}

			return &String{Value: str[start:end]}, nil
		}})

	stringEnv.Set("concat", &Builtin{
		Name: "string.concat",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'concat'. got=%d, want=2", len(args))
			}
			str1Obj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'concat' must be STRING, got %s", args[0].Type())
			}
			str2Obj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'concat' must be STRING, got %s", args[1].Type())
			}

			return &String{Value: str1Obj.Value + str2Obj.Value}, nil
		}})

	stringEnv.Set("split", &Builtin{
		Name: "string.split",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'split'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'split' must be STRING, got %s", args[0].Type())
			}
			delimObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'split' must be STRING, got %s", args[1].Type())
			}

			if len(delimObj.Value) != 1 {
				return nil, fmt.Errorf("runtime error: split delimiter must be a single character string")
			}

			parts := strings.Split(strObj.Value, delimObj.Value)
			elements := make([]Object, len(parts))
			for i, p := range parts {
				elements[i] = &String{Value: p}
			}
			return &Array{Elements: elements}, nil
		}})

	stringEnv.Set("contains", &Builtin{
		Name: "string.contains",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'contains'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'contains' must be STRING, got %s", args[0].Type())
			}
			subObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'contains' must be STRING, got %s", args[1].Type())
			}

			return &Boolean{Value: strings.Contains(strObj.Value, subObj.Value)}, nil
		}})

	stringEnv.Set("startsWith", &Builtin{
		Name: "string.startsWith",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'startsWith'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'startsWith' must be STRING, got %s", args[0].Type())
			}
			prefixObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'startsWith' must be STRING, got %s", args[1].Type())
			}

			return &Boolean{Value: strings.HasPrefix(strObj.Value, prefixObj.Value)}, nil
		}})

	stringEnv.Set("endsWith", &Builtin{
		Name: "string.endsWith",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'endsWith'. got=%d, want=2", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'endsWith' must be STRING, got %s", args[0].Type())
			}
			suffixObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'endsWith' must be STRING, got %s", args[1].Type())
			}

			return &Boolean{Value: strings.HasSuffix(strObj.Value, suffixObj.Value)}, nil
		}})

	stringEnv.Set("replace", &Builtin{
		Name: "string.replace",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'replace'. got=%d, want=3", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'replace' must be STRING, got %s", args[0].Type())
			}
			oldObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'replace' must be STRING, got %s", args[1].Type())
			}
			newObj, ok := args[2].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: third argument to 'replace' must be STRING, got %s", args[2].Type())
			}

			return &String{Value: strings.ReplaceAll(strObj.Value, oldObj.Value, newObj.Value)}, nil
		}})

	stringEnv.Set("toUpper", &Builtin{
		Name: "string.toUpper",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'toUpper'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'toUpper' must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.ToUpper(strObj.Value)}, nil
		}})

	stringEnv.Set("toLower", &Builtin{
		Name: "string.toLower",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'toLower'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'toLower' must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.ToLower(strObj.Value)}, nil
		}})

	stringEnv.Set("trim", &Builtin{
		Name: "string.trim",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'trim'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'trim' must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.TrimSpace(strObj.Value)}, nil
		}})

	stringEnv.Set("len", &Builtin{
		Name: "string.len",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'strlen'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'strlen' must be STRING, got %s", args[0].Type())
			}

			return &Number{Value: float64(len(strObj.Value))}, nil
		}})

	stringEnv.Set("join", &Builtin{
		Name: "string.join",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'join'. got=%d, want=2", len(args))
			}
			arrObj, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'join' must be ARRAY, got %s", args[0].Type())
			}
			delimObj, ok := args[1].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'join' must be STRING, got %s", args[1].Type())
			}

			var strElements []string
			for _, el := range arrObj.Elements {
				strEl, ok := el.(*String)
				if !ok {
					return nil, fmt.Errorf("semantic error: array elements must be STRING, got %s", el.Type())
				}
				strElements = append(strElements, strEl.Value)
			}

			return &String{Value: strings.Join(strElements, delimObj.Value)}, nil
		}})

	return &Module{
		Name: moduleName,
		Env:  stringEnv,
	}
}

// newDateModule initializes and returns a builtin "date" module,
// populating it with standard date manipulation functions.
func (e *Environment) newDateModule() *Module {
	moduleName := "date"
	dateEnv := NewEnvironment(e.BaseDir, moduleName, true)

	dateEnv.Set("year", &Builtin{
		Name: "date.year",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'year'. got=%d, want=1", len(args))
			}
			dateObj, ok := args[0].(*Date)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'year' must be DATE, got %s", args[0].Type())
			}
			return &Number{Value: float64(dateObj.Value.Year())}, nil
		}})

	dateEnv.Set("month", &Builtin{
		Name: "date.month",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'month'. got=%d, want=1", len(args))
			}
			dateObj, ok := args[0].(*Date)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'month' must be DATE, got %s", args[0].Type())
			}
			return &Number{Value: float64(dateObj.Value.Month())}, nil
		}})

	dateEnv.Set("day", &Builtin{
		Name: "date.day",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'day'. got=%d, want=1", len(args))
			}
			dateObj, ok := args[0].(*Date)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'day' must be DATE, got %s", args[0].Type())
			}
			return &Number{Value: float64(dateObj.Value.Day())}, nil
		}})

	dateEnv.Set("weekday", &Builtin{
		Name: "date.weekday",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'weekday'. got=%d, want=1", len(args))
			}
			dateObj, ok := args[0].(*Date)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'weekday' must be DATE, got %s", args[0].Type())
			}
			return &Number{Value: float64(dateObj.Value.Weekday())}, nil
		}})

	dateEnv.Set("today", &Builtin{
		Name: "date.today",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'today'. got=%d, want=0", len(args))
			}
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			return &Date{Value: today}, nil
		}})

	dateEnv.Set("parse", &Builtin{
		Name: "date.parse",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'parse'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'parse' must be STRING, got %s", args[0].Type())
			}
			parsed, err := time.Parse("2006-01-02", strObj.Value)
			if err != nil {
				return nil, fmt.Errorf("runtime error: invalid date format for 'parse', expected 'YYYY-MM-DD'")
			}
			return &Date{Value: parsed}, nil
		}})

	dateEnv.Set("addDays", &Builtin{
		Name: "date.addDays",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'addDays'. got=%d, want=2", len(args))
			}
			dateObj, ok := args[0].(*Date)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'addDays' must be DATE, got %s", args[0].Type())
			}
			numObj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'addDays' must be NUMBER, got %s", args[1].Type())
			}
			days := math.Floor(numObj.Value)
			newDate := dateObj.Value.AddDate(0, 0, int(days))
			return &Date{Value: newDate}, nil
		}})

	dateEnv.Set("diffDays", &Builtin{
		Name: "date.diffDays",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'diffDays'. got=%d, want=2", len(args))
			}
			date1, ok1 := args[0].(*Date)
			date2, ok2 := args[1].(*Date)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("semantic error: arguments to 'diffDays' must be DATE")
			}
			diff := date1.Value.Sub(date2.Value).Hours() / 24
			return &Number{Value: math.Abs(math.Round(diff))}, nil
		}})

	dateEnv.Set("new", &Builtin{
		Name: "date.new",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'new'. got=%d, want=3", len(args))
			}
			yearObj, ok1 := args[0].(*Number)
			monthObj, ok2 := args[1].(*Number)
			dayObj, ok3 := args[2].(*Number)
			if !ok1 || !ok2 || !ok3 {
				return nil, fmt.Errorf("semantic error: arguments to 'new' must be NUMBER")
			}

			year := int(math.Floor(yearObj.Value))
			month := int(math.Floor(monthObj.Value))
			day := int(math.Floor(dayObj.Value))

			if year < 1 || month < 1 || month > 12 || day < 1 || day > 31 {
				return nil, fmt.Errorf("runtime error: invalid date boundaries for 'new'")
			}

			d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

			// Check if the generated date matches the requested date to prevent rolling over (e.g., Feb 30 -> Mar 2)
			if d.Year() != year || int(d.Month()) != month || d.Day() != day {
				return nil, fmt.Errorf("runtime error: invalid date boundaries for 'new'")
			}

			return &Date{Value: d}, nil
		}})

	return &Module{
		Name: moduleName,
		Env:  dateEnv,
	}
}

// newMathModule initializes and returns a builtin "math" module,
// populating it with standard mathematical functions.
func (e *Environment) newMathModule() *Module {
	moduleName := "math"
	mathEnv := NewEnvironment(e.BaseDir, moduleName, true)

	mathEnv.Set("abs", &Builtin{
		Name: "math.abs",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'abs'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'abs' must be NUMBER, got %s", args[0].Type())
			}
			return &Number{Value: math.Abs(numObj.Value)}, nil
		}})

	mathEnv.Set("sqrt", &Builtin{
		Name: "math.sqrt",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'sqrt'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'sqrt' must be NUMBER, got %s", args[0].Type())
			}
			if numObj.Value < 0 {
				return nil, fmt.Errorf("runtime error: sqrt of negative number is undefined")
			}
			return &Number{Value: math.Sqrt(numObj.Value)}, nil
		}})

	mathEnv.Set("pow", &Builtin{
		Name: "math.pow",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'pow'. got=%d, want=2", len(args))
			}
			baseObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'pow' must be NUMBER, got %s", args[0].Type())
			}
			expObj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'pow' must be NUMBER, got %s", args[1].Type())
			}
			return &Number{Value: math.Pow(baseObj.Value, expObj.Value)}, nil
		}})

	mathEnv.Set("floor", &Builtin{
		Name: "math.floor",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'floor'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'floor' must be NUMBER, got %s", args[0].Type())
			}
			return &Number{Value: math.Floor(numObj.Value)}, nil
		}})

	mathEnv.Set("ceil", &Builtin{
		Name: "math.ceil",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'ceil'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'ceil' must be NUMBER, got %s", args[0].Type())
			}
			return &Number{Value: math.Ceil(numObj.Value)}, nil
		}})

	mathEnv.Set("round", &Builtin{
		Name: "math.round",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'round'. got=%d, want=1", len(args))
			}
			numObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'round' must be NUMBER, got %s", args[0].Type())
			}
			return &Number{Value: math.Round(numObj.Value)}, nil
		}})

	mathEnv.Set("min", &Builtin{
		Name: "math.min",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'min'. got=%d, want=2", len(args))
			}
			num1Obj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'min' must be NUMBER, got %s", args[0].Type())
			}
			num2Obj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'min' must be NUMBER, got %s", args[1].Type())
			}
			return &Number{Value: math.Min(num1Obj.Value, num2Obj.Value)}, nil
		}})

	mathEnv.Set("max", &Builtin{
		Name: "math.max",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'max'. got=%d, want=2", len(args))
			}
			num1Obj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'max' must be NUMBER, got %s", args[0].Type())
			}
			num2Obj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'max' must be NUMBER, got %s", args[1].Type())
			}
			return &Number{Value: math.Max(num1Obj.Value, num2Obj.Value)}, nil
		}})

	mathEnv.Set("log", &Builtin{
		Name: "math.log",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'log'. got=%d, want=2", len(args))
			}
			nObj, ok := args[0].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'log' must be NUMBER, got %s", args[0].Type())
			}
			bObj, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'log' must be NUMBER, got %s", args[1].Type())
			}

			if nObj.Value <= 0 {
				return nil, fmt.Errorf("runtime error: log of non-positive number is undefined")
			}
			if bObj.Value <= 0 || bObj.Value == 1 {
				return nil, fmt.Errorf("runtime error: log base must be positive and not equal to 1")
			}

			return &Number{Value: math.Log(nObj.Value) / math.Log(bObj.Value)}, nil
		}})

	mathEnv.Set("rand", &Builtin{
		Name: "math.rand",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'rand'. got=%d, want=0", len(args))
			}
			return &Number{Value: rand.Float64()}, nil
		}})

	mathEnv.Set("PI", &Number{Value: math.Pi})
	mathEnv.Set("E", &Number{Value: math.E})
	mathEnv.Set("SQRT2", &Number{Value: math.Sqrt2})
	mathEnv.Set("LN2", &Number{Value: math.Ln2})
	mathEnv.Set("LN10", &Number{Value: math.Ln10})
	mathEnv.Set("LOG2E", &Number{Value: math.Log2E})
	mathEnv.Set("LOG10E", &Number{Value: math.Log10E})

	return &Module{
		Name: moduleName,
		Env:  mathEnv,
	}
}

// newArrayModule initializes and returns a builtin "array" module,
// populating it with standard array manipulation functions.
func (e *Environment) newArrayModule() *Module {
	moduleName := "array"
	arrayEnv := NewEnvironment(e.BaseDir, moduleName, true)

	arrayEnv.Set("len", &Builtin{
		Name: "array.len",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for len. got=%d, want=1", len(args))
			}

			switch arg := args[0].(type) {
			case *Array:
				return &Number{Value: float64(len(arg.Elements))}, nil
			default:
				return nil, fmt.Errorf("semantic error: argument to 'len' not supported, got %s", args[0].Type())
			}
		}})

	arrayEnv.Set("push", &Builtin{
		Name: "array.push",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for push. got=%d, want=2", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'push' must be ARRAY, got %s", args[0].Type())
			}
			newElements := make([]Object, len(arr.Elements), len(arr.Elements)+1)
			copy(newElements, arr.Elements)
			newElements = append(newElements, args[1])
			return &Array{Elements: newElements}, nil
		}})

	arrayEnv.Set("pop", &Builtin{
		Name: "array.pop",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for pop. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'pop' must be ARRAY, got %s", args[0].Type())
			}
			if len(arr.Elements) > 0 {
				newElements := make([]Object, len(arr.Elements)-1)
				copy(newElements, arr.Elements[:len(arr.Elements)-1])
				return &Array{Elements: newElements}, nil
			}
			return &Array{Elements: []Object{}}, nil
		}})

	arrayEnv.Set("head", &Builtin{
		Name: "array.head",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for head. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'head' must be ARRAY, got %s", args[0].Type())
			}
			if len(arr.Elements) > 0 {
				return arr.Elements[0], nil
			}
			return nil, fmt.Errorf("runtime error: head on empty array")
		}})

	arrayEnv.Set("tail", &Builtin{
		Name: "array.tail",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for tail. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'tail' must be ARRAY, got %s", args[0].Type())
			}
			if len(arr.Elements) > 0 {
				newElements := make([]Object, len(arr.Elements)-1)
				copy(newElements, arr.Elements[1:])
				return &Array{Elements: newElements}, nil
			}
			return &Array{Elements: []Object{}}, nil
		}})

	arrayEnv.Set("last", &Builtin{
		Name: "array.last",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for last. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'last' must be ARRAY, got %s", args[0].Type())
			}
			if len(arr.Elements) > 0 {
				return arr.Elements[len(arr.Elements)-1], nil
			}
			return nil, fmt.Errorf("runtime error: last on empty array")
		}})

	arrayEnv.Set("copy", &Builtin{
		Name: "array.copy",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for copy. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'copy' must be ARRAY, got %s", args[0].Type())
			}
			newElements := make([]Object, len(arr.Elements))
			copy(newElements, arr.Elements)
			return &Array{Elements: newElements}, nil
		}})

	arrayEnv.Set("slice", &Builtin{
		Name: "array.slice",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for slice. got=%d, want=3", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'slice' must be ARRAY, got %s", args[0].Type())
			}
			startNum, ok := args[1].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'slice' must be NUMBER, got %s", args[1].Type())
			}
			endNum, ok := args[2].(*Number)
			if !ok {
				return nil, fmt.Errorf("semantic error: third argument to 'slice' must be NUMBER, got %s", args[2].Type())
			}

			start := int(startNum.Value)
			end := int(endNum.Value)

			if start < 0 || start > len(arr.Elements) {
				return nil, fmt.Errorf("runtime error: slice start index %d out of bounds for array of length %d", start, len(arr.Elements))
			}
			if end < 0 || end > len(arr.Elements) {
				return nil, fmt.Errorf("runtime error: slice end index %d out of bounds for array of length %d", end, len(arr.Elements))
			}
			if start > end {
				return nil, fmt.Errorf("runtime error: slice start index %d is greater than end index %d", start, end)
			}

			newElements := make([]Object, end-start)
			copy(newElements, arr.Elements[start:end])
			return &Array{Elements: newElements}, nil
		}})

	arrayEnv.Set("join", &Builtin{
		Name: "array.join",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'join'. got=%d, want=2", len(args))
			}
			arr1, ok := args[0].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: first argument to 'join' must be ARRAY, got %s", args[0].Type())
			}
			arr2, ok := args[1].(*Array)
			if !ok {
				return nil, fmt.Errorf("semantic error: second argument to 'join' must be ARRAY, got %s", args[1].Type())
			}

			newElements := make([]Object, len(arr1.Elements)+len(arr2.Elements))
			copy(newElements, arr1.Elements)
			copy(newElements[len(arr1.Elements):], arr2.Elements)
			return &Array{Elements: newElements}, nil
		}})

	return &Module{
		Name: moduleName,
		Env:  arrayEnv,
	}
}

// newLogModule initializes and returns a builtin "log" module,
// populating it with standard logging functions.
func (e *Environment) newLogModule() *Module {
	moduleName := "log"
	logEnv := NewEnvironment(e.BaseDir, moduleName, true)

	createLogFunc := func(level string) *Builtin {
		return &Builtin{
			Name: "log." + level,
			Fn: func(env *Environment, args ...Object) (Object, error) {
				if len(args) != 2 {
					return nil, fmt.Errorf("semantic error: wrong number of arguments for '%s'. got=%d, want=2", level, len(args))
				}

				msgObj, ok := args[0].(*String)
				if !ok {
					return nil, fmt.Errorf("semantic error: first argument to '%s' must be STRING, got %s", level, args[0].Type())
				}

				timestamp := time.Now().Format("2006-01-02 15:04:05.000")
				valStr := FormatObject(args[1])

				logPrefix := "Info"
				if level == "warn" {
					logPrefix = "Warning"
				} else if level == "error" {
					logPrefix = "Error"
				}

				output := fmt.Sprintf("%s [%s]: %s: %s", timestamp, logPrefix, msgObj.Value, valStr)
				fmt.Println(output)
				return &String{Value: output}, nil
			},
		}
	}

	logEnv.Set("info", createLogFunc("info"))
	logEnv.Set("warn", createLogFunc("warn"))
	logEnv.Set("error", createLogFunc("error"))

	logEnv.Set("export", &Builtin{
		Name: "log.export",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("arity error: expected 1 argument for 'export', got %d", len(args))
			}
			if e.ExportedValues != nil {
				*e.ExportedValues = append(*e.ExportedValues, args[0])
			}
			return nil, nil
		},
	})

	return &Module{
		Name: moduleName,
		Env:  logEnv,
	}
}

// newMapModule initializes and returns a builtin "map" module.
func (e *Environment) newMapModule() *Module {
	moduleName := "map"
	mapEnv := NewEnvironment(e.BaseDir, moduleName, true)

	mapEnv.Set("containsKey", &Builtin{Name: "map.containsKey"})
	mapEnv.Set("delete", &Builtin{Name: "map.delete"})

	return &Module{
		Name: moduleName,
		Env:  mapEnv,
	}
}

// newCastModule initializes and returns a builtin "cast" module.
func (e *Environment) newCastModule() *Module {
	moduleName := "cast"
	castEnv := NewEnvironment(e.BaseDir, moduleName, true)

	castEnv.Set("to", &Builtin{
		Name: "cast.to",
		Fn: func(env *Environment, args ...Object) (Object, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'to'. got=%d, want=2", len(args))
			}
			
			input := args[0]
			fallback := args[1]

			switch fallback.(type) {
			case *Number:
				switch arg := input.(type) {
				case *Number: return arg, nil
				case *String:
					if val, err := strconv.ParseFloat(arg.Value, 64); err == nil {
						return &Number{Value: val}, nil
					}
				}
			case *String:
				switch arg := input.(type) {
				case *String: return arg, nil
				case *Number: return &String{Value: strconv.FormatFloat(arg.Value, 'f', -1, 64)}, nil
				case *Boolean:
					if arg.Value { return &String{Value: "true"}, nil }
					return &String{Value: "false"}, nil
				case *Date:
					return &String{Value: arg.Value.Format("2006-01-02")}, nil
				}
			case *Boolean:
				switch arg := input.(type) {
				case *Boolean: return arg, nil
				case *String:
					if arg.Value == "true" { return &Boolean{Value: true}, nil }
					if arg.Value == "false" { return &Boolean{Value: false}, nil }
				}
			case *Date:
				switch arg := input.(type) {
				case *Date: return arg, nil
				case *String:
					if t, err := time.Parse("2006-01-02", arg.Value); err == nil {
						return &Date{Value: t}, nil
					}
				}
			}

			// If type conversion isn't supported or fails, return the fallback
			return fallback, nil
		}})

	return &Module{
		Name: moduleName,
		Env:  castEnv,
	}
}
