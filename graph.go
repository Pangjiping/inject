package inject

import (
	"fmt"
	"math/rand"
	"reflect"
)

// The Graph struct
type Graph struct {
	Logger      Logger // Optional, will trigger debug logging
	unnamed     []*Object
	unnamedType map[reflect.Type]bool
	named       map[string]*Object
}

// Provide objects to the Graph. The Object documentation describes
// the impact of various fields.
func (g *Graph) Provide(objects ...*Object) error {
	for _, object := range objects {

		// cached object's type and value
		object.reflectType = reflect.TypeOf(object.Value)
		object.reflectValue = reflect.ValueOf(object.Value)

		if object.Fields != nil {
			return fmt.Errorf(
				"fields were specified on object %s when it was provided", object)
		}

		if object.Name == "" {
			if !isStructPtr(object.reflectType) {
				return fmt.Errorf("expected unnamed object value to be a pointer to a struct but got type %s with value %v", object.reflectType, object.Value)
			}

			// use singleton
			if !object.private {
				if g.unnamedType == nil {
					g.unnamedType = make(map[reflect.Type]bool)
				}

				if g.unnamedType[object.reflectType] {
					return fmt.Errorf("provided two unnamed instances of type *%s.%s", object.reflectType.Elem().PkgPath(), object.reflectType.Elem().Name())
				}

				g.unnamed = append(g.unnamed, object)
			}
		} else {
			if g.named == nil {
				g.named = make(map[string]*Object)
			}

			if g.named[object.Name] != nil {
				return fmt.Errorf("provided two instances named %s", object.Name)
			}
			g.named[object.Name] = object
		}

		if g.Logger != nil {
			if object.created {
				g.Logger.Debugf("created %s", object)
			} else if object.embedded {
				g.Logger.Debugf("provided embedded %s", object)
			} else {
				g.Logger.Debugf("provided %s", object)
			}
		}
	}

	return nil
}

// Populate the incomplete Objects.
func (g *Graph) Populate() error {
	for _, object := range g.named {
		if object.Completed {
			continue
		}

		if err := g.populateExplicit(object); err != nil {
			return err
		}
	}

	// We append and modify our slice as we go along, so we don't use a standard
	// range loop, and do a single pass thru each object in our graph.
	i := 0
	for {
		if i == len(g.unnamed) {
			break
		}

		object := g.unnamed[i]
		i++

		if object.Completed {
			continue
		}

		if err := g.populateExplicit(object); err != nil {
			return err
		}
	}

	// A Second pass handles injecting Interface values to ensure we have created
	// all concrete types first.
	for _, object := range g.unnamed {
		if object.Completed {
			continue
		}

		if err := g.populateUnnamedInterface(object); err != nil {
			return err
		}
	}

	for _, object := range g.named {
		if object.Completed {
			continue
		}

		if err := g.populateUnnamedInterface(object); err != nil {
			return err
		}
	}

	return nil

}

func (g *Graph) populateExplicit(object *Object) error {
	// Ignore named value types.
	if object.Name != "" && !isStructPtr(object.reflectType) {
		return nil
	}

StructLoop:
	for i := 0; i < object.reflectValue.Elem().NumField(); i++ {
		field := object.reflectValue.Elem().Field(i)
		fieldType := field.Type()

		fieldTag := object.reflectType.Elem().Field(i).Tag
		fieldName := object.reflectType.Elem().Field(i).Name

		tag, err := parseTag(string(fieldTag))
		if err != nil {
			return fmt.Errorf("unexpected tag format `%s` for field %s in type %s", string(fieldTag), fieldName, object.reflectType)
		}

		// skip fields without tag
		if tag == nil {
			continue
		}

		// TODO: how to set unexported fields? no constructor!
		// cannot be used with unexported fields
		if !field.CanSet() {
			return fmt.Errorf("inject requested on unexported field %s in type %s", fieldName, object.reflectType)
		}

		// Inline tag on anything besides a struct is considered invalid.
		if tag.Inline && fieldType.Kind() != reflect.Struct {
			return fmt.Errorf("inline requested on non inlined field %s in type %s", fieldName, object.reflectType)
		}

		// don't overwrite existing values
		if !isNilOrZero(field, fieldType) {
			continue
		}

		// named injects must have been explicitly provided
		if tag.Name != "" {
			existedObject := g.named[tag.Name]
			if existedObject == nil {
				return fmt.Errorf("did not find object named %s required by field %s in type %s", tag.Name, fieldName, object.reflectType)
			}

			if !existedObject.reflectType.AssignableTo(fieldType) {
				return fmt.Errorf("object named %s of type %s is not assignable to field %s (%s) in type %s", tag.Name, fieldType, fieldName, existedObject.reflectType, object.reflectType)
			}

			field.Set(reflect.ValueOf(existedObject.Value))

			if g.Logger != nil {
				g.Logger.Debugf("assigned %s to field %s in %s", existedObject, fieldName, object)
			}

			object.addDependency(fieldName, existedObject)
			continue StructLoop
		}

		// Inline struct values indicate we want to traverse into it, but not
		// inject itself. We require an explicit "inline" tag for this to work.
		if fieldType.Kind() == reflect.Struct {
			if tag.Private {
				return fmt.Errorf("cannot use private inject on inline struct on field %s in type %s", fieldName, object.reflectType)
			}

			if !tag.Inline {
				return fmt.Errorf("inline struct on field %s in type %s requires an explicit \"inline\" tag", fieldName, object.reflectType)
			}

			if err := g.Provide(&Object{
				Value:    field.Addr().Interface(),
				private:  true,
				embedded: object.reflectType.Elem().Field(i).Anonymous,
			}); err != nil {
				return err
			}
			continue
		}

		// Interface injection is handled in a second pass
		if fieldType.Kind() == reflect.Interface {
			continue
		}

		// Maps are created and required to be private
		if fieldType.Kind() == reflect.Map {
			if !tag.Private {
				return fmt.Errorf("inject on map field %s in type %s must be named or private", fieldName, object.reflectType)
			}

			field.Set(reflect.MakeMap(fieldType))
			if g.Logger != nil {
				g.Logger.Debugf("made map for field %s in %s", fieldName, object)
			}

			continue
		}

		// Can only inject Pointers from here on.
		if !isStructPtr(fieldType) {
			return fmt.Errorf("found inject tag on unsupported field %s in type %s", fieldName, object.reflectType)
		}

		// Unless it's a private inject, we'll look for an existed instance of the same type.
		if !tag.Private {
			for _, existedObject := range g.unnamed {
				if existedObject.private {
					continue
				}
				if existedObject.reflectType.AssignableTo(fieldType) {
					field.Set(reflect.ValueOf(existedObject.Value))
					if g.Logger != nil {
						g.Logger.Debugf("assigned existing %s to field %s in %s", existedObject, fieldName, object)
					}

					object.addDependency(fieldName, existedObject)
					continue StructLoop
				}
			}
		}

		newValue := reflect.New(fieldType.Elem())
		newObject := &Object{
			Value:   newValue.Interface(),
			private: tag.Private,
			created: true,
		}

		// Add the new created object to the known set of objects.
		if err = g.Provide(newObject); err != nil {
			return err
		}

		// Finally assign the new created object to our field.
		field.Set(newValue)
		if g.Logger != nil {
			g.Logger.Debugf("assigned newly created %s to field %s in %s", newObject, fieldName, object)
		}

		object.addDependency(fieldName, newObject)
	}

	return nil
}

