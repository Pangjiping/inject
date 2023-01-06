package inject

// Populate is a short-hand for populating a graph with the given incomplete
// object values.
func Populate(values ...interface{}) error {
	var g Graph
	for _, v := range values {
		if err := g.Provide(&Object{Value: v}); err != nil {
			return err
		}
	}

	return g.Populate()
}
