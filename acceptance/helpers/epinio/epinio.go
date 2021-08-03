package epinio

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/proc"
)

type Epinio struct {
	Flags            string
	EpinioBinaryPath string
}

func NewEpinioHelper(epinioBinaryPath string) Epinio {
	return Epinio{
		EpinioBinaryPath: epinioBinaryPath,
	}
}

func (e *Epinio) Install() (string, error) {
	out, err := proc.Run(fmt.Sprintf("%s install %s", e.EpinioBinaryPath, e.Flags),
		"", false)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Epinio) Uninstall() (string, error) {
	out, err := proc.Run(fmt.Sprintf("%s uninstall", e.EpinioBinaryPath),
		"", false)
	if err != nil {
		return out, err
	}
	return out, nil
}