func (g *Graph) populateUnnamedInterface(object *Object) error {
	// Ignore named value types.
	if object.Name != "" && !isStructPtr(object.reflectType) {
		return nil
	}

	for i := 0; i < object.reflectValue.Elem().NumField(); i++ {
		field := object.reflectValue.Elem().Field(i)
		fieldType := field.Type()
		fieldTag := object.reflectType.Elem().Field(i).Tag
		fieldName := object.reflectType.Elem().Field(i).Name

		tag, err := parseTag(string(fieldTag))
		if err != nil {
			return fmt.Errorf("unexpected tag format `%s` for field %s in type %s", string(fieldTag), fieldName, object.reflectType)
		}

		// skip fields without a tag.
		if tag == nil {
			continue
		}

		// We only handle interface injection here. Other cases including errors
		// are handled in the first pass when we inject pointers.
		if fieldType.Kind() != reflect.Interface {
			continue
		}

		// Interface injection can't be private because we can't instantiate new
		// instances of an interface.
		if tag.Private {
			return fmt.Errorf("found private inject tag on interface field %s in type %s", fieldName, object.reflectType)
		}

		// Don't overwrite existing values.
		if !isNilOrZero(field, fieldType) {
			continue
		}

		// Named injects must have already been handled in populateExplicit.
		if tag.Name != "" {
			return fmt.Errorf("unhandled named instance with name %s", tag.Name)
		}

		// Find one, and only one assignable value for the field
		var foundObject *Object
		for _, existedObject := range g.unnamed {
			if existedObject.private {
				continue
			}

			if existedObject.reflectType.AssignableTo(fieldType) {
				if foundObject != nil {
					return fmt.Errorf("found two assignable values for field %s in type %s. one type "+
						"%s with value %v and another type %s with value %v",
						fieldName,
						object.reflectType,
						foundObject.reflectType,
						foundObject.Value,
						existedObject.reflectType,
						existedObject.reflectValue,
					)
				}

				foundObject = existedObject
				field.Set(reflect.ValueOf(existedObject.Value))
				if g.Logger != nil {
					g.Logger.Debugf(
						"assigned existing %s to interface field %s in %s",
						existedObject,
						fieldName,
						object,
					)
				}
				object.addDependency(fieldName, existedObject)
			}
		}

		// If we didn't find an assignable value, we're missing something.
		if foundObject == nil {
			return fmt.Errorf(
				"found no assignable value for field %s in type %s",
				fieldName,
				object.reflectType,
			)
		}
	}

	return nil
}

// Objects returns all known objects, named as well as unnamed. The returned
// elements are not in a stable order.
func (g *Graph) Objects() []*Object {
	objects := make([]*Object, 0, len(g.unnamed)+len(g.named))

	for _, object := range g.unnamed {
		if !object.embedded {
			objects = append(objects, object)
		}
	}

	for _, object := range g.named {
		if !object.embedded {
			objects = append(objects, object)
		}
	}

	// randomize to prevent callers from relying on ordering
	for i := 0; i < len(objects); i++ {
		j := rand.Intn(i + 1)
		objects[i], objects[j] = objects[j], objects[i]
	}

	return objects

}

func isStructPtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}

func isNilOrZero(v reflect.Value, t reflect.Type) bool {
	switch v.Kind() {
	default:
		return reflect.DeepEqual(v.Interface(), reflect.Zero(t).Interface())
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
}
