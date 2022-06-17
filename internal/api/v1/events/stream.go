// Package events contains the API handlers to manage events
package events

import (
	"net/http"
	"strings"

	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/events"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/gorilla/websocket"
)

// Stream handles the API endpoint GET /events/stream
// It establishes a websocket connection over which, events relevant to the
// current user are sent.
func (hc Controller) Stream(c *gin.Context) {
	//	ctx := c.Request.Context()

	var upgrader = newUpgrader()
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	// TODO: Connect to rabbitmq and start receiving (and filtering) events
	// relevant to the current user. We probably want to subscribe to events
	// on multiple channels (how?). A channel may be named after a namespace
	// so we can filter user's namespaces.
	// TODO: What happens if a user stays connected while an operator
	// removes their access to some namespace?
	// TODO: There are a lot of things to take into account, see here:
	// https://github.com/rabbitmq/amqp091-go/blob/main/_examples/simple-consumer/consumer.go
	err = events.Receive(conn, "namespaces")
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}
}

// https://pkg.go.dev/github.com/gorilla/websocket#hdr-Origin_Considerations
// Regarding matching accessControlAllowOrigin and origin header:
// https: //developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Origin
func newUpgrader() websocket.Upgrader {
	allowedOrigins := viper.GetStringSlice("access-control-allow-origin")
	return websocket.Upgrader{
		CheckOrigin: CheckOriginFunc(allowedOrigins),
	}
}

// TODO: De-duplicate
func CheckOriginFunc(allowedOrigins []string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		originHeader := r.Header.Get("Origin")

		if originHeader == "" {
			return true
		}

		if len(allowedOrigins) == 0 {
			return true
		}

		for _, allowedOrigin := range allowedOrigins {
			trimmedOrigin := strings.TrimSuffix(allowedOrigin, "/")
			trimmedHeader := strings.TrimSuffix(originHeader, "/")
			if trimmedOrigin == "*" || trimmedOrigin == trimmedHeader {
				return true
			}
		}

		return false
	}
}
