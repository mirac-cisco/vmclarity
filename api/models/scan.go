// Copyright © 2023 Cisco Systems, Inc. and its affiliates.
// All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

import "time"

func (s *Scan) GetState() (ScanState, bool) {
	var state ScanState
	var ok bool

	if s.State != nil {
		state, ok = *s.State, true
	}

	return state, ok
}

func (s *Scan) GetID() (string, bool) {
	var id string
	var ok bool

	if s.Id != nil {
		id, ok = *s.Id, true
	}

	return id, ok
}

func (s *Scan) GetScanConfigScope() (ScanScopeType, bool) {
	var scope ScanScopeType
	var ok bool

	if s.ScanConfigSnapshot != nil {
		scope, ok = s.ScanConfigSnapshot.GetScope()
	}

	return scope, ok
}

func (s *Scan) GetTimeoutSeconds() int {
	var timeoutSec int

	if s.ScanConfigSnapshot != nil {
		timeoutSec = s.ScanConfigSnapshot.GetTimeoutSeconds()
	}

	return timeoutSec
}

func (s *Scan) IsTimedOut(defaultTimeout time.Duration) bool {
	if s == nil || s.StartTime == nil {
		return false
	}
	// Use the provided timeout to calculate the timeoutTime by default.
	timeoutTime := s.StartTime.Add(defaultTimeout)
	// Use Scan.ScanConfigSnapshot.TimeoutSeconds to calculate timeoutTime
	// if it is set and its value is bigger than zero.
	if timeoutSeconds := s.GetTimeoutSeconds(); timeoutSeconds > 0 {
		timeoutTime = s.StartTime.Add(time.Duration(timeoutSeconds) * time.Second)
	}

	return time.Now().After(timeoutTime)
}
