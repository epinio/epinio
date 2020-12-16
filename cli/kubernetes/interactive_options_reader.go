package kubernetes

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type InteractiveOptionsReader struct {
	in  io.Reader
	out io.Writer
}

// NewInteractiveOptionsReader is the default reader used by the Installer
// when one is not defined. It asks the user questions on stdout and gets
// answers on stdin.
func NewInteractiveOptionsReader(stdout io.Writer, stdin io.Reader) InteractiveOptionsReader {
	return InteractiveOptionsReader{in: stdin, out: stdout}
}

// Read aks the user what value should the given InstallationOption have and
// returns that value validated and converted to the appropriate type as defined
// by the Type field of the InstallationOption.
func (reader InteractiveOptionsReader) Read(option InstallationOption) (interface{}, error) {
	var deployment string
	if option.DeploymentID == "" {
		deployment = "Shared"
	} else {
		deployment = string(option.DeploymentID)
	}

	possibleOptions := ""
	if option.Type == BooleanType {
		possibleOptions = "(y/n)"
	}

	prompt := fmt.Sprintf("[%s] %s %s (%s) : ", deployment, option.Name, option.Description, possibleOptions)
	reader.out.Write([]byte(prompt))
	bufReader := bufio.NewReader(reader.in)
	userValue, err := bufReader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	userValue = strings.TrimSpace(userValue)

	switch option.Type {
	case BooleanType:
		for {
			if userValue == "y" {
				return true, nil
			} else if userValue == "n" {
				return false, nil
			}

			reader.out.Write([]byte("It's either 'y' or 'n', please try again"))
			userValue, err = bufReader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			userValue = strings.TrimSpace(userValue)
		}
	case StringType:
		return userValue, nil
	case IntType:
		for {
			userInt, err := strconv.Atoi(userValue)
			if err == nil {
				return userInt, nil
			}

			reader.out.Write([]byte("Please provide an integrer value"))
			userValue, err = bufReader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			userValue = strings.TrimSpace(userValue)
		}
	default:
		return nil, errors.New("option Type not supported")
	}

	return nil, nil
}
