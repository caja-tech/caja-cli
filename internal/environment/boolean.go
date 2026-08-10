package environment

var (
	envTrue  = &Boolean{Value: true}
	envFalse = &Boolean{Value: false}
)

// True returns the singleton environment.Boolean object representing the boolean value true.
func True() *Boolean {
	return envTrue
}

// False returns the singleton environment.Boolean object representing the boolean value false.
func False() *Boolean {
	return envFalse
}

// NativeBoolToBooleanObject converts a native Go boolean into its corresponding
// environment.Boolean object wrapper, returning either the TRUE or FALSE singleton.
func NativeBoolToBooleanObject(input bool) *Boolean {
	if input {
		return envTrue
	}
	return envFalse
}
