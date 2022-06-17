package client

import (
	"fmt"
	"net/http"
	"net/url"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

func (c *Client) EventStream() error {
	token, err := c.AuthToken()
	if err != nil {
		return err
	}

	queryParams := url.Values{}
	queryParams.Add("authtoken", token)

	endpoint := api.WsRoutes.Path("EventStream")

	websocketURL := fmt.Sprintf("%s%s/%s?%s", c.WsURL, api.WsRoot, endpoint, queryParams.Encode())
	webSocketConn, resp, err := websocket.DefaultDialer.Dial(websocketURL, http.Header{})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to connect to websockets endpoint. Response was = %+v\nThe error is", resp))
	}

	for {
		msgType, message, err := webSocketConn.ReadMessage()
		if err != nil {
			return errors.Wrap(err, "receiving a message")
		}

		if msgType == websocket.TextMessage {
			fmt.Println(string(message))
		}
	}
}
