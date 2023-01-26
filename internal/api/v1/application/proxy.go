// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"k8s.io/client-go/rest"
)

func runProxy(ctx context.Context, rw http.ResponseWriter, req *http.Request, destination *url.URL) apierror.APIErrors {
	clientSetHTTP1, err := kubernetes.GetHTTP1Client(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	httpClient := clientSetHTTP1.CoreV1().RESTClient().(*rest.RESTClient).Client

	p := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = destination
			req.Host = destination.Host
			// let kube authentication work
			delete(req.Header, "Cookie")
			delete(req.Header, "Authorization")
		},
		Transport:     httpClient.Transport,
		FlushInterval: time.Millisecond * 100,
	}

	p.ServeHTTP(rw, req)

	return nil
}
