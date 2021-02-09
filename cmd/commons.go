package cmd

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func ExitIfError(err error) {
	ExitfIfError(err, "an unexpected error occurred")
}

func ExitfIfError(err error, message string) {
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("%s: %w", message, err))
		os.Exit(1)
	}
}

func ExitfWithMessage(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

func CreateKubeClient(configPath string) kubernetes.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	ExitfIfError(err, "an unexpected error occurred")

	clientset, err := kubernetes.NewForConfig(config)
	ExitfIfError(err, "an unexpected error occurred")

	return clientset
}
