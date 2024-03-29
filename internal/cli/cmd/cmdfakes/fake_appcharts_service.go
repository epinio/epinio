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
	"github.com/epinio/epinio/internal/cli/usercmd"
)

type FakeAppchartsService struct {
	ChartDefaultSetStub        func(context.Context, string) error
	chartDefaultSetMutex       sync.RWMutex
	chartDefaultSetArgsForCall []struct {
		arg1 context.Context
		arg2 string
	}
	chartDefaultSetReturns struct {
		result1 error
	}
	chartDefaultSetReturnsOnCall map[int]struct {
		result1 error
	}
	ChartDefaultShowStub        func(context.Context) error
	chartDefaultShowMutex       sync.RWMutex
	chartDefaultShowArgsForCall []struct {
		arg1 context.Context
	}
	chartDefaultShowReturns struct {
		result1 error
	}
	chartDefaultShowReturnsOnCall map[int]struct {
		result1 error
	}
	ChartListStub        func(context.Context) error
	chartListMutex       sync.RWMutex
	chartListArgsForCall []struct {
		arg1 context.Context
	}
	chartListReturns struct {
		result1 error
	}
	chartListReturnsOnCall map[int]struct {
		result1 error
	}
	ChartMatchingStub        func(string) []string
	chartMatchingMutex       sync.RWMutex
	chartMatchingArgsForCall []struct {
		arg1 string
	}
	chartMatchingReturns struct {
		result1 []string
	}
	chartMatchingReturnsOnCall map[int]struct {
		result1 []string
	}
	ChartShowStub        func(context.Context, string) error
	chartShowMutex       sync.RWMutex
	chartShowArgsForCall []struct {
		arg1 context.Context
		arg2 string
	}
	chartShowReturns struct {
		result1 error
	}
	chartShowReturnsOnCall map[int]struct {
		result1 error
	}
	GetAPIStub        func() usercmd.APIClient
	getAPIMutex       sync.RWMutex
	getAPIArgsForCall []struct {
	}
	getAPIReturns struct {
		result1 usercmd.APIClient
	}
	getAPIReturnsOnCall map[int]struct {
		result1 usercmd.APIClient
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeAppchartsService) ChartDefaultSet(arg1 context.Context, arg2 string) error {
	fake.chartDefaultSetMutex.Lock()
	ret, specificReturn := fake.chartDefaultSetReturnsOnCall[len(fake.chartDefaultSetArgsForCall)]
	fake.chartDefaultSetArgsForCall = append(fake.chartDefaultSetArgsForCall, struct {
		arg1 context.Context
		arg2 string
	}{arg1, arg2})
	stub := fake.ChartDefaultSetStub
	fakeReturns := fake.chartDefaultSetReturns
	fake.recordInvocation("ChartDefaultSet", []interface{}{arg1, arg2})
	fake.chartDefaultSetMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeAppchartsService) ChartDefaultSetCallCount() int {
	fake.chartDefaultSetMutex.RLock()
	defer fake.chartDefaultSetMutex.RUnlock()
	return len(fake.chartDefaultSetArgsForCall)
}

func (fake *FakeAppchartsService) ChartDefaultSetCalls(stub func(context.Context, string) error) {
	fake.chartDefaultSetMutex.Lock()
	defer fake.chartDefaultSetMutex.Unlock()
	fake.ChartDefaultSetStub = stub
}

func (fake *FakeAppchartsService) ChartDefaultSetArgsForCall(i int) (context.Context, string) {
	fake.chartDefaultSetMutex.RLock()
	defer fake.chartDefaultSetMutex.RUnlock()
	argsForCall := fake.chartDefaultSetArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeAppchartsService) ChartDefaultSetReturns(result1 error) {
	fake.chartDefaultSetMutex.Lock()
	defer fake.chartDefaultSetMutex.Unlock()
	fake.ChartDefaultSetStub = nil
	fake.chartDefaultSetReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeAppchartsService) ChartDefaultSetReturnsOnCall(i int, result1 error) {
	fake.chartDefaultSetMutex.Lock()
	defer fake.chartDefaultSetMutex.Unlock()
	fake.ChartDefaultSetStub = nil
	if fake.chartDefaultSetReturnsOnCall == nil {
		fake.chartDefaultSetReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.chartDefaultSetReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeAppchartsService) ChartDefaultShow(arg1 context.Context) error {
	fake.chartDefaultShowMutex.Lock()
	ret, specificReturn := fake.chartDefaultShowReturnsOnCall[len(fake.chartDefaultShowArgsForCall)]
	fake.chartDefaultShowArgsForCall = append(fake.chartDefaultShowArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.ChartDefaultShowStub
	fakeReturns := fake.chartDefaultShowReturns
	fake.recordInvocation("ChartDefaultShow", []interface{}{arg1})
	fake.chartDefaultShowMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeAppchartsService) ChartDefaultShowCallCount() int {
	fake.chartDefaultShowMutex.RLock()
	defer fake.chartDefaultShowMutex.RUnlock()
	return len(fake.chartDefaultShowArgsForCall)
}

func (fake *FakeAppchartsService) ChartDefaultShowCalls(stub func(context.Context) error) {
	fake.chartDefaultShowMutex.Lock()
	defer fake.chartDefaultShowMutex.Unlock()
	fake.ChartDefaultShowStub = stub
}

func (fake *FakeAppchartsService) ChartDefaultShowArgsForCall(i int) context.Context {
	fake.chartDefaultShowMutex.RLock()
	defer fake.chartDefaultShowMutex.RUnlock()
	argsForCall := fake.chartDefaultShowArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeAppchartsService) ChartDefaultShowReturns(result1 error) {
	fake.chartDefaultShowMutex.Lock()
	defer fake.chartDefaultShowMutex.Unlock()
	fake.ChartDefaultShowStub = nil
	fake.chartDefaultShowReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeAppchartsService) ChartDefaultShowReturnsOnCall(i int, result1 error) {
	fake.chartDefaultShowMutex.Lock()
	defer fake.chartDefaultShowMutex.Unlock()
	fake.ChartDefaultShowStub = nil
	if fake.chartDefaultShowReturnsOnCall == nil {
		fake.chartDefaultShowReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.chartDefaultShowReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeAppchartsService) ChartList(arg1 context.Context) error {
	fake.chartListMutex.Lock()
	ret, specificReturn := fake.chartListReturnsOnCall[len(fake.chartListArgsForCall)]
	fake.chartListArgsForCall = append(fake.chartListArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.ChartListStub
	fakeReturns := fake.chartListReturns
	fake.recordInvocation("ChartList", []interface{}{arg1})
	fake.chartListMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeAppchartsService) ChartListCallCount() int {
	fake.chartListMutex.RLock()
	defer fake.chartListMutex.RUnlock()
	return len(fake.chartListArgsForCall)
}

func (fake *FakeAppchartsService) ChartListCalls(stub func(context.Context) error) {
	fake.chartListMutex.Lock()
	defer fake.chartListMutex.Unlock()
	fake.ChartListStub = stub
}

func (fake *FakeAppchartsService) ChartListArgsForCall(i int) context.Context {
	fake.chartListMutex.RLock()
	defer fake.chartListMutex.RUnlock()
	argsForCall := fake.chartListArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeAppchartsService) ChartListReturns(result1 error) {
	fake.chartListMutex.Lock()
	defer fake.chartListMutex.Unlock()
	fake.ChartListStub = nil
	fake.chartListReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeAppchartsService) ChartListReturnsOnCall(i int, result1 error) {
	fake.chartListMutex.Lock()
	defer fake.chartListMutex.Unlock()
	fake.ChartListStub = nil
	if fake.chartListReturnsOnCall == nil {
		fake.chartListReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.chartListReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeAppchartsService) ChartMatching(arg1 string) []string {
	fake.chartMatchingMutex.Lock()
	ret, specificReturn := fake.chartMatchingReturnsOnCall[len(fake.chartMatchingArgsForCall)]
	fake.chartMatchingArgsForCall = append(fake.chartMatchingArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.ChartMatchingStub
	fakeReturns := fake.chartMatchingReturns
	fake.recordInvocation("ChartMatching", []interface{}{arg1})
	fake.chartMatchingMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeAppchartsService) ChartMatchingCallCount() int {
	fake.chartMatchingMutex.RLock()
	defer fake.chartMatchingMutex.RUnlock()
	return len(fake.chartMatchingArgsForCall)
}

func (fake *FakeAppchartsService) ChartMatchingCalls(stub func(string) []string) {
	fake.chartMatchingMutex.Lock()
	defer fake.chartMatchingMutex.Unlock()
	fake.ChartMatchingStub = stub
}

func (fake *FakeAppchartsService) ChartMatchingArgsForCall(i int) string {
	fake.chartMatchingMutex.RLock()
	defer fake.chartMatchingMutex.RUnlock()
	argsForCall := fake.chartMatchingArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeAppchartsService) ChartMatchingReturns(result1 []string) {
	fake.chartMatchingMutex.Lock()
	defer fake.chartMatchingMutex.Unlock()
	fake.ChartMatchingStub = nil
	fake.chartMatchingReturns = struct {
		result1 []string
	}{result1}
}

func (fake *FakeAppchartsService) ChartMatchingReturnsOnCall(i int, result1 []string) {
	fake.chartMatchingMutex.Lock()
	defer fake.chartMatchingMutex.Unlock()
	fake.ChartMatchingStub = nil
	if fake.chartMatchingReturnsOnCall == nil {
		fake.chartMatchingReturnsOnCall = make(map[int]struct {
			result1 []string
		})
	}
	fake.chartMatchingReturnsOnCall[i] = struct {
		result1 []string
	}{result1}
}

func (fake *FakeAppchartsService) ChartShow(arg1 context.Context, arg2 string) error {
	fake.chartShowMutex.Lock()
	ret, specificReturn := fake.chartShowReturnsOnCall[len(fake.chartShowArgsForCall)]
	fake.chartShowArgsForCall = append(fake.chartShowArgsForCall, struct {
		arg1 context.Context
		arg2 string
	}{arg1, arg2})
	stub := fake.ChartShowStub
	fakeReturns := fake.chartShowReturns
	fake.recordInvocation("ChartShow", []interface{}{arg1, arg2})
	fake.chartShowMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeAppchartsService) ChartShowCallCount() int {
	fake.chartShowMutex.RLock()
	defer fake.chartShowMutex.RUnlock()
	return len(fake.chartShowArgsForCall)
}

func (fake *FakeAppchartsService) ChartShowCalls(stub func(context.Context, string) error) {
	fake.chartShowMutex.Lock()
	defer fake.chartShowMutex.Unlock()
	fake.ChartShowStub = stub
}

func (fake *FakeAppchartsService) ChartShowArgsForCall(i int) (context.Context, string) {
	fake.chartShowMutex.RLock()
	defer fake.chartShowMutex.RUnlock()
	argsForCall := fake.chartShowArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeAppchartsService) ChartShowReturns(result1 error) {
	fake.chartShowMutex.Lock()
	defer fake.chartShowMutex.Unlock()
	fake.ChartShowStub = nil
	fake.chartShowReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeAppchartsService) ChartShowReturnsOnCall(i int, result1 error) {
	fake.chartShowMutex.Lock()
	defer fake.chartShowMutex.Unlock()
	fake.ChartShowStub = nil
	if fake.chartShowReturnsOnCall == nil {
		fake.chartShowReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.chartShowReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeAppchartsService) GetAPI() usercmd.APIClient {
	fake.getAPIMutex.Lock()
	ret, specificReturn := fake.getAPIReturnsOnCall[len(fake.getAPIArgsForCall)]
	fake.getAPIArgsForCall = append(fake.getAPIArgsForCall, struct {
	}{})
	stub := fake.GetAPIStub
	fakeReturns := fake.getAPIReturns
	fake.recordInvocation("GetAPI", []interface{}{})
	fake.getAPIMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeAppchartsService) GetAPICallCount() int {
	fake.getAPIMutex.RLock()
	defer fake.getAPIMutex.RUnlock()
	return len(fake.getAPIArgsForCall)
}

func (fake *FakeAppchartsService) GetAPICalls(stub func() usercmd.APIClient) {
	fake.getAPIMutex.Lock()
	defer fake.getAPIMutex.Unlock()
	fake.GetAPIStub = stub
}

func (fake *FakeAppchartsService) GetAPIReturns(result1 usercmd.APIClient) {
	fake.getAPIMutex.Lock()
	defer fake.getAPIMutex.Unlock()
	fake.GetAPIStub = nil
	fake.getAPIReturns = struct {
		result1 usercmd.APIClient
	}{result1}
}

func (fake *FakeAppchartsService) GetAPIReturnsOnCall(i int, result1 usercmd.APIClient) {
	fake.getAPIMutex.Lock()
	defer fake.getAPIMutex.Unlock()
	fake.GetAPIStub = nil
	if fake.getAPIReturnsOnCall == nil {
		fake.getAPIReturnsOnCall = make(map[int]struct {
			result1 usercmd.APIClient
		})
	}
	fake.getAPIReturnsOnCall[i] = struct {
		result1 usercmd.APIClient
	}{result1}
}

func (fake *FakeAppchartsService) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.chartDefaultSetMutex.RLock()
	defer fake.chartDefaultSetMutex.RUnlock()
	fake.chartDefaultShowMutex.RLock()
	defer fake.chartDefaultShowMutex.RUnlock()
	fake.chartListMutex.RLock()
	defer fake.chartListMutex.RUnlock()
	fake.chartMatchingMutex.RLock()
	defer fake.chartMatchingMutex.RUnlock()
	fake.chartShowMutex.RLock()
	defer fake.chartShowMutex.RUnlock()
	fake.getAPIMutex.RLock()
	defer fake.getAPIMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeAppchartsService) recordInvocation(key string, args []interface{}) {
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

var _ cmd.AppchartsService = new(FakeAppchartsService)
