// Package errors is a collection of various custom errors returned by Epinio methods.
// They are split in a separate package to avoid import loops.
package errors

import "fmt"

type NamespaceMissingError struct {
	Namespace string
}

func (n NamespaceMissingError) Error() string {
	return fmt.Sprintf("namespace %s does not exist", n.Namespace)
}
