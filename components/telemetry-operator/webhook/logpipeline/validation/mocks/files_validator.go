// Code generated by mockery v2.13.1. DO NOT EDIT.

package mocks

import (
	"github.com/kyma-project/kyma/components/telemetry-operator/apis/telemetry/v1alpha1"
	"github.com/stretchr/testify/mock"
)

// FilesValidator is an autogenerated mock type for the FilesValidator type
type FilesValidator struct {
	mock.Mock
}

// Validate provides a mock function with given fields: logPipeline, logPipelines
func (_m *FilesValidator) Validate(logPipeline *v1alpha1.LogPipeline, logPipelines *v1alpha1.LogPipelineList) error {
	ret := _m.Called(logPipeline, logPipelines)

	var r0 error
	if rf, ok := ret.Get(0).(func(*v1alpha1.LogPipeline, *v1alpha1.LogPipelineList) error); ok {
		r0 = rf(logPipeline, logPipelines)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewFilesValidator interface {
	mock.TestingT
	Cleanup(func())
}

// NewFilesValidator creates a new instance of FilesValidator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewFilesValidator(t mockConstructorTestingTNewFilesValidator) *FilesValidator {
	mock := &FilesValidator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}