// Package server provides the Epinio http server
package server

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/epinio/epinio/helpers/termui"

	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/filesystem"
	"github.com/epinio/epinio/internal/web"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/mattn/go-colorable"
)

// startEpinioServer is a helper which initializes and start the API server
func Start(wg *sync.WaitGroup, port int, _ *termui.UI, logger logr.Logger) (*http.Server, string, error) {
	// Support colors on Windows also
	gin.DefaultWriter = colorable.NewColorableStdout()

	// Static files
	var assetsDir http.FileSystem
	if os.Getenv("LOCAL_FILESYSTEM") == "true" {
		assetsDir = http.Dir(path.Join(".", "assets", "embedded-web-files", "assets"))
	} else {
		assetsDir = filesystem.Assets()
	}

	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return nil, "", err
	}

	// gin.SetMode(gin.ReleaseMode) // TODO 878 // export GIN_MODE=release|debug|test
	// !debug is default!

	// Endpoint structure ...
	// | Path              | Notes      | Logging
	// | ---               | ---        | ----
	// | /api/v1/...       | API        | Via "/api/v1" Group
	// | /ready            | L/R Probes |
	// | /assets           |            | Via "/assets" Group
	// | /                 | Dashboard  | Via individual attachment, web.Lemon()
	// | /info             | ditto      | ditto
	// | /orgs/target/:org | ditto      | ditto

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(gin.Recovery())

	web.Lemon(router)

	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	assets := router.Group("/assets")
	assets.Use(gin.Logger())
	assets.StaticFS("/", assetsDir)

	api := router.Group("/api/v1")
	api.Use(gin.Logger())
	api.Use(func(c *gin.Context) {
		user := c.GetHeader("X-Webauth-User")
		id := fmt.Sprintf("%d", rand.Intn(10000)) // nolint:gosec // Non-crypto use
		ctx := c.Request.Context()
		ctx = requestctx.ContextWithUser(ctx, user)
		ctx = requestctx.ContextWithID(ctx, id)
		c.Request = c.Request.WithContext(ctx)
	})
	apiv1.Lemon(api)

	srv := &http.Server{
		Handler: router,
	}

	go func() {
		defer wg.Done() // let caller know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			log.Fatalf("Epinio server failed to start: %v", err)
		}
	}()

	// report actual port back to user

	elements := strings.Split(listener.Addr().String(), ":")
	listeningPort := elements[len(elements)-1]

	return srv, listeningPort, nil
}
