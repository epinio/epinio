package usercmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (c *EpinioClient) Login(ctx context.Context, cmd *cobra.Command) error {
	log := c.Log.WithName("EnvList")
	log.Info("start")
	defer log.Info("return")

	cmd.Printf("Username: ")
	username, err := readUserInput()
	if err != nil {
		return err
	}

	cmd.Printf("Password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}

	fmt.Println(username, password)

	return nil
}

func readUserInput() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(s, "\n"), nil
}
