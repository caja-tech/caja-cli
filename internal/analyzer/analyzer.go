package analyzer

type Analyzer struct {
	scopes []map[string]bool
	errors []string
}

func New() *Analyzer {
	return &Analyzer{
		scopes: []map[string]bool{make(map[string]bool)},
	}
}

func (a *Analyzer) Errors() []string {
	return a.errors
}

func (a *Analyzer) pushScope() {
	a.scopes = append(a.scopes, make(map[string]bool))
}

func (a *Analyzer) popScope() {
	a.scopes = a.scopes[:len(a.scopes)-1]
}

func (a *Analyzer) declare(name string) {
	last := len(a.scopes) - 1
	a.scopes[last][name] = true
}

func (a *Analyzer) resolve(name string) bool {
	for i := len(a.scopes) - 1; i >= 0; i-- {
		if _, ok := a.scopes[i][name]; ok {
			return true
		}
	}
	return false
}
