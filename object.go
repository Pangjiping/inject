package inject

import (
	"bytes"
	"fmt"
	"reflect"
)

// An Object in the Graph
type Object struct {
	Value        interface{}
	Name         string             // Optional
	Completed    bool               // if true, the Value will be considered completed
	Fields       map[string]*Object // Populated with the field names that were injected and their corresponding *Object
	reflectType  reflect.Type
	reflectValue reflect.Value
	private      bool // If true, the Value will not be used and will only be populated
	created      bool // If true, the Object was created by us
	embedded     bool // If true, the Object is an embedded struct provided internally
}

// String representation suitable for human consumption.
func (o *Object) String() string {
	var buf bytes.Buffer
	fmt.Fprint(&buf, o.reflectType)

	if o.Name != "" {
		fmt.Fprintf(&buf, " named %s", o.Name)
	}

	return buf.String()
}

func (o *Object) addDependency(field string, dependency *Object) {
	if o.Fields == nil {
		o.Fields = make(map[string]*Object)
	}

	o.Fields[field] = dependency
}
