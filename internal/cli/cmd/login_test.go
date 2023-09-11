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

package cmd_test

import (
	"context"

	"github.com/epinio/epinio/internal/cli/cmd"
	"github.com/epinio/epinio/internal/cli/cmd/cmdfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command 'epinio login' and 'epinio logout'", func() {

	var (
		mockLoginService *cmdfakes.FakeLoginService
	)

	BeforeEach(func() {
		mockLoginService = &cmdfakes.FakeLoginService{}
	})

	When("the login is called", func() {

		It("will call the login service with the right address", func() {
			loginCmd := cmd.NewLoginCmd(mockLoginService)

			// https schema
			mockLoginService.LoginStub = func(_ context.Context, _, _, addr string, _ bool) error {
				Expect(addr).To(Equal("https://epinio.io"))
				return nil
			}

			args := []string{"https://epinio.io"}
			_, _, err := executeCmd(loginCmd, args, nil, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(mockLoginService.LoginCallCount()).To(Equal(1))
			Expect(mockLoginService.LoginOIDCCallCount()).To(BeZero())

			// http schema
			mockLoginService.LoginStub = func(_ context.Context, _, _, addr string, _ bool) error {
				Expect(addr).To(Equal("http://epinio.io"))
				return nil
			}

			args = []string{"http://epinio.io"}
			_, _, err = executeCmd(loginCmd, args, nil, nil)
			Expect(err).ToNot(HaveOccurred())

			// no schema
			mockLoginService.LoginStub = func(_ context.Context, _, _, addr string, _ bool) error {
				Expect(addr).To(Equal("https://epinio.io"))
				return nil
			}

			args = []string{"epinio.io"}
			_, _, err = executeCmd(loginCmd, args, nil, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("will call the Login with the specified values", func() {
			loginCmd := cmd.NewLoginCmd(mockLoginService)

			username, password, address := "myuser", "mypassword", "https://epinio.io"

			mockLoginService.LoginStub = func(_ context.Context, user, pass, addr string, trustCA bool) error {
				Expect(user).To(Equal(username))
				Expect(pass).To(Equal(password))
				Expect(addr).To(Equal(address))
				Expect(trustCA).To(BeTrue())

				return nil
			}

			args := []string{
				address,
				"--user", username,
				"--password", password,
				"--trust-ca",
			}

			_, _, err := executeCmd(loginCmd, args, nil, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(mockLoginService.LoginCallCount()).To(Equal(1))
			// OIDC should not have been called
			Expect(mockLoginService.LoginOIDCCallCount()).To(BeZero())
		})

		It("will call the OIDC Login when --oidc flag is specified", func() {
			loginCmd := cmd.NewLoginCmd(mockLoginService)

			address := "https://epinio.io"

			mockLoginService.LoginOIDCStub = func(_ context.Context, addr string, trustCA, prompt bool) error {
				Expect(addr).To(Equal(address))
				Expect(trustCA).To(BeTrue())
				Expect(prompt).To(BeTrue())

				return nil
			}

			args := []string{address, "--oidc", "--trust-ca", "--prompt"}
			_, _, err := executeCmd(loginCmd, args, nil, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(mockLoginService.LoginOIDCCallCount()).To(Equal(1))
			// Login should not have been called
			Expect(mockLoginService.LoginCallCount()).To(BeZero())
		})

	})

	When("the logout is called", func() {
		It("calls the logout service", func() {
			logoutCmd := cmd.NewLogoutCmd(mockLoginService)
			_, _, err := executeCmd(logoutCmd, []string{}, nil, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(mockLoginService.LogoutCallCount()).To(Equal(1))
		})
	})
})
