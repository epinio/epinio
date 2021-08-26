package cli

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ExitIfError is a short form of ExitfIfError, with a standard message
// It is currently not used
func ExitIfError(err error) {
	ExitfIfError(err, "an unexpected error occurred")
}

// ExitfIfError stops the application with an error exit code, after
// printing error and message, if there is an error.
func ExitfIfError(err error, message string) {
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("%s: %w", message, err))
		os.Exit(1)
	}
}

// ExitfWithMessage stops the application with an error exit code,
// after formatting and printing the message.
// It is currently not used
func ExitfWithMessage(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

// CreateKubeClient returns a client for access to kubernetes
// It is currently not used
func CreateKubeClient(configPath string) kubernetes.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	ExitfIfError(err, "an unexpected error occurred")

	clientset, err := kubernetes.NewForConfig(config)
	ExitfIfError(err, "an unexpected error occurred")

	return clientset
}
