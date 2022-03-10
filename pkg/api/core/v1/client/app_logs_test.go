package client_test

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func DescribeAppLogs() {

	const logMessages int = 5
	const logMessageDelay time.Duration = 100 * time.Millisecond

	var epinioClient *client.Client
	var statusCode int
	var responseBody string

	JustBeforeEach(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/authtoken", func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"token":"mytoken"}`)
		})

		mux.HandleFunc("/wapi/", func(w http.ResponseWriter, req *http.Request) {
			upgrader := websocket.Upgrader{}
			websocketConn, err := upgrader.Upgrade(w, req, http.Header{})
			Expect(err).ToNot(HaveOccurred())

			// write some random logs
			go func() {
				defer websocketConn.Close()

				followStr := req.URL.Query().Get("follow")
				follow, err := strconv.ParseBool(followStr)
				Expect(err).ToNot(HaveOccurred())

				messagesToWrite := logMessages

				// if following then write a lot of messages
				if follow {
					messagesToWrite = math.MaxInt
				}

				for i := 0; i < messagesToWrite; i++ {
					logMsg := fmt.Sprintf("log message #%d", i)
					websocketConn.WriteMessage(websocket.TextMessage, []byte(logMsg))
					time.Sleep(logMessageDelay)
				}
			}()
		})

		mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			fmt.Printf("##### %+v", req)

			w.WriteHeader(statusCode)
			fmt.Fprint(w, responseBody)
		})

		srv := httptest.NewServer(mux)

		wsURL := strings.Replace(srv.URL, "http://", "ws://", 1)
		epinioClient = client.New(srv.URL, wsURL, "", "")
	})

	When("no following the logs", func() {

		It("will read all the logs if waiting more", func() {
			totalLogTime := time.Duration(logMessages) * logMessageDelay
			waitingTime := totalLogTime + 1*time.Second

			ctx, cancel := context.WithTimeout(context.Background(), waitingTime)
			defer cancel()

			msgChan, err := epinioClient.AppLogs(ctx, "namespace-foo", "appname", "stageID", false)
			Expect(err).ToNot(HaveOccurred())

			messages := []string{}
			for msg := range msgChan {
				messages = append(messages, string(msg))
			}

			Expect(messages).To(HaveLen(logMessages))
		})

		It("will not read all the logs if cancelled before", func() {
			totalLogTime := time.Duration(logMessages) * logMessageDelay
			waitingTime := totalLogTime - 1*time.Second

			ctx, cancel := context.WithTimeout(context.Background(), waitingTime)
			defer cancel()

			msgChan, err := epinioClient.AppLogs(ctx, "namespace-foo", "appname", "stageID", false)
			Expect(err).ToNot(HaveOccurred())

			messages := []string{}
			for msg := range msgChan {
				messages = append(messages, string(msg))
			}

			Expect(len(messages)).To(BeNumerically("<", logMessages))
		})
	})

	When("following the logs", func() {

		It("will read more logs if waiting more time", func() {
			totalLogTime := time.Duration(logMessages) * logMessageDelay
			waitingTime := totalLogTime + 1*time.Second

			ctx, cancel := context.WithTimeout(context.Background(), waitingTime)
			defer cancel()

			msgChan, err := epinioClient.AppLogs(ctx, "namespace-foo", "appname", "stageID", true)
			Expect(err).ToNot(HaveOccurred())

			messages := []string{}
			for msg := range msgChan {
				messages = append(messages, string(msg))
			}

			Expect(len(messages)).To(BeNumerically(">", logMessages))
		})

		It("will read more less logs if cancelled before", func() {
			totalLogTime := time.Duration(logMessages) * logMessageDelay
			waitingTime := totalLogTime - 1*time.Second

			ctx, cancel := context.WithTimeout(context.Background(), waitingTime)
			defer cancel()

			msgChan, err := epinioClient.AppLogs(ctx, "namespace-foo", "appname", "stageID", true)
			Expect(err).ToNot(HaveOccurred())

			messages := []string{}
			for msg := range msgChan {
				messages = append(messages, string(msg))
			}

			Expect(len(messages)).To(BeNumerically("<", logMessages))
		})
	})
}
