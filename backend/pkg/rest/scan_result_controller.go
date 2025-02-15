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

package rest

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/openclarity/vmclarity/api/models"
	"github.com/openclarity/vmclarity/backend/pkg/common"
	databaseTypes "github.com/openclarity/vmclarity/backend/pkg/database/types"
	"github.com/openclarity/vmclarity/shared/pkg/utils"
)

func (s *ServerImpl) GetScanResults(ctx echo.Context, params models.GetScanResultsParams) error {
	dbScanResults, err := s.dbHandler.ScanResultsTable().GetScanResults(params)
	if err != nil {
		return sendError(ctx, http.StatusInternalServerError, fmt.Sprintf("failed to get scans results from db: %v", err))
	}

	return sendResponse(ctx, http.StatusOK, dbScanResults)
}

func (s *ServerImpl) PostScanResults(ctx echo.Context) error {
	var scanResult models.TargetScanResult
	err := ctx.Bind(&scanResult)
	if err != nil {
		return sendError(ctx, http.StatusBadRequest, fmt.Sprintf("failed to bind request: %v", err))
	}

	createdScanResult, err := s.dbHandler.ScanResultsTable().CreateScanResult(scanResult)
	if err != nil {
		var conflictErr *common.ConflictError
		if errors.As(err, &conflictErr) {
			existResponse := &models.TargetScanResultExists{
				Message:          utils.PointerTo(conflictErr.Reason),
				TargetScanResult: &createdScanResult,
			}
			return sendResponse(ctx, http.StatusConflict, existResponse)
		}
		return sendError(ctx, http.StatusInternalServerError, fmt.Sprintf("failed to create scan result in db: %v", err))
	}

	return sendResponse(ctx, http.StatusCreated, createdScanResult)
}

func (s *ServerImpl) GetScanResultsScanResultID(ctx echo.Context, scanResultID models.ScanResultID, params models.GetScanResultsScanResultIDParams) error {
	dbScanResult, err := s.dbHandler.ScanResultsTable().GetScanResult(scanResultID, params)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sendError(ctx, http.StatusNotFound, err.Error())
		}
		return sendError(ctx, http.StatusInternalServerError, fmt.Sprintf("failed to get scan result from db. scanResultID=%v: %v", scanResultID, err))
	}

	return sendResponse(ctx, http.StatusOK, dbScanResult)
}

// nolint:cyclop
func (s *ServerImpl) PatchScanResultsScanResultID(ctx echo.Context, scanResultID models.ScanResultID, params models.PatchScanResultsScanResultIDParams) error {
	// TODO: check that the provided scan and target IDs are valid
	var scanResult models.TargetScanResult
	err := ctx.Bind(&scanResult)
	if err != nil {
		return sendError(ctx, http.StatusBadRequest, fmt.Sprintf("failed to bind request: %v", err))
	}

	// check that a scan result with that id exists.
	_, err = s.dbHandler.ScanResultsTable().GetScanResult(scanResultID, models.GetScanResultsScanResultIDParams{})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sendError(ctx, http.StatusNotFound, fmt.Sprintf("scan result was not found. scanResultID=%v: %v", scanResultID, err))
		}
		return sendError(ctx, http.StatusInternalServerError, fmt.Sprintf("failed to get scan result. scanResultID=%v: %v", scanResultID, err))
	}

	// PATCH request might not contain the ID in the body, so set it from
	// the URL field so that the DB layer knows which object is being updated.
	if scanResult.Id != nil && *scanResult.Id != scanResultID {
		return sendError(ctx, http.StatusBadRequest, fmt.Sprintf("id in body %s does not match object %s to be updated", *scanResult.Id, scanResultID))
	}
	scanResult.Id = &scanResultID

	updatedScanResult, err := s.dbHandler.ScanResultsTable().UpdateScanResult(scanResult, params)
	if err != nil {
		var validationErr *common.BadRequestError
		var conflictErr *common.ConflictError
		var preconditionFailedErr *databaseTypes.PreconditionFailedError
		switch true {
		case errors.As(err, &conflictErr):
			existResponse := &models.TargetScanResultExists{
				Message:          utils.PointerTo(conflictErr.Reason),
				TargetScanResult: &updatedScanResult,
			}
			return sendResponse(ctx, http.StatusConflict, existResponse)
		case errors.As(err, &validationErr):
			return sendError(ctx, http.StatusBadRequest, err.Error())
		case errors.As(err, &preconditionFailedErr):
			return sendError(ctx, http.StatusPreconditionFailed, err.Error())
		default:
			return sendError(ctx, http.StatusInternalServerError, fmt.Sprintf("failed to update scan result in db. scanResultID=%v: %v", scanResultID, err))
		}
	}

	return sendResponse(ctx, http.StatusOK, updatedScanResult)
}

// nolint:cyclop
func (s *ServerImpl) PutScanResultsScanResultID(ctx echo.Context, scanResultID models.ScanResultID, params models.PutScanResultsScanResultIDParams) error {
	// TODO: check that the provided scan and target IDs are valid
	var scanResult models.TargetScanResult
	err := ctx.Bind(&scanResult)
	if err != nil {
		return sendError(ctx, http.StatusBadRequest, fmt.Sprintf("failed to bind request: %v", err))
	}

	// check that a scan result with that id exists.
	_, err = s.dbHandler.ScanResultsTable().GetScanResult(scanResultID, models.GetScanResultsScanResultIDParams{})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sendError(ctx, http.StatusNotFound, fmt.Sprintf("scan result was not found. scanResultID=%v: %v", scanResultID, err))
		}
		return sendError(ctx, http.StatusInternalServerError, fmt.Sprintf("failed to get scan result. scanResultID=%v: %v", scanResultID, err))
	}

	// PUT request might not contain the ID in the body, so set it from
	// the URL field so that the DB layer knows which object is being updated.
	if scanResult.Id != nil && *scanResult.Id != scanResultID {
		return sendError(ctx, http.StatusBadRequest, fmt.Sprintf("id in body %s does not match object %s to be updated", *scanResult.Id, scanResultID))
	}
	scanResult.Id = &scanResultID

	updatedScanResult, err := s.dbHandler.ScanResultsTable().SaveScanResult(scanResult, params)
	if err != nil {
		var validationErr *common.BadRequestError
		var conflictErr *common.ConflictError
		var preconditionFailedErr *databaseTypes.PreconditionFailedError
		switch true {
		case errors.As(err, &conflictErr):
			existResponse := &models.TargetScanResultExists{
				Message:          utils.PointerTo(conflictErr.Reason),
				TargetScanResult: &updatedScanResult,
			}
			return sendResponse(ctx, http.StatusConflict, existResponse)
		case errors.As(err, &validationErr):
			return sendError(ctx, http.StatusBadRequest, err.Error())
		case errors.As(err, &preconditionFailedErr):
			return sendError(ctx, http.StatusPreconditionFailed, err.Error())
		default:
			return sendError(ctx, http.StatusInternalServerError, fmt.Sprintf("failed to update scan result in db. scanResultID=%v: %v", scanResultID, err))
		}
	}

	return sendResponse(ctx, http.StatusOK, updatedScanResult)
}
