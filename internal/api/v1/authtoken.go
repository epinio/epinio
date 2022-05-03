package v1

import (
	"github.com/epinio/epinio/helpers/authtoken"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// AuthToken handles the API endpoint /auth-token.  It returns a JWT
// token for further logins
func AuthToken(c *gin.Context) APIErrors {
	requestContext := c.Request.Context()
	user := requestctx.User(requestContext).Username

	response.OKReturn(c, models.AuthTokenResponse{
		Token: authtoken.Create(user, authtoken.DefaultExpiry),
	})
	return nil
}
