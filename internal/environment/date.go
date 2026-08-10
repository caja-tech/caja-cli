package environment

import (
	"fmt"
	"math"
	"time"
)

// newDateModule initializes and returns a builtin "date" module,
// populating it with standard date manipulation functions.
func (env *Environment) newDateModule() *Module {
	moduleName := "date"
	dateEnv := NewEnvironment(env.BaseDir, moduleName, true)

	dateEnv.Set("year", &Builtin{
		Fn: func(args ...Object) (Object, error) {
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
		Fn: func(args ...Object) (Object, error) {
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
		Fn: func(args ...Object) (Object, error) {
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
		Fn: func(args ...Object) (Object, error) {
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
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'today'. got=%d, want=0", len(args))
			}
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			return &Date{Value: today}, nil
		}})

	dateEnv.Set("parseDate", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'parseDate'. got=%d, want=1", len(args))
			}
			strObj, ok := args[0].(*String)
			if !ok {
				return nil, fmt.Errorf("semantic error: argument to 'parseDate' must be STRING, got %s", args[0].Type())
			}
			parsed, err := time.Parse("2006-01-02", strObj.Value)
			if err != nil {
				return nil, fmt.Errorf("runtime error: invalid date format for 'parseDate', expected 'YYYY-MM-DD'")
			}
			return &Date{Value: parsed}, nil
		}})

	dateEnv.Set("addDays", &Builtin{
		Fn: func(args ...Object) (Object, error) {
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
		Fn: func(args ...Object) (Object, error) {
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

	dateEnv.Set("newDate", &Builtin{
		Fn: func(args ...Object) (Object, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("semantic error: wrong number of arguments for 'newDate'. got=%d, want=3", len(args))
			}
			yearObj, ok1 := args[0].(*Number)
			monthObj, ok2 := args[1].(*Number)
			dayObj, ok3 := args[2].(*Number)
			if !ok1 || !ok2 || !ok3 {
				return nil, fmt.Errorf("semantic error: arguments to 'newDate' must be NUMBER")
			}

			year := int(math.Floor(yearObj.Value))
			month := int(math.Floor(monthObj.Value))
			day := int(math.Floor(dayObj.Value))

			if year < 1 || month < 1 || month > 12 || day < 1 || day > 31 {
				return nil, fmt.Errorf("runtime error: invalid date boundaries for 'newDate'")
			}

			d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

			// Check if the generated date matches the requested date to prevent rolling over (e.g., Feb 30 -> Mar 2)
			if d.Year() != year || int(d.Month()) != month || d.Day() != day {
				return nil, fmt.Errorf("runtime error: invalid date boundaries for 'newDate'")
			}

			return &Date{Value: d}, nil
		}})

	return &Module{
		Name: moduleName,
		Env:  dateEnv,
	}
}
