package epinio

import (
	"github.com/epinio/epinio/acceptance/helpers/proc"
)

type Epinio struct {
	Flags            []string
	EpinioBinaryPath string
}

func NewEpinioHelper(epinioBinaryPath string) Epinio {
	return Epinio{
		EpinioBinaryPath: epinioBinaryPath,
	}
}

func (e *Epinio) Install() (string, error) {
	out, err := proc.Run("", false, e.EpinioBinaryPath, append([]string{"install"}, e.Flags...)...)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (e *Epinio) Uninstall() (string, error) {
	out, err := proc.Run("", false, e.EpinioBinaryPath, "uninstall")
	if err != nil {
		return out, err
	}
	return out, nil
}
