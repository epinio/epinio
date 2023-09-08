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

// Code generated by counterfeiter. DO NOT EDIT.
package cmdfakes

import (
	"context"
	"sync"

	"github.com/epinio/epinio/internal/cli/cmd"
)

type FakeLoginService struct {
	LoginStub        func(context.Context, string, string, string, bool) error
	loginMutex       sync.RWMutex
	loginArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 string
		arg5 bool
	}
	loginReturns struct {
		result1 error
	}
	loginReturnsOnCall map[int]struct {
		result1 error
	}
	LoginOIDCStub        func(context.Context, string, bool, bool) error
	loginOIDCMutex       sync.RWMutex
	loginOIDCArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 bool
		arg4 bool
	}
	loginOIDCReturns struct {
		result1 error
	}
	loginOIDCReturnsOnCall map[int]struct {
		result1 error
	}
	LogoutStub        func(context.Context) error
	logoutMutex       sync.RWMutex
	logoutArgsForCall []struct {
		arg1 context.Context
	}
	logoutReturns struct {
		result1 error
	}
	logoutReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeLoginService) Login(arg1 context.Context, arg2 string, arg3 string, arg4 string, arg5 bool) error {
	fake.loginMutex.Lock()
	ret, specificReturn := fake.loginReturnsOnCall[len(fake.loginArgsForCall)]
	fake.loginArgsForCall = append(fake.loginArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
		arg4 string
		arg5 bool
	}{arg1, arg2, arg3, arg4, arg5})
	stub := fake.LoginStub
	fakeReturns := fake.loginReturns
	fake.recordInvocation("Login", []interface{}{arg1, arg2, arg3, arg4, arg5})
	fake.loginMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4, arg5)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeLoginService) LoginCallCount() int {
	fake.loginMutex.RLock()
	defer fake.loginMutex.RUnlock()
	return len(fake.loginArgsForCall)
}

func (fake *FakeLoginService) LoginCalls(stub func(context.Context, string, string, string, bool) error) {
	fake.loginMutex.Lock()
	defer fake.loginMutex.Unlock()
	fake.LoginStub = stub
}

func (fake *FakeLoginService) LoginArgsForCall(i int) (context.Context, string, string, string, bool) {
	fake.loginMutex.RLock()
	defer fake.loginMutex.RUnlock()
	argsForCall := fake.loginArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4, argsForCall.arg5
}

func (fake *FakeLoginService) LoginReturns(result1 error) {
	fake.loginMutex.Lock()
	defer fake.loginMutex.Unlock()
	fake.LoginStub = nil
	fake.loginReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeLoginService) LoginReturnsOnCall(i int, result1 error) {
	fake.loginMutex.Lock()
	defer fake.loginMutex.Unlock()
	fake.LoginStub = nil
	if fake.loginReturnsOnCall == nil {
		fake.loginReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.loginReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeLoginService) LoginOIDC(arg1 context.Context, arg2 string, arg3 bool, arg4 bool) error {
	fake.loginOIDCMutex.Lock()
	ret, specificReturn := fake.loginOIDCReturnsOnCall[len(fake.loginOIDCArgsForCall)]
	fake.loginOIDCArgsForCall = append(fake.loginOIDCArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 bool
		arg4 bool
	}{arg1, arg2, arg3, arg4})
	stub := fake.LoginOIDCStub
	fakeReturns := fake.loginOIDCReturns
	fake.recordInvocation("LoginOIDC", []interface{}{arg1, arg2, arg3, arg4})
	fake.loginOIDCMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeLoginService) LoginOIDCCallCount() int {
	fake.loginOIDCMutex.RLock()
	defer fake.loginOIDCMutex.RUnlock()
	return len(fake.loginOIDCArgsForCall)
}

func (fake *FakeLoginService) LoginOIDCCalls(stub func(context.Context, string, bool, bool) error) {
	fake.loginOIDCMutex.Lock()
	defer fake.loginOIDCMutex.Unlock()
	fake.LoginOIDCStub = stub
}

func (fake *FakeLoginService) LoginOIDCArgsForCall(i int) (context.Context, string, bool, bool) {
	fake.loginOIDCMutex.RLock()
	defer fake.loginOIDCMutex.RUnlock()
	argsForCall := fake.loginOIDCArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeLoginService) LoginOIDCReturns(result1 error) {
	fake.loginOIDCMutex.Lock()
	defer fake.loginOIDCMutex.Unlock()
	fake.LoginOIDCStub = nil
	fake.loginOIDCReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeLoginService) LoginOIDCReturnsOnCall(i int, result1 error) {
	fake.loginOIDCMutex.Lock()
	defer fake.loginOIDCMutex.Unlock()
	fake.LoginOIDCStub = nil
	if fake.loginOIDCReturnsOnCall == nil {
		fake.loginOIDCReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.loginOIDCReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeLoginService) Logout(arg1 context.Context) error {
	fake.logoutMutex.Lock()
	ret, specificReturn := fake.logoutReturnsOnCall[len(fake.logoutArgsForCall)]
	fake.logoutArgsForCall = append(fake.logoutArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.LogoutStub
	fakeReturns := fake.logoutReturns
	fake.recordInvocation("Logout", []interface{}{arg1})
	fake.logoutMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeLoginService) LogoutCallCount() int {
	fake.logoutMutex.RLock()
	defer fake.logoutMutex.RUnlock()
	return len(fake.logoutArgsForCall)
}

func (fake *FakeLoginService) LogoutCalls(stub func(context.Context) error) {
	fake.logoutMutex.Lock()
	defer fake.logoutMutex.Unlock()
	fake.LogoutStub = stub
}

func (fake *FakeLoginService) LogoutArgsForCall(i int) context.Context {
	fake.logoutMutex.RLock()
	defer fake.logoutMutex.RUnlock()
	argsForCall := fake.logoutArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeLoginService) LogoutReturns(result1 error) {
	fake.logoutMutex.Lock()
	defer fake.logoutMutex.Unlock()
	fake.LogoutStub = nil
	fake.logoutReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeLoginService) LogoutReturnsOnCall(i int, result1 error) {
	fake.logoutMutex.Lock()
	defer fake.logoutMutex.Unlock()
	fake.LogoutStub = nil
	if fake.logoutReturnsOnCall == nil {
		fake.logoutReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.logoutReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeLoginService) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.loginMutex.RLock()
	defer fake.loginMutex.RUnlock()
	fake.loginOIDCMutex.RLock()
	defer fake.loginOIDCMutex.RUnlock()
	fake.logoutMutex.RLock()
	defer fake.logoutMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeLoginService) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ cmd.LoginService = new(FakeLoginService)
