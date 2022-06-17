package usercmd

import (
	"fmt"
)

func (c *EpinioClient) EventStream() error {
	c.ui.Note().Msg("Streaming events")

	err := c.API.EventStream()
	if err != nil {
		c.ui.Problem().Msg(fmt.Sprintf("failed to stream events: %s", err.Error()))
		return err
	}

	return nil
}
