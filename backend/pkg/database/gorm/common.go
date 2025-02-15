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

package gorm

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	jsonpatch "github.com/evanphx/json-patch"

	"github.com/openclarity/vmclarity/backend/pkg/database/types"
	"github.com/openclarity/vmclarity/shared/pkg/utils"
)

func getExistingObjByID(db *gorm.DB, schema, objID string, obj interface{}) error {
	filter := fmt.Sprintf("id eq '%s'", objID)
	err := ODataQuery(db, schema, &filter, nil, nil, nil, nil, nil, false, &obj)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return types.ErrNotFound
		}
		return err
	}

	return nil
}

func deleteObjByID(db *gorm.DB, objID string, obj interface{}) error {
	jsonQuotedID := fmt.Sprintf("\"%s\"", objID)
	if err := db.Where("`Data` -> '$.id' = ?", jsonQuotedID).Delete(obj).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return types.ErrNotFound
		}
		return err
	}

	return nil
}

func patchObject(original []byte, newobject interface{}) ([]byte, error) {
	marshaled, err := json.Marshal(newobject)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshal API model to DB model: %w", err)
	}

	// Apply the input doc as a json patch to the doc stored in the DB
	updated, err := jsonpatch.MergePatch(original, marshaled)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to apply patch to existing data: %w", err)
	}

	return updated, nil
}

func checkRevisionEtag(ifMatch *int, revision *int) error {
	if (ifMatch != nil && revision != nil && *ifMatch != *revision) || (ifMatch != nil && revision == nil) {
		return &types.PreconditionFailedError{
			Reason: fmt.Sprintf(
				"Revision %d does not match %d. The object may have been modified since you started the request.",
				*revision, *ifMatch),
		}
	}
	return nil
}

func bumpRevision(oldrevision *int) *int {
	if oldrevision != nil {
		return utils.PointerTo(*oldrevision + 1)
	}
	return utils.PointerTo(1)
}
