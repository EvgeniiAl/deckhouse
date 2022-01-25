/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package requirements

import (
	"fmt"
	"sync"

	"github.com/tidwall/gjson"
)

var (
	once            sync.Once
	defaultRegistry requirementsResolver
)

func Register(key string, f CheckFunc) {
	once.Do(
		func() {
			defaultRegistry = newRegistry()
		},
	)

	defaultRegistry.Register(key, f)
}

func CheckRequirement(key, value string, getter ValueGetter) (bool, error) {
	if defaultRegistry == nil {
		return true, nil
	}
	f, err := defaultRegistry.GetByKey(key)
	if err != nil {
		panic(err)
	}

	return f(value, getter)
}

type CheckFunc func(requirementValue string, getter ValueGetter) (bool, error)

type ValueGetter interface {
	Get(path string) gjson.Result
}

type requirementsResolver interface {
	Register(key string, f CheckFunc)
	GetByKey(key string) (CheckFunc, error)
}

type requirementsRegistry struct {
	checkers map[string]CheckFunc
}

func newRegistry() *requirementsRegistry {
	return &requirementsRegistry{
		checkers: make(map[string]CheckFunc),
	}
}

func (r *requirementsRegistry) Register(key string, f CheckFunc) {
	r.checkers[key] = f
}

func (r *requirementsRegistry) GetByKey(key string) (CheckFunc, error) {
	f, ok := r.checkers[key]
	if !ok {
		return nil, fmt.Errorf("check function for %q requirement is not registred", key)
	}

	return f, nil
}
