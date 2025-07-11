// Code generated by mockery v2.33.2. DO NOT EDIT.

package mocks

import (
	di "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"
	errors "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"

	mock "github.com/stretchr/testify/mock"
)

// HTTPSender is an autogenerated mock type for the HTTPSender type
type HTTPSender struct {
	mock.Mock
}

// HTTPPost provides a mock function with given fields: dic, data
func (_m *HTTPSender) HTTPPost(dic *di.Container, data any) errors.Error {
	ret := _m.Called(dic, data)

	var r0 errors.Error
	if rf, ok := ret.Get(0).(func(*di.Container, any) errors.Error); ok {
		r0 = rf(dic, data)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(errors.Error)
		}
	}

	return r0
}

// SetHmTTPRequestHeaders provides a mock function with given fields: httpRequestHeaders
func (_m *HTTPSender) SetHTTPRequestHeaders(httpRequestHeaders map[string]string) {
	_m.Called(httpRequestHeaders)
}

// SetSecretData provides a mock function with given fields: name, valueKey, headerName, valuePrefix
func (_m *HTTPSender) SetSecretData(name string, valueKey string, headerName string, valuePrefix string) {
	_m.Called(name, valueKey, headerName, valuePrefix)
}

// NewHTTPSender creates a new instance of HTTPSender. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewHTTPSender(t interface {
	mock.TestingT
	Cleanup(func())
}) *HTTPSender {
	mock := &HTTPSender{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
