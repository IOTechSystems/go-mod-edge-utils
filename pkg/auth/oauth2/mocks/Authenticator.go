// Code generated by mockery v2.42.2. DO NOT EDIT.

package mocks

import (
	http "net/http"

	errors "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"

	jwt "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/auth/jwt"

	mock "github.com/stretchr/testify/mock"

	xoauth2 "golang.org/x/oauth2"
)

// Authenticator is an autogenerated mock type for the Authenticator type
type Authenticator struct {
	mock.Mock
}

// Callback provides a mock function with given fields: _a0
func (_m *Authenticator) Callback(_a0 func(any) (*jwt.TokenDetails, errors.Error)) http.HandlerFunc {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Callback")
	}

	var r0 http.HandlerFunc
	if rf, ok := ret.Get(0).(func(func(any) (*jwt.TokenDetails, errors.Error)) http.HandlerFunc); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(http.HandlerFunc)
		}
	}

	return r0
}

// GetTokenByUserID provides a mock function with given fields: userId
func (_m *Authenticator) GetTokenByUserID(userId string) (*xoauth2.Token, errors.Error) {
	ret := _m.Called(userId)

	if len(ret) == 0 {
		panic("no return value specified for GetTokenByUserID")
	}

	var r0 *xoauth2.Token
	var r1 errors.Error
	if rf, ok := ret.Get(0).(func(string) (*xoauth2.Token, errors.Error)); ok {
		return rf(userId)
	}
	if rf, ok := ret.Get(0).(func(string) *xoauth2.Token); ok {
		r0 = rf(userId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*xoauth2.Token)
		}
	}

	if rf, ok := ret.Get(1).(func(string) errors.Error); ok {
		r1 = rf(userId)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(errors.Error)
		}
	}

	return r0, r1
}

// RequestAuth provides a mock function with given fields:
func (_m *Authenticator) RequestAuth() http.HandlerFunc {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for RequestAuth")
	}

	var r0 http.HandlerFunc
	if rf, ok := ret.Get(0).(func() http.HandlerFunc); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(http.HandlerFunc)
		}
	}

	return r0
}

// NewAuthenticator creates a new instance of Authenticator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAuthenticator(t interface {
	mock.TestingT
	Cleanup(func())
}) *Authenticator {
	mock := &Authenticator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
