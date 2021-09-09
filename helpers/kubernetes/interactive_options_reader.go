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

// Read asks the user what value should the given InstallationOption have and
// returns that value validated and converted to the appropriate type as defined
// by the Type field of the InstallationOption.
func (reader InteractiveOptionsReader) Read(option *InstallationOption) error {

	// Internal validation of Type field early. Prevent bogus prompting.
	switch option.Type {
	case BooleanType:
	case StringType:
	case IntType:
	default:
		return errors.New("Internal error: option Type not supported")
	}

	// Ignore anything which is already set by the user (cli option or similar).
	if option.UserSpecified {
		return nil
	}

	var deployment string
	if option.DeploymentID == "" {
		deployment = "Shared"
	} else {
		deployment = string(option.DeploymentID)
	}

	possibleOptions := ""
	if option.Type == BooleanType {
		possibleOptions = " (y/n)"
	}

	// Prefill with a default the user may accept (**)
	err := option.SetDefault()
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf("[%s] %s %s%s [%v]: ", deployment, option.Name, option.Description, possibleOptions, option.Value)

	if _, err := reader.out.Write([]byte(prompt)); err != nil {
		return err
	}

	bufReader := bufio.NewReader(reader.in)
	userValue, err := bufReader.ReadString('\n')
	if err != nil {
		return err
	}
	userValue = strings.TrimSpace(userValue)

	if userValue == "" {
		// Keep the default set by (**). And claim it as
		// user-specified (actually more `affirmed`).
		option.UserSpecified = true
		return nil
	}

	// TODO (post MVP): String and integer types should be
	// extended to call an option-specific validation function, if
	// present, which would perform additional checks on the
	// user's value. For example range limits, proper syntax of
	// the string, etc. They would then loop similar to the entry
	// for booleans. The loop could then actually move outside of
	// the switch, and boolean validation would use a standard
	// validation function.

	switch option.Type {
	case BooleanType:
		for {
			if userValue == "y" {
				option.Value = true
				option.UserSpecified = true
				return nil
			} else if userValue == "n" {
				option.Value = false
				option.UserSpecified = true
				return nil
			}
			if _, err := reader.out.Write([]byte("It's either 'y' or 'n', please try again")); err != nil {
				return err
			}
			userValue, err = bufReader.ReadString('\n')
			if err != nil {
				return err
			}
			userValue = strings.TrimSpace(userValue)
		}
	case StringType:
		option.Value = userValue
		option.UserSpecified = true
		return nil
	case IntType:
		for {
			userInt, err := strconv.Atoi(userValue)
			if err == nil {
				option.Value = userInt
				option.UserSpecified = true
				return nil
			}
			if _, err := reader.out.Write([]byte("Please provide an integer value")); err != nil {
				return err
			}
			userValue, err = bufReader.ReadString('\n')
			if err != nil {
				return err
			}
			userValue = strings.TrimSpace(userValue)
		}
	default:
		return errors.New("Internal error: option Type not supported")
	}
}
