// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package processor

import (
	"fmt"
	"regexp"

	"github.com/elastic/crd-ref-docs/config"
)

func compileConfig(conf *config.Config) (cc *compiledConfig, err error) {
	if conf == nil {
		return nil, nil
	}

	cc = &compiledConfig{
		ignoreTypes:         make([]*regexp.Regexp, len(conf.Processor.IgnoreTypes)),
		ignoreFields:        make([]*regexp.Regexp, len(conf.Processor.IgnoreFields)),
		ignoreGroupVersions: make([]*regexp.Regexp, len(conf.Processor.IgnoreGroupVersions)),
		useRawDocstring:     conf.Processor.UseRawDocstring,
		markers:             conf.Processor.CustomMarkers,
	}

	for i, t := range conf.Processor.IgnoreTypes {
		if cc.ignoreTypes[i], err = regexp.Compile(t); err != nil {
			return nil, fmt.Errorf("failed to compile type regex '%s': %w", t, err)
		}
	}

	for i, f := range conf.Processor.IgnoreFields {
		if cc.ignoreFields[i], err = regexp.Compile(f); err != nil {
			return nil, fmt.Errorf("failed to compile field regex '%s': %w", f, err)
		}
	}

	for i, gv := range conf.Processor.IgnoreGroupVersions {
		if cc.ignoreGroupVersions[i], err = regexp.Compile(gv); err != nil {
			return nil, fmt.Errorf("failed to compile group-version regex '%s': %w", gv, err)
		}
	}

	return
}

type compiledConfig struct {
	ignoreTypes         []*regexp.Regexp
	ignoreFields        []*regexp.Regexp
	ignoreGroupVersions []*regexp.Regexp
	useRawDocstring     bool
	markers             []config.Marker
}

func (cc *compiledConfig) shouldIgnoreGroupVersion(gv string) bool {
	if cc == nil {
		return false
	}

	for _, re := range cc.ignoreGroupVersions {
		if re.MatchString(gv) {
			return true
		}
	}

	return false
}

func (cc *compiledConfig) shouldIgnoreType(fqn string) bool {
	if cc == nil {
		return false
	}

	for _, re := range cc.ignoreTypes {
		if re.MatchString(fqn) {
			return true
		}
	}

	return false
}

func (cc *compiledConfig) shouldIgnoreField(typeName, fieldName string) bool {
	if cc == nil {
		return false
	}

	if fieldName == "-" {
		return true
	}

	fqn := typeName + "." + fieldName
	for _, re := range cc.ignoreFields {
		if re.MatchString(fqn) {
			return true
		}
	}

	return false
}
