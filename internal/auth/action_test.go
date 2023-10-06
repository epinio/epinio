package auth

import (
	"fmt"
	"testing"
)

func TestNewFromYAML(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions, err := LoadActions()
			fmt.Println(actions, err)
		})
	}
}
