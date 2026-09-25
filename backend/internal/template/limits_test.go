package template

import (
	"reflect"
	"testing"
)

// Review F29: NewLimits copies every field to its same-named Limits field, and the two inputs
// together name every Limits field, so a dropped or crossed field fails here.
func TestNewLimitsMapsEveryField(t *testing.T) {
	var ceilings Ceilings
	var bounds NumberBounds
	inputs := map[string]int{}
	value := 1
	for _, target := range []reflect.Value{reflect.ValueOf(&ceilings).Elem(), reflect.ValueOf(&bounds).Elem()} {
		for i := range target.NumField() {
			name := target.Type().Field(i).Name
			if _, taken := inputs[name]; taken {
				t.Fatalf("%s is in both Ceilings and NumberBounds", name)
			}
			target.Field(i).SetInt(int64(value))
			inputs[name] = value
			value++
		}
	}

	limits := reflect.ValueOf(NewLimits(ceilings, bounds))
	for i := range limits.NumField() {
		name := limits.Type().Field(i).Name
		want, named := inputs[name]
		if !named {
			t.Errorf("Limits.%s is named by neither Ceilings nor NumberBounds", name)
			continue
		}
		if got := int(limits.Field(i).Int()); got != want {
			t.Errorf("Limits.%s = %d, want %d from its same-named input", name, got, want)
		}
		delete(inputs, name)
	}
	for name := range inputs {
		t.Errorf("%s is an input no Limits field takes", name)
	}
}
